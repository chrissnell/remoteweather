package provision

import (
	"database/sql"
	"fmt"

	"github.com/chrissnell/remoteweather/pkg/config"
	_ "github.com/jackc/pgx/v5/stdlib"
	_ "modernc.org/sqlite"
)

// Config holds the provisioning configuration
type Config struct {
	PostgresHost     string
	PostgresPort     int
	PostgresAdmin    string
	PostgresPassword string
	UsePeerAuth      bool   // Use peer authentication instead of password
	DBName           string
	DBUser           string
	DBPassword       string
	SSLMode          string
	Timezone         string
	ConfigDBPath     string
}

// BuildConnString builds a PostgreSQL connection string based on config
func (cfg *Config) BuildConnString(dbname string) string {
	if cfg.UsePeerAuth {
		// Peer authentication - use Unix socket, no password needed
		// Omitting host makes pgx use Unix socket with peer auth
		return fmt.Sprintf("user=%s dbname=%s sslmode=disable", cfg.PostgresAdmin, dbname)
	}
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, dbname, cfg.SSLMode)
}

// PreflightChecks runs all pre-flight validation checks
func PreflightChecks(cfg *Config) error {
	fmt.Println("🔍 Pre-flight Checks")

	// Check PostgreSQL connection
	err := checkPostgreSQLConnection(cfg)
	if err != nil {
		// Check if it's an authentication error that we can fix
		if IsAuthError(err) {
			fmt.Printf("⚠️  Connection failed: %v\n", err)

			// Offer to auto-fix pg_hba.conf
			if err := AutoFixPgHba(cfg); err != nil {
				return fmt.Errorf("❌ %w", err)
			}

			// If we get here, auto-fix succeeded
			// Connection test happens inside AutoFixPgHba
		} else {
			return fmt.Errorf("❌ PostgreSQL connection failed: %w", err)
		}
	} else {
		fmt.Println("✅ PostgreSQL connection successful")
	}

	// Check TimescaleDB extension availability
	if err := checkTimescaleDBAvailable(cfg); err != nil {
		return fmt.Errorf("❌ TimescaleDB extension not available: %w", err)
	}
	fmt.Println("✅ TimescaleDB extension available")

	// Check/create config.db
	if err := checkConfigDB(cfg.ConfigDBPath); err != nil {
		return fmt.Errorf("❌ Config database check failed: %w", err)
	}
	fmt.Printf("✅ Config database ready: %s\n", cfg.ConfigDBPath)

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
	connStr := cfg.BuildConnString("postgres")

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
	connStr := cfg.BuildConnString("postgres")

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
// If it doesn't exist, it will be created with the default schema
func checkConfigDB(path string) error {
	// Use the config package to open/create the database
	// This will automatically initialize the schema if the database doesn't exist
	provider, err := config.NewSQLiteProvider(path)
	if err != nil {
		return fmt.Errorf("failed to open/create config database: %w", err)
	}
	defer provider.Close()

	// Verify we can load the config (validates schema is correct)
	_, err = provider.LoadConfig()
	if err != nil {
		return fmt.Errorf("failed to load config from database: %w", err)
	}

	return nil
}

// checkExistingResources checks if database or user already exists
func checkExistingResources(cfg *Config) (bool, error) {
	connStr := cfg.BuildConnString("postgres")

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

// DropExistingResources drops the database and user if they exist
func DropExistingResources(cfg *Config) error {
	connStr := cfg.BuildConnString("postgres")

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	// Check for existing database
	var dbExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = $1)", cfg.DBName).Scan(&dbExists)
	if err != nil {
		return err
	}

	// Check for existing user
	var userExists bool
	err = db.QueryRow("SELECT EXISTS(SELECT 1 FROM pg_roles WHERE rolname = $1)", cfg.DBUser).Scan(&userExists)
	if err != nil {
		return err
	}

	// Drop database if exists
	if dbExists {
		fmt.Printf("🗑️  Dropping database '%s'...\n", cfg.DBName)
		
		// Terminate existing connections to the database
		terminateQuery := `
			SELECT pg_terminate_backend(pg_stat_activity.pid)
			FROM pg_stat_activity
			WHERE pg_stat_activity.datname = $1
			AND pid <> pg_backend_pid()
		`
		_, err = db.Exec(terminateQuery, cfg.DBName)
		if err != nil {
			return fmt.Errorf("failed to terminate connections: %w", err)
		}

		// Drop the database
		dropDBQuery := fmt.Sprintf("DROP DATABASE IF EXISTS %s", cfg.DBName)
		_, err = db.Exec(dropDBQuery)
		if err != nil {
			return fmt.Errorf("failed to drop database: %w", err)
		}
		fmt.Printf("✅ Database '%s' dropped\n", cfg.DBName)
	}

	// Drop user if exists
	if userExists {
		fmt.Printf("🗑️  Dropping user '%s'...\n", cfg.DBUser)
		dropUserQuery := fmt.Sprintf("DROP USER IF EXISTS %s", cfg.DBUser)
		_, err = db.Exec(dropUserQuery)
		if err != nil {
			return fmt.Errorf("failed to drop user: %w", err)
		}
		fmt.Printf("✅ User '%s' dropped\n", cfg.DBUser)
	}

	if !dbExists && !userExists {
		fmt.Println("ℹ️  No existing database or user to drop")
	}

	return nil
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
