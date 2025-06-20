# Generic Database Migration System

This package provides a flexible, generic database migration system that can be used for any database schema migrations in the remoteweather project.

## Features

- **Generic Interface**: Works with SQLite, PostgreSQL, and other databases
- **Transaction Safety**: All migrations run in transactions
- **Bidirectional**: Supports both up and down migrations
- **File-based**: Migrations are stored as SQL files in directories
- **CLI Tool**: Includes a command-line utility for running migrations

## Architecture

The system consists of three main components:

1. **Migration Interface** (`migrate.go`): Core migration logic and interfaces
2. **File Provider** (`file_provider.go`): Loads migrations from filesystem
3. **CLI Tool** (`cmd/migrate/main.go`): Command-line interface

## Usage

### Migration Files

Create migration files in the format:
```
migrations/
├── config/
│   ├── 001_initial_schema.up.sql
│   ├── 001_initial_schema.down.sql
│   ├── 002_add_feature.up.sql
│   └── 002_add_feature.down.sql
└── weather/
    ├── 001_create_tables.up.sql
    └── 001_create_tables.down.sql
```

### CLI Commands

```bash
# Build the migration tool
go build -o bin/migrate cmd/migrate/main.go

# Check migration status
./bin/migrate -dsn config.db -dir migrations/config -command status

# Run all pending migrations
./bin/migrate -dsn config.db -dir migrations/config -command up

# Rollback to specific version
./bin/migrate -dsn config.db -dir migrations/config -command down -target 5

# Migrate to specific version (up or down)
./bin/migrate -dsn config.db -dir migrations/config -command to -target 3

# Show current version
./bin/migrate -dsn config.db -dir migrations/config -command version
```

### Programmatic Usage

```go
import (
    "database/sql"
    "github.com/chrissnell/remoteweather/pkg/migrate"
    _ "modernc.org/sqlite"
)

func main() {
    // Open database
    db, err := sql.Open("sqlite", "config.db")
    if err != nil {
        panic(err)
    }
    defer db.Close()

    // Create migration provider
    provider := migrate.NewFileProvider("migrations/config", "config_migrations")
    migrator := migrate.NewMigrator(db, provider)

    // Run migrations
    if err := migrator.MigrateUp(); err != nil {
        panic(err)
    }
}
```

## Configuration Database Schema

The initial migration creates the following tables for configuration management:

- `configs`: Main configuration table
- `devices`: Device configurations
- `storage_configs`: Storage backend configurations
- `controller_configs`: Controller configurations
- `weather_site_configs`: Weather site configurations

## Future Extensions

This system can be extended for:
- Weather database schema migrations
- TimescaleDB schema updates
- Adding new storage backends
- Any other database schema needs

## Future Considerations: Release Schema Version Management

**TODO**: We need to implement release-to-schema version tracking for production deployments:

### Current Capabilities:
- ✅ Track current schema version in database
- ✅ Query current version programmatically
- ✅ Migrate to specific target versions
- ✅ Detect pending migrations

### Still Needed:
- ❌ **Release compatibility matrix**: Map application versions to required schema versions
- ❌ **Startup version checking**: Verify schema compatibility on application startup
- ❌ **Automatic migration**: Optionally auto-migrate to required version during startup
- ❌ **Version requirement enforcement**: Prevent app startup with incompatible schema

### Implementation Ideas:
```go
// Example: Version requirement checking
const RequiredSchemaVersion = 5

func main() {
    migrator := migrate.NewMigrator(db, provider)
    currentVersion, _ := migrator.GetCurrentVersion()
    
    if currentVersion < RequiredSchemaVersion {
        log.Fatalf("Schema version %d required, current: %d. Run migrations.", 
            RequiredSchemaVersion, currentVersion)
    }
    
    // Or auto-migrate:
    // migrator.MigrateTo(RequiredSchemaVersion)
}
```

This would ensure that:
- Each release specifies its minimum required schema version
- Applications fail fast with clear error messages if schema is outdated
- Optional automatic migration reduces deployment friction
- Schema versions are tied to specific application releases

## Security

- All migrations run in transactions for safety
- Version tracking prevents duplicate migrations
- Rollback capability for safe testing
- File-based migrations are easy to review and version control 