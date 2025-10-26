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
    extratext10 text NULL,
    temp1 float4 NULL,
    temp2 float4 NULL,
    temp3 float4 NULL,
    temp4 float4 NULL,
    temp5 float4 NULL,
    temp6 float4 NULL,
    temp7 float4 NULL,
    temp8 float4 NULL,
    temp9 float4 NULL,
    temp10 float4 NULL,
    soiltemp5 float4 NULL,
    soiltemp6 float4 NULL,
    soiltemp7 float4 NULL,
    soiltemp8 float4 NULL,
    soiltemp9 float4 NULL,
    soiltemp10 float4 NULL,
    humidity1 float4 NULL,
    humidity2 float4 NULL,
    humidity3 float4 NULL,
    humidity4 float4 NULL,
    humidity5 float4 NULL,
    humidity6 float4 NULL,
    humidity7 float4 NULL,
    humidity8 float4 NULL,
    humidity9 float4 NULL,
    humidity10 float4 NULL,
    soilhum1 float4 NULL,
    soilhum2 float4 NULL,
    soilhum3 float4 NULL,
    soilhum4 float4 NULL,
    soilhum5 float4 NULL,
    soilhum6 float4 NULL,
    soilhum7 float4 NULL,
    soilhum8 float4 NULL,
    soilhum9 float4 NULL,
    soilhum10 float4 NULL,
    leafwetness5 float4 NULL,
    leafwetness6 float4 NULL,
    leafwetness7 float4 NULL,
    leafwetness8 float4 NULL,
    soiltens1 float4 NULL,
    soiltens2 float4 NULL,
    soiltens3 float4 NULL,
    soiltens4 float4 NULL,
    gdd int NULL,
    etos float4 NULL,
    etrs float4 NULL,
    leak1 int NULL,
    leak2 int NULL,
    leak3 int NULL,
    leak4 int NULL,
    battout int NULL,
    battin int NULL,
    batt1 int NULL,
    batt2 int NULL,
    batt3 int NULL,
    batt4 int NULL,
    batt5 int NULL,
    batt6 int NULL,
    batt7 int NULL,
    batt8 int NULL,
    batt9 int NULL,
    batt10 int NULL,
    batt_25 int NULL,
    batt_lightning int NULL,
    batleak1 int NULL,
    batleak2 int NULL,
    batleak3 int NULL,
    batleak4 int NULL,
    battsm1 int NULL,
    battsm2 int NULL,
    battsm3 int NULL,
    battsm4 int NULL,
    batt_co2 int NULL,
    batt_cellgateway int NULL,
    baromrelin float4 NULL,
    baromabsin float4 NULL,
    relay1 int NULL,
    relay2 int NULL,
    relay3 int NULL,
    relay4 int NULL,
    relay5 int NULL,
    relay6 int NULL,
    relay7 int NULL,
    relay8 int NULL,
    relay9 int NULL,
    relay10 int NULL,
    pm25 float4 NULL,
    pm25_24h float4 NULL,
    pm25_in float4 NULL,
    pm25_in_24h float4 NULL,
    pm25_in_aqin float4 NULL,
    pm25_in_24h_aqin float4 NULL,
    pm10_in_aqin float4 NULL,
    pm10_in_24h_aqin float4 NULL,
    co2 float4 NULL,
    co2_in_aqin int NULL,
    co2_in_24h_aqin int NULL,
    pm_in_temp_aqin float4 NULL,
    pm_in_humidity_aqin int NULL,
    aqi_pm25_aqin int NULL,
    aqi_pm25_24h_aqin int NULL,
    aqi_pm10_aqin int NULL,
    aqi_pm10_24h_aqin int NULL,
    aqi_pm25_in int NULL,
    aqi_pm25_in_24h int NULL,
    lightning_day int NULL,
    lightning_hour int NULL,
    lightning_time timestamp WITH TIME ZONE NULL,
    lightning_distance float4 NULL,
    tz text NULL,
    dateutc bigint NULL
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
    min(extrafloat10) as min_extrafloat10,
    -- Temperature sensors
    avg(temp1) as temp1,
    max(temp1) as max_temp1,
    min(temp1) as min_temp1,
    avg(temp2) as temp2,
    max(temp2) as max_temp2,
    min(temp2) as min_temp2,
    avg(temp3) as temp3,
    max(temp3) as max_temp3,
    min(temp3) as min_temp3,
    avg(temp4) as temp4,
    max(temp4) as max_temp4,
    min(temp4) as min_temp4,
    avg(temp5) as temp5,
    max(temp5) as max_temp5,
    min(temp5) as min_temp5,
    avg(temp6) as temp6,
    max(temp6) as max_temp6,
    min(temp6) as min_temp6,
    avg(temp7) as temp7,
    max(temp7) as max_temp7,
    min(temp7) as min_temp7,
    avg(temp8) as temp8,
    max(temp8) as max_temp8,
    min(temp8) as min_temp8,
    avg(temp9) as temp9,
    max(temp9) as max_temp9,
    min(temp9) as min_temp9,
    avg(temp10) as temp10,
    max(temp10) as max_temp10,
    min(temp10) as min_temp10,
    -- Additional soil temperature sensors
    avg(soiltemp5) as soiltemp5,
    max(soiltemp5) as max_soiltemp5,
    min(soiltemp5) as min_soiltemp5,
    avg(soiltemp6) as soiltemp6,
    max(soiltemp6) as max_soiltemp6,
    min(soiltemp6) as min_soiltemp6,
    avg(soiltemp7) as soiltemp7,
    max(soiltemp7) as max_soiltemp7,
    min(soiltemp7) as min_soiltemp7,
    avg(soiltemp8) as soiltemp8,
    max(soiltemp8) as max_soiltemp8,
    min(soiltemp8) as min_soiltemp8,
    avg(soiltemp9) as soiltemp9,
    max(soiltemp9) as max_soiltemp9,
    min(soiltemp9) as min_soiltemp9,
    avg(soiltemp10) as soiltemp10,
    max(soiltemp10) as max_soiltemp10,
    min(soiltemp10) as min_soiltemp10,
    -- Humidity sensors
    avg(humidity1) as humidity1,
    max(humidity1) as max_humidity1,
    min(humidity1) as min_humidity1,
    avg(humidity2) as humidity2,
    max(humidity2) as max_humidity2,
    min(humidity2) as min_humidity2,
    avg(humidity3) as humidity3,
    max(humidity3) as max_humidity3,
    min(humidity3) as min_humidity3,
    avg(humidity4) as humidity4,
    max(humidity4) as max_humidity4,
    min(humidity4) as min_humidity4,
    avg(humidity5) as humidity5,
    max(humidity5) as max_humidity5,
    min(humidity5) as min_humidity5,
    avg(humidity6) as humidity6,
    max(humidity6) as max_humidity6,
    min(humidity6) as min_humidity6,
    avg(humidity7) as humidity7,
    max(humidity7) as max_humidity7,
    min(humidity7) as min_humidity7,
    avg(humidity8) as humidity8,
    max(humidity8) as max_humidity8,
    min(humidity8) as min_humidity8,
    avg(humidity9) as humidity9,
    max(humidity9) as max_humidity9,
    min(humidity9) as min_humidity9,
    avg(humidity10) as humidity10,
    max(humidity10) as max_humidity10,
    min(humidity10) as min_humidity10,
    -- Soil humidity sensors
    avg(soilhum1) as soilhum1,
    max(soilhum1) as max_soilhum1,
    min(soilhum1) as min_soilhum1,
    avg(soilhum2) as soilhum2,
    max(soilhum2) as max_soilhum2,
    min(soilhum2) as min_soilhum2,
    avg(soilhum3) as soilhum3,
    max(soilhum3) as max_soilhum3,
    min(soilhum3) as min_soilhum3,
    avg(soilhum4) as soilhum4,
    max(soilhum4) as max_soilhum4,
    min(soilhum4) as min_soilhum4,
    avg(soilhum5) as soilhum5,
    max(soilhum5) as max_soilhum5,
    min(soilhum5) as min_soilhum5,
    avg(soilhum6) as soilhum6,
    max(soilhum6) as max_soilhum6,
    min(soilhum6) as min_soilhum6,
    avg(soilhum7) as soilhum7,
    max(soilhum7) as max_soilhum7,
    min(soilhum7) as min_soilhum7,
    avg(soilhum8) as soilhum8,
    max(soilhum8) as max_soilhum8,
    min(soilhum8) as min_soilhum8,
    avg(soilhum9) as soilhum9,
    max(soilhum9) as max_soilhum9,
    min(soilhum9) as min_soilhum9,
    avg(soilhum10) as soilhum10,
    max(soilhum10) as max_soilhum10,
    min(soilhum10) as min_soilhum10,
    -- Additional leaf wetness sensors
    avg(leafwetness5) as leafwetness5,
    max(leafwetness5) as max_leafwetness5,
    min(leafwetness5) as min_leafwetness5,
    avg(leafwetness6) as leafwetness6,
    max(leafwetness6) as max_leafwetness6,
    min(leafwetness6) as min_leafwetness6,
    avg(leafwetness7) as leafwetness7,
    max(leafwetness7) as max_leafwetness7,
    min(leafwetness7) as min_leafwetness7,
    avg(leafwetness8) as leafwetness8,
    max(leafwetness8) as max_leafwetness8,
    min(leafwetness8) as min_leafwetness8,
    -- Soil tension sensors
    avg(soiltens1) as soiltens1,
    max(soiltens1) as max_soiltens1,
    min(soiltens1) as min_soiltens1,
    avg(soiltens2) as soiltens2,
    max(soiltens2) as max_soiltens2,
    min(soiltens2) as min_soiltens2,
    avg(soiltens3) as soiltens3,
    max(soiltens3) as max_soiltens3,
    min(soiltens3) as min_soiltens3,
    avg(soiltens4) as soiltens4,
    max(soiltens4) as max_soiltens4,
    min(soiltens4) as min_soiltens4,
    -- Agricultural measurements
    avg(gdd)::int as gdd,
    max(gdd) as max_gdd,
    min(gdd) as min_gdd,
    avg(etos) as etos,
    max(etos) as max_etos,
    min(etos) as min_etos,
    avg(etrs) as etrs,
    max(etrs) as max_etrs,
    min(etrs) as min_etrs,
    -- Leak detection sensors
    avg(leak1)::int as leak1,
    max(leak1) as max_leak1,
    min(leak1) as min_leak1,
    avg(leak2)::int as leak2,
    max(leak2) as max_leak2,
    min(leak2) as min_leak2,
    avg(leak3)::int as leak3,
    max(leak3) as max_leak3,
    min(leak3) as min_leak3,
    avg(leak4)::int as leak4,
    max(leak4) as max_leak4,
    min(leak4) as min_leak4,
    -- Battery status
    avg(battout)::int as battout,
    max(battout) as max_battout,
    min(battout) as min_battout,
    avg(battin)::int as battin,
    max(battin) as max_battin,
    min(battin) as min_battin,
    avg(batt1)::int as batt1,
    max(batt1) as max_batt1,
    min(batt1) as min_batt1,
    avg(batt2)::int as batt2,
    max(batt2) as max_batt2,
    min(batt2) as min_batt2,
    avg(batt3)::int as batt3,
    max(batt3) as max_batt3,
    min(batt3) as min_batt3,
    avg(batt4)::int as batt4,
    max(batt4) as max_batt4,
    min(batt4) as min_batt4,
    avg(batt5)::int as batt5,
    max(batt5) as max_batt5,
    min(batt5) as min_batt5,
    avg(batt6)::int as batt6,
    max(batt6) as max_batt6,
    min(batt6) as min_batt6,
    avg(batt7)::int as batt7,
    max(batt7) as max_batt7,
    min(batt7) as min_batt7,
    avg(batt8)::int as batt8,
    max(batt8) as max_batt8,
    min(batt8) as min_batt8,
    avg(batt9)::int as batt9,
    max(batt9) as max_batt9,
    min(batt9) as min_batt9,
    avg(batt10)::int as batt10,
    max(batt10) as max_batt10,
    min(batt10) as min_batt10,
    avg(batt_25)::int as batt_25,
    max(batt_25) as max_batt_25,
    min(batt_25) as min_batt_25,
    avg(batt_lightning)::int as batt_lightning,
    max(batt_lightning) as max_batt_lightning,
    min(batt_lightning) as min_batt_lightning,
    avg(batleak1)::int as batleak1,
    max(batleak1) as max_batleak1,
    min(batleak1) as min_batleak1,
    avg(batleak2)::int as batleak2,
    max(batleak2) as max_batleak2,
    min(batleak2) as min_batleak2,
    avg(batleak3)::int as batleak3,
    max(batleak3) as max_batleak3,
    min(batleak3) as min_batleak3,
    avg(batleak4)::int as batleak4,
    max(batleak4) as max_batleak4,
    min(batleak4) as min_batleak4,
    avg(battsm1)::int as battsm1,
    max(battsm1) as max_battsm1,
    min(battsm1) as min_battsm1,
    avg(battsm2)::int as battsm2,
    max(battsm2) as max_battsm2,
    min(battsm2) as min_battsm2,
    avg(battsm3)::int as battsm3,
    max(battsm3) as max_battsm3,
    min(battsm3) as min_battsm3,
    avg(battsm4)::int as battsm4,
    max(battsm4) as max_battsm4,
    min(battsm4) as min_battsm4,
    avg(batt_co2)::int as batt_co2,
    max(batt_co2) as max_batt_co2,
    min(batt_co2) as min_batt_co2,
    avg(batt_cellgateway)::int as batt_cellgateway,
    max(batt_cellgateway) as max_batt_cellgateway,
    min(batt_cellgateway) as min_batt_cellgateway,
    -- Pressure measurements
    avg(baromrelin) as baromrelin,
    max(baromrelin) as max_baromrelin,
    min(baromrelin) as min_baromrelin,
    avg(baromabsin) as baromabsin,
    max(baromabsin) as max_baromabsin,
    min(baromabsin) as min_baromabsin,
    -- Relay states
    avg(relay1)::int as relay1,
    max(relay1) as max_relay1,
    min(relay1) as min_relay1,
    avg(relay2)::int as relay2,
    max(relay2) as max_relay2,
    min(relay2) as min_relay2,
    avg(relay3)::int as relay3,
    max(relay3) as max_relay3,
    min(relay3) as min_relay3,
    avg(relay4)::int as relay4,
    max(relay4) as max_relay4,
    min(relay4) as min_relay4,
    avg(relay5)::int as relay5,
    max(relay5) as max_relay5,
    min(relay5) as min_relay5,
    avg(relay6)::int as relay6,
    max(relay6) as max_relay6,
    min(relay6) as min_relay6,
    avg(relay7)::int as relay7,
    max(relay7) as max_relay7,
    min(relay7) as min_relay7,
    avg(relay8)::int as relay8,
    max(relay8) as max_relay8,
    min(relay8) as min_relay8,
    avg(relay9)::int as relay9,
    max(relay9) as max_relay9,
    min(relay9) as min_relay9,
    avg(relay10)::int as relay10,
    max(relay10) as max_relay10,
    min(relay10) as min_relay10,
    -- Air quality measurements
    avg(pm25) as pm25,
    max(pm25) as max_pm25,
    min(pm25) as min_pm25,
    avg(pm25_24h) as pm25_24h,
    max(pm25_24h) as max_pm25_24h,
    min(pm25_24h) as min_pm25_24h,
    avg(pm25_in) as pm25_in,
    max(pm25_in) as max_pm25_in,
    min(pm25_in) as min_pm25_in,
    avg(pm25_in_24h) as pm25_in_24h,
    max(pm25_in_24h) as max_pm25_in_24h,
    min(pm25_in_24h) as min_pm25_in_24h,
    avg(pm25_in_aqin) as pm25_in_aqin,
    max(pm25_in_aqin) as max_pm25_in_aqin,
    min(pm25_in_aqin) as min_pm25_in_aqin,
    avg(pm25_in_24h_aqin) as pm25_in_24h_aqin,
    max(pm25_in_24h_aqin) as max_pm25_in_24h_aqin,
    min(pm25_in_24h_aqin) as min_pm25_in_24h_aqin,
    avg(pm10_in_aqin) as pm10_in_aqin,
    max(pm10_in_aqin) as max_pm10_in_aqin,
    min(pm10_in_aqin) as min_pm10_in_aqin,
    avg(pm10_in_24h_aqin) as pm10_in_24h_aqin,
    max(pm10_in_24h_aqin) as max_pm10_in_24h_aqin,
    min(pm10_in_24h_aqin) as min_pm10_in_24h_aqin,
    avg(co2) as co2,
    max(co2) as max_co2,
    min(co2) as min_co2,
    avg(co2_in_aqin)::int as co2_in_aqin,
    max(co2_in_aqin) as max_co2_in_aqin,
    min(co2_in_aqin) as min_co2_in_aqin,
    avg(co2_in_24h_aqin)::int as co2_in_24h_aqin,
    max(co2_in_24h_aqin) as max_co2_in_24h_aqin,
    min(co2_in_24h_aqin) as min_co2_in_24h_aqin,
    avg(pm_in_temp_aqin) as pm_in_temp_aqin,
    max(pm_in_temp_aqin) as max_pm_in_temp_aqin,
    min(pm_in_temp_aqin) as min_pm_in_temp_aqin,
    avg(pm_in_humidity_aqin)::int as pm_in_humidity_aqin,
    max(pm_in_humidity_aqin) as max_pm_in_humidity_aqin,
    min(pm_in_humidity_aqin) as min_pm_in_humidity_aqin,
    avg(aqi_pm25_aqin)::int as aqi_pm25_aqin,
    max(aqi_pm25_aqin) as max_aqi_pm25_aqin,
    min(aqi_pm25_aqin) as min_aqi_pm25_aqin,
    avg(aqi_pm25_24h_aqin)::int as aqi_pm25_24h_aqin,
    max(aqi_pm25_24h_aqin) as max_aqi_pm25_24h_aqin,
    min(aqi_pm25_24h_aqin) as min_aqi_pm25_24h_aqin,
    avg(aqi_pm10_aqin)::int as aqi_pm10_aqin,
    max(aqi_pm10_aqin) as max_aqi_pm10_aqin,
    min(aqi_pm10_aqin) as min_aqi_pm10_aqin,
    avg(aqi_pm10_24h_aqin)::int as aqi_pm10_24h_aqin,
    max(aqi_pm10_24h_aqin) as max_aqi_pm10_24h_aqin,
    min(aqi_pm10_24h_aqin) as min_aqi_pm10_24h_aqin,
    avg(aqi_pm25_in)::int as aqi_pm25_in,
    max(aqi_pm25_in) as max_aqi_pm25_in,
    min(aqi_pm25_in) as min_aqi_pm25_in,
    avg(aqi_pm25_in_24h)::int as aqi_pm25_in_24h,
    max(aqi_pm25_in_24h) as max_aqi_pm25_in_24h,
    min(aqi_pm25_in_24h) as min_aqi_pm25_in_24h,
    -- Lightning data
    sum(lightning_day) as lightning_day,
    sum(lightning_hour) as lightning_hour,
    max(lightning_time) as lightning_time,
    min(lightning_distance) as lightning_distance,
    -- Other fields
    avg(radiation) as radiation,
    max(radiation) as max_radiation,
    min(radiation) as min_radiation,
    avg(uv) as uv,
    max(uv) as max_uv,
    min(uv) as min_uv
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
    min(extrafloat10) as min_extrafloat10,
    -- Temperature sensors
    avg(temp1) as temp1,
    max(temp1) as max_temp1,
    min(temp1) as min_temp1,
    avg(temp2) as temp2,
    max(temp2) as max_temp2,
    min(temp2) as min_temp2,
    avg(temp3) as temp3,
    max(temp3) as max_temp3,
    min(temp3) as min_temp3,
    avg(temp4) as temp4,
    max(temp4) as max_temp4,
    min(temp4) as min_temp4,
    avg(temp5) as temp5,
    max(temp5) as max_temp5,
    min(temp5) as min_temp5,
    avg(temp6) as temp6,
    max(temp6) as max_temp6,
    min(temp6) as min_temp6,
    avg(temp7) as temp7,
    max(temp7) as max_temp7,
    min(temp7) as min_temp7,
    avg(temp8) as temp8,
    max(temp8) as max_temp8,
    min(temp8) as min_temp8,
    avg(temp9) as temp9,
    max(temp9) as max_temp9,
    min(temp9) as min_temp9,
    avg(temp10) as temp10,
    max(temp10) as max_temp10,
    min(temp10) as min_temp10,
    -- Additional soil temperature sensors
    avg(soiltemp5) as soiltemp5,
    max(soiltemp5) as max_soiltemp5,
    min(soiltemp5) as min_soiltemp5,
    avg(soiltemp6) as soiltemp6,
    max(soiltemp6) as max_soiltemp6,
    min(soiltemp6) as min_soiltemp6,
    avg(soiltemp7) as soiltemp7,
    max(soiltemp7) as max_soiltemp7,
    min(soiltemp7) as min_soiltemp7,
    avg(soiltemp8) as soiltemp8,
    max(soiltemp8) as max_soiltemp8,
    min(soiltemp8) as min_soiltemp8,
    avg(soiltemp9) as soiltemp9,
    max(soiltemp9) as max_soiltemp9,
    min(soiltemp9) as min_soiltemp9,
    avg(soiltemp10) as soiltemp10,
    max(soiltemp10) as max_soiltemp10,
    min(soiltemp10) as min_soiltemp10,
    -- Humidity sensors
    avg(humidity1) as humidity1,
    max(humidity1) as max_humidity1,
    min(humidity1) as min_humidity1,
    avg(humidity2) as humidity2,
    max(humidity2) as max_humidity2,
    min(humidity2) as min_humidity2,
    avg(humidity3) as humidity3,
    max(humidity3) as max_humidity3,
    min(humidity3) as min_humidity3,
    avg(humidity4) as humidity4,
    max(humidity4) as max_humidity4,
    min(humidity4) as min_humidity4,
    avg(humidity5) as humidity5,
    max(humidity5) as max_humidity5,
    min(humidity5) as min_humidity5,
    avg(humidity6) as humidity6,
    max(humidity6) as max_humidity6,
    min(humidity6) as min_humidity6,
    avg(humidity7) as humidity7,
    max(humidity7) as max_humidity7,
    min(humidity7) as min_humidity7,
    avg(humidity8) as humidity8,
    max(humidity8) as max_humidity8,
    min(humidity8) as min_humidity8,
    avg(humidity9) as humidity9,
    max(humidity9) as max_humidity9,
    min(humidity9) as min_humidity9,
    avg(humidity10) as humidity10,
    max(humidity10) as max_humidity10,
    min(humidity10) as min_humidity10,
    -- Soil humidity sensors
    avg(soilhum1) as soilhum1,
    max(soilhum1) as max_soilhum1,
    min(soilhum1) as min_soilhum1,
    avg(soilhum2) as soilhum2,
    max(soilhum2) as max_soilhum2,
    min(soilhum2) as min_soilhum2,
    avg(soilhum3) as soilhum3,
    max(soilhum3) as max_soilhum3,
    min(soilhum3) as min_soilhum3,
    avg(soilhum4) as soilhum4,
    max(soilhum4) as max_soilhum4,
    min(soilhum4) as min_soilhum4,
    avg(soilhum5) as soilhum5,
    max(soilhum5) as max_soilhum5,
    min(soilhum5) as min_soilhum5,
    avg(soilhum6) as soilhum6,
    max(soilhum6) as max_soilhum6,
    min(soilhum6) as min_soilhum6,
    avg(soilhum7) as soilhum7,
    max(soilhum7) as max_soilhum7,
    min(soilhum7) as min_soilhum7,
    avg(soilhum8) as soilhum8,
    max(soilhum8) as max_soilhum8,
    min(soilhum8) as min_soilhum8,
    avg(soilhum9) as soilhum9,
    max(soilhum9) as max_soilhum9,
    min(soilhum9) as min_soilhum9,
    avg(soilhum10) as soilhum10,
    max(soilhum10) as max_soilhum10,
    min(soilhum10) as min_soilhum10,
    -- Additional leaf wetness sensors
    avg(leafwetness5) as leafwetness5,
    max(leafwetness5) as max_leafwetness5,
    min(leafwetness5) as min_leafwetness5,
    avg(leafwetness6) as leafwetness6,
    max(leafwetness6) as max_leafwetness6,
    min(leafwetness6) as min_leafwetness6,
    avg(leafwetness7) as leafwetness7,
    max(leafwetness7) as max_leafwetness7,
    min(leafwetness7) as min_leafwetness7,
    avg(leafwetness8) as leafwetness8,
    max(leafwetness8) as max_leafwetness8,
    min(leafwetness8) as min_leafwetness8,
    -- Soil tension sensors
    avg(soiltens1) as soiltens1,
    max(soiltens1) as max_soiltens1,
    min(soiltens1) as min_soiltens1,
    avg(soiltens2) as soiltens2,
    max(soiltens2) as max_soiltens2,
    min(soiltens2) as min_soiltens2,
    avg(soiltens3) as soiltens3,
    max(soiltens3) as max_soiltens3,
    min(soiltens3) as min_soiltens3,
    avg(soiltens4) as soiltens4,
    max(soiltens4) as max_soiltens4,
    min(soiltens4) as min_soiltens4,
    -- Agricultural measurements
    avg(gdd)::int as gdd,
    max(gdd) as max_gdd,
    min(gdd) as min_gdd,
    avg(etos) as etos,
    max(etos) as max_etos,
    min(etos) as min_etos,
    avg(etrs) as etrs,
    max(etrs) as max_etrs,
    min(etrs) as min_etrs,
    -- Leak detection sensors
    avg(leak1)::int as leak1,
    max(leak1) as max_leak1,
    min(leak1) as min_leak1,
    avg(leak2)::int as leak2,
    max(leak2) as max_leak2,
    min(leak2) as min_leak2,
    avg(leak3)::int as leak3,
    max(leak3) as max_leak3,
    min(leak3) as min_leak3,
    avg(leak4)::int as leak4,
    max(leak4) as max_leak4,
    min(leak4) as min_leak4,
    -- Battery status
    avg(battout)::int as battout,
    max(battout) as max_battout,
    min(battout) as min_battout,
    avg(battin)::int as battin,
    max(battin) as max_battin,
    min(battin) as min_battin,
    avg(batt1)::int as batt1,
    max(batt1) as max_batt1,
    min(batt1) as min_batt1,
    avg(batt2)::int as batt2,
    max(batt2) as max_batt2,
    min(batt2) as min_batt2,
    avg(batt3)::int as batt3,
    max(batt3) as max_batt3,
    min(batt3) as min_batt3,
    avg(batt4)::int as batt4,
    max(batt4) as max_batt4,
    min(batt4) as min_batt4,
    avg(batt5)::int as batt5,
    max(batt5) as max_batt5,
    min(batt5) as min_batt5,
    avg(batt6)::int as batt6,
    max(batt6) as max_batt6,
    min(batt6) as min_batt6,
    avg(batt7)::int as batt7,
    max(batt7) as max_batt7,
    min(batt7) as min_batt7,
    avg(batt8)::int as batt8,
    max(batt8) as max_batt8,
    min(batt8) as min_batt8,
    avg(batt9)::int as batt9,
    max(batt9) as max_batt9,
    min(batt9) as min_batt9,
    avg(batt10)::int as batt10,
    max(batt10) as max_batt10,
    min(batt10) as min_batt10,
    avg(batt_25)::int as batt_25,
    max(batt_25) as max_batt_25,
    min(batt_25) as min_batt_25,
    avg(batt_lightning)::int as batt_lightning,
    max(batt_lightning) as max_batt_lightning,
    min(batt_lightning) as min_batt_lightning,
    avg(batleak1)::int as batleak1,
    max(batleak1) as max_batleak1,
    min(batleak1) as min_batleak1,
    avg(batleak2)::int as batleak2,
    max(batleak2) as max_batleak2,
    min(batleak2) as min_batleak2,
    avg(batleak3)::int as batleak3,
    max(batleak3) as max_batleak3,
    min(batleak3) as min_batleak3,
    avg(batleak4)::int as batleak4,
    max(batleak4) as max_batleak4,
    min(batleak4) as min_batleak4,
    avg(battsm1)::int as battsm1,
    max(battsm1) as max_battsm1,
    min(battsm1) as min_battsm1,
    avg(battsm2)::int as battsm2,
    max(battsm2) as max_battsm2,
    min(battsm2) as min_battsm2,
    avg(battsm3)::int as battsm3,
    max(battsm3) as max_battsm3,
    min(battsm3) as min_battsm3,
    avg(battsm4)::int as battsm4,
    max(battsm4) as max_battsm4,
    min(battsm4) as min_battsm4,
    avg(batt_co2)::int as batt_co2,
    max(batt_co2) as max_batt_co2,
    min(batt_co2) as min_batt_co2,
    avg(batt_cellgateway)::int as batt_cellgateway,
    max(batt_cellgateway) as max_batt_cellgateway,
    min(batt_cellgateway) as min_batt_cellgateway,
    -- Pressure measurements
    avg(baromrelin) as baromrelin,
    max(baromrelin) as max_baromrelin,
    min(baromrelin) as min_baromrelin,
    avg(baromabsin) as baromabsin,
    max(baromabsin) as max_baromabsin,
    min(baromabsin) as min_baromabsin,
    -- Relay states
    avg(relay1)::int as relay1,
    max(relay1) as max_relay1,
    min(relay1) as min_relay1,
    avg(relay2)::int as relay2,
    max(relay2) as max_relay2,
    min(relay2) as min_relay2,
    avg(relay3)::int as relay3,
    max(relay3) as max_relay3,
    min(relay3) as min_relay3,
    avg(relay4)::int as relay4,
    max(relay4) as max_relay4,
    min(relay4) as min_relay4,
    avg(relay5)::int as relay5,
    max(relay5) as max_relay5,
    min(relay5) as min_relay5,
    avg(relay6)::int as relay6,
    max(relay6) as max_relay6,
    min(relay6) as min_relay6,
    avg(relay7)::int as relay7,
    max(relay7) as max_relay7,
    min(relay7) as min_relay7,
    avg(relay8)::int as relay8,
    max(relay8) as max_relay8,
    min(relay8) as min_relay8,
    avg(relay9)::int as relay9,
    max(relay9) as max_relay9,
    min(relay9) as min_relay9,
    avg(relay10)::int as relay10,
    max(relay10) as max_relay10,
    min(relay10) as min_relay10,
    -- Air quality measurements
    avg(pm25) as pm25,
    max(pm25) as max_pm25,
    min(pm25) as min_pm25,
    avg(pm25_24h) as pm25_24h,
    max(pm25_24h) as max_pm25_24h,
    min(pm25_24h) as min_pm25_24h,
    avg(pm25_in) as pm25_in,
    max(pm25_in) as max_pm25_in,
    min(pm25_in) as min_pm25_in,
    avg(pm25_in_24h) as pm25_in_24h,
    max(pm25_in_24h) as max_pm25_in_24h,
    min(pm25_in_24h) as min_pm25_in_24h,
    avg(pm25_in_aqin) as pm25_in_aqin,
    max(pm25_in_aqin) as max_pm25_in_aqin,
    min(pm25_in_aqin) as min_pm25_in_aqin,
    avg(pm25_in_24h_aqin) as pm25_in_24h_aqin,
    max(pm25_in_24h_aqin) as max_pm25_in_24h_aqin,
    min(pm25_in_24h_aqin) as min_pm25_in_24h_aqin,
    avg(pm10_in_aqin) as pm10_in_aqin,
    max(pm10_in_aqin) as max_pm10_in_aqin,
    min(pm10_in_aqin) as min_pm10_in_aqin,
    avg(pm10_in_24h_aqin) as pm10_in_24h_aqin,
    max(pm10_in_24h_aqin) as max_pm10_in_24h_aqin,
    min(pm10_in_24h_aqin) as min_pm10_in_24h_aqin,
    avg(co2) as co2,
    max(co2) as max_co2,
    min(co2) as min_co2,
    avg(co2_in_aqin)::int as co2_in_aqin,
    max(co2_in_aqin) as max_co2_in_aqin,
    min(co2_in_aqin) as min_co2_in_aqin,
    avg(co2_in_24h_aqin)::int as co2_in_24h_aqin,
    max(co2_in_24h_aqin) as max_co2_in_24h_aqin,
    min(co2_in_24h_aqin) as min_co2_in_24h_aqin,
    avg(pm_in_temp_aqin) as pm_in_temp_aqin,
    max(pm_in_temp_aqin) as max_pm_in_temp_aqin,
    min(pm_in_temp_aqin) as min_pm_in_temp_aqin,
    avg(pm_in_humidity_aqin)::int as pm_in_humidity_aqin,
    max(pm_in_humidity_aqin) as max_pm_in_humidity_aqin,
    min(pm_in_humidity_aqin) as min_pm_in_humidity_aqin,
    avg(aqi_pm25_aqin)::int as aqi_pm25_aqin,
    max(aqi_pm25_aqin) as max_aqi_pm25_aqin,
    min(aqi_pm25_aqin) as min_aqi_pm25_aqin,
    avg(aqi_pm25_24h_aqin)::int as aqi_pm25_24h_aqin,
    max(aqi_pm25_24h_aqin) as max_aqi_pm25_24h_aqin,
    min(aqi_pm25_24h_aqin) as min_aqi_pm25_24h_aqin,
    avg(aqi_pm10_aqin)::int as aqi_pm10_aqin,
    max(aqi_pm10_aqin) as max_aqi_pm10_aqin,
    min(aqi_pm10_aqin) as min_aqi_pm10_aqin,
    avg(aqi_pm10_24h_aqin)::int as aqi_pm10_24h_aqin,
    max(aqi_pm10_24h_aqin) as max_aqi_pm10_24h_aqin,
    min(aqi_pm10_24h_aqin) as min_aqi_pm10_24h_aqin,
    avg(aqi_pm25_in)::int as aqi_pm25_in,
    max(aqi_pm25_in) as max_aqi_pm25_in,
    min(aqi_pm25_in) as min_aqi_pm25_in,
    avg(aqi_pm25_in_24h)::int as aqi_pm25_in_24h,
    max(aqi_pm25_in_24h) as max_aqi_pm25_in_24h,
    min(aqi_pm25_in_24h) as min_aqi_pm25_in_24h,
    -- Lightning data
    sum(lightning_day) as lightning_day,
    sum(lightning_hour) as lightning_hour,
    max(lightning_time) as lightning_time,
    min(lightning_distance) as lightning_distance,
    -- Other fields
    avg(radiation) as radiation,
    max(radiation) as max_radiation,
    min(radiation) as min_radiation,
    avg(uv) as uv,
    max(uv) as max_uv,
    min(uv) as min_uv
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
    min(extrafloat10) as min_extrafloat10,
    -- Temperature sensors
    avg(temp1) as temp1,
    max(temp1) as max_temp1,
    min(temp1) as min_temp1,
    avg(temp2) as temp2,
    max(temp2) as max_temp2,
    min(temp2) as min_temp2,
    avg(temp3) as temp3,
    max(temp3) as max_temp3,
    min(temp3) as min_temp3,
    avg(temp4) as temp4,
    max(temp4) as max_temp4,
    min(temp4) as min_temp4,
    avg(temp5) as temp5,
    max(temp5) as max_temp5,
    min(temp5) as min_temp5,
    avg(temp6) as temp6,
    max(temp6) as max_temp6,
    min(temp6) as min_temp6,
    avg(temp7) as temp7,
    max(temp7) as max_temp7,
    min(temp7) as min_temp7,
    avg(temp8) as temp8,
    max(temp8) as max_temp8,
    min(temp8) as min_temp8,
    avg(temp9) as temp9,
    max(temp9) as max_temp9,
    min(temp9) as min_temp9,
    avg(temp10) as temp10,
    max(temp10) as max_temp10,
    min(temp10) as min_temp10,
    -- Additional soil temperature sensors
    avg(soiltemp5) as soiltemp5,
    max(soiltemp5) as max_soiltemp5,
    min(soiltemp5) as min_soiltemp5,
    avg(soiltemp6) as soiltemp6,
    max(soiltemp6) as max_soiltemp6,
    min(soiltemp6) as min_soiltemp6,
    avg(soiltemp7) as soiltemp7,
    max(soiltemp7) as max_soiltemp7,
    min(soiltemp7) as min_soiltemp7,
    avg(soiltemp8) as soiltemp8,
    max(soiltemp8) as max_soiltemp8,
    min(soiltemp8) as min_soiltemp8,
    avg(soiltemp9) as soiltemp9,
    max(soiltemp9) as max_soiltemp9,
    min(soiltemp9) as min_soiltemp9,
    avg(soiltemp10) as soiltemp10,
    max(soiltemp10) as max_soiltemp10,
    min(soiltemp10) as min_soiltemp10,
    -- Humidity sensors
    avg(humidity1) as humidity1,
    max(humidity1) as max_humidity1,
    min(humidity1) as min_humidity1,
    avg(humidity2) as humidity2,
    max(humidity2) as max_humidity2,
    min(humidity2) as min_humidity2,
    avg(humidity3) as humidity3,
    max(humidity3) as max_humidity3,
    min(humidity3) as min_humidity3,
    avg(humidity4) as humidity4,
    max(humidity4) as max_humidity4,
    min(humidity4) as min_humidity4,
    avg(humidity5) as humidity5,
    max(humidity5) as max_humidity5,
    min(humidity5) as min_humidity5,
    avg(humidity6) as humidity6,
    max(humidity6) as max_humidity6,
    min(humidity6) as min_humidity6,
    avg(humidity7) as humidity7,
    max(humidity7) as max_humidity7,
    min(humidity7) as min_humidity7,
    avg(humidity8) as humidity8,
    max(humidity8) as max_humidity8,
    min(humidity8) as min_humidity8,
    avg(humidity9) as humidity9,
    max(humidity9) as max_humidity9,
    min(humidity9) as min_humidity9,
    avg(humidity10) as humidity10,
    max(humidity10) as max_humidity10,
    min(humidity10) as min_humidity10,
    -- Soil humidity sensors
    avg(soilhum1) as soilhum1,
    max(soilhum1) as max_soilhum1,
    min(soilhum1) as min_soilhum1,
    avg(soilhum2) as soilhum2,
    max(soilhum2) as max_soilhum2,
    min(soilhum2) as min_soilhum2,
    avg(soilhum3) as soilhum3,
    max(soilhum3) as max_soilhum3,
    min(soilhum3) as min_soilhum3,
    avg(soilhum4) as soilhum4,
    max(soilhum4) as max_soilhum4,
    min(soilhum4) as min_soilhum4,
    avg(soilhum5) as soilhum5,
    max(soilhum5) as max_soilhum5,
    min(soilhum5) as min_soilhum5,
    avg(soilhum6) as soilhum6,
    max(soilhum6) as max_soilhum6,
    min(soilhum6) as min_soilhum6,
    avg(soilhum7) as soilhum7,
    max(soilhum7) as max_soilhum7,
    min(soilhum7) as min_soilhum7,
    avg(soilhum8) as soilhum8,
    max(soilhum8) as max_soilhum8,
    min(soilhum8) as min_soilhum8,
    avg(soilhum9) as soilhum9,
    max(soilhum9) as max_soilhum9,
    min(soilhum9) as min_soilhum9,
    avg(soilhum10) as soilhum10,
    max(soilhum10) as max_soilhum10,
    min(soilhum10) as min_soilhum10,
    -- Additional leaf wetness sensors
    avg(leafwetness5) as leafwetness5,
    max(leafwetness5) as max_leafwetness5,
    min(leafwetness5) as min_leafwetness5,
    avg(leafwetness6) as leafwetness6,
    max(leafwetness6) as max_leafwetness6,
    min(leafwetness6) as min_leafwetness6,
    avg(leafwetness7) as leafwetness7,
    max(leafwetness7) as max_leafwetness7,
    min(leafwetness7) as min_leafwetness7,
    avg(leafwetness8) as leafwetness8,
    max(leafwetness8) as max_leafwetness8,
    min(leafwetness8) as min_leafwetness8,
    -- Soil tension sensors
    avg(soiltens1) as soiltens1,
    max(soiltens1) as max_soiltens1,
    min(soiltens1) as min_soiltens1,
    avg(soiltens2) as soiltens2,
    max(soiltens2) as max_soiltens2,
    min(soiltens2) as min_soiltens2,
    avg(soiltens3) as soiltens3,
    max(soiltens3) as max_soiltens3,
    min(soiltens3) as min_soiltens3,
    avg(soiltens4) as soiltens4,
    max(soiltens4) as max_soiltens4,
    min(soiltens4) as min_soiltens4,
    -- Agricultural measurements
    avg(gdd)::int as gdd,
    max(gdd) as max_gdd,
    min(gdd) as min_gdd,
    avg(etos) as etos,
    max(etos) as max_etos,
    min(etos) as min_etos,
    avg(etrs) as etrs,
    max(etrs) as max_etrs,
    min(etrs) as min_etrs,
    -- Leak detection sensors
    avg(leak1)::int as leak1,
    max(leak1) as max_leak1,
    min(leak1) as min_leak1,
    avg(leak2)::int as leak2,
    max(leak2) as max_leak2,
    min(leak2) as min_leak2,
    avg(leak3)::int as leak3,
    max(leak3) as max_leak3,
    min(leak3) as min_leak3,
    avg(leak4)::int as leak4,
    max(leak4) as max_leak4,
    min(leak4) as min_leak4,
    -- Battery status
    avg(battout)::int as battout,
    max(battout) as max_battout,
    min(battout) as min_battout,
    avg(battin)::int as battin,
    max(battin) as max_battin,
    min(battin) as min_battin,
    avg(batt1)::int as batt1,
    max(batt1) as max_batt1,
    min(batt1) as min_batt1,
    avg(batt2)::int as batt2,
    max(batt2) as max_batt2,
    min(batt2) as min_batt2,
    avg(batt3)::int as batt3,
    max(batt3) as max_batt3,
    min(batt3) as min_batt3,
    avg(batt4)::int as batt4,
    max(batt4) as max_batt4,
    min(batt4) as min_batt4,
    avg(batt5)::int as batt5,
    max(batt5) as max_batt5,
    min(batt5) as min_batt5,
    avg(batt6)::int as batt6,
    max(batt6) as max_batt6,
    min(batt6) as min_batt6,
    avg(batt7)::int as batt7,
    max(batt7) as max_batt7,
    min(batt7) as min_batt7,
    avg(batt8)::int as batt8,
    max(batt8) as max_batt8,
    min(batt8) as min_batt8,
    avg(batt9)::int as batt9,
    max(batt9) as max_batt9,
    min(batt9) as min_batt9,
    avg(batt10)::int as batt10,
    max(batt10) as max_batt10,
    min(batt10) as min_batt10,
    avg(batt_25)::int as batt_25,
    max(batt_25) as max_batt_25,
    min(batt_25) as min_batt_25,
    avg(batt_lightning)::int as batt_lightning,
    max(batt_lightning) as max_batt_lightning,
    min(batt_lightning) as min_batt_lightning,
    avg(batleak1)::int as batleak1,
    max(batleak1) as max_batleak1,
    min(batleak1) as min_batleak1,
    avg(batleak2)::int as batleak2,
    max(batleak2) as max_batleak2,
    min(batleak2) as min_batleak2,
    avg(batleak3)::int as batleak3,
    max(batleak3) as max_batleak3,
    min(batleak3) as min_batleak3,
    avg(batleak4)::int as batleak4,
    max(batleak4) as max_batleak4,
    min(batleak4) as min_batleak4,
    avg(battsm1)::int as battsm1,
    max(battsm1) as max_battsm1,
    min(battsm1) as min_battsm1,
    avg(battsm2)::int as battsm2,
    max(battsm2) as max_battsm2,
    min(battsm2) as min_battsm2,
    avg(battsm3)::int as battsm3,
    max(battsm3) as max_battsm3,
    min(battsm3) as min_battsm3,
    avg(battsm4)::int as battsm4,
    max(battsm4) as max_battsm4,
    min(battsm4) as min_battsm4,
    avg(batt_co2)::int as batt_co2,
    max(batt_co2) as max_batt_co2,
    min(batt_co2) as min_batt_co2,
    avg(batt_cellgateway)::int as batt_cellgateway,
    max(batt_cellgateway) as max_batt_cellgateway,
    min(batt_cellgateway) as min_batt_cellgateway,
    -- Pressure measurements
    avg(baromrelin) as baromrelin,
    max(baromrelin) as max_baromrelin,
    min(baromrelin) as min_baromrelin,
    avg(baromabsin) as baromabsin,
    max(baromabsin) as max_baromabsin,
    min(baromabsin) as min_baromabsin,
    -- Relay states
    avg(relay1)::int as relay1,
    max(relay1) as max_relay1,
    min(relay1) as min_relay1,
    avg(relay2)::int as relay2,
    max(relay2) as max_relay2,
    min(relay2) as min_relay2,
    avg(relay3)::int as relay3,
    max(relay3) as max_relay3,
    min(relay3) as min_relay3,
    avg(relay4)::int as relay4,
    max(relay4) as max_relay4,
    min(relay4) as min_relay4,
    avg(relay5)::int as relay5,
    max(relay5) as max_relay5,
    min(relay5) as min_relay5,
    avg(relay6)::int as relay6,
    max(relay6) as max_relay6,
    min(relay6) as min_relay6,
    avg(relay7)::int as relay7,
    max(relay7) as max_relay7,
    min(relay7) as min_relay7,
    avg(relay8)::int as relay8,
    max(relay8) as max_relay8,
    min(relay8) as min_relay8,
    avg(relay9)::int as relay9,
    max(relay9) as max_relay9,
    min(relay9) as min_relay9,
    avg(relay10)::int as relay10,
    max(relay10) as max_relay10,
    min(relay10) as min_relay10,
    -- Air quality measurements
    avg(pm25) as pm25,
    max(pm25) as max_pm25,
    min(pm25) as min_pm25,
    avg(pm25_24h) as pm25_24h,
    max(pm25_24h) as max_pm25_24h,
    min(pm25_24h) as min_pm25_24h,
    avg(pm25_in) as pm25_in,
    max(pm25_in) as max_pm25_in,
    min(pm25_in) as min_pm25_in,
    avg(pm25_in_24h) as pm25_in_24h,
    max(pm25_in_24h) as max_pm25_in_24h,
    min(pm25_in_24h) as min_pm25_in_24h,
    avg(pm25_in_aqin) as pm25_in_aqin,
    max(pm25_in_aqin) as max_pm25_in_aqin,
    min(pm25_in_aqin) as min_pm25_in_aqin,
    avg(pm25_in_24h_aqin) as pm25_in_24h_aqin,
    max(pm25_in_24h_aqin) as max_pm25_in_24h_aqin,
    min(pm25_in_24h_aqin) as min_pm25_in_24h_aqin,
    avg(pm10_in_aqin) as pm10_in_aqin,
    max(pm10_in_aqin) as max_pm10_in_aqin,
    min(pm10_in_aqin) as min_pm10_in_aqin,
    avg(pm10_in_24h_aqin) as pm10_in_24h_aqin,
    max(pm10_in_24h_aqin) as max_pm10_in_24h_aqin,
    min(pm10_in_24h_aqin) as min_pm10_in_24h_aqin,
    avg(co2) as co2,
    max(co2) as max_co2,
    min(co2) as min_co2,
    avg(co2_in_aqin)::int as co2_in_aqin,
    max(co2_in_aqin) as max_co2_in_aqin,
    min(co2_in_aqin) as min_co2_in_aqin,
    avg(co2_in_24h_aqin)::int as co2_in_24h_aqin,
    max(co2_in_24h_aqin) as max_co2_in_24h_aqin,
    min(co2_in_24h_aqin) as min_co2_in_24h_aqin,
    avg(pm_in_temp_aqin) as pm_in_temp_aqin,
    max(pm_in_temp_aqin) as max_pm_in_temp_aqin,
    min(pm_in_temp_aqin) as min_pm_in_temp_aqin,
    avg(pm_in_humidity_aqin)::int as pm_in_humidity_aqin,
    max(pm_in_humidity_aqin) as max_pm_in_humidity_aqin,
    min(pm_in_humidity_aqin) as min_pm_in_humidity_aqin,
    avg(aqi_pm25_aqin)::int as aqi_pm25_aqin,
    max(aqi_pm25_aqin) as max_aqi_pm25_aqin,
    min(aqi_pm25_aqin) as min_aqi_pm25_aqin,
    avg(aqi_pm25_24h_aqin)::int as aqi_pm25_24h_aqin,
    max(aqi_pm25_24h_aqin) as max_aqi_pm25_24h_aqin,
    min(aqi_pm25_24h_aqin) as min_aqi_pm25_24h_aqin,
    avg(aqi_pm10_aqin)::int as aqi_pm10_aqin,
    max(aqi_pm10_aqin) as max_aqi_pm10_aqin,
    min(aqi_pm10_aqin) as min_aqi_pm10_aqin,
    avg(aqi_pm10_24h_aqin)::int as aqi_pm10_24h_aqin,
    max(aqi_pm10_24h_aqin) as max_aqi_pm10_24h_aqin,
    min(aqi_pm10_24h_aqin) as min_aqi_pm10_24h_aqin,
    avg(aqi_pm25_in)::int as aqi_pm25_in,
    max(aqi_pm25_in) as max_aqi_pm25_in,
    min(aqi_pm25_in) as min_aqi_pm25_in,
    avg(aqi_pm25_in_24h)::int as aqi_pm25_in_24h,
    max(aqi_pm25_in_24h) as max_aqi_pm25_in_24h,
    min(aqi_pm25_in_24h) as min_aqi_pm25_in_24h,
    -- Lightning data
    sum(lightning_day) as lightning_day,
    sum(lightning_hour) as lightning_hour,
    max(lightning_time) as lightning_time,
    min(lightning_distance) as lightning_distance,
    -- Other fields
    avg(radiation) as radiation,
    max(radiation) as max_radiation,
    min(radiation) as min_radiation,
    avg(uv) as uv,
    max(uv) as max_uv,
    min(uv) as min_uv
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
    min(extrafloat10) as min_extrafloat10,
    -- Temperature sensors
    avg(temp1) as temp1,
    max(temp1) as max_temp1,
    min(temp1) as min_temp1,
    avg(temp2) as temp2,
    max(temp2) as max_temp2,
    min(temp2) as min_temp2,
    avg(temp3) as temp3,
    max(temp3) as max_temp3,
    min(temp3) as min_temp3,
    avg(temp4) as temp4,
    max(temp4) as max_temp4,
    min(temp4) as min_temp4,
    avg(temp5) as temp5,
    max(temp5) as max_temp5,
    min(temp5) as min_temp5,
    avg(temp6) as temp6,
    max(temp6) as max_temp6,
    min(temp6) as min_temp6,
    avg(temp7) as temp7,
    max(temp7) as max_temp7,
    min(temp7) as min_temp7,
    avg(temp8) as temp8,
    max(temp8) as max_temp8,
    min(temp8) as min_temp8,
    avg(temp9) as temp9,
    max(temp9) as max_temp9,
    min(temp9) as min_temp9,
    avg(temp10) as temp10,
    max(temp10) as max_temp10,
    min(temp10) as min_temp10,
    -- Additional soil temperature sensors
    avg(soiltemp5) as soiltemp5,
    max(soiltemp5) as max_soiltemp5,
    min(soiltemp5) as min_soiltemp5,
    avg(soiltemp6) as soiltemp6,
    max(soiltemp6) as max_soiltemp6,
    min(soiltemp6) as min_soiltemp6,
    avg(soiltemp7) as soiltemp7,
    max(soiltemp7) as max_soiltemp7,
    min(soiltemp7) as min_soiltemp7,
    avg(soiltemp8) as soiltemp8,
    max(soiltemp8) as max_soiltemp8,
    min(soiltemp8) as min_soiltemp8,
    avg(soiltemp9) as soiltemp9,
    max(soiltemp9) as max_soiltemp9,
    min(soiltemp9) as min_soiltemp9,
    avg(soiltemp10) as soiltemp10,
    max(soiltemp10) as max_soiltemp10,
    min(soiltemp10) as min_soiltemp10,
    -- Humidity sensors
    avg(humidity1) as humidity1,
    max(humidity1) as max_humidity1,
    min(humidity1) as min_humidity1,
    avg(humidity2) as humidity2,
    max(humidity2) as max_humidity2,
    min(humidity2) as min_humidity2,
    avg(humidity3) as humidity3,
    max(humidity3) as max_humidity3,
    min(humidity3) as min_humidity3,
    avg(humidity4) as humidity4,
    max(humidity4) as max_humidity4,
    min(humidity4) as min_humidity4,
    avg(humidity5) as humidity5,
    max(humidity5) as max_humidity5,
    min(humidity5) as min_humidity5,
    avg(humidity6) as humidity6,
    max(humidity6) as max_humidity6,
    min(humidity6) as min_humidity6,
    avg(humidity7) as humidity7,
    max(humidity7) as max_humidity7,
    min(humidity7) as min_humidity7,
    avg(humidity8) as humidity8,
    max(humidity8) as max_humidity8,
    min(humidity8) as min_humidity8,
    avg(humidity9) as humidity9,
    max(humidity9) as max_humidity9,
    min(humidity9) as min_humidity9,
    avg(humidity10) as humidity10,
    max(humidity10) as max_humidity10,
    min(humidity10) as min_humidity10,
    -- Soil humidity sensors
    avg(soilhum1) as soilhum1,
    max(soilhum1) as max_soilhum1,
    min(soilhum1) as min_soilhum1,
    avg(soilhum2) as soilhum2,
    max(soilhum2) as max_soilhum2,
    min(soilhum2) as min_soilhum2,
    avg(soilhum3) as soilhum3,
    max(soilhum3) as max_soilhum3,
    min(soilhum3) as min_soilhum3,
    avg(soilhum4) as soilhum4,
    max(soilhum4) as max_soilhum4,
    min(soilhum4) as min_soilhum4,
    avg(soilhum5) as soilhum5,
    max(soilhum5) as max_soilhum5,
    min(soilhum5) as min_soilhum5,
    avg(soilhum6) as soilhum6,
    max(soilhum6) as max_soilhum6,
    min(soilhum6) as min_soilhum6,
    avg(soilhum7) as soilhum7,
    max(soilhum7) as max_soilhum7,
    min(soilhum7) as min_soilhum7,
    avg(soilhum8) as soilhum8,
    max(soilhum8) as max_soilhum8,
    min(soilhum8) as min_soilhum8,
    avg(soilhum9) as soilhum9,
    max(soilhum9) as max_soilhum9,
    min(soilhum9) as min_soilhum9,
    avg(soilhum10) as soilhum10,
    max(soilhum10) as max_soilhum10,
    min(soilhum10) as min_soilhum10,
    -- Additional leaf wetness sensors
    avg(leafwetness5) as leafwetness5,
    max(leafwetness5) as max_leafwetness5,
    min(leafwetness5) as min_leafwetness5,
    avg(leafwetness6) as leafwetness6,
    max(leafwetness6) as max_leafwetness6,
    min(leafwetness6) as min_leafwetness6,
    avg(leafwetness7) as leafwetness7,
    max(leafwetness7) as max_leafwetness7,
    min(leafwetness7) as min_leafwetness7,
    avg(leafwetness8) as leafwetness8,
    max(leafwetness8) as max_leafwetness8,
    min(leafwetness8) as min_leafwetness8,
    -- Soil tension sensors
    avg(soiltens1) as soiltens1,
    max(soiltens1) as max_soiltens1,
    min(soiltens1) as min_soiltens1,
    avg(soiltens2) as soiltens2,
    max(soiltens2) as max_soiltens2,
    min(soiltens2) as min_soiltens2,
    avg(soiltens3) as soiltens3,
    max(soiltens3) as max_soiltens3,
    min(soiltens3) as min_soiltens3,
    avg(soiltens4) as soiltens4,
    max(soiltens4) as max_soiltens4,
    min(soiltens4) as min_soiltens4,
    -- Agricultural measurements
    avg(gdd)::int as gdd,
    max(gdd) as max_gdd,
    min(gdd) as min_gdd,
    avg(etos) as etos,
    max(etos) as max_etos,
    min(etos) as min_etos,
    avg(etrs) as etrs,
    max(etrs) as max_etrs,
    min(etrs) as min_etrs,
    -- Leak detection sensors
    avg(leak1)::int as leak1,
    max(leak1) as max_leak1,
    min(leak1) as min_leak1,
    avg(leak2)::int as leak2,
    max(leak2) as max_leak2,
    min(leak2) as min_leak2,
    avg(leak3)::int as leak3,
    max(leak3) as max_leak3,
    min(leak3) as min_leak3,
    avg(leak4)::int as leak4,
    max(leak4) as max_leak4,
    min(leak4) as min_leak4,
    -- Battery status
    avg(battout)::int as battout,
    max(battout) as max_battout,
    min(battout) as min_battout,
    avg(battin)::int as battin,
    max(battin) as max_battin,
    min(battin) as min_battin,
    avg(batt1)::int as batt1,
    max(batt1) as max_batt1,
    min(batt1) as min_batt1,
    avg(batt2)::int as batt2,
    max(batt2) as max_batt2,
    min(batt2) as min_batt2,
    avg(batt3)::int as batt3,
    max(batt3) as max_batt3,
    min(batt3) as min_batt3,
    avg(batt4)::int as batt4,
    max(batt4) as max_batt4,
    min(batt4) as min_batt4,
    avg(batt5)::int as batt5,
    max(batt5) as max_batt5,
    min(batt5) as min_batt5,
    avg(batt6)::int as batt6,
    max(batt6) as max_batt6,
    min(batt6) as min_batt6,
    avg(batt7)::int as batt7,
    max(batt7) as max_batt7,
    min(batt7) as min_batt7,
    avg(batt8)::int as batt8,
    max(batt8) as max_batt8,
    min(batt8) as min_batt8,
    avg(batt9)::int as batt9,
    max(batt9) as max_batt9,
    min(batt9) as min_batt9,
    avg(batt10)::int as batt10,
    max(batt10) as max_batt10,
    min(batt10) as min_batt10,
    avg(batt_25)::int as batt_25,
    max(batt_25) as max_batt_25,
    min(batt_25) as min_batt_25,
    avg(batt_lightning)::int as batt_lightning,
    max(batt_lightning) as max_batt_lightning,
    min(batt_lightning) as min_batt_lightning,
    avg(batleak1)::int as batleak1,
    max(batleak1) as max_batleak1,
    min(batleak1) as min_batleak1,
    avg(batleak2)::int as batleak2,
    max(batleak2) as max_batleak2,
    min(batleak2) as min_batleak2,
    avg(batleak3)::int as batleak3,
    max(batleak3) as max_batleak3,
    min(batleak3) as min_batleak3,
    avg(batleak4)::int as batleak4,
    max(batleak4) as max_batleak4,
    min(batleak4) as min_batleak4,
    avg(battsm1)::int as battsm1,
    max(battsm1) as max_battsm1,
    min(battsm1) as min_battsm1,
    avg(battsm2)::int as battsm2,
    max(battsm2) as max_battsm2,
    min(battsm2) as min_battsm2,
    avg(battsm3)::int as battsm3,
    max(battsm3) as max_battsm3,
    min(battsm3) as min_battsm3,
    avg(battsm4)::int as battsm4,
    max(battsm4) as max_battsm4,
    min(battsm4) as min_battsm4,
    avg(batt_co2)::int as batt_co2,
    max(batt_co2) as max_batt_co2,
    min(batt_co2) as min_batt_co2,
    avg(batt_cellgateway)::int as batt_cellgateway,
    max(batt_cellgateway) as max_batt_cellgateway,
    min(batt_cellgateway) as min_batt_cellgateway,
    -- Pressure measurements
    avg(baromrelin) as baromrelin,
    max(baromrelin) as max_baromrelin,
    min(baromrelin) as min_baromrelin,
    avg(baromabsin) as baromabsin,
    max(baromabsin) as max_baromabsin,
    min(baromabsin) as min_baromabsin,
    -- Relay states
    avg(relay1)::int as relay1,
    max(relay1) as max_relay1,
    min(relay1) as min_relay1,
    avg(relay2)::int as relay2,
    max(relay2) as max_relay2,
    min(relay2) as min_relay2,
    avg(relay3)::int as relay3,
    max(relay3) as max_relay3,
    min(relay3) as min_relay3,
    avg(relay4)::int as relay4,
    max(relay4) as max_relay4,
    min(relay4) as min_relay4,
    avg(relay5)::int as relay5,
    max(relay5) as max_relay5,
    min(relay5) as min_relay5,
    avg(relay6)::int as relay6,
    max(relay6) as max_relay6,
    min(relay6) as min_relay6,
    avg(relay7)::int as relay7,
    max(relay7) as max_relay7,
    min(relay7) as min_relay7,
    avg(relay8)::int as relay8,
    max(relay8) as max_relay8,
    min(relay8) as min_relay8,
    avg(relay9)::int as relay9,
    max(relay9) as max_relay9,
    min(relay9) as min_relay9,
    avg(relay10)::int as relay10,
    max(relay10) as max_relay10,
    min(relay10) as min_relay10,
    -- Air quality measurements
    avg(pm25) as pm25,
    max(pm25) as max_pm25,
    min(pm25) as min_pm25,
    avg(pm25_24h) as pm25_24h,
    max(pm25_24h) as max_pm25_24h,
    min(pm25_24h) as min_pm25_24h,
    avg(pm25_in) as pm25_in,
    max(pm25_in) as max_pm25_in,
    min(pm25_in) as min_pm25_in,
    avg(pm25_in_24h) as pm25_in_24h,
    max(pm25_in_24h) as max_pm25_in_24h,
    min(pm25_in_24h) as min_pm25_in_24h,
    avg(pm25_in_aqin) as pm25_in_aqin,
    max(pm25_in_aqin) as max_pm25_in_aqin,
    min(pm25_in_aqin) as min_pm25_in_aqin,
    avg(pm25_in_24h_aqin) as pm25_in_24h_aqin,
    max(pm25_in_24h_aqin) as max_pm25_in_24h_aqin,
    min(pm25_in_24h_aqin) as min_pm25_in_24h_aqin,
    avg(pm10_in_aqin) as pm10_in_aqin,
    max(pm10_in_aqin) as max_pm10_in_aqin,
    min(pm10_in_aqin) as min_pm10_in_aqin,
    avg(pm10_in_24h_aqin) as pm10_in_24h_aqin,
    max(pm10_in_24h_aqin) as max_pm10_in_24h_aqin,
    min(pm10_in_24h_aqin) as min_pm10_in_24h_aqin,
    avg(co2) as co2,
    max(co2) as max_co2,
    min(co2) as min_co2,
    avg(co2_in_aqin)::int as co2_in_aqin,
    max(co2_in_aqin) as max_co2_in_aqin,
    min(co2_in_aqin) as min_co2_in_aqin,
    avg(co2_in_24h_aqin)::int as co2_in_24h_aqin,
    max(co2_in_24h_aqin) as max_co2_in_24h_aqin,
    min(co2_in_24h_aqin) as min_co2_in_24h_aqin,
    avg(pm_in_temp_aqin) as pm_in_temp_aqin,
    max(pm_in_temp_aqin) as max_pm_in_temp_aqin,
    min(pm_in_temp_aqin) as min_pm_in_temp_aqin,
    avg(pm_in_humidity_aqin)::int as pm_in_humidity_aqin,
    max(pm_in_humidity_aqin) as max_pm_in_humidity_aqin,
    min(pm_in_humidity_aqin) as min_pm_in_humidity_aqin,
    avg(aqi_pm25_aqin)::int as aqi_pm25_aqin,
    max(aqi_pm25_aqin) as max_aqi_pm25_aqin,
    min(aqi_pm25_aqin) as min_aqi_pm25_aqin,
    avg(aqi_pm25_24h_aqin)::int as aqi_pm25_24h_aqin,
    max(aqi_pm25_24h_aqin) as max_aqi_pm25_24h_aqin,
    min(aqi_pm25_24h_aqin) as min_aqi_pm25_24h_aqin,
    avg(aqi_pm10_aqin)::int as aqi_pm10_aqin,
    max(aqi_pm10_aqin) as max_aqi_pm10_aqin,
    min(aqi_pm10_aqin) as min_aqi_pm10_aqin,
    avg(aqi_pm10_24h_aqin)::int as aqi_pm10_24h_aqin,
    max(aqi_pm10_24h_aqin) as max_aqi_pm10_24h_aqin,
    min(aqi_pm10_24h_aqin) as min_aqi_pm10_24h_aqin,
    avg(aqi_pm25_in)::int as aqi_pm25_in,
    max(aqi_pm25_in) as max_aqi_pm25_in,
    min(aqi_pm25_in) as min_aqi_pm25_in,
    avg(aqi_pm25_in_24h)::int as aqi_pm25_in_24h,
    max(aqi_pm25_in_24h) as max_aqi_pm25_in_24h,
    min(aqi_pm25_in_24h) as min_aqi_pm25_in_24h,
    -- Lightning data
    sum(lightning_day) as lightning_day,
    sum(lightning_hour) as lightning_hour,
    max(lightning_time) as lightning_time,
    min(lightning_distance) as lightning_distance,
    -- Other fields
    avg(radiation) as radiation,
    max(radiation) as max_radiation,
    min(radiation) as min_radiation,
    avg(uv) as uv,
    max(uv) as max_uv,
    min(uv) as min_uv
