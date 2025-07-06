package timescaledb

const createTableSQL = `
CREATE TABLE IF NOT EXISTS weather (
    time timestamp WITH TIME ZONE NOT NULL,
    stationname text NULL,
    stationtype text NULL,
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
    potentialsolarwatts float4 NULL,
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
    sunset TIMESTAMP WITH TIME ZONE NULL,
    snowdistance float4 NULL,
    snowdepth float4 NULL,
    extrafloat1 float4 NULL,
    extrafloat2 float4 NULL,
    extrafloat3 float4 NULL,
    extrafloat4 float4 NULL,
    extrafloat5 float4 NULL,
    extrafloat6 float4 NULL,
    extrafloat7 float4 NULL,
    extrafloat8 float4 NULL,
    extrafloat9 float4 NULL,
    extrafloat10 float4 NULL,
    extratext1 text NULL,
    extratext2 text NULL,
    extratext3 text NULL,
    extratext4 text NULL,
    extratext5 text NULL,
    extratext6 text NULL,
    extratext7 text NULL,
    extratext8 text NULL,
    extratext9 text NULL,
    extratext10 text NULL
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
    RETURN ROW(sin_sum, cos_sum, state.accum + 1)::public.circular_avg_state;
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
    RETURN ROW(sin_sum, cos_sum, accum_sum)::public.circular_avg_state;
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
    STYPE = public.circular_avg_state,
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
    stationtype,
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
    avg(potentialsolarwatts) as potentialsolarwatts,
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
    avg(stationbatteryvoltage) as stationbatteryvoltage,
    avg(snowdistance) as snowdistance,
    avg(snowdepth) as snowdepth,
    avg(extrafloat1) as extrafloat1,
    avg(extrafloat2) as extrafloat2,
    avg(extrafloat3) as extrafloat3,
    avg(extrafloat4) as extrafloat4,
    avg(extrafloat5) as extrafloat5,
    avg(extrafloat6) as extrafloat6,
    avg(extrafloat7) as extrafloat7,
    avg(extrafloat8) as extrafloat8,
    avg(extrafloat9) as extrafloat9,
    avg(extrafloat10) as extrafloat10,
    max(extrafloat1) as max_extrafloat1,
    max(extrafloat2) as max_extrafloat2,
    max(extrafloat3) as max_extrafloat3,
    max(extrafloat4) as max_extrafloat4,
    max(extrafloat5) as max_extrafloat5,
    max(extrafloat6) as max_extrafloat6,
    max(extrafloat7) as max_extrafloat7,
    max(extrafloat8) as max_extrafloat8,
    max(extrafloat9) as max_extrafloat9,
    max(extrafloat10) as max_extrafloat10,
    min(extrafloat1) as min_extrafloat1,
    min(extrafloat2) as min_extrafloat2,
    min(extrafloat3) as min_extrafloat3,
    min(extrafloat4) as min_extrafloat4,
    min(extrafloat5) as min_extrafloat5,
    min(extrafloat6) as min_extrafloat6,
    min(extrafloat7) as min_extrafloat7,
    min(extrafloat8) as min_extrafloat8,
    min(extrafloat9) as min_extrafloat9,
    min(extrafloat10) as min_extrafloat10
FROM
    weather
GROUP BY bucket, stationname, stationtype;`

