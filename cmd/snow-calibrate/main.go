package main

import (
	"database/sql"
	"encoding/csv"
	"flag"
	"fmt"
	"math"
	"os"
	"sort"
	"time"

	_ "github.com/lib/pq"
	"gonum.org/v1/gonum/mat"
	"gonum.org/v1/gonum/stat"
)

// SnowReading represents a correlated snow distance and environmental factor reading
type SnowReading struct {
	Time         time.Time
	SnowDistance float64
	FactorValue  float64 // Environmental factor (temp, humidity, wind, etc.)
	FactorName   string
	SnowDrift    float64 // Deviation from auto-detected baseline
}

// MultiFactorReading represents readings with multiple environmental factors
type MultiFactorReading struct {
	Time         time.Time
	SnowDistance float64
	Temperature  float64
	Humidity     float64
	Windspeed    float64
	Barometer    float64
	SnowDrift    float64
}

// HourlyReading represents hourly averaged data from weather_1h table
type HourlyReading struct {
	Time                time.Time
	AvgSnowDistance     float64
	AvgFactorValue      float64
	CompensatedDistance float64
}

// ModelType represents different compensation models
type ModelType string

const (
	ModelConstant  ModelType = "constant"
	ModelLinear    ModelType = "linear"
	ModelQuadratic ModelType = "quadratic"
	ModelCubic     ModelType = "cubic"
)

// ANSI color codes
const (
	ColorReset   = "\033[0m"
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorWhite   = "\033[37m"
	ColorBold    = "\033[1m"
)

// CalibrationResult contains the analysis results for a specific model
type CalibrationResult struct {
	ModelType            ModelType
	ModelName            string
	FactorName           string    // Environmental factor name (temperature, humidity, etc.)
	BaselineDistance     float64
	Coefficients         []float64 // Model coefficients [c0, c1, c2, ...] where drift = c0 + c1*F + c2*F² + ...
	RSquared             float64
	AdjustedRSquared     float64
	MeanAbsoluteError    float64
	RootMeanSquaredError float64
	AIC                  float64 // Akaike Information Criterion (lower is better)
	BIC                  float64 // Bayesian Information Criterion (lower is better)
	SampleCount          int
	TemperatureRange     [2]float64
	DriftRange           [2]float64
}

// ComparisonResults contains all model results for comparison
type ComparisonResults struct {
	Models    []CalibrationResult
	BestByR2  CalibrationResult
	BestByAIC CalibrationResult
	BestByBIC CalibrationResult
}

func main() {
	// Command line flags
	var (
		dbHost        = flag.String("db-host", "localhost", "Database host")
		dbPort        = flag.Int("db-port", 5432, "Database port")
		dbUser        = flag.String("db-user", "postgres", "Database user")
		dbPass        = flag.String("db-pass", "", "Database password")
		dbName        = flag.String("db-name", "weather_v2_0_0", "Database name")
		station       = flag.String("station", "snow", "Snow gauge station name")
		factorStation = flag.String("factor-station", "CSI", "Station name for environmental factor")
		factor      = flag.String("factor", "all", "Environmental factor to test (all, temperature, humidity, windspeed, barometer)")
		hours       = flag.Int("hours", 24, "Number of hours of data to analyze")
		csvOutput   = flag.String("csv", "", "Optional CSV output file path")
		multiFactor = flag.Bool("multi", false, "Test two-factor combinations for better variance reduction")
	)
	flag.Parse()

	// Map factor name to database column
	factorColumns := map[string]string{
		"temperature": "outtemp",
		"humidity":    "outhumidity",
		"windspeed":   "windspeed",
		"barometer":   "barometer",
	}

	// Determine which factors to test
	var factorsToTest []string
	if *factor == "all" {
		factorsToTest = []string{"temperature", "humidity", "windspeed", "barometer"}
	} else {
		if _, ok := factorColumns[*factor]; !ok {
			fmt.Fprintf(os.Stderr, "Error: Unknown factor '%s'. Valid options: all, temperature, humidity, windspeed, barometer\n", *factor)
			os.Exit(1)
		}
		factorsToTest = []string{*factor}
	}

	// Connect to database
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		*dbHost, *dbPort, *dbUser, *dbPass, *dbName)

	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "Error pinging database: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("%s%sSnow Gauge Environmental Factor Calibration%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("============================================\n\n")
	fmt.Printf("%sConfiguration:%s\n", ColorBold, ColorReset)
	fmt.Printf("  Snow Station: %s%s%s\n", ColorCyan, *station, ColorReset)
	fmt.Printf("  Factor Station: %s%s%s\n", ColorCyan, *factorStation, ColorReset)
	fmt.Printf("  Testing Factors: %s%v%s\n", ColorYellow, factorsToTest, ColorReset)
	fmt.Printf("  Analysis Period: %s%d hours%s\n\n", ColorYellow, *hours, ColorReset)

	// Store results for each factor
	var allFactorResults []FactorResult

	// Test each factor
	for _, factorName := range factorsToTest {
		factorColumn := factorColumns[factorName]

		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorCyan, ColorReset)
		fmt.Printf("%sTesting Factor: %s%s%s\n", ColorBold, ColorYellow, factorName, ColorReset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorCyan, ColorReset)

		// Fetch correlated data
		readings := fetchCorrelatedData(db, *station, *factorStation, factorColumn, factorName, *hours)

		if len(readings) < 10 {
			fmt.Printf("%s⚠ Warning:%s Not enough data points for %s (%d). Skipping.\n\n", ColorYellow, ColorReset, factorName, len(readings))
			continue
		}

		fmt.Printf("%sCollected %d data points%s\n\n", ColorGreen, len(readings), ColorReset)

		// Find environmental factor compensation models
		results := findEnvironmentalModels(readings)

		// Store results
		allFactorResults = append(allFactorResults, FactorResult{
			FactorName: factorName,
			Results:    results,
			Readings:   readings,
		})

		// Display comparison for this factor
		displayComparison(results)
		fmt.Printf("\n")
	}

	// Compare all factors if testing multiple
	if len(allFactorResults) > 1 {
		displayFactorComparison(allFactorResults)
	}

	// Display metrics explanation
	displayMetricsExplanation()

	// If multi-factor flag is set, test two-factor combinations
	if *multiFactor {
		testMultiFactorCombinations(db, *station, *factorStation, *hours)
	}

	// Display details and generate code for the best overall factor
	if len(allFactorResults) > 0 {
		bestFactorResult := findBestFactor(allFactorResults)

		fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorGreen, ColorReset)
		fmt.Printf("%s%sBEST FACTOR: %s%s%s\n", ColorBold, ColorGreen, ColorYellow, bestFactorResult.FactorName, ColorReset)
		fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorGreen, ColorReset)

		displayBestModelDetails(bestFactorResult.Results.BestByAIC)
		generateCompensationCode(bestFactorResult.Results.BestByAIC)

		// Apply compensation to hourly averaged data
		displayHourlyCompensation(db, *station, *factorStation, bestFactorResult.Results.BestByAIC, *hours)

		// Optionally export to CSV (for best factor)
		if *csvOutput != "" {
			if err := exportCSV(*csvOutput, bestFactorResult.Readings, bestFactorResult.Results.BestByAIC); err != nil {
				fmt.Fprintf(os.Stderr, "%sError writing CSV: %v%s\n", ColorRed, err, ColorReset)
			} else {
				fmt.Printf("\n%sData exported to: %s%s%s\n", ColorGreen, ColorCyan, *csvOutput, ColorReset)
			}
		}
	}
}

