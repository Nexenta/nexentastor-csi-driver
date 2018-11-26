package driver

import (
	"fmt"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

// supportedControllerCapabilities - driver controller capabilities
var supportedControllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	//csi.ControllerServiceCapability_RPC_GET_CAPACITY, //TODO
	//csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS, //TODO
	//csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT, //TODO
}

// supportedVolumeCapabilityAccessModes - driver volume capabilities
var supportedVolumeCapabilityAccessModes = []csi.VolumeCapability_AccessMode{
	csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER,
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
		s.nsResolver, err = ns.NewResolver(ns.ResolverArgs{
			Address:  s.config.Address,
			Username: s.config.Username,
			Password: s.config.Password,
			Log:      s.log,
		})
		if err != nil {
			return fmt.Errorf("Cannot create NexentaStor resolver: %s", err)
		}
	}

	return nil
}

func (s *ControllerServer) resolveNS(datasetPath string) (ns.ProviderInterface, error) {
	nsProvider, err := s.nsResolver.Resolve(datasetPath)
	if err != nil { //TODO check not found error
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"Cannot resolve '%s' on any NexentaStor(s): %s",
			datasetPath,
			err,
		)
	}
	return nsProvider, nil
}

// ListVolumes - list volumes, shows only volumes created in defaultDataset
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
			Volume: &csi.Volume{Id: item.Path},
		}
	}

	l.Infof("found %d entries(s)", len(entries))

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// CreateVolume - creates FS on NexentaStor
func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse,
	error,
) {
	l := s.log.WithField("func", "CreateVolume()")
	l.Infof("request: '%+v'", req)

	err := s.refreshConfig()
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

	// get volume name from runtime params, generate uuid if not specified
	volumeName := req.GetName()
	if len(volumeName) == 0 {
		volumeName = fmt.Sprintf("csi-volume-%s", uuid.NewUUID().String())
	}

	volumePath := filepath.Join(datasetPath, volumeName)

	// get requested volume size from runtime params, set default if not specified
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()

	nsProvider, err := s.resolveNS(datasetPath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, datasetPath)

	res := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumePath,
			CapacityBytes: capacityBytes,
			Attributes: map[string]string{
				"dataIp":       reqParams["dataIp"],
				"mountOptions": reqParams["mountOptions"],
			},
		},
	}

	err = nsProvider.CreateFilesystem(ns.CreateFilesystemParams{
		Path:                volumePath,
		ReferencedQuotaSize: capacityBytes,
	})
	if err == nil {
		l.Infof("volume '%s' has been created", volumePath)
		return res, nil
	} else if ns.IsAlreadyExistNefError(err) {
		existingFilesystem, err := nsProvider.GetFilesystem(volumePath)
		if err != nil {
			return nil, status.Errorf(
				codes.AlreadyExists,
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

	return nil, err
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
		// codes.FailedPrecondition error means no NS found with this volumePath - that's OK
		if status.Code(err) == codes.FailedPrecondition {
			l.Infof("volume '%s' not found, that's OK for deletion request", volumePath)
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

	// if here, than volumePath exists on some NS
	err = nsProvider.DestroyFilesystem(volumePath)
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

// GetCapacity - not implemented
func (s *ControllerServer) GetCapacity(ctx context.Context, req *csi.GetCapacityRequest) (
	*csi.GetCapacityResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume - not implemented
func (s *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume - not implemented
func (s *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// CreateSnapshot - not implemented
func (s *ControllerServer) CreateSnapshot(ctx context.Context, req *csi.CreateSnapshotRequest) (
	*csi.CreateSnapshotResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// DeleteSnapshot - not implemented
func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ListSnapshots - not implemented
func (s *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (
	*csi.ListSnapshotsResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities - validate volume
func (s *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse,
	error,
) {
	l := s.log.WithField("func", "ValidateVolumeCapabilities()")
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
		return nil, status.Errorf(codes.NotFound, "Cannot find volume '%s': %s", volumePath, err)
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

	for _, reqC := range req.GetVolumeCapabilities() {
		reqMode := reqC.GetAccessMode().GetMode()
		found := false
		for _, volumeC := range supportedVolumeCapabilityAccessModes {
			if volumeC.GetMode() == reqMode {
				found = true
			}
		}
		l.Infof("requested capability: '%s', supported: %t", reqMode, found)
		if !found {
			message := fmt.Sprintf("Driver does not support mode: %s", reqMode)
			return &csi.ValidateVolumeCapabilitiesResponse{
				Supported: false,
				Message:   message,
			}, status.Error(codes.InvalidArgument, message)
		}
	}

	return &csi.ValidateVolumeCapabilitiesResponse{
		Supported: true,
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

// NewControllerServer - create an instance of controller service
func NewControllerServer(driver *Driver) (*ControllerServer, error) {
	l := driver.log.WithField("cmp", "ControllerServer")
	l.Info("create new ControllerServer...")

	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:  driver.config.Address,
		Username: driver.config.Username,
		Password: driver.config.Password,
		Log:      l,
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
