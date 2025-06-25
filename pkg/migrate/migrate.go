package migrate

import (
	"database/sql"
	"fmt"
	"sort"
	"time"
)

// Migration represents a single database migration
type Migration struct {
	Version int
	Name    string
	Up      string
	Down    string
}

// DB represents either a database connection or transaction
type DB interface {
	Exec(query string, args ...interface{}) (sql.Result, error)
	QueryRow(query string, args ...interface{}) *sql.Row
}

// MigrationProvider defines how migrations are loaded and managed
type MigrationProvider interface {
	GetMigrations() ([]Migration, error)
	GetCurrentVersion(db *sql.DB) (int, error)
	SetVersion(db DB, version int) error
	CreateMigrationTable(db *sql.DB) error
}

// Migrator handles the execution of migrations
type Migrator struct {
	db       *sql.DB
	provider MigrationProvider
}

// NewMigrator creates a new migrator instance
func NewMigrator(db *sql.DB, provider MigrationProvider) *Migrator {
	return &Migrator{
		db:       db,
		provider: provider,
	}
}

// MigrateUp runs all pending migrations up to the latest version
func (m *Migrator) MigrateUp() error {
	return m.MigrateTo(-1) // -1 means migrate to latest
}

// MigrateDown runs down migrations to revert to a specific version
func (m *Migrator) MigrateDown(targetVersion int) error {
	currentVersion, err := m.provider.GetCurrentVersion(m.db)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	if targetVersion >= currentVersion {
		return fmt.Errorf("target version %d must be less than current version %d", targetVersion, currentVersion)
	}

	migrations, err := m.provider.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	// Sort migrations by version descending for rollback
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version > migrations[j].Version
	})

	for _, migration := range migrations {
		if migration.Version > targetVersion && migration.Version <= currentVersion {
			if err := m.executeMigration(migration, false); err != nil {
				return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// MigrateTo runs migrations up or down to reach a specific version
func (m *Migrator) MigrateTo(targetVersion int) error {
	// Ensure migration table exists
	if err := m.provider.CreateMigrationTable(m.db); err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	currentVersion, err := m.provider.GetCurrentVersion(m.db)
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	migrations, err := m.provider.GetMigrations()
	if err != nil {
		return fmt.Errorf("failed to get migrations: %w", err)
	}

	// Sort migrations by version ascending
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	// Determine target version if -1 (latest)
	if targetVersion == -1 && len(migrations) > 0 {
		targetVersion = migrations[len(migrations)-1].Version
	}

	if targetVersion < currentVersion {
		return m.MigrateDown(targetVersion)
	}

	// Run up migrations
	for _, migration := range migrations {
		if migration.Version > currentVersion && migration.Version <= targetVersion {
			if err := m.executeMigration(migration, true); err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}

// GetCurrentVersion returns the current migration version
func (m *Migrator) GetCurrentVersion() (int, error) {
	if err := m.provider.CreateMigrationTable(m.db); err != nil {
		return 0, fmt.Errorf("failed to create migration table: %w", err)
	}
	return m.provider.GetCurrentVersion(m.db)
}

// GetPendingMigrations returns migrations that haven't been applied yet
func (m *Migrator) GetPendingMigrations() ([]Migration, error) {
	currentVersion, err := m.GetCurrentVersion()
	if err != nil {
		return nil, err
	}

	migrations, err := m.provider.GetMigrations()
	if err != nil {
		return nil, err
	}

	var pending []Migration
	for _, migration := range migrations {
		if migration.Version > currentVersion {
			pending = append(pending, migration)
		}
	}

	// Sort by version ascending
	sort.Slice(pending, func(i, j int) bool {
		return pending[i].Version < pending[j].Version
	})

	return pending, nil
}

// executeMigration runs a single migration up or down
func (m *Migrator) executeMigration(migration Migration, up bool) error {
	var sql string
	if up {
		sql = migration.Up
	} else {
		sql = migration.Down
	}

	if sql == "" {
		return fmt.Errorf("migration %d has no %s SQL", migration.Version, map[bool]string{true: "up", false: "down"}[up])
	}

	// Start transaction
	tx, err := m.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Execute migration
	if _, err := tx.Exec(sql); err != nil {
		return fmt.Errorf("failed to execute migration SQL: %w", err)
	}

	// Update version
	var newVersion int
	if up {
		newVersion = migration.Version
	} else {
		newVersion = migration.Version - 1
	}

	if err := m.provider.SetVersion(tx, newVersion); err != nil {
		return fmt.Errorf("failed to update migration version: %w", err)
	}

	// Commit transaction
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit migration transaction: %w", err)
	}

	direction := "up"
	if !up {
		direction = "down"
	}
	fmt.Printf("Applied migration %d (%s) %s at %s\n", migration.Version, migration.Name, direction, time.Now().Format(time.RFC3339))

	return nil
}

// SetVersion allows manually setting the migration version (use with caution)
func (m *Migrator) SetVersion(version int) error {
	return m.provider.SetVersion(m.db, version)
}
