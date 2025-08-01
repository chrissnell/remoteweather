#!/bin/sh
getent group remoteweather >/dev/null || groupadd -r remoteweather
getent passwd remoteweather >/dev/null || \
    useradd -r -g remoteweather -d /var/lib/remoteweather -s /sbin/nologin \
    -c "RemoteWeather daemon" remoteweather
exit 0