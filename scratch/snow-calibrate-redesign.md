# Snow Calibrate Tool - Environmental Factor Correlation

## Problem
The original tool required a manual `--baseline` parameter (e.g., 1798mm), which you had to measure manually.

## Solution
The tool now **auto-detects** the baseline as the mean distance during the analysis period (empty pad reading) and finds the correlation between **any environmental factor** and sensor drift.

## Supported Environmental Factors
- **temperature** - Outdoor temperature (°F)
- **humidity** - Outdoor humidity (%)
- **windspeed** - Wind speed (mph)
- **barometer** - Barometric pressure (inHg)

## How It Works

### Simple Approach
```
1. Collect distance and environmental factor data (empty pad, 24+ hours)
2. Baseline = mean(distance)  // Current average empty pad reading
3. Drift = distance - baseline  // How much it varies from average
4. Fit models: drift = f(factor)  // Find correlation
5. Use best model for factor compensation
```

### What You Get

**Compensation Models:**
- **Constant**: No environmental effect (baseline model)
- **Linear**: `drift = c0 + c1×F`
- **Quadratic**: `drift = c0 + c1×F + c2×F²`
- **Cubic**: `drift = c0 + c1×F + c2×F² + c3×F³`

The tool compares all models and picks the best one based on AIC (Akaike Information Criterion).

### Pure Correlation Analysis

The tool performs simple correlation analysis:
1. Finds the current baseline (mean distance with empty pad)
2. Calculates how much the distance drifts from baseline
3. Finds the correlation: `drift = f(environmental_factor)`
4. Tells you which factor has the strongest correlation (highest R²)

## Usage

### Test All Factors (Recommended - Default)
```bash
# Automatically test all environmental factors and find the best one
./snow-calibrate --db-host 10.50.0.35 --db-user weather --db-pass "..." --hours 24
```

This will:
1. Test temperature, humidity, wind speed, and barometric pressure
2. Show results for each factor
3. Compare all factors side-by-side
4. Identify which factor has the strongest correlation (highest R²)
5. Generate compensation code for the best factor

### Test Single Factor
```bash
# Test only temperature
./snow-calibrate --factor temperature --hours 24

# Test only humidity
./snow-calibrate --factor humidity --hours 24

# Test only wind speed
./snow-calibrate --factor windspeed --hours 24

# Test only barometer
./snow-calibrate --factor barometer --hours 24
```

### Example Output (Testing All Factors)
```
Snow Gauge Environmental Factor Calibration
============================================

Configuration:
  Snow Station: snow
  Factor Station: CSI
  Testing Factors: [temperature humidity windspeed barometer]
  Analysis Period: 24 hours

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Testing Factor: temperature
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Collected 1440 data points

Model Comparison
================
[... temperature results ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Testing Factor: humidity
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Collected 1440 data points

[... humidity results ...]

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
FACTOR COMPARISON (Best Model for Each)
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Factor          | Best Model      |       R² | RMSE(mm) |        AIC
----------------+-----------------+----------+----------+------------
temperature     | Linear          |   0.8234 |     1.25 |     245.32
humidity        | Quadratic       |   0.3421 |     2.45 |     312.89
windspeed       | Linear          |   0.1234 |     3.12 |     356.21
barometer       | Constant        |   0.0523 |     3.45 |     378.45

Interpretation:
  R² (coefficient of determination):
    - Closer to 1.0 = stronger correlation
    - > 0.7 = strong correlation
    - 0.3-0.7 = moderate correlation
    - < 0.3 = weak correlation
  The factor with the highest R² has the strongest correlation with sensor drift.

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
BEST FACTOR: temperature
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

[... detailed results and Go code for temperature ...]
```

### Interpreting Results

**Output includes:**
- Auto-detected baseline distance (mm and inches)
- Temperature range analyzed
- Drift range observed
- Best model by AIC with quality metrics (R², RMSE)
- Go code for compensation function

