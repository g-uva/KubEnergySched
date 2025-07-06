#!/bin/bash

SCRIPT_DIR=$( cd -- "$( dirname -- "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )
cd $SCRIPT_DIR/../helm/

kubectl patch pv external-pv -p '{"metadata":{"finalizers": []}}' --type=merge
kubectl delete podmonitor compute-scaphandre -n eu-central
kubectl delete pv external-pv
helm uninstall eu-cluster -n eu-central
helm uninstall eu-monitoring -n eu-central

echo "eu-cluster removed successfully :)"