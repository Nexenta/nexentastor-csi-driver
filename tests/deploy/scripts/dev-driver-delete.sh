#!/usr/bin/env bash

kubectl delete -f examples/kubernetes/nginx-dynamic-volume.yaml
kubectl delete -f examples/kubernetes/nginx-persistent-volume.yaml
kubectl delete -f tests/deploy/driver-local.yaml
