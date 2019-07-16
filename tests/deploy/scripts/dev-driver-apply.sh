#!/usr/bin/env bash

kubectl create secret generic nexentastor-csi-driver-config \
    --from-file=deploy/kubernetes/nexentastor-csi-driver-config.yaml
kubectl apply -f tests/deploy/driver-local.yaml
sleep 10
kubectl apply -f examples/kubernetes/nginx-persistent-volume.yaml
#sleep 10
#kubectl apply -f examples/kubernetes/nginx-dynamic-volume.yaml
sleep 10
kubectl apply -f examples/kubernetes/snapshot-class.yaml
kubectl apply -f examples/kubernetes/take-snapshot.yaml
sleep 10
kubectl apply -f examples/kubernetes/nginx-snapshot-volume.yaml
