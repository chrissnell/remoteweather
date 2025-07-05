-- Reverse the data migration by clearing individual component fields
-- This preserves the original connection string for backward compatibility

UPDATE storage_configs 
SET 
    timescale_host = '',
    timescale_port = 5432,
    timescale_database = '',
    timescale_user = '',
    timescale_password = '',
    timescale_ssl_mode = 'prefer',
    timescale_timezone = ''
WHERE backend_type = 'timescaledb' 
  AND timescale_connection_string IS NOT NULL 
  AND timescale_connection_string != ''; 