#!/bin/bash
# Depending on where we're executing this script, we need it's "absolute" path.
SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR/../helm/
helm install eu-cluster . -n eu-central --create-namespace