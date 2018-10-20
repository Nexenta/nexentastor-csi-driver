package driver_test

import (
	"flag"
	"fmt"
	"os"
	"testing"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/k8s"
	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

type config struct {
	k8sConnectionString string
	k8sDeploymentFile   string
	k8sSecretFile       string
}

var c *config

func TestMain(m *testing.M) {
	var (
		k8sConnectionString = flag.String("k8sConnectionString", "", "K8s connection string [user@host]")
		k8sDeploymentFile   = flag.String("k8sDeploymentFile", "", "path to driver deployment yaml file")
		k8sSecretFile       = flag.String("k8sSecretFile", "", "path to yaml driver config file (for k8s secret)")
	)

	flag.Parse()

	if *k8sConnectionString == "" {
		fmt.Println("Parameter '--k8sConnectionString' is missed")
		os.Exit(1)
	} else if *k8sDeploymentFile == "" {
		fmt.Println("Parameter '--k8sDeploymentFile' is missed")
		os.Exit(1)
	} else if *k8sSecretFile == "" {
		fmt.Println("Parameter '--k8sSecretFile' is missed")
		os.Exit(1)
	}

	c = &config{
		k8sConnectionString: *k8sConnectionString,
		k8sDeploymentFile:   *k8sDeploymentFile,
		k8sSecretFile:       *k8sSecretFile,
	}

	os.Exit(m.Run())
}

func TestDriver_deploy(t *testing.T) {
	rc, err := remote.NewClient(c.k8sConnectionString)
	if err != nil {
		t.Errorf("Cannot create connection: %v", err)
		return
	}

	k8sDriver, err := k8s.NewDeployment(rc, c.k8sDeploymentFile, c.k8sSecretFile)
	if err != nil {
		t.Errorf("Cannot create K8s deployment: %v", err)
		return
	}

	t.Run("install driver", func(t *testing.T) {
		if err := k8sDriver.CreateSecret(); err != nil {
			t.Fatal(err)
		}

		waitPods := []string{
			"nexentastor-csi-attacher-.*Running",
			"nexentastor-csi-provisioner-.*Running",
			"nexentastor-csi-driver-.*Running",
		}

		if err := k8sDriver.Apply(waitPods); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("install nginx with dynamic volume provisioning", func(t *testing.T) {
		k8sNginx, err := k8s.NewDeployment(rc, "./_configs/nginx-storage-class.yaml", "")
		if err != nil {
			t.Errorf("Cannot create K8s nginx deployment: %v", err)
			return
		}

		if err := k8sNginx.Apply([]string{"nginx-storage-class.*Running"}); err != nil {
			k8sNginx.CleanUp()
			t.Fatal(err)
		}

		// validate volume on nginx

		if err := k8sNginx.Delete([]string{"nginx-storage-class"}); err != nil {
			k8sNginx.CleanUp()
			t.Fatal(err)
		}
	})

	t.Run("uninstall driver", func(t *testing.T) {
		if err := k8sDriver.Delete([]string{"nexentastor-csi-.*"}); err != nil {
			t.Fatal(err)
		}

		if err := k8sDriver.DeleteSecret(); err != nil {
			t.Fatal(err)
		}
	})

	k8sDriver.CleanUp()
}
