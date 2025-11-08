package provision

import (
	"bufio"
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// DetectHbaPath gets pg_hba.conf location from PostgreSQL
func DetectHbaPath(cfg *Config) (string, error) {
	// Try to connect to query settings (might fail auth but still connect)
	connStr := cfg.BuildConnString("postgres")

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return detectHbaPathFallback()
	}
	defer db.Close()

	var hbaPath string
	err = db.QueryRow("SELECT setting FROM pg_settings WHERE name = 'hba_file'").Scan(&hbaPath)
	if err != nil {
		// If we can't query, try common locations
		return detectHbaPathFallback()
	}

	return hbaPath, nil
}

// detectHbaPathFallback tries common pg_hba.conf locations
func detectHbaPathFallback() (string, error) {
	commonPaths := []string{
		"/etc/postgresql/16/main/pg_hba.conf",
		"/etc/postgresql/15/main/pg_hba.conf",
		"/etc/postgresql/14/main/pg_hba.conf",
		"/etc/postgresql/13/main/pg_hba.conf",
		"/var/lib/pgsql/data/pg_hba.conf",
		"/var/lib/pgsql/16/data/pg_hba.conf",
		"/var/lib/pgsql/15/data/pg_hba.conf",
		"/var/lib/pgsql/14/data/pg_hba.conf",
		"/var/lib/postgres/data/pg_hba.conf",
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("could not locate pg_hba.conf in common locations")
}

// IsAuthError determines if error is pg_hba.conf related
func IsAuthError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "authentication failed") ||
		strings.Contains(errStr, "no pg_hba.conf entry") ||
		strings.Contains(errStr, "password authentication failed") ||
		strings.Contains(errStr, "FATAL")
}

// PromptUserForFix asks user if they want to auto-fix pg_hba.conf
func PromptUserForFix() bool {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Printf("%s%s⚠️  PostgreSQL Authentication Configuration Required%s\n", ColorBold, ColorBrightYellow, ColorReset)
	fmt.Printf("%s====================================================%s\n", ColorBrightYellow, ColorReset)
	fmt.Println()
	fmt.Println("The provisioner cannot connect to PostgreSQL because password")
	fmt.Println("authentication is not enabled in pg_hba.conf.")
	fmt.Println()
	fmt.Println("What I'll do to fix this:")
	fmt.Println()
	fmt.Println("  1. Find your pg_hba.conf file")
	fmt.Println("  2. Create a timestamped backup of it")
	fmt.Println("  3. Add this line to enable password authentication:")
	fmt.Println()
	fmt.Printf("     %s%slocal   all   all   scram-sha-256%s\n", ColorBold, ColorBrightCyan, ColorReset)
	fmt.Println()
	fmt.Println("  4. Reload PostgreSQL to apply the change")
	fmt.Println()
	fmt.Println("This allows any Unix user (including root) to connect to any")
	fmt.Println("PostgreSQL database by providing a password. Your backup file")
	fmt.Println("will be saved if you need to restore the original settings.")
	fmt.Println()
	fmt.Print("Would you like me to fix this automatically? [y/N]: ")

	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	return response == "y" || response == "yes"
}


// BackupHbaFile creates timestamped backup
func BackupHbaFile(hbaPath string) (string, error) {
	timestamp := time.Now().Format("20060102-150405")
	backupPath := fmt.Sprintf("%s.provisioner-backup.%s", hbaPath, timestamp)

	cmd := exec.Command("cp", hbaPath, backupPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("backup failed: %w\n%s", err, output)
	}

	return backupPath, nil
}

// ModifyHbaFile modifies pg_hba.conf to allow peer and password auth
func ModifyHbaFile(hbaPath string) error {
	// Read current content
	content, err := os.ReadFile(hbaPath)
	if err != nil {
		return fmt.Errorf("failed to read pg_hba.conf: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Check if our password rule already exists at the right position
	hasPasswordRule := false
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "local") &&
			strings.Contains(trimmed, "all") &&
			strings.Contains(trimmed, "all") &&
			strings.Contains(trimmed, "scram-sha-256") {
			hasPasswordRule = true
			break
		}
	}

	if hasPasswordRule {
		fmt.Println("ℹ️  Password authentication rule already exists in pg_hba.conf")
		return nil
	}

	// Build new lines - insert our rule at the VERY TOP (before any other rules)
	newLines := []string{}
	inserted := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Insert BEFORE the first "local" rule (so our rule takes precedence)
		if !inserted && strings.HasPrefix(trimmed, "local") {
			newLines = append(newLines, "# Allow any Unix user to connect with password (provisioner-added)")
			newLines = append(newLines, "local   all             all                                     scram-sha-256")
			newLines = append(newLines, "")
			inserted = true
		}

		newLines = append(newLines, line)
	}

	// If we never found a "local" line, append at the end
	if !inserted {
		newLines = append(newLines, "")
		newLines = append(newLines, "# Allow any Unix user to connect with password (provisioner-added)")
		newLines = append(newLines, "local   all             all                                     scram-sha-256")
	}

	newContent := strings.Join(newLines, "\n")

	// Write to temp file first
	tmpFile := "/tmp/pg_hba.conf.tmp"
	err = os.WriteFile(tmpFile, []byte(newContent), 0600)
	if err != nil {
		return fmt.Errorf("failed to write temp file: %w", err)
	}

	// Move temp file to actual location
	cmd := exec.Command("mv", tmpFile, hbaPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to move file: %w\n%s", err, output)
	}

	// Set proper permissions
	cmd = exec.Command("chmod", "0640", hbaPath)
	cmd.Run()

	fmt.Println("ℹ️  Added password auth rule at TOP of pg_hba.conf (takes precedence over peer auth)")

	return nil
}

