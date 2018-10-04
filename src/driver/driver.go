package driver

import (
	"github.com/sirupsen/logrus"
)

const DriverName = "nexentastor-csi-plugin"

var (
	version string
	commit  string
)

// GetVersion - to set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/driver/driver.version=0.0.1"
func GetVersion() string {
	if version == "" {
		return "-"
	}
	return version
}

// GetCommit - to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/driver/driver.commit=asdf"
func GetCommit() string {
	if commit == "" {
		return "-"
	}
	return commit
}

// Driver - K8s CSI driver for NexentaStor
type Driver struct {
	Endpoint string
	Log      *logrus.Entry
}

// Run - run the driver
func (d *Driver) Run() {
	d.Log.Warn("Run")
}

// NewDriver - new driver instance
func NewDriver(nodeID string, endpoint string, log *logrus.Entry) *Driver {
	driverLog := log.WithFields(logrus.Fields{
		"cmp": "Driver",
	})

	driverLog.Infof("New '%v' driver created", DriverName)

	d := &Driver{
		Endpoint: endpoint,
		Log:      driverLog,
	}

	return d
}
