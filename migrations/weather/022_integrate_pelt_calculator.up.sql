-- Migration 022: Integrate PELT-based snow calculator for 72h + seasonal
--
-- Rationale: Replace SQL-based 72h and seasonal calculations with PELT
-- (Pruned Exact Linear Time) statistical change point detection algorithm.
-- This provides more accurate multi-day snowfall calculations by detecting
-- actual accumulation events vs. noise from settling/compaction.
--
-- Changes:
-- 1. Remove TimescaleDB scheduled job (replaced by Go goroutine)
-- 2. Drop get_new_snow_72h SQL function (replaced by PELT in Go)
-- 3. Drop calculate_total_season_snowfall SQL function (replaced by PELT in Go)
-- 4. Keep get_new_snow_midnight and get_new_snow_24h (dual-threshold still optimal)
--
-- No changes to snow_totals_cache table structure - same columns, different algorithms

-- =============================================================================
-- STEP 1: Remove TimescaleDB scheduled job (no longer needed)
-- =============================================================================

DO $$
DECLARE
    job_record RECORD;
BEGIN
    -- Find and delete the refresh_snow_cache job
    SELECT job_id INTO job_record
    FROM timescaledb_information.jobs
    WHERE proc_name = 'refresh_snow_cache'
    LIMIT 1;

    IF FOUND THEN
        PERFORM delete_job(job_record.job_id);
        RAISE NOTICE 'Deleted TimescaleDB job for refresh_snow_cache (job_id: %)', job_record.job_id;
    ELSE
        RAISE NOTICE 'No TimescaleDB job found for refresh_snow_cache';
    END IF;
END $$;

-- =============================================================================
-- STEP 2: Drop refresh_snow_cache function (no longer called by job)
-- =============================================================================

DROP FUNCTION IF EXISTS refresh_snow_cache(INT, JSONB);

-- =============================================================================
-- STEP 3: Drop 72h and seasonal SQL functions (replaced by Go PELT calculator)
-- =============================================================================

DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);
DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT);

-- =============================================================================
-- STEP 4: Keep midnight/24h SQL functions (still fast and accurate)
-- =============================================================================

-- get_new_snow_midnight(TEXT, FLOAT) - KEPT
-- get_new_snow_24h(TEXT, FLOAT) - KEPT
-- These continue to use the dual-threshold algorithm for short-term calculations

-- =============================================================================
-- STEP 5: Update table comment to document the hybrid approach
-- =============================================================================

COMMENT ON TABLE snow_totals_cache IS
'Snow accumulation cache. Updated every 30s by Go goroutine in snowcache controller.
Midnight/24h: Dual-threshold algorithm (SQL functions)
72h/Seasonal: PELT change point detection algorithm (Go snow package)
This hybrid approach balances speed (SQL for short-term) with accuracy (PELT for multi-day).';
