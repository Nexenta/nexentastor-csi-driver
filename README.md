# nexentastor-csi-driver

NexentaStor CSI driver for Kubernetes.

[![Build Status](https://travis-ci.org/Nexenta/nexentastor-csi-driver.svg?branch=master)](https://travis-ci.org/Nexenta/nexentastor-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nexenta/nexentastor-csi-driver)](https://goreportcard.com/report/github.com/Nexenta/nexentastor-csi-driver)

- [Full driver documentation](https://nexenta.github.io/nexentastor-csi-driver/)
- [NexentaStor product page](https://nexenta.com/products/nexentastor)

## Supported versions

- NexentaStor 5.x
- Kubernetes 1.11


## Installation

1. Create NexentaStor dataset for the driver, example: `csiDriverPool/csiDriverDataset`.
    By default driver will create filesystems in this dataset, and share them to create Kubernetes volumes.
2. Clone driver repository
   ```bash
   git clone https://github.com/Nexenta/nexentastor-csi-driver.git
   cd nexentastor-csi-driver
   ```
3. Edit `./kubernetes/nexentastor-csi-driver-config.yaml` file. Driver configuration example:
    ```yaml
    restIp: https://10.3.3.4:8443,https://10.3.3.5:8443 # [required] NexentaStor REST API endpoint(s)
    username: admin                                     # [required] NexentaStor REST API username
    password: p@ssword                                  # [required] NexentaStor REST API password
    defaultDataset: csiDriverPool/csiDriverDataset      # default 'pool/dataset' for driver's filesystems
    defaultDataIp: 20.20.20.21                          # default NexentaStor data IP or HA VIP
    defaultNfsMountOptions: noatime                     # default NFS mount options (mount -o ...)
    ```
    All driver configuration options:

    | Name                     | Description                                                     | Required   | Example                                       |
    | ------------------------ | --------------------------------------------------------------- | ---------- | --------------------------------------------- |
    | `restIp`                 | NexentaStor REST API endpoint(s), `,` to separate cluster nodes | yes        | `https://10.3.3.4:8443,https://10.3.3.5:8443` |
    | `username`               | NexentaStor REST API username                                   | yes        | `admin`                                       |
    | `password`               | NexentaStor REST API password                                   | yes        | `p@ssword`                                    |
    | `defaultDataset`         | parent dataset for driver's filesystems [pool/dataset]          | no         | `csiDriverPool/csiDriverDataset`              |
    | `defaultDataIp`          | NexentaStor data IP or HA VIP for mounting NFS shares           | yes for PV | `20.20.20.21`                                 |
    | `defaultNfsMountOptions` | NFS mount options for: `mount -o ...` (default: "")             | no         | `noatime,nosuid`                              |
    | `debug`                  | print more logs (default: false)                                | no         | `true`                                        |

    **Note**: if parameter `defaultDataset` (`defaultDataIp`) is not specified in driver configuration,
    then parameter `dataset` (`dataIp`) must be specified in _StorageClass_ configuration.
4. Create Kubernetes secret from the file:
    ```bash
    kubectl create secret generic nexentastor-csi-driver-config --from-file=./kubernetes/nexentastor-csi-driver-config.yaml
    ```
5. Register driver to Kubernetes:
   ```bash
   kubectl apply -f ./kubernetes/nexentastor-csi-driver-1.0.0.yaml
   ```


## Usage

### Dynamically provisioned volumes

For dynamic volume provisioning, the administrator needs to set up a _StorageClass_ pointing to the driver.
Default driver parameters may be overwritten in `parameters` section:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nexentastor-csi-driver-dynamic-provisioning
provisioner: nexentastor-csi-driver
parameters:
  #dataset: customPool/customDataset # to overwrite "defaultDataset" config property [pool/dataset]
  #dataIp: 20.20.20.253              # to overwrite "defaultDataIp" config property
  #nfsMountOptions: noatime          # to overwrite "defaultNfsMountOptions" in config secret
```

#### Parameters

| Name              | Description                                            | Example                    |
| ----------------- | -------------------------------------------------------| -------------------------- |
| `dataset`         | parent dataset for driver's filesystems [pool/dataset] | `customPool/customDataset` |
| `dataIp`          | NexentaStor data IP or HA VIP for mounting NFS shares  | `20.20.20.253`             |
| `nfsMountOptions` | nfs mount options (`mount -o ...`)                     | `noatime`                  |

#### Example

Run Nginx server using _StorageClass_:

```bash
kubectl apply -f ./examples/nginx-storage-class.yaml

# to delete this pod:
kubectl delete -f ./examples/nginx-storage-class.yaml
```

### Pre-provisioned volumes

The driver can use already existing NexentaStor filesystem,
in this case, _PersistentVolume_ and _PersistentVolumeClaim_ should be configured.

#### _PersistentVolume_ configuration

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nexentastor-csi-driver-nginx-pv
  labels:
    name: nexentastor-csi-driver-nginx-pv
spec:
  accessModes:
  - ReadWriteMany
  capacity:
    storage: 1Gi
  csi:
    driver: nexentastor-csi-driver
    volumeHandle: csiDriverPool/csiDriverDataset/nginx-persistent
```

CSI Parameters:

| Name           | Description                                                       | Example                  |
| -------------- | ----------------------------------------------------------------- | ------------------------ |
| `driver`       | installed driver name "nexentastor-csi-driver"                    | `nexentastor-csi-driver` |
| `volumeHandle` | path to existing NexentaStor filesystem [pool/dataset/filesystem] | `PoolA/datasetA/nginx`   |

#### _PersistentVolumeClaim_ (pointed to created _PersistentVolume_)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nexentastor-csi-driver-nginx-pvc
spec:
  accessModes:
  - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  selector:
    matchExpressions:
    - key: name
      operator: In
      values: ["nexentastor-csi-driver-nginx-pv"]
```

#### Example

Run nginx server using PersistentVolume.

**Note:** Pre-configured filesystem should exist on the NexentaStor:
`csiDriverPool/csiDriverDataset/nginx-persistent`.

```bash
kubectl apply -f ./examples/nginx-persistent-volume.yaml

# to delete this pod:
kubectl delete -f ./examples/nginx-persistent-volume.yaml
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
# build locally
make

# build container (+ using build container)
make container-build

# update deps
~/go/bin/dep ensure
```

### Run

Without installation to k8s cluster only version command works:
```bash
./bin/nexentastor-csi-driver --version
```

### Publish

```bash
# push the latest built container to local registry (see `Makefile`)
make container-push-local

# push the latest built container to hub.docker.com
make container-push-remote
```

### Tests

```bash
# run all tests using local registry (`REGISTRY_LOCAL` in `Makefile`)
make test-local
# run all tests using hub.docker.com registry (`REGISTRY` in `Makefile`)
make test-remote

# run tests in container:
# - RSA keys from host's ~/.ssh directory will be used by container.
#   Make sure all remote hosts used in tests have host's RSA key added as trusted
#   (ssh-copy-id -i ~/.ssh/id_rsa.pub user@host)
# - "export NOCOLORS=true" to run w/o colors
#
# for local image
make container-test-local
# for remote image from hub.docker.com
make container-test-remote

# add red for fails
make test | grep --color 'FAIL\|$'

# Unit tests with options
go test ./tests/unit/rest -v -count 1
go test ./tests/unit/config -v -count 1

# Tests check NexentaStor API provider (same options for `./resolver_test.go`)
go test ./tests/e2e/ns/provider_test.go -v -count 1 \
    --address="https://10.3.199.254:8443" \
    --username="admin" \
    --password="pass" \
    --pool="myPool" \
    --dataset="myDataset" \
    --filesystem="myFs" \
    --log="true"

# Tests install driver to k8s and run nginx pod with mounted volume
# "export NOCOLORS=true" to run w/o colors
go test tests/e2e/driver/driver_test.go -v -count 1 \
    --k8sConnectionString="root@10.3.199.250" \
    --k8sDeploymentFile="./_configs/driver-master-local.yaml" \
    --k8sSecretFile="./_configs/driver-config-single.yaml"
```

See `Makefile` for more examples.

## Troubleshooting

- Configure Docker to trust insecure registries:
  ```bash
  echo "{\"insecure-registries\":[\"10.3.199.92:5000\"]}" > /etc/docker/daemon.json
  service docker restart
  ```
