# Snow Gauge Environmental Calibration Tool

Advanced statistical tool for calibrating ultrasonic snow depth sensors against environmental factors to minimize drift and maximize accuracy.

## Overview

Ultrasonic snow depth sensors can drift due to environmental factors (temperature, humidity, barometric pressure, wind). This tool uses **multiple regression analysis** to find the optimal environmental compensation equation that keeps readings as stable as possible.

## Key Features

✅ **Multi-Factor Analysis** - Tests all environmental factor combinations
✅ **Automatic Baseline Detection** - No manual baseline required
✅ **Variance Optimization** - Directly minimizes variability in corrected readings
✅ **1% Accuracy Spec Check** - Validates sensor performance against manufacturer specs
✅ **Color-Coded Output** - Easy-to-read results with visual indicators
✅ **Hourly Compensation Preview** - See corrected values applied to real data
✅ **Production-Ready Code Generation** - Copy-paste compensation functions

## Quick Start

### Basic Analysis (Single Factors)
```bash
./snow-calibrate --hours 72
```

Tests all environmental factors individually and shows which one explains sensor drift best.

### Advanced Analysis (Multi-Factor)
```bash
./snow-calibrate --multi --hours 168
```

Tests **all two-factor combinations** to find the best equation for stable readings.

## Usage

### Command-Line Options

```
-db-host string       Database host (default "localhost")
-db-port int          Database port (default 5432)
-db-user string       Database user (default "postgres")
-db-pass string       Database password
-db-name string       Database name (default "weather_v2_0_0")
-station string       Snow gauge station name (default "snow")
-factor-station string Station for environmental factors (default "CSI")
-factor string        Factor to test: all, temperature, humidity, windspeed, barometer (default "all")
-hours int            Hours of data to analyze (default 24)
-csv string           Optional CSV export path
-multi                Enable multi-factor (two-factor) analysis
```

### Recommended Analysis Periods

| Duration | Purpose | When to Use |
|----------|---------|-------------|
| 24 hours | Quick check | Initial testing, debugging |
| 72 hours | Standard analysis | Regular calibration, good correlation expected |
| 168 hours (1 week) | Comprehensive | Poor correlation, multi-factor analysis, production calibration |

⚠️ **CRITICAL**: Ensure **NO SNOW** is present during calibration period!

## What the Tool Shows

### 1. Single-Factor Comparison

Tests each environmental factor independently:
- Temperature
- Humidity
- Wind Speed
- Barometric Pressure

**Output**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
FACTOR COMPARISON (Best Model for Each)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Factor          | Best Model      |       R² | RMSE(mm) |        AIC
----------------+-----------------+----------+----------+------------
barometer ★     | Linear          |   0.3845 |     1.82 |     245.32
temperature     | Quadratic       |   0.2156 |     2.05 |     267.89
humidity        | Linear          |   0.1234 |     2.34 |     289.45
windspeed       | Constant        |   0.0456 |     2.51 |     298.12
```

The ★ indicates the best single factor by R² (correlation strength).

### 2. Metrics Explanation

Comprehensive guide to understanding:
- **R²** - Proportion of variance explained (higher = better correlation)
- **Adjusted R²** - R² adjusted for model complexity
- **RMSE** - Root Mean Squared Error (average prediction error in mm)
- **MAE** - Mean Absolute Error
- **AIC** - Akaike Information Criterion (model selection, lower = better)
- **BIC** - Bayesian Information Criterion (more conservative than AIC)

### 3. Best Model Details

Shows the winning equation with examples:
```
Best Model Details (Cubic)
=====================

Model equation:
  drift = -125.4321 + 0.0234 × B + -0.0012 × B² + 0.0001 × B³
  (B in inHg, drift in mm)

Quality Metrics:
  R² = 0.3845
  Adjusted R² = 0.3712
  RMSE = 1.82 mm (0.072 inches)
  MAE = 1.45 mm (0.057 inches)
  Sample size = 156

Barometer Impact Examples:
  At  29.5 inHg:   2.34 mm drift (0.092 inches)
  At  29.8 inHg:   1.12 mm drift (0.044 inches)
  At  30.0 inHg:  -0.23 mm drift (-0.009 inches)
  At  30.2 inHg:  -1.45 mm drift (-0.057 inches)
  At  30.5 inHg:  -3.12 mm drift (-0.123 inches)