const create5mViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_5m
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('5 minutes', time) as bucket,
    stationname,
    stationtype,
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
    avg(potentialsolarwatts) as potentialsolarwatts,
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
    avg(stationbatteryvoltage) as stationbatteryvoltage,
    avg(snowdistance) as snowdistance,
    avg(snowdepth) as snowdepth,
    avg(extrafloat1) as extrafloat1,
    avg(extrafloat2) as extrafloat2,
    avg(extrafloat3) as extrafloat3,
    avg(extrafloat4) as extrafloat4,
    avg(extrafloat5) as extrafloat5,
    avg(extrafloat6) as extrafloat6,
    avg(extrafloat7) as extrafloat7,
    avg(extrafloat8) as extrafloat8,
    avg(extrafloat9) as extrafloat9,
    avg(extrafloat10) as extrafloat10,
    max(extrafloat1) as max_extrafloat1,
    max(extrafloat2) as max_extrafloat2,
    max(extrafloat3) as max_extrafloat3,
    max(extrafloat4) as max_extrafloat4,
    max(extrafloat5) as max_extrafloat5,
    max(extrafloat6) as max_extrafloat6,
    max(extrafloat7) as max_extrafloat7,
    max(extrafloat8) as max_extrafloat8,
    max(extrafloat9) as max_extrafloat9,
    max(extrafloat10) as max_extrafloat10,
    min(extrafloat1) as min_extrafloat1,
    min(extrafloat2) as min_extrafloat2,
    min(extrafloat3) as min_extrafloat3,
    min(extrafloat4) as min_extrafloat4,
    min(extrafloat5) as min_extrafloat5,
    min(extrafloat6) as min_extrafloat6,
    min(extrafloat7) as min_extrafloat7,
    min(extrafloat8) as min_extrafloat8,
    min(extrafloat9) as min_extrafloat9,
    min(extrafloat10) as min_extrafloat10
FROM
    weather
GROUP BY bucket, stationname, stationtype;`

const create1hViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1h
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 hour', time) as bucket,
    stationname,
    stationtype,
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
    avg(potentialsolarwatts) as potentialsolarwatts,
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
    avg(stationbatteryvoltage) as stationbatteryvoltage,
    avg(snowdistance) as snowdistance,
    avg(snowdepth) as snowdepth,
    avg(extrafloat1) as extrafloat1,
    avg(extrafloat2) as extrafloat2,
    avg(extrafloat3) as extrafloat3,
    avg(extrafloat4) as extrafloat4,
    avg(extrafloat5) as extrafloat5,
    avg(extrafloat6) as extrafloat6,
    avg(extrafloat7) as extrafloat7,
    avg(extrafloat8) as extrafloat8,
    avg(extrafloat9) as extrafloat9,
    avg(extrafloat10) as extrafloat10,
    max(extrafloat1) as max_extrafloat1,
    max(extrafloat2) as max_extrafloat2,
    max(extrafloat3) as max_extrafloat3,
    max(extrafloat4) as max_extrafloat4,
    max(extrafloat5) as max_extrafloat5,
    max(extrafloat6) as max_extrafloat6,
    max(extrafloat7) as max_extrafloat7,
    max(extrafloat8) as max_extrafloat8,
    max(extrafloat9) as max_extrafloat9,
    max(extrafloat10) as max_extrafloat10,
    min(extrafloat1) as min_extrafloat1,
    min(extrafloat2) as min_extrafloat2,
    min(extrafloat3) as min_extrafloat3,
    min(extrafloat4) as min_extrafloat4,
    min(extrafloat5) as min_extrafloat5,
    min(extrafloat6) as min_extrafloat6,
    min(extrafloat7) as min_extrafloat7,
    min(extrafloat8) as min_extrafloat8,
    min(extrafloat9) as min_extrafloat9,
    min(extrafloat10) as min_extrafloat10
FROM
    weather
GROUP BY bucket, stationname, stationtype;`

