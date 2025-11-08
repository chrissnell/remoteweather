package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/chrissnell/remoteweather/cmd/remoteweather-timescaledb-provisioner/provision"
)

// Color constants
const (
	colorReset         = "\033[0m"
	colorBrightCyan    = "\033[96m"
	colorBrightYellow  = "\033[93m"
	colorBold          = "\033[1m"
)

const (
	DefaultDBName    = "remoteweather"
	DefaultDBUser    = "remoteweather"
	DefaultHost      = "localhost"
	DefaultPort      = 5432
	DefaultSSLMode   = "prefer"
	DefaultTimezone  = "UTC"
	DefaultConfigDB  = "/var/lib/remoteweather/config.db"
	DefaultAdminUser = "postgres"
)

func main() {
	// Check for root privileges
	if os.Geteuid() != 0 {
		fmt.Println("❌ This tool must be run as root")
		fmt.Println()
		fmt.Println("Root access is required to:")
		fmt.Println("  • Modify pg_hba.conf for PostgreSQL access")
		fmt.Println("  • Write to /var/lib/remoteweather/config.db")
		fmt.Println("  • Reload PostgreSQL configuration")
		fmt.Println()
		fmt.Println("Just run:")
		fmt.Printf("  %s%ssudo remoteweather-timescaledb-provisioner init%s\n", colorBold, colorBrightCyan, colorReset)
		fmt.Println()
		fmt.Println("Don't know your PostgreSQL postgres password? No problem!")
		fmt.Println("The tool will automatically configure PostgreSQL for you.")
		fmt.Println()
		os.Exit(1)
	}

	// Define command-line flags
	initCmd := flag.NewFlagSet("init", flag.ExitOnError)
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	testCmd := flag.NewFlagSet("test", flag.ExitOnError)

	// Init command flags
	dbName := initCmd.String("db-name", DefaultDBName, "Database name to create")
	dbUser := initCmd.String("db-user", DefaultDBUser, "Database user to create")
	postgresHost := initCmd.String("postgres-host", DefaultHost, "PostgreSQL host")
	postgresPort := initCmd.Int("postgres-port", DefaultPort, "PostgreSQL port")
	postgresAdmin := initCmd.String("postgres-admin", DefaultAdminUser, "PostgreSQL admin user")
	postgresAdminPassword := initCmd.String("postgres-admin-password", "", "PostgreSQL admin password (or use POSTGRES_ADMIN_PASSWORD env var)")
	sslMode := initCmd.String("ssl-mode", DefaultSSLMode, "SSL mode (disable, require, prefer)")
	timezone := initCmd.String("timezone", DefaultTimezone, "Database timezone")
	configDB := initCmd.String("config-db", DefaultConfigDB, "Path to remoteweather config.db")
	interactive := initCmd.Bool("interactive", false, "Run in interactive mode")
	reprovision := initCmd.Bool("reprovision", false, "Drop existing database and user before provisioning (DESTRUCTIVE)")

	// Status command flags
	statusConfigDB := statusCmd.String("config-db", DefaultConfigDB, "Path to remoteweather config.db")

	// Test command flags
	testConfigDB := testCmd.String("config-db", DefaultConfigDB, "Path to remoteweather config.db")

	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "init":
		initCmd.Parse(os.Args[2:])
		runInit(*dbName, *dbUser, *postgresHost, *postgresPort, *postgresAdmin,
			*postgresAdminPassword, *sslMode, *timezone, *configDB, *interactive, *reprovision)

	case "status":
		statusCmd.Parse(os.Args[2:])
		runStatus(*statusConfigDB)

	case "test":
		testCmd.Parse(os.Args[2:])
		runTest(*testConfigDB)

	default:
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("remoteweather TimescaleDB Provisioner")
	fmt.Println()
	fmt.Println("Usage:")
	fmt.Println("  remoteweather-timescaledb-provisioner init [flags]")
	fmt.Println("  remoteweather-timescaledb-provisioner status [flags]")
	fmt.Println("  remoteweather-timescaledb-provisioner test [flags]")
	fmt.Println()
	fmt.Println("Commands:")
	fmt.Println("  init     Provision TimescaleDB database and user")
	fmt.Println("  status   Show current configuration from config.db")
	fmt.Println("  test     Test database connection")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Standard usage - just run it!")
	fmt.Printf("  %s%ssudo remoteweather-timescaledb-provisioner init%s\n", colorBold, colorBrightCyan, colorReset)
	fmt.Println()
	fmt.Println("  # If you know your postgres password, set it via environment variable")
	fmt.Printf("  %sexport POSTGRES_ADMIN_PASSWORD='yourpassword'%s\n", colorBrightCyan, colorReset)
	fmt.Printf("  %ssudo remoteweather-timescaledb-provisioner init%s\n", colorBrightCyan, colorReset)
	fmt.Println()
	fmt.Println("  # Or provide it via command line flag")
	fmt.Printf("  %ssudo remoteweather-timescaledb-provisioner init --postgres-admin-password yourpassword%s\n", colorBrightCyan, colorReset)
	fmt.Println()
	fmt.Println("  # Don't know the password? No problem!")
	fmt.Println("  # Just run without setting it and the tool will auto-configure PostgreSQL")
	fmt.Println()
	fmt.Println("  # Custom configuration")
	fmt.Println("  remoteweather-timescaledb-provisioner init \\")
	fmt.Println("    --db-name myweatherdb \\")
	fmt.Println("    --postgres-host 192.168.1.100 \\")
	fmt.Println("    --postgres-admin-password secret")
	fmt.Println()
	fmt.Println("  # Re-provision (drop and recreate)")
	fmt.Println("  remoteweather-timescaledb-provisioner init --reprovision")
}

