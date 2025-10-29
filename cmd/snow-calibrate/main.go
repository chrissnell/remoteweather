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

// SnowTempReading represents a correlated snow distance and temperature reading
type SnowTempReading struct {
	Time         time.Time
	SnowDistance float64
	Temperature  float64
	SnowDrift    float64 // Deviation from baseline (1798mm)
}

// ModelType represents different compensation models
type ModelType string

const (
	ModelConstant  ModelType = "constant"
	ModelLinear    ModelType = "linear"
	ModelQuadratic ModelType = "quadratic"
	ModelCubic     ModelType = "cubic"
)

// CalibrationResult contains the analysis results for a specific model
type CalibrationResult struct {
	ModelType            ModelType
	ModelName            string
	BaselineDistance     float64
	Coefficients         []float64 // Model coefficients [c0, c1, c2, ...] where drift = c0 + c1*T + c2*T² + ...
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
		dbHost      = flag.String("db-host", "localhost", "Database host")
		dbPort      = flag.Int("db-port", 5432, "Database port")
		dbUser      = flag.String("db-user", "postgres", "Database user")
		dbPass      = flag.String("db-pass", "", "Database password")
		dbName      = flag.String("db-name", "weather_v2_0_0", "Database name")
		station     = flag.String("station", "snow", "Snow gauge station name")
		tempStation = flag.String("temp-station", "CSI", "Temperature station name")
		baseline    = flag.Float64("baseline", 1798.0, "Baseline snow distance in mm (no snow)")
		hours       = flag.Int("hours", 24, "Number of hours of data to analyze")
		csvOutput   = flag.String("csv", "", "Optional CSV output file path")
	)
	flag.Parse()

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

	fmt.Printf("Snow Gauge Temperature Compensation Calibration\n")
	fmt.Printf("===============================================\n\n")
	fmt.Printf("Configuration:\n")
	fmt.Printf("  Snow Station: %s\n", *station)
	fmt.Printf("  Temp Station: %s\n", *tempStation)
	fmt.Printf("  Baseline Distance: %.2f mm\n", *baseline)
	fmt.Printf("  Analysis Period: %d hours\n\n", *hours)

	// Fetch correlated data
	readings := fetchCorrelatedData(db, *station, *tempStation, *baseline, *hours)

	if len(readings) < 10 {
		fmt.Fprintf(os.Stderr, "Error: Not enough data points (%d). Need at least 10.\n", len(readings))
		os.Exit(1)
	}

	fmt.Printf("Collected %d data points\n\n", len(readings))

	// Test all models
	results := testAllModels(readings, *baseline)

	// Display comparison
	displayComparison(results)

	// Display best model details
	displayBestModelDetails(results.BestByAIC)

	// Generate code for best model
	generateCompensationCode(results.BestByAIC)

	// Optionally export to CSV
	if *csvOutput != "" {
		if err := exportCSV(*csvOutput, readings, results.BestByAIC); err != nil {
			fmt.Fprintf(os.Stderr, "Error writing CSV: %v\n", err)
		} else {
			fmt.Printf("\nData exported to: %s\n", *csvOutput)
		}
	}
}

func fetchCorrelatedData(db *sql.DB, snowStation, tempStation string, baseline float64, hours int) []SnowTempReading {
	query := `
		SELECT
			s.time,
			s.snowdistance,
			t.temperature
		FROM weather s
		INNER JOIN weather t
			ON t.stationname = $2
			AND t.time >= s.time - INTERVAL '2 minutes'
			AND t.time <= s.time + INTERVAL '2 minutes'
		WHERE s.stationname = $1
		  AND s.time >= NOW() - INTERVAL '1 hour' * $3
		  AND s.snowdistance IS NOT NULL
		  AND t.temperature IS NOT NULL
		ORDER BY s.time
	`

	rows, err := db.Query(query, snowStation, tempStation, hours)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error querying data: %v\n", err)
		os.Exit(1)
	}
	defer rows.Close()

	var readings []SnowTempReading
	for rows.Next() {
		var r SnowTempReading
		if err := rows.Scan(&r.Time, &r.SnowDistance, &r.Temperature); err != nil {
			fmt.Fprintf(os.Stderr, "Error scanning row: %v\n", err)
			continue
		}
		r.SnowDrift = r.SnowDistance - baseline
		readings = append(readings, r)
	}

	return readings
}

