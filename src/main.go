package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"

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
	config, err := driver.GetConfig()
	if err != nil {
		log.Fatalf("Cannot use config file: %v", err)
	}

	log.Info("Config file options:")
	log.Infof("- NexentaStor address: %v", config.Address)
	log.Infof("- NexentaStor username: %v", config.Username)
	log.Infof("- Default dataset: %v", config.DefaultDataset)
	log.Infof("- Default data IP: %v", config.DefaultDataIP)

	d := driver.NewDriver(driver.Args{
		NodeID:   *nodeID,
		Endpoint: *endpoint,
		Log:      log,
	})

	d.Run()
}
