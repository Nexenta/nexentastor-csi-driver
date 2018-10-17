package driver

import (
	"fmt"
	"os"
	"strings"

	"github.com/Nexenta/nexentastor-csi-driver/src/arrays"
	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/kubernetes/pkg/util/mount"
)

// NodeServer - k8s csi driver node server
type NodeServer struct {
	*csiCommon.DefaultNodeServer

	Log *logrus.Entry
}

func (s *NodeServer) resolveNS(cfg *config.Config, datasetPath string) (ns.ProviderInterface, error) {
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

// NodeGetId - returns node id where pod is running
func (s *NodeServer) NodeGetId(ctx context.Context, req *csi.NodeGetIdRequest) (
	*csi.NodeGetIdResponse,
	error,
) {
	s.Log.WithField("func", "NodeGetId()").Infof("request: %+v", req)
	return s.DefaultNodeServer.NodeGetId(ctx, req)
}

// NodeGetCapabilities - get node capabilities
func (s *NodeServer) NodeGetCapabilities(ctx context.Context, req *csi.NodeGetCapabilitiesRequest) (
	*csi.NodeGetCapabilitiesResponse,
	error,
) {
	s.Log.WithField("func", "NodeGetCapabilities()").Infof("request: %+v", req)
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
func (s *NodeServer) NodePublishVolume(ctx context.Context, req *csi.NodePublishVolumeRequest) (
	*csi.NodePublishVolumeResponse,
	error,
) {
	l := s.Log.WithField("func", "NodePublishVolume()")
	l.Infof("request: %+v", req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path must be provided")
	}

	aclRuleSet := ns.ACLReadWrite
	mountOptions := []string{} //TODO look up config for NFS share options
	if req.GetReadonly() {
		aclRuleSet = ns.ACLReadOnly
		if !arrays.StringsContains(mountOptions, "ro") {
			mountOptions = append(mountOptions, "ro")
		}
	}

	cfg, err := config.Get()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	nsProvider, err := s.resolveNS(cfg, volumeID)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, volumeID)

	filesystem, err := nsProvider.GetFilesystem(volumeID)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"Cannot get filesystem '%v' volume: %v",
			volumeID,
			err,
		)
	} else if filesystem == nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot find filesystem '%v'", volumeID)
	}

	if !filesystem.SharedOverNfs {
		// create share
		err = nsProvider.CreateNfsShare(filesystem.Path)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Cannot share filesystem '%v': %v", filesystem.Path, err)
		}
	}

	// apply filesystem acl (overwrites every time)
	err = nsProvider.SetFilesystemACL(filesystem.Path, aclRuleSet)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Cannot set filesystem ACL for '%v': %v", filesystem.Path, err)
	}

	mounter := mount.New("")
	notMountPoint, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.MkdirAll(targetPath, 0750); err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"Failed to mkdir to share target path '%v': %v",
					targetPath,
					err,
				)
			}
			notMountPoint = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				"Cannot ensure that target path '%v' can be used as a mount point: %v",
				targetPath,
				err,
			)
		}
	}

	if !notMountPoint { // already mounted
		return nil, status.Errorf(codes.Internal, "Target path '%v' is already a mount point", targetPath)
	}

	nfsEndpoint := fmt.Sprintf("%v:%v", cfg.DefaultDataIP, filesystem.MountPoint)

	l.Infof(
		"mount params: targetPath: '%v', nfsEndpoint: '%v', fsType: '%v', "+
			"readOnly: '%v', volumeAttributes: '%v', mountFlags '%v'",
		targetPath,
		nfsEndpoint,
		req.GetVolumeCapability().GetMount().GetFsType(),
		req.GetReadonly(),
		req.GetVolumeAttributes(),
		req.GetVolumeCapability().GetMount().GetMountFlags(),
	)

	err = mounter.Mount(nfsEndpoint, targetPath, "nfs", mountOptions)
	if err != nil {
		if os.IsPermission(err) {
			return nil, status.Errorf(
				codes.PermissionDenied,
				"Permission denied to mount '%v' to '%v': %v",
				nfsEndpoint,
				targetPath,
				err,
			)
		} else if strings.Contains(err.Error(), "invalid argument") {
			return nil, status.Errorf(
				codes.InvalidArgument,
				"Cannot mount '%v' to '%v', invalid argument: %v",
				nfsEndpoint,
				targetPath,
				err,
			)
		}
		return nil, status.Errorf(
			codes.Internal,
			"Failed to mount '%v' to '%v': %v",
			nfsEndpoint,
			targetPath,
			err,
		)
	}

	l.Infof("volume '%v' has beed published to '%v'", volumeID, targetPath)
	return &csi.NodePublishVolumeResponse{}, nil
}

// NodeUnpublishVolume - umount NS fs from the node and delete directory if successful
func (s *NodeServer) NodeUnpublishVolume(ctx context.Context, req *csi.NodeUnpublishVolumeRequest) (
	*csi.NodeUnpublishVolumeResponse,
	error,
) {
	l := s.Log.WithField("func", "NodeUnpublishVolume()")
	l.Infof("request: %+v", req)

	volumeID := req.GetVolumeId()
	if len(volumeID) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	targetPath := req.GetTargetPath()
	if len(targetPath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Target path must be provided")
	}

	mounter := mount.New("")

	if err := mounter.Unmount(targetPath); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to unmount target path '%v': %v", targetPath, err)
	}

	notMountPoint, err := mounter.IsLikelyNotMountPoint(targetPath)
	if err != nil {
		if os.IsNotExist(err) {
			l.Warnf("mount point '%v' already doesn't exist: '%v', return OK", targetPath, err)
			return &csi.NodeUnpublishVolumeResponse{}, nil
		}
		return nil, status.Errorf(
			codes.Internal,
			"Cannot ensure that target path '%v' is a mount point: '%v'",
			targetPath,
			err,
		)
	} else if !notMountPoint { // still mounted
		return nil, status.Errorf(codes.Internal, "Target path '%v' is still mounted", targetPath)
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return nil, status.Errorf(codes.Internal, "Cannot remove unmounted target path '%v': %v", targetPath, err)
	}

	l.Infof("volume '%v' has beed unpublished from '%v'", volumeID, targetPath)
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

// NodeStageVolume - stage volume
func (s *NodeServer) NodeStageVolume(ctx context.Context, req *csi.NodeStageVolumeRequest) (
	*csi.NodeStageVolumeResponse,
	error,
) {
	s.Log.WithField("func", "NodeStageVolume()").Infof("request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// NodeUnstageVolume - unstage volume
func (s *NodeServer) NodeUnstageVolume(ctx context.Context, req *csi.NodeUnstageVolumeRequest) (
	*csi.NodeUnstageVolumeResponse,
	error,
) {
	s.Log.WithField("func", "NodeUnstageVolume()").Infof("request: %+v", req)
	return nil, status.Error(codes.Unimplemented, "")
}

// NewNodeServer - create an instance of node service
func NewNodeServer(driver *Driver) *NodeServer {
	l := driver.Log.WithField("cmp", "NodeServer")

	l.Info("new NodeServer has been created")

	return &NodeServer{
		DefaultNodeServer: csiCommon.NewDefaultNodeServer(driver.csiDriver),
		Log:               l,
	}
}
