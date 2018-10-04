package provider_test

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

const (
	defaultAddress         = "https://10.3.199.254:8443"
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

func arrayContains(array []string, value string) bool {
	for _, v := range array {
		if v == value {
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

func TestProvider_NewProvider(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	nsp, err := ns.NewProvider(ns.ProviderArgs{
		Address:  c.address,
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
	}

	t.Run("GetPools", func(t *testing.T) {
		pools, err := nsp.GetPools()
		if err != nil {
			t.Error(err)
		} else if !arrayContains(pools, c.pool) {
			t.Errorf("Pool %v doesn't exist on NS %v", c.pool, c.address)
		}
	})

	t.Run("GetFilesystems", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.pool)
		if err != nil {
			t.Error(err)
		} else if !arrayContains(filesystems, c.pool) {
			t.Errorf("No %v pool in result", c.dataset)
		} else if !arrayContains(filesystems, c.dataset) {
			t.Errorf("Dataset %v doesn't exist", c.dataset)
		}
	})

	t.Run("GetFilesystem_exists", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.dataset)
		if err != nil {
			t.Error(err)
		} else if filesystem != c.dataset {
			t.Errorf("No %v filesystem in the result", c.dataset)
		}
	})

	t.Run("GetFilesystem_not_exists", func(t *testing.T) {
		nonExistingName := "NON_EXISTING"
		filesystem, err := nsp.GetFilesystem(nonExistingName)
		if err != nil {
			t.Error(err)
		} else if filesystem != "" {
			t.Errorf("Filesystem %v should not exist, but found in the result: %v", nonExistingName, filesystem)
		}
	})

	t.Run("CreateFilesystem", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if arrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v already exists on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.CreateFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		}
		filesystems, err = nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !arrayContains(filesystems, c.filesystem) {
			t.Errorf("New filesystem %v wasn't created on NS %v", c.filesystem, c.address)
		}
	})

	t.Run("CreateNfsShare", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !arrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.CreateNfsShare(c.filesystem)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("nfs share should appear on NS", func(t *testing.T) {
		//TODO other way to cut out host from address
		host := strings.Split(c.address, "//")[1]
		host = strings.Split(host, ":")[0]

		out, err := exec.Command("showmount", "-e", host).Output()
		if err != nil {
			t.Error(err)
		} else if !strings.Contains(fmt.Sprintf("%s", out), c.filesystem) {
			t.Errorf("cannot find '%v' nfs in the 'showmount' output: \n---\n%s\n---\n", c.filesystem, out)
		}
	})

	t.Run("DeleteNfsShare", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !arrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.DeleteNfsShare(c.filesystem)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("DestroyFilesystem", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !arrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doens't exist on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.DestroyFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		}
		filesystems, err = nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
		} else if arrayContains(filesystems, c.filesystem) {
			t.Errorf("Filesystem %v still exists on NS %v", c.filesystem, c.address)
		}
	})
}