func testAllModels(readings []SnowTempReading, baseline float64) ComparisonResults {
	models := []CalibrationResult{
		fitConstantModel(readings, baseline),
		fitLinearModel(readings, baseline),
		fitPolynomialModel(readings, baseline, 2), // Quadratic
		fitPolynomialModel(readings, baseline, 3), // Cubic
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

	return comparison
}

func fitConstantModel(readings []SnowTempReading, baseline float64) CalibrationResult {
	n := len(readings)

	// Extract drift values
	drifts := make([]float64, n)
	for i, r := range readings {
		drifts[i] = r.SnowDrift
	}

	// Constant model: drift = c0 (mean drift)
	meanDrift := stat.Mean(drifts, nil)

	result := CalibrationResult{
		ModelType:        ModelConstant,
		ModelName:        "Constant Offset",
		BaselineDistance: baseline,
		Coefficients:     []float64{meanDrift},
		SampleCount:      n,
	}

	// Calculate errors
	result.RSquared = 0.0 // Constant model explains no variance
	result.AdjustedRSquared = 0.0
	result.MeanAbsoluteError = calculateMAE(nil, drifts, func(t float64) float64 { return meanDrift })
	result.RootMeanSquaredError = calculateRMSE(nil, drifts, func(t float64) float64 { return meanDrift })

	// Calculate AIC and BIC
	k := 1.0 // number of parameters
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	// Calculate ranges
	temps := make([]float64, n)
	for i, r := range readings {
		temps[i] = r.Temperature
	}
	minTemp, maxTemp := minMax(temps)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minTemp, maxTemp}
	result.DriftRange = [2]float64{minDrift, maxDrift}

	return result
}

func fitLinearModel(readings []SnowTempReading, baseline float64) CalibrationResult {
	n := len(readings)

	// Extract temperature and drift values
	temps := make([]float64, n)
	drifts := make([]float64, n)
	for i, r := range readings {
		temps[i] = r.Temperature
		drifts[i] = r.SnowDrift
	}

	// Linear regression: drift = c0 + c1*T
	slope, intercept := stat.LinearRegression(temps, drifts, nil, false)

	result := CalibrationResult{
		ModelType:        ModelLinear,
		ModelName:        "Linear",
		BaselineDistance: baseline,
		Coefficients:     []float64{intercept, slope},
		SampleCount:      n,
	}

	predictFunc := func(t float64) float64 {
		return intercept + slope*t
	}

	// Calculate errors and metrics
	result.RSquared = calculateRSquared(temps, drifts, predictFunc)
	result.AdjustedRSquared = calculateAdjustedRSquared(result.RSquared, float64(n), 2.0)
	result.MeanAbsoluteError = calculateMAE(temps, drifts, predictFunc)
	result.RootMeanSquaredError = calculateRMSE(temps, drifts, predictFunc)

	k := 2.0 // intercept + slope
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	minTemp, maxTemp := minMax(temps)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minTemp, maxTemp}
	result.DriftRange = [2]float64{minDrift, maxDrift}

	return result
}

