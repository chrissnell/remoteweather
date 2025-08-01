# remoteweather 

**remoteweather** is a service that pulls live weather data from your weather station hardware, stores it in a time-series database, and shares it over a variety of mediums.  It supports popular weather services, gRPC, APRS/CWOP/AX.25 (both RF and APRS-IS routing), along with an attractive and responsive live-streaming Web interface for desktops, tablets, and smartphones.

## Features
* Powers a [dynamically-generated live weather website](https://suncrestweather.com) with live charts and live-updating readings.
* Stores historical data in TimescaleDB for graphing and analysis, and rapid delivery of data for charting.
* Sends live weather data to PWS Weather, Weather Underground, and others (API keys required)
* Sends data to APRS/CWOP via APRS-IS.  Ham radio license or CWOP station registration required.
* Streams live data over [gRPC](https://grpc.io).  Roll your own client or [try my Linux client](https://github.com/chrissnell/grpc-weather-bar).
* Supports Campbell Scientific Inc. and Davis Weather Instruments stations.  Stations are implemented as a Go interface so developers can add other implementations.


## Quick Start
You will need a few things to use **remoteweather**:

1. A Campbell Scientific Inc station and datalogger, or a Davis Instruments VantagePro or [VantagePro 2](http://www.davisnet.com/product/wireless-vantage-pro2-with-standard-radiation-shield/) weather station.  This should also work with the Davis Instruments Vue but I haven't tested.

2. A way to connect your station to a Linux server.  There are several options for this:
  *  For Campbell Scientific, you can use a RF407 device (900MHz), direct serial, or ethernet
  *  For Davis Instruments, you will need a WeatherLink (or clone) serial/USB cable to connect your VantagePro console to your server.
  *  A Davis Instruments [Wireless Weather Envoy](http://www.davisnet.com/product/wireless-weather-envoy/) is also a good option for Davis stations.  This device has a 900MHz reciever that decodes the transmissions from the VantagePro station and makes them available over TCP/IP. 

3. One or more of the following:
  *  [TimescaleDB](https://github.com/timescale/timescaledb), if you want to store data over time and power a live website like the one I have at [suncrestweather.com](https://suncrestweather.com).
  *  A [PWS Weather API account](https://www.pwsweather.com) for sending live data to PWS.
  *  A [Weather Underground API account](https://www.wunderground.com/api) for sending live data to WU.   The base account level is free and is sufficient.
  *  A ham radio license if you want to send live data to APRS-IS
  *  A [CWOP](http://wxqa.com/) ID if you want to send live data to CWOP
  *  [grpc-weather-bar](https://github.com/chrissnell/grpc-weather-bar), if you want to display live weather on your Linux desktop

### Installation

The easiest and recommended way to use **remoteweather** is to use the ready-made Docker image, `chrissnell/remoteweather/v4.0`.  This image makes use of `gosu` to drop root privileges to `nobody:nobody`. I have included an example [Docker Compose file](https://github.com/chrissnell/remoteweather/blob/master/example/docker-compose.yml) and [systemd unit file](https://github.com/chrissnell/remoteweather/blob/master/example/remoteweather.service) to get you started.

To use Dockerized **remoteweather**, follow these steps:

1. Drop the systemd unit file `remoteweather.service` wherever you keep your aftermarket unit files.  On my Ubuntu server, that's `/etc/systemd/user/`.  

2. Create a directory for the **remoteweather** configuration and compose files.  I recommend `/etc/remoteweather`.  If you call it something different, be sure to edit the `remoteweather.service` and `docker-compose.yml` files to reflect your path.  Most folks won't have to edit these.

3. Copy the `config.yaml` and the `docker-compose.yml` files from this GitHub repo into your `/etc/remoteweather` directory.

4. Start `remoteweather.service` by running this as root:  `systemctl start remoteweather.service`

5. Have a look at remoteweather logs to make sure everything is working correctly: `journalctl -u remoteweather -f`

6. Make sure that `remoteweather.service` starts at boot time by running `systemctl enable /etc/remoteweather/user/remoteweather.service`

## gRPC Support

remoteweather includes a built-in **gRPC** server that can serve up a stream of live weather readings to compatible clients.  I have written an example client, [grpc-weather-bar](https://github.com/chrissnell/grpc-weather-bar), that reads live weather from remoteweather over the network and display it within [Polybar](https://github.com/jaagr/polybar), a desktop stats bar for Linux.  

If you would like to build your own client, have a look at the [protobuf spec](https://github.com/chrissnell/remoteweather/blob/master/protobuf/grpcweather.proto).

## Documentation

Additional documentation can be found in the [docs/](docs/) directory:

- [Dynamic Configuration Reloading](docs/DYNAMIC_CONFIG.md) - How to reload configuration without restarting the service
- [SQLite Configuration Backend](docs/SQLITE_CONFIG_BACKEND.md) - Using SQLite instead of YAML for configuration
- [Commands](docs/COMMANDS.md) - Command-line utilities and tools
- [Packaging](docs/PACKAGING.md) - Information about building and packaging
