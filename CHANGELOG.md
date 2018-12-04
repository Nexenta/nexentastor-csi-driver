
<a name="0.2.0"></a>
## [0.2.0](https://github.com/Nexenta/nexentastor-csi-driver/compare/0.1.0...0.2.0) (2018-12-04)

### Features

* NEX-19013 - support kubernetes >=1.12
* NEX-19118 - add SMB share support
* NEX-18885 - validate restIp config value
* NEX-19034 - check if configured NSs is an actual cluster
* NEX-19100 - add --config-dir cli option
* NEX-19100 - csi-sanity tests container and configuration
* NEX-18885 - change driver volumes prefix to pvc-ns-*
* NEX-18885 - log GRPC errors before return

### Fixes

* NEX-19119 - add 'vers=3' to default NFS options
* NEX-19100 - include csi-sanity tests to build pipeline
* NEX-19100 - list all supported volume capabilities
* NEX-19148 - use 'referencedQuotaSize' instead of 'quotaSize'
* NEX-18885 - do not recreate nsResolver/nsProvider/client on each request, watch config for changes instead
* NEX-19103 - apply ACL rules only when auto provisioning a volume
* NEX-18885 - detect is it controller or node pod and don't start unnecessary servers
* NEX-18885 - show less logs for attacher and provisioner, they can log secure data


<a name="0.1.0"></a>
## 0.1.0 (2018-11-09)

### Features

* NEX-19000 - check NS license on driver start
* NEX-18959 - nfs options for mount command

### Fixes

* NEX-19102 - switch versioning to 0.X.0
* NEX-18885 - change driver name to reverse domain name notation
