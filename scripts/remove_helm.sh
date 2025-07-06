#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR/../helm/

helm uninstall eu-cluster -n eu-central
kubectl delete pv external-pv

echo "eu-cluster removed successfully :)"