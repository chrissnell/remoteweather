# RemoteWeather

**RemoteWeather** is a professional weather monitoring system that collects data from your weather station, stores it for historical analysis, and shares it through multiple channels including a beautiful live website, weather services, and amateur radio networks.

## What RemoteWeather Does

* **Live Weather Website** - Creates a professional weather website with real-time updates, interactive charts, and mobile-friendly design (see [example](https://suncrestweather.com))
* **Historical Data Storage** - Stores all weather readings in TimescaleDB for long-term analysis, trends, and graphing
* **Weather Service Integration** - Automatically uploads your data to popular services like Weather Underground, PWS Weather, and Aeris Weather
* **Amateur Radio Support** - Transmits weather data via APRS/CWOP for ham radio operators and citizen weather observers
* **Multiple Access Methods** - View your data via web browser, mobile device, or custom applications using gRPC streaming

## System Requirements

### Hardware Requirements
RemoteWeather supports the following weather stations:
* **Campbell Scientific** stations with dataloggers (CR series)
* **Davis Instruments** VantagePro, VantagePro2, and Vue
* **AirGradient** air quality monitors
* **Ambient Weather** customized stations
* **Snow gauge** monitoring systems
* Generic stations via gRPC receiver

### Connection Options
* **Serial/USB** - Direct connection using WeatherLink or compatible cables
* **Network** - Ethernet or Wi-Fi connection for network-enabled stations
* **Radio Frequency** - 900MHz RF receivers (Campbell RF407, Davis Wireless Envoy)
* **TCP/IP** - For stations with network interfaces

### Software Requirements
* **Operating System**: Linux (Ubuntu, Debian, CentOS, or similar)
* **Database**: TimescaleDB (PostgreSQL extension) for weather data storage
* **Configuration**: SQLite database for system configuration
* **Docker** (optional but recommended): For simplified installation

## Getting Started

### Step 1: Initial Setup

1. **Download RemoteWeather**
   ```bash
   wget https://github.com/chrissnell/remoteweather/releases/latest/download/remoteweather-linux-amd64
   chmod +x remoteweather-linux-amd64
   ```

2. **First Run - Bootstrap Configuration**
   
   When you run RemoteWeather for the first time, it automatically creates a configuration database:
   ```bash
   ./remoteweather-linux-amd64
   ```
   
   The system will:
   - Create a SQLite configuration database at `~/.config/remoteweather/config.db`
   - Generate a secure access token for the management interface
   - Start the management API on `http://localhost:8081`
   
   **Important**: Save the access token displayed during bootstrap - you'll need it to configure the system!

### Step 2: Configure Your System

1. **Access the Management Interface**
   
   Open your web browser and go to: `http://localhost:8081`
   
   Enter the access token from the bootstrap process when prompted.

2. **Set Up Your Weather Station**
   
   In the management interface:
   - Click "Weather Stations" → "Add Station"
   - Select your station type (Davis, Campbell Scientific, etc.)
   - Enter connection details (serial port, IP address, etc.)
   - Test the connection to verify it's working

3. **Configure TimescaleDB Storage**
   
   - Click "Storage Backends" → "Add Storage"
   - Select "TimescaleDB"
   - Enter your database connection details:
     - Host: `localhost` (or your database server)
     - Port: `5432`
     - Database name: `weather`
     - Username and password
   - The system will automatically create the necessary tables

4. **Enable Services** (Optional)
   
   Add any weather services you want to use:
   - **Web Interface**: Enable the REST server controller for the live weather website
   - **Weather Underground**: Add your station ID and API key
   - **PWS Weather**: Add your station ID and password
   - **APRS/CWOP**: Add your callsign (ham radio) or CWOP ID

### Step 3: Run RemoteWeather

Once configured, RemoteWeather will automatically start collecting and sharing weather data.

**For continuous operation**, set up RemoteWeather as a system service:

```bash
# Create systemd service file
sudo nano /etc/systemd/system/remoteweather.service

# Add the following content:
[Unit]
Description=RemoteWeather Service
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/remoteweather
Restart=always
User=weather

[Install]
WantedBy=multi-user.target

# Enable and start the service
sudo systemctl enable remoteweather
sudo systemctl start remoteweather
```

## Using RemoteWeather

### Viewing Your Weather Data

* **Web Interface**: Access your weather website at `http://your-server:8080` (or configured port)
* **Management API**: Access configuration at `http://your-server:8081`
* **Mobile Access**: The web interface is fully responsive and works on all devices

### Available Controllers

RemoteWeather includes these controllers for different purposes:

* **REST Server** - Provides the main weather website and API
* **Management API** - System configuration and monitoring interface
* **Weather Underground** - Uploads data to wunderground.com
* **PWS Weather** - Uploads data to pwsweather.com
* **Aeris Weather** - Integration with Aeris Weather network
* **APRS** - Transmits data to APRS-IS network

### Storage Backends

* **TimescaleDB** - Primary storage for all weather readings with time-series optimization
* **APRS** - Routes data to APRS/CWOP networks
* **gRPC Stream** - Provides real-time data streaming for custom applications

## Troubleshooting

### Viewing Logs
```bash
# If running directly
./remoteweather -v  # Verbose output

# If running as systemd service
sudo journalctl -u remoteweather -f
```

### Common Issues

1. **"Cannot connect to weather station"**
   - Verify the station is powered on
   - Check cable connections
   - Ensure correct serial port or IP address in configuration

2. **"Database connection failed"**
   - Verify TimescaleDB is running: `sudo systemctl status postgresql`
   - Check database credentials in configuration
   - Ensure database exists and user has permissions

3. **"Management API not accessible"**
   - Check if RemoteWeather is running
   - Verify firewall allows port 8081
   - Ensure you're using the correct access token

## Docker Installation (Alternative)

For easier deployment, RemoteWeather is available as a Docker image:

```bash
# Pull the latest image
docker pull chrissnell/remoteweather:latest

# Run with bootstrap (first time)
docker run -it --rm \
  -v remoteweather-config:/root/.config/remoteweather \
  -p 8080:8080 \
  -p 8081:8081 \
  chrissnell/remoteweather:latest

# Save the access token shown during bootstrap!

# For production, use docker-compose:
docker-compose up -d
```

Example `docker-compose.yml`:
```yaml
version: '3'
services:
  remoteweather:
    image: chrissnell/remoteweather:latest
    ports:
      - "8080:8080"  # Weather website
      - "8081:8081"  # Management API
    volumes:
      - remoteweather-config:/root/.config/remoteweather
      - /dev/ttyUSB0:/dev/ttyUSB0  # For serial weather stations
    devices:
      - /dev/ttyUSB0:/dev/ttyUSB0
    restart: unless-stopped

  timescaledb:
    image: timescale/timescaledb:latest-pg14
    environment:
      POSTGRES_PASSWORD: weatherpass
      POSTGRES_DB: weather
    volumes:
      - timescale-data:/var/lib/postgresql/data
    restart: unless-stopped

volumes:
  remoteweather-config:
  timescale-data:
```

## Advanced Features

### Custom Applications

RemoteWeather provides a gRPC interface for building custom applications:

* **Real-time Data Stream** - Subscribe to live weather updates
* **Historical Queries** - Request specific date ranges
* **Custom Clients** - Build desktop widgets, mobile apps, or integrations

Example client: [grpc-weather-bar](https://github.com/chrissnell/grpc-weather-bar) for Linux desktop

### Data Export

You can export your weather data directly from TimescaleDB:

```sql
-- Connect to database
psql -U weather -d weather

-- Export last 30 days as CSV
\COPY (SELECT * FROM readings WHERE time > NOW() - INTERVAL '30 days') 
TO '/tmp/weather_export.csv' CSV HEADER;
```

## Support

### Getting Help

* **Issues**: Report bugs at [GitHub Issues](https://github.com/chrissnell/remoteweather/issues)
* **Documentation**: Check the [docs/](docs/) directory for detailed guides
* **Examples**: See example configurations in the repository

### Community Resources

* **Weather Station Forums**: Hardware-specific help
* **TimescaleDB Community**: Database optimization tips
* **Amateur Radio Groups**: APRS/CWOP configuration assistance

## License

RemoteWeather is open source software. See LICENSE file for details.
