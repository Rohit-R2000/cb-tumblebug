#!/bin/bash

source ../conf.env

echo "####################################################################"
echo "## 3. image: List"
echo "####################################################################"


curl -sX GET http://localhost:1323/tumblebug/ns/$NS_ID/resources/image | json_pp #|| return 1
