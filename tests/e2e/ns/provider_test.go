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

func TestProvider_NewProvider(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	nsp, err := ns.NewProvider(ns.ProviderArgs{
		Address:  c.address,
		Username: c.username,
		Password: c.password,
		Log:      l,
	})
	if err != nil {
		t.Error(err)
	}

	t.Run("GetLicense()", func(t *testing.T) {
		license, err := nsp.GetLicense()
		if err != nil {
			t.Error(err)
		} else if !license.Valid {
			t.Errorf("License %v is not valid, on NS %v", license, c.address)
		} else if license.Expires[0:2] != "20" {
			t.Errorf("License expires date should starts with '20': %v, on NS %v", license, c.address)
		}
	})

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

		err = nsp.CreateFilesystem(ns.CreateFilesystemParams{
			Path: c.filesystem,
		})
		if err != nil {
			t.Error(err)
			return
		}
		filesystems, err = nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Errorf("New filesystem %v wasn't created on NS %v", c.filesystem, c.address)
		}
	})

	t.Run("GetFilesystem() created filesystem should not be shared", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
		} else if filesystem == nil {
			t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
		} else if filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %v should not be shared over NFS (NS %v)", c.filesystem, c.address)
		} else if filesystem.SharedOverSmb {
			t.Errorf("Created filesystem %v should not be shared over SMB (NS %v)", c.filesystem, c.address)
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

	t.Run("GetFilesystem() created filesystem should be shared over NFS", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
		} else if filesystem == nil {
			t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
		} else if !filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %v should be shared (NS %v)", c.filesystem, c.address)
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

	testSmbShareName := "testShareName"
	for _, smbShareName := range []string{testSmbShareName, ""} {
		smbShareName := smbShareName

		t.Run(
			fmt.Sprintf("CreateSmbShare() should create SMB share with '%s' share name", smbShareName),
			func(t *testing.T) {
				filesystems, err := nsp.GetFilesystems(c.dataset)
				if err != nil {
					t.Error(err)
					return
				} else if !filesystemArrayContains(filesystems, c.filesystem) {
					t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
					return
				}

				err = nsp.CreateSmbShare(c.filesystem, smbShareName)
				if err != nil {
					t.Error(err)
				}
			},
		)

		t.Run("GetFilesystem() created filesystem should be shared over SMB", func(t *testing.T) {
			filesystem, err := nsp.GetFilesystem(c.filesystem)
			if err != nil {
				t.Error(err)
			} else if filesystem == nil {
				t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
			} else if !filesystem.SharedOverSmb {
				t.Errorf("Created filesystem %v should be shared over SMB (NS %v)", c.filesystem, c.address)
			}
		})

		t.Run("GetSmbShareName() should return SMB share name", func(t *testing.T) {
			filesystem, err := nsp.GetFilesystem(c.filesystem)
			if err != nil {
				t.Error(err)
				return
			} else if filesystem == nil {
				t.Errorf("Filesystem %v wasn't found on NS %v", c.filesystem, c.address)
				return
			}

			var expectedShareName string
			if smbShareName == "" {
				expectedShareName = filesystem.GetDefaultSmbShareName()
			} else {
				expectedShareName = smbShareName
			}

			shareName, err := nsp.GetSmbShareName(c.filesystem)
			if err != nil {
				t.Error(err)
			} else if shareName != expectedShareName {
				t.Errorf(
					"expected shareName='%s' but got '%s', for filesystem '%s' on NS %s",
					expectedShareName,
					shareName,
					c.filesystem,
					c.address,
				)
			}
		})

		//TODO test SMB share, mount cifs?

		t.Run("DeleteSmbShare()", func(t *testing.T) {
			filesystems, err := nsp.GetFilesystems(c.dataset)
			if err != nil {
				t.Error(err)
				return
			} else if !filesystemArrayContains(filesystems, c.filesystem) {
				t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
				return
			}

			err = nsp.DeleteSmbShare(c.filesystem)
			if err != nil {
				t.Error(err)
			}
		})
	}

	t.Run("DestroyFilesystem()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v doesn't exist on NS %v", c.filesystem, c.address)
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

		err = nsp.CreateFilesystem(ns.CreateFilesystemParams{
			Path:      c.filesystem,
			QuotaSize: quotaSize,
		})
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

		// clean up
		nsp.DestroyFilesystem(c.filesystem)
	})

	t.Run("GetFilesystemAvailableCapacity()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %v already exists on NS %v", c.filesystem, c.address)
			return
		}

		var quotaSize int64 = 3 * 1024 * 1024 * 1024

		err = nsp.CreateFilesystem(ns.CreateFilesystemParams{
			Path:      c.filesystem,
			QuotaSize: quotaSize,
		})
		if err != nil {
			t.Error(err)
			return
		}

		availableCapacity, err := nsp.GetFilesystemAvailableCapacity(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		} else if availableCapacity == 0 {
			t.Errorf("New filesystem %v indicates wrong available capacity (0), on: %v", c.filesystem, c.address)
		} else if availableCapacity >= quotaSize {
			t.Errorf(
				"New filesystem %v available capacity expected to be more or equal to %v, but got %v (NS %v)",
				c.filesystem,
				quotaSize,
				availableCapacity,
				c.address,
			)
		}

		// clean up
		nsp.DestroyFilesystem(c.filesystem)
	})
}
