package driver

import (
	"fmt"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

// Name - driver name
var Name = "com.nexenta.nexentastor-csi-driver"

// Version - driver version, to set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Version=0.0.1"
var Version string

// Commit - driver last commit, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Commit=..."
var Commit string

// DateTime - driver build datetime, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.DateTime=..."
var DateTime string

// Driver - K8s CSI driver for NexentaStor
type Driver struct {
	Endpoint string
	Config   *config.Config
	Log      *logrus.Entry

	csiDriver *csiCommon.CSIDriver
}

// Run - run the driver
func (d *Driver) Run() {
	d.Log.Info("run")

	grpcServer := csiCommon.NewNonBlockingGRPCServer()

	grpcServer.Start(
		d.Endpoint,
		NewIdentityServer(d),
		NewControllerServer(d),
		NewNodeServer(d),
	)

	grpcServer.Wait()
}

// Validate - validate driver configuration:
// - check NS connection
// - check NS license
func (d *Driver) Validate() error {
	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:  d.Config.Address,
		Username: d.Config.Username,
		Password: d.Config.Password,
		Log:      d.Log,
	})
	if err != nil {
		return fmt.Errorf("Driver validation failed, cannot create NexentaStor(s) resolver: %v", err)
	}

	for _, nsProvider := range nsResolver.Nodes {
		license, err := nsProvider.GetLicense()
		if err != nil {
			return fmt.Errorf("Driver validation failed: %v", err)
		} else if !license.Valid {
			return fmt.Errorf(
				"Driver validation failed, NexentaStor %v has invalid license (expired: %v)",
				nsProvider,
				license.Expires,
			)
		}
	}

	return nil
}

// Args - params to crete new driver
type Args struct {
	NodeID   string
	Endpoint string
	Config   *config.Config
	Log      *logrus.Entry
}

// NewDriver - new driver instance
func NewDriver(args Args) (*Driver, error) {
	l := args.Log.WithField("cmp", "Driver")

	if args.Config == nil {
		return nil, fmt.Errorf("args.Config is required")
	} else if args.Log == nil {
		return nil, fmt.Errorf("args.Log is required")
	}

	l.Infof("new %v@%v-%v (%v) driver has been created", Name, Version, Commit, DateTime)

	csiDriver := csiCommon.NewCSIDriver(Name, Version, args.NodeID)

	csiDriver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			//csi.ControllerServiceCapability_RPC_GET_CAPACITY, //TODO
			//csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS, //TODO
			//csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT, //TODO
		},
	)

	csiDriver.AddVolumeCapabilityAccessModes(
		[]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
	)

	d := &Driver{
		Endpoint:  args.Endpoint,
		Config:    args.Config,
		Log:       l,
		csiDriver: csiDriver,
	}

	return d, nil
}
