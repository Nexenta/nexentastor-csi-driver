#!/usr/bin/env bash

#
# Combine all driver logs into single stdout
#

{ \
    kubectl logs -f --all-containers=true nexentastor-csi-controller-0 & \
    kubectl logs -f --all-containers=true $(kubectl get pods | awk '/nexentastor-csi-driver/ {print $1;exit}'); \
}
