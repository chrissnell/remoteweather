package provision

import (
	"database/sql"
	"fmt"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// CreateUser creates the database user with generated password and grants privileges
func CreateUser(cfg *Config) error {
	fmt.Println("👤 Creating User")

	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	// Create user with password
	createUserSQL := fmt.Sprintf("CREATE USER %s WITH PASSWORD '%s'", cfg.DBUser, cfg.DBPassword)
	_, err = db.Exec(createUserSQL)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	fmt.Printf("✅ User '%s' created\n", cfg.DBUser)

	// Grant database privileges
	if err := grantPrivileges(cfg); err != nil {
		return err
	}

	fmt.Println()
	return nil
}

// grantPrivileges grants all necessary privileges to the database user
func grantPrivileges(cfg *Config) error {
	// Connect to postgres database to grant database-level privileges
	connStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=postgres sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.SSLMode)

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to connect to PostgreSQL: %w", err)
	}
	defer db.Close()

	// Grant all privileges on database
	grantDBSQL := fmt.Sprintf("GRANT ALL PRIVILEGES ON DATABASE %s TO %s", cfg.DBName, cfg.DBUser)
	_, err = db.Exec(grantDBSQL)
	if err != nil {
		return fmt.Errorf("failed to grant database privileges: %w", err)
	}

	fmt.Printf("✅ Database privileges granted\n")

	// Connect to the target database to grant schema and table privileges
	dbConnStr := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.PostgresHost, cfg.PostgresPort, cfg.PostgresAdmin, cfg.PostgresPassword, cfg.DBName, cfg.SSLMode)

	targetDB, err := sql.Open("pgx", dbConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to target database: %w", err)
	}
	defer targetDB.Close()

	// Grant schema privileges
	grantSchemaSQL := fmt.Sprintf("GRANT ALL ON SCHEMA public TO %s", cfg.DBUser)
	_, err = targetDB.Exec(grantSchemaSQL)
	if err != nil {
		return fmt.Errorf("failed to grant schema privileges: %w", err)
	}

	// Grant default privileges for future tables
	grantDefaultTablesSQL := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON TABLES TO %s", cfg.DBUser)
	_, err = targetDB.Exec(grantDefaultTablesSQL)
	if err != nil {
		return fmt.Errorf("failed to grant default table privileges: %w", err)
	}

	// Grant default privileges for future sequences
	grantDefaultSeqSQL := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON SEQUENCES TO %s", cfg.DBUser)
	_, err = targetDB.Exec(grantDefaultSeqSQL)
	if err != nil {
		return fmt.Errorf("failed to grant default sequence privileges: %w", err)
	}

	// Grant default privileges for future functions
	grantDefaultFuncSQL := fmt.Sprintf("ALTER DEFAULT PRIVILEGES IN SCHEMA public GRANT ALL ON FUNCTIONS TO %s", cfg.DBUser)
	_, err = targetDB.Exec(grantDefaultFuncSQL)
	if err != nil {
		return fmt.Errorf("failed to grant default function privileges: %w", err)
	}

	fmt.Printf("✅ Schema and default privileges granted\n")

	return nil
}
