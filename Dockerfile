FROM quay.io/prometheus/busybox:latest

COPY ./bin/ethereum-exporter /bin/ethereum-exporter

ENTRYPOINT ["/bin/ethereum-exporter"]
