#!/bin/bash
# Navigate to helm directory.
./navigate_helm.sh
# Restart Helm release with `eu-cluster`.
helm upgrade eu-cluster . -n eu-central