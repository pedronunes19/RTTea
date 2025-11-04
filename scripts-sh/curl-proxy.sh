#!/bin/bash

# take input args
IP=$1
IP_PROXY=$2
AUTH_STRING=$3


# auth variations
if [ -z "$AUTH_STRING" ]; then
  curl --proxy "$IP_PROXY" "$IP" -w "%{time_connect} %{time_total}"
else
  curl -U "$AUTH_STRING" --proxy "$IP_PROXY" "$IP" -w "%{time_connect} %{time_total}"
fi

