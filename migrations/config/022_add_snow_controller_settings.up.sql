-- Add snow cache controller fields to controller_configs table
ALTER TABLE controller_configs ADD COLUMN snow_station_name TEXT;
ALTER TABLE controller_configs ADD COLUMN snow_base_distance REAL DEFAULT 0.0;
ALTER TABLE controller_configs ADD COLUMN snow_smoothing_window INTEGER DEFAULT 5;
ALTER TABLE controller_configs ADD COLUMN snow_penalty REAL DEFAULT 3.0;
ALTER TABLE controller_configs ADD COLUMN snow_min_accumulation REAL DEFAULT 5.0;
ALTER TABLE controller_configs ADD COLUMN snow_min_segment_size INTEGER DEFAULT 2;
