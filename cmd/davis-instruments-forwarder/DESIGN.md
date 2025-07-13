# Weather Station Forwarder Design Guide

This document outlines the design pattern for creating standalone weather station forwarders that read data from weather stations and forward it to a gRPC receiver.

## Architecture Overview

```
[Weather Station] <--serial/network--> [Forwarder] <--gRPC--> [gRPC Receiver]
```

## Core Components

### 1. Configuration

Weather station forwarders should accept configuration via:
- Command-line flags (primary)
- Environment variables (fallback)

Required configuration:
- Connection details (serial port OR network address)
- gRPC server address
- Station name (unique identifier)

Optional configuration:
- Station metadata (location, APRS callsign)
- Operational settings (log level, timeouts)

### 2. Connection Management

Forwarders should handle connection types that are appropriate to the station hardware being forwarded.  Some stations are serial-only, some have network endpoints, and some have both.  Some stations export their data via HTTP post and will need a HTTP listener to send to.

#### Serial Connection
```go
// Use github.com/tarm/serial
config := &serial.Config{Name: port, Baud: baudRate}
conn, err := serial.OpenPort(config)
```

#### Network Connection
```go
conn, err := net.DialTimeout("tcp", address, timeout)
```

### 3. Protocol Implementation

Each weather station type has its own protocol. The forwarder must:
1. Send commands (where needed) to request data
2. Parse the station's response format
3. Handle protocol-specific quirks (wake sequences, packet formats, malformed data)

### 4. Data Conversion

Convert station-specific data format to the standard gRPC WeatherReading message:
```go
pbReading := &pb.WeatherReading{
    ReadingTimestamp:   timestamppb.New(time.Now()),
    StationName:        stationName,
    StationType:        "station_type",
    // ... map fields from station format to protobuf
}
```

### 5. gRPC Forwarding

Use streaming RPC to send readings:
```go
stream, err := client.SendWeatherReadings(ctx)
stream.Send(pbReading)
stream.CloseAndRecv()
```

## Implementation Template

```go
package main

import (
    // Standard imports
    "context"
    "flag"
    "log"
    // ... other imports
)

type forwarderConfig struct {
    // Connection settings
    serialPort  string
    networkAddr string
    
    // Required settings
    grpcServer  string
    stationName string
    
    // Optional settings
    // ... station-specific options
}

func main() {
    cfg := parseConfig()
    ctx := setupSignalHandling()
    
    if err := runForwarder(ctx, cfg); err != nil {
        log.Fatal(err)
    }
}

func runForwarder(ctx context.Context, cfg *forwarderConfig) error {
    // 1. Connect to gRPC server
    grpcClient := connectToGRPC(cfg.grpcServer)
    
    // 2. Connect to weather station
    stationConn := connectToStation(cfg)
    
    // 3. Main loop
    for {
        select {
        case <-ctx.Done():
            return nil
        default:
            reading := getStationReading(stationConn)
            forwardReading(grpcClient, reading)
        }
    }
}
```

## Best Practices

### Error Handling
- Log errors with context
- Implement retry logic for transient failures
- Graceful degradation (continue operating despite errors)

### Resource Management
- Minimize memory allocation in the main loop
- Use buffered channels where appropriate
- Close connections properly on shutdown

### Reliability
- Implement station wake/keep-alive mechanisms
- Handle partial reads and timeouts
- Automatic reconnection on failure

### Observability
- Structured logging with appropriate levels
- Key metrics: readings forwarded, errors, connection status
- Debug mode for protocol troubleshooting

## Station-Specific Considerations

### Davis Instruments
- LOOP protocol with 99-byte packets
- Wake sequence required (send newline, wait for response)
- CRC validation important
- Request multiple packets (e.g., 20) for efficiency

### Other Station Types
When implementing forwarders for other stations:
1. Study the station's communication protocol
2. Identify the minimal command set needed
3. Map station data fields to WeatherReading protobuf
4. Handle station-specific quirks (timing, encoding, etc.)

## Testing

1. **Unit tests**: Protocol parsing, data conversion
2. **Integration tests**: Mock station responses
3. **End-to-end tests**: Real hardware or simulators
4. **Reliability tests**: Connection failures, partial data

## Deployment

Forwarders are designed to run on minimal hardware:
- Raspberry Pi or similar SBC
- Industrial PCs
- Container environments
- Systemd service on Linux

Example systemd service:
```ini
[Unit]
Description=Davis Weather Station Forwarder
After=network.target

[Service]
Type=simple
ExecStart=/usr/local/bin/davis-instruments-forwarder
Restart=always
Environment="DAVIS_SERIAL_PORT=/dev/ttyUSB0"
Environment="DAVIS_GRPC_SERVER=localhost:50051"
Environment="DAVIS_STATION_NAME=BackyardStation"

[Install]
WantedBy=multi-user.target
```