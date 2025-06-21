-- Drop tables in reverse order to handle foreign key constraints
DROP TRIGGER IF EXISTS update_configs_timestamp;
DROP INDEX IF EXISTS idx_weather_site_configs_controller_id;
DROP INDEX IF EXISTS idx_controller_configs_type;
DROP INDEX IF EXISTS idx_controller_configs_config_id;
DROP INDEX IF EXISTS idx_storage_configs_type;
DROP INDEX IF EXISTS idx_storage_configs_config_id;
DROP INDEX IF EXISTS idx_devices_name;
DROP INDEX IF EXISTS idx_devices_config_id;

DROP TABLE IF EXISTS weather_site_configs;
DROP TABLE IF EXISTS controller_configs;
DROP TABLE IF EXISTS storage_configs;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS configs; 