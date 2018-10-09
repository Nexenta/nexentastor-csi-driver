# nexentastor-csi-driver

NexentaStor CSI driver for Kubernetes.

## Installation

1. Clone driver repository
   ```bash
   git clone https://github.com/Nexenta/nexentastor-csi-driver.git
   cd nexentastor-csi-driver
   ```
2. Create Kubernetes secret
   ```bash
   # edit `./kubernetes/nexentastor-csi-driver-config.yaml` file
   kubectl create secret generic nexentastor-csi-driver-config --from-file=./kubernetes/secret/nexentastor-csi-driver-config.yaml
   ```
3. Register plugin to Kubernetes
   ```bash
   kubectl apply -f ./kubernetes
   ```

## Examples

### Run nginx server with PersistentVolumeClaim

```bash
kubectl apply -f ./examples/nginx-dynamic.yaml
```

## Development

### Build

```bash
~/go/bin/dep ensure
make
```

### Run

```bash
make && ./bin/nexentastor-csi-plugin --rest-ip="https://10.3.199.253:8443,https://10.3.199.252:8443" --username="admin" --password="Nexenta@1"
```

### Tests

```bash
# run all tests
make test
# or
make test | grep --color 'FAIL\|$'

# with options
go test ./tests/**             # run all
go test ./tests/** -v          # more output
go test ./tests/** -v -count 1 # disable cache

# NexentaStor provider test options
go test ./tests/ns_provider -v -count 1 \
    --address="https://10.3.199.254:8443" \
    --username="admin" \
    --password="pass" \
    --pool="myPool" \
    --dataset="myDataset" \
    --filesystem="myFs" \
    --log="true"
```
