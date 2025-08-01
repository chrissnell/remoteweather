// Package main provides a command-line database migration tool.
package main

import (

	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strconv"

	"github.com/chrissnell/remoteweather/pkg/migrate"
	_ "modernc.org/sqlite" // SQLite driver
	"runtime"
)

// getDefaultMigrationDir returns the OS-specific default migration directory
func getDefaultMigrationDir() string {
	switch runtime.GOOS {
	case "linux":
		return "/usr/share/remoteweather/migrations/config"
	case "darwin":
		return "/usr/local/share/remoteweather/migrations/config"
	case "windows":
		return "C:\\ProgramData\\remoteweather\\migrations\\config"
	default:
		return "migrations"
	}
}

func main() {
	var (
		dbDriver       = flag.String("driver", "sqlite", "Database driver (sqlite, postgres)")
		dbDSN          = flag.String("dsn", "", "Database connection string")
		migrationDir   = flag.String("dir", getDefaultMigrationDir(), "Migration directory")
		migrationTable = flag.String("table", "schema_migrations", "Migration table name")
		command        = flag.String("command", "up", "Migration command: up, down, version, status")
		targetVersion  = flag.String("target", "", "Target version for down/to commands")
		helpFlag       = flag.Bool("help", false, "Show help")
	)

	flag.Parse()

	if *helpFlag {
		showHelp()
		return
	}

	if *dbDSN == "" {
		fmt.Fprintf(os.Stderr, "Error: -dsn flag is required\n")
		showHelp()
		os.Exit(1)
	}

	// Open database connection
	db, err := sql.Open(*dbDriver, *dbDSN)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	// Create migration provider
	provider := migrate.NewFileProvider(*migrationDir, *migrationTable)
	migrator := migrate.NewMigrator(db, provider)

	// Execute command
	switch *command {
	case "up":
		if err := migrator.MigrateUp(); err != nil {
			log.Fatalf("Migration up failed: %v", err)
		}
	case "down":
		if *targetVersion == "" {
			fmt.Fprintf(os.Stderr, "Error: -target flag is required for down command\n")
			os.Exit(1)
		}
		target, err := strconv.Atoi(*targetVersion)
		if err != nil {
			log.Fatalf("Invalid target version: %v", err)
		}
		if err := migrator.MigrateDown(target); err != nil {
			log.Fatalf("Migration down failed: %v", err)
		}
	case "to":
		if *targetVersion == "" {
			fmt.Fprintf(os.Stderr, "Error: -target flag is required for to command\n")
			os.Exit(1)
		}
		target, err := strconv.Atoi(*targetVersion)
		if err != nil {
			log.Fatalf("Invalid target version: %v", err)
		}
		if err := migrator.MigrateTo(target); err != nil {
			log.Fatalf("Migration to target failed: %v", err)
		}
	case "version":
		version, err := migrator.GetCurrentVersion()
		if err != nil {
			log.Fatalf("Failed to get current version: %v", err)
		}
		fmt.Printf("Current version: %d\n", version)
		return
	case "status":
		if err := showStatus(migrator); err != nil {
			log.Fatalf("Status command failed: %v", err)
		}
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", *command)
		showHelp()
		os.Exit(1)
	}

	fmt.Println("Migration completed successfully")
}

func showStatus(migrator *migrate.Migrator) error {
	currentVersion, err := migrator.GetCurrentVersion()
	if err != nil {
		return fmt.Errorf("failed to get current version: %w", err)
	}

	pending, err := migrator.GetPendingMigrations()
	if err != nil {
		return fmt.Errorf("failed to get pending migrations: %w", err)
	}

	fmt.Printf("Current version: %d\n", currentVersion)
	fmt.Printf("Pending migrations: %d\n", len(pending))

	if len(pending) > 0 {
		fmt.Println("\nPending migrations:")
		for _, migration := range pending {
			fmt.Printf("  %d: %s\n", migration.Version, migration.Name)
		}
	}

	return nil
}

func showHelp() {
	fmt.Println("Database Migration Tool")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  migrate [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  -driver string     Database driver (default: sqlite)")
	fmt.Println("  -dsn string        Database connection string (required)")
	fmt.Printf("  -dir string        Migration directory (default: %s)\n", getDefaultMigrationDir())
	fmt.Println("  -table string      Migration table name (default: schema_migrations)")
	fmt.Println("  -command string    Migration command (default: up)")
	fmt.Println("  -target string     Target version for down/to commands")
	fmt.Println("  -help              Show this help message")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  up                 Apply all pending migrations")
	fmt.Println("  down               Roll back to target version")
	fmt.Println("  to                 Migrate to specific version (up or down)")
	fmt.Println("  version            Show current migration version")
	fmt.Println("  status             Show migration status")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  migrate -dsn config.db -command up")
	fmt.Println("  migrate -dsn config.db -command down -target 5")
	fmt.Println("  migrate -dsn config.db -command status")
	fmt.Println("  migrate -dsn config.db -dir migrations/config -table config_migrations")
}
