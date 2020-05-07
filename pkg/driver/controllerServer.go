package driver

import (
    "fmt"
    "path/filepath"
    "strings"
    "strconv"

    "github.com/container-storage-interface/spec/lib/go/csi"
    timestamp "github.com/golang/protobuf/ptypes/timestamp"
    "github.com/kubernetes-csi/csi-lib-utils/protosanitizer"
    "github.com/sirupsen/logrus"
    "golang.org/x/net/context"
    "google.golang.org/grpc/codes"
    "google.golang.org/grpc/status"

    "github.com/Nexenta/go-nexentastor/pkg/ns"
    "github.com/Nexenta/nexentastor-csi-driver/pkg/config"
)

const TopologyKeyZone = "topology.kubernetes.io/zone"

// supportedControllerCapabilities - driver controller capabilities
var supportedControllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
    csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
    csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
    csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
    csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
    csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
    csi.ControllerServiceCapability_RPC_GET_CAPACITY,
    csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
}

// supportedVolumeCapabilities - driver volume capabilities
var supportedVolumeCapabilities = []*csi.VolumeCapability{
    {
        AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY},
    },
    {
        AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
    },
    {
        AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
    },
    {
        AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
    },
    {
        AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
    },
}

// ControllerServer - k8s csi driver controller server
type ControllerServer struct {
    nsResolverMap   map[string]ns.Resolver
    config          *config.Config
    log             *logrus.Entry
}

type ResolveNSParams struct {
    datasetPath string
    zone        string
    configName  string
}

type ResolveNSResponse struct {
    datasetPath string
    nsProvider  ns.ProviderInterface
    configName  string
}

func (s *ControllerServer) refreshConfig(secret string) error {
    changed, err := s.config.Refresh(secret)
    if err != nil {
        return err
    }
    if changed {
        s.log.Info("config has been changed, updating...")
        for name, cfg := range s.config.NsMap {
            resolver, err := ns.NewResolver(ns.ResolverArgs{
                Address:            cfg.Address,
                Username:           cfg.Username,
                Password:           cfg.Password,
                Log:                s.log,
                InsecureSkipVerify: true, //TODO move to config
            })
            s.nsResolverMap[name] = *resolver
            if err != nil {
                return fmt.Errorf("Cannot create NexentaStor resolver: %s", err)
            }
        }
    }

    return nil
}

func (s *ControllerServer) resolveNS(params ResolveNSParams) (response ResolveNSResponse, err error) {
    l := s.log.WithField("func", "resolveNS()")
    l.Infof("Resolving NS with params: %+v", params)
    if len(params.zone) == 0 {
        response, err = s.resolveNSNoZone(params)
    } else {
        response, err = s.resolveNSWithZone(params)
    }
    if err != nil {
        code := codes.Internal
        if ns.IsNotExistNefError(err) {
            code = codes.NotFound
        }
        return response, status.Errorf(
            code,
            "Cannot resolve '%s' on any NexentaStor(s): %s",
            params.datasetPath,
            err,
        )
    } else {
        l.Infof("resolved NS: [%s], %s, %s", response.configName, response.nsProvider, response.datasetPath)
        return response, nil
    }
}

func (s *ControllerServer) resolveNSNoZone(params ResolveNSParams) (response ResolveNSResponse, err error) {
    // No zone -> pick NS for given dataset and configName. TODO: load balancing
    l := s.log.WithField("func", "resolveNSNoZone()")
    l.Infof("Resolving without zone, params: %+v", params)
    var nsProvider ns.ProviderInterface
    datasetPath := params.datasetPath
    if len(params.configName) > 0 {
        if datasetPath == "" {
            datasetPath = s.config.NsMap[params.configName].DefaultDataset
        }
        resolver := s.nsResolverMap[params.configName]
        nsProvider, err = resolver.Resolve(datasetPath)
        if err != nil {
            return response, err
        }
        response = ResolveNSResponse{
            datasetPath: datasetPath,
            nsProvider: nsProvider,
            configName: params.configName,
        }
        return response, nil
    } else {
        for name, resolver := range s.nsResolverMap {
            if params.datasetPath == "" {
                datasetPath = s.config.NsMap[name].DefaultDataset
            }
            nsProvider, err = resolver.Resolve(datasetPath)
            if nsProvider != nil {
                response = ResolveNSResponse{
                    datasetPath: datasetPath,
                    nsProvider: nsProvider,
                    configName: name,
                }
                return response, err
            }
        }
    }
    return response, status.Errorf(codes.NotFound, fmt.Sprintf("No nsProvider found for params: %+v", params))
}

