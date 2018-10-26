# build container
FROM golang:1.11 as builder
WORKDIR /go/src/github.com/Nexenta/nexentastor-csi-driver/
COPY . ./
RUN make build

# driver container
FROM alpine:3.8
RUN apk update
RUN mkdir -p /config/
COPY --from=builder /go/src/github.com/Nexenta/nexentastor-csi-driver/bin/nexentastor-csi-driver /nexentastor-csi-driver
ENTRYPOINT ["/nexentastor-csi-driver"]
