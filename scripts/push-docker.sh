#!/bin/bash

# I am saving the token in an .env file for security reasons.
# The token should expire after 90 days.
GHCR_TOKEN=$(cat .env/GHCR_TOKEN)
if [ -z "$GHCR_TOKEN" ]; then
  echo "Error: GHCR_TOKEN is not set in .env/GHCR_TOKEN"
  exit 1
fi
echo $GHCR_TOKEN | docker login ghcr.io -u g-uva --password-stdin

docker build -t ghrc.io/g-uva/centralunit:latest ../manifest/templates/centralunit-deployment.yaml
docker build -t ghrc.io/g-uva/compute-node:latest ../manifest/templates/compute-node.yaml