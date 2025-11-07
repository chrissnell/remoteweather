package provision

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// UpdateConfigDB updates the remoteweather SQLite config database with TimescaleDB connection details
func UpdateConfigDB(cfg *Config) error {
	fmt.Println("⚙️  Updating Configuration")

	db, err := sql.Open("sqlite", cfg.ConfigDBPath)
	if err != nil {
		return fmt.Errorf("failed to open config database: %w", err)
	}
	defer db.Close()

	// Get the config ID (should be 1 for 'default')
	var configID int64
	err = db.QueryRow("SELECT id FROM configs WHERE name = 'default'").Scan(&configID)
	if err == sql.ErrNoRows {
		// Create default config if it doesn't exist
		result, err := db.Exec("INSERT INTO configs (name) VALUES ('default')")
		if err != nil {
			return fmt.Errorf("failed to create default config: %w", err)
		}
		configID, err = result.LastInsertId()
		if err != nil {
			return fmt.Errorf("failed to get config ID: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to query config ID: %w", err)
	}

	// Check if TimescaleDB storage config already exists
	var existingID int64
	err = db.QueryRow(`
		SELECT id FROM storage_configs
		WHERE config_id = ? AND backend_type = 'timescaledb'
	`, configID).Scan(&existingID)

	if err == sql.ErrNoRows {
		// Insert new TimescaleDB storage config
		insertSQL := `
			INSERT INTO storage_configs (
				config_id, backend_type, enabled,
				timescale_host, timescale_port, timescale_database,
				timescale_user, timescale_password,
				timescale_ssl_mode, timescale_timezone
			) VALUES (?, 'timescaledb', 1, ?, ?, ?, ?, ?, ?, ?)
		`
		_, err = db.Exec(insertSQL,
			configID,
			cfg.PostgresHost,
			cfg.PostgresPort,
			cfg.DBName,
			cfg.DBUser,
			cfg.DBPassword,
			cfg.SSLMode,
			cfg.Timezone,
		)
		if err != nil {
			return fmt.Errorf("failed to insert storage config: %w", err)
		}
	} else if err != nil {
		return fmt.Errorf("failed to check existing storage config: %w", err)
	} else {
		// Update existing TimescaleDB storage config
		updateSQL := `
			UPDATE storage_configs SET
				enabled = 1,
				timescale_host = ?,
				timescale_port = ?,
				timescale_database = ?,
				timescale_user = ?,
				timescale_password = ?,
				timescale_ssl_mode = ?,
				timescale_timezone = ?
			WHERE id = ?
		`
		_, err = db.Exec(updateSQL,
			cfg.PostgresHost,
			cfg.PostgresPort,
			cfg.DBName,
			cfg.DBUser,
			cfg.DBPassword,
			cfg.SSLMode,
			cfg.Timezone,
			existingID,
		)
		if err != nil {
			return fmt.Errorf("failed to update storage config: %w", err)
		}
	}

	fmt.Println("✅ Config database updated with connection details")
	fmt.Println()
	return nil
}

// GetStorageConfig retrieves the current TimescaleDB config from config.db
func GetStorageConfig(configDBPath string) (*Config, error) {
	db, err := sql.Open("sqlite", configDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config database: %w", err)
	}
	defer db.Close()

	cfg := &Config{
		ConfigDBPath: configDBPath,
	}

	query := `
		SELECT
			timescale_host, timescale_port, timescale_database,
			timescale_user, timescale_password,
			timescale_ssl_mode, timescale_timezone
		FROM storage_configs
		WHERE config_id = (SELECT id FROM configs WHERE name = 'default')
		  AND backend_type = 'timescaledb'
	`

	var host, database, user, password, sslMode, timezone sql.NullString
	var port sql.NullInt64

	err = db.QueryRow(query).Scan(&host, &port, &database, &user, &password, &sslMode, &timezone)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no TimescaleDB configuration found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to query storage config: %w", err)
	}

	cfg.PostgresHost = host.String
	cfg.PostgresPort = int(port.Int64)
	cfg.DBName = database.String
	cfg.DBUser = user.String
	cfg.DBPassword = password.String
	cfg.SSLMode = sslMode.String
	cfg.Timezone = timezone.String

	return cfg, nil
}