// FactorResult holds results for a single environmental factor
type FactorResult struct {
	FactorName string
	Results    ComparisonResults
	Readings   []SnowReading
}

func displayFactorComparison(factorResults []FactorResult) {
	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorCyan, ColorReset)
	fmt.Printf("%sFACTOR COMPARISON (Best Model for Each)%s\n", colorizeHeader(""), "")
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorCyan, ColorReset)

	fmt.Printf("%-15s | %-15s | %8s | %8s | %10s\n", "Factor", "Best Model", "R²", "RMSE(mm)", "AIC")
	fmt.Printf("----------------+-----------------+----------+----------+------------\n")

	// Find best R² for highlighting
	bestR2 := 0.0
	for _, fr := range factorResults {
		if fr.Results.BestByAIC.RSquared > bestR2 {
			bestR2 = fr.Results.BestByAIC.RSquared
		}
	}

	for _, fr := range factorResults {
		best := fr.Results.BestByAIC
		r2Display := colorizeRSquared(best.RSquared)

		// Highlight the best factor
		factorName := fr.FactorName
		if best.RSquared == bestR2 {
			factorName = fmt.Sprintf("%s%s%s ★", ColorBold, fr.FactorName, ColorReset)
		}

		fmt.Printf("%-24s | %-15s | %s | %8.2f | %10.2f\n",
			factorName, best.ModelName, r2Display, best.RootMeanSquaredError, best.AIC)
	}

	fmt.Printf("\n%sInterpretation:%s\n", ColorBold, ColorReset)
	fmt.Printf("  R² (coefficient of determination):\n")
	fmt.Printf("    - %s> 0.7%s  = strong correlation\n", ColorGreen, ColorReset)
	fmt.Printf("    - %s0.3-0.7%s = moderate correlation\n", ColorYellow, ColorReset)
	fmt.Printf("    - %s< 0.3%s  = weak correlation\n", ColorRed, ColorReset)
	fmt.Printf("  The factor with the highest R² (marked with ★) has the strongest correlation with sensor drift.\n")
}

func findBestFactor(factorResults []FactorResult) FactorResult {
	if len(factorResults) == 0 {
		return FactorResult{}
	}

	best := factorResults[0]
	for _, fr := range factorResults[1:] {
		// Use R² as the primary criterion for best factor
		if fr.Results.BestByAIC.RSquared > best.Results.BestByAIC.RSquared {
			best = fr
		}
	}
	return best
}

func fetchCorrelatedData(db *sql.DB, snowStation, factorStation, factorColumn, factorName string, hours int) []SnowReading {
	// Build dynamic query with the specified factor column
	query := fmt.Sprintf(`
		SELECT
			s.time,
			s.snowdistance,
			f.%s
		FROM weather s
		INNER JOIN weather f
			ON f.stationname = $2
			AND f.time >= s.time - INTERVAL '2 minutes'
			AND f.time <= s.time + INTERVAL '2 minutes'
		WHERE s.stationname = $1
		  AND s.time >= NOW() - INTERVAL '1 hour' * $3
		  AND s.snowdistance IS NOT NULL
		  AND f.%s IS NOT NULL
		ORDER BY s.time
	`, factorColumn, factorColumn)

	rows, err := db.Query(query, snowStation, factorStation, hours)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying data: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var readings []SnowReading
	for rows.Next() {
		var r SnowReading
		r.FactorName = factorName
		if err := rows.Scan(&r.Time, &r.SnowDistance, &r.FactorValue); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			continue
		}
		// Don't calculate drift yet - we need to determine baseline first
		readings = append(readings, r)
	}

	return readings
}

// findEnvironmentalModels finds correlation between environmental factor and distance
// Baseline = mean distance (current reading with empty pad)
// Models describe how distance changes with the environmental factor
func findEnvironmentalModels(readings []SnowReading) ComparisonResults {
	// Extract factor values and raw distance values
	n := len(readings)
	factorValues := make([]float64, n)
	distances := make([]float64, n)
	for i, r := range readings {
		factorValues[i] = r.FactorValue
		distances[i] = r.SnowDistance
	}

	// Calculate baseline as mean distance (current empty pad reading)
	baseline := stat.Mean(distances, nil)

	// Calculate drift from baseline for all readings
	for i := range readings {
		readings[i].SnowDrift = readings[i].SnowDistance - baseline
	}

	factorName := readings[0].FactorName

	// Fit models to predict drift from environmental factor
	models := []CalibrationResult{
		fitConstantDriftModel(factorValues, readings, baseline),
		fitLinearDriftModel(factorValues, readings, baseline),
		fitPolynomialDriftModel(factorValues, readings, baseline, 2), // Quadratic
		fitPolynomialDriftModel(factorValues, readings, baseline, 3), // Cubic
	}

	// Find best models by different criteria
	var comparison ComparisonResults
	comparison.Models = models

	// Best by R²
	bestR2 := models[0]
	for _, m := range models {
		if m.RSquared > bestR2.RSquared {
			bestR2 = m
		}
	}
	comparison.BestByR2 = bestR2

	// Best by AIC (lower is better, balances fit quality with model complexity)
	bestAIC := models[0]
	for _, m := range models {
		if m.AIC < bestAIC.AIC {
			bestAIC = m
		}
	}
	comparison.BestByAIC = bestAIC

	// Best by BIC (lower is better, penalizes complexity more than AIC)
	bestBIC := models[0]
	for _, m := range models {
		if m.BIC < bestBIC.BIC {
			bestBIC = m
		}
	}
	comparison.BestByBIC = bestBIC

	fmt.Printf("Baseline Determination:\n")
	fmt.Printf("  Mean distance (empty pad): %.2f mm (%.3f inches)\n", baseline, baseline/25.4)
	fmt.Printf("  %s range: %.2f to %.2f\n", factorName, bestAIC.TemperatureRange[0], bestAIC.TemperatureRange[1])
	fmt.Printf("  Drift range: %.2f mm to %.2f mm\n\n", bestAIC.DriftRange[0], bestAIC.DriftRange[1])

	return comparison
}

func fitConstantDriftModel(factorValues []float64, readings []SnowReading, baseline float64) CalibrationResult {
	n := len(readings)

	// Constant model: drift is independent of environmental factor (always zero)
	// This is the baseline - no compensation
	drifts := make([]float64, n)
	factorName := ""
	for i, r := range readings {
		drifts[i] = r.SnowDrift
		if i == 0 {
			factorName = r.FactorName
		}
	}

	result := CalibrationResult{
		ModelType:        ModelConstant,
		ModelName:        "Constant (No Compensation)",
		FactorName:       factorName,
		BaselineDistance: baseline,
		Coefficients:     []float64{0.0}, // No drift
		SampleCount:      n,
	}

	predictFunc := func(t float64) float64 { return 0.0 }

	// Calculate errors based on drift prediction
	result.RSquared = 0.0 // Constant model explains no variance by definition
	result.AdjustedRSquared = 0.0
	result.MeanAbsoluteError = calculateMAE(factorValues, drifts, predictFunc)
	result.RootMeanSquaredError = calculateRMSE(factorValues, drifts, predictFunc)

	// Calculate AIC and BIC
	k := 1.0 // number of parameters
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	// Calculate ranges
	minFactor, maxFactor := minMax(factorValues)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minFactor, maxFactor}
	result.DriftRange = [2]float64{minDrift, maxDrift}

	return result
}

