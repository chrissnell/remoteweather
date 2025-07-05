# Live Data Simulator

The live-data-simulator pulls real weather data from https://suncrestweather.com/latest?station=CSI every 4 seconds and serves it to multiple weather stations with slight variations.

## Purpose

This utility enables testing with realistic live weather data by:
- Fetching real weather data from a live weather station
- Creating TCP servers for each configured weather station
- Applying slight skewing to make each station's data unique
- Serving Campbell Scientific formatted JSON data

## Usage

```bash
# Build the simulator
go build

# Run with default settings (config.db, 4-second interval, port 7100+)
./live-data-simulator

# Run with custom configuration
./live-data-simulator -config /path/to/config.db -interval 5s -base-port 8000
```

## Command Line Options

- `-config`: Path to configuration database (default: `config.db`)
- `-interval`: Interval between data fetches (default: `4s`)
- `-base-port`: Base port for station servers (default: `7100`)

## How It Works

1. Queries the configuration database for all enabled weather stations
2. Creates a TCP server for each station on sequential ports (base-port, base-port+1, etc.)
3. Fetches live weather data every 4 seconds from suncrestweather.com
4. Applies unique skewing factors to each station's data
5. Serves Campbell Scientific formatted JSON to connected clients

## Station Configuration

Configure your weather stations to connect to localhost with the assigned ports:
- First station: localhost:7100
- Second station: localhost:7101
- And so on...

Each station will receive slightly different data based on its unique skewing factor.

## Data Format

The simulator serves Campbell Scientific format JSON:

```json
{
  "batt_volt": 13.2,
  "airtemp_f": 72.5,
  "rh": 45.2,
  "baro": 30.15,
  "baro_temp_f": 75.1,
  "slr_w": 850.5,
  "slr_mj": 0.85,
  "rain_in": 0.0,
  "wind_s": 8.5,
  "wind_d": 180
}
``` 