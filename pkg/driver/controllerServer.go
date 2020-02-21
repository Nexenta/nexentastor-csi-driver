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

// supportedControllerCapabilities - driver controller capabilities
var supportedControllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS,
	csi.ControllerServiceCapability_RPC_CLONE_VOLUME,
	csi.ControllerServiceCapability_RPC_GET_CAPACITY,
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
	nsResolver *ns.Resolver
	config     *config.Config
	log        *logrus.Entry
}

func (s *ControllerServer) refreshConfig(secret string) error {
	changed, err := s.config.Refresh(secret)
	if err != nil {
		return err
	}
	if changed {
		s.log.Info("config has been changed, updating...")
		s.nsResolver, err = ns.NewResolver(ns.ResolverArgs{
			Address:            s.config.Address,
			Username:           s.config.Username,
			Password:           s.config.Password,
			Log:                s.log,
			InsecureSkipVerify: true, //TODO move to config
		})
		if err != nil {
			return fmt.Errorf("Cannot create NexentaStor resolver: %s", err)
		}
	}

	return nil
}

func (s *ControllerServer) resolveNS(datasetPath string) (ns.ProviderInterface, error) {
	nsProvider, err := s.nsResolver.Resolve(datasetPath)
	if err != nil {
		code := codes.Internal
		if ns.IsNotExistNefError(err) {
			code = codes.NotFound
		}
		return nil, status.Errorf(
			code,
			"Cannot resolve '%s' on any NexentaStor(s): %s",
			datasetPath,
			err,
		)
	}
	return nsProvider, nil
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
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
	}

	nsProvider, err := s.resolveNS(s.config.DefaultDataset)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, s.config.DefaultDataset)

	filesystems, nextToken, err := nsProvider.GetFilesystemsWithStartingToken(
		s.config.DefaultDataset,
		startingToken,
		maxEntries,
	)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot get filesystems: %s", err)
	} else if startingToken != "" && len(filesystems) == 0 {
		return nil, status.Errorf(
			codes.Aborted,
			"Failed to find filesystem started from token '%s': %s",
			startingToken,
			err,
		)
	}

	entries := make([]*csi.ListVolumesResponse_Entry, len(filesystems))
	for i, item := range filesystems {
		entries[i] = &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{VolumeId: item.Path},
		}
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
	var secret string
	secrets := req.GetSecrets()
	for _, v := range secrets {
		secret = v
	}

	err = s.refreshConfig(secret)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
	}
	volumeName := req.GetName()
	if len(volumeName) == 0 {
		return nil, status.Error(codes.InvalidArgument, "req.Name must be provided")
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

	reqParams := req.GetParameters()
	if reqParams == nil {
		reqParams = make(map[string]string)
	}

	// get dataset path from runtime params, set default if not specified
	datasetPath := ""
	if v, ok := reqParams["dataset"]; ok {
		datasetPath = v
	} else {
		datasetPath = s.config.DefaultDataset
	}

	nsProvider, err := s.resolveNS(datasetPath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, datasetPath)

	// use full path as volume ID
	volumePath := filepath.Join(datasetPath, volumeName)

	// get requested volume size from runtime params, set default if not specified
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()

	res = &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			//ContentSource //TODO add id created from snapshot
			VolumeId:      volumePath,
			CapacityBytes: capacityBytes,
			VolumeContext: map[string]string{
				"dataIp":       reqParams["dataIp"],
				"mountOptions": reqParams["mountOptions"],
				"mountFsType":  reqParams["mountFsType"],
			},
		},
	}

	var sourceSnapshotID string
	var sourceVolumeID string
	if volumeContentSource := req.GetVolumeContentSource(); volumeContentSource != nil {
		if sourceSnapshot := volumeContentSource.GetSnapshot(); sourceSnapshot != nil {
			sourceSnapshotID = sourceSnapshot.GetSnapshotId()
			res.Volume.ContentSource = req.GetVolumeContentSource()
		} else if sourceVolume := volumeContentSource.GetVolume(); sourceVolume != nil {
			sourceVolumeID = sourceVolume.GetVolumeId()
			res.Volume.ContentSource = req.GetVolumeContentSource()
		} else {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"Only snapshots and volumes are supported as volume content source, but got type: %s",
				volumeContentSource.GetType(),
			)
		}
	}

	if sourceSnapshotID != "" {
		// create new volume using existing snapshot
		res, err = s.createNewVolumeFromSnapshot(nsProvider, sourceSnapshotID, volumePath, capacityBytes, res)
	}
	if sourceVolumeID != "" {
		// clone existing volume
		res, err = s.createClonedVolume(nsProvider, sourceVolumeID, volumePath, volumeName, capacityBytes, res)
	}

	res, err = s.createNewVolume(nsProvider, volumePath, capacityBytes, res)
	if err != nil {
		return nil, err
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
	res *csi.CreateVolumeResponse,
) (*csi.CreateVolumeResponse, error) {
	l := s.log.WithField("func", "createNewVolume()")

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
				return nil, status.Errorf(
					codes.Internal,
					"Volume '%s' already exists, but volume properties request failed: %s",
					volumePath,
					err,
				)
			} else if capacityBytes != 0 && existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
				return nil, status.Errorf(
					codes.AlreadyExists,
					"Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
					volumePath,
					capacityBytes,
					existingFilesystem.GetReferencedQuotaSize(),
				)
			}

			l.Infof("volume '%s' already exists and can be used", volumePath)
			return res, nil
		}

		return nil, status.Errorf(
			codes.Internal,
			"Cannot create volume '%s': %s",
			volumePath,
			err,
		)
	}

	l.Infof("volume '%s' has been created", volumePath)
	return res, nil
}

