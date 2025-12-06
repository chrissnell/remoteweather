-- Migration 025 rollback: Remove snow_depth_est_5m table

-- Drop the hypertable and all its data
DROP TABLE IF EXISTS snow_depth_est_5m CASCADE;
