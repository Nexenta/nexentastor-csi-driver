# nexentastor-csi-driver

[![Build Status](https://travis-ci.org/Nexenta/nexentastor-csi-driver.svg?branch=master)](https://travis-ci.org/Nexenta/nexentastor-csi-driver)
[![Go Report Card](https://goreportcard.com/badge/github.com/Nexenta/nexentastor-csi-driver)](https://goreportcard.com/report/github.com/Nexenta/nexentastor-csi-driver)
[![Conventional Commits](https://img.shields.io/badge/Conventional%20Commits-1.0.0-yellow.svg)](https://conventionalcommits.org)

This is a **development branch**, for the most recent stable version see
[documentation](https://nexenta.github.io/nexentastor-csi-driver/).

NexentaStor product page: [https://nexenta.com/products/nexentastor](https://nexenta.com/products/nexentastor).

## Supported versions

|                             | NexentaStor 5.1                                                       | NexentaStor 5.2                                                       |
|-----------------------------|-----------------------------------------------------------------------|-----------------------------------------------------------------------|
| Kubernetes >=1.10.5 <1.12.1 | [0.1.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/0.1.0) | [0.1.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/0.1.0) |
| Kubernetes >=1.12.1 <1.13   | [0.2.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/0.2.0) | [0.2.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/0.2.0) |
| Kubernetes >=1.13           | [1.0.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/1.0.0) | [1.0.0](https://github.com/Nexenta/nexentastor-csi-driver/tree/1.0.0) |
| Kubernetes >=1.13           | master                                                                | master                                                                |

## Requirements

- [This page explains](https://github.com/kubernetes-csi/docs/blob/387dce893e59c1fcf3f4192cbea254440b6f0f07/book/src/Setup.md)
  how to configure Kubernetes for CSI drivers
- Depends on preferred mount filesystem type, following utilities must be installed on each Kubernetes node:
  ```bash
  # for NFS
  apt install -y nfs-common rpcbind
  # for SMB
  apt install -y cifs-utils rpcbind
  ```
- Kubernetes CSI drivers require `CSIDriver` and `CSINodeInfo` resource types
  [to be defined on the cluster](https://github.com/kubernetes-csi/docs/blob/460a49286fe164a78fde3114e893c48b572a36c8/book/src/Setup.md#csidriver-custom-resource-alpha).
  Check if they are already defined:
  ```bash
  kubectl get customresourcedefinition.apiextensions.k8s.io/csidrivers.csi.storage.k8s.io
  kubectl get customresourcedefinition.apiextensions.k8s.io/csinodeinfos.csi.storage.k8s.io
  ```
  If the cluster doesn't have "csidrivers" and "csinodeinfos" resource types, create them:
  ```bash
  kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/ce972859c46136a1f4a9fe119d05482a739c6311/pkg/crd/manifests/csidriver.yaml
  kubectl create -f https://raw.githubusercontent.com/kubernetes/csi-api/ce972859c46136a1f4a9fe119d05482a739c6311/pkg/crd/manifests/csinodeinfo.yaml
  ```

## Installation

1. Create NexentaStor dataset for the driver, example: `csiDriverPool/csiDriverDataset`.
   By default driver will create filesystems in this dataset, and share them in order to create Kubernetes volumes.
2. Clone driver repository
   ```bash
   git clone https://github.com/Nexenta/nexentastor-csi-driver.git
   cd nexentastor-csi-driver
   #git branch       # to use other that master branch
   #git checkout ...
   ```
3. Edit `./deploy/kubernetes/nexentastor-csi-driver-config.yaml` file. Driver configuration example:
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
   kubectl create secret generic nexentastor-csi-driver-config --from-file=./deploy/kubernetes/nexentastor-csi-driver-config.yaml
   ```
5. Register driver to Kubernetes:
   ```bash
   kubectl apply -f ./deploy/kubernetes/nexentastor-csi-driver.yaml
   ```

## Usage

### Dynamically provisioned volumes

For dynamic volume provisioning, the administrator needs to set up a _StorageClass_ pointing to the driver.
Default driver configuration may be overwritten in `parameters` section:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: nexentastor-csi-driver-dynamic-provisioning
provisioner: nexentastor-csi-driver.nexenta.com
mountOptions:                        # list of options for `mount -o ...` command
  - noatime                          #
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

Run Nginx server using _StorageClass_:

```bash
kubectl apply -f ./deploy/kubernetes/examples/nginx-storage-class.yaml

# to delete this pod:
kubectl delete -f ./deploy/kubernetes/examples/nginx-storage-class.yaml
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
    driver: nexentastor-csi-driver.nexenta.com
    volumeHandle: csiDriverPool/csiDriverDataset/nginx-persistent
  mountOptions: # list of options for `mount -o ...` command
    - noatime   #
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
        values: ['nexentastor-csi-driver-nginx-pv']
```

#### Example

Run nginx server using PersistentVolume.

**Note:** Pre-configured filesystem should exist on the NexentaStor:
`csiDriverPool/csiDriverDataset/nginx-persistent`.

```bash
kubectl apply -f ./deploy/kubernetes/examples/nginx-persistent-volume.yaml

# to delete this pod:
kubectl delete -f ./deploy/kubernetes/examples/nginx-persistent-volume.yaml
```

## Uninstall

Using the same files as for installation:

```bash
# delete driver
kubectl delete -f ./deploy/kubernetes/nexentastor-csi-driver.yaml

# delete secret
kubectl delete secret nexentastor-csi-driver-config
```

## Development

Commits should follow [Conventional Commits Spec](https://conventionalcommits.org).

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

See [Makefile](Makefile) for more examples.

```bash
# run all tests using local registry (`REGISTRY_LOCAL` in `Makefile`)
make test-all-local-image
# run all tests using hub.docker.com registry (`REGISTRY` in `Makefile`)
make test-all-remote-image

# run tests in container:
# - RSA keys from host's ~/.ssh directory will be used by container.
#   Make sure all remote hosts used in tests have host's RSA key added as trusted
#   (ssh-copy-id -i ~/.ssh/id_rsa.pub user@host)
# - "export NOCOLORS=true" to run w/o colors
#
# for local image
make test-all-local-image-container
# for remote image from hub.docker.com
make test-all-remote-image-container
```

End-to-end NexentaStor/K8s test parameters:
```bash
# Tests for NexentaStor API provider (same options for `./resolver/resolver_test.go`)
go test ./tests/e2e/ns/provider/provider_test.go -v -count 1 \
    --address="https://10.3.199.254:8443" \
    --username="admin" \
    --password="pass" \
    --pool="myPool" \
    --dataset="myDataset" \
    --filesystem="myFs" \
    --cluster=true \
    --log=true

# Tests install driver to k8s and run nginx pod with mounted volume
# "export NOCOLORS=true" to run w/o colors
go test tests/e2e/driver/driver_test.go -v -count 1 \
    --k8sConnectionString="root@10.3.199.250" \
    --k8sDeploymentFile="./_configs/driver-local.yaml" \
    --k8sSecretFile="./_configs/driver-config-single.yaml"
```

### Release

```bash
# go get -u github.com/git-chglog/git-chglog/cmd/git-chglog
git-chglog --next-tag X.X.X -o CHANGELOG.md
git add CHANGELOG.md
git commit -m "release X.X.X"
git push
git co -b X.X.X
make container-build && make container-push-local && make test-all-local-image-container && make container-push-remote
vim README.md
git add README.md
git ci -m "release X.X.X"
git push
git tag vX.X.X
git push --tags
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
- Driver logs
  ```bash
  kubectl logs -f nexentastor-csi-attacher-0 driver
  kubectl logs -f nexentastor-csi-provisioner-0 driver
  kubectl logs -f $(kubectl get pods | awk '/nexentastor-csi-driver/ {print $1;exit}') driver
  # combine all pods:
  kubectl get pods | awk '/nexentastor-csi-/ {system("kubectl logs " $1 " driver &")}'
  ```
- Show termination message in case driver failed to run:
  ```bash
  kubectl get pod nexentastor-csi-attacher-0 -o go-template="{{range .status.containerStatuses}}{{.lastState.terminated.message}}{{end}}"
  ```
- Configure Docker to trust insecure registries:
  ```bash
  # add `{"insecure-registries":["10.3.199.92:5000"]}` to:
  vim /etc/docker/daemon.json
  service docker restart
  ```
