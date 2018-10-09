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

func (cs *ControllerServer) resolveNS(cfg *config.Config, datasetPath string) (ns.ProviderInterface, error) {
	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:  cfg.Address,
		Username: cfg.Username,
		Password: cfg.Password,
		Log:      cs.Log,
	})
	if err != nil {
		return nil, fmt.Errorf("Cannot create NexentaStor resolver: %v", err)
	}

	nsProvider, err := nsResolver.Resolve(datasetPath)
	if err != nil {
		return nil, fmt.Errorf("Cannot resolve NexentaStor: %v", err)
	}

	return nsProvider, nil
}

// CreateVolume - creates FS on NexentaStor
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse,
	error,
) {
	cs.Log.Infof("CreateVolume(): %+v", req)

	cfg, err := config.Get()
	if err != nil {
		return nil, fmt.Errorf("Cannot use config file: %v", err)
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

	nsProvider, err := cs.resolveNS(cfg, datasetPath)
	if err != nil {
		return nil, err
	}

	res := &csi.CreateVolumeResponse{
		Volume: &csi.Volume{
			Id:            volumePath,
			CapacityBytes: capacityBytes,
			Attributes:    req.GetParameters(),
		},
	}

	err = nsProvider.CreateFilesystem(volumePath, nefParams)
	if err == nil {
		return res, nil
	} else if ns.IsAlreadyExistNefError(err) {
		existingVolume, err := nsProvider.GetFilesystem(volumePath)
		if err != nil {
			return nil, fmt.Errorf(
				"Volume '%v' already exists, but filesystem properties request failed: %v",
				volumePath,
				err,
			)
		} else if existingVolume.QuotaSize != capacityBytes {
			return nil, fmt.Errorf(
				"Volume '%v' already exists, but with a different size: requested=%v, existing=%v",
				volumePath,
				capacityBytes,
				existingVolume.QuotaSize,
			)
		}
		cs.Log.Infof("Volume '%v' already exists and can be used", volumePath)
		return res, nil
	}

	return nil, err
}

// DeleteVolume - destroys FS on NexentaStor
func (cs *ControllerServer) DeleteVolume(ctx context.Context, req *csi.DeleteVolumeRequest) (
	*csi.DeleteVolumeResponse,
	error,
) {
	cs.Log.Infof("DeleteVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerPublishVolume - publish volume
func (cs *ControllerServer) ControllerPublishVolume(ctx context.Context, req *csi.ControllerPublishVolumeRequest) (
	*csi.ControllerPublishVolumeResponse,
	error,
) {
	cs.Log.Infof("ControllerPublishVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ControllerUnpublishVolume - unpublish volume
func (cs *ControllerServer) ControllerUnpublishVolume(ctx context.Context, req *csi.ControllerUnpublishVolumeRequest) (
	*csi.ControllerUnpublishVolumeResponse,
	error,
) {
	cs.Log.Infof("ControllerUnpublishVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ListVolumes - list volumes
func (cs *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (
	*csi.ListVolumesResponse,
	error,
) {
	cs.Log.Infof("ListVolumes(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// ValidateVolumeCapabilities - validate volume capabilities
func (cs *ControllerServer) ValidateVolumeCapabilities(ctx context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (
	*csi.ValidateVolumeCapabilitiesResponse,
	error,
) {
	for _, cap := range req.VolumeCapabilities {
		if cap.GetBlock() != nil {
			return &csi.ValidateVolumeCapabilitiesResponse{Supported: false, Message: ""}, nil
		}
	}
	return &csi.ValidateVolumeCapabilitiesResponse{Supported: true}, nil
}

// NewControllerServer - create an instance of controller service
func NewControllerServer(driver *Driver) *ControllerServer {
	nodeServerLog := driver.Log.WithFields(logrus.Fields{
		"cmp": "ControllerServer",
	})

	nodeServerLog.Info("New ControllerServer is created")

	return &ControllerServer{
		DefaultControllerServer: csiCommon.NewDefaultControllerServer(driver.csiDriver),
		Log:                     nodeServerLog,
	}
}
