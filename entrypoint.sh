#!/bin/bash -e

if [ -z "${GOPHERWX_CONFIG}" ]; then
  echo The env var GOPHERWX_CONFIG needs to be defined. 
  echo This variable points gopherwx towards its config file.
  echo This image accepts a volume, /config, that you can
  echo use for passing in a config file externally.
  echo Exiting...
  exit 1
fi

if [ $GOPHERWX_DEBUG = "true" ]; then
  exec /gopherwx -config=$GOPHERWX_CONFIG -debug
else
  exec /gopherwx -config=$GOPHERWX_CONFIG
fi