```

### 4. Hourly Compensation Results

Shows your actual readings before and after compensation:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
HOURLY COMPENSATION RESULTS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Time (Hour)          |  Barometer | Raw Dist (mm) |   Drift (mm) | Corrected (mm)
---------------------+------------+--------------+--------------+--------------
2025-10-29 13:00     |  30.3 inHg |      1796.28 |         1.42 |      1794.86
2025-10-29 12:00     |  30.3 inHg |      1793.60 |         0.05 |      1793.56
2025-10-29 09:00     |  30.4 inHg |      1791.84 |        -1.52 |      1793.36
...

Summary:
  Average drift correction: 0.00 mm (0.000 inches)
  Drift range: -1.78 to 3.28 mm

Variance Analysis:
  Raw distance std dev:       2.24 mm (0.088 inches)
  Corrected distance std dev: 1.51 mm (0.059 inches)
  Variance reduction:         32.7% (Moderate)

Accuracy Specification Check (1%):
  Mean sensor reading:        1795.45 mm
  1% accuracy threshold:     ±17.95 mm (±0.707 inches)
  Raw data variability:       2.24 mm (spec requires ≤17.95 mm) PASSES
  Corrected data variability: 1.51 mm (spec requires ≤17.95 mm) PASSES

📊 Specification Compliance:
  ✅ Sensor MEETS 1% accuracy spec without compensation
```

### 5. Multi-Factor Analysis (--multi flag)

Tests **all two-factor combinations**:
```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
MULTI-FACTOR ANALYSIS (Two-Factor Combinations)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Testing all two-factor combinations on 156 data points

Factor Combination            |       R² |  Var Red % | Corr StdDev | Status
-------------------------------+----------+------------+------------+-----------------
Barometer + Temperature       |   0.4521 |      45.2% |    1.23 mm | Moderate
Temperature + Humidity        |   0.4103 |      41.0% |    1.32 mm | Moderate
Barometer + Humidity          |   0.3567 |      35.7% |    1.44 mm | Moderate
...

🏆 BEST TWO-FACTOR COMBINATION:
  Factors: Barometer + Temperature
  Equation: drift = 125.4321 + (-0.0234)×barometer + 0.0512×temperature
  R² = 0.4521
  Variance reduction: 45.2%
  Corrected std dev: 1.23 mm (0.048 inches)

📊 1% Accuracy Specification:
  Specification requires:   ±18.00 mm (1% of ~1800 mm installation height)
  Two-factor corrected:     1.23 mm PASSES
  ✅ Meets spec with 93.2% margin
```

### 6. Generated Go Code

Production-ready compensation function:
```go
// Barometer compensation function - Cubic model
// Calibrated on 156 samples, R² = 0.3845, RMSE = 1.82 mm
func compensateSnowDistanceForBarometer(rawDistance float32, barometer float32) float32 {
    // Cubic model: drift = c0 + c1*F + c2*F² + c3*F³
    const c0 = -125.432100
    const c1 = 0.023400
    const c2 = -0.001200
    const c3 = 0.000100
    f := float64(barometer)
    expectedDrift := float32(c0 + c1*f + c2*f*f + c3*f*f*f)
    compensated := rawDistance - expectedDrift
    return compensated
}

// Usage in your snow gauge code:
// compensatedDistance := compensateSnowDistanceForBarometer(snowdistance, barometer)
// snowDepth := baseDistance - compensatedDistance
```

## Understanding the Results

### Variance Reduction Percentage

This is the **key metric** - it shows how much more stable your corrected readings are:

| Variance Reduction | Interpretation | Action |
|-------------------|----------------|--------|
| **>50%** | Excellent! Environmental factors explain most variance | ✅ Implement compensation in production |
| **25-50%** | Moderate. Factors help but don't explain everything | 🟡 Consider if 1-2mm variance is acceptable for your use case |
| **<25%** | Poor. Factors don't explain the variance | 🔴 Likely at sensor physics limits or unmeasured factors |

### 1% Accuracy Specification

Most ultrasonic sensors are rated for **±1% accuracy**. For a sensor at 1800mm height:
- **Spec requires**: ±18mm variability
- **Typical performance**: 2-3mm variability (0.1-0.2%)
- **Excellent performance**: <1.5mm variability (<0.1%)

If your sensor shows **>18mm variability**, it's failing its specification and may need:
- Environmental compensation (what this tool provides)
- Hardware fixes (calibration, cleaning, repair)
- Replacement

### R² Interpretation

| R² Value | Correlation | Meaning |
|----------|-------------|---------|
| **>0.7** | Strong | Factor strongly affects sensor drift |
| **0.3-0.7** | Moderate | Factor partially explains drift |
| **<0.3** | Weak | Factor has little effect on drift |

## Integration Guide

### 1. Analyze Your Data
```bash
./snow-calibrate --multi --hours 168 > calibration_report.txt
```

### 2. Check Specification Compliance

Look for the "📊 Specification Compliance" section:
- ✅ **PASSES**: Sensor meets spec (good!)
- ❌ **FAILS**: Sensor needs compensation or hardware attention

