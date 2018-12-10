package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/pkg/config"
	"github.com/Nexenta/nexentastor-csi-driver/pkg/driver"
)

const (
	defaultEndpoint  = "unix:///var/lib/kubelet/plugins_registry/nexentastor-csi-driver.nexenta.com/csi.sock"
	defaultConfigDir = "/config"
	defaultRole      = driver.RoleAll
)

func main() {
	var (
		nodeID    = flag.String("nodeid", "", "Kubernetes node ID")
		endpoint  = flag.String("endpoint", defaultEndpoint, "CSI endpoint")
		configDir = flag.String("config-dir", defaultConfigDir, "driver config endpoint")
		role      = flag.String("role", "", fmt.Sprintf("driver role: %v", driver.Roles))
		version   = flag.Bool("version", false, "Print driver version")
	)

	flag.Parse()

	if *version {
		fmt.Printf("%s@%s-%s (%s)\n", driver.Name, driver.Version, driver.Commit, driver.DateTime)
		os.Exit(0)
	}

	// init logger
	l := logrus.New().WithFields(logrus.Fields{
		"nodeID": *nodeID,
		"cmp":    "Main",
	})

	// logger formatter
	l.Logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"nodeID", "cmp", "ns", "func", "req", "reqID", "job"},
	})

	l.Info("Run driver with CLI options:")
	l.Infof("- Role:             '%s'", *role)
	l.Infof("- Node ID:          '%s'", *nodeID)
	l.Infof("- CSI endpoint:     '%s'", *endpoint)
	l.Infof("- Config directory: '%s'", *configDir)

	// validate driver instance role
	validatedRole, err := driver.ParseRole(string(*role))
	if err != nil {
		l.Warn(err)
	}

	// initial read and validate config file
	cfg, err := config.New(*configDir)
	if err != nil {
		l.Fatalf("Cannot use config file: %s", err)
	}
	l.Infof("Config file: '%s'", cfg.GetFilePath())

	// logger level
	if cfg.Debug {
		l.Logger.SetLevel(logrus.DebugLevel)
	} else {
		l.Logger.SetLevel(logrus.InfoLevel)
	}

	l.Info("Config file options:")
	l.Infof("- NexentaStor address(es): %s", cfg.Address)
	l.Infof("- NexentaStor username: %s", cfg.Username)
	l.Infof("- Default dataset: %s", cfg.DefaultDataset)
	l.Infof("- Default data IP: %s", cfg.DefaultDataIP)

	d, err := driver.NewDriver(driver.Args{
		Role:     validatedRole,
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
	writeErr := ioutil.WriteFile("/dev/termination-log", []byte(fmt.Sprintf("\n%s\n", err)), os.ModePerm)
	if writeErr != nil {
		l.Warnf("Failed to write termination message: %s", writeErr)
	}
}
