package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// NodeServer - k8s csi driver node server
type NodeServer struct {
	*csiCommon.DefaultNodeServer

	Log *logrus.Entry
}

// NodeGetId - returns node id where pod is running
func (ns *NodeServer) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (
	*csi.NodeGetIdResponse,
	error,
) {
	ns.Log.Infof("NodeGetId(): %+v", req)
	return ns.DefaultNodeServer.NodeGetId(ctx, req)
}

// NodeGetCapabilities - get node capabilities
func (ns *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse,
	error,
) {
	ns.Log.Infof("NodeGetCapabilities(): %+v", req)
	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: []*csi.NodeServiceCapability{
			{
				Type: &csi.NodeServiceCapability_Rpc{
					Rpc: &csi.NodeServiceCapability_RPC{
						Type: csi.NodeServiceCapability_RPC_UNKNOWN,
					},
				},
			},
		},
	}, nil
}

// NodePublishVolume - mounts NS fs to the node
func (ns *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse,
	error,
) {
	ns.Log.Infof("NodePublishVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
	//return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume - umount NS fs from the node
func (ns *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse,
	error,
) {
	ns.Log.Infof("NodePublishVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
	//return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeStageVolume - stage volume
func (ns *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse,
	error,
) {
	ns.Log.Infof("NodeStageVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume - unstage volume
func (ns *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse,
	error,
) {
	ns.Log.Infof("NodeUnstageVolume(): %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// NewNodeServer - create an instance of node service
func NewNodeServer(driver *Driver) *NodeServer {
	nodeServerLog := driver.Log.WithFields(logrus.Fields{
		"cmp": "NodeServer",
	})

	nodeServerLog.Info("New NodeServer is created")

	return &NodeServer{
		DefaultNodeServer: csiCommon.NewDefaultNodeServer(driver.csiDriver),
		Log:               nodeServerLog,
	}
}
