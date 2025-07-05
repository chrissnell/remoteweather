-- Migrate existing TimescaleDB connection string data to individual components
-- This migration parses connection strings and populates the new component fields

-- Update records that have connection strings but empty component fields
UPDATE storage_configs 
SET 
    timescale_host = CASE 
        WHEN timescale_connection_string LIKE '%host=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'host=') + 5,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'host=') + 5), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'host=') + 5), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE 'localhost'
    END,
    timescale_port = CASE 
        WHEN timescale_connection_string LIKE '%port=%' THEN 
            CAST(TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'port=') + 5,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'port=') + 5), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'port=') + 5), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            )) AS INTEGER)
        ELSE 5432
    END,
    timescale_database = CASE 
        WHEN timescale_connection_string LIKE '%dbname=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'dbname=') + 7,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'dbname=') + 7), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'dbname=') + 7), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE 'weather'
    END,
    timescale_user = CASE 
        WHEN timescale_connection_string LIKE '%user=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'user=') + 5,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'user=') + 5), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'user=') + 5), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE 'weather'
    END,
    timescale_password = CASE 
        WHEN timescale_connection_string LIKE '%password=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'password=') + 9,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'password=') + 9), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'password=') + 9), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE ''
    END,
    timescale_ssl_mode = CASE 
        WHEN timescale_connection_string LIKE '%sslmode=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'sslmode=') + 8,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'sslmode=') + 8), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'sslmode=') + 8), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE 'prefer'
    END,
    timescale_timezone = CASE 
        WHEN timescale_connection_string LIKE '%TimeZone=%' THEN 
            TRIM(SUBSTR(timescale_connection_string, 
                INSTR(timescale_connection_string, 'TimeZone=') + 9,
                CASE 
                    WHEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'TimeZone=') + 9), ' ') > 0 
                    THEN INSTR(SUBSTR(timescale_connection_string, INSTR(timescale_connection_string, 'TimeZone=') + 9), ' ') - 1
                    ELSE LENGTH(timescale_connection_string)
                END
            ))
        ELSE ''
    END
WHERE backend_type = 'timescaledb' 
  AND timescale_connection_string IS NOT NULL 
  AND timescale_connection_string != ''
  AND (timescale_host = '' OR timescale_host IS NULL); 