func (s *ControllerServer) resolveNSWithZone(params ResolveNSParams) (response ResolveNSResponse, err error) {
    // Pick NS with corresponding zone. TODO: load balancing
    l := s.log.WithField("func", "resolveNSWithZone()")
    l.Infof("Resolving with zone, params: %+v", params)
    var nsProvider ns.ProviderInterface
    datasetPath := params.datasetPath
    if len(params.configName) > 0 {
        if s.config.NsMap[params.configName].Zone != params.zone {
            msg := fmt.Sprintf(
                "requested zone [%s] does not match requested NexentaStor name [%s]", params.zone, params.configName)
            return response, status.Errorf(codes.FailedPrecondition, msg)
        }
        if datasetPath == "" {
            datasetPath = s.config.NsMap[params.configName].DefaultDataset
        }
        resolver := s.nsResolverMap[params.configName]
        nsProvider, err = resolver.Resolve(datasetPath)
        if err != nil {
            return response, err
        }
        response = ResolveNSResponse{
            datasetPath: datasetPath,
            nsProvider: nsProvider,
            configName: params.configName,
        }
        return response, nil
    } else {
        for name, resolver := range s.nsResolverMap {
            if params.datasetPath == "" {
                datasetPath = s.config.NsMap[name].DefaultDataset
            } else {
                datasetPath = params.datasetPath
            }
            if params.zone == s.config.NsMap[name].Zone {
                nsProvider, err = resolver.Resolve(datasetPath)
                if nsProvider != nil {
                    l.Infof("Found dataset %s on NexentaStor [%s]", datasetPath, name)
                    response = ResolveNSResponse{
                        datasetPath: datasetPath,
                        nsProvider: nsProvider,
                        configName: name,
                    }
                    l.Infof("configName: %+v", name)
                    return response, nil
                }
            }
        }
    }
    return response, status.Errorf(codes.NotFound, fmt.Sprintf("No nsProvider found for params: %+v", params))
}