func fitLinearDriftModel(factorValues []float64, readings []SnowReading, baseline float64) CalibrationResult {
	n := len(readings)

	// Extract drifts
	drifts := make([]float64, n)
	factorName := ""
	for i, r := range readings {
		drifts[i] = r.SnowDrift
		if i == 0 {
			factorName = r.FactorName
		}
	}

	// Linear regression: drift = c0 + c1*Factor
	slope, intercept := stat.LinearRegression(factorValues, drifts, nil, false)

	result := CalibrationResult{
		ModelType:        ModelLinear,
		ModelName:        "Linear",
		FactorName:       factorName,
		BaselineDistance: baseline,
		Coefficients:     []float64{intercept, slope},
		SampleCount:      n,
	}

	predictFunc := func(f float64) float64 {
		return intercept + slope*f
	}

	// Calculate errors and metrics
	result.RSquared = calculateRSquared(factorValues, drifts, predictFunc)
	result.AdjustedRSquared = calculateAdjustedRSquared(result.RSquared, float64(n), 2.0)
	result.MeanAbsoluteError = calculateMAE(factorValues, drifts, predictFunc)
	result.RootMeanSquaredError = calculateRMSE(factorValues, drifts, predictFunc)

	k := 2.0 // intercept + slope
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	minFactor, maxFactor := minMax(factorValues)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minFactor, maxFactor}
	result.DriftRange = [2]float64{minDrift, maxDrift}

	return result
}

func fitPolynomialDriftModel(factorValues []float64, readings []SnowReading, baseline float64, degree int) CalibrationResult {
	n := len(readings)

	// Extract drifts
	drifts := make([]float64, n)
	factorName := ""
	for i, r := range readings {
		drifts[i] = r.SnowDrift
		if i == 0 {
			factorName = r.FactorName
		}
	}

	// Build Vandermonde matrix for polynomial regression
	// Model: drift = c0 + c1*F + c2*F² + ... + cd*F^d
	X := mat.NewDense(n, degree+1, nil)
	for i := 0; i < n; i++ {
		for j := 0; j <= degree; j++ {
			X.Set(i, j, math.Pow(factorValues[i], float64(j)))
		}
	}

	y := mat.NewVecDense(n, drifts)

	// Solve using QR decomposition
	var qr mat.QR
	qr.Factorize(X)

	var coeffs mat.Dense
	err := coeffs.Solve(X, y)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error solving polynomial regression: %v\n", err)
		return CalibrationResult{}
	}

	// Extract coefficients
	coeff := make([]float64, degree+1)
	for i := 0; i <= degree; i++ {
		coeff[i] = coeffs.At(i, 0)
	}

	modelType := ModelQuadratic
	modelName := "Quadratic"
	if degree == 3 {
		modelType = ModelCubic
		modelName = "Cubic"
	}

	result := CalibrationResult{
		ModelType:        modelType,
		ModelName:        modelName,
		FactorName:       factorName,
		BaselineDistance: baseline,
		Coefficients:     coeff,
		SampleCount:      n,
	}

	predictFunc := func(f float64) float64 {
		pred := 0.0
		for i, c := range coeff {
			pred += c * math.Pow(f, float64(i))
		}
		return pred
	}

	// Calculate errors and metrics
	result.RSquared = calculateRSquared(factorValues, drifts, predictFunc)
	result.AdjustedRSquared = calculateAdjustedRSquared(result.RSquared, float64(n), float64(degree+1))
	result.MeanAbsoluteError = calculateMAE(factorValues, drifts, predictFunc)
	result.RootMeanSquaredError = calculateRMSE(factorValues, drifts, predictFunc)

	k := float64(degree + 1)
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	minFactor, maxFactor := minMax(factorValues)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minFactor, maxFactor}
	result.DriftRange = [2]float64{minDrift, maxDrift}

	return result
}

func calculateRSquared(x, y []float64, predictFunc func(float64) float64) float64 {
	var sumY float64
	for _, val := range y {
		sumY += val
	}
	meanY := sumY / float64(len(y))

	var ssTot, ssRes float64
	for i := range y {
		var predicted float64
		if x != nil {
			predicted = predictFunc(x[i])
		} else {
			predicted = predictFunc(0)
		}
		ssTot += math.Pow(y[i]-meanY, 2)
		ssRes += math.Pow(y[i]-predicted, 2)
	}

	if ssTot == 0 {
		return 0
	}
	return 1 - (ssRes / ssTot)
}

func calculateAdjustedRSquared(r2, n, k float64) float64 {
	if n-k-1 <= 0 {
		return 0
	}
	return 1 - ((1-r2)*(n-1))/(n-k-1)
}

func calculateMAE(x, y []float64, predictFunc func(float64) float64) float64 {
	var sumAbsError float64
	for i := range y {
		var predicted float64
		if x != nil {
			predicted = predictFunc(x[i])
		} else {
			predicted = predictFunc(0)
		}
		sumAbsError += math.Abs(y[i] - predicted)
	}
	return sumAbsError / float64(len(y))
}

func calculateRMSE(x, y []float64, predictFunc func(float64) float64) float64 {
	var sumSqError float64
	for i := range y {
		var predicted float64
		if x != nil {
			predicted = predictFunc(x[i])
		} else {
			predicted = predictFunc(0)
		}
		sumSqError += math.Pow(y[i]-predicted, 2)
	}
	return math.Sqrt(sumSqError / float64(len(y)))
}

func calculateAIC(n, rmse, k float64) float64 {
	// AIC = 2k + n*ln(SSE/n)
	// where SSE = sum of squared errors = n * rmse²
	sse := n * rmse * rmse
	if sse <= 0 {
		return math.Inf(1)
	}
	return 2*k + n*math.Log(sse/n)
}

func calculateBIC(n, rmse, k float64) float64 {
	// BIC = k*ln(n) + n*ln(SSE/n)
	sse := n * rmse * rmse
	if sse <= 0 {
		return math.Inf(1)
	}
	return k*math.Log(n) + n*math.Log(sse/n)
}

