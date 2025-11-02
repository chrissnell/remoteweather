-- Remove the TimescaleDB job if it exists
DO $$
DECLARE
    job_id_to_delete INT;
BEGIN
    -- Find the job ID
    SELECT job_id INTO job_id_to_delete
    FROM timescaledb_information.jobs
    WHERE proc_name = 'refresh_almanac_cache';

    -- Delete the job if found
    IF job_id_to_delete IS NOT NULL THEN
        PERFORM delete_job(job_id_to_delete);
    END IF;
END $$;

-- Drop functions
DROP FUNCTION IF EXISTS refresh_almanac_cache_single(TEXT);
DROP FUNCTION IF EXISTS refresh_almanac_cache(TEXT, INT, JSONB);

-- Drop table
DROP TABLE IF EXISTS almanac_cache;
