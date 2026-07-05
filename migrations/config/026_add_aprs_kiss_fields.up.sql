-- Migration 026: Add APRS KISS transport fields to devices table
-- Enables sending APRS weather packets over KISS to a TNC (serial or network)
-- as an alternative to APRS-IS.

ALTER TABLE devices ADD COLUMN aprs_transport TEXT DEFAULT 'aprs-is';
ALTER TABLE devices ADD COLUMN aprs_kiss_connection TEXT;
ALTER TABLE devices ADD COLUMN aprs_kiss_serial_device TEXT;
ALTER TABLE devices ADD COLUMN aprs_kiss_serial_baud INTEGER DEFAULT 9600;
ALTER TABLE devices ADD COLUMN aprs_kiss_tcp_address TEXT;
ALTER TABLE devices ADD COLUMN aprs_kiss_path TEXT DEFAULT 'WIDE1-1,WIDE2-1';
ALTER TABLE devices ADD COLUMN aprs_kiss_destination TEXT DEFAULT 'APRS';
