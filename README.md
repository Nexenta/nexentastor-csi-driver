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

## Uninstall

Using the same files as for installation:
```bash
kubectl delete -f ./kubernetes
```

## Examples

### Run nginx server with PersistentVolumeClaim

```bash
kubectl apply -f ./examples/nginx-dynamic.yaml

# to delete this pod:
kubectl delete -f ./examples/nginx-dynamic.yaml
```

## Development

### Build

```bash
~/go/bin/dep ensure
make
```

### Run

Without installation to k8s cluster only version command works:
```bash
./bin/nexentastor-csi-plugin --version
```

### Tests

```bash
# run all tests
make test
# or
make test | grep --color 'FAIL\|$'

# with options
go test ./tests/unit/rest -v -count 1
go test ./tests/unit/config -v -count 1
go test ./tests/e2e/ns_provider -v -count 1 --address="https://10.3.199.254:8443"
go test ./tests/e2e/ns_resolver -v -count 1 --address="https://10.3.199.254:8443"
go test ./tests/e2e/ns_resolver -v -count 1 --address="https://10.3.199.252:8443,https://10.3.199.253:8443"

# NexentaStor provider test options
go test ./tests/e2e/ns_provider -v -count 1 \
    --address="https://10.3.199.254:8443" \
    --username="admin" \
    --password="pass" \
    --pool="myPool" \
    --dataset="myDataset" \
    --filesystem="myFs" \
    --log="true"
```

See `Makefile` for more examples.
