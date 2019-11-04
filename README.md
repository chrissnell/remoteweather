# gopherwx 

**gopherwx** is a service that pulls live weather data from a Davis Instruments Vantage Pro2 station and does stuff with it.  

## Features
* Powers a [live weather website](https://carlisleweather.com) with graphs (front-end code available [here](https://github.com/chrissnell/mhkweather.com))
* Stores historical data in InfluxDB for graphing and analysis.
* Sends live weather data to Weather Underground for use as a WU Personal Weather Station (PWS).  WU API key required.
* Sends data to APRS/CWOP.  Ham radio license or CWOP station registration required.
* Streams live data over [gRPC](https://grpc.io).  Roll your own client or [try my Linux client](https://github.com/chrissnell/grpc-weather-bar).


## Quick Start
You will need a few things to use **gopherwx**:

1. A Davis Instruments VantagePro or [VantagePro 2](http://www.davisnet.com/product/wireless-vantage-pro2-with-standard-radiation-shield/) weather station.  This may work with the Vue.  I'm not certain.

2. A way to connect your station to a Linux server.  There are several options for this:
  *  A WeatherLink (or clone) serial/USB cable to connect your VantagePro console to your server.
  *  A Davis Instruments [Wireless Weather Envoy](http://www.davisnet.com/product/wireless-weather-envoy/).  This device has a 900MHz reciever that decodes the transmissions from the VantagePro station and makes them available over TCP/IP. 

3. One or more of the following:
  *  [InfluxDB](https://github.com/influxdata/influxdb), if you want to have your own weather website like the one I have at [carlisleweather.com](https://carlisleweather.com).
  *  A [Weather Underground API account](https://www.wunderground.com/api) for sending live data to WU.   The base account level is free and is sufficient.
  *  A ham radio license if you want to send live data to APRS-IS
  *  A [CWOP](http://wxqa.com/) ID if you want to send live data to CWOP
  *  [grpc-weather-bar](https://github.com/chrissnell/grpc-weather-bar), if you want to display live weather on your Linux desktop

### Installation

The easiest and recommended way to use **gopherwx** is to use the ready-made Docker image, `chrissnell/gopherwx`.  This image makes use of `gosu` to drop root privileges to `nobody:nobody`. I have included an example [Docker Compose file](https://github.com/chrissnell/gopherwx/blob/master/example/docker-compose.yml) and [systemd unit file](https://github.com/chrissnell/gopherwx/blob/master/example/gopherwx.service) to get you started.

To use Dockerized **gopherwx**, follow these steps:

1. Drop the systemd unit file `gopherwx.service` wherever you keep your aftermarket unit files.  On my Ubuntu server, that's `/etc/systemd/user/`.  

2. Create a directory for the **gopherwx** configuration and compose files.  I recommend `/etc/gopherwx`.  If you call it something different, be sure to edit the `gopherwx.service` and `docker-compose.yml` files to reflect your path.  Most folks won't have to edit these.

3. Copy the `config.yaml` and the `docker-compose.yml` files from this GitHub repo into your `/etc/gopherwx` directory.

4. Start `gopherwx.service` by running this as root:  `systemctl start gopherwx.service`

5. Have a look at gopherwx logs to make sure everything is working correctly: `journalctl -u gopherwx -f`

6. Make sure that `gopherwx.service` starts at boot time by running `systemctl enable /etc/gopherwx/user/gopherwx.service`

## Setting up a live weather website

**gopherwx** doesn't serve a weather website, it only pulls the weather readings from the Davis device and stores them in the InfluxDB database for you to use as you wish.  To make a website for your data, you will need a couple of additional components:

* A live weather website.  I have provided the source code/HTML for https://carlisleweather.com in [this repository](https://github.com/chrissnell/mhkweather.com).

* [**weather-proxy**](https://github.com/chrissnell/weather-proxy) - Acts a gateway between people on the Internet and your InfluxDB database.  Since InfluxDB isn't really suitable for exposing directly to the internet, `weather-proxy` acts as a middleman, handling requests from your website visitors' browsers and querying InfluxDB to pull the requested weather data.  `weather-proxy` is suitable for exposure to the open Internet.

## gRPC Support

gopherwx includes a built-in **gRPC** server that can serve up a stream of live weather readings to compatible clients.  I have written an example client, [grpc-weather-bar](https://github.com/chrissnell/grpc-weather-bar), that reads live weather from gopherwx over the network and display it within [Polybar](https://github.com/jaagr/polybar), a desktop stats bar for Linux.  

If you would like to build your own client, have a look at the [protobuf spec](https://github.com/chrissnell/gopherwx/blob/master/protobuf/grpcweather.proto).
