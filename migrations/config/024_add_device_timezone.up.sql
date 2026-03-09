-- Add timezone column to devices table for per-station timezone support
ALTER TABLE devices ADD COLUMN timezone TEXT DEFAULT '';
