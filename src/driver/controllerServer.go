package driver

import (
	"fmt"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common" //TODO get rid of it
	"github.com/pborman/uuid"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

const (
	defaultFilesystemSize int64 = 1024 * 1024 * 1024 // 1Gb
)

// ControllerServer - k8s csi driver controller server
type ControllerServer struct {
	*csiCommon.DefaultControllerServer

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
			return fmt.Errorf("Cannot create NexentaStor resolver: %v", err)
		}
	}

	return nil
}

func (s *ControllerServer) resolveNS(datasetPath string) (ns.ProviderInterface, error) {
	nsProvider, err := s.nsResolver.Resolve(datasetPath)
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

// ListVolumes - list volumes, shows only volumes created in defaultDataset
func (s *ControllerServer) ListVolumes(ctx context.Context, req *csi.ListVolumesRequest) (
	*csi.ListVolumesResponse,
	error,
) {
	l := s.log.WithField("func", "ListVolumes()")
	l.Infof("request: %+v", req)

	err := s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	nsProvider, err := s.resolveNS(s.config.DefaultDataset)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, s.config.DefaultDataset)

	filesystems, err := nsProvider.GetFilesystems(s.config.DefaultDataset)
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
	l := s.log.WithField("func", "CreateVolume()")
	l.Infof("request: %+v", req)

	err := s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
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
		volumeName = fmt.Sprintf("csi-volume-%v", uuid.NewUUID().String())
	}

	volumePath := filepath.Join(datasetPath, volumeName)

	// get requested volume size from runtime params, set default if not specified
	capacityBytes := req.GetCapacityRange().GetRequiredBytes()
	if capacityBytes == 0 {
		capacityBytes = defaultFilesystemSize
	}

	nsProvider, err := s.resolveNS(datasetPath)
	if err != nil {
		return nil, err
	}

	l.Infof("resolved NS: %v, %v", nsProvider, datasetPath)

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
		Path:      volumePath,
		QuotaSize: capacityBytes,
	})
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
	l := s.log.WithField("func", "DeleteVolume()")
	l.Infof("request: %+v", req)

	err := s.refreshConfig()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	volumePath := req.GetVolumeId()
	if len(volumePath) == 0 {
		return nil, status.Error(codes.InvalidArgument, "Volume ID must be provided")
	}

	nsProvider, err := s.resolveNS(volumePath)
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
		return nil, fmt.Errorf("Cannot create NexentaStor resolver: %v", err)
	}

	return &ControllerServer{
		DefaultControllerServer: csiCommon.NewDefaultControllerServer(driver.csiDriver),
		nsResolver:              nsResolver,
		config:                  driver.config,
		log:                     l,
	}, nil
}
