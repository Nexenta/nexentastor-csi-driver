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
		// address        = flag.String("rest-ip", "", "NexentaStor API address(s) [schema://host:port,...]")
		// username       = flag.String("username", "", "NexentaStor API username")
		// password       = flag.String("password", "", "NexentaStor API password")
		// defaultDataset = flag.String("default-dataset", "", "default dataset to create filesystems on")
		// defaultDataIP  = flag.String("default-data-ip", "", "default data IP for sharing filesystems")
		version = flag.Bool("version", false, "Print driver version")
	)

	flag.Parse()

	if *version {
		fmt.Printf("%v@%v-%v (%v)\n", driver.Name, driver.Version, driver.Commit, driver.DateTime)
		os.Exit(0)
	}

	// if len(*address) == 0 {
	// 	fmt.Print(
	// 		"NexentaStor REST API address is not set, use 'restIp' option in the secret",
	// 	)
	// 	os.Exit(1)
	// }

	// if len(*username) == 0 {
	// 	fmt.Print(
	// 		"NexentaStor REST API username is not set, use 'username' option in the secret",
	// 	)
	// 	os.Exit(1)
	// }

	// if len(*password) == 0 {
	// 	fmt.Print(
	// 		"NexentaStor REST API password is not set, use 'password' option in the secret",
	// 	)
	// 	os.Exit(1)
	// }

	// init logger
	log := logrus.New().WithFields(logrus.Fields{
		//"nodeId":    *nodeID,
		"cmp": "Main",
	})

	// logger level (set from config?)
	log.Logger.SetLevel(logrus.DebugLevel)

	log.Info("Start driver with:")
	log.Infof("- CSI endpoint:    '%v'\n", *endpoint)
	log.Infof("- Node ID:         '%v'\n", *nodeID)
	// log.Infof("- NS address(s):   '%v'\n", *address)
	// log.Infof("- Default dataset: '%v'\n", *defaultDataset)
	// log.Infof("- Default data IP: '%v'\n", *defaultDataIP)

	d := driver.NewDriver(driver.Args{
		NodeID:   *nodeID,
		Endpoint: *endpoint,
		Log:      log,
	})

	d.Run()
}