### 3. Evaluate Variance Reduction

- **>50%**: Great! Use the generated code
- **25-50%**: Acceptable for most applications
- **<25%**: May need hardware solutions

### 4. Copy the Generated Code

Find the "Go Code Implementation" section and copy the function.

### 5. Integrate into Your Snow Gauge Code

Likely location: `internal/weatherstations/snowgauge/station.go`

```go
// In your reading processing function:
func (s *SnowGauge) processReading(raw Reading) ProcessedReading {
    // Get environmental factor (from weather station)
    barometer := s.getBarometer() // Your implementation

    // Apply compensation
    compensated := compensateSnowDistanceForBarometer(
        raw.snowdistance,
        barometer,
    )

    // Calculate snow depth
    snowDepth := s.baseDistance - compensated

    return ProcessedReading{
        SnowDepth: snowDepth,
        // ... other fields
    }
}
```

### 6. Test and Monitor

1. Deploy with compensation enabled
2. Monitor for 24-48 hours during clear weather
3. Verify drift is reduced
4. Test with actual snow event
5. Adjust if needed

## Troubleshooting

### "Not enough multi-factor data points"

**Cause**: Missing environmental data (temp, humidity, wind, barometer)

**Fix**:
- Ensure your weather station records all factors
- Check database for NULL values
- Increase `--hours` to get more matching data points

### Low Variance Reduction (<25%)

**Possible causes**:
1. **Sensor noise dominates** - You're at the physics limit (~1-2mm for ultrasonic)
2. **Unmeasured factors** - Direct sunlight, icing, vibration, electronics heating
3. **Sensor malfunction** - Needs calibration or replacement

**Try**:
- Multi-factor analysis: `--multi` flag
- Longer data period: `--hours 336` (2 weeks)
- Hardware inspection: Clean sensor, check mounting, verify power

### Sensor FAILS 1% Spec Even With Compensation

**Immediate actions**:
1. Try `--multi` to test two-factor combinations
2. Collect more data: `--hours 336`
3. Inspect hardware for issues

**Long-term solutions**:
- Better sensor shielding (sun, wind, temperature)
- Multiple sensor averaging (2-3 sensors → √n variance reduction)
- Upgrade to laser/radar sensor (±0.1-0.5mm typical)

### Generated Code References Wrong Factor

This was a bug in earlier versions. Current version (with FactorName tracking) generates correct code. If you see this:
1. Rebuild: `go build -o snow-calibrate cmd/snow-calibrate/main.go`
2. Re-run calibration
3. Check that "BEST FACTOR" matches code function name

## Advanced Usage

### Export Data for External Analysis
```bash
./snow-calibrate --hours 168 --csv raw_data.csv
```

Analyze in Python, R, Excel, etc. for:
- Three-factor models
- Non-linear relationships
- Time-series analysis
- Custom algorithms

### Test Specific Factor
```bash
./snow-calibrate --factor barometer --hours 72
```

### Different Stations
```bash
./snow-calibrate \
  --station snow_remote \
  --factor-station weather_main \
  --hours 168
```

## Performance Tips

### Optimal Data Collection

1. **24-72 hours**: Good for well-correlated sensors
2. **168+ hours**: Better for multi-factor or weak correlation
3. **Clear weather only**: No snow, rain, or fog during calibration

### Database Performance

- Index on `stationname, time` improves query speed
- Analyze during low-traffic periods for large datasets
- Use `--hours 24` for quick checks, `--hours 168` for production

## References

### Statistical Methods

- **Linear Regression**: Ordinary Least Squares (OLS)
- **Polynomial Regression**: Vandermonde matrix with QR decomposition
- **Multi-factor Regression**: Multiple linear regression via QR solve
- **Model Selection**: AIC/BIC for complexity penalty
- **Variance Analysis**: Standard deviation comparison

### Libraries Used

- `gonum.org/v1/gonum/stat` - Statistical functions
- `gonum.org/v1/gonum/mat` - Matrix operations
- PostgreSQL/TimescaleDB - Time-series database

## Support

For issues or questions:
1. Check this README
2. Review example output above
3. Examine `/scratch/sensor-variance-analysis.md` for detailed theory
4. Check `/scratch/accuracy-spec-check.md` for specification details

## Version History

**Current**: Multi-factor regression with 1% spec checking
- ✅ Multi-factor (two-factor) analysis
- ✅ Automatic 1% accuracy specification checking
- ✅ Variance reduction optimization
- ✅ Dynamic factor name tracking
- ✅ Color-coded output
- ✅ Comprehensive metrics explanation
- ✅ Hourly compensation preview

**Previous**: Single-factor temperature-only calibration
