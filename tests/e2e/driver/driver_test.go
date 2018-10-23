package driver_test

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/k8s"
	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

type config struct {
	k8sConnectionString string
	k8sDeploymentFile   string
	k8sSecretFile       string
}

var c *config
var l *logrus.Entry

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

	// init logger
	l = logrus.New().WithField("title", "tests")

	// logger formatter
	l.Logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"title", "address", "cmp", "func", "opt"},
	})

	l.Info("run...")

	os.Exit(m.Run())
}

func TestDriver_deploy(t *testing.T) {
	rc, err := remote.NewClient(c.k8sConnectionString, l)
	if err != nil {
		t.Errorf("Cannot create connection: %v", err)
		return
	}

	k8sDriver, err := k8s.NewDeployment(rc, c.k8sDeploymentFile, c.k8sSecretFile, l)
	defer k8sDriver.CleanUp()
	if err != nil {
		t.Errorf("Cannot create K8s deployment: %v", err)
		return
	}

	installed := t.Run("install driver", func(t *testing.T) {
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
	if !installed {
		t.Fatal()
	}

	t.Run("install nginx with dynamic volume provisioning", func(t *testing.T) {
		getNginxRunCommnad := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx nginx-storage-class-test-rw -- /bin/bash -c \"%v\"", cmd)
		}

		k8sNginx, err := k8s.NewDeployment(rc, "./_configs/nginx-storage-class-test-rw.yaml", "", l)
		defer k8sNginx.CleanUp()
		if err != nil {
			t.Fatalf("Cannot create K8s nginx deployment: %v", err)
		}

		if err := k8sNginx.Apply([]string{"nginx-storage-class-test-rw.*Running"}); err != nil {
			t.Fatal(err)
		}

		// write data to nginx container
		if _, err := rc.Exec(getNginxRunCommnad("echo 'test' > /usr/share/nginx/html/data.txt")); err != nil {
			t.Fatal(fmt.Errorf("Cannot write date to nginx volume: %v", err))
		}

		// check if data has been written
		if _, err := rc.Exec(getNginxRunCommnad("grep 'test' /usr/share/nginx/html/data.txt")); err != nil {
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %v", err))
		}

		if err := k8sNginx.Delete([]string{"nginx-storage-class-test-rw"}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("install nginx with dynamic volume provisioning [read only]", func(t *testing.T) {
		getNginxRunCommnad := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx nginx-storage-class-test-ro -- /bin/bash -c \"%v\"", cmd)
		}

		k8sNginx, err := k8s.NewDeployment(rc, "./_configs/nginx-storage-class-test-ro.yaml", "", l)
		defer k8sNginx.CleanUp()
		if err != nil {
			t.Fatalf("Cannot create K8s nginx deployment: %v", err)
		}

		if err := k8sNginx.Apply([]string{"nginx-storage-class-test-ro.*Running"}); err != nil {
			t.Fatal(err)
		}

		// writing data to read-only nginx container should failed
		if _, err := rc.Exec(getNginxRunCommnad("echo 'test' > /usr/share/nginx/html/data.txt")); err == nil {
			t.Fatal("Writing data to read-only volume on nginx container should failed, but it's not")
		} else if !strings.Contains(fmt.Sprint(err), "Read-only file system") {
			t.Fatalf("Error doesn't contain 'Read-only file system' rmessage")
		}

		if err := k8sNginx.Delete([]string{"nginx-storage-class-test-ro"}); err != nil {
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
}