// ListVolumes - list volumes, shows only volumes created in defaultDataset
//TODO return only shared fs?
func (s *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (
    *csi.ListVolumesResponse,
    error,
) {
    l := s.log.WithField("func", "ListVolumes()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    startingToken := req.GetStartingToken()
    maxEntries := int(req.GetMaxEntries())
    if maxEntries < 0 {
        return nil, status.Errorf(codes.InvalidArgument, "req.MaxEntries must be 0 or greater, got: %d", maxEntries)
    }

    err := s.refreshConfig("")
    if err != nil {
        return nil, status.Errorf(codes.Aborted, "Cannot use config file: %s", err)
    }
    nextToken := ""
    entries := []*csi.ListVolumesResponse_Entry{}
    filesystems := []ns.Filesystem{}
    for configName, _ := range s.config.NsMap {
        params := ResolveNSParams{
            configName: configName,
        }
        resolveResp, err := s.resolveNS(params)
        if err != nil {
            return nil, err
        }
        nsProvider := resolveResp.nsProvider
        datasetPath := resolveResp.datasetPath

        filesystems, nextToken, err = nsProvider.GetFilesystemsWithStartingToken(
            datasetPath,
            startingToken,
            maxEntries,
        )
        for _, item := range filesystems {
            entries = append(entries, &csi.ListVolumesResponse_Entry{
                Volume: &csi.Volume{VolumeId: fmt.Sprintf("%s:%s", configName, item.Path)},
            })
        }
    }
        
    if len(entries) == 0 {
        return nil, status.Errorf(codes.Aborted, "Cannot get filesystems: %s", err)
    } else if startingToken != "" && len(entries) == 0 {
        return nil, status.Errorf(
            codes.Aborted,
            fmt.Sprintf("Failed to find filesystem started from token '%s': %s", startingToken, err),
        )
    }


    l.Infof("found %d entries(s)", len(entries))

    return &csi.ListVolumesResponse{
        Entries:   entries,
        NextToken: nextToken,
    }, nil
}

// CreateVolume - creates FS on NexentaStor
func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
    res *csi.CreateVolumeResponse,
    err error,
) {
    l := s.log.WithField("func", "CreateVolume()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))
    volumeName := req.GetName()
    if len(volumeName) == 0 {
        return nil, status.Error(codes.InvalidArgument, "req.Name must be provided")
    }
    var secret string
    secrets := req.GetSecrets()
    for _, v := range secrets {
        secret = v
    }

    err = s.refreshConfig(secret)
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    //TODO validate VolumeCapability
    volumeCapabilities := req.GetVolumeCapabilities()
    if volumeCapabilities == nil {
        return nil, status.Error(codes.InvalidArgument, "req.VolumeCapabilities must be provided")
    }
    for _, reqC := range volumeCapabilities {
        supported := validateVolumeCapability(reqC)
        if !supported {
            message := fmt.Sprintf("Driver does not support volume capability mode: %s", reqC.GetAccessMode().GetMode())
            l.Warn(message)
            return nil, status.Error(codes.FailedPrecondition, message)
        }
    }

    var sourceSnapshotId string
    var sourceVolumeId string
    var volumePath string
    var contentSource *csi.VolumeContentSource
    var nsProvider ns.ProviderInterface
    var resolveResp ResolveNSResponse

    if volumeContentSource := req.GetVolumeContentSource(); volumeContentSource != nil {
        if sourceSnapshot := volumeContentSource.GetSnapshot(); sourceSnapshot != nil {
            sourceSnapshotId = sourceSnapshot.GetSnapshotId()
            contentSource = req.GetVolumeContentSource()
        } else if sourceVolume := volumeContentSource.GetVolume(); sourceVolume != nil {
            sourceVolumeId = sourceVolume.GetVolumeId()
            contentSource = req.GetVolumeContentSource()
        } else {
            return nil, status.Errorf(
                codes.InvalidArgument,
                "Only snapshots and volumes are supported as volume content source, but got type: %s",
                volumeContentSource.GetType(),
            )
        }
    }

    reqParams := req.GetParameters()
    if reqParams == nil {
        reqParams = make(map[string]string)
    }

    // get dataset path from runtime params, set default if not specified
    datasetPath := ""
    if v, ok := reqParams["dataset"]; ok {
        datasetPath = v
    }
    configName := ""
    if v, ok := reqParams["configName"]; ok {
        configName = v
    }
    requirements := req.GetAccessibilityRequirements()
    zone := s.pickAvailabilityZone(requirements)
    params := ResolveNSParams{
        datasetPath: datasetPath,
        zone: zone,
        configName: configName,
    }

    // get requested volume size from runtime params, set default if not specified
    capacityBytes := req.GetCapacityRange().GetRequiredBytes()
    if sourceSnapshotId != "" {
        // create new volume using existing snapshot
        splittedSnap := strings.Split(sourceSnapshotId, ":")
        if len(splittedSnap) != 2 {
            return nil, status.Error(codes.NotFound, fmt.Sprintf("SnapshotId is in wrong format: %s", sourceSnapshotId))
        }
        configName, sourceSnapshot := splittedSnap[0], splittedSnap[1]
        params.configName = configName
        resolveResp, err = s.resolveNS(params)
        if err != nil {
            return nil, err
        }
        nsProvider = resolveResp.nsProvider
        datasetPath = resolveResp.datasetPath
        volumePath = filepath.Join(datasetPath, volumeName)
        err = s.createNewVolumeFromSnapshot(nsProvider, sourceSnapshot, volumePath, capacityBytes)
    } else if sourceVolumeId != "" {
        // clone existing volume
        splittedVol := strings.Split(sourceVolumeId, ":")
        if len(splittedVol) != 2 {
            return nil, status.Error(codes.NotFound, fmt.Sprintf("VolumeId is in wrong format: %s", sourceVolumeId))
        }
        configName, sourceVolume := splittedVol[0], splittedVol[1]
        params.configName = configName
        resolveResp, err = s.resolveNS(params)
        if err != nil {
            return nil, err
        }
        nsProvider = resolveResp.nsProvider
        datasetPath = resolveResp.datasetPath
        volumePath = filepath.Join(datasetPath, volumeName)
        err = s.createClonedVolume(nsProvider, sourceVolume, volumePath, volumeName, capacityBytes)
    } else {
        resolveResp, err = s.resolveNS(params)
        if err != nil {
            return nil, err
        }
        nsProvider = resolveResp.nsProvider
        datasetPath = resolveResp.datasetPath
        volumePath = filepath.Join(datasetPath, volumeName)
        err = s.createNewVolume(nsProvider, volumePath, capacityBytes)
    }
    if err != nil {
        return nil, err
    }
    res = &csi.CreateVolumeResponse{
        Volume: &csi.Volume{
            ContentSource: contentSource,
            VolumeId:      fmt.Sprintf("%s:%s", resolveResp.configName, volumePath),
            CapacityBytes: capacityBytes,
            VolumeContext: map[string]string{
                "dataIp":       reqParams["dataIp"],
                "mountOptions": reqParams["mountOptions"],
                "mountFsType":  reqParams["mountFsType"],
            },
        },
    }
    if len(zone) > 0 {
        res.Volume.AccessibleTopology = []*csi.Topology{
            {
                Segments: map[string]string{TopologyKeyZone: zone},
            },
        }
    }

    // Create NFS share if passed in params
    if v, ok := reqParams["nfsAccessList"]; ok {
        ruleList := strings.Split(v, ",")
        nfsParams := ns.CreateNfsShareParams{
            Filesystem: volumePath,
        }

        for _, item := range ruleList {
            mode, address := "", ""
            mask := 0
            etype := "fqdn"
            if strings.Contains(item, ":") {
                splittedRule := strings.Split(item, ":")
                mode, address = strings.TrimSpace(splittedRule[0]), strings.TrimSpace(splittedRule[1])
            } else {
                mode, address = "rw", strings.TrimSpace(item)
            }

            if strings.Contains(address, "/") {
                splittedAddress := strings.Split(address, "/")[:2]
                address = splittedAddress[0]
                mask, err = strconv.Atoi(splittedAddress[1])
                    if err != nil {
                        return nil, err
                    }
            }
            if mask != 0 {
                etype = "network"
            }

            if mode == "ro" {
                nfsParams.ReadOnlyList = append(nfsParams.ReadOnlyList, ns.NfsRuleList{
                    Entity: address,
                    Etype: etype,
                    Mask: mask,
                })
            } else {
                nfsParams.ReadWriteList = append(nfsParams.ReadWriteList, ns.NfsRuleList{
                    Entity: address,
                    Etype: etype,
                    Mask: mask,
                })
            }
        }
        err = nsProvider.CreateNfsShare(nfsParams)
        if err != nil {
            return nil, err
        }
    }

    return res, nil
}

func (s *ControllerServer) createNewVolume(
    nsProvider ns.ProviderInterface,
    volumePath string,
    capacityBytes int64,
) (error) {
    l := s.log.WithField("func", "createNewVolume()")
    l.Infof("nsProvider: %s, volumePath: %s", nsProvider, volumePath)

    err := nsProvider.CreateFilesystem(ns.CreateFilesystemParams{
        Path:                volumePath,
        ReferencedQuotaSize: capacityBytes,
        //TODO consider to use option:
        // reservationSize (integer, optional): Sets the minimum amount of disk space guaranteed to a dataset
        // and its descendants. Value zero means no quota.
    })

    if err != nil {
        if ns.IsAlreadyExistNefError(err) {
            existingFilesystem, err := nsProvider.GetFilesystem(volumePath)
            if err != nil {
                return status.Errorf(
                    codes.Internal,
                    "Volume '%s' already exists, but volume properties request failed: %s",
                    volumePath,
                    err,
                )
            } else if capacityBytes != 0 && existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
                return status.Errorf(
                    codes.AlreadyExists,
                    "Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
                    volumePath,
                    capacityBytes,
                    existingFilesystem.GetReferencedQuotaSize(),
                )
            }

            l.Infof("volume '%s' already exists and can be used", volumePath)
            return nil
        }

        return status.Errorf(
            codes.Internal,
            "Cannot create volume '%s': %s",
            volumePath,
            err,
        )
    }

    l.Infof("volume '%s' has been created", volumePath)
    return nil
}

// create new volume using existing snapshot
func (s *ControllerServer) createNewVolumeFromSnapshot(
    nsProvider ns.ProviderInterface,
    sourceSnapshotID string,
    volumePath string,
    capacityBytes int64,
) (error) {
    l := s.log.WithField("func", "createNewVolumeFromSnapshot()")
    l.Infof("snapshot: %s", sourceSnapshotID)

    snapshot, err := nsProvider.GetSnapshot(sourceSnapshotID)
    if err != nil {
        message := fmt.Sprintf("Failed to find snapshot '%s': %s", sourceSnapshotID, err)
        if ns.IsNotExistNefError(err) || ns.IsBadArgNefError(err) {
            return status.Error(codes.NotFound, message)
        }
        return status.Error(codes.NotFound, message)
    }

    err = nsProvider.CloneSnapshot(snapshot.Path, ns.CloneSnapshotParams{
        TargetPath: volumePath,
        ReferencedQuotaSize: capacityBytes,
    })
    if err != nil {
        if ns.IsAlreadyExistNefError(err) {
            //TODO validate snapshot's "bytesReferenced" is less than required volume size

            // existingFilesystem, err := nsProvider.GetFilesystem(volumePath)
            // if err != nil {
            //  return nil, status.Errorf(
            //      codes.Internal,
            //      "Volume '%s' already exists, but volume properties request failed: %s",
            //      volumePath,
            //      err,
            //  )
            // } else if existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
            //  return nil, status.Errorf(
            //      codes.AlreadyExists,
            //      "Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
            //      volumePath,
            //      capacityBytes,
            //      existingFilesystem.GetReferencedQuotaSize(),
            //  )
            // }

            l.Infof("volume '%s' already exists and can be used", volumePath)
            return nil
        }

        return status.Errorf(
            codes.Internal,
            "Cannot create volume '%s' using snapshot '%s': %s",
            volumePath,
            snapshot.Path,
            err,
        )
    }

    //TODO resize volume after cloning from snapshot if needed

    l.Infof("volume '%s' has been created using snapshot '%s'", volumePath, snapshot.Path)
    return nil
}

func (s *ControllerServer) createClonedVolume(
    nsProvider ns.ProviderInterface,
    sourceVolumeID string,
    volumePath string,
    volumeName string,
    capacityBytes int64,
) (error) {

    l := s.log.WithField("func", "createClonedVolume()")
    l.Infof("clone volume source: %+v, target: %+v", sourceVolumeID, volumePath)

    snapName := fmt.Sprintf("k8s-clone-snapshot-%s", volumeName)
    snapshotPath := fmt.Sprintf("%s@%s", sourceVolumeID, snapName)

    _, err := s.CreateSnapshotOnNS(nsProvider, sourceVolumeID, snapName)
    if err != nil {
        return err
    }

    err = nsProvider.CloneSnapshot(snapshotPath, ns.CloneSnapshotParams{
        TargetPath: volumePath,
        ReferencedQuotaSize: capacityBytes,
    })

    if err != nil {
        if ns.IsAlreadyExistNefError(err) {
            //TODO validate snapshot's "bytesReferenced" is less than required volume size

            // existingFilesystem, err := nsProvider.GetFilesystem(volumePath)
            // if err != nil {
            //  return status.Errorf(
            //      codes.Internal,
            //      "Volume '%s' already exists, but volume properties request failed: %s",
            //      volumePath,
            //      err,
            //  )
            // } else if existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
            //  return status.Errorf(
            //      codes.AlreadyExists,
            //      "Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
            //      volumePath,
            //      capacityBytes,
            //      existingFilesystem.GetReferencedQuotaSize(),
            //  )
            // }

            l.Infof("volume '%s' already exists and can be used", volumePath)
            return nil
        }

        return status.Errorf(
            codes.NotFound,
            "Cannot create volume '%s' using snapshot '%s': %s",
            volumePath,
            snapshotPath,
            err,
        )
    }

    l.Infof("successfully created cloned volume %+v", volumePath)
    return nil
}

// DeleteVolume - destroys FS on NexentaStor
func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (
    *csi.DeleteVolumeResponse,
    error,
) {
    l := s.log.WithField("func", "DeleteVolume()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    var secret string
    secrets := req.GetSecrets()
    for _, v := range secrets {
        secret = v
    }
    err := s.refreshConfig(secret)
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    volumeId := req.GetVolumeId()
    if len(volumeId) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
    }
    splittedVol := strings.Split(volumeId, ":")
    if len(splittedVol) != 2 {
        l.Infof("Got wrong volumeId, but that is OK for deletion")
        return &csi.DeleteVolumeResponse{}, nil
    }
    configName, volumePath := splittedVol[0], splittedVol[1]

    params := ResolveNSParams{
        datasetPath: volumePath,
        configName: configName,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        if status.Code(err) == codes.NotFound {
            l.Infof("volume '%s' not found, that's OK for deletion request", volumePath)
            return &csi.DeleteVolumeResponse{}, nil
        }
        return nil, err
    }
    nsProvider := resolveResp.nsProvider

    // if here, than volumePath exists on some NS
    err = nsProvider.DestroyFilesystem(volumePath, ns.DestroyFilesystemParams{
        DestroySnapshots:               true,
        PromoteMostRecentCloneIfExists: true,
    })
    if err != nil && !ns.IsNotExistNefError(err) {
        return nil, status.Errorf(
            codes.Internal,
            "Cannot delete '%s' volume: %s",
            volumePath,
            err,
        )
    }

    l.Infof("volume '%s' has been deleted", volumePath)
    return &csi.DeleteVolumeResponse{}, nil
}

func (s *ControllerServer) CreateSnapshotOnNS(nsProvider ns.ProviderInterface, volumePath, snapName string) (
    snapshot ns.Snapshot, err error) {

    l := s.log.WithField("func", "CreateSnapshotOnNS()")
    l.Infof("creating snapshot %+v@%+v", volumePath, snapName)
    //TODO req.GetParameters() - read recursive param?

    //K8s doesn't allow to have same named snapshots for different volumes
    sourcePath := filepath.Dir(volumePath)

    existingSnapshots, err := nsProvider.GetSnapshots(sourcePath, true)
    if err != nil {
        return snapshot, status.Errorf(codes.NotFound, "Cannot get snapshots list: %s", err)
    }
    for _, s := range existingSnapshots {
        if s.Name == snapName && s.Parent != volumePath {
            return snapshot, status.Errorf(
                codes.AlreadyExists,
                "Snapshot '%s' already exists for filesystem: %s",
                snapName,
                s.Path,
            )
        }
    }

    snapshotPath := fmt.Sprintf("%s@%s", volumePath, snapName)

    // if here, than volumePath exists on some NS
    err = nsProvider.CreateSnapshot(ns.CreateSnapshotParams{
        Path: snapshotPath,
    })
    if err != nil && !ns.IsAlreadyExistNefError(err) {
        return snapshot, status.Errorf(codes.Internal, "Cannot create snapshot '%s': %s", snapshotPath, err)
    }

    snapshot, err = nsProvider.GetSnapshot(snapshotPath)
    if err != nil {
        return snapshot, status.Errorf(
            codes.Internal,
            "Snapshot '%s' has been created, but snapshot properties request failed: %s",
            snapshotPath,
            err,
        )
    }
    l.Infof("successfully created snapshot %+v@%+v", volumePath, snapName)
    return snapshot, nil
}


// CreateSnapshot creates a snapshot of given volume
func (s *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
    *csi.CreateSnapshotResponse,
    error,
) {
    l := s.log.WithField("func", "CreateSnapshot()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    err := s.refreshConfig("")
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    sourceVolumeId := req.GetSourceVolumeId()
    if len(sourceVolumeId) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Snapshot source volume ID must be provided")
    }
    splittedVol := strings.Split(sourceVolumeId, ":")
    if len(splittedVol) != 2 {
        return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("VolumeId is in wrong format: %s", sourceVolumeId))
    }
    if len(splittedVol) != 2 {
        return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("VolumeId is in wrong format: %s", sourceVolumeId))
    }
    configName, volumePath := splittedVol[0], splittedVol[1]

    name := req.GetName()
    if len(name) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Snapshot name must be provided")
    }

    params := ResolveNSParams{
        datasetPath: volumePath,
        configName:  configName,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        return nil, err
    }

    snapshotPath := fmt.Sprintf("%s@%s", volumePath, name)
    createdSnapshot, err := s.CreateSnapshotOnNS(resolveResp.nsProvider, volumePath, name)
    if err != nil {
        return nil, err
    }
    creationTime := &timestamp.Timestamp{
        Seconds: createdSnapshot.CreationTime.Unix(),
    }

    snapshotId := fmt.Sprintf("%s:%s", configName, snapshotPath)
    res := &csi.CreateSnapshotResponse{
        Snapshot: &csi.Snapshot{
            SnapshotId:     snapshotId,
            SourceVolumeId: sourceVolumeId,
            CreationTime:   creationTime,
            ReadyToUse:     true, //TODO use actual state
            //SizeByte: 0 //TODO size of zero means it is unspecified
        },
    }

    if ns.IsAlreadyExistNefError(err) {
        l.Infof("snapshot '%s' already exists and can be used", snapshotId)
        return res, nil
    }

    l.Infof("snapshot '%s' has been created", snapshotId)
    return res, nil
}

