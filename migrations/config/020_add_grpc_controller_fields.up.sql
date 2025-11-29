-- Add gRPC-specific fields to controller_configs table
ALTER TABLE controller_configs ADD COLUMN grpc_port INTEGER;
ALTER TABLE controller_configs ADD COLUMN grpc_listen_addr TEXT;
ALTER TABLE controller_configs ADD COLUMN grpc_cert TEXT;
ALTER TABLE controller_configs ADD COLUMN grpc_key TEXT;

-- Set sensible defaults for existing configurations
-- This ensures smooth transition without breaking existing setups
UPDATE controller_configs
SET grpc_port = 50051
WHERE grpc_port IS NULL;

UPDATE controller_configs
SET grpc_listen_addr = COALESCE(rest_listen_addr, '0.0.0.0')
WHERE grpc_listen_addr IS NULL;

-- Log the migration
-- (Informational comment: gRPC now runs on separate port from REST)
