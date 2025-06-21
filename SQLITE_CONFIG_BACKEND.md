# SQLite Configuration Backend

This document describes the SQLite configuration backend implementation for RemoteWeather, which provides an alternative to YAML-based configuration files.

## Overview

The SQLite configuration backend allows RemoteWeather to store and retrieve configuration data from a SQLite database instead of YAML files. This provides several advantages:

- **Better data integrity**: Schema validation and foreign key constraints
- **Versioning support**: Built-in migration system for schema evolution
- **Query capabilities**: SQL-based configuration queries and reporting
- **Atomic updates**: Transaction-based configuration changes
- **Centralized management**: Single database for multiple configuration profiles

## Architecture

### Components

1. **Configuration Providers** (`pkg/config/`)
   - `provider.go`: Common interfaces and data structures
   - `provider_yaml.go`: YAML file provider (existing functionality)
   - `provider_sqlite.go`: SQLite database provider (new)

2. **Migration System** (`pkg/migrate/`)
   - `migrate.go`: Core migration engine with transaction safety
   - `file_provider.go`: File-based migration loader
   - Database schema versioning and rollback support

3. **Schema** (`migrations/config/`)
   - `001_initial_schema.up.sql`: Initial database schema
   - `001_initial_schema.down.sql`: Schema rollback

4. **Utilities**
   - `cmd/config-convert/`: YAML to SQLite conversion tool
   - `cmd/config-test/`: Configuration comparison testing
   - `cmd/migrate/`: Database migration management

### Database Schema

The SQLite schema consists of five main tables:

- `configs`: Main configuration profiles (supports multiple named configs)
- `devices`: Weather station and sensor device configurations
- `storage_configs`: Storage backend configurations (InfluxDB, TimescaleDB, etc.)
- `controller_configs`: Controller configurations (PWS Weather, REST server, etc.)
- `weather_site_configs`: Weather site-specific settings (nested under REST controllers)

## Usage

### Command Line Flags

The main application now supports a `-config-backend` flag:

```bash
# Use YAML configuration (default)
./remoteweather -config config.yaml -config-backend yaml

# Use SQLite configuration
./remoteweather -config config.db -config-backend sqlite
```

### Converting YAML to SQLite

Use the `config-convert` utility to migrate existing YAML configurations:

```bash
# Convert with confirmation
./config-convert -yaml config.yaml -sqlite config.db

# Force overwrite existing database
./config-convert -yaml config.yaml -sqlite config.db -force

# Dry run (show what would be converted)
./config-convert -yaml config.yaml -sqlite config.db -dry-run
```

### Example Configuration

Example YAML configuration:
```yaml
devices:
  - name: "weather-station-1"
    type: "davis"
    hostname: "192.168.1.100"
    port: "22222"
    solar:
      latitude: 45.5152
      longitude: -122.6784
      altitude: 50.0

storage:
  timescaledb:
    connection-string: "postgresql://user:password@localhost:5432/weather"

controllers:
  - type: "rest"
    rest:
      port: 8080
      listen-addr: "0.0.0.0"
      weather-site:
        station-name: "My Weather Station"
        pull-from-device: "weather-station-1"
        page-title: "Local Weather"
```

After conversion, this data is stored in normalized SQLite tables with proper relationships and constraints.

## Testing and Validation

### Configuration Comparison

Use the `config-test` utility to verify YAML and SQLite configurations are equivalent:

```bash
./config-test -yaml config.yaml -sqlite config.db
```

Example output:
```
Configuration Comparison Test
===========================
Loading YAML configuration: test-config-simple.yaml
Loading SQLite configuration: test-config-simple.db

Comparison Results:
==================
Devices - YAML: 1, SQLite: 1
✓ Device count matches
✓ Device weather-station-1 matches

Storage Configuration:
✓ InfluxDB: both nil
✓ TimescaleDB: both nil
✓ GRPC: both nil
✓ APRS: both nil

Controllers - YAML: 1, SQLite: 1
✓ Controller count matches
✓ Controller rest matches

Test completed!
```