// create new volume using existing snapshot
func (s *ControllerServer) createNewVolumeFromSnapshot(
	nsProvider ns.ProviderInterface,
	sourceSnapshotID string,
	volumePath string,
	capacityBytes int64,
	res *csi.CreateVolumeResponse,
) (*csi.CreateVolumeResponse, error) {
	l := s.log.WithField("func", "createNewVolumeFromSnapshot()")
	l.Infof("snapshot: %s", sourceSnapshotID)

	snapshot, err := nsProvider.GetSnapshot(sourceSnapshotID)
	if err != nil {
		message := fmt.Sprintf("Failed to find snapshot '%s': %s", sourceSnapshotID, err)
		if ns.IsNotExistNefError(err) || ns.IsBadArgNefError(err) {
			return nil, status.Error(codes.NotFound, message)
		}
		return nil, status.Error(codes.Internal, message)
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
			// 	return nil, status.Errorf(
			// 		codes.Internal,
			// 		"Volume '%s' already exists, but volume properties request failed: %s",
			// 		volumePath,
			// 		err,
			// 	)
			// } else if existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
			// 	return nil, status.Errorf(
			// 		codes.AlreadyExists,
			// 		"Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
			// 		volumePath,
			// 		capacityBytes,
			// 		existingFilesystem.GetReferencedQuotaSize(),
			// 	)
			// }

			l.Infof("volume '%s' already exists and can be used", volumePath)
			return res, nil
		}

		return nil, status.Errorf(
			codes.Internal,
			"Cannot create volume '%s' using snapshot '%s': %s",
			volumePath,
			snapshot.Path,
			err,
		)
	}

	//TODO resize volume after cloning from snapshot if needed

	l.Infof("volume '%s' has been created using snapshot '%s'", volumePath, snapshot.Path)
	return res, nil
}

