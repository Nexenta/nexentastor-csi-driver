
<a name="v1.3.0"></a>
## [v1.3.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/v1.2.0...v1.3.0) (2020-03-31)

### Bug Fixes

* changed err.Code when snapshot not found for sanity tests
* added protobuf dep and updated csi
* added kubernetes-csi dep to /vendor

### Features

* implemented volume expand fix: fixed volume cloning
* NEX-20687 - use k8s secrets to support multi-NS configurations

### Pull Requests

* Merge pull request [#5](https://github.com/Nexenta/nexentastor-csi-driver/issues/5) from Nexenta/qa/test
* Merge pull request [#4](https://github.com/Nexenta/nexentastor-csi-driver/issues/4) from Nexenta/module
* Merge pull request [#1](https://github.com/Nexenta/nexentastor-csi-driver/issues/1) from Nexenta/eugenei-qa-branch


<a name="v1.2.0"></a>
## [v1.2.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/v1.1.0...v1.2.0) (2019-09-11)

### Bug Fixes

* do not compile regexp's on each request
* driver could fail if no log object provided

### Features

* update csi-sanity tests to v2.1.0
* NEX-21217 - add volume pagination support
* NEX-21231 - volume snapshot feature
* driver configuration for k8s >=1.14


<a name="v1.1.0"></a>
## [v1.1.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/v1.0.1...v1.1.0) (2019-07-18)

### Bug Fixes

* return NotFound error code if source snapshot for a new volume is not found
* NEX-20939 - disable snapshot list capability for current version
* NEX-20161 - rpcbind is not running in container, suse doesn't mount vers=3
* Makefile to check env variables existence
* license and rsf cluster checks log only a warning for now
* NEX-19118 - mountFsType parameter doesn't work in storage class config
* NEX-18844 - node server - return right error code
* NEX-18885 - update tests/production driver configs
* use http.Method* instead of strings
* NEX-18885 - use actual snapshot creation time
* NEX-19172 - combine provisioner and attacher containers together
* NEX-19172 - use quay.io/k8scsi/csi-attacher:v1.0.1 instead of quay.io/k8scsi/csi-attacher:v0.4.1
* NEX-19172 - use csi-node-driver-registrar:v1.0.1 instead of driver-registrar:v1.0-canary

### Features

* update CSI spec to v1.1.0
* NEX-20837 - add timeo=100 as default options for NFS mounts
* NEX-18844 - create new volume from snapshot
* NEX-18844 - add clone snapshot, promote filesystem api; snapshot tests
* NEX-18844 - snapshots list method
* NEX-18885 - create/delete volume snapshots


<a name="v1.0.1"></a>
## [v1.0.1](https://github.com/Nexenta/nexentastor-csi-driver/compare/v1.0.0...v1.0.1) (2019-02-01)

### Bug Fixes

* NEX-19118 - mountFsType parameter doesn't work in storage class config


<a name="v1.0.0"></a>
## [v1.0.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/v0.2.0...v1.0.0) (2018-12-12)

### Features

* NEX-19172 - support k8s 1.13, migrate to CSI v1.0.0
* NEX-19172 - CSI spec v1.0.0 support


<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/v0.1.0...v0.2.0) (2018-12-04)

### Bug Fixes

* NEX-18885 - troubleshooting docs, format yaml files, microk8s config
* NEX-19100 - include csi-sanity tests to build pipeline
* NEX-19100 - list all supported volume capabilities
* NEX-18885 - s/src/pkg/
* NEX-18885 - don't generate volume name if not presented
* NEX-19172 - remove csi-common dependency
* NEX-18885 - use Pool struct instead of strings
* NEX-18885 - use response struct instead of interface for rest calls
* NEX-19148 - use 'referencedQuotaSize' instead of 'quotaSize'
* NEX-19148 - use typed params instead of interface - create smb/nfs share
* NEX-19148 - use typed params instead of interface - create filesystem
* NEX-19118 - add SMB share support - README
* NEX-18885 - do not recreate nsResolver/nsProvider/client on each request, watch config for changes instead
* NEX-19103 - apply ACL rules only when auto provisioning a volume
* NEX-18885 - lock/unlock reqiestID while changing
* NEX-19102 - swich versioning to 0.X.0
* NEX-18885 - detect is it controller or node pod and don't start unnecessary servers
* NEX-18885 - show less logs for attacher and provisioner, they can log secure data
* NEX-18883 - add secretName to tests configuration
* NEX-19013 - jenkins tests config

### Features

* NEX-19100 - csi-sanity tests container and configuration
* NEX-19100 - add --config-dir cli option
* NEX-18885 - validate restIp config value
* NEX-19034 - check if configured NSs is an actual cluster
* NEX-19118 - add SMB share support
* NEX-19143 - replace system:csi-* roles for k8s v1.13
* NEX-19119 - add 'vers=3' to default NFS options
* NEX-19119 - add FindRegexpIndexesString and AppendIfRegexpNotExistString array methods
* NEX-18959 - support mountOptions k8s configuration
* NEX-18885 - change driver volumes prefix to pvc-ns-*
* NEX-18885 - log GRPC errors before return
* NEX-18987 - add GetFilesystemAvailableCapacity() method
* NEX-19013 - support kubernetes >=1.12


<a name="v0.1.0"></a>
## v0.1.0 (2018-11-09)

### Bug Fixes

* NEX-19102 - swich versioning to 0.X.0
* NEX-18885 - use actual branch configs for e2e tests, separeate configs by version
* NEX-18885 - clean up after fs creation test
* NEX-18885 - change driver name to reverse domain name notation
* NEX-18885 - change driver name to reverse domain name notation
* NEX-18883 - remove goroutine, spelling, http codes
* NEX-18883 - don't log to much
* ns provider interval messages doesn't show up
* comment out inhereted methods
* comment out inhereted methods, add todos for future

### Features

* NEX-19000 - check NS license on driver start
* NEX-18959 - nfs options for mount command
* NEX-18885 - add identity server fror probe request
* NEX-18883 - add NOCOLOR option to tests
* NEX-18883 - configure jenkins
* NEX-18883 - configure jenkins
* NEX-18883 - configure jenkins
* NEX-18883 - build in container, run tests in container, jenkins build preparation
* nexentastor provider, logLn and getPools methods, auth