const create1dViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1d
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 day', time) as bucket,
    stationname,
    stationtype,
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
    avg(potentialsolarwatts) as potentialsolarwatts,
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
    avg(stationbatteryvoltage) as stationbatteryvoltage,
    avg(snowdistance) as snowdistance,
    avg(snowdepth) as snowdepth,
    avg(extrafloat1) as extrafloat1,
    avg(extrafloat2) as extrafloat2,
    avg(extrafloat3) as extrafloat3,
    avg(extrafloat4) as extrafloat4,
    avg(extrafloat5) as extrafloat5,
    avg(extrafloat6) as extrafloat6,
    avg(extrafloat7) as extrafloat7,
    avg(extrafloat8) as extrafloat8,
    avg(extrafloat9) as extrafloat9,
    avg(extrafloat10) as extrafloat10,
    max(extrafloat1) as max_extrafloat1,
    max(extrafloat2) as max_extrafloat2,
    max(extrafloat3) as max_extrafloat3,
    max(extrafloat4) as max_extrafloat4,
    max(extrafloat5) as max_extrafloat5,
    max(extrafloat6) as max_extrafloat6,
    max(extrafloat7) as max_extrafloat7,
    max(extrafloat8) as max_extrafloat8,
    max(extrafloat9) as max_extrafloat9,
    max(extrafloat10) as max_extrafloat10,
    min(extrafloat1) as min_extrafloat1,
    min(extrafloat2) as min_extrafloat2,
    min(extrafloat3) as min_extrafloat3,
    min(extrafloat4) as min_extrafloat4,
    min(extrafloat5) as min_extrafloat5,
    min(extrafloat6) as min_extrafloat6,
    min(extrafloat7) as min_extrafloat7,
    min(extrafloat8) as min_extrafloat8,
    min(extrafloat9) as min_extrafloat9,
    min(extrafloat10) as min_extrafloat10
FROM
    weather
GROUP BY bucket, stationname, stationtype;`

const dropRainSinceMidnightViewSQL = `DROP VIEW IF EXISTS today_rainfall;`

const createRainSinceMidnightViewSQL = `CREATE VIEW today_rainfall AS
SELECT
    COALESCE(SUM(period_rain), 0) +
    (SELECT COALESCE(SUM(rainincremental), 0)
     FROM weather
     WHERE time >= (SELECT max(bucket) FROM weather_5m)) AS total_rain
