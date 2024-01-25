#!/bin/sh


if [ -z "${REMOTEWEATHER_CONFIG}" ]; then
  echo The env var REMOTEWEATHER_CONFIG needs to be defined. 
  echo This variable points RemoteWeather towards its config file.
  echo This image accepts a volume, /config, that you can
  echo use for passing in a config file externally.
  echo Exiting...
  exit 1
fi

if [ "$REMOTEWEATHER_DEBUG" = "true" ]; then
  exec /remoteweather -config=$REMOTEWEATHER_CONFIG -debug
else
  exec /remoteweather -config=$REMOTEWEATHER_CONFIG
fi