## Migration Management

### Database Migrations

The system uses a file-based migration system with transaction safety:

```bash
# Run all pending migrations
./migrate -database config.db up

# Rollback to specific version
./migrate -database config.db down 0

# Check current version
./migrate -database config.db version
```

### Schema Evolution

Future schema changes can be added as numbered migration files:

- `002_add_new_feature.up.sql` - Forward migration
- `002_add_new_feature.down.sql` - Rollback migration

The migration system tracks applied versions and ensures consistency.

## Implementation Details

### Provider Interface

Both YAML and SQLite providers implement the same `ConfigProvider` interface:

```go
type ConfigProvider interface {
    LoadConfig() (*ConfigData, error)
    GetDevices() ([]DeviceData, error)
    GetStorageConfig() (*StorageData, error)
    GetControllers() ([]ControllerData, error)
    IsReadOnly() bool
    Close() error
}
```

### Data Structures

The configuration uses separate data structures (`pkg/config`) to avoid coupling with the main application, enabling future flexibility.

### Transaction Safety

All write operations use database transactions to ensure atomicity:

```go
func (s *SQLiteProvider) SaveConfig(configData *ConfigData) error {
    tx, err := s.db.Begin()
    if err != nil {
        return err
    }
    defer tx.Rollback()

    // ... perform operations ...

    return tx.Commit()
}
```

## Benefits and Use Cases

### Development Benefits

1. **Schema validation**: Database constraints prevent invalid configurations
2. **Referential integrity**: Foreign keys ensure device references are valid
3. **Transaction safety**: All-or-nothing configuration updates
4. **Query capabilities**: SQL-based configuration analysis and reporting

### Production Benefits

1. **Centralized management**: Single database for multiple application instances
2. **Backup and restore**: Standard database backup procedures
3. **Audit trails**: Potential for change tracking and history
4. **Performance**: Efficient queries for large configuration datasets

### Migration Path

The implementation provides a smooth migration path:

1. **Phase 1**: Dual support (current) - both YAML and SQLite work
2. **Phase 2**: Gradual adoption - teams can migrate at their own pace
3. **Phase 3**: Future enhancement - additional SQLite-specific features

## File Structure

```
pkg/config/
├── provider.go           # Common interfaces and data structures
├── provider_yaml.go      # YAML file provider
└── provider_sqlite.go    # SQLite database provider

pkg/migrate/
├── migrate.go           # Migration engine
└── file_provider.go     # File-based migration loader

migrations/config/
├── 001_initial_schema.up.sql    # Initial schema
└── 001_initial_schema.down.sql  # Schema rollback

cmd/
├── config-convert/      # YAML to SQLite conversion utility
├── config-test/         # Configuration comparison testing
└── migrate/            # Migration management CLI
```

## Future Enhancements

Potential future improvements:

1. **Web UI**: Configuration management interface
2. **Multi-tenancy**: Support for multiple organization configs
3. **Change tracking**: Audit log for configuration changes
4. **Import/Export**: Additional format support
5. **Validation**: Enhanced configuration validation rules
6. **Replication**: Multi-instance configuration synchronization

## Testing

The implementation includes comprehensive testing:

- Unit tests for provider functionality
- Integration tests for database operations
- Comparison tests for YAML/SQLite equivalence
- Migration tests for schema evolution

Run tests with:
```bash
go test ./pkg/config/...
go test ./pkg/migrate/...
```

## Security Considerations

- SQLite files should have appropriate filesystem permissions
- Database connections use parameterized queries to prevent SQL injection
- Schema migrations are transaction-safe to prevent corruption
- Configuration data may contain sensitive information (API keys, passwords)

## Performance

- SQLite provides excellent read performance for configuration data
- Write operations are transaction-safe but slightly slower than file operations
- Database file size is minimal for typical configuration volumes
- No external dependencies (pure Go SQLite driver) 