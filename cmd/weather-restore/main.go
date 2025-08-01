package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host      string
	Port      int
	Database  string
	User      string
	Password  string
	SSLMode   string
	CSVFile   string
	BatchSize int
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
	flag.StringVar(&cfg.CSVFile, "file", "", "CSV file to restore from (required)")
	flag.IntVar(&cfg.BatchSize, "batch", 1000, "Number of rows to insert per batch")
	flag.Parse()

	if cfg.CSVFile == "" {
		log.Fatal("CSV file is required. Use -file flag")
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

	// Open CSV file
	file, err := os.Open(cfg.CSVFile)
	if err != nil {
		log.Fatalf("Failed to open CSV file: %v", err)
	}
	defer file.Close()

	// Get file size for progress tracking
	fileInfo, err := file.Stat()
	if err != nil {
		log.Fatalf("Failed to stat file: %v", err)
	}
	fileSize := fileInfo.Size()

	// Create a reader that tracks progress
	progressReader := &progressReader{
		reader:   file,
		total:    fileSize,
		progress: 0,
	}

	// Parse CSV
	csvReader := csv.NewReader(progressReader)
	
	// Read header
	headers, err := csvReader.Read()
	if err != nil {
		log.Fatalf("Failed to read CSV headers: %v", err)
	}

	log.Printf("Found %d columns in CSV: %v", len(headers), headers)

	// Verify weather table exists and get its schema
	tableColumns, columnTypes, err := getTableColumns(ctx, pool, "weather")
	if err != nil {
		log.Fatalf("Failed to get table schema: %v", err)
	}

	log.Printf("Weather table has %d columns", len(tableColumns))

	// Check which CSV columns exist in the database
	var matchedColumns []string
	var missingColumns []string
	columnMap := make(map[string]bool)
	
	for _, col := range tableColumns {
		columnMap[col] = true
	}

	for _, header := range headers {
		if columnMap[header] {
			matchedColumns = append(matchedColumns, header)
		} else {
			missingColumns = append(missingColumns, header)
		}
	}

	if len(missingColumns) > 0 {
		log.Printf("WARNING: The following columns from CSV are not in the database and will be skipped: %v", missingColumns)
	}

	log.Printf("Will import %d columns: %v", len(matchedColumns), matchedColumns)

	// Find indices of matched columns in the CSV
	columnIndices := make(map[string]int)
	for i, header := range headers {
		if columnMap[header] {
			columnIndices[header] = i
		}
	}

	// Restore data
	if err := restoreData(ctx, pool, csvReader, matchedColumns, columnIndices, columnTypes, cfg.BatchSize, progressReader); err != nil {
		log.Fatalf("Failed to restore data: %v", err)
	}

	log.Println("Restore completed successfully!")
}

func getTableColumns(ctx context.Context, pool *pgxpool.Pool, tableName string) ([]string, map[string]string, error) {
	query := `
		SELECT column_name, data_type 
		FROM information_schema.columns 
		WHERE table_name = $1 
		ORDER BY ordinal_position
	`
	
	rows, err := pool.Query(ctx, query, tableName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to query table schema: %w", err)
	}
	defer rows.Close()

	var columns []string
	columnTypes := make(map[string]string)
	
	for rows.Next() {
		var column, dataType string
		if err := rows.Scan(&column, &dataType); err != nil {
			return nil, nil, fmt.Errorf("failed to scan column: %w", err)
		}
		columns = append(columns, column)
		columnTypes[column] = dataType
	}

	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("row error: %w", err)
	}

	if len(columns) == 0 {
		return nil, nil, fmt.Errorf("table %s not found or has no columns", tableName)
	}

	return columns, columnTypes, nil
}

// parseTimestamp parses various timestamp formats
func parseTimestamp(value string) (time.Time, error) {
	// Try common timestamp formats
	formats := []string{
		"2006-01-02 15:04:05.999999 -0700 MST",  // Format from the error message
		"2006-01-02 15:04:05.999999 -0700",
		"2006-01-02 15:04:05.999999",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05.999999Z",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
		time.RFC3339Nano,
	}
	
	for _, format := range formats {
		if t, err := time.Parse(format, value); err == nil {
			return t, nil
		}
	}
	
	return time.Time{}, fmt.Errorf("unable to parse timestamp: %s", value)
}

