# nexentastor-csi-driver

[NexentaStor](https://nexenta.com/products/nexentastor) CSI driver for Kubernetes.

## Installation

1. Create default NexentaStor dataset for driver. Example: `csiDriverPool/csiDriverDataset`
2. Clone driver repository
   ```bash
   git clone https://github.com/Nexenta/nexentastor-csi-driver.git
   cd nexentastor-csi-driver
   ```
3. Edit `./kubernetes/nexentastor-csi-driver-config.yaml` file. Driver configuration example:
    ```yaml
    restIp: https://10.3.199.252:8443,https://10.3.199.253:8443 # [required] NexentaStor REST API endpoint(s)
    username: admin                                             # [required] NexentaStor REST API username
    password: p@ssword                                          # [required] NexentaStor REST API password
    defaultDataset: csiDriverPool/csiDriverDataset              # default parent dataset for creating fs/volume
    defaultDataIp: 20.20.20.252                                 # default NexentaStor data IP or HA VIP
    debug: true                                                 # more logs
    ```
4. Create Kubernetes secret from the file:
    ```bash
    kubectl create secret generic nexentastor-csi-driver-config --from-file=./kubernetes/nexentastor-csi-driver-config.yaml
    ```
5. Register driver to Kubernetes:
   ```bash
   kubectl apply -f ./kubernetes/nexentastor-csi-driver-1.0.0.yaml
   ```

## Usage examples

### Run nginx server using StorageClass (dynamic provisioning)

```bash
kubectl apply -f ./examples/nginx-storage-class.yaml

# to delete this pod:
kubectl delete -f ./examples/nginx-storage-class.yaml
```

### Run nginx server using PersistenVolume (pre provisioning)

Pre configured filesystem should exist on NexentaStor: `csiDriverPool/csiDriverDataset/nginx-persistent`

```bash
kubectl apply -f ./examples/nginx-pesistent-volume.yaml

# to delete this pod:
kubectl delete -f ./examples/nginx-pesistent-volume.yaml
```

### Overwrite default config in StorageClass definition

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nexentastor-csi-driver-dynamic-provisioning
provisioner: nexentastor-csi-driver
parameters:
  dataset: customPool/customDataset # to overwrite "defaultDataset" config property
  dataIp: 20.20.20.253              # to overwrite "defaultDataIp" config property
```

## Uninstall

Using the same files as for installation:
```bash
kubectl delete -f ./kubernetes/nexentastor-csi-driver-1.0.0.yaml
kubectl delete secret nexentastor-csi-driver-config
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
./bin/nexentastor-csi-driver --version
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
