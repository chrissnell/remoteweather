-- SQLite doesn't support DROP COLUMN in older versions; the columns are
-- harmless if unused. For PostgreSQL:
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_upload_interval;
ALTER TABLE devices DROP COLUMN IF EXISTS aeris_refresh_interval;
