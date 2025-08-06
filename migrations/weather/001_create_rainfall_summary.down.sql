-- Remove the TimescaleDB job
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM timescaledb_information.jobs 
        WHERE proc_name = 'update_rainfall_summary'
    ) THEN
        SELECT delete_job(job_id) 
        FROM timescaledb_information.jobs 
        WHERE proc_name = 'update_rainfall_summary';
    END IF;
END $$;

-- Drop functions
DROP FUNCTION IF EXISTS get_rainfall_with_recent(TEXT);
DROP FUNCTION IF EXISTS update_rainfall_summary();

-- Drop table
DROP TABLE IF EXISTS rainfall_summary;

-- Note: We don't drop schema_migrations as other migrations might depend on it