-- SQLite doesn't support DROP COLUMN directly
-- This would require recreating the table without wll_* columns
-- For development/testing, this is marked as informational only
-- In production, a full table recreation migration would be needed

-- Informational: The following columns would need to be removed:
-- wll_host, wll_port, wll_broadcast, wll_sensor_mapping, wll_poll_interval
