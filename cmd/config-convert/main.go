package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/chrissnell/remoteweather/pkg/config"
	"github.com/chrissnell/remoteweather/pkg/migrate"
	_ "modernc.org/sqlite"
)

func main() {
	var (
		yamlFile      = flag.String("yaml", "", "Path to YAML configuration file (required)")
		sqliteFile    = flag.String("sqlite", "", "Path to SQLite database file (required)")
		migrationsDir = flag.String("migrations-dir", "", "Path to migrations directory (default: auto-detect)")
		force         = flag.Bool("force", false, "Overwrite existing SQLite database")
		dryRun        = flag.Bool("dry-run", false, "Show what would be done without executing")
	)
	flag.Parse()

	if *yamlFile == "" || *sqliteFile == "" {
		fmt.Fprintf(os.Stderr, "Usage: %s -yaml <config.yaml> -sqlite <config.db>\n", os.Args[0])
		flag.PrintDefaults()
		os.Exit(1)
	}

	// Auto-detect migrations directory if not specified
	if *migrationsDir == "" {
		*migrationsDir = detectMigrationsDir()
	}

	// Check if YAML file exists
	if _, err := os.Stat(*yamlFile); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: YAML file does not exist: %s\n", *yamlFile)
		os.Exit(1)
	}

	// Check if migrations directory exists
	if _, err := os.Stat(*migrationsDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "Error: Migrations directory does not exist: %s\n", *migrationsDir)
		fmt.Fprintf(os.Stderr, "Use -migrations-dir to specify the correct path\n")
		os.Exit(1)
	}

	// Check if SQLite file already exists
	if _, err := os.Stat(*sqliteFile); err == nil && !*force {
		fmt.Fprintf(os.Stderr, "Error: SQLite file already exists: %s\n", *sqliteFile)
		fmt.Fprintf(os.Stderr, "Use -force to overwrite or choose a different filename\n")
		os.Exit(1)
	}

	fmt.Printf("Converting YAML configuration to SQLite...\n")
	fmt.Printf("  Source: %s\n", *yamlFile)
	fmt.Printf("  Target: %s\n", *sqliteFile)
	fmt.Printf("  Migrations: %s\n", *migrationsDir)

	if *dryRun {
		fmt.Println("DRY RUN - No changes will be made")
	}

	// Load YAML configuration
	fmt.Printf("Loading YAML configuration...\n")
	yamlProvider := config.NewYAMLProvider(*yamlFile)
	configData, err := yamlProvider.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading YAML configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("  Loaded %d devices, %d controllers\n", len(configData.Devices), len(configData.Controllers))

	if *dryRun {
		printConfigSummary(configData)
		fmt.Println("DRY RUN complete - no database created")
		return
	}

	// Remove existing SQLite file if force is specified
	if *force {
		if err := os.Remove(*sqliteFile); err != nil && !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Error removing existing SQLite file: %v\n", err)
			os.Exit(1)
		}
	}

	// Create and initialize SQLite database
	fmt.Printf("Creating SQLite database...\n")
	err = createSQLiteDatabase(*sqliteFile, *migrationsDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating SQLite database: %v\n", err)
		os.Exit(1)
	}

	// Load configuration into SQLite database
	fmt.Printf("Loading configuration into SQLite database...\n")
	err = loadConfigIntoSQLite(*sqliteFile, configData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration into SQLite: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Conversion completed successfully!\n")
	fmt.Printf("You can now use the SQLite backend with: -config-backend sqlite -config %s\n", *sqliteFile)
}

// detectMigrationsDir attempts to find the migrations directory in common locations
func detectMigrationsDir() string {
	// List of possible locations in order of preference
	candidates := []string{
		"migrations/config",                                // Development/source directory
		"/usr/share/remoteweather/migrations/config",       // AUR package location
		"/usr/local/share/remoteweather/migrations/config", // Alternative system location
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Default fallback
	return "migrations/config"
}

func createSQLiteDatabase(dbPath string, migrationsDir string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// Open database connection
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Run database migrations using the specified directory
	provider := migrate.NewFileProvider(migrationsDir, "schema_migrations")
	migrator := migrate.NewMigrator(db, provider)
	return migrator.MigrateUp()
}

func loadConfigIntoSQLite(dbPath string, configData *config.ConfigData) error {
	// Create SQLite provider (which will open the database)
	sqliteProvider, err := config.NewSQLiteProvider(dbPath)
	if err != nil {
		return fmt.Errorf("failed to create SQLite provider: %w", err)
	}
	defer sqliteProvider.Close()

	// Insert configuration data
	err = insertConfigData(sqliteProvider, configData)
	if err != nil {
		return fmt.Errorf("failed to insert configuration data: %w", err)
	}

	return nil
}

func insertConfigData(provider *config.SQLiteProvider, configData *config.ConfigData) error {
	fmt.Printf("  Inserting %d devices...\n", len(configData.Devices))
	fmt.Printf("  Inserting storage configuration...\n")
	fmt.Printf("  Inserting %d controllers...\n", len(configData.Controllers))

	// Use the SaveConfig method to insert all data
	err := provider.SaveConfig(configData)
	if err != nil {
		return fmt.Errorf("failed to save configuration: %w", err)
	}

	fmt.Printf("  Configuration successfully inserted into database\n")
	return nil
}

func printConfigSummary(configData *config.ConfigData) {
	fmt.Println("\nConfiguration Summary:")
	fmt.Printf("Devices (%d):\n", len(configData.Devices))
	for _, device := range configData.Devices {
		fmt.Printf("  - %s (%s)\n", device.Name, device.Type)
	}

	fmt.Printf("\nStorage Backends:\n")
	if configData.Storage.TimescaleDB != nil {
		fmt.Printf("  - TimescaleDB: %s\n", configData.Storage.TimescaleDB.ConnectionString)
	}
	if configData.Storage.GRPC != nil {
		fmt.Printf("  - gRPC: %s:%d\n", configData.Storage.GRPC.ListenAddr, configData.Storage.GRPC.Port)
	}
	if configData.Storage.APRS != nil {
		fmt.Printf("  - APRS: %s\n", configData.Storage.APRS.Callsign)
	}

	fmt.Printf("\nControllers (%d):\n", len(configData.Controllers))
	for _, controller := range configData.Controllers {
		fmt.Printf("  - %s\n", controller.Type)
	}
}
