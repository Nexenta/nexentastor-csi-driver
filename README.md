# nexentastor-csi-driver (v1.1.0)

[![Build Status](https://travis-ci.org/Nexenta/nexentastor-csi-driver.svg?branch=master)](https://travis-ci.org/Nexenta/nexentastor-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nexenta/nexentastor-csi-driver)](https://goreportcard.com/report/github.com/Nexenta/nexentastor-csi-driver)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)

NexentaStor product page: [https://nexenta.com/products/nexentastor](https://nexenta.com/products/nexentastor).

## Supported versions

- Kubernetes 1.13

For other supported versions see [this page](https://github.com/Nexenta/nexentastor-csi-driver#supported-versions).

## Features

- Persistence (beyond pod lifetime)
- Dynamic provisioning
- Supported access mode: read/write multiple pods
- NFS/SMB mount protocols.

## Requirements

- Kubernetes cluster must allow privileged pods, this flag must be set for the API server and the kubelet
  ([instructions](https://github.com/kubernetes-csi/docs/blob/735f1ef4adfcb157afce47c64d750b71012c8151/book/src/Setup.md#enable-privileged-pods)):
  ```
  --allow-privileged=true
  ```
- Mount propagation must be enabled, the Docker daemon for the cluster must allow shared mounts
  ([instructions](https://github.com/kubernetes-csi/docs/blob/735f1ef4adfcb157afce47c64d750b71012c8151/book/src/Setup.md#enabling-mount-propagation))
- Kubernetes CSI drivers require `CSIDriver` and `CSINodeInfo` resource types
  [to be defined on the cluster](https://github.com/kubernetes-csi/docs/blob/460a49286fe164a78fde3114e893c48b572a36c8/book/src/Setup.md#csidriver-custom-resource-alpha).
  Check if they are already defined:
  ```bash
  kubectl get customresourcedefinition.apiextensions.k8s.io/csidrivers.csi.storage.k8s.io
  kubectl get customresourcedefinition.apiextensions.k8s.io/csinodeinfos.csi.storage.k8s.io
  ```
  If the cluster doesn't have "csidrivers" and "csinodeinfos" resource types, create them:
  ```bash
  kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/release-1.13/pkg/crd/manifests/csidriver.yaml
  kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/release-1.13/pkg/crd/manifests/csinodeinfo.yaml
  ```
- Depends on preferred mount filesystem type, following utilities must be installed on each Kubernetes node:
  ```bash
  # for NFS
  apt install -y rpcbind nfs-common
  # for SMB
  apt install -y rpcbind cifs-utils
  ```

## Installation

1. Create NexentaStor dataset for the driver, example: `csiDriverPool/csiDriverDataset`.
   By default, the driver will create filesystems in this dataset and mount them to use as Kubernetes volumes.
2. Clone driver repository
   ```bash
   git clone https://github.com/Nexenta/nexentastor-csi-driver.git
   cd nexentastor-csi-driver
   #git branch       # to use other than master branch
   #git checkout ...
   ```
3. Edit `deploy/kubernetes/nexentastor-csi-driver-config.yaml` file. Driver configuration example:
   ```yaml
   restIp: https://10.3.3.4:8443,https://10.3.3.5:8443     # [required] NexentaStor REST API endpoint(s)
   username: admin                                         # [required] NexentaStor REST API username
   password: p@ssword                                      # [required] NexentaStor REST API password
   defaultDataset: csiDriverPool/csiDriverDataset          # default 'pool/dataset' to use
   defaultDataIp: 20.20.20.21                              # default NexentaStor data IP or HA VIP
   defaultMountFsType: nfs                                 # default mount fs type [nfs|cifs]
   defaultMountOptions: noatime                            # default mount options (mount -o ...)

   # for CIFS mounts:
   #defaultMountFsType: cifs                               # default mount fs type [nfs|cifs]
   #defaultMountOptions: username=admin,password=Nexenta@1 # username/password must be defined for CIFS
   ```

   All driver configuration options:

   | Name                  | Description                                                     | Required   | Example                                                      |
   |-----------------------|-----------------------------------------------------------------|------------|--------------------------------------------------------------|
   | `restIp`              | NexentaStor REST API endpoint(s); `,` to separate cluster nodes | yes        | `https://10.3.3.4:8443`                                      |
   | `username`            | NexentaStor REST API username                                   | yes        | `admin`                                                      |
   | `password`            | NexentaStor REST API password                                   | yes        | `p@ssword`                                                   |
   | `defaultDataset`      | parent dataset for driver's filesystems [pool/dataset]          | no         | `csiDriverPool/csiDriverDataset`                             |
   | `defaultDataIp`       | NexentaStor data IP or HA VIP for mounting shares               | yes for PV | `20.20.20.21`                                                |
   | `defaultMountFsType`  | mount filesystem type [nfs, cifs](default: 'nfs')               | no         | `cifs`                                                       |
   | `defaultMountOptions` | NFS/CIFS mount options: `mount -o ...` (default: "")            | no         | NFS: `noatime,nosuid`<br>CIFS: `username=admin,password=123` |
   | `debug`               | print more logs (default: false)                                | no         | `true`                                                       |

   **Note**: if parameter `defaultDataset`/`defaultDataIp` is not specified in driver configuration,
   then parameter `dataset`/`dataIp` must be specified in _StorageClass_ configuration.

   **Note**: all default parameters (`default*`) may be overwritten in specific _StorageClass_ configuration.

   **Note**: if `defaultMountFsType` is set to `cifs` then parameter `defaultMountOptions` must include
   CIFS username and password (`username=admin,password=123`).

4. Create Kubernetes secret from the file:
   ```bash
   kubectl create secret generic nexentastor-csi-driver-config --from-file=deploy/kubernetes/nexentastor-csi-driver-config.yaml
   ```
5. Register driver to Kubernetes:
   ```bash
   kubectl apply -f deploy/kubernetes/nexentastor-csi-driver.yaml
   ```

NexentaStor CSI driver's pods should be running after installation:

```bash
$ kubectl get pods
NAME                           READY   STATUS    RESTARTS   AGE
nexentastor-csi-controller-0   3/3     Running   0          42s
nexentastor-csi-node-cwp4v     2/2     Running   0          42s
```

## Usage

### Dynamically provisioned volumes

For dynamic volume provisioning, the administrator needs to set up a _StorageClass_ pointing to the driver.
In this case Kubernetes generates volume name automatically (for example `pvc-ns-cfc67950-fe3c-11e8-a3ca-005056b857f8`).
Default driver configuration may be overwritten in `parameters` section:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nexentastor-csi-driver-cs-nginx-dynamic
provisioner: nexentastor-csi-driver.nexenta.com
mountOptions:                        # list of options for `mount -o ...` command
#  - noatime                         #
parameters:
  #dataset: customPool/customDataset # to overwrite "defaultDataset" config property [pool/dataset]
  #dataIp: 20.20.20.253              # to overwrite "defaultDataIp" config property
  #mountFsType: nfs                  # to overwrite "defaultMountFsType" config property
  #mountOptions: noatime             # to overwrite "defaultMountOptions" config property
```

#### Parameters

| Name           | Description                                            | Example                                               |
|----------------|--------------------------------------------------------|-------------------------------------------------------|
| `dataset`      | parent dataset for driver's filesystems [pool/dataset] | `customPool/customDataset`                            |
| `dataIp`       | NexentaStor data IP or HA VIP for mounting shares      | `20.20.20.253`                                        |
| `mountFsType`  | mount filesystem type [nfs, cifs](default: 'nfs')      | `cifs`                                                |
| `mountOptions` | NFS/CIFS mount options: `mount -o ...`                 | NFS: `noatime`<br>CIFS: `username=admin,password=123` |

#### Example

Run Nginx pod with dynamically provisioned volume:

```bash
kubectl apply -f examples/kubernetes/nginx-dynamic-volume.yaml

# to delete this pod:
kubectl delete -f examples/kubernetes/nginx-dynamic-volume.yaml
```

### Pre-provisioned volumes

The driver can use already existing NexentaStor filesystem,
in this case, _StorageClass_, _PersistentVolume_ and _PersistentVolumeClaim_ should be configured.

#### _StorageClass_ configuration

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nexentastor-csi-driver-cs-nginx-persistent
provisioner: nexentastor-csi-driver.nexenta.com
mountOptions:                        # list of options for `mount -o ...` command
#  - noatime                         #
parameters:
  #dataset: customPool/customDataset # to overwrite "defaultDataset" config property [pool/dataset]
  #dataIp: 20.20.20.253              # to overwrite "defaultDataIp" config property
  #mountFsType: nfs                  # to overwrite "defaultMountFsType" config property
  #mountOptions: noatime             # to overwrite "defaultMountOptions" config property
```

#### _PersistentVolume_ configuration

```yaml
apiVersion: v1
kind: PersistentVolume
metadata:
  name: nexentastor-csi-driver-pv-nginx-persistent
  labels:
    name: nexentastor-csi-driver-pv-nginx-persistent
spec:
  storageClassName: nexentastor-csi-driver-cs-nginx-persistent
  accessModes:
    - ReadWriteMany
  capacity:
    storage: 1Gi
  csi:
    driver: nexentastor-csi-driver.nexenta.com
    volumeHandle: csiDriverPool/csiDriverDataset/nginx-persistent
  #mountOptions:  # list of options for `mount` command
  #  - noatime    #
```

CSI Parameters:

| Name           | Description                                                       | Example                              |
|----------------|-------------------------------------------------------------------|--------------------------------------|
| `driver`       | installed driver name "nexentastor-csi-driver.nexenta.com"        | `nexentastor-csi-driver.nexenta.com` |
| `volumeHandle` | path to existing NexentaStor filesystem [pool/dataset/filesystem] | `PoolA/datasetA/nginx`               |

#### _PersistentVolumeClaim_ (pointed to created _PersistentVolume_)

```yaml
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: nexentastor-csi-driver-pvc-nginx-persistent
spec:
  storageClassName: nexentastor-csi-driver-cs-nginx-persistent
  accessModes:
    - ReadWriteMany
  resources:
    requests:
      storage: 1Gi
  selector:
    matchLabels:
      # to create 1-1 relationship for pod - persistent volume use unique labels
      name: nexentastor-csi-driver-pv-nginx-persistent
```

#### Example

Run nginx server using PersistentVolume.

**Note:** Pre-configured filesystem should exist on the NexentaStor:
`csiDriverPool/csiDriverDataset/nginx-persistent`.

```bash
kubectl apply -f examples/kubernetes/nginx-persistent-volume.yaml

# to delete this pod:
kubectl delete -f examples/kubernetes/nginx-persistent-volume.yaml
```

## Uninstall

Using the same files as for installation:

```bash
# delete driver
kubectl delete -f deploy/kubernetes/nexentastor-csi-driver.yaml

# delete secret
kubectl delete secret nexentastor-csi-driver-config
```

## Troubleshooting

- Show installed drivers:
  ```bash
  kubectl get csidrivers.csi.storage.k8s.io
  kubectl describe csidrivers.csi.storage.k8s.io
  ```
- Error:
  ```
  MountVolume.MountDevice failed for volume "pvc-ns-<...>" :
  driver name nexentastor-csi-driver.nexenta.com not found in the list of registered CSI drivers
  ```
  Make sure _kubelet_ configured with `--root-dir=/var/lib/kubelet`, otherwise update paths in the driver yaml file
  ([all requirements](https://github.com/kubernetes-csi/docs/blob/387dce893e59c1fcf3f4192cbea254440b6f0f07/book/src/Setup.md#enabling-features)).
- "VolumeSnapshotDataSource" feature gate is disabled:
  ```bash
  vim /var/lib/kubelet/config.yaml
  # ```
  # featureGates:
  #   VolumeSnapshotDataSource: true
  # ```
  vim /etc/kubernetes/manifests/kube-apiserver.yaml
  # ```
  #     - --feature-gates=VolumeSnapshotDataSource=true
  # ```
  ```
- Driver logs
  ```bash
  kubectl logs -f nexentastor-csi-controller-0 driver
  kubectl logs -f $(kubectl get pods | awk '/nexentastor-csi-node/ {print $1;exit}') driver
  ```
- Show termination message in case driver failed to run:
  ```bash
  kubectl get pod nexentastor-csi-controller-0 -o go-template="{{range .status.containerStatuses}}{{.lastState.terminated.message}}{{end}}"
  ```
- Configure Docker to trust insecure registries:
  ```bash
  # add `{"insecure-registries":["10.3.199.92:5000"]}` to:
  vim /etc/docker/daemon.json
  service docker restart
  ```

## Development

Commits should follow [Conventional Commits Spec](https://conventionalcommits.org).
Commit messages which include `feat:` and `fix:` prefixes will be included in CHANGELOG automatically.

### Build

```bash
# print variables and help
make

# build go app on local machine
make build

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
# push the latest built container to the local registry (see `Makefile`)
make container-push-local

# push the latest built container to hub.docker.com
make container-push-remote
```

### Tests

`test-all-*` instructions run:
- unit tests
- CSI sanity tests from https://github.com/kubernetes-csi/csi-test
- End-to-end driver tests with real K8s and NS appliances.

See [Makefile](Makefile) for more examples.

```bash
# Test options to be set before run tests:
# - NOCOLORS=true            # to run w/o colors
# - TEST_K8S_IP=10.3.199.250 # e2e k8s tests

# run all tests using local registry (`REGISTRY_LOCAL` in `Makefile`)
TEST_K8S_IP=10.3.199.250 make test-all-local-image
# run all tests using hub.docker.com registry (`REGISTRY` in `Makefile`)
TEST_K8S_IP=10.3.199.250 make test-all-remote-image

# run tests in container:
# - RSA keys from host's ~/.ssh directory will be used by container.
#   Make sure all remote hosts used in tests have host's RSA key added as trusted
#   (ssh-copy-id -i ~/.ssh/id_rsa.pub user@host)
#
# run all tests using local registry (`REGISTRY_LOCAL` in `Makefile`)
TEST_K8S_IP=10.3.199.250 make test-all-local-image-container
# run all tests using hub.docker.com registry (`REGISTRY` in `Makefile`)
TEST_K8S_IP=10.3.199.250 make test-all-remote-image-container
```

End-to-end K8s test parameters:

```bash
# Tests install driver to k8s and run nginx pod with mounted volume
# "export NOCOLORS=true" to run w/o colors
go test tests/e2e/driver_test.go -v -count 1 \
    --k8sConnectionString="root@10.3.199.250" \
    --k8sDeploymentFile="../../deploy/kubernetes/nexentastor-csi-driver.yaml" \
    --k8sSecretFile="./_configs/driver-config-single-default.yaml"
```

All development happens in `master` branch,
when it's time to publish a new version,
new git tag should be created.

1. Build and test the new version using local registry:
   ```bash
   # build development version:
   make container-build
   # publish to local registry
   make container-push-local
   # test plugin using local registry
   TEST_K8S_IP=10.3.199.250 make test-all-local-image-container
   ```

2. To release a new version run command:
   ```bash
   VERSION=X.X.X make release
   ```
   This script does following:
   - generates new `CHANGELOG.md`
   - builds driver container 'nexentastor-csi-driver'
   - Login to hub.docker.com will be requested
   - publishes driver version 'nexenta/nexentastor-csi-driver:X.X.X' to hub.docker.com
   - creates new Git tag 'X.X.X' and pushes to the repository.

3. Update Github [releases](https://github.com/Nexenta/nexentastor-csi-driver/releases).
