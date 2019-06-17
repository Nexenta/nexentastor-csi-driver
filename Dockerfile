# build container
FROM golang:1.11 as builder
WORKDIR /go/src/github.com/Nexenta/nexentastor-csi-driver/
COPY . ./
ARG VERSION
ENV VERSION=$VERSION
RUN make build

# driver container
FROM alpine:3.8
LABEL name="nexentastor-csi-driver"
LABEL maintainer="Nexenta Systems, Inc."
LABEL description="NexentaStor CSI Driver"
LABEL io.k8s.description="NexentaStor CSI Driver"
# install nfs and smb dependencies
RUN apk add --no-cache rpcbind nfs-utils cifs-utils
# create driver config folder and print version
RUN mkdir -p /config/
COPY --from=builder /go/src/github.com/Nexenta/nexentastor-csi-driver/bin/nexentastor-csi-driver /nexentastor-csi-driver
RUN /nexentastor-csi-driver --version
# init script: runs rpcbind before starting the plugin
RUN echo $'#!/usr/bin/env sh\nrpcbind;\n/nexentastor-csi-driver "$@";\n' > /init.sh
RUN chmod +x /init.sh
ENTRYPOINT ["/init.sh"]
