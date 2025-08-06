package migrate

import (
	"database/sql"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// FileProvider loads migrations from filesystem
type FileProvider struct {
	dir            string
	migrationTable string
	dbDriver       string // "sqlite" or "postgres"
}

// NewFileProvider creates a new file-based migration provider
func NewFileProvider(dir string, migrationTable string) *FileProvider {
	if migrationTable == "" {
		migrationTable = "schema_migrations"
	}
	return &FileProvider{
		dir:            dir,
		migrationTable: migrationTable,
		dbDriver:       "sqlite", // Default to sqlite for backward compatibility
	}
}

// NewFileProviderWithDriver creates a new file-based migration provider with specific driver
func NewFileProviderWithDriver(dir string, migrationTable string, dbDriver string) *FileProvider {
	if migrationTable == "" {
		migrationTable = "schema_migrations"
	}
	return &FileProvider{
		dir:            dir,
		migrationTable: migrationTable,
		dbDriver:       dbDriver,
	}
}

// GetMigrations loads all migrations from the filesystem
func (fp *FileProvider) GetMigrations() ([]Migration, error) {
	var migrations []Migration
	migrationFiles := make(map[int]*Migration)

	// Regular expression to match migration files
	// Format: 001_migration_name.up.sql or 001_migration_name.down.sql
	upRegex := regexp.MustCompile(`^(\d+)_(.+)\.up\.sql$`)
	downRegex := regexp.MustCompile(`^(\d+)_(.+)\.down\.sql$`)

	err := filepath.WalkDir(fp.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		filename := d.Name()

		// Check for up migration
		if matches := upRegex.FindStringSubmatch(filename); matches != nil {
			version, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("invalid version number in file %s: %w", filename, err)
			}

			name := strings.ReplaceAll(matches[2], "_", " ")

			// Read file content
			content, err := fs.ReadFile(os.DirFS(filepath.Dir(path)), filename)
			if err != nil {
				return fmt.Errorf("failed to read migration file %s: %w", path, err)
			}

			if migrationFiles[version] == nil {
				migrationFiles[version] = &Migration{
					Version: version,
					Name:    name,
				}
			}
			migrationFiles[version].Up = string(content)
		}

		// Check for down migration
		if matches := downRegex.FindStringSubmatch(filename); matches != nil {
			version, err := strconv.Atoi(matches[1])
			if err != nil {
				return fmt.Errorf("invalid version number in file %s: %w", filename, err)
			}

			name := strings.ReplaceAll(matches[2], "_", " ")

			// Read file content
			content, err := fs.ReadFile(os.DirFS(filepath.Dir(path)), filename)
			if err != nil {
				return fmt.Errorf("failed to read migration file %s: %w", path, err)
			}

			if migrationFiles[version] == nil {
				migrationFiles[version] = &Migration{
					Version: version,
					Name:    name,
				}
			}
			migrationFiles[version].Down = string(content)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to read migration directory %s: %w", fp.dir, err)
	}

	// Convert map to slice
	for _, migration := range migrationFiles {
		migrations = append(migrations, *migration)
	}

	// Sort by version
	sort.Slice(migrations, func(i, j int) bool {
		return migrations[i].Version < migrations[j].Version
	})

	return migrations, nil
}

// CreateMigrationTable creates the migration tracking table
func (fp *FileProvider) CreateMigrationTable(db *sql.DB) error {
	var query string
	
	if fp.dbDriver == "postgres" {
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				version INTEGER PRIMARY KEY,
				applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
			)
		`, fp.migrationTable)
	} else {
		// SQLite
		query = fmt.Sprintf(`
			CREATE TABLE IF NOT EXISTS %s (
				version INTEGER PRIMARY KEY,
				applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
			)
		`, fp.migrationTable)
	}

	_, err := db.Exec(query)
	if err != nil {
		return fmt.Errorf("failed to create migration table: %w", err)
	}

	return nil
}

// GetCurrentVersion returns the highest applied migration version
func (fp *FileProvider) GetCurrentVersion(db *sql.DB) (int, error) {
	query := fmt.Sprintf("SELECT COALESCE(MAX(version), 0) FROM %s", fp.migrationTable)

	var version int
	err := db.QueryRow(query).Scan(&version)
	if err != nil {
		return 0, fmt.Errorf("failed to get current version: %w", err)
	}

	return version, nil
}

// SetVersion sets the migration version
func (fp *FileProvider) SetVersion(db DB, version int) error {
	var query string
	var err error

	if version == 0 {
		// Delete all version records when rolling back to 0
		query = fmt.Sprintf("DELETE FROM %s", fp.migrationTable)
		_, err = db.Exec(query)
	} else {
		if fp.dbDriver == "postgres" {
			// PostgreSQL uses ON CONFLICT for upsert
			query = fmt.Sprintf(`
				INSERT INTO %s (version, applied_at) 
				VALUES ($1, CURRENT_TIMESTAMP)
				ON CONFLICT (version) DO UPDATE SET applied_at = CURRENT_TIMESTAMP
			`, fp.migrationTable)
		} else {
			// SQLite uses INSERT OR REPLACE
			query = fmt.Sprintf(`
				INSERT OR REPLACE INTO %s (version, applied_at) 
				VALUES (?, CURRENT_TIMESTAMP)
			`, fp.migrationTable)
		}
		_, err = db.Exec(query, version)
	}

	if err != nil {
		return fmt.Errorf("failed to set version: %w", err)
	}

	return nil
}