func minMax(values []float64) (float64, float64) {
	if len(values) == 0 {
		return 0, 0
	}
	min, max := values[0], values[0]
	for _, v := range values {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	return min, max
}

func displayComparison(results ComparisonResults) {
	fmt.Printf("%sModel Comparison%s\n", ColorBold, ColorReset)
	fmt.Printf("================\n\n")

	// Sort models by AIC for display
	models := make([]CalibrationResult, len(results.Models))
	copy(models, results.Models)
	sort.Slice(models, func(i, j int) bool {
		return models[i].AIC < models[j].AIC
	})

	fmt.Printf("%-15s | %8s | %8s | %8s | %10s | %10s\n", "Model", "R²", "Adj R²", "RMSE(mm)", "AIC", "BIC")
	fmt.Printf("----------------+----------+----------+----------+------------+------------\n")

	for _, m := range models {
		marker := ""
		r2Display := colorizeRSquared(m.RSquared)
		if m.ModelType == results.BestByAIC.ModelType {
			marker = fmt.Sprintf(" %s← BEST (AIC)%s", ColorGreen, ColorReset)
		}
		fmt.Printf("%-15s | %s | %8.4f | %8.2f | %10.2f | %10.2f%s\n",
			m.ModelName, r2Display, m.AdjustedRSquared, m.RootMeanSquaredError, m.AIC, m.BIC, marker)
	}

	fmt.Printf("\n%sRecommendation:%s\n", ColorBold, ColorReset)
	fmt.Printf("  Best model by AIC: %s%s%s\n", ColorCyan, results.BestByAIC.ModelName, ColorReset)
	if results.BestByAIC.ModelType != results.BestByBIC.ModelType {
		fmt.Printf("  Best model by BIC: %s%s%s (more conservative, penalizes complexity)\n",
			ColorCyan, results.BestByBIC.ModelName, ColorReset)
	}

	if results.BestByAIC.RSquared < 0.3 {
		fmt.Printf("\n  %s⚠ WARNING:%s Low R² (%s) - factor may not be primary drift cause!\n",
			ColorRed, ColorReset, colorizeRSquared(results.BestByAIC.RSquared))
		fmt.Printf("  Consider testing other environmental factors\n")
	} else if results.BestByAIC.RSquared < 0.7 {
		fmt.Printf("\n  %sℹ INFO:%s Moderate correlation (R²=%s) - useful but may not capture all drift\n",
			ColorYellow, ColorReset, colorizeRSquared(results.BestByAIC.RSquared))
	} else {
		fmt.Printf("\n  %s✓ EXCELLENT:%s Strong correlation (R²=%s) - factor is primary drift cause\n",
			ColorGreen, ColorReset, colorizeRSquared(results.BestByAIC.RSquared))
	}
	fmt.Println()
}

func displayBestModelDetails(model CalibrationResult) {
	fmt.Printf("%sBest Model Details (%s%s%s)%s\n", ColorBold, ColorCyan, model.ModelName, ColorBold, ColorReset)
	fmt.Printf("=====================\n\n")

	// Get factor display info
	factorSymbol := "F"
	factorUnit := ""
	factorName := model.FactorName
	if factorName == "" {
		factorName = "factor"
	}

	switch factorName {
	case "temperature":
		factorSymbol = "T"
		factorUnit = "°F"
	case "humidity":
		factorSymbol = "H"
		factorUnit = "%"
	case "windspeed":
		factorSymbol = "W"
		factorUnit = "mph"
	case "barometer":
		factorSymbol = "B"
		factorUnit = "inHg"
	}

	fmt.Printf("Model equation:\n  ")
	switch model.ModelType {
	case ModelConstant:
		fmt.Printf("drift = %.4f mm\n", model.Coefficients[0])
	case ModelLinear:
		fmt.Printf("drift = %.6f + %.6f × %s\n", model.Coefficients[0], model.Coefficients[1], factorSymbol)
	case ModelQuadratic:
		fmt.Printf("drift = %.6f + %.6f × %s + %.6f × %s²\n",
			model.Coefficients[0], model.Coefficients[1], factorSymbol, model.Coefficients[2], factorSymbol)
	case ModelCubic:
		fmt.Printf("drift = %.6f + %.6f × %s + %.6f × %s² + %.6f × %s³\n",
			model.Coefficients[0], model.Coefficients[1], factorSymbol,
			model.Coefficients[2], factorSymbol, model.Coefficients[3], factorSymbol)
	}

	if factorUnit != "" {
		fmt.Printf("  (%s in %s, drift in mm)\n\n", factorSymbol, factorUnit)
	} else {
		fmt.Printf("  (drift in mm)\n\n")
	}

	fmt.Printf("%sQuality Metrics:%s\n", ColorBold, ColorReset)
	fmt.Printf("  R² = %s\n", colorizeRSquared(model.RSquared))
	fmt.Printf("  Adjusted R² = %.4f\n", model.AdjustedRSquared)
	fmt.Printf("  RMSE = %.2f mm (%.3f inches)\n", model.RootMeanSquaredError, model.RootMeanSquaredError/25.4)
	fmt.Printf("  MAE = %.2f mm (%.3f inches)\n", model.MeanAbsoluteError, model.MeanAbsoluteError/25.4)
	fmt.Printf("  Sample size = %s%d%s\n\n", ColorCyan, model.SampleCount, ColorReset)

	// Factor-specific example values
	var exampleValues []float64
	switch factorName {
	case "temperature":
		exampleValues = []float64{20, 30, 40, 50, 60}
	case "humidity":
		exampleValues = []float64{20, 40, 60, 80, 100}
	case "windspeed":
		exampleValues = []float64{0, 5, 10, 15, 20}
	case "barometer":
		exampleValues = []float64{29.5, 29.8, 30.0, 30.2, 30.5}
	default:
		exampleValues = []float64{20, 40, 60, 80, 100}
	}

	fmt.Printf("%s Impact Examples:\n", capitalize(factorName))
	for _, val := range exampleValues {
		drift := evaluateModel(model, val)
		if factorUnit != "" {
			fmt.Printf("  At %6.1f %s: %6.2f mm drift (%6.3f inches)\n", val, factorUnit, drift, drift/25.4)
		} else {
			fmt.Printf("  At %6.1f: %6.2f mm drift (%6.3f inches)\n", val, drift, drift/25.4)
		}
	}
	fmt.Println()
}

func evaluateModel(model CalibrationResult, temp float64) float64 {
	result := 0.0
	for i, coeff := range model.Coefficients {
		result += coeff * math.Pow(temp, float64(i))
	}
	return result
}

func displayMetricsExplanation() {
	fmt.Printf("\n")
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s%sMETRICS EXPLANATION%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorCyan, ColorReset)

	fmt.Printf("%sR² (Coefficient of Determination):%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Range: 0.0 to 1.0 (higher is better)\n")
	fmt.Printf("  • Meaning: Proportion of variance in sensor drift explained by the environmental factor\n")
	fmt.Printf("  • Interpretation:\n")
	fmt.Printf("    - %sR² > 0.7%s  = Strong correlation - factor is primary cause of drift\n", ColorGreen, ColorReset)
	fmt.Printf("    - %sR² 0.3-0.7%s = Moderate correlation - factor contributes to drift\n", ColorYellow, ColorReset)
	fmt.Printf("    - %sR² < 0.3%s  = Weak correlation - factor has little effect on drift\n", ColorRed, ColorReset)
	fmt.Printf("  • Example: R² = 0.85 means 85%% of sensor drift is explained by this factor\n\n")

	fmt.Printf("%sAdjusted R²:%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Same as R² but penalizes model complexity\n")
	fmt.Printf("  • Prevents overfitting by accounting for number of parameters\n")
	fmt.Printf("  • Use this when comparing models with different numbers of coefficients\n\n")

	fmt.Printf("%sRMSE (Root Mean Squared Error):%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Units: mm (millimeters)\n")
	fmt.Printf("  • Meaning: Average magnitude of prediction errors\n")
	fmt.Printf("  • Interpretation: Lower is better - shows typical error size\n")
	fmt.Printf("  • Example: RMSE = 2.5 mm means predictions are typically off by ±2.5 mm\n\n")

	fmt.Printf("%sMAE (Mean Absolute Error):%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Units: mm (millimeters)\n")
	fmt.Printf("  • Meaning: Average absolute difference between predicted and actual drift\n")
	fmt.Printf("  • Similar to RMSE but less sensitive to large outliers\n")
	fmt.Printf("  • Example: MAE = 2.0 mm means average error is 2.0 mm\n\n")

	fmt.Printf("%sAIC (Akaike Information Criterion):%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Lower is better (can be negative)\n")
	fmt.Printf("  • Balances model fit quality vs. complexity\n")
	fmt.Printf("  • Use this to choose between different model types (constant/linear/quadratic/cubic)\n")
	fmt.Printf("  • Prevents overfitting by penalizing extra parameters\n\n")

	fmt.Printf("%sBIC (Bayesian Information Criterion):%s\n", ColorBold, ColorReset)
	fmt.Printf("  • Lower is better (can be negative)\n")
	fmt.Printf("  • More conservative than AIC - penalizes complexity more heavily\n")
	fmt.Printf("  • Prefers simpler models compared to AIC\n\n")

	fmt.Printf("%sFor This Tool:%s\n", ColorBold, ColorReset)
	fmt.Printf("  %s1.%s Use R² to compare %sDIFFERENT FACTORS%s (temperature vs humidity vs wind...)\n", ColorCyan, ColorReset, ColorYellow, ColorReset)
	fmt.Printf("     → Factor with highest R² has strongest effect on sensor drift\n")
	fmt.Printf("  %s2.%s Use AIC to compare %sDIFFERENT MODELS%s for the %sSAME FACTOR%s\n", ColorCyan, ColorReset, ColorYellow, ColorReset, ColorYellow, ColorReset)
	fmt.Printf("     → Model with lowest AIC is best (balances accuracy vs complexity)\n")
	fmt.Printf("  %s3.%s Use RMSE to understand prediction accuracy in real units (mm)\n", ColorCyan, ColorReset)
	fmt.Printf("     → Lower RMSE = more accurate compensation\n\n")
}