func restoreData(ctx context.Context, pool *pgxpool.Pool, reader *csv.Reader, columns []string, columnIndices map[string]int, columnTypes map[string]string, batchSize int, progress *progressReader) error {
	// Start transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Prepare rows channel for batch processing
	rows := make([][]interface{}, 0, batchSize)
	rowCount := 0
	
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Extract only the columns we need
		row := make([]interface{}, len(columns))
		for i, col := range columns {
			csvIndex := columnIndices[col]
			if csvIndex < len(record) && record[csvIndex] != "" {
				value := record[csvIndex]
				colType := columnTypes[col]
				
				// Convert based on column type
				switch colType {
				case "timestamp with time zone", "timestamp without time zone":
					// Parse various timestamp formats
					parsedTime, err := parseTimestamp(value)
					if err != nil {
						return fmt.Errorf("failed to parse timestamp for column %s: %w", col, err)
					}
					row[i] = parsedTime
				case "integer", "bigint", "smallint":
					if value == "" {
						row[i] = nil
					} else {
						intVal, err := strconv.ParseInt(value, 10, 64)
						if err != nil {
							row[i] = nil // Convert parse errors to NULL
						} else {
							row[i] = intVal
						}
					}
				case "numeric", "real", "double precision":
					if value == "" {
						row[i] = nil
					} else {
						floatVal, err := strconv.ParseFloat(value, 64)
						if err != nil {
							row[i] = nil // Convert parse errors to NULL
						} else {
							row[i] = floatVal
						}
					}
				case "boolean":
					if value == "" {
						row[i] = nil
					} else {
						row[i] = (value == "true" || value == "t" || value == "1")
					}
				default:
					// For text, varchar, and other types, use string as-is
					row[i] = value
				}
			} else {
				row[i] = nil
			}
		}

		rows = append(rows, row)
		rowCount++

		// Process batch when full
		if len(rows) >= batchSize {
			_, err := tx.CopyFrom(
				ctx,
				pgx.Identifier{"weather"},
				columns,
				pgx.CopyFromRows(rows),
			)
			if err != nil {
				return fmt.Errorf("failed to copy batch: %w", err)
			}
			
			percentage := float64(progress.progress) / float64(progress.total) * 100
			log.Printf("Processed %d rows (%.1f%%)", rowCount, percentage)
			
			// Clear the rows slice for next batch
			rows = rows[:0]
		}
	}

	// Process any remaining rows
	if len(rows) > 0 {
		_, err := tx.CopyFrom(
			ctx,
			pgx.Identifier{"weather"},
			columns,
			pgx.CopyFromRows(rows),
		)
		if err != nil {
			return fmt.Errorf("failed to copy final batch: %w", err)
		}
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully imported %d rows", rowCount)
	return nil
}

// progressReader wraps a reader to track read progress
type progressReader struct {
	reader   io.Reader
	total    int64
	progress int64
}

func (pr *progressReader) Read(p []byte) (n int, err error) {
	n, err = pr.reader.Read(p)
	pr.progress += int64(n)
	return n, err
}

// Alternative implementation using batch INSERTs instead of COPY
func restoreDataBatchInsert(ctx context.Context, pool *pgxpool.Pool, reader *csv.Reader, columns []string, columnIndices map[string]int, batchSize int) error {
	// Build parameterized INSERT query
	placeholders := make([]string, len(columns))
	for i := range placeholders {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	
	insertQuery := fmt.Sprintf(
		"INSERT INTO weather (%s) VALUES (%s)",
		strings.Join(columns, ", "),
		strings.Join(placeholders, ", "),
	)

	// Start transaction
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to start transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	batch := make([][]interface{}, 0, batchSize)
	rowCount := 0

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read CSV record: %w", err)
		}

		// Extract values for matched columns
		values := make([]interface{}, len(columns))
		for i, col := range columns {
			csvIndex := columnIndices[col]
			if csvIndex < len(record) && record[csvIndex] != "" {
				values[i] = record[csvIndex]
			} else {
				values[i] = nil
			}
		}

		batch = append(batch, values)

		// Execute batch when it reaches the size limit
		if len(batch) >= batchSize {
			if err := executeBatch(ctx, tx, insertQuery, batch); err != nil {
				return err
			}
			rowCount += len(batch)
			log.Printf("Processed %d rows", rowCount)
			batch = batch[:0]
		}
	}

	// Execute remaining batch
	if len(batch) > 0 {
		if err := executeBatch(ctx, tx, insertQuery, batch); err != nil {
			return err
		}
		rowCount += len(batch)
	}

	// Commit transaction
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	log.Printf("Successfully imported %d rows", rowCount)
	return nil
}

func executeBatch(ctx context.Context, tx pgx.Tx, query string, batch [][]interface{}) error {
	b := &pgx.Batch{}
	for _, values := range batch {
		b.Queue(query, values...)
	}
	
	results := tx.SendBatch(ctx, b)
	defer results.Close()

	for range batch {
		if _, err := results.Exec(); err != nil {
			return fmt.Errorf("failed to execute insert: %w", err)
		}
	}

	return nil
}
