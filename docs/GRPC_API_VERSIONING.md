# gRPC API Versioning

## Version 1.0

The gRPC API is now versioned. Version 1.0 includes all available weather data fields.

### What Changed
- **Service**: `Weather` → `WeatherV1`
- **Package**: `github.com/chrissnell/remoteweather/protocols/remoteweather` (package name is `v1`)
- **HTTP paths**: `/v1/weather/*`
- **Fields**: Expanded from 12 to 103 fields covering all sensor data

### Client Migration
```go
// Old
client := remoteweather.NewWeatherClient(conn)

// New
client := weather.NewWeatherV1Client(conn)
```

### API Endpoints
- `GET /v1/weather/latest` - Get latest reading
- `GET /v1/weather/span/{duration}` - Get time-span data
- `POST /v1/weather/live` - Live streaming

### Field Coverage
WeatherReading now includes:
- Basic environmental (temp, humidity, pressure, wind)
- Extended sensors (7 extra temp, 4 soil, 4 leaf sensors)
- Rain data (rate, incremental, daily, monthly, yearly)
- Solar data (watts, UV, radiation)
- System status (alarms, battery, forecast)
- Extensibility (10 extra float + 10 text fields)

### Future Versions
- **Minor versions** (v1.1, v1.2): Add optional fields, maintain compatibility
- **Major versions** (v2.0): Breaking changes, new service `WeatherV2`

Both versions can run simultaneously with different HTTP paths. 