func generateCompensationCode(model CalibrationResult) {
	fmt.Printf("%sGo Code Implementation%s\n", ColorBold, ColorReset)
	fmt.Printf("======================\n\n")

	factorParam := model.FactorName
	if factorParam == "" {
		factorParam = "factor"
	}
	funcName := fmt.Sprintf("compensateSnowDistanceFor%s", capitalize(factorParam))

	fmt.Printf("// %s compensation function - %s model\n", capitalize(model.FactorName), model.ModelName)
	fmt.Printf("// Calibrated on %d samples, R² = %.4f, RMSE = %.2f mm\n",
		model.SampleCount, model.RSquared, model.RootMeanSquaredError)
	fmt.Printf("func %s(rawDistance float32, %s float32) float32 {\n", funcName, factorParam)

	switch model.ModelType {
	case ModelConstant:
		fmt.Printf("    // Constant offset model\n")
		fmt.Printf("    const offset = %.6f  // mm\n", model.Coefficients[0])
		fmt.Printf("    compensated := rawDistance - float32(offset)\n")

	case ModelLinear:
		fmt.Printf("    // Linear model: drift = c0 + c1*F\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    expectedDrift := float32(c0 + c1*float64(%s))\n", factorParam)
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")

	case ModelQuadratic:
		fmt.Printf("    // Quadratic model: drift = c0 + c1*F + c2*F²\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    const c2 = %.6f\n", model.Coefficients[2])
		fmt.Printf("    f := float64(%s)\n", factorParam)
		fmt.Printf("    expectedDrift := float32(c0 + c1*f + c2*f*f)\n")
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")

	case ModelCubic:
		fmt.Printf("    // Cubic model: drift = c0 + c1*F + c2*F² + c3*F³\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    const c2 = %.6f\n", model.Coefficients[2])
		fmt.Printf("    const c3 = %.6f\n", model.Coefficients[3])
		fmt.Printf("    f := float64(%s)\n", factorParam)
		fmt.Printf("    expectedDrift := float32(c0 + c1*f + c2*f*f + c3*f*f*f)\n")
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")
	}

	fmt.Printf("    return compensated\n")
	fmt.Printf("}\n\n")

	fmt.Printf("// Usage in your snow gauge code:\n")
	fmt.Printf("// compensatedDistance := %s(snowdistance, %s)\n", funcName, factorParam)
	fmt.Printf("// snowDepth := baseDistance - compensatedDistance\n")
}

func capitalize(s string) string {
	if s == "" {
		return s
	}
	return string(s[0]-32) + s[1:]
}

// colorizeRSquared returns color-coded R² string based on correlation strength
func colorizeRSquared(r2 float64) string {
	color := ColorRed
	if r2 >= 0.7 {
		color = ColorGreen
	} else if r2 >= 0.3 {
		color = ColorYellow
	}
	return fmt.Sprintf("%s%.4f%s", color, r2, ColorReset)
}

// colorizeHeader returns a bold, colored header
func colorizeHeader(text string) string {
	return fmt.Sprintf("%s%s%s%s", ColorBold, ColorCyan, text, ColorReset)
}

func exportCSV(filename string, readings []SnowReading, model CalibrationResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	factorName := "Unknown"
	if len(readings) > 0 {
		factorName = readings[0].FactorName
	}

	// Write header
	header := []string{"Time", factorName, "SnowDistance_mm", "Drift_mm", "Predicted_Drift_mm", "Residual_mm"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, r := range readings {
		predicted := evaluateModel(model, r.FactorValue)
		residual := r.SnowDrift - predicted

		record := []string{
			r.Time.Format(time.RFC3339),
			fmt.Sprintf("%.2f", r.FactorValue),
			fmt.Sprintf("%.2f", r.SnowDistance),
			fmt.Sprintf("%.2f", r.SnowDrift),
			fmt.Sprintf("%.2f", predicted),
			fmt.Sprintf("%.2f", residual),
		}
		if err := writer.Write(record); err != nil {
			return err
		}
	}

	return nil
}

// fetchHourlyData retrieves hourly averaged snow distance and factor values from weather_1h
func fetchHourlyData(db *sql.DB, snowStation, factorStation, factorColumn, factorName string, hours int) []HourlyReading {
	// Map factor names to weather_1h column names
	factorColumns := map[string]string{
		"temperature": "outtemp",
		"humidity":    "outhumidity",
		"windspeed":   "windspeed",
		"barometer":   "barometer",
	}

	column := factorColumns[factorName]
	if column == "" {
		column = factorColumn
	}

	query := fmt.Sprintf(`
		SELECT
			s.bucket,
			s.snowdistance,
			f.%s
		FROM weather_1h s
		JOIN weather_1h f ON s.bucket = f.bucket AND f.stationname = $2
		WHERE s.stationname = $1
		  AND s.bucket >= NOW() - INTERVAL '%d hours'
		  AND s.snowdistance IS NOT NULL
		  AND f.%s IS NOT NULL
		ORDER BY s.bucket DESC
	`, column, hours, column)

	rows, err := db.Query(query, snowStation, factorStation)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError querying hourly data: %v%s\n", ColorRed, err, ColorReset)
		return nil
	}
	defer rows.Close()

	var readings []HourlyReading
	for rows.Next() {
		var r HourlyReading
		if err := rows.Scan(&r.Time, &r.AvgSnowDistance, &r.AvgFactorValue); err != nil {
			fmt.Fprintf(os.Stderr, "%sError scanning hourly row: %v%s\n", ColorRed, err, ColorReset)
			continue
		}
		readings = append(readings, r)
	}

	return readings
}

