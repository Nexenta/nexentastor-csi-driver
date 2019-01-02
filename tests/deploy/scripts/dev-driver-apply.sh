#!/usr/bin/env bash

kubectl apply -f tests/deploy/driver-local-manual.yaml
sleep 10
kubectl apply -f examples/kubernetes/nginx-persistent-volume.yaml
sleep 10
kubectl apply -f examples/kubernetes/snapshot-class.yaml
kubectl apply -f examples/kubernetes/take-snapshot.yaml