func runInit(dbName, dbUser, postgresHost string, postgresPort int, postgresAdmin, postgresAdminPassword string,
	sslMode, timezone, configDB string, interactive, reprovision bool) {

	fmt.Println("🚀 remoteweather TimescaleDB Provisioner")
	fmt.Println("========================================")
	fmt.Println()

	// Get admin password from env if not provided
	if postgresAdminPassword == "" {
		postgresAdminPassword = os.Getenv("POSTGRES_ADMIN_PASSWORD")
	}

	// Show configuration
	fmt.Println("Configuration:")
	fmt.Printf("  PostgreSQL Host: %s:%d\n", postgresHost, postgresPort)
	fmt.Printf("  Database Name: %s\n", dbName)
	fmt.Printf("  Database User: %s\n", dbUser)
	fmt.Printf("  SSL Mode: %s\n", sslMode)
	fmt.Printf("  Timezone: %s\n", timezone)
	fmt.Printf("  Config DB: %s\n", configDB)
	fmt.Println()

	// Interactive mode - allow customization
	if interactive {
		reader := bufio.NewReader(os.Stdin)
		fmt.Printf("PostgreSQL admin user [%s]: ", postgresAdmin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			postgresAdmin = input
		}
		fmt.Println()
	}

	// Generate password for database user
	dbPassword, err := provision.GeneratePassword(provision.PasswordLength)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to generate password: %v\n", err)
		os.Exit(1)
	}

	// Create config
	// UsePeerAuth will be auto-detected: if password is empty, peer auth will be attempted
	cfg := &provision.Config{
		PostgresHost:     postgresHost,
		PostgresPort:     postgresPort,
		PostgresAdmin:    postgresAdmin,
		PostgresPassword: postgresAdminPassword,
		UsePeerAuth:      postgresAdminPassword == "",
		DBName:           dbName,
		DBUser:           dbUser,
		DBPassword:       dbPassword,
		SSLMode:          sslMode,
		Timezone:         timezone,
		ConfigDBPath:     configDB,
	}

	// Handle reprovision flag
	if reprovision {
		fmt.Println("⚠️  DESTRUCTIVE OPERATION WARNING")
		fmt.Println("=====================================")
		fmt.Println()
		fmt.Printf("This will DROP the following resources if they exist:\n")
		fmt.Printf("  • Database: %s\n", dbName)
		fmt.Printf("  • User: %s\n", dbUser)
		fmt.Println()
		fmt.Println("⚠️  ALL DATA IN THE DATABASE WILL BE PERMANENTLY DELETED")
		fmt.Println()

		reader := bufio.NewReader(os.Stdin)
		fmt.Print("Type 'yes' to confirm you understand and want to proceed: ")
		confirmation, _ := reader.ReadString('\n')
		confirmation = strings.TrimSpace(confirmation)

		if confirmation != "yes" {
			fmt.Println("❌ Operation cancelled")
			os.Exit(0)
		}
		fmt.Println()

		// Drop existing resources
		if err := provision.DropExistingResources(cfg); err != nil {
			fmt.Fprintf(os.Stderr, "❌ Failed to drop existing resources: %v\n", err)
			os.Exit(1)
		}
		fmt.Println()
	}

	// Run pre-flight checks
	if err := provision.PreflightChecks(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	// Create database
	if err := provision.CreateDatabase(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create database: %v\n", err)
		os.Exit(1)
	}

	// Enable TimescaleDB extension
	if err := provision.EnableTimescaleDB(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to enable TimescaleDB: %v\n", err)
		os.Exit(1)
	}

	// Create user and grant privileges
	if err := provision.CreateUser(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to create user: %v\n", err)
		os.Exit(1)
	}

	// Display generated password
	provision.DisplayPasswordWarning(dbPassword)

	// Update config.db
	if err := provision.UpdateConfigDB(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to update config database: %v\n", err)
		os.Exit(1)
	}

	// Test connection
	fmt.Println("🔍 Verifying Connection")
	if err := provision.TestConnection(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Connection test failed: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("✅ Connection verified")
	fmt.Println()

	// Print success message
	fmt.Println("✅ Provisioning Complete!")
	fmt.Println()
	fmt.Println("Connection Details:")
	fmt.Printf("  Host: %s:%d\n", cfg.PostgresHost, cfg.PostgresPort)
	fmt.Printf("  Database: %s\n", cfg.DBName)
	fmt.Printf("  User: %s\n", cfg.DBUser)
	fmt.Printf("  SSL Mode: %s\n", cfg.SSLMode)
	fmt.Println("  TimescaleDB: enabled")
	fmt.Println()
	fmt.Printf("%s%sNext Steps:%s\n", colorBold, colorBrightYellow, colorReset)
	fmt.Println("  1. Start remoteweather:")
	fmt.Printf("     %s%s./remoteweather --config config.db%s\n", colorBold, colorBrightCyan, colorReset)
	fmt.Println()
	fmt.Println("  2. remoteweather will automatically:")
	fmt.Println("     ✓ Connect to TimescaleDB")
	fmt.Println("     ✓ Create all tables and hypertables")
	fmt.Println("     ✓ Set up aggregation policies")
	fmt.Println("     ✓ Run any pending migrations")
	fmt.Println()
	fmt.Println("Manual Connection (if needed):")
	fmt.Printf("  %spsql -h %s -p %d -U %s -d %s%s\n", colorBrightCyan, cfg.PostgresHost, cfg.PostgresPort, cfg.DBUser, cfg.DBName, colorReset)
	fmt.Println()
}

func runStatus(configDB string) {
	fmt.Println("📊 Current TimescaleDB Configuration")
	fmt.Println("====================================")
	fmt.Println()

	cfg, err := provision.GetStorageConfig(configDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to read configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Host:     %s:%d\n", cfg.PostgresHost, cfg.PostgresPort)
	fmt.Printf("Database: %s\n", cfg.DBName)
	fmt.Printf("User:     %s\n", cfg.DBUser)
	fmt.Printf("SSL Mode: %s\n", cfg.SSLMode)
	fmt.Printf("Timezone: %s\n", cfg.Timezone)
	fmt.Println()
}

func runTest(configDB string) {
	fmt.Println("🔍 Testing TimescaleDB Connection")
	fmt.Println("==================================")
	fmt.Println()

	cfg, err := provision.GetStorageConfig(configDB)
	if err != nil {
		fmt.Fprintf(os.Stderr, "❌ Failed to read configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Testing connection to %s:%d/%s...\n", cfg.PostgresHost, cfg.PostgresPort, cfg.DBName)

	if err := provision.TestConnection(cfg); err != nil {
		fmt.Fprintf(os.Stderr, "❌ Connection test failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("✅ Connection successful")
	fmt.Println("✅ TimescaleDB extension is enabled")
	fmt.Println("✅ User has table creation privileges")
	fmt.Println()
}