// DeleteSnapshot deletes snapshots
func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
    *csi.DeleteSnapshotResponse,
    error,
) {
    l := s.log.WithField("func", "DeleteSnapshot()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    err := s.refreshConfig("")
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    snapshotPath := req.GetSnapshotId()
    if len(snapshotPath) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Snapshot ID must be provided")
    }

    volume := ""
    splittedString := strings.Split(snapshotPath, "@")
    if len(splittedString) == 2 {
        volume = splittedString[0]
    } else {
        l.Infof("snapshot '%s' not found, that's OK for deletion request", snapshotPath)
        return &csi.DeleteSnapshotResponse{}, nil
    }
    splittedVol := strings.Split(volume, ":")
    if len(splittedVol) != 2 {
        return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("VolumeId is in wrong format: %s", volume))
    }
    configName, volumePath := splittedVol[0], splittedVol[1]

    params := ResolveNSParams{
        datasetPath: volumePath,
        configName: configName,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        if status.Code(err) == codes.NotFound {
            l.Infof("snapshot '%s' not found, that's OK for deletion request", snapshotPath)
            return &csi.DeleteSnapshotResponse{}, nil
        }
        return nil, err
    }
    nsProvider := resolveResp.nsProvider

    // if here, than volumePath exists on some NS
    err = nsProvider.DestroySnapshot(snapshotPath)
    if err != nil && !ns.IsNotExistNefError(err) {
        message := fmt.Sprintf("Failed to delete snapshot '%s'", snapshotPath)
        if ns.IsBusyNefError(err) {
            message += ", it has dependent filesystem"
        }
        return nil, status.Errorf(codes.Internal, "%s: %s", message, err)
    }

    l.Infof("snapshot '%s' has been deleted", snapshotPath)
    return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots returns the list of snapshots
