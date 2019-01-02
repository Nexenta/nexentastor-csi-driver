#!/usr/bin/env bash

echo
echo -e "\n------------- CONTROLLER -------------\n"
echo
kubectl logs --all-containers=true nexentastor-csi-controller-0;

echo
echo -e "\n------------- NODE -------------\n"
echo
kubectl logs --all-containers=true $(kubectl get pods | awk '/nexentastor-csi-node/ {print $1;exit}');
