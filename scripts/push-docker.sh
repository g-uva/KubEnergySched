#!/bin/bash

# I am saving the token in an .env file for security reasons.
# The token should expire after 90 days.
set -a 
source .env
set +a
if [ -z "$GHCR_TOKEN" ]; then
  echo "Error: GHCR_TOKEN is not set in .env/GHCR_TOKEN"
  exit 1
fi
echo $GHCR_TOKEN | docker login ghcr.io -u g-uva --password-stdin

pwd

docker build -t ghcr.io/g-uva/centralunit:latest ./manifest/templates/centralunit-deployment.yaml
docker build -t ghcr.io/g-uva/compute-node:latest ./manifest/templates/compute-node.yaml