// applyCompensation applies the calibration model to calculate compensated distance
func applyCompensation(model CalibrationResult, rawDistance, factorValue float64) float64 {
	// Calculate expected drift based on the model
	drift := evaluateModel(model, factorValue)

	// Compensate by subtracting the expected drift
	compensated := rawDistance - drift

	return compensated
}

// displayHourlyCompensation shows a table of hourly averaged data with compensation applied
func displayHourlyCompensation(db *sql.DB, snowStation, factorStation string, model CalibrationResult, hours int) {
	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorCyan, ColorReset)
	fmt.Printf("%s%sHOURLY COMPENSATION RESULTS%s\n", ColorBold, ColorCyan, ColorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorCyan, ColorReset)

	// Get factor column mapping
	factorColumns := map[string]string{
		"temperature": "outtemp",
		"humidity":    "outhumidity",
		"windspeed":   "windspeed",
		"barometer":   "barometer",
	}

	factorColumn := factorColumns[model.FactorName]
	if factorColumn == "" {
		fmt.Fprintf(os.Stderr, "%sError: Unknown factor name %s%s\n", ColorRed, model.FactorName, ColorReset)
		return
	}

	// Fetch hourly averaged data
	readings := fetchHourlyData(db, snowStation, factorStation, factorColumn, model.FactorName, hours)

	if len(readings) == 0 {
		fmt.Printf("%s⚠ Warning:%s No hourly data available\n\n", ColorYellow, ColorReset)
		return
	}

	// Apply compensation to each reading
	for i := range readings {
		readings[i].CompensatedDistance = applyCompensation(model, readings[i].AvgSnowDistance, readings[i].AvgFactorValue)
	}

	// Get factor display info
	factorUnit := ""
	switch model.FactorName {
	case "temperature":
		factorUnit = "°F"
	case "humidity":
		factorUnit = "%"
	case "windspeed":
		factorUnit = "mph"
	case "barometer":
		factorUnit = "inHg"
	}

	fmt.Printf("Applying %s%s%s compensation model to hourly averages from weather_1h\n\n", ColorCyan, model.ModelName, ColorReset)

	// Table header
	fmt.Printf("%-20s | %10s | %12s | %12s | %12s\n",
		"Time (Hour)",
		capitalize(model.FactorName),
		"Raw Dist (mm)",
		"Drift (mm)",
		"Corrected (mm)")
	fmt.Printf("---------------------+------------+--------------+--------------+--------------\n")

	// Display data (limit to first 24 hours for readability)
	displayCount := len(readings)
	if displayCount > 24 {
		displayCount = 24
	}

	for i := 0; i < displayCount; i++ {
		r := readings[i]
		drift := evaluateModel(model, r.AvgFactorValue)

		// Color code the drift based on magnitude
		driftStr := fmt.Sprintf("%8.2f", drift)
		if math.Abs(drift) > 5.0 {
			driftStr = fmt.Sprintf("%s%8.2f%s", ColorRed, drift, ColorReset)
		} else if math.Abs(drift) > 2.0 {
			driftStr = fmt.Sprintf("%s%8.2f%s", ColorYellow, drift, ColorReset)
		} else {
			driftStr = fmt.Sprintf("%s%8.2f%s", ColorGreen, drift, ColorReset)
		}

		factorStr := fmt.Sprintf("%.1f %s", r.AvgFactorValue, factorUnit)

		fmt.Printf("%-20s | %10s | %12.2f | %s | %12.2f\n",
			r.Time.Format("2006-01-02 15:04"),
			factorStr,
			r.AvgSnowDistance,
			driftStr,
			r.CompensatedDistance)
	}

	if len(readings) > displayCount {
		fmt.Printf("\n%s... showing first %d of %d hours%s\n", ColorYellow, displayCount, len(readings), ColorReset)
	}

	// Summary statistics
	fmt.Printf("\n%sSummary:%s\n", ColorBold, ColorReset)

	var totalDrift float64
	var minDrift, maxDrift float64 = math.MaxFloat64, -math.MaxFloat64

	// Calculate variance before and after compensation
	rawDistances := make([]float64, len(readings))
	correctedDistances := make([]float64, len(readings))

	for i, r := range readings {
		drift := evaluateModel(model, r.AvgFactorValue)
		rawDistances[i] = r.AvgSnowDistance
		correctedDistances[i] = r.CompensatedDistance

		totalDrift += drift

		if drift < minDrift {
			minDrift = drift
		}
		if drift > maxDrift {
			maxDrift = drift
		}
	}

	avgDrift := totalDrift / float64(len(readings))

	// Calculate standard deviation before and after
	rawStdDev := stat.StdDev(rawDistances, nil)
	correctedStdDev := stat.StdDev(correctedDistances, nil)
	varianceReduction := ((rawStdDev - correctedStdDev) / rawStdDev) * 100

	fmt.Printf("  Average drift correction: %s%.2f mm%s (%.3f inches)\n",
		ColorCyan, avgDrift, ColorReset, avgDrift/25.4)
	fmt.Printf("  Drift range: %.2f to %.2f mm\n", minDrift, maxDrift)

	fmt.Printf("\n%sVariance Analysis:%s\n", ColorBold, ColorReset)
	fmt.Printf("  Raw distance std dev:       %s%.2f mm%s (%.3f inches)\n",
		ColorYellow, rawStdDev, ColorReset, rawStdDev/25.4)
	fmt.Printf("  Corrected distance std dev: %s%.2f mm%s (%.3f inches)\n",
		ColorCyan, correctedStdDev, ColorReset, correctedStdDev/25.4)

	// Color code the variance reduction
	reductionColor := ColorRed
	reductionLabel := "Poor"
	if varianceReduction > 50 {
		reductionColor = ColorGreen
		reductionLabel = "Excellent"
	} else if varianceReduction > 25 {
		reductionColor = ColorYellow
		reductionLabel = "Moderate"
	}

	fmt.Printf("  Variance reduction:         %s%.1f%% (%s)%s\n",
		reductionColor, varianceReduction, reductionLabel, ColorReset)

	// Calculate 1% accuracy specification compliance
	meanDistance := stat.Mean(rawDistances, nil)
	onePercentThreshold := meanDistance * 0.01 // 1% of mean reading

	fmt.Printf("\n%sAccuracy Specification Check (1%%):%s\n", ColorBold, ColorReset)
	fmt.Printf("  Mean sensor reading:        %.2f mm\n", meanDistance)
	fmt.Printf("  1%% accuracy threshold:     ±%.2f mm (±%.3f inches)\n",
		onePercentThreshold, onePercentThreshold/25.4)

	// Check raw data against spec
	rawAccuracyColor := ColorRed
	rawAccuracyLabel := "FAILS"
	if rawStdDev <= onePercentThreshold {
		rawAccuracyColor = ColorGreen
		rawAccuracyLabel = "PASSES"
	}
	fmt.Printf("  Raw data variability:       %.2f mm %s(spec requires ≤%.2f mm) %s%s%s\n",
		rawStdDev, rawAccuracyColor, onePercentThreshold, rawAccuracyLabel, ColorReset, "")

	// Check corrected data against spec
	correctedAccuracyColor := ColorRed
	correctedAccuracyLabel := "FAILS"
	if correctedStdDev <= onePercentThreshold {
		correctedAccuracyColor = ColorGreen
		correctedAccuracyLabel = "PASSES"
	}
	fmt.Printf("  Corrected data variability: %.2f mm %s(spec requires ≤%.2f mm) %s%s%s\n",
		correctedStdDev, correctedAccuracyColor, onePercentThreshold, correctedAccuracyLabel, ColorReset, "")

	// Overall assessment
	fmt.Printf("\n%s📊 Specification Compliance:%s\n", ColorBold, ColorReset)
	if rawStdDev <= onePercentThreshold {
		fmt.Printf("  ✅ %sSensor MEETS 1%% accuracy spec without compensation%s\n", ColorGreen, ColorReset)
	} else if correctedStdDev <= onePercentThreshold {
		fmt.Printf("  ✅ %sSensor MEETS 1%% accuracy spec WITH %s compensation%s\n",
			ColorGreen, model.FactorName, ColorReset)
		fmt.Printf("  💡 Compensation is %sREQUIRED%s to meet specification\n", ColorYellow, ColorReset)
	} else {
		fmt.Printf("  ❌ %sSensor FAILS 1%% accuracy spec even with compensation%s\n", ColorRed, ColorReset)
		fmt.Printf("  📊 Current variability: %.2f mm (%.1f%% of reading)\n",
			correctedStdDev, (correctedStdDev/meanDistance)*100)
		fmt.Printf("  📊 Specification requires: ±%.2f mm (1.0%% of reading)\n", onePercentThreshold)
		fmt.Printf("  🔴 %sEXCEEDS spec by %.1fx%s\n",
			ColorRed, correctedStdDev/onePercentThreshold, ColorReset)

		// Suggest next steps
		if varianceReduction < 40 {
			fmt.Printf("\n  %s💡 Recommendation:%s Try multi-factor analysis with %s--multi%s flag\n",
				ColorBold, ColorReset, ColorYellow, ColorReset)
			fmt.Printf("     Combining environmental factors may improve accuracy\n")
		} else {
			fmt.Printf("\n  %s⚠️  Recommendation:%s Sensor may have reached precision limits\n", ColorBold, ColorReset)
			fmt.Printf("     Consider hardware solutions (shielding, calibration, replacement)\n")
		}
	}

	fmt.Printf("\n  Total readings: %s%d hours%s\n\n", ColorCyan, len(readings), ColorReset)
}