func (s *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (
    *csi.ListSnapshotsResponse,
    error,
) {
    l := s.log.WithField("func", "ListSnapshots()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    //TODO try this when list issue is solved
    err := s.refreshConfig("")
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    if req.GetSnapshotId() != "" {
        // identity information for a specific snapshot, can be used to list only a specific snapshot
        return s.getSnapshotListWithSingleSnapshot(req.GetSnapshotId(), req)
    } else if req.GetSourceVolumeId() != "" {
        // identity information for the source volume, can be used to list snapshots by volume
        return s.getFilesystemSnapshotList(req.GetSourceVolumeId(), req)
    } else {
        // return list of all snapshots from default datasets
        response := csi.ListSnapshotsResponse{
            Entries: []*csi.ListSnapshotsResponse_Entry{},
        }
        for _, cfg := range s.config.NsMap {
            resp, _ := s.getFilesystemSnapshotList(cfg.DefaultDataset, req)
            for _, snapshot := range resp.Entries {
                response.Entries = append(response.Entries, snapshot)
            }
            if len(resp.NextToken) > 0 {
                response.NextToken = resp.NextToken
                return &response, nil
            }
        }
        return &response, nil
    }
}

func (s *ControllerServer) getSnapshotListWithSingleSnapshot(snapshotId string, req *csi.ListSnapshotsRequest) (
    *csi.ListSnapshotsResponse,
    error,
) {
    l := s.log.WithField("func", "getSnapshotListWithSingleSnapshot()")
    l.Infof("Snapshot path: %s", snapshotId)

    response := csi.ListSnapshotsResponse{
        Entries: []*csi.ListSnapshotsResponse_Entry{},
    }

    splittedSnapshotPath := strings.Split(snapshotId, "@")
    if len(splittedSnapshotPath) != 2 {
        // bad snapshotID format, but it's ok, driver should return empty response
        l.Infof("Bad snapshot format: %s", splittedSnapshotPath)
        return &response, nil
    }
    splittedSnapshotId := strings.Split(splittedSnapshotPath[0], ":")
    configName, filesystemPath := splittedSnapshotId[0], splittedSnapshotId[1]

    params := ResolveNSParams{
        datasetPath: filesystemPath,
        configName: configName,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        if status.Code(err) == codes.NotFound {
            l.Infof("filesystem '%s' not found, that's OK for list request", filesystemPath)
            return &response, nil
        }
        return nil, err
    }
    nsProvider := resolveResp.nsProvider
    snapshotPath := fmt.Sprintf("%s@%s", filesystemPath, splittedSnapshotPath[1])
    snapshot, err := nsProvider.GetSnapshot(snapshotPath)
    if err != nil {
        if ns.IsNotExistNefError(err) {
            return &response, nil
        }
        return nil, status.Errorf(codes.Internal, "Cannot get snapshot '%s' for snapshot list: %s", snapshotPath, err)
    }
    response.Entries = append(response.Entries, convertNSSnapshotToCSISnapshot(snapshot, configName))
    l.Infof("snapshot '%s' found for '%s' filesystem", snapshot.Path, filesystemPath)
    return &response, nil
}

func (s *ControllerServer) getFilesystemSnapshotList(volumeId string, req *csi.ListSnapshotsRequest) (
    *csi.ListSnapshotsResponse,
    error,
) {
    l := s.log.WithField("func", "getFilesystemSnapshotList()")
    l.Infof("filesystem path: %s", volumeId)

    startingToken := req.GetStartingToken()
    maxEntries := req.GetMaxEntries()

    response := csi.ListSnapshotsResponse{
        Entries: []*csi.ListSnapshotsResponse_Entry{},
    }
    splittedVol := strings.Split(volumeId, ":")
    volumePath := ""
    configName := ""
    params := ResolveNSParams{}
    if len(splittedVol) == 2 {
        configName, volumePath = splittedVol[0], splittedVol[1]
        params.datasetPath = volumePath
        params.configName = configName
    } else {
        volumePath = volumeId
        params.datasetPath = volumePath
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        l.Infof("volume '%s' not found, that's OK for list request", volumePath)
        return &response, nil
    }

    nsProvider := resolveResp.nsProvider
    snapshots, err := nsProvider.GetSnapshots(volumePath, true)
    if err != nil {
        return nil, status.Errorf(codes.Internal, "Cannot get snapshot list for '%s': %s", volumePath, err)
    }

    for i, snapshot := range snapshots {
        // skip all snapshots before startring token
        if snapshot.Path == startingToken {
            startingToken = ""
        }
        if startingToken != "" {
            continue
        }

        response.Entries = append(response.Entries, convertNSSnapshotToCSISnapshot(snapshot, resolveResp.configName))

        // if the requested maximum is reached (and specified) than set next token
        if maxEntries != 0 && int32(len(response.Entries)) == maxEntries {
            if i+1 < len(snapshots) { // next snapshots index exists
                l.Infof(
                    "max entries count (%d) has been reached while getting snapshots for '%s' filesystem, "+
                        "send response with next_token for pagination",
                    maxEntries,
                    volumePath,
                )
                response.NextToken = snapshots[i+1].Path
                return &response, nil
            }
        }
    }

    l.Infof("found %d snapshot(s) for %s filesystem", len(response.Entries), volumePath)

    return &response, nil
}

func convertNSSnapshotToCSISnapshot(snapshot ns.Snapshot, configName string) *csi.ListSnapshotsResponse_Entry {
    return &csi.ListSnapshotsResponse_Entry{
        Snapshot: &csi.Snapshot{
            SnapshotId:     fmt.Sprintf("%s:%s", configName, snapshot.Path),
            SourceVolumeId: fmt.Sprintf("%s:%s", configName, snapshot.Parent),
            CreationTime: &timestamp.Timestamp{
                Seconds: snapshot.CreationTime.Unix(),
            },
            ReadyToUse: true, //TODO use actual state
            //SizeByte: 0 //TODO size of zero means it is unspecified
        },
    }
}

// ValidateVolumeCapabilities validates volume capabilities
// Shall return confirmed only if all the volume
// capabilities specified in the request are supported.
func (s *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
    *csi.ValidateVolumeCapabilitiesResponse,
    error,
) {
    l := s.log.WithField("func", "ValidateVolumeCapabilities()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    volumeId := req.GetVolumeId()
    if len(volumeId) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
    }
    splittedVol := strings.Split(volumeId, ":")
    if len(splittedVol) != 2 {
        return nil, status.Error(codes.NotFound, fmt.Sprintf("VolumeId is in wrong format: %s", volumeId))
    }

    volumeCapabilities := req.GetVolumeCapabilities()
    if volumeCapabilities == nil {
        return nil, status.Error(codes.InvalidArgument, "req.VolumeCapabilities must be provided")
    }

    // volume attributes are passed from ControllerServer.CreateVolume()
    volumeContext := req.GetVolumeContext()

    var secret string
    secrets := req.GetSecrets()
    for _, v := range secrets {
        secret = v
    }
    err := s.refreshConfig(secret)
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }

    for _, reqC := range volumeCapabilities {
        supported := validateVolumeCapability(reqC)
        l.Infof("requested capability: '%s', supported: %t", reqC.GetAccessMode().GetMode(), supported)
        if !supported {
            message := fmt.Sprintf("Driver does not support volume capability mode: %s", reqC.GetAccessMode().GetMode())
            l.Warn(message)
            return &csi.ValidateVolumeCapabilitiesResponse{
                Message: message,
            }, nil
        }
    }

    return &csi.ValidateVolumeCapabilitiesResponse{
        Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
            VolumeCapabilities: supportedVolumeCapabilities,
            VolumeContext:      volumeContext,
        },
    }, nil
}

