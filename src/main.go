package main

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"

	"github.com/Nexenta/nexentastor-csi-driver/src/driver"
	"github.com/Nexenta/nexentastor-csi-driver/src/nexentastor"
)

const (
	defaultEndpoint = "unix:///var/lib/kubelet/plugins/com.nexenta.nexentastor-csi-plugin/csi.sock"
)

func main() {
	var (
		nodeID   = flag.String("nodeid", "", "Kubernetes node ID")
		endpoint = flag.String("endpoint", defaultEndpoint, "CSI endpoint")
		address  = flag.String("address", "", "NexentaStor API address [schema://host:port,...]")
		username = flag.String("username", "", "overwrite NexentaStor API username from config")
		password = flag.String("password", "", "overwrite NexentaStor API password from config")
		version  = flag.Bool("version", false, "Print driver version")
	)

	flag.Parse()

	if *version {
		fmt.Printf("Version: %s, commit: %s\n", driver.GetVersion(), driver.GetCommit())
		os.Exit(0)
	}

	if len(*address) == 0 {
		fmt.Print(
			"NexentaStor address is not set, use 'address' option in config file or CLI",
		)
		os.Exit(1)
	}

	if len(*username) == 0 {
		fmt.Print(
			"NexentaStor username is not set, use 'username' option in config file or CLI",
		)
		os.Exit(1)
	}

	if len(*password) == 0 {
		fmt.Print(
			"NexentaStor password is not set, use 'password' option in config file or CLI",
		)
		os.Exit(1)
	}

	// init logger
	log := logrus.New().WithFields(logrus.Fields{
		//"nodeId":    *nodeID,
		//"address":   *address,
		"cmp": "Main",
	})

	// logger level (set from config?)
	log.Logger.SetLevel(logrus.DebugLevel)

	log.Info("Start driver with:")
	log.Infof("- CSI endpoint: '%v'\n", *endpoint)
	log.Infof("- Node ID:      '%v'\n", *nodeID)
	log.Infof("- NS address:   '%v'\n", *address)

	//TESTS

	ns, err := nexentastor.NewProvider(nexentastor.ProviderArgs{
		Address:  *address,
		Username: fmt.Sprint(*username),
		Password: fmt.Sprint(*password),
		Log:      log,
	})
	if err != nil {
		log.Error(err)
	}

	pools, err := ns.GetPools()
	if err != nil {
		log.Error(err)
	}

	pools, err = ns.GetPools()
	if err != nil {
		log.Error(err)
	}

	log.Infof("pools: %v", pools)
}
