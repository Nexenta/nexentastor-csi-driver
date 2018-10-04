package resolver_test

import (
	"flag"
	"fmt"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
	//"os/exec"
	//"strings"
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

func stringArrayContains(array []string, value string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
}

func nodesArrayContains(array []ns.ProviderInterface, value ns.ProviderInterface) bool {
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

func TestResolver_NewResolverSingle(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	addressArray := strings.Split(fmt.Sprint(c.address), ",")

	nsp1, err := ns.NewProvider(ns.ProviderArgs{
		Address:  addressArray[0],
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	nodes := []ns.ProviderInterface{nsp1}
	nsr, err := ns.NewResolver(ns.ResolverArgs{
		Nodes: nodes,
		Log:   logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("Resolve should return single NS", func(t *testing.T) {
		ns, err := nsr.Resolve(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if ns != nsp1 {
			t.Errorf("No NS returned by resolver")
			return
		}
	})
}
func TestResolver_NewResolverDuplicatedAddress(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	addressArray := strings.Split(fmt.Sprint(c.address), ",")

	nsp1, err := ns.NewProvider(ns.ProviderArgs{
		Address:  addressArray[0],
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("Resolver should detect same duplicated NS in config", func(t *testing.T) {
		nodes := []ns.ProviderInterface{nsp1, nsp1}
		_, err := ns.NewResolver(ns.ResolverArgs{
			Nodes: nodes,
			Log:   logger,
		})
		if err == nil {
			t.Error("Resolver doesn't detect duplicated NS")
			return
		}
	})
}
func TestResolver_NewResolverMulti(t *testing.T) {
	t.Logf("Using NS: %v", c.address)

	addressArray := strings.Split(fmt.Sprint(c.address), ",")

	nsp1, err := ns.NewProvider(ns.ProviderArgs{
		Address:  addressArray[0],
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	nsp2, err := ns.NewProvider(ns.ProviderArgs{
		Address:  addressArray[1],
		Username: c.username,
		Password: c.password,
		Log:      logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	nodes := []ns.ProviderInterface{nsp1, nsp2}
	nsr, err := ns.NewResolver(ns.ResolverArgs{
		Nodes: nodes,
		Log:   logger,
	})
	if err != nil {
		t.Error(err)
		return
	}

	t.Run("Resolve should return NS with requested dataset", func(t *testing.T) {
		ns, err := nsr.Resolve(c.dataset)
		if err != nil {
			t.Error(err)
			return
		} else if ns == nil {
			t.Error("No NS returned by resolver")
			return
		} else if !nodesArrayContains(nodes, ns) {
			t.Errorf("Nodes (%v) don't contain resolved NS: %v", nodes, c.address)
			return
		}

		filesystems, err := ns.GetFilesystems(c.pool)
		if err != nil {
			t.Errorf("NS Error: %v", err)
			return
		} else if !stringArrayContains(filesystems, c.dataset) {
			t.Errorf("Returned NS (%v) doesn't contain dataset: %v", ns, c.dataset)
			return
		}
	})

	t.Run("Resolve should return error if dataset not exists", func(t *testing.T) {
		ns, err := nsr.Resolve("not/exists")
		if err == nil {
			t.Errorf("Resolver return NS for non-existing datastore: %v", ns)
			return
		}
	})
}