// ControllerGetCapabilities - controller capabilities
func (s *ControllerServer) ControllerGetCapabilities(ctx context.Context, req *csi.ControllerGetCapabilitiesRequest) (
    *csi.ControllerGetCapabilitiesResponse,
    error,
) {
    s.log.WithField("func", "ControllerGetCapabilities()").Infof("request: '%+v'", req)

    var capabilities []*csi.ControllerServiceCapability
    for _, c := range supportedControllerCapabilities {
        capabilities = append(capabilities, newControllerServiceCapability(c))
    }
    return &csi.ControllerGetCapabilitiesResponse{
        Capabilities: capabilities,
    }, nil
}

func newControllerServiceCapability(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
    return &csi.ControllerServiceCapability{
        Type: &csi.ControllerServiceCapability_Rpc{
            Rpc: &csi.ControllerServiceCapability_RPC{
                Type: cap,
            },
        },
    }
}

func validateVolumeCapability(requestedVolumeCapability *csi.VolumeCapability) bool {
    // block is not supported
    if requestedVolumeCapability.GetBlock() != nil {
        return false
    }

    requestedMode := requestedVolumeCapability.GetAccessMode().GetMode()
    for _, volumeCapability := range supportedVolumeCapabilities {
        if volumeCapability.GetAccessMode().GetMode() == requestedMode {
            return true
        }
    }
    return false
}

