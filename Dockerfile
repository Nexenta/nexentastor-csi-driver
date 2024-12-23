ARG BASE_IMAGE=alpine:3.20
ARG BUILD_IMAGE=golang:1.23.1-alpine3.20

# build container
FROM $BUILD_IMAGE AS builder
WORKDIR /go/src/github.com/Nexenta/nexentastor-csi-driver/
COPY . ./
ARG VERSION
ENV VERSION=$VERSION
RUN apk add --no-cache make git
RUN go version
RUN make build &&\
    cp ./bin/nexentastor-csi-driver /

# driver container
FROM $BASE_IMAGE
LABEL name="nexentastor-csi-driver"
LABEL maintainer="Nexenta Systems, Inc."
LABEL description="NexentaStor CSI Driver"
LABEL io.k8s.description="NexentaStor CSI Driver"
# install nfs and smb dependencies
RUN apk add --no-cache rpcbind nfs-utils cifs-utils ca-certificates
RUN apk update && apk add "libcrypto3>=3.3.2-r1" "libssl3>=3.3.2-r1" && rm -rf /var/cache/apt/*
# create driver config folder and print version
RUN mkdir -p /config/
COPY --from=builder /nexentastor-csi-driver /
RUN /nexentastor-csi-driver --version
# init script: runs rpcbind before starting the plugin
RUN echo $'#!/usr/bin/env sh\nupdate-ca-certificates\nrpcbind;\n/nexentastor-csi-driver "$@";\n' > /init.sh
RUN chmod +x /init.sh
ENTRYPOINT ["/init.sh"]