func (s *ControllerServer) createClonedVolume(
	nsProvider ns.ProviderInterface,
	sourceVolumeID string,
	volumePath string,
	volumeName string,
	capacityBytes int64,
	res *csi.CreateVolumeResponse,
) (*csi.CreateVolumeResponse, error) {

	l := s.log.WithField("func", "createClonedVolume()")
	l.Infof("clone volume source: %+v, target: %+v", sourceVolumeID, volumeName)

	snapName := fmt.Sprintf("k8s-clone-snapshot-%s", volumeName)
	snapshotPath := fmt.Sprintf("%s@%s", sourceVolumeID, snapName)

	_, err := s.CreateSnapshotOnNS(nsProvider, sourceVolumeID, snapName)
	if err != nil {
		msg := fmt.Sprintf("Could not create snapshot '%s'", snapshotPath)
		l.Infof(msg)
		return nil, status.Errorf(
			codes.NotFound,
			msg,
			err,
		)
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
			// 	return nil, status.Errorf(
			// 		codes.Internal,
			// 		"Volume '%s' already exists, but volume properties request failed: %s",
			// 		volumePath,
			// 		err,
			// 	)
			// } else if existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
			// 	return nil, status.Errorf(
			// 		codes.AlreadyExists,
			// 		"Volume '%s' already exists, but with a different size: requested=%d, existing=%d",
			// 		volumePath,
			// 		capacityBytes,
			// 		existingFilesystem.GetReferencedQuotaSize(),
			// 	)
			// }

			l.Infof("volume '%s' already exists and can be used", volumePath)
			return res, nil
		}

		return nil, status.Errorf(
			codes.Internal,
			"Cannot create volume '%s' using snapshot '%s': %s",
			volumePath,
			snapshotPath,
			err,
		)
	}

	return res, nil
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

	volumePath := req.GetVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		l.Infof("%s", status.Code(err))
		if status.Code(err) == codes.NotFound {
			l.Infof("volume '%s' not found, that's OK for deletion request", volumePath)
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

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
		return snapshot, status.Errorf(codes.Internal, "Cannot get snapshots list: %s", err)
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

	volumePath := req.GetSourceVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot source volume ID must be provided")
	}

	name := req.GetName()
	if len(name) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Snapshot name must be provided")
	}

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		return nil, err
	}
	l.Infof("resolved NS: %s, path: %s", nsProvider, volumePath)

	snapshotPath := fmt.Sprintf("%s@%s", volumePath, name)
	createdSnapshot, err := s.CreateSnapshotOnNS(nsProvider, volumePath, name)
	if err != nil {
		l.Infof("Could not create snapshot '%s'", snapshotPath)
		return nil, err
	}
	creationTime := &timestamp.Timestamp{
		Seconds: createdSnapshot.CreationTime.Unix(),
	}

	res := &csi.CreateSnapshotResponse{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshotPath,
			SourceVolumeId: volumePath,
			CreationTime:   creationTime,
			ReadyToUse:     true, //TODO use actual state
			//SizeByte: 0 //TODO size of zero means it is unspecified
		},
	}

	if ns.IsAlreadyExistNefError(err) {
		l.Infof("snapshot '%s' already exists and can be used", snapshotPath)
		return res, nil
	}

	l.Infof("snapshot '%s' has been created", snapshotPath)
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

	volumePath := ""
	splittedString := strings.Split(snapshotPath, "@")
	if len(splittedString) == 2 {
		volumePath = splittedString[0]
	} else {
		l.Infof("snapshot '%s' not found, that's OK for deletion request", snapshotPath)
		return &csi.DeleteSnapshotResponse{}, nil
	}

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			l.Infof("snapshot '%s' not found, that's OK for deletion request", snapshotPath)
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, snapshotPath)

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
	} else if s.config.DefaultDataset != "" {
		// return list of all snapshots from default dataset
		return s.getFilesystemSnapshotList(s.config.DefaultDataset, req)
	}

	// no volume id provided, return empty list
	return &csi.ListSnapshotsResponse{
		Entries: []*csi.ListSnapshotsResponse_Entry{},
	}, nil
}