func fitPolynomialModel(readings []SnowTempReading, baseline float64, degree int) CalibrationResult {
	n := len(readings)

	// Extract temperature and drift values
	temps := make([]float64, n)
	drifts := make([]float64, n)
	for i, r := range readings {
		temps[i] = r.Temperature
		drifts[i] = r.SnowDrift
	}

	// Build Vandermonde matrix for polynomial regression
	X := mat.NewDense(n, degree+1, nil)
	for i := 0; i < n; i++ {
		for j := 0; j <= degree; j++ {
			X.Set(i, j, math.Pow(temps[i], float64(j)))
		}
	}

	y := mat.NewVecDense(n, drifts)

	// Solve using QR decomposition
	var qr mat.QR
	qr.Factorize(X)

	coeffs := mat.NewVecDense(degree+1, nil)
	err := qr.SolveTo(coeffs, false, y)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error solving polynomial regression: %v\n", err)
		return CalibrationResult{}
	}

	// Extract coefficients
	coeff := make([]float64, degree+1)
	for i := 0; i <= degree; i++ {
		coeff[i] = coeffs.AtVec(i)
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
		BaselineDistance: baseline,
		Coefficients:     coeff,
		SampleCount:      n,
	}

	predictFunc := func(t float64) float64 {
		pred := 0.0
		for i, c := range coeff {
			pred += c * math.Pow(t, float64(i))
		}
		return pred
	}

	// Calculate errors and metrics
	result.RSquared = calculateRSquared(temps, drifts, predictFunc)
	result.AdjustedRSquared = calculateAdjustedRSquared(result.RSquared, float64(n), float64(degree+1))
	result.MeanAbsoluteError = calculateMAE(temps, drifts, predictFunc)
	result.RootMeanSquaredError = calculateRMSE(temps, drifts, predictFunc)

	k := float64(degree + 1)
	result.AIC = calculateAIC(float64(n), result.RootMeanSquaredError, k)
	result.BIC = calculateBIC(float64(n), result.RootMeanSquaredError, k)

	minTemp, maxTemp := minMax(temps)
	minDrift, maxDrift := minMax(drifts)
	result.TemperatureRange = [2]float64{minTemp, maxTemp}
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
	fmt.Printf("Model Comparison\n")
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
		if m.ModelType == results.BestByAIC.ModelType {
			marker = " ← BEST (AIC)"
		}
		fmt.Printf("%-15s | %8.4f | %8.4f | %8.2f | %10.2f | %10.2f%s\n",
			m.ModelName, m.RSquared, m.AdjustedRSquared, m.RootMeanSquaredError, m.AIC, m.BIC, marker)
	}

	fmt.Printf("\nRecommendation:\n")
	fmt.Printf("  Best model by AIC: %s\n", results.BestByAIC.ModelName)
	if results.BestByAIC.ModelType != results.BestByBIC.ModelType {
		fmt.Printf("  Best model by BIC: %s (more conservative, penalizes complexity)\n", results.BestByBIC.ModelName)
	}

	if results.BestByAIC.RSquared < 0.3 {
		fmt.Printf("\n  ⚠ WARNING: Low R² (%.4f) - temperature may not be primary drift factor!\n", results.BestByAIC.RSquared)
		fmt.Printf("  Consider other factors: humidity, pressure, sensor heating, vibration\n")
	} else if results.BestByAIC.RSquared < 0.7 {
		fmt.Printf("\n  ℹ Moderate correlation (R²=%.4f) - useful but may not capture all drift\n", results.BestByAIC.RSquared)
	} else {
		fmt.Printf("\n  ✓ Strong correlation (R²=%.4f) - temperature is primary drift factor\n", results.BestByAIC.RSquared)
	}
	fmt.Println()
}

