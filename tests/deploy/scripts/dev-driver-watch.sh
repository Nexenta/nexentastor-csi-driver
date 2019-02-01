#!/usr/bin/env bash

watch -n 1 --diff "\
    kubectl get nodes -o wide && echo && \
    kubectl get secrets && echo && \
    kubectl get pods -o wide && echo && \
    kubectl get pv && echo && \
    kubectl get pvc && echo && \
    kubectl get volumesnapshots.snapshot.storage.k8s.io"
