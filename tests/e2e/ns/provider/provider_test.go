package provider_test

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/pkg/ns"
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
	cluster    bool
}

var c *config
var l *logrus.Entry

func poolArrayContains(array []ns.Pool, value string) bool {
	for _, v := range array {
		if v.Name == value {
			return true
		}
	}
	return false
}

func filesystemArrayContains(array []ns.Filesystem, value string) bool {
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
		cluster    = flag.Bool("cluster", false, "this is a NS cluster")
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
		dataset:    fmt.Sprintf("%s/%s", *pool, *dataset),
		filesystem: fmt.Sprintf("%s/%s/%s", *pool, *dataset, *filesystem),
		cluster:    *cluster,
	}

	os.Exit(m.Run())
}

func TestProvider_NewProvider(t *testing.T) {
	t.Logf("Using NS: %s", c.address)

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
			t.Errorf("License %+v is not valid, on NS %s", license, c.address)
		} else if license.Expires[0:2] != "20" {
			t.Errorf("License expires date should starts with '20': %+v, on NS %s", license, c.address)
		}
	})

	t.Run("GetPools()", func(t *testing.T) {
		pools, err := nsp.GetPools()
		if err != nil {
			t.Error(err)
		} else if !poolArrayContains(pools, c.pool) {
			t.Errorf("Pool %s doesn't exist on NS %s", c.pool, c.address)
		}
	})

	t.Run("GetFilesystems()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.pool)
		if err != nil {
			t.Error(err)
		} else if filesystemArrayContains(filesystems, c.pool) {
			t.Errorf("Pool %s should not be in the results", c.pool)
		} else if !filesystemArrayContains(filesystems, c.dataset) {
			t.Errorf("Dataset %s doesn't exist", c.dataset)
		}
	})

	t.Run("GetFilesystem() exists", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.dataset)
		if err != nil {
			t.Error(err)
		} else if filesystem.Path != c.dataset {
			t.Errorf("No %s filesystem in the result", c.dataset)
		}
	})

	t.Run("GetFilesystem() not exists", func(t *testing.T) {
		nonExistingName := "NON_EXISTING"
		filesystem, err := nsp.GetFilesystem(nonExistingName)
		if err != nil && !strings.Contains(err.Error(), "not found") {
			t.Error(err)
		} else if filesystem.Path != "" {
			t.Errorf("Filesystem %s should not exist, but found in the result: %v", nonExistingName, filesystem)
		}
	})

	t.Run("CreateFilesystem()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %s already exists on NS %s", c.filesystem, c.address)
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
			t.Errorf("New filesystem %s wasn't created on NS %s", c.filesystem, c.address)
		}
	})

	t.Run("GetFilesystem() created filesystem should not be shared", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
		} else if filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %s should not be shared over NFS (NS %s)", c.filesystem, c.address)
		} else if filesystem.SharedOverSmb {
			t.Errorf("Created filesystem %s should not be shared over SMB (NS %s)", c.filesystem, c.address)
		}
	})

	t.Run("CreateNfsShare()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %s doesn't exist on NS %s", c.filesystem, c.address)
			return
		}

		err = nsp.CreateNfsShare(ns.CreateNfsShareParams{
			Filesystem: c.filesystem,
		})
		if err != nil {
			t.Error(err)
		}
	})

	t.Run("GetFilesystem() created filesystem should be shared over NFS", func(t *testing.T) {
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
		} else if !filesystem.SharedOverNfs {
			t.Errorf("Created filesystem %s should be shared (NS %s)", c.filesystem, c.address)
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
			t.Errorf("cannot find '%s' nfs in the 'showmount' output: \n---\n%s\n---\n", c.filesystem, out)
		}
	})

	t.Run("DeleteNfsShare()", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if !filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %s doesn't exist on NS %s", c.filesystem, c.address)
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
					t.Skipf("Filesystem %s doesn't exist on NS %s", c.filesystem, c.address)
					return
				}

				err = nsp.CreateSmbShare(ns.CreateSmbShareParams{
					Filesystem: c.filesystem,
					ShareName:  smbShareName,
				})
				if err != nil {
					t.Error(err)
				}
			},
		)

		t.Run("GetFilesystem() created filesystem should be shared over SMB", func(t *testing.T) {
			filesystem, err := nsp.GetFilesystem(c.filesystem)
			if err != nil {
				t.Error(err)
			} else if !filesystem.SharedOverSmb {
				t.Errorf("Created filesystem %s should be shared over SMB (NS %s)", c.filesystem, c.address)
			}
		})

		t.Run("GetSmbShareName() should return SMB share name", func(t *testing.T) {
			filesystem, err := nsp.GetFilesystem(c.filesystem)
			if err != nil {
				t.Error(err)
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
				t.Skipf("Filesystem %s doesn't exist on NS %s", c.filesystem, c.address)
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
			t.Skipf("Filesystem %s doesn't exist on NS %s", c.filesystem, c.address)
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
			t.Errorf("Filesystem %s still exists on NS %s", c.filesystem, c.address)
		}
	})

	t.Run("CreateFilesystem() with referenced quota size", func(t *testing.T) {
		filesystems, err := nsp.GetFilesystems(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if filesystemArrayContains(filesystems, c.filesystem) {
			t.Skipf("Filesystem %s already exists on NS %s", c.filesystem, c.address)
			return
		}

		var referencedQuotaSize int64 = 2 * 1024 * 1024 * 1024

		err = nsp.CreateFilesystem(ns.CreateFilesystemParams{
			Path:                c.filesystem,
			ReferencedQuotaSize: referencedQuotaSize,
		})
		if err != nil {
			t.Error(err)
			return
		}
		filesystem, err := nsp.GetFilesystem(c.filesystem)
		if err != nil {
			t.Error(err)
			return
		} else if filesystem.GetReferencedQuotaSize() != referencedQuotaSize {
			t.Errorf(
				"New filesystem %s referenced quota size expected to be %d, but got %d (NS %s)",
				filesystem.Path,
				referencedQuotaSize,
				filesystem.GetReferencedQuotaSize(),
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
			t.Skipf("Filesystem %s already exists on NS %s", c.filesystem, c.address)
			return
		}

		var referencedQuotaSize int64 = 3 * 1024 * 1024 * 1024

		err = nsp.CreateFilesystem(ns.CreateFilesystemParams{
			Path:                c.filesystem,
			ReferencedQuotaSize: referencedQuotaSize,
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
			t.Errorf("New filesystem %s indicates wrong available capacity (0), on: %s", c.filesystem, c.address)
		} else if availableCapacity >= referencedQuotaSize {
			t.Errorf(
				"New filesystem %s available capacity expected to be more or equal to %d, but got %d (NS %s)",
				c.filesystem,
				referencedQuotaSize,
				availableCapacity,
				c.address,
			)
		}

		// clean up
		nsp.DestroyFilesystem(c.filesystem)
	})

	t.Run("GetRSFClusters()", func(t *testing.T) {
		expectedToBeACluster := c.cluster

		clusters, err := nsp.GetRSFClusters()
		if err != nil {
			t.Error(err)
			return
		}

		if expectedToBeACluster && len(clusters) == 0 {
			t.Errorf(
				"NS %s expected to be in a cluster (--cluster=true flag) but got no clusters from the API",
				c.address,
			)
		} else if !expectedToBeACluster && len(clusters) > 0 {
			t.Errorf(
				"NS %s expected not to be in a cluster (--cluster=false flag) but got clusters from the API: %+v",
				c.address,
				clusters,
			)
		}
	})
}