FROM weather_5m
WHERE bucket >= date_trunc('day', now());`

const createIndexesSQL = `
CREATE INDEX IF NOT EXISTS weather_1m_bucket_stationname_idx ON weather_1m (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_5m_bucket_stationname_idx ON weather_5m (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_1h_bucket_stationname_idx ON weather_1h (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_1d_bucket_stationname_idx ON weather_1d (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_stationname_time_idx ON weather (stationname, time DESC);`

const addAggregationPolicy1mSQL = `SELECT add_continuous_aggregate_policy('weather_1m', INTERVAL '1 month', INTERVAL '1 minute', INTERVAL '1 minute', if_not_exists => true);`
const addAggregationPolicy5mSQL = `SELECT add_continuous_aggregate_policy('weather_5m', INTERVAL '6 months', INTERVAL '5 minutes', INTERVAL '5 minutes', if_not_exists => true);`
const addAggregationPolicy1hSQL = `SELECT add_continuous_aggregate_policy('weather_1h', INTERVAL '2 years', INTERVAL '1 hour', INTERVAL '1 hour', if_not_exists => true);`
const addAggregationPolicy1dSQL = `SELECT add_continuous_aggregate_policy('weather_1d', INTERVAL '10 years', INTERVAL '1 day', INTERVAL '1 day', if_not_exists => true);`

const addRetentionPolicySQL = `SELECT add_retention_policy('weather', INTERVAL '365 days', if_not_exists => true);`
const addRetentionPolicy1mSQL = `SELECT add_retention_policy('weather_1m', INTERVAL '1 month', if_not_exists => true);`
const addRetentionPolicy5mSQL = `SELECT add_retention_policy('weather_5m', INTERVAL '6 months', if_not_exists => true);`
const addRetentionPolicy1hSQL = `SELECT add_retention_policy('weather_1h', INTERVAL '2 years', if_not_exists => true);`
const addRetentionPolicy1dSQL = `SELECT add_retention_policy('weather_1d', INTERVAL '10 years', if_not_exists => true);`

const addSnowDepthCalculations = `SELECT 1;` // Combined into one statement to execute snow functions

const createSnowDelta72hSQL = `CREATE OR REPLACE FUNCTION get_new_snow_72h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;  -- The earliest sensor reading in the last 72 hours
    latest_reading FLOAT; -- The latest sensor reading in the last 72 hours
BEGIN
    -- Get the earliest sensor reading in the last 72 hours.
    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '72 hours'
     ORDER BY time ASC
     LIMIT 1;

    -- Get the latest sensor reading in the last 72 hours.
    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '72 hours'
     ORDER BY time DESC
     LIMIT 1;

    -- If there are no readings, return NULL.
    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    -- Calculate snowfall as the difference between the initial and latest readings.
    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;`

const createSnowDelta24hSQL = `CREATE OR REPLACE FUNCTION get_new_snow_24h(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;  -- The earliest sensor reading in the last 24 hours
    latest_reading FLOAT; -- The latest sensor reading in the last 24 hours
BEGIN
    -- Get the earliest sensor reading in the last 24 hours.
    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '24 hours'
     ORDER BY time ASC
     LIMIT 1;

    -- Get the latest sensor reading in the last 24 hours.
    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= now() - interval '24 hours'
     ORDER BY time DESC
     LIMIT 1;

    -- If there are no readings, return NULL.
    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    -- Calculate snowfall as the difference between the initial and latest readings.
    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;`

const createSnowDeltaSinceMidnightSQL = `CREATE OR REPLACE FUNCTION get_new_snow_midnight(
    p_stationname TEXT,
    p_base_distance FLOAT
) RETURNS TABLE(snowfall FLOAT) AS $$
DECLARE
    first_reading FLOAT;  -- The earliest sensor reading since midnight
    latest_reading FLOAT; -- The latest sensor reading since midnight
    midnight TIMESTAMPTZ;
BEGIN
    -- Define midnight for the current day.
    midnight := date_trunc('day', now());

    -- Get the earliest sensor reading since midnight.
    SELECT snowdistance
      INTO first_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= midnight
     ORDER BY time ASC
     LIMIT 1;

    -- Get the latest sensor reading since midnight.
    SELECT snowdistance
      INTO latest_reading
      FROM weather
     WHERE stationname = p_stationname
       AND time >= midnight
     ORDER BY time DESC
     LIMIT 1;

    -- If there are no readings, return NULL.
    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN;
    END IF;

    -- Calculate snowfall as the difference between the initial and latest readings.
    -- A higher first_reading and a lower latest_reading indicate snow accumulation.
    RETURN QUERY SELECT first_reading - latest_reading AS snowfall;
END;
$$ LANGUAGE plpgsql;`

const createSnowSeasonTotalSQL = `CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
    p_stationname TEXT,
    base_distance FLOAT,
    start_of_season TIMESTAMPTZ = NULL
) RETURNS FLOAT AS $$
DECLARE
    total_snowfall FLOAT := 0.0;
    previous_snowdistance FLOAT := NULL;
    current_snowdistance FLOAT;
    current_bucket TIMESTAMPTZ;
    local_start_of_season TIMESTAMPTZ;
    season_end TIMESTAMPTZ;
    current_year INTEGER;
    current_month INTEGER;
    today_snowfall FLOAT := 0.0;
    today_previous_snowdistance FLOAT := NULL;
    today_current_snowdistance FLOAT;
    midnight TIMESTAMPTZ;
BEGIN
    -- Determine the current snow season (October 1 to May 1)
    IF start_of_season IS NULL THEN
        current_year := extract(YEAR FROM now())::INT;
        current_month := extract(MONTH FROM now())::INT;
        
        -- Determine which season we're in
        IF current_month >= 10 THEN
            -- October-December: current season (Oct 1 current year to May 1 next year)
            local_start_of_season := make_timestamptz(current_year, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSIF current_month <= 4 THEN
            -- January-April: current season (Oct 1 previous year to May 1 current year)
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        ELSE
            -- May-September: off-season, use most recent completed season
            local_start_of_season := make_timestamptz(current_year - 1, 10, 1, 0, 0, 0, current_setting('TimeZone'));
        END IF;
    ELSE
        local_start_of_season := start_of_season;
    END IF;

    -- Calculate season end (May 1 of the following year)
    season_end := local_start_of_season + interval '7 months';

    -- Iterate through weather_1d table to calculate snowfall from daily deltas
    FOR current_bucket, current_snowdistance IN 
        SELECT bucket, snowdistance 
        FROM weather_1d 
        WHERE weather_1d.stationname = p_stationname
          AND bucket >= local_start_of_season
          AND bucket < season_end
          AND snowdistance IS NOT NULL
        ORDER BY bucket
    LOOP
        -- Check if the current snowdistance is valid (not exceeding base_distance)
        IF current_snowdistance <= base_distance THEN
            -- Handle initial case where previous_snowdistance is NULL
            IF previous_snowdistance IS NOT NULL THEN
                -- Calculate snowfall: if snowdistance decreased, snow fell
                IF current_snowdistance < previous_snowdistance THEN
                    total_snowfall := total_snowfall + (previous_snowdistance - current_snowdistance);
                END IF;
            END IF;
            previous_snowdistance := current_snowdistance;
        END IF;
    END LOOP;

    -- Add today's snowfall from weather_1h since midnight (only if we're in the current season)
    IF now() >= local_start_of_season AND now() < season_end THEN
        midnight := date_trunc('day', now());
        
        FOR current_bucket, today_current_snowdistance IN 
            SELECT bucket, snowdistance 
            FROM weather_1h 
            WHERE weather_1h.stationname = p_stationname
              AND bucket >= midnight
              AND snowdistance IS NOT NULL
            ORDER BY bucket
        LOOP
            -- Check if the current snowdistance is valid (not exceeding base_distance)
            IF today_current_snowdistance <= base_distance THEN
                -- Handle initial case where previous_snowdistance is NULL
                IF today_previous_snowdistance IS NOT NULL THEN
                    -- Calculate snowfall: if snowdistance decreased, snow fell
                    IF today_current_snowdistance < today_previous_snowdistance THEN
                        today_snowfall := today_snowfall + (today_previous_snowdistance - today_current_snowdistance);
                    END IF;
                END IF;
                today_previous_snowdistance := today_current_snowdistance;
            END IF;
        END LOOP;
        
        total_snowfall := total_snowfall + today_snowfall;
    END IF;

    -- Return the total snowfall, ensuring it's not negative
    RETURN GREATEST(total_snowfall, 0.0);
END;
$$ LANGUAGE plpgsql;
`

const createSnowStormTotalSQL = `CREATE OR REPLACE FUNCTION calculate_storm_snowfall(
    p_stationname TEXT
) RETURNS TABLE (
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_snowfall FLOAT
) AS $$
DECLARE
    storm_start_ts TIMESTAMPTZ := NULL;
    storm_end_ts TIMESTAMPTZ := NULL;
    total_snowfall_amount FLOAT := 0.0;
    current_bucket TIMESTAMPTZ;
    current_snowdistance FLOAT;
    previous_snowdistance FLOAT;
    snowfall_this_hour FLOAT;
    consecutive_no_snowfall INTEGER := 0;
    found_storm_start BOOLEAN := FALSE;
    buckets_array TIMESTAMPTZ[];
    snowdistance_array FLOAT[];
    i INTEGER;
    j INTEGER;
BEGIN
    -- First, collect all hourly data into arrays for easier processing
    SELECT array_agg(bucket ORDER BY bucket DESC), array_agg(snowdistance ORDER BY bucket DESC)
    INTO buckets_array, snowdistance_array
    FROM weather_1h
    WHERE weather_1h.stationname = p_stationname
      AND bucket >= now() - interval '30 days'  -- Look back 30 days maximum
      AND snowdistance IS NOT NULL
    ORDER BY bucket DESC;

    -- If no data, return zeros
    IF buckets_array IS NULL OR array_length(buckets_array, 1) = 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Step 1: Find storm end (most recent positive snowfall within 24h, or current time if recent)
    FOR i IN 1..array_length(buckets_array, 1) LOOP
        -- Skip first bucket (no previous to compare with)
        IF i = 1 THEN
            CONTINUE;
        END IF;

        current_bucket := buckets_array[i];
        current_snowdistance := snowdistance_array[i];
        previous_snowdistance := snowdistance_array[i-1];

        -- Check if this hour had positive snowfall (>= 10mm decrease)
        IF previous_snowdistance - current_snowdistance >= 10 THEN
            -- Found positive snowfall
            IF storm_end_ts IS NULL THEN
                -- This is our storm end (most recent positive snowfall)
                storm_end_ts := current_bucket;
            END IF;
        END IF;

        -- Stop looking if we're beyond 24 hours
        IF current_bucket < now() - interval '24 hours' THEN
            EXIT;
        END IF;
    END LOOP;

    -- If no storm end found, no recent storm
    IF storm_end_ts IS NULL THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Step 2: Find storm start (working backward from storm end, find first bucket with 5 consecutive no-snowfall buckets before it)
    FOR i IN 1..array_length(buckets_array, 1) LOOP
        current_bucket := buckets_array[i];
        
        -- Stop if we've gone past our storm end
        IF current_bucket > storm_end_ts THEN
            CONTINUE;
        END IF;

        -- Check if this bucket has 5 consecutive no-snowfall buckets before it
        consecutive_no_snowfall := 0;
        
        -- Look at the 5 buckets before this one
        FOR j IN (i+1)..(i+5) LOOP
            -- Make sure we don't go beyond array bounds
            IF j > array_length(buckets_array, 1) THEN
                EXIT;
            END IF;
            
            -- Skip if this is the first bucket in the sequence (no previous to compare)
            IF j = array_length(buckets_array, 1) THEN
                consecutive_no_snowfall := consecutive_no_snowfall + 1;
                CONTINUE;
            END IF;
            
            -- Check if this bucket had no positive snowfall
            IF snowdistance_array[j+1] - snowdistance_array[j] < 10 THEN
                consecutive_no_snowfall := consecutive_no_snowfall + 1;
            ELSE
                EXIT; -- Break the inner loop if we found positive snowfall
            END IF;
        END LOOP;

        -- If we found 5 consecutive hours of no snowfall, this is our storm start
        IF consecutive_no_snowfall = 5 THEN
            storm_start_ts := current_bucket;
            found_storm_start := TRUE;
            EXIT;
        END IF;
    END LOOP;

    -- If no storm start found, use the earliest bucket in our dataset
    IF NOT found_storm_start THEN
        storm_start_ts := buckets_array[array_length(buckets_array, 1)];
    END IF;

    -- Step 3: Calculate total snowfall between storm start and storm end
    FOR i IN 1..array_length(buckets_array, 1) LOOP
        current_bucket := buckets_array[i];
        
        -- Skip buckets outside our storm window
        IF current_bucket > storm_end_ts OR current_bucket < storm_start_ts THEN
            CONTINUE;
        END IF;

        -- Skip first bucket (no previous to compare with)
        IF i = 1 THEN
            CONTINUE;
        END IF;

        current_snowdistance := snowdistance_array[i];
        previous_snowdistance := snowdistance_array[i-1];

        -- Add positive snowfall (snowdistance decreased)
        snowfall_this_hour := previous_snowdistance - current_snowdistance;
        IF snowfall_this_hour > 0 THEN
            total_snowfall_amount := total_snowfall_amount + snowfall_this_hour;
        END IF;
    END LOOP;

    -- Return the storm information
    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;`
