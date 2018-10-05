package driver

import (
	"github.com/sirupsen/logrus"
)

// Name - driver name
var Name = "nexentastor-csi-plugin"

// Version - driver version, to set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Version=0.0.1"
var Version string

// Commit - driver last commit, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Commit=asdf"
var Commit string

// Driver - K8s CSI driver for NexentaStor
type Driver struct {
	Endpoint string
	Log      *logrus.Entry
}

// Run - run the driver
func (d *Driver) Run() {
	d.Log.Warn("Run")
}

// Args - params to crete new driver
type Args struct {
	NodeID   string
	Endpoint string
	Log      *logrus.Entry
}

// NewDriver - new driver instance
func NewDriver(args Args) *Driver {
	driverLog := args.Log.WithFields(logrus.Fields{
		"cmp": "Driver",
	})

	driverLog.Infof("New %v@%v-%v driver created", Name, Version, Commit)

	d := &Driver{
		Endpoint: args.Endpoint,
		Log:      driverLog,
	}

	return d
}
