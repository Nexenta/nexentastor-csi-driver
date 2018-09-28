# nexentastor-csi-driver

NexentaStor CSI driver for Kubernetes.

## Build

```bash
make
```

## Tests

Run all tests:
```bash
go test ./tests/**             # run all
go test ./tests/** -v          # more output
go test ./tests/** -v -count 1 # disable cache
```

Run NexentaStor API tests with options:
```bash
go test ./tests/nexentastor -v -count 1 \
    --address="https://10.3.199.254:8443" \
    --username="admin" \
    --password="pass" \
    --pool="myPool" \
    --dataset="myDataset" \
    --filesystem="myFs" \
    --log="true"
```
