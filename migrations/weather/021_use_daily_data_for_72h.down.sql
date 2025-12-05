-- Migration 021 rollback: Restore hourly dual-threshold for 72h

DROP FUNCTION IF EXISTS get_new_snow_72h(TEXT, FLOAT);

CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS FLOAT AS $$
BEGIN
    RETURN get_new_snow_dual_threshold_from_table(
        p_stationname,
        p_base_distance,
        interval '72 hours',
        'weather_1h'
    );
END;
$$ LANGUAGE plpgsql;

COMMENT ON FUNCTION get_new_snow_72h(TEXT, FLOAT) IS
'Calculates 72h snowfall using dual-threshold on weather_1h.
Used by cache refresh job, not called directly by API handlers.';