func displayBestModelDetails(model CalibrationResult) {
	fmt.Printf("Best Model Details (%s)\n", model.ModelName)
	fmt.Printf("=====================\n\n")

	fmt.Printf("Model equation:\n  ")
	switch model.ModelType {
	case ModelConstant:
		fmt.Printf("drift = %.4f mm\n", model.Coefficients[0])
	case ModelLinear:
		fmt.Printf("drift = %.6f + %.6f × T\n", model.Coefficients[0], model.Coefficients[1])
	case ModelQuadratic:
		fmt.Printf("drift = %.6f + %.6f × T + %.6f × T²\n",
			model.Coefficients[0], model.Coefficients[1], model.Coefficients[2])
	case ModelCubic:
		fmt.Printf("drift = %.6f + %.6f × T + %.6f × T² + %.6f × T³\n",
			model.Coefficients[0], model.Coefficients[1], model.Coefficients[2], model.Coefficients[3])
	}
	fmt.Printf("  (T in °F, drift in mm)\n\n")

	fmt.Printf("Quality Metrics:\n")
	fmt.Printf("  R² = %.4f\n", model.RSquared)
	fmt.Printf("  Adjusted R² = %.4f\n", model.AdjustedRSquared)
	fmt.Printf("  RMSE = %.2f mm (%.3f inches)\n", model.RootMeanSquaredError, model.RootMeanSquaredError/25.4)
	fmt.Printf("  MAE = %.2f mm (%.3f inches)\n", model.MeanAbsoluteError, model.MeanAbsoluteError/25.4)
	fmt.Printf("  Sample size = %d\n\n", model.SampleCount)

	fmt.Printf("Temperature Impact Examples:\n")
	for _, temp := range []float64{20, 30, 40, 50, 60} {
		drift := evaluateModel(model, temp)
		fmt.Printf("  At %3.0f°F: %6.2f mm drift (%6.3f inches)\n", temp, drift, drift/25.4)
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

func generateCompensationCode(model CalibrationResult) {
	fmt.Printf("Go Code Implementation\n")
	fmt.Printf("======================\n\n")

	fmt.Printf("// Temperature compensation function - %s model\n", model.ModelName)
	fmt.Printf("// Calibrated on %d samples, R² = %.4f, RMSE = %.2f mm\n",
		model.SampleCount, model.RSquared, model.RootMeanSquaredError)
	fmt.Printf("func compensateSnowDistanceForTemperature(rawDistance float32, temperature float32) float32 {\n")

	switch model.ModelType {
	case ModelConstant:
		fmt.Printf("    // Constant offset model\n")
		fmt.Printf("    const offset = %.6f  // mm\n", model.Coefficients[0])
		fmt.Printf("    compensated := rawDistance - float32(offset)\n")

	case ModelLinear:
		fmt.Printf("    // Linear model: drift = c0 + c1*T\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    expectedDrift := float32(c0 + c1*float64(temperature))\n")
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")

	case ModelQuadratic:
		fmt.Printf("    // Quadratic model: drift = c0 + c1*T + c2*T²\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    const c2 = %.6f\n", model.Coefficients[2])
		fmt.Printf("    t := float64(temperature)\n")
		fmt.Printf("    expectedDrift := float32(c0 + c1*t + c2*t*t)\n")
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")

	case ModelCubic:
		fmt.Printf("    // Cubic model: drift = c0 + c1*T + c2*T² + c3*T³\n")
		fmt.Printf("    const c0 = %.6f\n", model.Coefficients[0])
		fmt.Printf("    const c1 = %.6f\n", model.Coefficients[1])
		fmt.Printf("    const c2 = %.6f\n", model.Coefficients[2])
		fmt.Printf("    const c3 = %.6f\n", model.Coefficients[3])
		fmt.Printf("    t := float64(temperature)\n")
		fmt.Printf("    expectedDrift := float32(c0 + c1*t + c2*t*t + c3*t*t*t)\n")
		fmt.Printf("    compensated := rawDistance - expectedDrift\n")
	}

	fmt.Printf("    return compensated\n")
	fmt.Printf("}\n\n")

	fmt.Printf("// Usage in your snow gauge code:\n")
	fmt.Printf("// compensatedDistance := compensateSnowDistanceForTemperature(snowdistance, temperature)\n")
	fmt.Printf("// snowDepth := baseDistance - compensatedDistance\n")
}

func exportCSV(filename string, readings []SnowTempReading, model CalibrationResult) error {
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write header
	header := []string{"Time", "Temperature_F", "SnowDistance_mm", "Drift_mm", "Predicted_Drift_mm", "Residual_mm"}
	if err := writer.Write(header); err != nil {
		return err
	}

	// Write data
	for _, r := range readings {
		predicted := evaluateModel(model, r.Temperature)
		residual := r.SnowDrift - predicted

		record := []string{
			r.Time.Format(time.RFC3339),
			fmt.Sprintf("%.2f", r.Temperature),
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
