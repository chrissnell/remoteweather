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
    avg(inhumidity) as inhumidity,
	max(inhumidity) as max_inhumidity,
	min(inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(outtemp) as max_outtemp,
	min(outtemp) as min_outtemp,
    avg(windspeed) as windspeed,
	max(windspeed) as max_windspeed,
    circular_avg(winddir) as winddir,
    avg(windchill) as windchill,
	min(windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(heatindex) as max_heatindex,
    avg(outhumidity) as outhumidity,
	max(outhumidity) as max_outhumidity,
	min(outhumidity) as min_outhumidity,
    avg(rainrate) as rainrate,
	max(rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(uv) as uv,
	max(uv) as max_uv,
    avg(solarjoules) as solarjoules,
    avg(solarwatts) as solarwatts,
	max(solarwatts) as max_solarwatts,
    avg(potentialsolarwatts) as potentialsolarwatts,
	max(potentialsolarwatts) as max_potentialsolarwatts,
    avg(radiation) as radiation,
	max(radiation) as max_radiation
FROM weather
GROUP BY bucket, stationname, stationtype
WITH NO DATA;`

const create5mViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_5m
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('5 minutes', bucket) as bucket,
    stationname,
    stationtype,
    avg(barometer) as barometer,
	max(max_barometer) as max_barometer,
	min(min_barometer) as min_barometer,
    avg(intemp) as intemp,
	max(max_intemp) as max_intemp,
	min(min_intemp) as min_intemp,
    avg(inhumidity) as inhumidity,
	max(max_inhumidity) as max_inhumidity,
	min(min_inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(max_outtemp) as max_outtemp,
	min(min_outtemp) as min_outtemp,
    avg(windspeed) as windspeed,
	max(max_windspeed) as max_windspeed,
    circular_avg(winddir) as winddir,
    avg(windchill) as windchill,
	min(min_windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(max_heatindex) as max_heatindex,
    avg(outhumidity) as outhumidity,
	max(max_outhumidity) as max_outhumidity,
	min(min_outhumidity) as min_outhumidity,
    avg(rainrate) as rainrate,
	max(max_rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(uv) as uv,
	max(max_uv) as max_uv,
    avg(solarjoules) as solarjoules,
    avg(solarwatts) as solarwatts,
	max(max_solarwatts) as max_solarwatts,
    avg(potentialsolarwatts) as potentialsolarwatts,
	max(max_potentialsolarwatts) as max_potentialsolarwatts,
    avg(radiation) as radiation,
	max(max_radiation) as max_radiation
FROM weather_1m
GROUP BY bucket, stationname, stationtype
WITH NO DATA;`

const create1hViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1h
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 hour', bucket) as bucket,
    stationname,
    stationtype,
    avg(barometer) as barometer,
	max(max_barometer) as max_barometer,
	min(min_barometer) as min_barometer,
    avg(intemp) as intemp,
	max(max_intemp) as max_intemp,
	min(min_intemp) as min_intemp,
    avg(inhumidity) as inhumidity,
	max(max_inhumidity) as max_inhumidity,
	min(min_inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(max_outtemp) as max_outtemp,
	min(min_outtemp) as min_outtemp,
    avg(windspeed) as windspeed,
	max(max_windspeed) as max_windspeed,
    circular_avg(winddir) as winddir,
    avg(windchill) as windchill,
	min(min_windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(max_heatindex) as max_heatindex,
    avg(outhumidity) as outhumidity,
	max(max_outhumidity) as max_outhumidity,
	min(min_outhumidity) as min_outhumidity,
    avg(rainrate) as rainrate,
	max(max_rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(uv) as uv,
	max(max_uv) as max_uv,
    avg(solarjoules) as solarjoules,
    avg(solarwatts) as solarwatts,
	max(max_solarwatts) as max_solarwatts,
    avg(potentialsolarwatts) as potentialsolarwatts,
	max(max_potentialsolarwatts) as max_potentialsolarwatts,
    avg(radiation) as radiation,
	max(max_radiation) as max_radiation
FROM weather_5m
GROUP BY bucket, stationname, stationtype
WITH NO DATA;`

const create1dViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1d
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('1 day', bucket) as bucket,
    stationname,
    stationtype,
    avg(barometer) as barometer,
	max(max_barometer) as max_barometer,
	min(min_barometer) as min_barometer,
    avg(intemp) as intemp,
	max(max_intemp) as max_intemp,
	min(min_intemp) as min_intemp,
    avg(inhumidity) as inhumidity,
	max(max_inhumidity) as max_inhumidity,
	min(min_inhumidity) as min_inhumidity,
    avg(outtemp) as outtemp,
	max(max_outtemp) as max_outtemp,
	min(min_outtemp) as min_outtemp,
    avg(windspeed) as windspeed,
	max(max_windspeed) as max_windspeed,
    circular_avg(winddir) as winddir,
    avg(windchill) as windchill,
	min(min_windchill) as min_windchill,
    avg(heatindex) as heatindex,
	max(max_heatindex) as max_heatindex,
    avg(outhumidity) as outhumidity,
	max(max_outhumidity) as max_outhumidity,
	min(min_outhumidity) as min_outhumidity,
    avg(rainrate) as rainrate,
	max(max_rainrate) as max_rainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(uv) as uv,
	max(max_uv) as max_uv,
    avg(solarjoules) as solarjoules,
    avg(solarwatts) as solarwatts,
	max(max_solarwatts) as max_solarwatts,
    avg(potentialsolarwatts) as potentialsolarwatts,
	max(max_potentialsolarwatts) as max_potentialsolarwatts,
    avg(radiation) as radiation,
	max(max_radiation) as max_radiation
FROM weather_1h
GROUP BY bucket, stationname, stationtype
WITH NO DATA;`

const dropRainSinceMidnightViewSQL = `DROP MATERIALIZED VIEW IF EXISTS weather_rain_since_midnight CASCADE;`

const createRainSinceMidnightViewSQL = `CREATE MATERIALIZED VIEW weather_rain_since_midnight
WITH (timescaledb.continuous, timescaledb.materialized_only = false)
AS
SELECT
    time_bucket('5 minutes', time) as bucket,
	stationname,
	stationtype,
	MAX(dayrain) as rain_since_midnight
FROM weather
WHERE EXTRACT(hour FROM time AT TIME ZONE 'America/Los_Angeles') BETWEEN 0 AND 23
GROUP BY bucket, stationname, stationtype
WITH NO DATA;`

const addAggregationPolicy1mSQL = `SELECT add_continuous_aggregate_policy('weather_1m',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '1 minute',
    schedule_interval => INTERVAL '1 minute',
    if_not_exists => TRUE);`

const addAggregationPolicy5mSQL = `SELECT add_continuous_aggregate_policy('weather_5m',
    start_offset => INTERVAL '1 hour',
    end_offset => INTERVAL '5 minutes',
    schedule_interval => INTERVAL '5 minutes',
    if_not_exists => TRUE);`

const addAggregationPolicy1hSQL = `SELECT add_continuous_aggregate_policy('weather_1h',
    start_offset => INTERVAL '3 hours',
    end_offset => INTERVAL '1 hour',
    schedule_interval => INTERVAL '1 hour',
    if_not_exists => TRUE);`

const addAggregationPolicy1dSQL = `SELECT add_continuous_aggregate_policy('weather_1d',
    start_offset => INTERVAL '3 days',
    end_offset => INTERVAL '1 day',
    schedule_interval => INTERVAL '1 day',
    if_not_exists => TRUE);`
