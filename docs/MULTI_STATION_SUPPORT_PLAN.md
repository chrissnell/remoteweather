# Multi-Station Support Implementation Plan

## Overview

Enable weather service controllers (PWS, Weather Underground, APRS, Aeris) to support multiple weather stations by moving per-station configuration into the devices table.

## Core Design Principle

**All weather service credentials and settings move to the devices table** - making each weather station self-contained and configurable through the device management UI.

## Database Changes

### 1. Add Weather Service Fields to Devices Table (Migration 012)

```sql
-- PWS Weather
ALTER TABLE devices ADD COLUMN pws_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN pws_station_id TEXT;
ALTER TABLE devices ADD COLUMN pws_password TEXT;
ALTER TABLE devices ADD COLUMN pws_upload_interval INTEGER DEFAULT 60;

-- Weather Underground
ALTER TABLE devices ADD COLUMN wu_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN wu_station_id TEXT;
ALTER TABLE devices ADD COLUMN wu_password TEXT;
ALTER TABLE devices ADD COLUMN wu_upload_interval INTEGER DEFAULT 300;

-- APRS (callsign/passcode already exist, just add these)
ALTER TABLE devices ADD COLUMN aprs_symbol_table CHAR(1) DEFAULT '/';
ALTER TABLE devices ADD COLUMN aprs_symbol_code CHAR(1) DEFAULT '_';
ALTER TABLE devices ADD COLUMN aprs_comment TEXT;

-- Aeris Weather
ALTER TABLE devices ADD COLUMN aeris_enabled BOOLEAN DEFAULT FALSE;
ALTER TABLE devices ADD COLUMN aeris_api_client_id TEXT;
ALTER TABLE devices ADD COLUMN aeris_api_client_secret TEXT;
```

### 2. Clean Up Controller Configs Table (Migration 013)

Remove per-station fields, keeping only:
- API endpoints
- APRS server configuration
- Global enable/disable flags

## Controller Changes

### Simple Pattern for All Controllers

```go
// Instead of single device from pull_from_device:
devices := loadEnabledDevices("pws")  // or "wu", "aprs", "aeris"

// Monitor each device separately:
for _, device := range devices {
    go monitorDevice(device)
}
```

### PWS/WU/APRS Controllers
- Query readings by device name
- Send using device-specific credentials
- Use device-specific upload intervals

### Aeris Controller
- Fetch forecasts for each device's lat/lon
- Store with existing location-based key
- No changes to forecast storage (already supports multiple locations)

## REST API Changes

### Forecast Endpoints
Add station parameter to forecast routes:
```
GET /api/forecast/{station_name}/hourly
GET /api/forecast/{station_name}/daily
```

The API maps station_name → device lat/lon → aeris_weather_forecasts lookup.

## No New APIs Needed

Existing device endpoints handle everything:
- `GET/POST/PUT/DELETE /api/devices`

Weather service configs are just additional device fields.

## Implementation Steps

1. Create database migrations:
   - Migration to add weather service fields to devices table
   - Migration to remove single-station fields from controller_configs
2. Update database bootstrapper:
   - Update initializeSchema() in SQLite provider to include new fields
   - Ensure new installations get the correct schema
3. Update DeviceData struct with new fields
4. Update SQLite provider queries
5. Modify controllers to iterate over enabled devices
6. Update REST forecast endpoints to accept station_name
7. Update UI to show weather service fields in device editor
