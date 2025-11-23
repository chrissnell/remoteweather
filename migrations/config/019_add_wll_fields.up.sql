-- Add WeatherLink Live configuration fields to devices table
ALTER TABLE devices ADD COLUMN wll_host TEXT;
ALTER TABLE devices ADD COLUMN wll_port INTEGER DEFAULT 80;
ALTER TABLE devices ADD COLUMN wll_broadcast BOOLEAN DEFAULT TRUE;
ALTER TABLE devices ADD COLUMN wll_sensor_mapping TEXT;
ALTER TABLE devices ADD COLUMN wll_poll_interval INTEGER DEFAULT 60;
