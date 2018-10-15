package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
)

// Name - driver name
var Name = "nexentastor-csi-plugin"

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
	Log      *logrus.Entry

	csiDriver *csiCommon.CSIDriver

	ids *csiCommon.DefaultIdentityServer
	cs  *ControllerServer
	ns  *NodeServer

	cap   []*csi.VolumeCapability_AccessMode
	cscap []*csi.ControllerServiceCapability
}

// Run - run the driver
func (d *Driver) Run() {
	d.Log.Info("run")

	csiCommon.RunControllerandNodePublishServer(
		d.Endpoint,
		d.csiDriver,
		NewControllerServer(d),
		NewNodeServer(d),
	)
}

// Args - params to crete new driver
type Args struct {
	NodeID   string
	Endpoint string
	Log      *logrus.Entry
}

// NewDriver - new driver instance
func NewDriver(args Args) *Driver {
	l := args.Log.WithField("cmp", "Driver")
	l.Infof("new %v@%v-%v (%v) driver has been created", Name, Version, Commit, DateTime)

	csiDriver := csiCommon.NewCSIDriver(Name, Version, args.NodeID)

	csiDriver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		},
	)

	csiDriver.AddVolumeCapabilityAccessModes(
		[]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
	)

	d := &Driver{
		Endpoint:  args.Endpoint,
		Log:       l,
		csiDriver: csiDriver,
	}

	return d
}
