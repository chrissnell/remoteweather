package main

import (
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"strings"

	"context"
	
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type BackupFormat string

const (
	FormatCSV  BackupFormat = "csv"
	FormatJSON BackupFormat = "json"
	FormatSQL  BackupFormat = "sql"
)

type Config struct {
	Host     string
	Port     int
	Database string
	User     string
	Password string
	SSLMode  string
	Format   BackupFormat
	Output   string
	Query    string
}

func main() {
	var cfg Config
	
	// Parse command line flags
	flag.StringVar(&cfg.Host, "host", "localhost", "Database host")
	flag.IntVar(&cfg.Port, "port", 5432, "Database port")
	flag.StringVar(&cfg.Database, "database", "weather", "Database name")
	flag.StringVar(&cfg.User, "user", "postgres", "Database user")
	flag.StringVar(&cfg.Password, "password", "", "Database password")
	flag.StringVar(&cfg.SSLMode, "sslmode", "disable", "SSL mode (disable, require, etc)")
	formatStr := flag.String("format", "csv", "Backup format: csv, json, or sql")
	flag.StringVar(&cfg.Output, "output", "weather_backup", "Output file base name (extension added automatically)")
	flag.StringVar(&cfg.Query, "query", "", "Optional WHERE clause for filtering data (e.g., \"time > '2024-01-01'\")")
	flag.Parse()

	// Validate format
	switch BackupFormat(*formatStr) {
	case FormatCSV, FormatJSON, FormatSQL:
		cfg.Format = BackupFormat(*formatStr)
	default:
		log.Fatalf("Invalid format: %s. Must be csv, json, or sql", *formatStr)
	}

	// Build connection string
	connStr := fmt.Sprintf("host=%s port=%d dbname=%s user=%s password=%s sslmode=%s",
		cfg.Host, cfg.Port, cfg.Database, cfg.User, cfg.Password, cfg.SSLMode)

	// Connect to database
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	log.Printf("Connected to database %s@%s:%d", cfg.Database, cfg.Host, cfg.Port)

	// Build query
	query := "SELECT * FROM weather"
	countQuery := "SELECT COUNT(*) FROM weather"
	if cfg.Query != "" {
		query += " WHERE " + cfg.Query
		countQuery += " WHERE " + cfg.Query
	}
	query += " ORDER BY time"

	// Get total count for progress tracking
	var totalCount int64
	err = pool.QueryRow(ctx, countQuery).Scan(&totalCount)
	if err != nil {
		log.Fatalf("Failed to get record count: %v", err)
	}
	log.Printf("Found %d records to backup", totalCount)

	// Execute backup based on format
	switch cfg.Format {
	case FormatCSV:
		if err := backupToCSV(ctx, pool, query, cfg.Output+".csv", totalCount); err != nil {
			log.Fatalf("CSV backup failed: %v", err)
		}
	case FormatJSON:
		if err := backupToJSON(ctx, pool, query, cfg.Output+".json", totalCount); err != nil {
			log.Fatalf("JSON backup failed: %v", err)
		}
	case FormatSQL:
		if err := backupToSQL(ctx, pool, query, cfg.Output+".sql", totalCount); err != nil {
			log.Fatalf("SQL backup failed: %v", err)
		}
	}

	log.Printf("Backup completed successfully")
}

func backupToCSV(ctx context.Context, pool *pgxpool.Pool, query string, filename string, totalCount int64) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Execute query
	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	// Get column names from the query result
	fieldDescs := rows.FieldDescriptions()
	columns := make([]string, len(fieldDescs))
	for i, fd := range fieldDescs {
		columns[i] = string(fd.Name)
	}
	
	if err := writer.Write(columns); err != nil {
		return fmt.Errorf("failed to write headers: %w", err)
	}

	count := int64(0)
	lastProgress := -1
	for rows.Next() {
		values, err := pgx.RowToMap(rows)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Convert row to CSV format
		record := make([]string, len(columns))
		for i, col := range columns {
			if val, ok := values[col]; ok && val != nil {
				record[i] = fmt.Sprintf("%v", val)
			}
		}

		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write record: %w", err)
		}

		count++
		// Show progress at each percentage point
		if totalCount > 0 {
			progress := int(count * 100 / totalCount)
			if progress != lastProgress {
				log.Printf("Progress: %d%% (%d/%d records)", progress, count, totalCount)
				lastProgress = progress
			}
		} else if count%10000 == 0 {
			log.Printf("Processed %d records...", count)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	log.Printf("Exported %d records to %s", count, filename)
	return nil
}

func backupToJSON(ctx context.Context, pool *pgxpool.Pool, query string, filename string, totalCount int64) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Start JSON array
	if _, err := file.WriteString("[\n"); err != nil {
		return err
	}

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("  ", "  ")

	count := int64(0)
	lastProgress := -1
	first := true
	for rows.Next() {
		values, err := pgx.RowToMap(rows)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Add comma between records
		if !first {
			if _, err := file.WriteString(",\n"); err != nil {
				return err
			}
		}
		first = false

		// Write record
		if _, err := file.WriteString("  "); err != nil {
			return err
		}
		if err := encoder.Encode(values); err != nil {
			return fmt.Errorf("failed to encode record: %w", err)
		}

		count++
		// Show progress at each percentage point
		if totalCount > 0 {
			progress := int(count * 100 / totalCount)
			if progress != lastProgress {
				log.Printf("Progress: %d%% (%d/%d records)", progress, count, totalCount)
				lastProgress = progress
			}
		} else if count%10000 == 0 {
			log.Printf("Processed %d records...", count)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	// Close JSON array
	if _, err := file.WriteString("\n]"); err != nil {
		return err
	}

	log.Printf("Exported %d records to %s", count, filename)
	return nil
}

func backupToSQL(ctx context.Context, pool *pgxpool.Pool, query string, filename string, totalCount int64) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Write header
	fmt.Fprintf(file, "-- Weather data backup generated on %s\n", time.Now().Format(time.RFC3339))
	fmt.Fprintf(file, "-- Query: %s\n", query)
	fmt.Fprintln(file, "-- This backup uses explicit column names to handle schema changes")
	fmt.Fprintln(file, "\nBEGIN;")
	fmt.Fprintln(file)

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	count := int64(0)
	lastProgress := -1
	
	for rows.Next() {
		values, err := pgx.RowToMap(rows)
		if err != nil {
			return fmt.Errorf("failed to scan row: %w", err)
		}

		// Build INSERT statement with only the columns that exist in the backup
		var cols []string
		var vals []string
		
		for col, val := range values {
			cols = append(cols, col)
			
			if val == nil {
				vals = append(vals, "NULL")
			} else {
				switch v := val.(type) {
				case string:
					vals = append(vals, fmt.Sprintf("'%s'", strings.ReplaceAll(v, "'", "''")))
				case time.Time:
					vals = append(vals, fmt.Sprintf("'%s'", v.Format(time.RFC3339)))
				case bool:
					vals = append(vals, fmt.Sprintf("%t", v))
				case int, int32, int64:
					vals = append(vals, fmt.Sprintf("%d", v))
				case float32, float64:
					vals = append(vals, fmt.Sprintf("%v", v))
				default:
					vals = append(vals, fmt.Sprintf("'%v'", v))
				}
			}
		}
		
		fmt.Fprintf(file, "INSERT INTO weather (%s) VALUES (%s);\n", 
			strings.Join(cols, ", "), strings.Join(vals, ", "))

		count++
		// Show progress at each percentage point
		if totalCount > 0 {
			progress := int(count * 100 / totalCount)
			if progress != lastProgress {
				log.Printf("Progress: %d%% (%d/%d records)", progress, count, totalCount)
				lastProgress = progress
			}
		} else if count%10000 == 0 {
			log.Printf("Processed %d records...", count)
		}
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("row iteration error: %w", err)
	}

	fmt.Fprintln(file, "\nCOMMIT;")
	
	log.Printf("Exported %d records to %s", count, filename)
	return nil
}