func (s *ControllerServer) getSnapshotListWithSingleSnapshot(snapshotPath string, req *csi.ListSnapshotsRequest) (
	*csi.ListSnapshotsResponse,
	error,
) {
	l := s.log.WithField("func", "getSnapshotListWithSingleSnapshot()")
	l.Infof("snapshots path: %s", snapshotPath)

	response := csi.ListSnapshotsResponse{
		Entries: []*csi.ListSnapshotsResponse_Entry{},
	}

	splittedSnapshotPath := strings.Split(snapshotPath, "@")
	if len(splittedSnapshotPath) != 2 {
		// bad snapshotID format, but it's ok, driver should return empty response
		return &response, nil
	}

	filesystemPath := splittedSnapshotPath[0]

	nsProvider, err := s.resolveNS(filesystemPath)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			l.Infof("filesystem '%s' not found, that's OK for list request", filesystemPath)
			return &response, nil
		}
		return nil, err
	}

	snapshot, err := nsProvider.GetSnapshot(snapshotPath)
	if err != nil {
		if ns.IsNotExistNefError(err) {
			return &response, nil
		}
		return nil, status.Errorf(codes.Internal, "Cannot get snapshot '%s' for snapshot list: %s", snapshotPath, err)
	}

	response.Entries = append(response.Entries, convertNSSnapshotToCSISnapshot(snapshot))

	l.Infof("snapshot '%s' found for '%s' filesystem", snapshot.Path, filesystemPath)

	return &response, nil
}

func (s *ControllerServer) getFilesystemSnapshotList(filesystemPath string, req *csi.ListSnapshotsRequest) (
	*csi.ListSnapshotsResponse,
	error,
) {
	l := s.log.WithField("func", "getFilesystemSnapshotList()")
	l.Infof("filesystem path: %s", filesystemPath)

	startingToken := req.GetStartingToken()
	maxEntries := req.GetMaxEntries()

	response := csi.ListSnapshotsResponse{
		Entries: []*csi.ListSnapshotsResponse_Entry{},
	}

	nsProvider, err := s.resolveNS(filesystemPath)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			l.Infof("volume '%s' not found, that's OK for list request", filesystemPath)
			return &response, nil
		}
		return nil, err
	}

	snapshots, err := nsProvider.GetSnapshots(filesystemPath, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot get snapshot list for '%s': %s", filesystemPath, err)
	}

	for i, snapshot := range snapshots {
		// skip all snapshots before startring token
		if snapshot.Path == startingToken {
			startingToken = ""
		}
		if startingToken != "" {
			continue
		}

		response.Entries = append(response.Entries, convertNSSnapshotToCSISnapshot(snapshot))

		// if the requested maximum is reached (and specified) than set next token
		if maxEntries != 0 && int32(len(response.Entries)) == maxEntries {
			if i+1 < len(snapshots) { // next snapshots index exists
				l.Infof(
					"max entries count (%d) has been reached while getting snapshots for '%s' filesystem, "+
						"send response with next_token for pagination",
					maxEntries,
					filesystemPath,
				)
				response.NextToken = snapshots[i+1].Path
				return &response, nil
			}
		}
	}

	l.Infof("found %d snapshot(s) for %s filesystem", len(response.Entries), filesystemPath)

	return &response, nil
}

func convertNSSnapshotToCSISnapshot(snapshot ns.Snapshot) *csi.ListSnapshotsResponse_Entry {
	return &csi.ListSnapshotsResponse_Entry{
		Snapshot: &csi.Snapshot{
			SnapshotId:     snapshot.Path,
			SourceVolumeId: snapshot.Parent,
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

	volumePath := req.GetVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "req.VolumeId must be provided")
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

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

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
	} else {
		datasetPath = s.config.DefaultDataset
	}

	nsProvider, err := s.resolveNS(datasetPath)
	if err != nil {
		return nil, err
	}

	filesystem, err := nsProvider.GetFilesystem(datasetPath)
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
	s.log.WithField("func", "ControllerExpandVolume()").Warnf("request: '%+v' - not implemented", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// NewControllerServer - create an instance of controller service
func NewControllerServer(driver *Driver) (*ControllerServer, error) {
	l := driver.log.WithField("cmp", "ControllerServer")
	l.Info("create new ControllerServer...")

	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:            driver.config.Address,
		Username:           driver.config.Username,
		Password:           driver.config.Password,
		Log:                l,
		InsecureSkipVerify: true, //TODO move to config
	})
	if err != nil {
		return nil, fmt.Errorf("Cannot create NexentaStor resolver: %s", err)
	}

	return &ControllerServer{
		nsResolver: nsResolver,
		config:     driver.config,
		log:        l,
	}, nil
}
