#!/usr/bin/env bash

#
# Combine all driver logs into single stdout
#

{ \
    kubectl logs -f nexentastor-csi-provisioner-0 driver &\
    kubectl logs -f nexentastor-csi-attacher-0 driver & \
    kubectl logs -f $(kubectl get pods | awk '/nexentastor-csi-driver/ {print $1;exit}') driver; \
}
