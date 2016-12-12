# gopherwx 

**gopherwx** is a service that pulls live weather data from a Davis Instruments Vantage Pro2 station and stores it in an InfluxDB database, where it can be used to make a live display and historical weather graphs.

## Quick Start
You will need a few things to use **gopherwx**:

1. A Davis Instruments VantagePro or [VantagePro 2](http://www.davisnet.com/product/wireless-vantage-pro2-with-standard-radiation-shield/) weather station.
2. A Davis Instruments [Wireless Weather Envoy](http://www.davisnet.com/product/wireless-weather-envoy/).  This device has a 900MHz reciever that decodes the transmissions from the VantagePro station and makes them available over TCP/IP.  Note: **gopherwx** does not currently support direct serial connection to a VantagePro console because the author doesn't have a wired Weatherlink device.  However, this could be easily implemented if someone had a WL or a WL clone to loan.
3. [InfluxDB](https://github.com/influxdata/influxdb) configured on your server.

The easiest and recommended way to use **gopherwx** is to use the ready-made Docker image, `chrissnell/gopherwx`.  This image makes use of `gosu` to drop root privileges to `nobody:nobody`. I have included an example [Docker Compose file](https://github.com/chrissnell/gopherwx/blob/master/example/docker-compose.yml) and [systemd unit file](https://github.com/chrissnell/gopherwx/blob/master/example/gopherwx.service) to get you started.

To use Dockerized **gopherwx**, follow these steps:

1. Drop the systemd unit file `gopherwx.service` wherever you keep your aftermarket unit files.  On my Ubuntu server, that's `/etc/systemd/user/`.  

2. Create a directory for the **gopherwx** configuration and compose files.  I recommend `/etc/gopherwx`.  If you call it something different, be sure to edit the `gopherwx.service` and `docker-compose.yml` files to reflect your path.  Most folks won't have to edit these.

3. Copy the `config.yaml` and the `docker-compose.yml` files from this GitHub repo into your `/etc/gopherwx` directory.

4. Start `gopherwx.service` by running this as root:  `systemctl start gopherwx.service`

5. Have a look at gopherwx logs to make sure everything is working correctly: `journalctl -u gopherwx -f`

6. Make sure that `gopherwx.service` starts at boot time by running `systemctl enable /etc/gopherwx/user/gopherwx.service`
