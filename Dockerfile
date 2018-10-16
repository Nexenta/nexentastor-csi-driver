FROM alpine:3.8

RUN apk update
RUN mkdir -p /config/

COPY ./bin/nexentastor-csi-driver /nexentastor-csi-driver

ENTRYPOINT ["/nexentastor-csi-driver"]
