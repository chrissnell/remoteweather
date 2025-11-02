-- Remove snow indexes added by this migration
DROP INDEX IF EXISTS weather_1h_stationname_snowdistance_bucket_idx;
DROP INDEX IF EXISTS weather_1d_stationname_snowdistance_bucket_idx;
