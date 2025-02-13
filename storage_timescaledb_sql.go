package main

const createTableSQL = `
CREATE TABLE IF NOT EXISTS weather (
    time timestamp WITH TIME ZONE NOT NULL,
    stationname text NULL,
    barometer float4 NULL,
    intemp float4 NULL,
    inhumidity float4 NULL,
    outtemp float4 NULL,
    windspeed float4 NULL,
    windspeed10 float4 NULL,
    winddir float4 NULL,
    windchill float4 NULL,
    heatindex float4 NULL,
    extratemp1 float4 NULL,
    extratemp2 float4 NULL,
    extratemp3 float4 NULL,
    extratemp4 float4 NULL,
    extratemp5 float4 NULL,
    extratemp6 float4 NULL,
    extratemp7 float4 NULL,
    soiltemp1 float4 NULL,
    soiltemp2 float4 NULL,
    soiltemp3 float4 NULL,
    soiltemp4 float4 NULL,
    leaftemp1 float4 NULL,
    leaftemp2 float4 NULL,
    leaftemp3 float4 NULL,
    leaftemp4 float4 NULL,
    outhumidity float4 NULL,
    extrahumidity1 float4 NULL,
    extrahumidity2 float4 NULL,
    extrahumidity3 float4 NULL,
    extrahumidity4 float4 NULL,
    extrahumidity5 float4 NULL,
    extrahumidity6 float4 NULL,
    extrahumidity7 float4 NULL,
    rainrate float4 NULL,
    rainincremental float4 NULL,
	uv float4 NULL,
    solarjoules float4 NULL,
    solarwatts float4 NULL,
	radiation float4 NULL,
    stormrain float4 NULL,
    stormstart timestamp WITH TIME ZONE NULL,
    dayrain float4 NULL,
    monthrain float4 NULL,
    yearrain float4 NULL,
    dayet float4 NULL,
    monthet float4 NULL,
    yearet float4 NULL,
    soilmoisture1 float4 NULL,
    soilmoisture2 float4 NULL,
    soilmoisture3 float4 NULL,
    soilmoisture4 float4 NULL,
    leafwetness1 float4 NULL,
    leafwetness2 float4 NULL,
    leafwetness3 float4 NULL,
    leafwetness4 float4 NULL,
    insidealarm int NULL,
    rainalarm int NULL,
    outsidealarm1 int NULL,
    outsidealarm2 int NULL,
    extraalarm1 int NULL,
    extraalarm2 int NULL,
    extraalarm3 int NULL,
    extraalarm4 int NULL,
    extraalarm5 int NULL,
    extraalarm6 int NULL,
    extraalarm7 int NULL,
    extraalarm8 int NULL,
    soilleafalarm1 int NULL,
    soilleafalarm2 int NULL,
    soilleafalarm3 int NULL,
    soilleafalarm4 int NULL,
    txbatterystatus int NULL,
    consbatteryvoltage float4 NULL,
    stationbatteryvoltage float4 NULL,
    forecasticon int NULL,
    forecastrule int NULL,
    sunrise TIMESTAMP WITH TIME ZONE NULL,
    sunset TIMESTAMP WITH TIME ZONE NULL
);`

const createExtensionSQL = `CREATE EXTENSION IF NOT EXISTS timescaledb;`

const createHypertableSQL = `SELECT create_hypertable('weather', 'time', if_not_exists => true);`

const createCircAvgStateTypeSQL = `CREATE TYPE circular_avg_state AS (
    sin_sum real,
    cos_sum real,
    accum real
  );
  `

const createCircAvgStateFunctionSQL = `CREATE OR REPLACE FUNCTION circular_avg_state_accumulator(state circular_avg_state, reading real)
RETURNS circular_avg_state
STRICT
IMMUTABLE
LANGUAGE plpgsql
AS $$
DECLARE
    sin_sum real;
    cos_sum real;
BEGIN
    sin_sum := state.sin_sum + SIND(reading);
    cos_sum := state.cos_sum + COSD(reading);
    RETURN ROW(sin_sum, cos_sum, state.accum + 1)::circular_avg_state;
END;
$$;
`

