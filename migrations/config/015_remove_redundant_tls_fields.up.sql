-- Remove unused REST server TLS certificate fields from controller_configs table
-- REST server now uses TLS certificates from weather_websites table
-- Note: We keep management_cert and management_key as those are still used by the management API
-- Note: We keep devices.tls_cert_file and devices.tls_key_file as those are used by grpcreceiver device type
ALTER TABLE controller_configs DROP COLUMN rest_cert;
ALTER TABLE controller_configs DROP COLUMN rest_key;