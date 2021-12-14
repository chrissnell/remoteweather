package main

const createTableSQL = `
CREATE TABLE IF NOT EXISTS weather (
    time timestamp WITHOUT TIME ZONE NOT NULL,
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
	uv float4 NULL,
	radiation float4 NULL,
    stormrain float4 NULL,
    stormstart timestamp WITHOUT TIME ZONE NULL,
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
    forecasticon int NULL,
    forecastrule int NULL,
    sunrise TIMESTAMP WITHOUT TIME ZONE NULL,
    sunset TIMESTAMP WITHOUT TIME ZONE NULL
);`

const createExtensionSQL = `CREATE EXTENSION IF NOT EXISTS timescaledb;`

const createHypertableSQL = `SELECT create_hypertable('weather', 'time', if_not_exists => true);`

const create5mViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_5m
WITH (timescaledb.continuous)
AS
SELECT
    time_bucket('5 minutes', time) as bucket,
    stationname,
    avg(barometer) as barometer,
    avg(intemp) as intemp,
    avg(inhumidity) as inhumidity,
    avg(outtemp) as outtemp,
    avg(outhumidity) as outhumidity,
    avg(windspeed) as windspeed,
    max(windspeed) as maxwindspeed,
    avg(windchill) as windchill,
    avg(heatindex) as heatindex,
    avg(rainrate) as rainrate,
    max(rainrate) as maxrainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname
WITH NO DATA;`

const create1hViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1h
WITH (timescaledb.continuous)
AS
SELECT
    time_bucket('1 hour', time) as bucket,
    stationname,
    avg(barometer) as barometer,
    avg(intemp) as intemp,
    avg(inhumidity) as inhumidity,
    avg(outtemp) as outtemp,
    avg(outhumidity) as outhumidity,
    avg(windspeed) as windspeed,
    max(windspeed) as maxwindspeed,
    avg(windchill) as windchill,
    avg(heatindex) as heatindex,
    avg(rainrate) as rainrate,
    max(rainrate) as maxrainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname
WITH NO DATA;`

const create1dViewSQL = `CREATE MATERIALIZED VIEW IF NOT EXISTS weather_1d
WITH (timescaledb.continuous)
AS
SELECT
    time_bucket('1 day', time) as bucket,
    stationname,
    avg(barometer) as barometer,
    avg(intemp) as intemp,
    avg(inhumidity) as inhumidity,
    avg(outtemp) as outtemp,
    avg(outhumidity) as outhumidity,
    avg(windspeed) as windspeed,
    max(windspeed) as maxwindspeed,
    avg(windchill) as windchill,
    avg(heatindex) as heatindex,
    avg(rainrate) as rainrate,
    max(rainrate) as maxrainrate,
    max(dayrain) as dayrain,
    max(monthrain) as monthrain,
    max(yearrain) as yearrain,
    avg(consbatteryvoltage) as consbatteryvoltage
FROM
    weather
GROUP BY bucket, stationname
WITH NO DATA;`
