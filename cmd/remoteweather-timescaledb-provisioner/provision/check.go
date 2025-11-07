package provision

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Config holds the provisioning configuration
type Config struct {
	PostgresHost     string
	PostgresPort     int
	PostgresAdmin    string
	PostgresPassword string
	DBName           string
	DBUser           string
	DBPassword       string
	SSLMode          string
	Timezone         string
	ConfigDBPath     string
}

// PreflightChecks runs all pre-flight validation checks
func PreflightChecks(cfg *Config) error {
	fmt.Println("🔍 Pre-flight Checks")

	// Check PostgreSQL connection
	if err := checkPostgreSQLConnection(cfg); err != nil {
		return fmt.Errorf("❌ PostgreSQL connection failed: %w", err)
	}
	fmt.Println("✅ PostgreSQL connection successful")

	// Check TimescaleDB extension availability
	if err := checkTimescaleDBAvailable(cfg); err != nil {
		return fmt.Errorf("❌ TimescaleDB extension not available: %w", err)
	}
	fmt.Println("✅ TimescaleDB extension available")

	// Check config.db exists
	if err := checkConfigDB(cfg.ConfigDBPath); err != nil {
		return fmt.Errorf("❌ Config database check failed: %w", err)
	}
	fmt.Printf("✅ Config database found: %s\n", cfg.ConfigDBPath)

	// Check for existing database/user conflicts
	conflicts, err := checkExistingResources(cfg)
	if err != nil {
		return fmt.Errorf("❌ Failed to check existing resources: %w", err)
	}
	if conflicts {
		return fmt.Errorf("❌ Database or user already exists")
	}
	fmt.Println("✅ No existing database/user conflicts")

	fmt.Println()
	return nil
}

// checkPostgreSQLConnection verifies PostgreSQL is accessible
func checkPostgreSQLConnection(cfg *Config) error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return err
	}

	// Get PostgreSQL version
	var version string
	err = db.QueryRow("SELECT version()").Scan(&version)
	if err != nil {
		return err
	}

	return nil
}

// checkTimescaleDBAvailable checks if TimescaleDB extension is available
func checkTimescaleDBAvailable(cfg *Config) error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	var available bool
	query := "SELECT EXISTS(SELECT 1 FROM pg_available_extensions WHERE name = 'timescaledb')"
	err = db.QueryRow(query).Scan(&available)
	if err != nil {
		return err
	}

	if !available {
		return fmt.Errorf("timescaledb extension not found in pg_available_extensions")
	}

	return nil
}

// checkConfigDB verifies the config database exists and is accessible
func checkConfigDB(path string) error {
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("config database not found at %s", path)
		}
		return err
	}

	// Try to open and query the database
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return fmt.Errorf("failed to open config database: %w", err)
	}
	defer db.Close()

	// Verify configs table exists
	var tableName string
	err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='configs'").Scan(&tableName)
	if err == sql.ErrNoRows {
		return fmt.Errorf("configs table not found in database - is this a valid remoteweather config.db?")
	}
	if err != nil {
		return fmt.Errorf("failed to query database: %w", err)
	}

	return nil
}

// checkExistingResources checks if database or user already exists
func checkExistingResources(cfg *Config) (bool, error) {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return false, err
	}
	defer db.Close()

	// Check for existing database
	var dbExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.DBName).Scan(&dbExists)
	if err != nil {
		return false, err
	}

	// Check for existing user
	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", cfg.DBUser).Scan(&userExists)
	if err != nil {
		return false, err
	}

	if dbExists {
		fmt.Printf("⚠️  Database '%s' already exists\n", cfg.DBName)
	}
	if userExists {
		fmt.Printf("⚠️  User '%s' already exists\n", cfg.DBUser)
	}

	return dbExists || userExists, nil
}

// TestConnection tests the newly created database connection
func TestConnection(cfg *Config) error {
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	// Verify TimescaleDB extension is enabled
	var extExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_extension WHERE extname = 'timescaledb')").Scan(&extExists)
	if err != nil {
		return fmt.Errorf("failed to check TimescaleDB extension: %w", err)
	}
	if !extExists {
		return fmt.Errorf("TimescaleDB extension not enabled")
	}

	// Test table creation permission
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS _provisioner_test (id SERIAL PRIMARY KEY)")
	if err != nil {
		return fmt.Errorf("failed to create test table: %w", err)
	}
	_, err = db.Exec("DROP TABLE IF EXISTS _provisioner_test")
	if err != nil {
		return fmt.Errorf("failed to drop test table: %w", err)
	}

	return nil
}
