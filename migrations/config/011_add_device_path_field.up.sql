-- Add path field to devices table for ambient-customized weather stations
ALTER TABLE devices ADD COLUMN path TEXT DEFAULT '';