// fetchMultiFactorData retrieves snow distance with ALL environmental factors
func fetchMultiFactorData(db *sql.DB, snowStation, factorStation string, hours int) []MultiFactorReading {
	query := `
		SELECT
			s.time,
			s.snowdistance,
			f.outtemp,
			f.outhumidity,
			f.windspeed,
			f.barometer
		FROM weather s
		INNER JOIN weather f
			ON f.stationname = $2
			AND f.time >= s.time - INTERVAL '2 minutes'
			AND f.time <= s.time + INTERVAL '2 minutes'
		WHERE s.stationname = $1
		  AND s.time >= NOW() - INTERVAL '1 hour' * $3
		  AND s.snowdistance IS NOT NULL
		  AND f.outtemp IS NOT NULL
		  AND f.outhumidity IS NOT NULL
		  AND f.windspeed IS NOT NULL
		  AND f.barometer IS NOT NULL
		ORDER BY s.time
	`

	rows, err := db.Query(query, snowStation, factorStation, hours)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError querying multi-factor data: %v%s\n", ColorRed, err, ColorReset)
		return nil
	}
	defer rows.Close()

	var readings []MultiFactorReading
	for rows.Next() {
		var r MultiFactorReading
		if err := rows.Scan(&r.Time, &r.SnowDistance, &r.Temperature, &r.Humidity, &r.Windspeed, &r.Barometer); err != nil {
			fmt.Fprintf(os.Stderr, "%sError scanning multi-factor row: %v%s\n", ColorRed, err, ColorReset)
			continue
		}
		readings = append(readings, r)
	}

	// Calculate baseline and drift
	if len(readings) > 0 {
		distances := make([]float64, len(readings))
		for i, r := range readings {
			distances[i] = r.SnowDistance
		}
		baseline := stat.Mean(distances, nil)

		for i := range readings {
			readings[i].SnowDrift = readings[i].SnowDistance - baseline
		}
	}

	return readings
}

// Multi-factor model result
type MultiFactorModel struct {
	FactorNames          []string
	Coefficients         []float64 // [intercept, coef1, coef2, ...]
	RSquared             float64
	VarianceReduction    float64 // Percentage reduction in std dev
	CorrectedStdDev      float64
	RawStdDev            float64
	RMSE                 float64
	SampleCount          int
}

// fitTwoFactorLinearModel fits: drift = c0 + c1*factor1 + c2*factor2
func fitTwoFactorLinearModel(readings []MultiFactorReading, factor1Name, factor2Name string) MultiFactorModel {
	n := len(readings)
	if n < 3 {
		return MultiFactorModel{}
	}

	// Extract factor values based on names
	getFactor := func(r MultiFactorReading, name string) float64 {
		switch name {
		case "temperature":
			return r.Temperature
		case "humidity":
			return r.Humidity
		case "windspeed":
			return r.Windspeed
		case "barometer":
			return r.Barometer
		default:
			return 0
		}
	}

	// Build design matrix X = [1, factor1, factor2]
	X := mat.NewDense(n, 3, nil)
	y := mat.NewVecDense(n, nil)
	drifts := make([]float64, n)
	factor1Vals := make([]float64, n)
	factor2Vals := make([]float64, n)

	for i, r := range readings {
		f1 := getFactor(r, factor1Name)
		f2 := getFactor(r, factor2Name)
		factor1Vals[i] = f1
		factor2Vals[i] = f2
		drifts[i] = r.SnowDrift

		X.Set(i, 0, 1.0)      // intercept
		X.Set(i, 1, f1)       // factor1
		X.Set(i, 2, f2)       // factor2
		y.SetVec(i, r.SnowDrift)
	}

	// Solve using QR decomposition: X * coeffs = y
	var qr mat.QR
	qr.Factorize(X)

	var coeffs mat.Dense
	err := coeffs.Solve(X, y)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%sError solving two-factor regression: %v%s\n", ColorRed, err, ColorReset)
		return MultiFactorModel{}
	}

	// Extract coefficients
	c0 := coeffs.At(0, 0) // intercept
	c1 := coeffs.At(1, 0) // factor1 coefficient
	c2 := coeffs.At(2, 0) // factor2 coefficient

	// Calculate predictions and metrics
	rawDistances := make([]float64, n)
	correctedDistances := make([]float64, n)
	var ssRes, ssTot float64
	meanDrift := stat.Mean(drifts, nil)

	for i, r := range readings {
		predicted := c0 + c1*factor1Vals[i] + c2*factor2Vals[i]
		residual := r.SnowDrift - predicted

		rawDistances[i] = r.SnowDistance
		correctedDistances[i] = r.SnowDistance - predicted

		ssRes += residual * residual
		ssTot += (r.SnowDrift - meanDrift) * (r.SnowDrift - meanDrift)
	}

	r2 := 1.0 - (ssRes / ssTot)
	if ssTot == 0 {
		r2 = 0
	}

	rmse := math.Sqrt(ssRes / float64(n))
	rawStdDev := stat.StdDev(rawDistances, nil)
	correctedStdDev := stat.StdDev(correctedDistances, nil)
	varianceReduction := ((rawStdDev - correctedStdDev) / rawStdDev) * 100

	return MultiFactorModel{
		FactorNames:       []string{factor1Name, factor2Name},
		Coefficients:      []float64{c0, c1, c2},
		RSquared:          r2,
		VarianceReduction: varianceReduction,
		CorrectedStdDev:   correctedStdDev,
		RawStdDev:         rawStdDev,
		RMSE:              rmse,
		SampleCount:       n,
	}
}

