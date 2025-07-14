# Weather Data Backup Tool

This tool extracts data from the TimescaleDB `weather` hypertable and saves it in various formats for backup or migration purposes.

## Usage

```bash
go run cmd/weather-backup/main.go [flags]
```

## Flags

- `-host`: Database host (default: "localhost")
- `-port`: Database port (default: 5432)
- `-database`: Database name (default: "weather")
- `-user`: Database user (default: "postgres")
- `-password`: Database password (required)
- `-sslmode`: SSL mode - disable, require, etc (default: "disable")
- `-format`: Backup format - csv, json, or sql (default: "csv")
- `-output`: Output file base name (default: "weather_backup")
- `-query`: Optional WHERE clause for filtering data (e.g., "time > '2024-01-01'")

## Examples

### Full backup to CSV
```bash
go run cmd/weather-backup/main.go -password=yourpassword -format=csv -output=weather_full
```

### Backup last month's data to JSON
```bash
go run cmd/weather-backup/main.go -password=yourpassword -format=json -query="time > NOW() - INTERVAL '1 month'" -output=weather_last_month
```

### Create SQL dump for reimport
```bash
go run cmd/weather-backup/main.go -password=yourpassword -format=sql -output=weather_dump
```

## Output Formats

### CSV
- Standard CSV format with headers
- Compatible with most spreadsheet applications
- Easy to process with standard tools

### JSON
- Array of JSON objects
- Each record is a complete object with all fields
- Suitable for NoSQL databases or programmatic processing

### SQL
- PostgreSQL-compatible INSERT statements
- Wrapped in a transaction (BEGIN/COMMIT)
- Can be directly executed to restore data

## Restoring Data with Schema Changes

This tool is designed to handle schema changes gracefully. When you add new columns to the weather table, the restoration will still work because:

- **SQL format**: Uses explicit column names in INSERT statements, so it only inserts columns that existed at backup time. New columns will get their default values.
- **CSV format**: Includes column headers that match the backup data, not the new schema.

### From CSV (Using the restore tool - recommended)
Use the companion `weather-restore` tool which automatically handles schema differences:
```bash
go run cmd/weather-restore/main.go -file=weather_backup.csv -password=yourpassword
```

The restore tool will:
- Automatically detect which columns exist in both the CSV and database
- Skip columns that no longer exist
- Leave new columns as NULL or their default values

### From CSV (Manual method)
If you prefer using psql directly, you need to specify the columns:
```bash
# First, get the column list from the CSV header
head -1 weather_backup.csv

# Then use COPY with explicit column list
psql -h localhost -U postgres -d weather -c "\COPY weather (time,stationname,stationtype,...) FROM 'weather_backup.csv' WITH CSV HEADER"
```

### From SQL (Recommended for schema changes)
```bash
psql -h localhost -U postgres -d weather < weather_dump.sql
```
The SQL format automatically handles schema changes because each INSERT specifies its column names.

### From JSON
You'll need to write a custom script to parse the JSON and insert into the database, mapping only the fields that exist in the JSON.

## Migration Workflow

1. Run this backup tool to extract your data
2. Drop and recreate your database  
3. Start remoteweather (it will automatically create the schema with any new columns)
4. Restore your data using the SQL format (recommended) or CSV with explicit columns

## Notes

- The tool preserves all column data types and NULL values
- Large backups are processed in chunks with progress reporting
- The SQL format properly escapes string values and handles timestamps
- Consider using compression for large backups: `gzip weather_backup.csv`