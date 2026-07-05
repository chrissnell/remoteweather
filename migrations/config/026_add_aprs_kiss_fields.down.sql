-- SQLite doesn't support DROP COLUMN in older versions; the columns are
-- harmless if unused. For PostgreSQL:
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_transport;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_connection;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_serial_device;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_serial_baud;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_tcp_address;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_path;
ALTER TABLE devices DROP COLUMN IF EXISTS aprs_kiss_destination;
