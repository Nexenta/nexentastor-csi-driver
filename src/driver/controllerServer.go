package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ControllerServer - k8s csi driver controller server
type ControllerServer struct {
	*csiCommon.DefaultControllerServer

	Log *logrus.Entry
}

// CreateVolume - creates FS on NexentaStor
func (cs *ControllerServer) CreateVolume(ctx context.Context, req *csi.CreateVolumeRequest) (
	*csi.CreateVolumeResponse,
	error,
) {
	cs.Log.Infof("CreateVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
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
