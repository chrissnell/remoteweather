# Weather Data CSV Restore Tool

This tool intelligently restores CSV backups to the weather hypertable, automatically handling schema differences between the backup and the current database schema.

## Features

- **Schema-aware restoration**: Automatically detects which columns exist in both the CSV and the database
- **Handles new columns**: Any new columns in the database that aren't in the CSV will be set to NULL or their default values
- **Skips obsolete columns**: Columns in the CSV that no longer exist in the database are safely ignored
- **Progress tracking**: Shows progress percentage for large files
- **Efficient loading**: Uses PostgreSQL's COPY command for fast bulk loading
- **Transaction safety**: All data is loaded in a single transaction - either all succeeds or all fails

## Usage

```bash
go run cmd/weather-restore/main.go -file=weather_backup.csv [flags]
```

## Flags

- `-file`: Path to the CSV file to restore (required)
- `-host`: Database host (default: "localhost")
- `-port`: Database port (default: 5432)
- `-database`: Database name (default: "weather")
- `-user`: Database user (default: "postgres")
- `-password`: Database password (required)
- `-sslmode`: SSL mode - disable, require, etc (default: "disable")
- `-batch`: Number of rows to process per batch (default: 1000)

## Examples

### Basic restore
```bash
go run cmd/weather-restore/main.go -file=weather_backup.csv -password=yourpassword
```

### Restore to remote database
```bash
go run cmd/weather-restore/main.go \
  -file=weather_backup.csv \
  -host=db.example.com \
  -port=5432 \
  -database=weather_new \
  -user=weatheruser \
  -password=yourpassword \
  -sslmode=require
```

## How It Works

1. **Column Detection**: The tool reads the CSV headers and queries the database schema
2. **Column Matching**: It identifies which CSV columns exist in the current database
3. **Smart Import**: Only imports data for columns that exist in both the CSV and database
4. **Progress Tracking**: Shows percentage complete based on file size

## Example Output

```
Connected to database weather@localhost:5432
Found 95 columns in CSV: [time stationname stationtype barometer ...]
Weather table has 98 columns
WARNING: The following columns from CSV are not in the database and will be skipped: [old_column1 old_column2]
Will import 93 columns: [time stationname stationtype ...]
Processed 10000 rows (12.5%)
Processed 20000 rows (25.0%)
...
Successfully imported 80000 rows
Restore completed successfully!
```

## Schema Migration Workflow

1. **Backup old data**: Use `weather-backup` to export current data
2. **Update schema**: Modify remoteweather code to add new columns
3. **Recreate database**: Drop and recreate with new schema
4. **Restore data**: Use this tool to import old data
   - Old columns → Imported normally
   - New columns → Set to NULL/default
   - Deleted columns → Skipped automatically

## Performance Tips

- The tool uses PostgreSQL's COPY command for optimal performance
- For very large datasets, consider:
  - Temporarily disabling indexes during import
  - Increasing shared_buffers in PostgreSQL
  - Running on the database server to avoid network overhead

## Troubleshooting

### "Column not found" errors
This shouldn't happen with this tool, but if it does, check that:
- The CSV file has a header row
- The database table exists
- Column names match exactly (case-sensitive)

### Out of memory
For extremely large files:
- The tool streams data, so memory usage should be constant
- Check PostgreSQL's work_mem setting
- Consider splitting the CSV into smaller files

### Slow performance
- Ensure no other processes are heavily using the database
- Check for triggers that might slow down inserts
- Consider dropping indexes and recreating after import