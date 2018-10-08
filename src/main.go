package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/driver"
)

const (
	defaultEndpoint = "unix:///var/lib/kubelet/plugins/com.nexenta.nexentastor-csi-plugin/csi.sock"
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
	log := logrus.New().WithFields(logrus.Fields{
		//"nodeId":    *nodeID,
		"cmp": "Main",
	})

	// logger level (set from config?)
	log.Logger.SetLevel(logrus.DebugLevel)

	log.Info("Start driver with CLI options:")
	log.Infof("- CSI endpoint:    '%v'", *endpoint)
	log.Infof("- Node ID:         '%v'", *nodeID)

	// initial config file validation
	cfg, err := config.Get()
	if err != nil {
		log.Fatalf("Cannot use config file: %v", err)
	}

	log.Info("Config file options:")
	log.Infof("- NexentaStor address: %v", cfg.Address)
	log.Infof("- NexentaStor username: %v", cfg.Username)
	log.Infof("- Default dataset: %v", cfg.DefaultDataset)
	log.Infof("- Default data IP: %v", cfg.DefaultDataIP)

	d := driver.NewDriver(driver.Args{
		NodeID:   *nodeID,
		Endpoint: *endpoint,
		Log:      log,
	})

	d.Run()
}
