#!/usr/bin/env bash

{ \
    kubectl logs -f nexentastor-csi-provisioner-0 driver &\
    kubectl logs -f nexentastor-csi-attacher-0 driver & \
    kubectl logs -f $(kubectl get pods | awk '/nexentastor-csi-driver/ {print $1;exit}') driver; \
}
