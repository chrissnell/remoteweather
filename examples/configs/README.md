# Configuration Examples

This directory contains example configuration files for RemoteWeather that demonstrate different deployment scenarios.

## Available Examples

### Basic Setup
- **`weather-station-basic.yaml`** - Minimal configuration with a single Davis weather station and REST web interface
- Perfect for getting started or simple home weather station setups

### Full-Featured Setup  
- **`weather-station-full.yaml`** - Comprehensive configuration showing all available features
- Includes multiple devices, storage backends, and third-party integrations
- Good reference for production deployments

## Using These Examples

### 1. Copy and Customize
```bash
# Copy an example to your config location
cp examples/configs/weather-station-basic.yaml config.yaml

# Edit the configuration for your environment
nano config.yaml
```

### 2. YAML Configuration (Default)
```bash
# Run with YAML configuration
./remoteweather -config config.yaml
```

### 3. SQLite Configuration
```bash
# Convert YAML to SQLite database
./config-convert -yaml config.yaml -sqlite config.db

# Run with SQLite configuration
./remoteweather -config-backend sqlite -config config.db
```

## Configuration Sections

### Devices
Configure your weather stations and sensors:
- **Davis**: Network-connected Davis weather stations
- **Campbell**: Campbell Scientific dataloggers via serial
- **SnowGauge**: Ultrasonic snow depth sensors

### Storage
Choose your data storage backends:
- **TimescaleDB**: Recommended for production (PostgreSQL + time-series)

- **gRPC**: Real-time streaming for client applications
- **APRS**: Automatic Packet Reporting System for ham radio

### Controllers
Add external integrations:
- **REST**: Web interface for viewing weather data
- **PWS Weather**: Upload to PWS Weather network
- **Weather Underground**: Upload to Weather Underground
- **Aeris Weather**: Weather forecasting integration

## Customization Tips

### Device Configuration
1. Update hostnames and ports for your actual devices
2. Set correct latitude/longitude for solar calculations
3. Adjust wind direction correction for local magnetic declination
4. Configure snow gauge base distance when no snow is present

### Storage Configuration
1. Update database connection strings with your credentials
2. Consider using TimescaleDB for production deployments
3. Enable gRPC for real-time client applications
4. Configure APRS with your callsign and location

### Controller Configuration
1. Obtain API keys for third-party services
2. Set upload intervals based on your data needs
3. Configure SSL certificates for secure web access
4. Customize weather site branding and content

## Security Considerations

- **Never commit real API keys or passwords to version control**
- Use environment variables or secure configuration management
- Enable SSL/TLS for web interfaces in production
- Restrict database access to authorized networks only
- Use strong passwords and rotate credentials regularly

## Testing Your Configuration

Use the built-in validation tools:

```bash
# Test YAML syntax and structure
./remoteweather -config config.yaml -debug

# Convert and validate SQLite equivalent
./config-convert -yaml config.yaml -sqlite test.db -dry-run

# Compare YAML and SQLite configurations
./config-test -yaml config.yaml -sqlite test.db
```

## Need Help?

- Check the main README.md for general setup instructions
- Review SQLITE_CONFIG_BACKEND.md for SQLite-specific features
- Look at the source code for detailed configuration options
- Join the community forums for support and tips 