package driver

import (
	"fmt"
	"path/filepath"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	defaultFilesystemSize int64 = 1024 * 1024 * 1024 // 1Gb
)

// ControllerServer - k8s csi driver controller server
type ControllerServer struct {
	*csiCommon.DefaultControllerServer

	Log *logrus.Entry
}

func (s *ControllerServer) resolveNS(cfg *config.Config, datasetPath string) (ns.ProviderInterface, error) {
	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		Log:      s.Log,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot create NexentaStor resolver: %v", err)
	}

	nsProvider, err := nsResolver.Resolve(datasetPath)
	if err != nil {
		return nil, status.Errorf(
			codes.FailedPrecondition,
			"Cannot resolve '%v' on any NexentaStor(s): %v",
			datasetPath,
			err,
		)
	}

	return nsProvider, nil
}

// ListVolumes - list volumes
func (s *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (
	*csi.ListVolumesResponse,
	error,
) {
	l := s.Log.WithField("func", "ListVolumes()")
	l.Infof("request: %+v", req)

	cfg, err := config.Get()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	nsProvider, err := s.resolveNS(cfg, cfg.DefaultDataset)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, cfg.DefaultDataset)

	filesystems, err := nsProvider.GetFilesystems(cfg.DefaultDataset)
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot get filesystems: %v", err)
	}

	entries := make([]*csi.ListVolumesResponse_Entry, len(filesystems))
	for i, item := range filesystems {
		entries[i] = &csi.ListVolumesResponse_Entry{
			Volume: &csi.Volume{Id: item.Path},
		}
	}

	l.Infof("found %v entries(s)", len(entries))

	return &csi.ListVolumesResponse{
		Entries: entries,
	}, nil
}

// CreateVolume - creates FS on NexentaStor
func (s *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse,
	error,
) {
	l := s.Log.WithField("func", "CreateVolume()")
	l.Infof("request: %+v", req)

	cfg, err := config.Get()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	reqParams := req.GetParameters()
	if reqParams == nil {
		reqParams = make(map[string]string)
	}

	nefParams := make(map[string]interface{})

	// get dataset path from runtime params, set default if not specified
	datasetPath := ""
	if dataset, ok := reqParams["dataset"]; ok {
		datasetPath = dataset
	} else {
		datasetPath = cfg.DefaultDataset
	}

	// get volume name from runtime params, generate uuid if not specified
	volumeName := req.GetName()
	if len(volumeName) == 0 {
		volumeName = fmt.Sprintf("csi-volume-%v", uuid.NewUUID().String())
	}

	volumePath := filepath.Join(datasetPath, volumeName)

	// get requested volume size from runtime params, set default if not specified
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if capacityBytes == 0 {
		capacityBytes = defaultFilesystemSize
	}
	nefParams["quotaSize"] = capacityBytes

	nsProvider, err := s.resolveNS(cfg, datasetPath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, datasetPath)

	res := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumePath,
			CapacityBytes: capacityBytes,
		},
	}

	err = nsProvider.CreateFilesystem(volumePath, nefParams)
	if err == nil {
		l.Infof("volume '%v' has been created", volumePath)
		return res, nil
	} else if ns.IsAlreadyExistNefError(err) {
		existingVolume, err := nsProvider.GetFilesystem(volumePath)
		if err != nil {
			return nil, status.Errorf(
				codes.AlreadyExists,
				"Volume '%v' already exists, but volume properties request failed: %v",
				volumePath,
				err,
			)
		} else if existingVolume.QuotaSize != capacityBytes {
			return nil, status.Errorf(
				codes.AlreadyExists,
				"Volume '%v' already exists, but with a different size: requested=%v, existing=%v",
				volumePath,
				capacityBytes,
				existingVolume.QuotaSize,
			)
		}
		l.Infof("volume '%v' already exists and can be used", volumePath)
		return res, nil
	}

	return nil, err
}

// DeleteVolume - destroys FS on NexentaStor
func (s *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse,
	error,
) {
	l := s.Log.WithField("func", "DeleteVolume()")
	l.Infof("request: %+v", req)

	cfg, err := config.Get()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	volumePath := req.GetVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	nsProvider, err := s.resolveNS(cfg, volumePath)
	if err != nil {
		// codes.FailedPrecondition error means no NS found with this volumePath - that's OK
		if status.Code(err) == codes.FailedPrecondition {
			l.Infof("volume '%v' not found, that's OK for deletion request", volumePath)
			return &csi.DeleteVolumeResponse{}, nil
		}
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, volumePath)

	// if here, than volumePath exists on some NS
	err = nsProvider.DestroyFilesystem(volumePath)
	if err != nil && !ns.IsNotExistNefError(err) {
		return nil, status.Errorf(
			codes.Internal,
			"Cannot delete '%v' volume: %v",
			volumePath,
			err,
		)
	}

	l.Infof("volume '%v' has been deleted", volumePath)
	return &csi.DeleteVolumeResponse{}, nil
}

// ControllerPublishVolume - publish volume
func (s *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse,
	error,
) {
	s.Log.WithField("func", "ControllerPublishVolume()").Infof("request: %+v", req)
	return &csi.ControllerPublishVolumeResponse{}, nil
}

// ControllerUnpublishVolume - unpublish volume
func (s *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse,
	error,
) {
	s.Log.WithField("func", "ControllerUnpublishVolume()").Infof("request: %+v", req)
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

// ValidateVolumeCapabilities - validate volume capabilities (only mount is supported)
func (s *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse,
	error,
) {
	supported := true
	for _, cap := range req.VolumeCapabilities {
		if cap.GetBlock() != nil {
			supported = false
		}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{Supported: supported}, nil
}

// NewControllerServer - create an instance of controller service
func NewControllerServer(driver *Driver) *ControllerServer {
	l := driver.Log.WithField("cmp", "ControllerServer")

	l.Info("new ControllerServer has been created")

	return &ControllerServer{
		DefaultControllerServer: csiCommon.NewDefaultControllerServer(driver.csiDriver),
		Log:                     l,
	}
}
