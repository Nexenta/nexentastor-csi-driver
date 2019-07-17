package driver

import (
	"fmt"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
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
	//csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT, //TODO
	//csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS, //TODO
	//csi.ControllerServiceCapability_RPC_CLONE_VOLUME, //TODO
	//csi.ControllerServiceCapability_RPC_GET_CAPACITY, //TODO
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

func (s *ControllerServer) refreshConfig() error {
	changed, err := s.config.Refresh()
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
	l.Infof("request: '%+v'", req)

	err := s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
	}

	nsProvider, err := s.resolveNS(s.config.DefaultDataset)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, s.config.DefaultDataset)

	filesystems, err := nsProvider.GetFilesystems(s.config.DefaultDataset)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot get filesystems: %s", err)
	}

	entries := make([]*csi.ListVolumesResponse_Entry, len(filesystems))
	for i, item := range filesystems {
		entries[i] = &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{VolumeId: item.Path},
		}
	}

	l.Infof("found %d entries(s)", len(entries))

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// CreateVolume - creates FS on NexentaStor
func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	res *csi.CreateVolumeResponse,
	err error,
) {
	l := s.log.WithField("func", "CreateVolume()")
	l.Infof("request: '%+v'", req)

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

	err = s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
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

	return s.createNewVolume(nsProvider, volumePath, capacityBytes, res)
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
			} else if existingFilesystem.GetReferencedQuotaSize() != capacityBytes {
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

// DeleteVolume - destroys FS on NexentaStor
func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse,
	error,
) {
	l := s.log.WithField("func", "DeleteVolume()")
	l.Infof("request: '%+v'", req)

	err := s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
	}

	volumePath := req.GetVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			l.Infof("volume '%s' not found, that's OK for deletion request", volumePath)
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

	// if here, than volumePath exists on some NS
	err = nsProvider.DestroyFilesystemWithClones(volumePath, false)
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

// CreateSnapshot creates a snapshot of given volume
func (s *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse,
	error,
) {
	s.log.WithField("func", "CreateSnapshot()").Warnf("request: '%+v' - not implemented", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot deletes snapshots
func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse,
	error,
) {
	s.log.WithField("func", "DeleteSnapshot()").Warnf("request: '%+v' - not implemented", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots returns the list of snapshots
func (s *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (
	*csi.ListSnapshotsResponse,
	error,
) {
	s.log.WithField("func", "ListSnapshots()").Warnf("request: '%+v' - not implemented", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities validates volume capabilities
// Shall return confirmed only if all the volume
// capabilities specified in the request are supported.
func (s *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse,
	error,
) {
	l := s.log.WithField("func", "ValidateVolumeCapabilities()")
	l.Infof("request: '%+v'", req)

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

	err := s.refreshConfig()
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

// GetCapacity - not implemented
func (s *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse,
	error,
) {
	s.log.WithField("func", "GetCapacity()").Warnf("request: '%+v' - not implemented", req)
	return nil, status.Error(codes.Unimplemented, "")
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
