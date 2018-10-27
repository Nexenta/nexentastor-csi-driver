package resolver_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

const (
	defaultUsername       = "admin"
	defaultPassword       = "Nexenta@1"
	defaultPoolName       = "csiDriverPool"
	defaultDatasetName    = "csiDriverDataset"
	defaultFilesystemName = "csiDriverFs"
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
var l *logrus.Entry

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
		address    = flag.String("address", "", "NS API [schema://host:port,...]")
		username   = flag.String("username", defaultUsername, "overwrite NS API username from config")
		password   = flag.String("password", defaultPassword, "overwrite NS API password from config")
		pool       = flag.String("pool", defaultPoolName, "pool on NS")
		dataset    = flag.String("dataset", defaultDatasetName, "dataset on NS")
		filesystem = flag.String("filesystem", defaultFilesystemName, "filesystem on NS")
		log        = flag.Bool("log", false, "show logs")
	)

	flag.Parse()

	l = logrus.New().WithField("ns", *address)
	l.Logger.SetLevel(logrus.PanicLevel)
	if *log {
		l.Logger.SetLevel(logrus.DebugLevel)
	}

	if *address == "" {
		l.Fatal("--address=[schema://host:port,...] flag cannot be empty")
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
		Log:      l,
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
