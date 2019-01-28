#!/bin/bash

# This script sends 100 client requests.

i="0"

while [ true ]
do
go run peer_client/main.go $i
i=$[$i+1]
done
