# Weather Station Capabilities Implementation Plan

**Date**: 2025-10-25
**Purpose**: Implement a capability system for weather stations to ensure controllers and functions only operate on stations with appropriate capabilities (WEATHER, SNOW, AIRQUALITY).

---

## Table of Contents
1. [Overview](#overview)
2. [Current State](#current-state)
3. [Proposed Capability Types](#proposed-capability-types)
4. [Phase 1: Define Capability System](#phase-1-define-capability-system)
5. [Phase 2: Replace Type Checks](#phase-2-replace-type-checks-with-capability-checks)
6. [Phase 3: Add Capability Gates](#phase-3-add-capability-gates-to-measurement-functions)
7. [Phase 4: Update Controllers](#phase-4-update-controllers-for-capability-awareness)
8. [Phase 5: Update Stations](#phase-5-update-station-implementations)
9. [Phase 6: Update Manager](#phase-6-update-manager-layer)
10. [Phase 7: Update Interface](#phase-7-update-interface)
11. [APRS Specific Implementation](#aprs-specific-implementation)
12. [Implementation Order](#implementation-order)
13. [Testing Strategy](#testing-strategy)

---

## Overview

### Problem Statement
Currently, the system uses string-based station types (`"davis"`, `"snowgauge"`, `"airgradient"`) to determine functionality. This leads to:
- ❌ APRS controller sending snow gauge and air quality data to APRS-IS
- ❌ Rainfall calculations running on snow gauges
- ❌ Snow depth queries on air quality sensors
- ❌ Type checks scattered throughout codebase (10+ locations)

### Solution
Implement a capability-based system where each station declares what it can measure:
- **WEATHER**: Temperature, humidity, pressure, wind, rainfall
- **SNOW**: Snow depth measurements
- **AIRQUALITY**: PM2.5, CO2, VOC, NOx measurements

---

## Current State

### Station Types
- `davis` - Weather station (Vantage Pro2)
- `campbell` - Weather station (Campbell Scientific)
- `ambient-customized` - Weather station (Ambient Weather)
- `snowgauge` - Snow depth sensor
- `airgradient` - Air quality sensor
- `grpcreceiver` - Remote station (dynamic capabilities)

### Current Interface
```go
// internal/weatherstations/interface.go:9-13
type WeatherStation interface {
    StartWeatherStation() error
    StopWeatherStation() error
    StationName() string
}
```

---

## Proposed Capability Types

### Capability Definition
```go
type Capability uint8

const (
    Weather    Capability = 1 << 0  // 0x01 - Standard weather data
    Snow       Capability = 1 << 1  // 0x02 - Snow depth measurements
    AirQuality Capability = 1 << 2  // 0x04 - Air quality metrics
)
```

### Capability Mapping
| Station Type | Capabilities |
|--------------|-------------|
| `davis` | `Weather` |
| `campbell` | `Weather` |
| `ambient-customized` | `Weather` (possibly `Weather \| AirQuality`) |
| `snowgauge` | `Snow` |
| `airgradient` | `AirQuality` |
| `grpcreceiver` | Dynamic (query from remote) |

---

## Phase 1: Define Capability System

### 1.1 Create Capability Types

**New File**: `internal/weatherstations/capabilities.go`

```go
package weatherstations

import "strings"

// Capability represents a specific measurement capability of a weather station
type Capability uint8

const (
    Weather    Capability = 1 << 0  // 0x01 - Standard weather (temp, humidity, pressure, wind, rain)
    Snow       Capability = 1 << 1  // 0x02 - Snow depth measurements
    AirQuality Capability = 1 << 2  // 0x04 - Air quality (PM2.5, CO2, VOC, NOx)
)

// String returns the string representation of a capability
func (c Capability) String() string {
    switch c {
    case Weather:
        return "Weather"
    case Snow:
        return "Snow"
    case AirQuality:
        return "AirQuality"
    default:
        return "Unknown"
    }
}

// Capabilities represents a set of capabilities using a bitmask
type Capabilities uint8

// Has checks if a specific capability is present
func (c Capabilities) Has(cap Capability) bool {
    return (uint8(c) & uint8(cap)) != 0
}

// Add adds a capability to the set
func (c *Capabilities) Add(cap Capability) {
    *c = Capabilities(uint8(*c) | uint8(cap))
}

// Remove removes a capability from the set
func (c *Capabilities) Remove(cap Capability) {
    *c = Capabilities(uint8(*c) &^ uint8(cap))
}

// List returns all capabilities as a slice
func (c Capabilities) List() []Capability {
    var caps []Capability
    if c.Has(Weather) {
        caps = append(caps, Weather)
    }
    if c.Has(Snow) {
        caps = append(caps, Snow)
    }
    if c.Has(AirQuality) {
        caps = append(caps, AirQuality)
    }
    return caps
}

// String returns a comma-separated string of capabilities
func (c Capabilities) String() string {
    caps := c.List()
    if len(caps) == 0 {
        return "None"
    }

    strs := make([]string, len(caps))
    for i, cap := range caps {
        strs[i] = cap.String()
    }
    return strings.Join(strs, ", ")
}
```

---

## Phase 2: Replace Type Checks with Capability Checks

### 2.1 Station Type Validation

**Files to Update:**

#### internal/controllers/management/handlers_config.go:83-84
```go
// CURRENT:
validTypes := []string{"campbellscientific", "davis", "snowgauge", "ambient-customized", "grpcreceiver", "airgradient"}
if !contains(validTypes, device.Type) {
    h.sendError(w, http.StatusBadRequest, fmt.Sprintf("Invalid device type. Must be one of: %v", validTypes), nil)
    return
}

// KEEP for backward compatibility but note that capabilities are primary going forward
```

#### pkg/config/provider.go:405-407
```go
// CURRENT:
hasAmbientCustomized := device.Type == "ambient-customized" && device.Port != ""
hasGRPCReceiver := device.Type == "grpcreceiver" && device.Port != ""
hasAirGradient := device.Type == "airgradient" && device.Hostname != ""

// FUTURE: After capability implementation, connection validation should be capability-aware
```

#### pkg/config/provider.go:418
```go
// CURRENT:
if device.Type == "airgradient" && device.Hostname != "" && device.Port == "" {
    device.Port = "80"
}

// CHANGE TO (after capability system):
if station.Capabilities().Has(AirQuality) && device.Hostname != "" && device.Port == "" {
    device.Port = "80"
}
```

#### pkg/config/provider.go:423
```go
// CURRENT:
if device.Type == "snowgauge" {
    if device.BaseSnowDistance <= 0 {
        errors = append(errors, ValidationError{
            Field:   fmt.Sprintf("devices[%d].base_snow_distance", i),
            Value:   fmt.Sprintf("%d", device.BaseSnowDistance),
            Message: "snow gauge must have base_snow_distance > 0",
        })
    }
}

// CHANGE TO:
if station.Capabilities().Has(Snow) {
    if device.BaseSnowDistance <= 0 {
        // ... validation error
    }
}
```

#### pkg/config/provider.go:434
```go
// CURRENT:
if device.Type == "ambient-customized" && device.Path != "" {
    // Ensure path starts with /
    if !strings.HasPrefix(device.Path, "/") {
        // ... error
    }
}

// KEEP as type-specific configuration (not capability-based)
```

#### internal/controllers/management/handlers_config.go:517-586
```go
// validateDeviceConnectionSettings - Replace entire switch statement

// CURRENT:
func (h *Handlers) validateDeviceConnectionSettings(device *config.DeviceData) error {
    switch device.Type {
    case "campbellscientific", "davis":
        // TCP or serial validation
    case "snowgauge":
        // gRPC validation
    case "airgradient":
        // HTTP validation
    // ... etc
    }
}

// FUTURE: Capability-based validation
// Note: Some connection types are still type-specific, so this may remain partially type-based
```

---

## Phase 3: Add Capability Gates to Measurement Functions

### 3.1 Rainfall Functions (HIGH PRIORITY)

#### internal/controllers/rainfall_calc.go:13
```go
// CURRENT:
func CalculateDailyRainfall(db *database.Client, stationName string) float32 {
    // Get today's rainfall in two fast queries to avoid slow subquery
    loc, err := time.LoadLocation("America/Los_Angeles")
    // ... continues

// ADD AT START:
func CalculateDailyRainfall(db *database.Client, stationName string, stationMgr managers.WeatherStationManager) float32 {
    // Check if station has weather capability
    station := stationMgr.GetStation(stationName)
    if station == nil {
        log.Warnf("Station %s not found for rainfall calculation", stationName)
        return 0.0
    }

    if !station.Capabilities().Has(weatherstations.Weather) {
        log.Debugf("Skipping rainfall calculation for %s - no Weather capability", stationName)
        return 0.0
    }

    // ... original implementation
}
```

#### internal/controllers/aprs/controller.go:394-397
```go
// CURRENT:
// Calculate accurate daily rainfall from incremental values using the same
// methodology as REST API
calculatedDayRain := controllers.CalculateDailyRainfall(a.DB, reading.StationName)
// Then we add our rainfall since midnight

// CHANGE TO:
// Only calculate rainfall if station has weather capability
if station := a.stationManager.GetStation(reading.StationName); station != nil {
    if station.Capabilities().Has(weatherstations.Weather) {
        calculatedDayRain := controllers.CalculateDailyRainfall(a.DB, reading.StationName, a.stationManager)
    }
}
```

#### internal/controllers/wunderground/controller.go:128-130
```go
// ADD capability check before:
calculatedDayRain := controllers.CalculateDailyRainfall(p.DB, r.StationName, p.stationManager)
```

#### internal/controllers/pwsweather/controller.go:124-126
```go
// ADD capability check before:
calculatedDayRain := controllers.CalculateDailyRainfall(p.DB, r.StationName, p.stationManager)
```

#### internal/controllers/restserver/handlers.go:265-271
```go
// CURRENT:
// Add total rainfall for the day using the shared optimized calculation
if dbClient != nil {
    // Create a temporary database.Client wrapper for the CalculateDailyRainfall function
    dbClient := &database.Client{TimescaleDBConn: h.controller.DB}
    calculatedDayRain := controllers.CalculateDailyRainfall(dbClient, stationName)
    latestReading.RainfallDay = calculatedDayRain
}

// ADD capability check
```

#### internal/controllers/restserver/handlers.go:274-319
```go
// Rainfall 24h/48h/72h calculations
// Wrap entire section in Weather capability check:

if station := h.controller.stationManager.GetStation(stationName); station != nil {
    if station.Capabilities().Has(weatherstations.Weather) {
        // ... existing rainfall calculations
    }
}
```

### 3.2 Snow Functions (HIGH PRIORITY)

#### internal/controllers/restserver/handlers.go:351-518
```go
// GetSnowLatest - already checks website.SnowEnabled
// ADD station capability check:

func (h *Handlers) GetSnowLatest(w http.ResponseWriter, req *http.Request) {
    website := h.getWebsite(req)
    if website == nil {
        // ... error
    }

    if !website.SnowEnabled {
        http.Error(w, "snow data not enabled for this website", http.StatusNotFound)
        return
    }

    // ADD: Check if station has Snow capability
    if station := h.controller.stationManager.GetStation(website.SnowDeviceName); station != nil {
        if !station.Capabilities().Has(weatherstations.Snow) {
            log.Warnf("Snow device %s does not have Snow capability", website.SnowDeviceName)
            http.Error(w, "configured snow device does not support snow measurements", http.StatusInternalServerError)
            return
        }
    }

    // ... rest of function
}
```

#### internal/grpcutil/utils.go:42-50
```go
// CURRENT:
func (dm *DeviceManager) GetSnowBaseDistance(deviceName string) float32 {
    dm.mu.RLock()
    defer dm.mu.RUnlock()

    if device, ok := dm.devices[deviceName]; ok {
        return float32(device.BaseSnowDistance)
    }
    return 0.0
}

// ADD capability validation:
func (dm *DeviceManager) GetSnowBaseDistance(deviceName string, stationMgr managers.WeatherStationManager) float32 {
    // Check if station has Snow capability
    if station := stationMgr.GetStation(deviceName); station != nil {
        if !station.Capabilities().Has(weatherstations.Snow) {
            log.Debugf("Device %s does not have Snow capability", deviceName)
            return 0.0
        }
    }

    dm.mu.RLock()
    defer dm.mu.RUnlock()

    if device, ok := dm.devices[deviceName]; ok {
        return float32(device.BaseSnowDistance)
    }
    return 0.0
}
```

#### internal/controllers/restserver/controller.go:789-792
```go
// CURRENT:
func (c *Controller) getSnowBaseDistanceForStation(stationName string) float64 {
    return float64(c.DeviceManager.GetSnowBaseDistance(stationName))
}

// ADD capability validation
```

### 3.3 Air Quality Functions (MEDIUM PRIORITY)

- Air quality data handling in `internal/weatherstations/airgradient/station.go:168-198` is fine
- Any REST endpoints serving air quality data should check `AirQuality` capability
- Future air quality upload services should check capability before sending

---

## Phase 4: Update Controllers for Capability Awareness

### 4.1 APRS Controller (CRITICAL)

**File**: internal/controllers/aprs/controller.go

#### Step 1: Add Station Manager to Controller
```go
// Line 27-37
type Controller struct {
    ctx              context.Context
    cancel           context.CancelFunc
    configProvider   config.ConfigProvider
    DB               *database.Client
    wg               *sync.WaitGroup
    logger           *zap.SugaredLogger
    running          bool
    runningMutex     sync.RWMutex
    stationManager   managers.WeatherStationManager  // ADD THIS
}
```

#### Step 2: Update Constructor
```go
// Line 40-79
func New(configProvider config.ConfigProvider, stationManager managers.WeatherStationManager) (*Controller, error) {
    // ... existing validation ...

    a := &Controller{
        configProvider: configProvider,
        DB:             db,
        wg:             &sync.WaitGroup{},
        stationManager: stationManager,  // ADD THIS
    }

    // ... rest of constructor
}
```

#### Step 3: Add Capability Check to sendStationReadingToAPRSIS
```go
// Line 251-260
func (a *Controller) sendStationReadingToAPRSIS(ctx context.Context, wg *sync.WaitGroup, device config.DeviceData) {
    wg.Add(1)
    defer wg.Done()

    // CHECK WEATHER CAPABILITY FIRST
    station := a.stationManager.GetStation(device.Name)
    if station == nil {
        log.Debugf("Station %s not found, skipping APRS report", device.Name)
        return
    }

    // Only send to APRS-IS if station has weather capability
    if !station.Capabilities().Has(weatherstations.Weather) {
        log.Debugf("Skipping APRS report for %s - no Weather capability (type: %s)",
            device.Name, device.Type)
        return
    }

    // Get latest reading from database
    reading, err := a.DB.GetReadingsFromTimescaleDB(device.Name)
    if err != nil {
        log.Errorf("Error getting reading for %s: %v", device.Name, err)
        return
    }

    // ... rest of function continues normally
}
```

#### Alternative: Simple Type-Based Check (Interim Solution)
```go
// Quick fix until full capability system is implemented
func (a *Controller) sendStationReadingToAPRSIS(ctx context.Context, wg *sync.WaitGroup, device config.DeviceData) {
    wg.Add(1)
    defer wg.Done()

    // Skip non-weather station types
    if device.Type == "snowgauge" || device.Type == "airgradient" {
        log.Debugf("Skipping APRS report for %s - type %s does not report weather data",
            device.Name, device.Type)
        return
    }

    // ... rest continues normally
}
```

### 4.2 Weather Underground Controller

**File**: internal/controllers/wunderground/controller.go

Add Weather capability check before uploading (similar to APRS)

### 4.3 PWS Weather Controller

**File**: internal/controllers/pwsweather/controller.go

Add Weather capability check before uploading (similar to APRS)

### 4.4 REST Server

**File**: internal/controllers/restserver/handlers.go

- Weather endpoints: Check Weather capability before serving data
- Snow endpoints: Check Snow capability (in addition to `SnowEnabled`)
- Air quality endpoints: Check AirQuality capability

---

## Phase 5: Update Station Implementations

Add `Capabilities()` method to each station type.

### 5.1 Davis Station
**File**: internal/weatherstations/davis/station.go

```go
// Add after StationName() method
func (s *Station) Capabilities() weatherstations.Capabilities {
    return weatherstations.Weather
}
```

### 5.2 Campbell Station
**File**: internal/weatherstations/campbell/station.go

```go
func (s *Station) Capabilities() weatherstations.Capabilities {
    return weatherstations.Weather
}
```

### 5.3 Ambient Customized Station
**File**: internal/weatherstations/ambientcustomized/station.go

```go
func (s *Station) Capabilities() weatherstations.Capabilities {
    // May support both weather and air quality
    return weatherstations.Weather
    // Or if it supports air quality:
    // caps := weatherstations.Weather
    // caps.Add(weatherstations.AirQuality)
    // return caps
}
```

### 5.4 Snow Gauge Station
**File**: internal/weatherstations/snowgauge/station.go

```go
func (s *Station) Capabilities() weatherstations.Capabilities {
    return weatherstations.Snow
}
```

### 5.5 AirGradient Station
**File**: internal/weatherstations/airgradient/station.go

```go
func (s *Station) Capabilities() weatherstations.Capabilities {
    return weatherstations.AirQuality
}
```

### 5.6 gRPC Receiver Station
**File**: internal/weatherstations/grpcreceiver/station.go

```go
func (s *Station) Capabilities() weatherstations.Capabilities {
    // Dynamic based on remote station type
    // Query from registry to determine remote station capabilities

    // Map station type to capabilities
    // This would need to be determined from RemoteStation metadata
    // For now, default to Weather for backward compatibility
    return weatherstations.Weather
}
```

---

## Phase 6: Update Manager Layer

### 6.1 Add GetStation Method to Interface

**File**: internal/managers/weatherstation.go:21-26

```go
type WeatherStationManager interface {
    StartWeatherStations() error
    AddWeatherStation(deviceName string) error
    RemoveWeatherStation(deviceName string) error
    ReloadWeatherStationsConfig() error
    GetStation(deviceName string) weatherstations.WeatherStation  // ADD THIS
}
```

### 6.2 Implement GetStation

**File**: internal/managers/weatherstation.go

```go
// Add new method to weatherStationManager
func (wsm *weatherStationManager) GetStation(deviceName string) weatherstations.WeatherStation {
    wsm.mu.RLock()
    defer wsm.mu.RUnlock()

    station, exists := wsm.stations[deviceName]
    if !exists {
        return nil
    }
    return station
}
```

### 6.3 Store Capabilities for Quick Lookup (Optional)

**File**: internal/managers/weatherstation.go:172-219

In `createStationFromConfig()`, after creating a station, optionally cache its capabilities for faster lookups.

---

## Phase 7: Update Interface

### 7.1 Extend WeatherStation Interface

**File**: internal/weatherstations/interface.go:9-13

```go
type WeatherStation interface {
    StartWeatherStation() error
    StopWeatherStation() error
    StationName() string
    Capabilities() Capabilities  // ADD THIS
}
```

---

## APRS Specific Implementation

### Benefits of APRS Capability Check

✅ **Prevents Invalid Data**: Snow gauges won't send invalid weather data to APRS-IS
✅ **Reduces Network Traffic**: Only weather stations use APRS bandwidth
✅ **Cleaner Logs**: Eliminates "no weather data" errors for non-weather stations
✅ **APRS Spec Compliance**: APRS is designed for weather reporting, not air quality or snow-only data

### Test Scenarios

| Station Type | Has Weather Cap | APRS Enabled | Should Send to APRS-IS |
|--------------|----------------|--------------|------------------------|
| Davis | ✅ Yes | ✅ Yes | ✅ Yes |
| Campbell | ✅ Yes | ✅ Yes | ✅ Yes |
| Snow Gauge | ❌ No | ✅ Yes | ❌ No (skip) |
| AirGradient | ❌ No | ✅ Yes | ❌ No (skip) |
| Davis | ✅ Yes | ❌ No | ❌ No (APRS disabled) |

---

## Implementation Order

### Priority 1: Critical (Do First)
1. ✅ Create capability types (`internal/weatherstations/capabilities.go`)
2. ✅ Extend WeatherStation interface
3. ✅ Implement `Capabilities()` in all 6 station types
4. ✅ Add capability check to APRS controller (most critical)

### Priority 2: High (Do Next)
5. ✅ Add capability gates to rainfall calculations (6 locations)
6. ✅ Add capability gates to snow functions (4 locations)
7. ✅ Update Weather Underground controller
8. ✅ Update PWS Weather controller

### Priority 3: Medium (After Core)
9. ✅ Update REST server endpoints with capability checks
10. ✅ Add `GetStation()` to WeatherStationManager
11. ✅ Replace type checks in validation/config files

### Priority 4: Low (Polish)
12. ✅ Add air quality capability checks (if endpoints exist)
13. ✅ Update documentation
14. ✅ Add capability information to management API responses

---

## Testing Strategy

### Unit Tests

#### Capability System Tests
```go
// Test capability operations
func TestCapabilities(t *testing.T) {
    caps := weatherstations.Capabilities(0)

    // Test Add
    caps.Add(weatherstations.Weather)
    assert.True(t, caps.Has(weatherstations.Weather))

    // Test multiple capabilities
    caps.Add(weatherstations.AirQuality)
    assert.True(t, caps.Has(weatherstations.Weather))
    assert.True(t, caps.Has(weatherstations.AirQuality))
    assert.False(t, caps.Has(weatherstations.Snow))

    // Test Remove
    caps.Remove(weatherstations.Weather)
    assert.False(t, caps.Has(weatherstations.Weather))
    assert.True(t, caps.Has(weatherstations.AirQuality))
}
```

#### Station Capability Tests
```go
func TestDavisCapabilities(t *testing.T) {
    station := davis.NewStation(...)
    caps := station.Capabilities()

    assert.True(t, caps.Has(weatherstations.Weather))
    assert.False(t, caps.Has(weatherstations.Snow))
    assert.False(t, caps.Has(weatherstations.AirQuality))
}

func TestSnowGaugeCapabilities(t *testing.T) {
    station := snowgauge.NewStation(...)
    caps := station.Capabilities()

    assert.True(t, caps.Has(weatherstations.Snow))
    assert.False(t, caps.Has(weatherstations.Weather))
}
```

### Integration Tests

#### APRS Controller Tests
```go
func TestAPRSSkipsNonWeatherStations(t *testing.T) {
    // Setup snow gauge with APRS enabled
    snowDevice := config.DeviceData{
        Name: "snow1",
        Type: "snowgauge",
        APRSEnabled: true,
        APRSCallsign: "TEST123",
    }

    // Send report - should skip
    controller.sendStationReadingToAPRSIS(ctx, wg, snowDevice)

    // Verify no APRS packet was sent
    // (check logs or mock APRS connection)
}
```

#### Rainfall Calculation Tests
```go
func TestRainfallSkipsSnowGauge(t *testing.T) {
    // Call rainfall calculation on snow gauge
    rain := controllers.CalculateDailyRainfall(db, "snow1", stationMgr)

    // Should return 0 and log skip message
    assert.Equal(t, float32(0.0), rain)
}
```

### Manual Testing Checklist

- [ ] Davis station sends to APRS-IS
- [ ] Snow gauge does NOT send to APRS-IS (even if APRS enabled)
- [ ] AirGradient does NOT send to APRS-IS
- [ ] Rainfall calculations skip snow gauges
- [ ] Snow calculations skip weather-only stations
- [ ] Mixed capability stations work correctly
- [ ] Management API shows capability information
- [ ] Configuration validation accepts all station types
- [ ] Logs clearly indicate when stations are skipped due to capabilities

---

## Migration Considerations

### Backward Compatibility
- ✅ Existing stations continue to work (they'll implement `Capabilities()`)
- ✅ Station type strings remain valid for configuration
- ✅ No database schema changes required
- ✅ API remains compatible (capabilities are additive)

### Rollout Strategy
1. Implement capability system (no behavior changes yet)
2. Add capability checks to APRS (first visible change - prevents bad data)
3. Add capability checks to other controllers
4. Gradually replace type checks with capability checks
5. Update documentation and API to expose capabilities

### Configuration Updates
No changes required to existing configuration files. The system will infer capabilities from station type during runtime.

---

## Future Enhancements

### Multi-Capability Stations
Support stations that have multiple capabilities:
```go
// Example: Weather station with air quality sensor
caps := weatherstations.Weather
caps.Add(weatherstations.AirQuality)
return caps
```

### Capability-Based Routing
REST API could filter/route based on capabilities:
```
GET /api/weather?capabilities=weather
GET /api/stations?capabilities=snow,weather
```

### Dynamic Capability Discovery
Remote stations could advertise their capabilities:
```go
// gRPC registration includes capability flags
message RemoteStationConfig {
    string station_id = 1;
    string station_name = 2;
    repeated string capabilities = 3; // ["weather", "airquality"]
}
```

### Capability-Aware Data Aggregation
Aggregate data only from stations with appropriate capabilities:
```sql
SELECT AVG(temperature)
FROM readings r
JOIN stations s ON r.station_name = s.name
WHERE s.capabilities & 0x01 != 0  -- Has Weather capability
```

---

## Summary

| Metric | Value |
|--------|-------|
| **Files to Modify** | ~25 files |
| **New Files** | 1 (capabilities.go) |
| **Lines of Code** | ~300 LOC |
| **Priority Locations** | 15 critical |
| **Estimated Effort** | 2-3 days |

### Key Benefits
✅ Type-safe capability checking
✅ Prevents invalid data being sent to external services
✅ Cleaner, more maintainable code
✅ Better error messages and logging
✅ Supports multi-capability stations in future
✅ No breaking changes to existing deployments

### Critical Success Factors
1. All 6 station types implement `Capabilities()` correctly
2. APRS controller checks Weather capability before sending
3. Rainfall/snow calculations check capabilities
4. Backward compatibility maintained throughout
