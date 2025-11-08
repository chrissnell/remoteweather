package provision

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CreateDatabase creates the PostgreSQL database with proper encoding
func CreateDatabase(cfg *Config) error {
	fmt.Println("🗄️  Creating Database")

	connStr := cfg.BuildConnString("postgres")

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	// Create database with UTF8 encoding
	createDBSQL := fmt.Sprintf(`
		CREATE DATABASE %s
		ENCODING 'UTF8'
		LC_COLLATE 'en_US.UTF-8'
		LC_CTYPE 'en_US.UTF-8'
		TEMPLATE template0
	`, cfg.DBName)

	_, err = db.Exec(createDBSQL)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}

	fmt.Printf("✅ Database '%s' created with UTF8 encoding\n", cfg.DBName)
	fmt.Println()
	return nil
}

// EnableTimescaleDB enables the TimescaleDB extension on the database
func EnableTimescaleDB(cfg *Config) error {
	fmt.Println("🔌 Enabling TimescaleDB Extension")

	// Connect to the newly created database
	connStr := cfg.BuildConnString(cfg.DBName)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// Enable TimescaleDB extension
	_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS timescaledb")
	if err != nil {
		return fmt.Errorf("failed to create TimescaleDB extension: %w", err)
	}

	// Verify extension was created
	var version string
	err = db.QueryRow("SELECT extversion FROM pg_extension WHERE extname = 'timescaledb'").Scan(&version)
	if err != nil {
		return fmt.Errorf("failed to verify TimescaleDB extension: %w", err)
	}

	fmt.Printf("✅ TimescaleDB extension enabled (version %s)\n", version)
	fmt.Println()
	return nil
}
