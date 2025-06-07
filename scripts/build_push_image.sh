#!/bin/bash

# Depending on where we're executing this script, we need it's "absolute" path.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR/..

# Central Unit
docker build -f ./centralunit/Dockerfile -t goncaloferreirauva/centralunit .
docker push goncaloferreirauva/centralunit:latest

# # Compute node
docker build -f ./computenode/Dockerfile -t goncaloferreirauva/computenode .
docker push goncaloferreirauva/computenode:latest
