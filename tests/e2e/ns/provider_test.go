package provider_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/arrays"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

// defaults
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

	logger = logrus.New().WithField("ns", *address)
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

	t.Run("GetPools()", func(t *testing.T) {
		pools, err := nsp.GetPools()
		if err != nil {
			t.Error(err)
		} else if !arrays.ContainsString(pools, c.pool) {
			t.Errorf("Pool %v doesn't exist on NS %v", c.pool, c.address)
		}
	})

	t.Run("GetFilesystems()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.pool)
		if err != nil {
			t.Error(err)
		} else if filesystemArrayContains(filesystems, c.pool) {
			t.Errorf("Pool %v should not be in the results", c.pool)
		} else if !filesystemArrayContains(filesystems, c.dataset) {
			t.Errorf("Dataset %v doesn't exist", c.dataset)
		}
	})

	t.Run("GetFilesystem() exists", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.dataset)
		if err != nil {
			t.Error(err)
		} else if filesystem == nil || filesystem.Path != c.dataset {
			t.Errorf("No %v filesystem in the result", c.dataset)
		}
	})

	t.Run("GetFilesystem() not exists", func(t *testing.T) {
		nonExistingName := "NON_EXISTING"
		filesystem, err := nsp.GetFilesystem(nonExistingName)
		if err != nil {
			t.Error(err)
		} else if filesystem != nil {
			t.Errorf("Filesystem %v should not exist, but found in the result: %v", nonExistingName, filesystem)
		}
	})

	t.Run("CreateFilesystem()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v already exists on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.CreateFilesystem(c.filesystem, nil)
		if err != nil {
			t.Error(err)
			return
		}
		filesystems, err = nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Errorf("New filesystem %v wasn't created on NS %v", c.filesystem, c.address)
		}
	})

	t.Run("GetFilesystem() created filesystem should not be shared", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		} else if filesystem == nil {
			t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
			return
		} else if filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %v should not be shared (NS %v)", c.filesystem, c.address)
			return
		}
	})

	t.Run("CreateNfsShare()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.CreateNfsShare(c.filesystem)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("GetFilesystem() created filesystem should be shared", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		} else if filesystem == nil {
			t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
			return
		} else if !filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %v should be shared (NS %v)", c.filesystem, c.address)
			return
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

	t.Run("DeleteNfsShare()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
			return
		}

		err = nsp.DeleteNfsShare(c.filesystem)
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("DestroyFilesystem()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
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
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Errorf("Filesystem %v still exists on NS %v", c.filesystem, c.address)
		}
	})

	t.Run("CreateFilesystem() with quota size", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v already exists on NS %v", c.filesystem, c.address)
			return
		}

		var quotaSize int64 = 2 * 1024 * 1024 * 1024

		params := make(map[string]interface{})
		params["quotaSize"] = quotaSize

		err = nsp.CreateFilesystem(c.filesystem, params)
		if err != nil {
			t.Error(err)
			return
		}
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		} else if filesystem == nil {
			t.Errorf("New filesystem %v wasn't created on NS %v", c.filesystem, c.address)
		} else if filesystem.QuotaSize != quotaSize {
			t.Errorf(
				"New filesystem %v quota size expected to be %v, but got %v (NS %v)",
				filesystem.Path,
				quotaSize,
				filesystem.QuotaSize,
				c.address,
			)
		}
	})
}