const createCircAvgCombinerFunctionSQL = `CREATE OR REPLACE FUNCTION circular_avg_state_combiner(state1 circular_avg_state, state2 circular_avg_state)
RETURNS circular_avg_state
STRICT
IMMUTABLE
LANGUAGE plpgsql
AS $$
DECLARE
    sin_sum real;
    cos_sum real;
    accum_sum real;
BEGIN
    sin_sum := state1.sin_sum + state2.sin_sum;
    cos_sum := state1.cos_sum + state2.cos_sum;
    accum_sum := state1.accum + state2.accum;
    RETURN ROW(sin_sum, cos_sum, accum_sum)::circular_avg_state;
END;
$$;`

const createCircAvgFinalizerFunctionSQL = `CREATE OR REPLACE FUNCTION circular_avg_final(state circular_avg_state)
RETURNS real
STRICT
IMMUTABLE
LANGUAGE plpgsql
AS $$
DECLARE
    sin_avg real;
    cos_avg real;
    atan2_result real;
    final_result real;
BEGIN
    sin_avg := state.sin_sum / state.accum;
    cos_avg := state.cos_sum / state.accum;
    atan2_result := ATAN2D(sin_avg, cos_avg);
    if atan2_result < 0 THEN
        final_result := atan2_result + 360;
    ELSE
        final_result := atan2_result;
    END IF;

    RETURN final_result;
END;
$$;
`

const createCircAvgAggregateFunctionSQL = `CREATE OR REPLACE AGGREGATE circular_avg (real)
(
    SFUNC = circular_avg_state_accumulator,
    STYPE = circular_avg_state,
    COMBINEFUNC = circular_avg_state_combiner,
    FINALFUNC = circular_avg_final,
    INITCOND = '(0,0,0)',
    PARALLEL = SAFE
);`

