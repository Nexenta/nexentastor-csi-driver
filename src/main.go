package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/driver"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"
)

const (
	defaultEndpoint = "unix:///var/lib/kubelet/plugins/com.nexenta.nexentastor-csi-driver/csi.sock"
)

func main() {
	var (
		nodeID   = flag.String("nodeid", "", "Kubernetes node ID")
		endpoint = flag.String("endpoint", defaultEndpoint, "CSI endpoint")
		version  = flag.Bool("version", false, "Print driver version")
	)

	flag.Parse()

	if *version {
		fmt.Printf("%v@%v-%v (%v)\n", driver.Name, driver.Version, driver.Commit, driver.DateTime)
		os.Exit(0)
	}

	// init logger
	l := logrus.New().WithField("cmp", "Main")

	// logger formater
	l.Logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"cmp", "ns", "func", "req", "reqID", "job"},
	})

	l.Info("Start driver with CLI options:")
	l.Infof("- CSI endpoint:    '%v'", *endpoint)
	l.Infof("- Node ID:         '%v'", *nodeID)

	// initial config file validation
	cfg, err := config.Get()
	if err != nil {
		l.Fatalf("Cannot use config file: %v", err)
	}

	// logger level
	if cfg.Debug {
		l.Logger.SetLevel(logrus.DebugLevel)
	} else {
		l.Logger.SetLevel(logrus.InfoLevel)
	}

	l.Info("Config file options:")
	l.Infof("- NexentaStor address: %v", cfg.Address)
	l.Infof("- NexentaStor username: %v", cfg.Username)
	l.Infof("- Default dataset: %v", cfg.DefaultDataset)
	l.Infof("- Default data IP: %v", cfg.DefaultDataIP)

	d := driver.NewDriver(driver.Args{
		NodeID:   *nodeID,
		Endpoint: *endpoint,
		Log:      l,
	})

	d.Run()
}
