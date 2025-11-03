# Dynamic Configuration Reloading

RemoteWeather now supports dynamic configuration reloading without requiring a service restart. This allows you to add, remove, or modify weather stations, storage backends, and controllers on the fly.

## Features

### Storage Backends
- Dynamically add/remove TimescaleDB, gRPC, and APRS storage backends
- Existing connections are gracefully closed when storage backends are removed
- New storage backends are automatically started and begin receiving readings

### Weather Stations
- Add/remove Campbell Scientific, Davis, and Snow Gauge weather stations
- Newly added stations automatically start collecting data
- Removed stations stop collecting data (full cleanup on next restart)

### Controllers
- Add/remove REST API, management API, PWS Weather, Weather Underground, and AerisWeather controllers
- Controllers are started/stopped as needed based on configuration changes

## Usage

### Management API Endpoint

**POST /api/config/reload**

Triggers a dynamic configuration reload across all components.

**Headers:**
- `Authorization: Bearer <your-auth-token>`

**Response:**
```json
{
  "success": true,
  "message": "Configuration reloaded successfully", 
  "timestamp": 1703123456
}
```

### Configuration Changes

After modifying your configuration file (YAML or SQLite), call the reload endpoint:

```bash
curl -X POST \
  -H "Authorization: Bearer your-management-api-token" \
  http://localhost:8081/api/config/reload
```

### Example Workflow

1. **Add a new weather station:**
   - Edit your config file to add a new device
   - Call `/api/config/reload`
   - The new station starts collecting data immediately

2. **Remove a storage backend:**
   - Remove the storage configuration from your config file
   - Call `/api/config/reload`
   - The storage backend stops receiving new readings

3. **Add a new controller:**
   - Add controller configuration to your config file
   - Call `/api/config/reload`
   - The new controller starts serving requests

## Limitations

- Weather stations and controllers that are removed will continue running until the next application restart (they stop accepting new work but don't fully shut down)
- Storage backends are properly cleaned up when removed
- Configuration validation happens before any changes are applied
- If any component fails to start, the reload operation continues with other components

## Implementation Details

- Storage engines are tracked by name and can be individually started/stopped
- Weather stations are stored in a map for efficient lookup and management
- Controllers are managed by type, allowing only one controller of each type
- All managers support incremental configuration updates
- Changes are applied atomically per manager type

## Monitoring

Check the application logs for detailed information about configuration reloads:

```
INFO: Management API triggered configuration reload
INFO: Added TimescaleDB storage engine
INFO: Added and started weather station: station-001
INFO: Configuration reloaded successfully
``` 