FROM
    weather
GROUP BY bucket, stationname, stationtype;`

const dropRainSinceMidnightViewSQL = `DROP VIEW IF EXISTS today_rainfall;`

const createRainSinceMidnightViewSQL = `CREATE VIEW today_rainfall AS
SELECT 
    COALESCE(
        (SELECT SUM(period_rain) 
         FROM weather_5m 
         WHERE bucket >= date_trunc('day', now())
         LIMIT 1), 
        0
    ) + 
    COALESCE(
        (SELECT SUM(rainincremental) 
         FROM weather 
         WHERE time >= GREATEST(
             date_trunc('day', now()),
             (SELECT COALESCE(MAX(bucket), date_trunc('day', now())) 
              FROM weather_5m 
              LIMIT 1)
         )
         LIMIT 1), 
        0
    ) AS total_rain;`

const createIndexesSQL = `
-- Primary indexes for continuous aggregates (critical for performance)
CREATE INDEX IF NOT EXISTS weather_1m_stationname_bucket_idx ON weather_1m (stationname, bucket DESC);
CREATE INDEX IF NOT EXISTS weather_5m_stationname_bucket_idx ON weather_5m (stationname, bucket DESC);
CREATE INDEX IF NOT EXISTS weather_1h_stationname_bucket_idx ON weather_1h (stationname, bucket DESC);
CREATE INDEX IF NOT EXISTS weather_1d_stationname_bucket_idx ON weather_1d (stationname, bucket DESC);
-- Legacy indexes (kept for compatibility)
CREATE INDEX IF NOT EXISTS weather_1m_bucket_stationname_idx ON weather_1m (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_5m_bucket_stationname_idx ON weather_5m (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_1h_bucket_stationname_idx ON weather_1h (stationname, bucket);
CREATE INDEX IF NOT EXISTS weather_1d_bucket_stationname_idx ON weather_1d (stationname, bucket);
-- Weather table indexes
CREATE INDEX IF NOT EXISTS weather_stationname_time_idx ON weather (stationname, time DESC);
CREATE INDEX IF NOT EXISTS weather_time_stationname_idx ON weather (time DESC, stationname);
-- Rainfall summary index
CREATE INDEX IF NOT EXISTS rainfall_summary_stationname_idx ON rainfall_summary (stationname);`

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

const createCurrentSnowfallRateSQL = `CREATE OR REPLACE FUNCTION calculate_current_snowfall_rate(
    p_stationname TEXT
) RETURNS FLOAT AS $$
DECLARE
    current_snowdistance FLOAT;
    past_snowdistance FLOAT;
    snowfall_30min FLOAT;
    hourly_rate FLOAT;
    current_ts TIMESTAMPTZ;
    past_ts TIMESTAMPTZ;
BEGIN
    -- Get the current time
    current_ts := now();
    past_ts := current_ts - interval '30 minutes';

    -- Get the most recent snowdistance reading
    SELECT snowdistance INTO current_snowdistance
    FROM weather_1m
    WHERE weather_1m.stationname = p_stationname
      AND bucket >= current_ts - interval '5 minutes'  -- Within last 5 minutes
      AND snowdistance IS NOT NULL
    ORDER BY bucket DESC
    LIMIT 1;

    -- Get the snowdistance from 30 minutes ago (closest reading)
    SELECT snowdistance INTO past_snowdistance
    FROM weather_1m
    WHERE weather_1m.stationname = p_stationname
      AND bucket >= past_ts - interval '2 minutes'  -- Allow 2-minute window
      AND bucket <= past_ts + interval '2 minutes'
      AND snowdistance IS NOT NULL
    ORDER BY abs(extract(epoch from (bucket - past_ts)))
    LIMIT 1;

    -- If we don't have both readings, return 0
    IF current_snowdistance IS NULL OR past_snowdistance IS NULL THEN
        RETURN 0.0;
    END IF;

    -- Calculate 30-minute snowfall (positive values only)
    snowfall_30min := past_snowdistance - current_snowdistance;
    
    -- If no positive snowfall, return 0
    IF snowfall_30min <= 0 THEN
        RETURN 0.0;
    END IF;

    -- Extrapolate to hourly rate (30 minutes * 2 = 60 minutes)
    hourly_rate := snowfall_30min * 2;

    -- Return the estimated hourly snowfall rate
    RETURN hourly_rate;
END;
$$ LANGUAGE plpgsql;`

const createCurrentRainfallRateSQL = `CREATE OR REPLACE FUNCTION calculate_current_rainfall_rate(
    p_stationname TEXT
) RETURNS FLOAT AS $$
DECLARE
    rainfall_30min FLOAT := 0.0;
    hourly_rate FLOAT;
    current_ts TIMESTAMPTZ;
    past_ts TIMESTAMPTZ;
BEGIN
    -- Get the current time
    current_ts := now();
    past_ts := current_ts - interval '30 minutes';

    -- Sum all rainfall in the last 30 minutes from weather_1m
    SELECT COALESCE(SUM(period_rain), 0) INTO rainfall_30min
    FROM weather_1m
    WHERE weather_1m.stationname = p_stationname
      AND bucket >= past_ts
      AND bucket <= current_ts
      AND period_rain IS NOT NULL;

    -- If no rainfall in the last 30 minutes, return 0
    IF rainfall_30min <= 0 THEN
        RETURN 0.0;
    END IF;

    -- Extrapolate to hourly rate (30 minutes * 2 = 60 minutes)
    hourly_rate := rainfall_30min * 2;

    -- Return the estimated hourly rainfall rate
    RETURN hourly_rate;
END;
$$ LANGUAGE plpgsql;`

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

const createSnowSeasonTotalSQL = `DROP FUNCTION IF EXISTS calculate_total_season_snowfall(TEXT, FLOAT, TIMESTAMPTZ);
CREATE OR REPLACE FUNCTION calculate_total_season_snowfall(
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

    -- Add today's snowfall by comparing latest reading to most recent weather_1d value (only if we're in the current season)
    IF now() >= local_start_of_season AND now() < season_end THEN
        DECLARE
            latest_raw_snowdistance FLOAT;
            latest_daily_snowdistance FLOAT;
            today_start_snowdistance FLOAT;
        BEGIN
            -- Get the most recent raw snowdistance reading
            SELECT snowdistance INTO latest_raw_snowdistance
            FROM weather
            WHERE weather.stationname = p_stationname
              AND snowdistance IS NOT NULL
              AND snowdistance <= base_distance
            ORDER BY time DESC
            LIMIT 1;

            -- Get the most recent daily aggregated snowdistance (yesterday or earlier)
            SELECT snowdistance INTO latest_daily_snowdistance
            FROM weather_1d
            WHERE weather_1d.stationname = p_stationname
              AND bucket < date_trunc('day', now())  -- Before today
              AND snowdistance IS NOT NULL
            ORDER BY bucket DESC
            LIMIT 1;

            -- If we have both readings, calculate today's snowfall
            IF latest_raw_snowdistance IS NOT NULL AND latest_daily_snowdistance IS NOT NULL THEN
                today_snowfall := latest_daily_snowdistance - latest_raw_snowdistance;
                -- Only add positive snowfall
                IF today_snowfall > 0 THEN
                    total_snowfall := total_snowfall + today_snowfall;
                END IF;
            END IF;
        END;
    END IF;

    -- Return the total snowfall, ensuring it's not negative
    RETURN GREATEST(total_snowfall, 0.0);
END;
$$ LANGUAGE plpgsql;
`

const createRainStormTotalSQL = `CREATE OR REPLACE FUNCTION calculate_storm_rainfall(
    p_stationname TEXT
) RETURNS TABLE (
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_rainfall FLOAT
) AS $$
DECLARE
    storm_start_ts TIMESTAMPTZ;
    storm_end_ts TIMESTAMPTZ;
    total_rainfall_amount FLOAT := 0.0;
BEGIN
    -- Simple approach: storm is any rainfall in the last 24 hours
    -- Calculate total rainfall in the last 24 hours
    SELECT COALESCE(SUM(period_rain), 0) INTO total_rainfall_amount
    FROM weather_5m
    WHERE weather_5m.stationname = p_stationname
      AND bucket >= now() - interval '24 hours'
      AND period_rain IS NOT NULL
      AND period_rain > 0;

    -- If no rainfall, return no storm
    IF total_rainfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Set storm period as last 24 hours
    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    -- Return the storm information
    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_rainfall_amount;
END;
$$ LANGUAGE plpgsql;`

const createSnowStormTotalSQL = `CREATE OR REPLACE FUNCTION calculate_storm_snowfall(
    p_stationname TEXT
) RETURNS TABLE (
    storm_start TIMESTAMPTZ,
    storm_end TIMESTAMPTZ,
    total_snowfall FLOAT
) AS $$
DECLARE
    storm_start_ts TIMESTAMPTZ;
    storm_end_ts TIMESTAMPTZ;
    total_snowfall_amount FLOAT := 0.0;
    first_reading FLOAT;
    latest_reading FLOAT;
BEGIN
    -- Simple approach: storm is any snowfall in the last 24 hours
    -- Get the earliest snowdistance reading in the last 24 hours
    SELECT snowdistance INTO first_reading
    FROM weather
    WHERE weather.stationname = p_stationname
      AND time >= now() - interval '24 hours'
      AND snowdistance IS NOT NULL
    ORDER BY time ASC
    LIMIT 1;

    -- Get the latest snowdistance reading in the last 24 hours
    SELECT snowdistance INTO latest_reading
    FROM weather
    WHERE weather.stationname = p_stationname
      AND time >= now() - interval '24 hours'
      AND snowdistance IS NOT NULL
    ORDER BY time DESC
    LIMIT 1;

    -- If no readings, return no storm
    IF first_reading IS NULL OR latest_reading IS NULL THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Calculate total snowfall as difference between first and latest readings
    total_snowfall_amount := first_reading - latest_reading;
    
    -- If no positive snowfall, return no storm
    IF total_snowfall_amount <= 0 THEN
        RETURN QUERY SELECT NULL::TIMESTAMPTZ, NULL::TIMESTAMPTZ, 0::FLOAT;
        RETURN;
    END IF;

    -- Set storm period as last 24 hours
    storm_start_ts := now() - interval '24 hours';
    storm_end_ts := now();

    -- Return the storm information
    RETURN QUERY SELECT storm_start_ts, storm_end_ts, total_snowfall_amount;
END;
$$ LANGUAGE plpgsql;`

const createWindGustSQL = `CREATE OR REPLACE FUNCTION calculate_wind_gust(
    p_stationname TEXT
) RETURNS FLOAT AS $$
DECLARE
    max_windspeed FLOAT;
BEGIN
    -- Get the maximum windspeed from the last 10 minutes
    SELECT MAX(windspeed) INTO max_windspeed
    FROM weather
    WHERE weather.stationname = p_stationname
      AND time >= now() - interval '10 minutes'
      AND windspeed IS NOT NULL;

    -- Return the maximum windspeed, or 0 if no readings found
    RETURN COALESCE(max_windspeed, 0.0);
END;
$$ LANGUAGE plpgsql;`

const createRainfallSummaryTableSQL = `CREATE TABLE IF NOT EXISTS rainfall_summary (
    stationname TEXT PRIMARY KEY,
    rain_24h REAL DEFAULT 0,
    rain_48h REAL DEFAULT 0,
    rain_72h REAL DEFAULT 0,
    last_updated TIMESTAMPTZ DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_rainfall_summary_stationname 
ON rainfall_summary (stationname);`

const createUpdateRainfallSummarySQL = `CREATE OR REPLACE FUNCTION update_rainfall_summary(job_id INT DEFAULT NULL, config JSONB DEFAULT NULL)
RETURNS void AS $$
DECLARE
    last_hour TIMESTAMPTZ;
    last_5min TIMESTAMPTZ;
    cutoff_24h TIMESTAMPTZ;
    cutoff_48h TIMESTAMPTZ;
    cutoff_72h TIMESTAMPTZ;
BEGIN
    -- Calculate time boundaries
    last_hour := date_trunc('hour', NOW());
    last_5min := date_trunc('hour', NOW()) + 
        (EXTRACT(MINUTE FROM NOW())::INT / 5 * 5 || ' minutes')::INTERVAL;
    cutoff_24h := NOW() - INTERVAL '24 hours';
    cutoff_48h := NOW() - INTERVAL '48 hours';
    cutoff_72h := NOW() - INTERVAL '72 hours';
    
    -- Update all active stations using hierarchical aggregates
    INSERT INTO rainfall_summary (stationname, rain_24h, rain_48h, rain_72h, last_updated)
    SELECT 
        s.stationname,
        -- 24-hour rainfall: hourly + 5min + recent
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_24h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_24h,
        
        -- 48-hour rainfall
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_48h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_48h,
        
        -- 72-hour rainfall
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_1h 
             WHERE stationname = s.stationname 
             AND bucket >= cutoff_72h AND bucket < last_hour), 0) +
        COALESCE(
            (SELECT SUM(period_rain) FROM weather_5m 
             WHERE stationname = s.stationname 
             AND bucket >= last_hour AND bucket < last_5min), 0) +
        COALESCE(
            (SELECT SUM(rainincremental) FROM weather 
             WHERE stationname = s.stationname 
             AND time >= last_5min), 0) as rain_72h,
        
        NOW()
    FROM (
        -- Only update stations that have recent activity
        SELECT DISTINCT stationname 
        FROM weather 
        WHERE time >= NOW() - INTERVAL '10 minutes'
    ) s
    ON CONFLICT (stationname) DO UPDATE SET
        rain_24h = EXCLUDED.rain_24h,
        rain_48h = EXCLUDED.rain_48h,
        rain_72h = EXCLUDED.rain_72h,
        last_updated = EXCLUDED.last_updated;
END;
$$ LANGUAGE plpgsql;`

const createGetRainfallWithRecentSQL = `CREATE OR REPLACE FUNCTION get_rainfall_with_recent(p_stationname TEXT)
RETURNS TABLE(rain_24h REAL, rain_48h REAL, rain_72h REAL) AS $$
DECLARE
    v_rain_24h REAL;
    v_rain_48h REAL;
    v_rain_72h REAL;
    v_last_updated TIMESTAMPTZ;
    v_recent_rain REAL;
BEGIN
    -- Get the summary data
    SELECT rs.rain_24h, rs.rain_48h, rs.rain_72h, rs.last_updated
    INTO v_rain_24h, v_rain_48h, v_rain_72h, v_last_updated
    FROM rainfall_summary rs
    WHERE rs.stationname = p_stationname
    LIMIT 1;
    
    -- If no summary exists, calculate from scratch
    IF NOT FOUND THEN
        RETURN QUERY
        SELECT 
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '24 hours' THEN period_rain END), 0)::REAL,
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '48 hours' THEN period_rain END), 0)::REAL,
            COALESCE(SUM(CASE WHEN bucket >= NOW() - INTERVAL '72 hours' THEN period_rain END), 0)::REAL
        FROM weather_5m
        WHERE stationname = p_stationname 
        AND bucket >= NOW() - INTERVAL '72 hours';
        RETURN;
    END IF;
    
    -- Get recent rain since last update
    SELECT COALESCE(SUM(rainincremental), 0)
    INTO v_recent_rain
    FROM weather
    WHERE stationname = p_stationname
    AND time > v_last_updated;
    
    -- Return combined values
    RETURN QUERY 
    SELECT 
        (v_rain_24h + v_recent_rain)::REAL,
        (v_rain_48h + v_recent_rain)::REAL,
        (v_rain_72h + v_recent_rain)::REAL;
END;
$$ LANGUAGE plpgsql;`

const addRainfallSummaryJobSQL = `DO $$
BEGIN
    -- Check if job already exists before creating
    IF NOT EXISTS (
        SELECT 1 FROM timescaledb_information.jobs 
        WHERE proc_name = 'update_rainfall_summary'
    ) THEN
        PERFORM add_job(
            'update_rainfall_summary',
            '1 minute',
            initial_start => NOW()
        );
    END IF;
END $$;`
