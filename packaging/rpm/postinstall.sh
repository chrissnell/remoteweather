#!/bin/sh
mkdir -p /var/lib/remoteweather
chown remoteweather:remoteweather /var/lib/remoteweather
chmod 755 /var/lib/remoteweather
systemctl daemon-reload
systemctl enable remoteweather.service
exit 0