func (s *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (
    *csi.GetCapacityResponse,
    error,
) {
    l := s.log.WithField("func", "GetCapacity()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    reqParams := req.GetParameters()
    if reqParams == nil {
        reqParams = make(map[string]string)
    }

    // get dataset path from runtime params, set default if not specified
    datasetPath := ""
    if v, ok := reqParams["dataset"]; ok {
        datasetPath = v
    }

    params := ResolveNSParams{
        datasetPath: datasetPath,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        return nil, err
    }

    nsProvider := resolveResp.nsProvider
    filesystem, err := nsProvider.GetFilesystem(resolveResp.datasetPath)
    if err != nil {
        return nil, err
    }

    availableCapacity := filesystem.GetReferencedQuotaSize()
    l.Infof("Available capacity: '%+v' bytes", availableCapacity)
    return &csi.GetCapacityResponse{
        AvailableCapacity: availableCapacity,
    }, nil
}

// ControllerPublishVolume - not supported
func (s *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
    *csi.ControllerPublishVolumeResponse,
    error,
) {
    s.log.WithField("func", "ControllerPublishVolume()").Warnf("request: '%+v' - not implemented", req)
    return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume - not supported
func (s *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
    *csi.ControllerUnpublishVolumeResponse,
    error,
) {
    s.log.WithField("func", "ControllerUnpublishVolume()").Warnf("request: '%+v' - not implemented", req)
    return nil, status.Error(codes.Unimplemented, "")
}

// ControllerExpandVolume - not supported
func (s *ControllerServer) ControllerExpandVolume(ctx context.Context, req *csi.ControllerExpandVolumeRequest) (
    *csi.ControllerExpandVolumeResponse,
    error,
) {
    l := s.log.WithField("func", "ControllerExpandVolume()")
    l.Infof("request: '%+v'", protosanitizer.StripSecrets(req))

    var secret string
    secrets := req.GetSecrets()
    for _, v := range secrets {
        secret = v
    }
    err := s.refreshConfig(secret)
    if err != nil {
        return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
    }
    capacityBytes := req.GetCapacityRange().GetRequiredBytes()
    if capacityBytes == 0 {
        return nil, status.Error(codes.InvalidArgument, "GetRequiredBytes must be >0")
    }

    volumeId := req.GetVolumeId()
    if len(volumeId) == 0 {
        return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
    }
    splittedVol := strings.Split(volumeId, ":")
    if len(splittedVol) != 2 {
        return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("VolumeId is in wrong format: %s", volumeId))
    }
    configName, volumePath := splittedVol[0], splittedVol[1]

    params := ResolveNSParams{
        datasetPath: volumePath,
        configName: configName,
    }
    resolveResp, err := s.resolveNS(params)
    if err != nil {
        return nil, err
    }
    nsProvider := resolveResp.nsProvider

    l.Infof("expanding volume %+v to %+v bytes", volumePath, capacityBytes)
    err = nsProvider.UpdateFilesystem(volumePath, ns.UpdateFilesystemParams{
        ReferencedQuotaSize: capacityBytes,
    })
    if err != nil {
        return nil, fmt.Errorf("Failed to expand volume volume %s: %s", volumePath, err)
    }
    return &csi.ControllerExpandVolumeResponse{
        CapacityBytes: capacityBytes,
    }, nil
}

func (s *ControllerServer) pickAvailabilityZone(requirement *csi.TopologyRequirement) string {
    l := s.log.WithField("func", "s.pickAvailabilityZone()")
    l.Infof("AccessibilityRequirements: '%+v'", requirement)
    if requirement == nil {
        return ""
    }
    for _, topology := range requirement.GetPreferred() {
        zone, exists := topology.GetSegments()[TopologyKeyZone]
        if exists {
            return zone
        }
    }
    for _, topology := range requirement.GetRequisite() {
        zone, exists := topology.GetSegments()[TopologyKeyZone]
        if exists {
            return zone
        }
    }
    return ""
}

// NewControllerServer - create an instance of controller service
func NewControllerServer(driver *Driver) (*ControllerServer, error) {
    l := driver.log.WithField("cmp", "ControllerServer")
    l.Info("create new ControllerServer...")
    resolverMap := make(map[string]ns.Resolver)

    for name, cfg := range driver.config.NsMap {
        nsResolver, err := ns.NewResolver(ns.ResolverArgs{
            Address:            cfg.Address,
            Username:           cfg.Username,
            Password:           cfg.Password,
            Log:                l,
            InsecureSkipVerify: true, //TODO move to config
        })
        if err != nil {
            return nil, fmt.Errorf("Cannot create NexentaStor resolver: %s", err)
        }
        resolverMap[name] = *nsResolver
    }

    l.Info("Resolver map: %+v", resolverMap)
    return &ControllerServer{
        nsResolverMap: resolverMap,
        config:     driver.config,
        log:        l,
    }, nil
}
