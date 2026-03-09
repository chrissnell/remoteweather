-- SQLite doesn't support DROP COLUMN directly; recreate without timezone
-- For SQLite, this is a no-op since the column is harmless if unused
-- For PostgreSQL:
ALTER TABLE devices DROP COLUMN IF EXISTS timezone;
