-- Migration script to convert old YAML config to SQLite database
-- This script preserves all exact values including passwords, coordinates, and device names

BEGIN TRANSACTION;

-- Get the config ID (should be 1 for the default config)
-- First clear any existing data to ensure clean migration
DELETE FROM devices WHERE config_id = 1;
DELETE FROM storage_configs WHERE config_id = 1;
DELETE FROM controller_configs WHERE config_id = 1;
DELETE FROM weather_websites WHERE config_id = 1;

-- Insert the CSI device (Campbell Scientific)
INSERT INTO devices (
    config_id, name, type, enabled, hostname, port, serial_device, baud, 
    wind_dir_correction, base_snow_distance, website_id, 
    latitude, longitude, altitude, aprs_enabled, aprs_callsign
) VALUES (
    1, 'CSI', 'campbellscientific', 1, NULL, NULL, '/dev/ttyS0', 115200,
    -90, NULL, NULL,
    40.475737, -111.845664, 1900, 0, NULL
);

-- Insert TimescaleDB storage configuration
INSERT INTO storage_configs (
    config_id, backend_type, enabled, timescale_connection_string
) VALUES (
    1, 'timescaledb', 1, 'host=localhost port=5432 dbname=weather user=weather password=KYB*uvh@gvr4ydz7wae TimeZone=US/Mountain'
);

-- Insert gRPC storage configuration
INSERT INTO storage_configs (
    config_id, backend_type, enabled, grpc_cert, grpc_key, grpc_listen_addr, grpc_port, grpc_pull_from_device
) VALUES (
    1, 'grpc', 1, '/etc/letsencrypt/live/home.chrissnell.com/fullchain.pem', '/etc/letsencrypt/live/home.chrissnell.com/privkey.pem', 
    '0.0.0.0', 7500, 'CSI'
);

-- Insert APRS storage configuration
INSERT INTO storage_configs (
    config_id, backend_type, enabled, aprs_callsign, aprs_server, aprs_location_lat, aprs_location_lon
) VALUES (
    1, 'aprs', 1, 'NW5W-12', 'rotate.aprs2.net:14580', 40.475819, -111.845340
);

-- Insert AerisWeather controller
INSERT INTO controller_configs (
    config_id, controller_type, enabled, aeris_api_client_id, aeris_api_client_secret, 
    aeris_latitude, aeris_longitude
) VALUES (
    1, 'aerisweather', 1, 'ifQYbbGuoyTrVhxo0R0Or', 'EaRkxdhK5kJ1qYXN8lSoEZFmdOp4QgraB8c5IRvQ',
    40.49399730034367, -111.80614334777675
);

-- Insert PWS Weather controller
INSERT INTO controller_configs (
    config_id, controller_type, enabled, pws_station_id, pws_api_key, pws_upload_interval, pws_pull_from_device
) VALUES (
    1, 'pwsweather', 1, 'SUNCRESTUTAH', '91f78eb9553d7b993eb2200a862eed9a', '60', 'CSI'
);

-- Insert Weather Underground controller
INSERT INTO controller_configs (
    config_id, controller_type, enabled, wu_station_id, wu_api_key, wu_upload_interval, wu_pull_from_device
) VALUES (
    1, 'weatherunderground', 1, 'KUTDRAPE187', 'QSRLWoCy', '60', 'CSI'
);

-- Insert REST controller
INSERT INTO controller_configs (
    config_id, controller_type, enabled, rest_cert, rest_key, rest_port, rest_listen_addr
) VALUES (
    1, 'rest', 1, '/etc/letsencrypt/live/home.chrissnell.com/fullchain.pem', '/etc/letsencrypt/live/home.chrissnell.com/privkey.pem',
    7501, '0.0.0.0'
);

-- Insert weather website configuration
INSERT INTO weather_websites (
    config_id, name, device_id, hostname, page_title, about_station_html, snow_enabled, snow_device_name
) VALUES (
    1, 'Suncrest Live Weather', 'CSI', NULL, 'Suncrest Weather Server', 
    '<h2>About This Station</h2>
<p>
This weather station sits high on a rooftop in the Oak Vista subdivision of Suncrest, Utah.
The weather measurements update every three seconds. Graphs are updated automatically as new data is collected.
</p>
<h4>Station Hardware</h4>
<p>
This is a scientific-grade station from Campbell Scientific Inc. and is solar-powered and wireless. The station has sensors for temperature, rainfall, humidity, barometric pressure, wind speed and direction, and solar radiation.  The rainfall sensor only works when temperatures are above freezing and will not measure snowfall.
The readings are sent wirelessly over 900MHz to a receiver inside the house, which is attached to a server via USB.
</p>
<h4>Station Software</h4>
<p>
The software powering this station is RemoteWeather and was written by me, Chris Snell, in the Go programming language.
</p>
<h4>Other Ways to Use This Station</h4>
<p>
In addition to suncrestweather.com, this station also regularly transmits its data to the following services:
</p>
<p>
APRS/CWOP: <a href="https://aprs.fi/#!call=a%2FNW5W-12">https://aprs.fi/#!call=a%2FNW5W-12</a>
</p>
<p>
PWS Weather: <a href="https://www.pwsweather.com/station/pws/suncrestutah">https://www.pwsweather.com/station/pws/suncrestutah</a>
</p>',
    0, 'backyard-snow'
);

COMMIT; 