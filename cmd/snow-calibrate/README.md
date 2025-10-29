# Snow Gauge Temperature Calibration Tool

This tool analyzes the correlation between temperature and snow gauge sensor drift to generate a temperature compensation function.

## Problem

Ultrasonic snow depth sensors can drift due to environmental factors, primarily temperature changes. This creates false snow accumulation readings when no snow is present.

## Solution

This tool:
1. Collects paired snow distance and temperature readings from your database
2. Performs linear regression to find the relationship between temperature and sensor drift
3. Generates a compensation function you can use in your code
4. Provides statistical quality metrics (R², MAE) to assess the correlation

## Usage

### Basic Usage

```bash
go run cmd/snow-calibrate/main.go \
  -db-host localhost \
  -db-port 5432 \
  -db-user postgres \
  -db-pass your_password \
  -db-name weather_v2_0_0 \
  -station snow \
  -baseline 1798 \
  -hours 48
```

### Parameters

- `-db-host`: Database host (default: localhost)
- `-db-port`: Database port (default: 5432)
- `-db-user`: Database user (default: postgres)
- `-db-pass`: Database password
- `-db-name`: Database name (default: weather_v2_0_0)
- `-station`: Snow gauge station name (default: snow)
- `-temp-station`: Temperature station name (auto-detected if not specified)
- `-baseline`: Baseline snow distance in mm when no snow present (default: 1798)
- `-hours`: Hours of historical data to analyze (default: 24)
- `-csv`: Optional CSV output file path for data export

### Recommended Analysis Period

- **Minimum**: 24 hours to capture daily temperature cycle
- **Optimal**: 48-72 hours for better statistical significance
- **Maximum**: 168 hours (1 week) to include weekly patterns

Ensure no actual snow is present during the calibration period!

### Example Output

```
Snow Gauge Temperature Calibration Tool
========================================

Configuration:
  Snow Station: snow
  Temp Station: davis
  Baseline Distance: 1798.00 mm
  Analysis Period: 48 hours

Collected 576 data points

Analysis Results
================

Temperature Correlation:
  Drift = 0.123456 × Temperature + -2.345678
  (Drift in mm, Temperature in °F)

Model Quality:
  R² (coefficient of determination): 0.8542
  → Strong correlation
  Mean Absolute Error: 0.85 mm
  Sample Count: 576

Data Ranges:
  Temperature: 28.4°F to 56.2°F (27.8°F range)
  Drift: -2.45 mm to 4.32 mm (6.77 mm range)

Temperature Impact Examples:
  At  20°F: -0.35 mm drift (-0.014 inches)
  At  30°F: 1.88 mm drift (0.074 inches)
  At  40°F: 4.12 mm drift (0.162 inches)
  At  50°F: 6.35 mm drift (0.250 inches)
  At  60°F: 8.59 mm drift (0.338 inches)

Go Code for Temperature Compensation
=====================================

// Add this function to your snow gauge processing code:

func compensateSnowDistanceForTemperature(rawDistance float32, temperature float32) float32 {
    // Temperature compensation based on calibration
    // Calibrated on 576 samples with R² = 0.8542
    const slope = 0.123456  // mm per °F
    const intercept = -2.345678  // mm baseline offset

    // Calculate expected drift due to temperature
    expectedDrift := float32(slope*float64(temperature) + intercept)

    // Compensate the reading
    compensated := rawDistance - expectedDrift

    return compensated
}
```

## Integration

After running the calibration:

1. Review the R² value:
   - **> 0.8**: Strong correlation, safe to use compensation
   - **0.5-0.8**: Moderate correlation, may help but test thoroughly
   - **< 0.5**: Weak correlation, temperature isn't the main factor

2. If R² is acceptable, copy the generated `compensateSnowDistanceForTemperature` function

3. Apply it in your snow gauge reading processing (probably in `internal/weatherstations/snowgauge/station.go`)

4. Test with real snow events to ensure compensation doesn't suppress actual snowfall

## CSV Export

Use the `-csv` flag to export raw data for further analysis:

```bash
go run cmd/snow-calibrate/main.go \
  -baseline 1798 \
  -hours 48 \
  -csv calibration_data.csv
```

The CSV contains:
- `Time`: Reading timestamp
- `Temperature_F`: Temperature in Fahrenheit
- `SnowDistance_mm`: Raw sensor distance
- `Drift_mm`: Deviation from baseline

You can analyze this in Excel, Python, R, etc. to explore more complex models (polynomial, multi-factor, etc.)

## Troubleshooting

### "Not enough data points"

Ensure:
- Your snow gauge has been running for the specified hours
- Temperature data is available from the weather station
- Both stations are recording to the database

### "Weak correlation" (R² < 0.5)

Temperature may not be the primary drift factor. Consider:
- **Humidity/Condensation**: Moisture on sensor face
- **Air Pressure**: Affects ultrasonic wave propagation
- **Electronics Temperature**: Internal sensor heating
- **Mounting Vibration**: Wind or structural movement
- **Multi-path Reflections**: Objects near sensor

Try collecting data over multiple days with varied conditions and look for other patterns.

### Negative Compensation at Low Temps

This is normal if your sensor reads longer distances (false snow) at low temperatures. The compensation will subtract this drift.

## Next Steps

1. Run calibration during clear, snow-free period
2. Apply compensation function to your code
3. Monitor for 24-48 hours during clear weather
4. Verify drift is reduced
5. Test with actual snow event to ensure real snow is still detected
