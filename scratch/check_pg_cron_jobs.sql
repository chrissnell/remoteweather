-- Check if pg_cron extension is installed
SELECT * FROM pg_extension WHERE extname = 'pg_cron';

-- List all cron jobs
SELECT
    jobid,
    schedule,
    command,
    nodename,
    nodeport,
    database,
    username,
    active,
    jobname
FROM cron.job
ORDER BY jobid;

-- Check recent job runs
SELECT
    jr.runid,
    jr.jobid,
    j.jobname,
    jr.job_pid,
    jr.database,
    jr.username,
    jr.command,
    jr.status,
    jr.return_message,
    jr.start_time,
    jr.end_time,
    (jr.end_time - jr.start_time) as duration
FROM cron.job_run_details jr
LEFT JOIN cron.job j ON jr.jobid = j.jobid
ORDER BY jr.start_time DESC
LIMIT 20;

-- Check for almanac-related jobs specifically
SELECT * FROM cron.job WHERE jobname LIKE '%almanac%' OR command LIKE '%almanac%';

-- Check job statistics (success/failure counts)
SELECT
    j.jobname,
    j.schedule,
    COUNT(*) as total_runs,
    SUM(CASE WHEN jr.status = 'succeeded' THEN 1 ELSE 0 END) as successful,
    SUM(CASE WHEN jr.status = 'failed' THEN 1 ELSE 0 END) as failed,
    MAX(jr.start_time) as last_run
FROM cron.job j
LEFT JOIN cron.job_run_details jr ON j.jobid = jr.jobid
GROUP BY j.jobname, j.schedule
ORDER BY last_run DESC NULLS LAST;
