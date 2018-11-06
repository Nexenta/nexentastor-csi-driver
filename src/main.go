package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/driver"
)

const (
	defaultEndpoint  = "unix:///var/lib/kubelet/plugins/com.nexenta.nexentastor-csi-driver/csi.sock"
	defaultConfigDir = "/config"
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

	// logger formatter
	l.Logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"cmp", "ns", "func", "req", "reqID", "job"},
	})

	l.Info("Start driver with CLI options:")
	l.Infof("- CSI endpoint:    '%v'", *endpoint)
	l.Infof("- Node ID:         '%v'", *nodeID)

	// initial read and validate config file
	cfg, err := config.New(defaultConfigDir)
	if err != nil {
		l.Fatalf("Cannot use config file: %v", err)
	}
	l.Infof("Config file: '%v'", cfg.GetFilePath())

	// logger level
	if cfg.Debug {
		l.Logger.SetLevel(logrus.DebugLevel)
	} else {
		l.Logger.SetLevel(logrus.InfoLevel)
	}

	l.Info("Config file options:")
	l.Infof("- NexentaStor address(es): %v", cfg.Address)
	l.Infof("- NexentaStor username: %v", cfg.Username)
	l.Infof("- Default dataset: %v", cfg.DefaultDataset)
	l.Infof("- Default data IP: %v", cfg.DefaultDataIP)

	d, err := driver.NewDriver(driver.Args{
		NodeID:   *nodeID,
		Endpoint: *endpoint,
		Config:   cfg,
		Log:      l,
	})
	if err != nil {
		writeTerminationMessage(err, l)
		l.Fatal(err)
	}

	// validate driver configuration, NS licenses
	err = d.Validate()
	if err != nil {
		writeTerminationMessage(err, l)
		l.Fatal(err)
	}

	// run driver
	err = d.Run()
	if err != nil {
		writeTerminationMessage(err, l)
		l.Fatal(err)
	}
}

// Kubernetes retrieves termination messages from the termination message file of a Container,
// which as a default value of /dev/termination-log
func writeTerminationMessage(err error, l *logrus.Entry) {
	writeErr := ioutil.WriteFile("/dev/termination-log", []byte(fmt.Sprintf("\n%v\n", err)), os.ModePerm)
	if writeErr != nil {
		l.Warnf("Failed to write termination message: %v", writeErr)
	}
}
