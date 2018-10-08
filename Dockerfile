FROM alpine:3.8

RUN apk update
RUN mkdir -p /config/

COPY ./bin/nexentastor-csi-plugin /nexentastor-csi-plugin

ENTRYPOINT ["/nexentastor-csi-plugin"]
