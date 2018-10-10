package resolver_test

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"testing"

	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

const (
	defaultAddress         = "https://10.3.199.252:8443,https://10.3.199.253:8443"
	defaultUsername        = "admin"
	defaultPassword        = "Nexenta@1"
	defaultPoolName        = "csiDriverPool"
	defaultcDatasetName    = "csiDriverDataset"
	defaultcFilesystemName = "csiDriverFs"
)

type config struct {
	address    string
	username   string
	password   string
	pool       string
	dataset    string
	filesystem string
}

var c *config
var logger *logrus.Entry

func filesystemArrayContains(array []*ns.Filesystem, value string) bool {
	for _, v := range array {
		if v.Path == value {
			return true
		}
	}
	return false
}

func TestMain(m *testing.M) {
	var (
		address    = flag.String("address", defaultAddress, "NS API [schema://host:port,...]")
		username   = flag.String("username", defaultUsername, "overwrite NS API username from config")
		password   = flag.String("password", defaultPassword, "overwrite NS API password from config")
		pool       = flag.String("pool", defaultPoolName, "pool on NS")
		dataset    = flag.String("dataset", defaultcDatasetName, "dataset on NS")
		filesystem = flag.String("filesystem", defaultcFilesystemName, "filesystem on NS")
		log        = flag.Bool("log", false, "show logs")
	)

	flag.Parse()

	logger = logrus.New().WithFields(logrus.Fields{
		"ns": *address,
	})
	logger.Logger.SetLevel(logrus.PanicLevel)
	if *log {
		logger.Logger.SetLevel(logrus.DebugLevel)
	}

	c = &config{
		address:    *address,
		username:   *username,
		password:   *password,
		pool:       *pool,
		dataset:    fmt.Sprintf("%v/%v", *pool, *dataset),
		filesystem: fmt.Sprintf("%v/%v/%v", *pool, *dataset, *filesystem),
	}

	os.Exit(m.Run())
}

func TestResolver_NewResolverMulti(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	nsr, err := ns.NewResolver(ns.ResolverArgs{
		Address:  c.address,
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("Resolve() should return NS with requested dataset", func(t *testing.T) {
		nsProvider, err := nsr.Resolve(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if nsProvider == nil {
			t.Error("No NS returned by resolver")
			return
		}

		filesystems, err := nsProvider.GetFilesystems(c.pool)
		if err != nil {
			t.Errorf("NS Error: %v", err)
			return
		} else if !filesystemArrayContains(filesystems, c.dataset) {
			t.Errorf("Returned NS (%v) doesn't contain dataset: %v", nsProvider, c.dataset)
			return
		}
	})

	t.Run("Resolve() should return error if dataset not exists", func(t *testing.T) {
		nsProvider, err := nsr.Resolve("not/exists")
		if err == nil {
			t.Errorf("Resolver return NS for non-existing datastore: %v", nsProvider)
			return
		}
	})
}
