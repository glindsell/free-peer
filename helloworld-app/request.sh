#!/bin/bash

# This script sends 100 client requests.

i="0"

while [ $i -lt 100 ]
do
go run greeter_client/main.go
i=$[$i+1]
done