const create1mViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1m
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 minute', time) as bucket,
    stationname,
    avg(barometer) as barometer,
	max(barometer) as max_barometer,
	min(barometer) as min_barometer,
    avg(intemp) as intemp,
	max(intemp) as max_intemp,
	min(intemp) as min_intemp,
    avg(extratemp1) as extratemp1,
	max(extratemp1) as max_extratemp1,
	min(extratemp1) as min_extratemp1,
    avg(inhumidity) as inhumidity,
	max(inhumidity) as max_inhumidity,
	min(inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(outtemp) as max_outtemp,
	min(outtemp) as min_outtemp,
    avg(outhumidity) as outhumidity,
	max(outhumidity) as max_outhumidity,
	min(outhumidity) as min_outhumidity,
    avg(solarwatts) as solarwatts,
    avg(solarjoules) as solarjoules,
    circular_avg(winddir) as winddir,
    avg(windspeed) as windspeed,
    max(windspeed) as max_windspeed,
    avg(windchill) as windchill,
	min(windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(heatindex) as max_heatindex,
    sum(rainincremental) as period_rain,
    avg(rainrate) as rainrate,
    max(rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage,
    avg(stationbatteryvoltage) as stationbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname;`

const create5mViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_5m
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('5 minutes', time) as bucket,
    stationname,
    avg(barometer) as barometer,
	max(barometer) as max_barometer,
	min(barometer) as min_barometer,
    avg(intemp) as intemp,
	max(intemp) as max_intemp,
	min(intemp) as min_intemp,
    avg(extratemp1) as extratemp1,
	max(extratemp1) as max_extratemp1,
	min(extratemp1) as min_extratemp1,
    avg(inhumidity) as inhumidity,
	max(inhumidity) as max_inhumidity,
	min(inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(outtemp) as max_outtemp,
	min(outtemp) as min_outtemp,
    avg(outhumidity) as outhumidity,
	max(outhumidity) as max_outhumidity,
	min(outhumidity) as min_outhumidity,
    avg(solarwatts) as solarwatts,
    avg(solarjoules) as solarjoules,
    circular_avg(winddir) as winddir,
    avg(windspeed) as windspeed,
    max(windspeed) as max_windspeed,
    avg(windchill) as windchill,
	min(windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(heatindex) as max_heatindex,
    sum(rainincremental) as period_rain,
    avg(rainrate) as rainrate,
    max(rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage,
    avg(stationbatteryvoltage) as stationbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname;`

const create1hViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1h
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 hour', time) as bucket,
    stationname,
    avg(barometer) as barometer,
	max(barometer) as max_barometer,
	min(barometer) as min_barometer,
    avg(intemp) as intemp,
	max(intemp) as max_intemp,
	min(intemp) as min_intemp,
    avg(extratemp1) as extratemp1,
	max(extratemp1) as max_extratemp1,
	min(extratemp1) as min_extratemp1,
    avg(inhumidity) as inhumidity,
	max(inhumidity) as max_inhumidity,
	min(inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(outtemp) as max_outtemp,
	min(outtemp) as min_outtemp,
    avg(outhumidity) as outhumidity,
	max(outhumidity) as max_outhumidity,
	min(outhumidity) as min_outhumidity,
    avg(solarwatts) as solarwatts,
    avg(solarjoules) as solarjoules,
    circular_avg(winddir) as winddir,
    avg(windspeed) as windspeed,
    max(windspeed) as max_windspeed,
    avg(windchill) as windchill,
	min(windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(heatindex) as max_heatindex,
    sum(rainincremental) as period_rain,
    avg(rainrate) as rainrate,
    max(rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage,
    avg(stationbatteryvoltage) as stationbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname;`

const create1dViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1d
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 day', time) as bucket,
    stationname,
    avg(barometer) as barometer,
	max(barometer) as max_barometer,
	min(barometer) as min_barometer,
    avg(intemp) as intemp,
	max(intemp) as max_intemp,
	min(intemp) as min_intemp,
    avg(extratemp1) as extratemp1,
	max(extratemp1) as max_extratemp1,
	min(extratemp1) as min_extratemp1,
    avg(inhumidity) as inhumidity,
	max(inhumidity) as max_inhumidity,
	min(inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(outtemp) as max_outtemp,
	min(outtemp) as min_outtemp,
    avg(outhumidity) as outhumidity,
	max(outhumidity) as max_outhumidity,
	min(outhumidity) as min_outhumidity,
    avg(solarwatts) as solarwatts,
    avg(solarjoules) as solarjoules,
    circular_avg(winddir) as winddir,
    avg(windspeed) as windspeed,
    max(windspeed) as max_windspeed,
    avg(windchill) as windchill,
	min(windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(heatindex) as max_heatindex,
    sum(rainincremental) as period_rain,
    avg(rainrate) as rainrate,
    max(rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage,
    avg(stationbatteryvoltage) as stationbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname;`

const dropRainSinceMidnightViewSQL = `DROP VIEW IF EXISTS today_rainfall;`

const createRainSinceMidnightViewSQL = `CREATE VIEW today_rainfall AS
SELECT
    COALESCE(SUM(period_rain), 0) +
    (SELECT COALESCE(SUM(rainincremental), 0)
     FROM weather
     WHERE time >= (SELECT max(bucket) FROM weather_5m)) AS total_rain
FROM weather_5m
WHERE bucket >= date_trunc('day', now());`

const addAggregationPolicy1mSQL = `SELECT add_continuous_aggregate_policy('weather_1m', INTERVAL '1 month', INTERVAL '1 minute', INTERVAL '1 minute', if_not_exists => true);`
const addAggregationPolicy5mSQL = `SELECT add_continuous_aggregate_policy('weather_5m', INTERVAL '6 months', INTERVAL '5 minutes', INTERVAL '5 minutes', if_not_exists => true);`
const addAggregationPolicy1hSQL = `SELECT add_continuous_aggregate_policy('weather_1h', INTERVAL '2 years', INTERVAL '1 hour', INTERVAL '1 hour', if_not_exists => true);`
const addAggregationPolicy1dSQL = `SELECT add_continuous_aggregate_policy('weather_1d', INTERVAL '10 years', INTERVAL '1 day', INTERVAL '1 day', if_not_exists => true);`

const addRetentionPolicy = `SELECT add_retention_policy('weather', INTERVAL '14 days', if_not_exists => true);`
const addRetentionPolicy1m = `SELECT add_retention_policy('weather_1m', INTERVAL '1 month', if_not_exists => true);`
const addRetentionPolicy5m = `SELECT add_retention_policy('weather_5m', INTERVAL '6 months', if_not_exists => true);`
const addRetentionPolicy1h = `SELECT add_retention_policy('weather_1h', INTERVAL '2 years', if_not_exists => true);`
const addRetentionPolicy1d = `SELECT add_retention_policy('weather_1d', INTERVAL '10 years', if_not_exists => true);`

const createSnowDeltaFunctionSQL = `CREATE OR REPLACE FUNCTION calculate_snow_depth_delta(
    stationname TEXT,
    base_distance FLOAT,
    start_time TIMESTAMPTZ
) RETURNS FLOAT AS $$
DECLARE
    start_depth FLOAT;
    end_depth FLOAT;
    delta_time INTERVAL;
BEGIN
    -- Calculate the time difference between now and start_time
    delta_time := now() - start_time;

    -- Determine which view to use based on the time delta
    IF delta_time > INTERVAL '1 day' THEN
        -- Use weather_1d for start depth and weather for end depth
        SELECT base_distance - barometer INTO start_depth
        FROM weather_1d
        WHERE weather_1d.stationname = calculate_snow_depth_delta.stationname
        AND bucket = date_trunc('day', start_time);

        SELECT base_distance - barometer INTO end_depth
        FROM weather
        WHERE weather.stationname = calculate_snow_depth_delta.stationname
        ORDER BY time DESC
        LIMIT 1;

    ELSIF delta_time > INTERVAL '1 hour' THEN
        -- Use weather_1h for start depth and weather for end depth
        SELECT base_distance - barometer INTO start_depth
        FROM weather_1h
        WHERE weather_1h.stationname = calculate_snow_depth_delta.stationname
        AND bucket = date_trunc('hour', start_time);

        SELECT base_distance - barometer INTO end_depth
        FROM weather
        WHERE weather.stationname = calculate_snow_depth_delta.stationname
        ORDER BY time DESC
        LIMIT 1;

    ELSE 
        -- Use weather table for both start and end depth
        SELECT base_distance - barometer INTO start_depth
        FROM weather
        WHERE weather.stationname = calculate_snow_depth_delta.stationname
        AND time = (SELECT max(time) FROM weather WHERE time <= start_time AND stationname = calculate_snow_depth_delta.stationname);

        SELECT base_distance - barometer INTO end_depth
        FROM weather
        WHERE weather.stationname = calculate_snow_depth_delta.stationname
        ORDER BY time DESC
        LIMIT 1;
    END IF;

    -- Return the delta between end_depth and start_depth
    RETURN end_depth - start_depth;
END;
$$ LANGUAGE plpgsql;
`

const createSnowSeasonTotalSQL = `CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    total_snowfall FLOAT := 0.0;
    previous_depth FLOAT := NULL;
    current_depth FLOAT;
    current_bucket TIMESTAMPTZ;
    local_start_of_season TIMESTAMPTZ;
BEGIN
    -- If start_of_season is not provided, set it to the most recent October 1st
    IF start_of_season IS NULL THEN
        local_start_of_season := make_timestamptz(
            extract(YEAR FROM now())::INT, 10, 1, 0, 0, 0, current_setting('TimeZone')
        );
    ELSE
        local_start_of_season := start_of_season;
    END IF;

    -- Cursor to iterate through the weather_1d table in chronological order
    FOR current_bucket, current_depth IN 
        SELECT bucket, barometer 
        FROM weather_1d 
        WHERE weather_1d.stationname = calculate_total_snowfall.stationname
          AND bucket >= local_start_of_season
        ORDER BY bucket
    LOOP
        -- Check if the current depth is valid (not exceeding base_distance)
        IF current_depth <= base_distance THEN
            -- Handle initial case where previous_depth is NULL
            IF previous_depth IS NULL THEN
                previous_depth := current_depth;
            ELSE
                -- Calculate increase in snow depth, only adding if there's an increase
                IF current_depth > previous_depth THEN
                    total_snowfall := total_snowfall + (current_depth - previous_depth);
                END IF;
                previous_depth := current_depth;
            END IF;
        ELSE
            -- Log or handle invalid data point (barometer > base_distance)
            RAISE NOTICE 'Invalid data point for bucket % at station %: barometer value exceeds base_distance', current_bucket, stationname;
        END IF;
    END LOOP;

    -- Return the total snowfall, ensuring it's not negative or null
    RETURN GREATEST(total_snowfall, 0.0);
END;
$$ LANGUAGE plpgsql;
`

const createSnowStormTotalSQL = `
CREATE OR REPLACE FUNCTION calculate_storm_snowfall(
    stationname TEXT,
    base_distance FLOAT
) RETURNS TABLE (
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_snowfall FLOAT
) AS $$
DECLARE
    storm_id INTEGER := 0;
    previous_snowfall FLOAT := NULL;
    storm_start TIMESTAMPTZ := NULL;
    current_bucket TIMESTAMPTZ;
    current_snowfall FLOAT;
    hours_below_threshold INTEGER := 0;
    latest_measurement TIMESTAMPTZ;
    storm_detected BOOLEAN := FALSE;
BEGIN
    -- Get the latest measurement time from the weather table
    SELECT MAX(time) INTO latest_measurement FROM weather WHERE weather.stationname = calculate_storm_snowfall.stationname;

    FOR current_bucket, current_snowfall IN 
        SELECT bucket, base_distance - barometer AS snowfall_amount 
        FROM weather_1h
        WHERE weather_1h.stationname = calculate_storm_snowfall.stationname
        ORDER BY bucket
    LOOP
        -- Current snowfall (depth increase from base_distance)
        current_snowfall := GREATEST(current_snowfall, 0); -- Ensure no negative snowfall
        
        -- Detect storm start
        IF storm_start IS NULL AND current_snowfall >= 10 THEN
            storm_start := current_bucket;
            previous_snowfall := 0;
            storm_id := storm_id + 1;
            storm_detected := TRUE;
        END IF;

        -- Accumulate snowfall if part of a storm
        IF storm_start IS NOT NULL THEN
            IF current_snowfall >= 10 THEN
                -- Reset counter for consecutive hours below threshold
                hours_below_threshold := 0;
                previous_snowfall := previous_snowfall + current_snowfall;
            ELSE
                -- Increment counter for consecutive hours below threshold
                hours_below_threshold := hours_below_threshold + 1;

                -- Check if we've had 8 hours below threshold to end storm
                IF hours_below_threshold >= 8 THEN
                    RETURN QUERY SELECT storm_start, current_bucket, previous_snowfall;
                    storm_start := NULL;  -- Reset for next potential storm
                    previous_snowfall := NULL;
                END IF;
            END IF;
        END IF;

    END LOOP;

    -- Handle case where the last storm in data hasn't ended
    IF storm_start IS NOT NULL THEN
        RETURN QUERY SELECT storm_start, latest_measurement, previous_snowfall;
    END IF;

    -- If no storm was detected, return 0 for total snowfall
    IF NOT storm_detected THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
    END IF;

    RETURN;
END;
$$ LANGUAGE plpgsql;`
