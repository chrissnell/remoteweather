#!/bin/sh
systemctl stop remoteweather.service || true
systemctl disable remoteweather.service || true
exit 0