# remoteweather TimescaleDB Provisioner

A command-line tool to provision TimescaleDB database and user for remoteweather.

## Features

- Creates PostgreSQL database with proper UTF-8 encoding
- Enables TimescaleDB extension automatically
- Creates database user with generated secure password
- Grants all necessary privileges
- Updates remoteweather config.db with connection details
- Provides status and test commands

## Installation

### From Source

```bash
cd cmd/remoteweather-timescaledb-provisioner
go build -o remoteweather-timescaledb-provisioner
```

### Install with go install

```bash
go install github.com/chrissnell/remoteweather/cmd/remoteweather-timescaledb-provisioner@latest
```

## Prerequisites

- PostgreSQL 12+ with TimescaleDB extension installed
- Access to PostgreSQL admin account (usually `postgres`)
- remoteweather config.db file

## Usage

### Initialize Database (Interactive)

```bash
remoteweather-timescaledb-provisioner init --interactive
```

This will prompt you for the PostgreSQL admin password and use default values for all other settings.

### Initialize Database (Non-Interactive)

Using environment variable for password:

```bash
export POSTGRES_ADMIN_PASSWORD='yourpassword'
remoteweather-timescaledb-provisioner init
```

Or pass password directly:

```bash
remoteweather-timescaledb-provisioner init \
  --postgres-admin-password yourpassword
```

### Custom Configuration

```bash
remoteweather-timescaledb-provisioner init \
  --db-name myweatherdb \
  --db-user myweatheruser \
  --postgres-host 192.168.1.100 \
  --postgres-port 5432 \
  --postgres-admin postgres \
  --postgres-admin-password secret \
  --ssl-mode prefer \
  --timezone UTC \
  --config-db /path/to/config.db
```

### Check Configuration Status

```bash
remoteweather-timescaledb-provisioner status --config-db ./config.db
```

### Test Database Connection

```bash
remoteweather-timescaledb-provisioner test --config-db ./config.db
```

## Default Values

| Setting | Default Value |
|---------|--------------|
| Database Name | `remoteweather` |
| Database User | `remoteweather` |
| PostgreSQL Host | `localhost` |
| PostgreSQL Port | `5432` |
| PostgreSQL Admin | `postgres` |
| SSL Mode | `prefer` |
| Timezone | `UTC` |
| Config DB Path | `./config.db` |

## What It Does

1. **Pre-flight Checks**
   - Verifies PostgreSQL is accessible
   - Confirms TimescaleDB extension is available
   - Validates config.db exists and is valid
   - Checks for existing database/user conflicts

2. **Database Creation**
   - Creates database with UTF-8 encoding
   - Sets locale to `en_US.UTF-8`

3. **TimescaleDB Setup**
   - Enables TimescaleDB extension on the new database

4. **User Creation**
   - Generates secure 24-character random password
   - Creates database user
   - Grants all privileges on database
   - Grants schema privileges (public schema)
   - Sets up default privileges for future tables, sequences, and functions

5. **Configuration Update**
   - Updates remoteweather config.db with connection details
   - Stores connection information in `storage_configs` table

6. **Verification**
   - Tests connection with new user credentials
   - Verifies TimescaleDB extension is enabled
   - Confirms user has table creation privileges

## Password Security

- Generates cryptographically secure 24-character passwords
- Includes uppercase, lowercase, numbers, and special characters
- Displays password once during provisioning (save it!)
- Automatically stores password in config.db for remoteweather

## What It Does NOT Do

- Does NOT run migrations (remoteweather handles this)
- Does NOT create schema/tables (remoteweather does this automatically)
- Does NOT set up hypertables or aggregation policies (remoteweather handles this)

## After Provisioning

Once provisioning is complete, simply start remoteweather:

```bash
./remoteweather --config config.db
```

remoteweather will automatically:
- Connect to TimescaleDB using the provisioned credentials
- Create all necessary tables and hypertables
- Set up aggregation policies and retention policies
- Run any pending migrations

## Troubleshooting

### "PostgreSQL connection failed"

Ensure PostgreSQL is running and accessible:
```bash
psql -h localhost -p 5432 -U postgres -d postgres
```

### "TimescaleDB extension not available"

Install TimescaleDB extension (see main README for installation instructions).

### "Config database not found"

Make sure you're running the command from the correct directory or specify the full path:
```bash
remoteweather-timescaledb-provisioner init --config-db /full/path/to/config.db
```

### "Database or user already exists"

If you need to recreate, manually drop the existing database and user first:
```sql
DROP DATABASE remoteweather;
DROP USER remoteweather;
```

## Example Output

```
🚀 remoteweather TimescaleDB Provisioner
========================================

Configuration:
  PostgreSQL Host: localhost:5432
  Database Name: remoteweather
  Database User: remoteweather
  SSL Mode: prefer
  Timezone: UTC
  Config DB: ./config.db

🔍 Pre-flight Checks
✅ PostgreSQL connection successful
✅ TimescaleDB extension available
✅ Config database found: ./config.db
✅ No existing database/user conflicts

🗄️  Creating Database
✅ Database 'remoteweather' created with UTF8 encoding

🔌 Enabling TimescaleDB Extension
✅ TimescaleDB extension enabled (version 2.14.2)

👤 Creating User
✅ User 'remoteweather' created
✅ Database privileges granted
✅ Schema and default privileges granted

🔐 Generated Password
╔════════════════════════════════════════════════╗
║  ⚠️  SAVE THIS PASSWORD - IT WON'T BE SHOWN AGAIN  ║
╚════════════════════════════════════════════════╝

  Password: Kp9$mX2#nQ@7vL4!wR8^zY3&

This password has been saved to your config.db
and will be used by remoteweather automatically.

⚙️  Updating Configuration
✅ Config database updated with connection details

🔍 Verifying Connection
✅ Connection verified

✅ Provisioning Complete!

Connection Details:
  Host: localhost:5432
  Database: remoteweather
  User: remoteweather
  SSL Mode: prefer
  TimescaleDB: enabled

Next Steps:
  1. Start remoteweather: ./remoteweather --config config.db
  2. remoteweather will automatically:
     ✓ Connect to TimescaleDB
     ✓ Create all tables and hypertables
     ✓ Set up aggregation policies
     ✓ Run any pending migrations

Manual Connection (if needed):
  psql -h localhost -p 5432 -U remoteweather -d remoteweather
```
