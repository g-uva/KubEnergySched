#!/bin/bash
cd .. # navigate to the root directory

# Central Unit
docker build -f ./centralunit/Dockerfile -t goncaloferreirauva/centralunit .
docker push goncaloferreirauva/centralunit:latest

# Compute node
docker build -f ./computenode/Dockerfile -t goncaloferreirauva/computenode .
docker push goncaloferreirauva/computenode:latest
