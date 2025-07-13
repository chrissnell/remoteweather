# Davis Instruments Weather Station Forwarder

A standalone, lightweight forwarder for Davis Instruments weather stations that reads data via serial or network connection and forwards it to a gRPC receiver. Network connections use the high-performance gnet library for improved scalability.

## Features

- Supports both serial (USB/RS232) and network (TCP/IP) connections to Davis weather stations
- Network connections use high-performance gnet event-driven library
- Forwards weather data using gRPC protocol
- Minimal resource usage - suitable for embedded hardware (Raspberry Pi, etc.)
- Configurable via command-line flags or environment variables
- Optional APRS support for location data
- Reliable operation with automatic reconnection

## Usage

```bash
davis-instruments-forwarder [OPTIONS]
```

### Required Options

- `--server` or `DAVIS_GRPC_SERVER`: gRPC server address (e.g., `localhost:50051`)
- `--name` or `DAVIS_STATION_NAME`: Weather station name

### Connection Options (one required)

- `--serial` or `DAVIS_SERIAL_PORT`: Serial port (e.g., `/dev/ttyUSB0`)
- `--network` or `DAVIS_NETWORK_ADDR`: Network address (e.g., `192.168.1.100:22222`)

### Optional Configuration

- `--baud`: Baud rate for serial connection (default: 19200)
- `--aprs` or `DAVIS_APRS_CALLSIGN`: APRS callsign
- `--lat`: Station latitude
- `--lon`: Station longitude  
- `--alt`: Station altitude in meters
- `--log`: Log level (`info` or `debug`, default: `info`)

## Examples

### Serial Connection
```bash
davis-instruments-forwarder \
  --serial /dev/ttyUSB0 \
  --server grpc.example.com:50051 \
  --name "Backyard Station"
```

### Network Connection
```bash
davis-instruments-forwarder \
  --network 192.168.1.100:22222 \
  --server localhost:50051 \
  --name "Rooftop Station" \
  --aprs W1ABC \
  --lat 42.3601 \
  --lon -71.0589 \
  --alt 50
```

### Using Environment Variables
```bash
export DAVIS_SERIAL_PORT=/dev/ttyUSB0
export DAVIS_GRPC_SERVER=localhost:50051
export DAVIS_STATION_NAME="My Station"
davis-instruments-forwarder
```

## Building

```bash
go build -o davis-instruments-forwarder main.go
```

## Protocol Details

The forwarder:
1. Connects to a Davis weather station using the LOOP protocol
2. Requests 20 LOOP packets at a time
3. Converts each packet to a gRPC WeatherReading message
4. Forwards the reading to the configured gRPC server
5. Continues indefinitely until interrupted

## Error Handling

- Automatically attempts to wake sleeping Davis consoles
- Reconnects on connection failures
- Logs all errors with appropriate context
- Graceful shutdown on SIGINT/SIGTERM