package driver

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/container-storage-interface/spec/lib/go/csi"
	timestamp "github.com/golang/protobuf/ptypes/timestamp"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nexenta/nexentastor-csi-driver/pkg/config"
	"github.com/Nexenta/nexentastor-csi-driver/pkg/ns"
)

// supportedControllerCapabilities - driver controller capabilities
var supportedControllerCapabilities = []csi.ControllerServiceCapability_RPC_Type{
	csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
	csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT,
	//csi.ControllerServiceCapability_RPC_GET_CAPACITY, //TODO
	//csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS, //TODO
}

// supportedVolumeCapabilities - driver volume capabilities
var supportedVolumeCapabilities = []*csi.VolumeCapability{
	&csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_READER_ONLY},
	},
	&csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER},
	},
	&csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_READER_ONLY},
	},
	&csi.VolumeCapability{
		AccessMode: &csi.VolumeCapability_AccessMode{Mode: csi.VolumeCapability_AccessMode_MULTI_NODE_SINGLE_WRITER},
	},
	&csi.VolumeCapability{
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
	*csi.CreateVolumeResponse,
	error,
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

	nsProvider, err := s.resolveNS(datasetPath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, datasetPath)

	// use full path as volume ID
	volumePath := filepath.Join(datasetPath, volumeName)

	// get requested volume size from runtime params, set default if not specified
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()

	res := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			VolumeId:      volumePath,
			CapacityBytes: capacityBytes,
			VolumeContext: map[string]string{
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
	l := s.log.WithField("func", "CreateSnapshot()")
	l.Infof("request: '%+v'", req)

	err := s.refreshConfig()
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

	//TODO req.GetParameters() - read recursive param?

	nsProvider, err := s.resolveNS(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "volume '%s' not found, cannot take a snapshot: %s", volumePath, err)
	}

	l.Infof("resolved NS: %s, %s", nsProvider, volumePath)

	//K8s doesn't allow to have same named snapshots for different volumes
	parentVolumePath := filepath.Dir(volumePath)
	existingSnapshots, err := nsProvider.GetSnapshots(parentVolumePath, true)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot get snapshots list: %s", err)
	}
	for _, s := range existingSnapshots {
		if s.Name == name && s.Parent != volumePath {
			return nil, status.Errorf(
				codes.AlreadyExists,
				"Snapshot '%s' already exists for filesystem: %s",
				name,
				s.Path,
			)
		}
	}

	snapshotPath := fmt.Sprintf("%s@%s", volumePath, name)

	// if here, than volumePath exists on some NS
	err = nsProvider.CreateSnapshot(ns.CreateSnapshotParams{
		Path: snapshotPath,
	})
	if err != nil && !ns.IsAlreadyExistNefError(err) {
		return nil, status.Errorf(codes.Internal, "Cannot create snapshot '%s': %s", snapshotPath, err)
	}

	createdSnapshot, err := nsProvider.GetSnapshot(snapshotPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"Snapshot '%s' has been created, but snapshot properties request failed: %s",
			snapshotPath,
			err,
		)
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
			//SizeByte: 0 // size of zero means it is unspecified
		},
	}

	if ns.IsAlreadyExistNefError(err) {
		l.Infof("snapshot '%s' already exists and can be used", snapshotPath)
		return res, nil
	}

	l.Infof("snapshot '%s' has been created", snapshotPath)
	return res, nil
}

// DeleteSnapshot - not implemented
func (s *ControllerServer) DeleteSnapshot(ctx context.Context, req *csi.DeleteSnapshotRequest) (
	*csi.DeleteSnapshotResponse,
	error,
) {
	l := s.log.WithField("func", "DeleteSnapshot()")
	l.Infof("request: '%+v'", req)

	err := s.refreshConfig()
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
		// codes.FailedPrecondition error means no NS found with this volumePath - that's OK
		if status.Code(err) == codes.FailedPrecondition {
			l.Infof("snapshot '%s' not found, that's OK for deletion request", snapshotPath)
			return &csi.DeleteSnapshotResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %s, %s", nsProvider, snapshotPath)

	// if here, than volumePath exists on some NS
	err = nsProvider.DestroySnapshot(snapshotPath)
	if err != nil && !ns.IsNotExistNefError(err) {
		return nil, status.Errorf(
			codes.Internal,
			"Cannot delete '%s' snapshot: %s",
			snapshotPath,
			err,
		)
	}

	l.Infof("snapshot '%s' has been deleted", snapshotPath)
	return &csi.DeleteSnapshotResponse{}, nil
}

// ListSnapshots - list of snapshots
func (s *ControllerServer) ListSnapshots(ctx context.Context, req *csi.ListSnapshotsRequest) (
	*csi.ListSnapshotsResponse,
	error,
) {
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities - validate volume capabilities, shall return confirmed only if all the volume
// capabilities specified in the request are supported
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
		return nil, status.Errorf(codes.NotFound, "Cannot find volume '%s': %s", volumePath, err)
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
	requestedMode := requestedVolumeCapability.GetAccessMode().GetMode()
	for _, volumeCapability := range supportedVolumeCapabilities {
		if volumeCapability.GetAccessMode().GetMode() == requestedMode {
			return true
		}
	}
	return false
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