// ReloadPostgreSQL reloads configuration via SQL
func ReloadPostgreSQL(cfg *Config) error {
	connStr := cfg.BuildConnString("postgres")

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return err
	}
	defer db.Close()

	_, err = db.Exec("SELECT pg_reload_conf()")
	if err != nil {
		return err
	}

	return nil
}

// ReloadPostgreSQLSystemctl uses systemctl as fallback
func ReloadPostgreSQLSystemctl() error {
	services := []string{
		"postgresql",
		"postgresql@16-main",
		"postgresql@15-main",
		"postgresql@14-main",
		"postgresql-16",
		"postgresql-15",
		"postgresql-14",
	}

	for _, service := range services {
		cmd := exec.Command("systemctl", "reload", service)
		err := cmd.Run()
		if err == nil {
			fmt.Printf("✅ Reloaded via systemctl (%s)\n", service)
			return nil
		}
	}

	return fmt.Errorf("could not reload PostgreSQL service")
}

// AutoFixPgHba orchestrates the entire fix process for pg_hba.conf
func AutoFixPgHba(cfg *Config) error {
	// Step 1: Ask user permission
	if !PromptUserForFix() {
		return fmt.Errorf("user declined auto-fix.\n\n" +
			"Please manually add to pg_hba.conf:\n" +
			"  local   all   all   scram-sha-256\n\n" +
			"Then reload: systemctl reload postgresql")
	}

	// Step 2: Detect pg_hba.conf location
	fmt.Println()
	fmt.Println("🔍 Detecting pg_hba.conf location...")
	hbaPath, err := DetectHbaPath(cfg)
	if err != nil {
		return fmt.Errorf("failed to detect pg_hba.conf: %w", err)
	}
	fmt.Printf("✅ Found: %s\n", hbaPath)

	// Step 3: Final confirmation
	reader := bufio.NewReader(os.Stdin)
	fmt.Println()
	fmt.Println("Ready to modify pg_hba.conf:")
	fmt.Printf("  File: %s\n", hbaPath)
	fmt.Println("  Change: Add password authentication for all Unix users")
	fmt.Println()
	fmt.Print("Proceed with modification? [y/N]: ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response != "y" && response != "yes" {
		return fmt.Errorf("operation cancelled by user")
	}

	// Step 4: Backup
	fmt.Println()
	fmt.Println("💾 Creating backup...")
	backupPath, err := BackupHbaFile(hbaPath)
	if err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	fmt.Printf("✅ Backup created: %s\n", backupPath)

	// Step 5: Modify pg_hba.conf
	fmt.Println("✏️  Modifying pg_hba.conf...")
	if err := ModifyHbaFile(hbaPath); err != nil {
		return fmt.Errorf("modification failed: %w\nBackup available at: %s", err, backupPath)
	}
	fmt.Println("✅ pg_hba.conf updated")

	// Step 6: Reload - try SQL first, then systemctl
	fmt.Println("🔄 Reloading PostgreSQL configuration...")
	if err := ReloadPostgreSQL(cfg); err != nil {
		// Try systemctl as fallback
		if err := ReloadPostgreSQLSystemctl(); err != nil {
			return fmt.Errorf("reload failed: %w\nBackup available at: %s\n\n"+
				"Try manually: systemctl reload postgresql", err, backupPath)
		}
	} else {
		fmt.Println("✅ Configuration reloaded via SQL")
	}

	// Step 7: Wait a moment for changes to apply
	fmt.Println("⏳ Waiting for configuration to apply...")
	time.Sleep(2 * time.Second)

	// Step 8: Test connection again
	fmt.Println("🔍 Testing connection...")
	if err := checkPostgreSQLConnection(cfg); err != nil {
		return fmt.Errorf("connection still failing after fix: %w\n"+
			"Backup available at: %s\n\n"+
			"The pg_hba.conf has been modified but connection still fails.\n"+
			"Please check PostgreSQL logs for details.", err, backupPath)
	}
	fmt.Println("✅ Connection successful!")
	fmt.Println()

	return nil
}
