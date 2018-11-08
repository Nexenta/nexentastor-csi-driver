# build container
FROM golang:1.11 as builder
WORKDIR /go/src/github.com/Nexenta/nexentastor-csi-driver/
COPY . ./
RUN make build

# driver container
FROM alpine:3.8
LABEL name="nexentastor-csi-driver"
LABEL maintainer="Nexenta Systems, Inc."
LABEL description="NexentaStor CSI Driver"
LABEL io.k8s.description="NexentaStor CSI Driver"
RUN apk update
RUN mkdir -p /config/
COPY --from=builder /go/src/github.com/Nexenta/nexentastor-csi-driver/bin/nexentastor-csi-driver /nexentastor-csi-driver
ENTRYPOINT ["/nexentastor-csi-driver"]
