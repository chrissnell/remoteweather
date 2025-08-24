-- Add station_id field to storage_configs for gRPC remote stations
ALTER TABLE storage_configs ADD COLUMN station_id TEXT;