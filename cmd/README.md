# RemoteWeather Commands

This directory contains the main RemoteWeather application and weather station emulators for testing.

## Applications

### `remoteweather/`
Main RemoteWeather application that collects, processes, and serves weather data.

**Usage:**
```bash
./remoteweather -config config.yaml [-debug]
```

## Weather Station Emulators

### `campbell-emulator/`
Campbell Scientific weather station emulator that outputs JSON data over TCP.

**Usage:**
```bash
./campbell-emulator [-port 8123]
```

**Features:**
- Generates realistic weather data with seasonal and daily patterns
- Outputs JSON packets every 2 seconds
- Default port: 8123
- Connect RemoteWeather with: `hostname: localhost, port: 8123`

**Sample Output:**
```json
{"batt_volt":13.525517,"airtemp_f":71.996002,"rh":27.408001,"baro":30.176264,"baro_temp_f":79.087997,"slr_mj":0.008491,"slr_w":849.119934,"rain_in":0.000000,"wind_s":2.077000,"wind_d":70}
```

### `davis-emulator/`
Davis Instruments weather station emulator that responds to LOOP commands with binary packets.

**Usage:**
```bash
./davis-emulator [-port 22222]
```

**Features:**
- Implements Davis LOOP protocol with binary packets
- Responds to wake commands and `LPS 2 1` LOOP requests
- Sends 20 LOOP packets per request with proper CRC16 checksums
- Generates realistic weather data with seasonal and daily patterns
- Default port: 22222
- Connect RemoteWeather with: `hostname: localhost, port: 22222`

**Protocol:**
- Wake: `\n\r` → responds with `\n\r`
- LOOP: `LPS 2 1` → responds with ACK (`\x06`) + 20 binary LOOP packets
- Each packet is 99 bytes with CRC16 checksum

## Building

Build all commands:
```bash
go build ./cmd/remoteweather
go build ./cmd/campbell-emulator  
go build ./cmd/davis-emulator
```

Or build individually:
```bash
cd cmd/remoteweather && go build
cd cmd/campbell-emulator && go build
cd cmd/davis-emulator && go build
``` 