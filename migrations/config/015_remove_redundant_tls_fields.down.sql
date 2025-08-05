-- Restore REST server TLS certificate fields to controller_configs table
ALTER TABLE controller_configs ADD COLUMN rest_cert TEXT;
ALTER TABLE controller_configs ADD COLUMN rest_key TEXT;