**Example output:**
```
Baseline Determination:
  Mean distance (empty pad): 1798.34 mm (70.800 inches)
  Temperature range: 25.0°F to 45.0°F
  Drift range: -3.50 mm to 4.20 mm

Model Comparison
================
Model           |       R² |  Adj R² |  RMSE(mm) |        AIC |        BIC
----------------+----------+----------+----------+------------+------------
Linear          |   0.8234 |   0.8198 |     1.25 |     245.32 |     253.11 ← BEST (AIC)
Quadratic       |   0.8456 |   0.8401 |     1.18 |     248.45 |     259.67
Constant        |   0.0000 |   0.0000 |     2.95 |     312.89 |     318.23
Cubic           |   0.8512 |   0.8430 |     1.17 |     251.23 |     265.90
```

## Benefits

1. **Test Multiple Factors**
   - Temperature, humidity, wind speed, barometric pressure
   - Find which environmental factor has the strongest correlation
   - Run multiple tests to compare R² values

2. **Simple & Direct**
   - Just correlation between two values: factor at time t, distance at time t
   - No complex reference concepts

3. **Auto-Detected Baseline**
   - Mean of current empty pad readings
   - No manual measurement needed

4. **Environmental Compensation**
   - Find how much sensor drifts with each factor
   - Apply correction based on current environmental conditions

5. **Best Model Selection**
   - Automatically compares constant/linear/quadratic/cubic
   - Uses AIC to balance fit quality vs. complexity

## Workflow: Finding the Primary Factor

**Simple approach (recommended):**
```bash
# Test all factors in one run
./snow-calibrate --db-host 10.50.0.35 --db-user weather --db-pass "..." --hours 24
```

The tool will automatically:
1. Test all environmental factors
2. Compare R² values
3. Identify the factor with strongest correlation
4. Generate compensation code for that factor

## Implemented Features

**✅ Completed in this version:**
1. **Dynamic Factor Names**: Generated Go code now uses the correct factor (temperature, humidity, windspeed, barometer) instead of hardcoded "temperature"
2. **Metrics Explanation**: Comprehensive explanation of R², Adjusted R², RMSE, MAE, AIC, and BIC with practical interpretation
3. **Colorized Output**: ANSI color codes for better readability:
   - Green for strong correlations (R² > 0.7)
   - Yellow for moderate correlations (R² 0.3-0.7)
   - Red for weak correlations (R² < 0.3)
   - Cyan headers and highlighted best factors
   - Color-coded warnings and recommendations
4. **Hourly Compensation Table**: After determining the best factor and model, the tool now:
   - Fetches hourly averaged data from the `weather_1h` table
   - Applies the compensation function to each hourly reading
   - Displays a table showing:
     - Time (hour bucket)
     - Environmental factor value (e.g., temperature in °F)
     - Raw snow distance (mm)
     - Calculated drift (mm) - color coded by magnitude
     - Corrected snow distance (mm)
   - Shows summary statistics: average drift, drift range, total readings
   - Limits display to first 24 hours for readability

### Example Hourly Compensation Table Output

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
HOURLY COMPENSATION RESULTS
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

Applying Linear compensation model to hourly averages from weather_1h

Time (Hour)          | Barometer  | Raw Dist (mm) | Drift (mm)   | Corrected (mm)
---------------------+------------+--------------+--------------+--------------
2025-10-29 16:00     |   30.2 inHg |       245.32 |        -2.15 |       247.47
2025-10-29 15:00     |   30.1 inHg |       246.18 |        -1.89 |       248.07
2025-10-29 14:00     |   30.0 inHg |       247.05 |        -1.63 |       248.68
...

Summary:
  Average drift correction: -1.92 mm (-0.076 inches)
  Drift range: -3.45 to -0.58 mm
  Total readings: 72 hours
```

## Future: Multi-Factor Compensation

The tool is designed to eventually support compensating for multiple factors simultaneously. For example:

```go
// Future: Combined compensation
drift = c0 + c1×temperature + c2×humidity
```

The current single-factor analysis helps you understand which factors are most important, which will inform future multi-factor models.