// testMultiFactorCombinations tests all two-factor combinations
func testMultiFactorCombinations(db *sql.DB, snowStation, factorStation string, hours int) {
	fmt.Printf("\n%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n", ColorMagenta, ColorReset)
	fmt.Printf("%s%sMULTI-FACTOR ANALYSIS (Two-Factor Combinations)%s\n", ColorBold, ColorMagenta, ColorReset)
	fmt.Printf("%s━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━%s\n\n", ColorMagenta, ColorReset)

	// Fetch data with all factors
	readings := fetchMultiFactorData(db, snowStation, factorStation, hours)

	if len(readings) < 10 {
		fmt.Printf("%s⚠ Warning:%s Not enough multi-factor data points (%d). Need at least 10.\n\n",
			ColorYellow, ColorReset, len(readings))
		return
	}

	fmt.Printf("Testing all two-factor combinations on %s%d data points%s\n\n",
		ColorGreen, len(readings), ColorReset)

	// Test all combinations
	factors := []string{"temperature", "humidity", "windspeed", "barometer"}
	var models []MultiFactorModel

	for i := 0; i < len(factors); i++ {
		for j := i + 1; j < len(factors); j++ {
			model := fitTwoFactorLinearModel(readings, factors[i], factors[j])
			if model.SampleCount > 0 {
				models = append(models, model)
			}
		}
	}

	// Sort by variance reduction (best first)
	sort.Slice(models, func(i, j int) bool {
		return models[i].VarianceReduction > models[j].VarianceReduction
	})

	// Display results table
	fmt.Printf("%-30s | %8s | %10s | %10s | %15s\n",
		"Factor Combination", "R²", "Var Red %", "Corr StdDev", "Status")
	fmt.Printf("-------------------------------+----------+------------+------------+-----------------\n")

	for _, m := range models {
		factorPair := fmt.Sprintf("%s + %s", capitalize(m.FactorNames[0]), capitalize(m.FactorNames[1]))

		// Color code variance reduction
		varRedColor := ColorRed
		status := "Poor"
		if m.VarianceReduction > 50 {
			varRedColor = ColorGreen
			status = "Excellent"
		} else if m.VarianceReduction > 25 {
			varRedColor = ColorYellow
			status = "Moderate"
		}

		r2Display := colorizeRSquared(m.RSquared)

		fmt.Printf("%-30s | %s | %s%9.1f%%%s | %10.2f mm | %s%-15s%s\n",
			factorPair,
			r2Display,
			varRedColor, m.VarianceReduction, ColorReset,
			m.CorrectedStdDev,
			varRedColor, status, ColorReset)
	}

	// Highlight the best combination
	if len(models) > 0 {
		best := models[0]
		fmt.Printf("\n%s🏆 BEST TWO-FACTOR COMBINATION:%s\n", ColorBold, ColorReset)
		fmt.Printf("  Factors: %s%s + %s%s\n",
			ColorCyan, capitalize(best.FactorNames[0]), capitalize(best.FactorNames[1]), ColorReset)
		fmt.Printf("  Equation: drift = %.4f + %.4f×%s + %.4f×%s\n",
			best.Coefficients[0], best.Coefficients[1], best.FactorNames[0],
			best.Coefficients[2], best.FactorNames[1])
		fmt.Printf("  R² = %s\n", colorizeRSquared(best.RSquared))
		fmt.Printf("  Variance reduction: %s%.1f%%%s\n", ColorGreen, best.VarianceReduction, ColorReset)
		fmt.Printf("  Corrected std dev: %s%.2f mm%s (%.3f inches)\n\n",
			ColorCyan, best.CorrectedStdDev, ColorReset, best.CorrectedStdDev/25.4)

		// Compare to single-factor best
		fmt.Printf("%sComparison to Single Factors:%s\n", ColorBold, ColorReset)
		fmt.Printf("  Raw data std dev:     %.2f mm\n", best.RawStdDev)
		fmt.Printf("  Best two-factor:      %.2f mm (%.1f%% reduction)\n",
			best.CorrectedStdDev, best.VarianceReduction)

		// 1% accuracy specification check for multi-factor
		// Use typical snow gauge installation height for spec calculation
		typicalHeight := 1800.0 // mm, typical sensor mounting height
		onePercentSpec := typicalHeight * 0.01

		fmt.Printf("\n%s📊 1%% Accuracy Specification:%s\n", ColorBold, ColorReset)
		fmt.Printf("  Specification requires:   ±%.2f mm (1%% of ~%.0f mm installation height)\n",
			onePercentSpec, typicalHeight)

		specColor := ColorRed
		specLabel := "FAILS"
		if best.CorrectedStdDev <= onePercentSpec {
			specColor = ColorGreen
			specLabel = "PASSES"
		}

		fmt.Printf("  Two-factor corrected:     %.2f mm %s%s%s\n",
			best.CorrectedStdDev, specColor, specLabel, ColorReset)

		if best.CorrectedStdDev > onePercentSpec {
			excessFactor := best.CorrectedStdDev / onePercentSpec
			fmt.Printf("  %s🔴 EXCEEDS spec by %.1fx%s\n", ColorRed, excessFactor, ColorReset)
		} else {
			margin := ((onePercentSpec - best.CorrectedStdDev) / onePercentSpec) * 100
			fmt.Printf("  %s✅ Meets spec with %.1f%% margin%s\n", ColorGreen, margin, ColorReset)
		}

		// Show improvement over single factors
		fmt.Printf("\n%s💡 Insight:%s ", ColorBold, ColorReset)
		if best.VarianceReduction > 50 {
			fmt.Printf("Combining factors provides %sexcellent%s variance reduction!\n", ColorGreen, ColorReset)
		} else if best.VarianceReduction > 35 {
			fmt.Printf("Combining factors provides %smoderate improvement%s.\n", ColorYellow, ColorReset)
		} else {
			fmt.Printf("%sCombining factors doesn't help much%s.\n", ColorRed, ColorReset)
			fmt.Printf("  This suggests the remaining variance is likely %ssensor noise%s or unmeasured factors.\n",
				ColorYellow, ColorReset)
		}
	}

	fmt.Println()
}
