#!/bin/bash

# Navigate to project root directory.
./navigate_proj_root.sh

# Central Unit
docker build -f ./centralunit/Dockerfile -t goncaloferreirauva/centralunit .
docker push goncaloferreirauva/centralunit:latest

# Compute node
docker build -f ./computenode/Dockerfile -t goncaloferreirauva/computenode .
docker push goncaloferreirauva/computenode:latest

# Benchmark
docker build -f ./benchmark/Dockerfile -t goncaloferreirauva/benchmark .
docker push goncaloferreirauva/benchmark:latest
