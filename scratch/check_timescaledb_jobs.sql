-- Check all TimescaleDB background jobs
SELECT
    job_id,
    application_name,
    schedule_interval,
    max_runtime,
    max_retries,
    retry_period,
    proc_schema,
    proc_name,
    owner,
    scheduled,
    config,
    next_start,
    hypertable_schema,
    hypertable_name
FROM timescaledb_information.jobs
ORDER BY job_id;

-- Check job statistics
SELECT
    job_id,
    last_run_started_at,
    last_successful_finish,
    last_run_status,
    job_status,
    last_run_duration,
    next_scheduled_run,
    total_runs,
    total_successes,
    total_failures
FROM timescaledb_information.job_stats
ORDER BY job_id;

-- Check for almanac-related jobs
SELECT * FROM timescaledb_information.jobs
WHERE proc_name LIKE '%almanac%';

-- Check for rainfall-related jobs
SELECT * FROM timescaledb_information.jobs
WHERE proc_name LIKE '%rainfall%';

-- Manually trigger almanac refresh for testing
SELECT refresh_almanac_cache();

-- Manually trigger almanac refresh for specific station
SELECT refresh_almanac_cache('CSI');

-- Check almanac cache contents
SELECT
    stationname,
    COUNT(*) as metric_count,
    MAX(updated_at) as last_updated
FROM almanac_cache
GROUP BY stationname
ORDER BY stationname;

-- View all metrics for a station
SELECT * FROM almanac_cache WHERE stationname = 'CSI' ORDER BY metric_name;
