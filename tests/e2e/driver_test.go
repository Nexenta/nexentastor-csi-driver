package driver_test

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	nested "github.com/antonfisher/nested-logrus-formatter"
	"github.com/educlos/testrail"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/k8s"
	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

const (
	defaultSecretName = "nexentastor-csi-driver-config"
)

type config struct {
	k8sConnectionString string
	k8sDeploymentFile   string
	k8sSecretFile       string
	k8sSecretName       string
}

var c *config
var l *logrus.Entry
var username = os.Getenv("TESTRAIL_USR")
var password = os.Getenv("TESTRAIL_PSWD")
var url = os.Getenv("TESTRAIL_URL")
var testResult testrail.SendableResult

func TestMain(m *testing.M) {
	var (
		k8sConnectionString = flag.String("k8sConnectionString", "", "K8s connection string [user@host]")
		k8sDeploymentFile   = flag.String("k8sDeploymentFile", "", "path to driver deployment yaml file")
		k8sSecretFile       = flag.String("k8sSecretFile", "", "path to yaml driver config file (for k8s secret)")
		k8sSecretName       = flag.String("k8sSecretName", defaultSecretName, "k8s secret name")
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
		k8sSecretName:       *k8sSecretName,
	}

	// init logger
	l = logrus.New().WithField("title", "tests")

	noColors := false
	if v := os.Getenv("NOCOLORS"); v != "" && v != "false" {
		noColors = true
	}

	// logger formatter
	l.Logger.SetFormatter(&nested.Formatter{
		HideKeys:    true,
		FieldsOrder: []string{"title", "address", "cmp", "name", "func"},
		NoColors:    noColors,
	})

	l.Info("run...")
	l.Info("Config:")
	l.Infof(" - k8s server:    %s", c.k8sConnectionString)
	l.Infof(" - driver yaml:   %s", c.k8sDeploymentFile)
	l.Infof(" - driver config: %s", c.k8sSecretFile)
	l.Infof(" - secret name:   %s", c.k8sSecretName)

	os.Exit(m.Run())
}

func TestDriver_deploy(t *testing.T) {
	// connect to TestRail
	client := testrail.NewClient(url, username, password)

	rc, err := remote.NewClient(c.k8sConnectionString, l)
	if err != nil {
		t.Errorf("Cannot create connection: %s", err)
		return
	}

	// DEBUG
	deb, err := rc.Exec("echo $TESTRAIL_USR")
	if err != nil {
		t.Errorf("cannot get env var TESTRAIL_USR: %s; out: %s", err, deb)
		return
	}

	out, err := rc.Exec("kubectl version")
	if err != nil {
		t.Errorf("cannot get kubectl version: %s; out: %s", err, out)
		return
	}
	t.Logf("kubectl version:\n%s", out)
	l.Infof("kubectl version:\n%s", out)

	k8sDriver, err := k8s.NewDeployment(k8s.DeploymentArgs{
		RemoteClient: rc,
		ConfigFile:   c.k8sDeploymentFile,
		SecretFile:   c.k8sSecretFile,
		SecretName:   c.k8sSecretName,
		Log:          l,
	})
	defer k8sDriver.CleanUp()
	defer k8sDriver.Delete(nil)
	if err != nil {
		t.Errorf("Cannot create K8s deployment: %s", err)
		return
	}

	installed := t.Run("install driver", func(t *testing.T) {
		t.Log("create k8s secret for driver")
		k8sDriver.DeleteSecret()
		if err := k8sDriver.CreateSecret(); err != nil {
			t.Fatal(err)
		}

		waitPods := []string{
			"nexentastor-csi-controller-.*Running",
			"nexentastor-csi-node-.*Running",
		}

		t.Log("instal the driver")
		if err := k8sDriver.Apply(waitPods); err != nil {
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Installation - success"
		if _, err := client.AddResultForCase(5151, 706718, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}
		t.Log("done.")
	})
	if !installed {
		testResult.StatusID = 5
		testResult.Comment = "Installation - failed"
		if _, err := client.AddResultForCase(5151, 706718, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}
		t.Fatal()
	}

	t.Run("deploy nginx pod with dynamic volume provisioning [read-write]", func(t *testing.T) {
		nginxPodName := "nginx-dynamic-volume"
		testResult.StatusID = 5
		testResult.Comment = "Create Pod and Mount Volume - failed"

		getNginxRunCommand := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", nginxPodName, cmd)
		}

		k8sNginx, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-dynamic-volume.yaml",
			Log:          l,
		})
		defer k8sNginx.CleanUp()
		defer k8sNginx.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}

		t.Log("deploy nginx container with read-write volume")
		if err := k8sNginx.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("write data to the volume")
		if _, err := rc.Exec(getNginxRunCommand("echo 'test' > /usr/share/nginx/html/data.txt")); err != nil {
			if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Cannot write data to nginx volume: %s", err))
		}

		t.Log("check if the data has been written to the volume")
		if _, err := rc.Exec(getNginxRunCommand("grep 'test' /usr/share/nginx/html/data.txt")); err != nil {
			if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("delete the nginx container with read-write volume")
		if err := k8sNginx.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Create Pod and Mount Volume - success"
		if _, err := client.AddResultForCase(5151, 717580, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})

	t.Run("deploy nginx pod with dynamic volume provisioning [read-only]", func(t *testing.T) {
		nginxPodName := "nginx-dynamic-volume-ro"
		testResult.StatusID = 5
		testResult.Comment = "Create Pod and Mount Volume - failed"

		getNginxRunCommand := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", nginxPodName, cmd)
		}

		k8sNginx, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-dynamic-volume-ro.yaml",
			Log:          l,
		})
		defer k8sNginx.CleanUp()
		defer k8sNginx.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}

		t.Log("deploy nginx container with read-only volume")
		if err := k8sNginx.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("write data to a read-only volume should failed")
		if _, err := rc.Exec(getNginxRunCommand("echo 'test' > /usr/share/nginx/html/data.txt")); err == nil {
			if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal("Writing data to read-only volume on nginx container should failed, but it's not")
		} else if !strings.Contains(fmt.Sprint(err), "Read-only file system") {
			if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Error doesn't contain 'Read-only file system' message: %s", err)
		}

		t.Log("delete the nginx container with read-only volume")
		if err := k8sNginx.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Create Pod and Mount Volume - success"
		if _, err := client.AddResultForCase(5151, 795976, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})

	t.Run("deploy nginx pod with persistent volume", func(t *testing.T) {
		command := ""
		data := time.Now().Format(time.RFC3339)
		nginxPodName := "nginx-persistent-volume"
		testResult.StatusID = 5
		testResult.Comment = "Create Pod and Mount Volume - failed"

		getNginxRunCommand := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", nginxPodName, cmd)
		}

		t.Log("deploy first nginx container with persistent volume")
		k8sNginx1, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-persistent-volume.yaml",
			Log:          l,
		})
		defer k8sNginx1.CleanUp()
		defer k8sNginx1.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}
		if err := k8sNginx1.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("write data to the first nginx container persistent volume")
		command = fmt.Sprintf("echo '%s' > /usr/share/nginx/html/data.txt", data)
		if _, err := rc.Exec(getNginxRunCommand(command)); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Cannot write date to nginx volume: %s", err))
		}

		t.Log("check if data has been written")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data)
		if _, err := rc.Exec(getNginxRunCommand(command)); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("delete the first nginx container")
		if err := k8sNginx1.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("deploy second container with the same persistent volume")
		k8sNginx2, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-persistent-volume.yaml",
			Log:          l,
		})
		defer k8sNginx2.CleanUp()
		defer k8sNginx2.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}
		if err := k8sNginx2.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		// check if data is still there
		t.Log("check if data in the persistent volume is the same as it was before")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data)
		if _, err := rc.Exec(getNginxRunCommand(command)); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("delete the second nginx container")
		if err := k8sNginx2.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Create Pod and Mount Volume - success"
		if _, err := client.AddResultForCase(5151, 717581, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})

	t.Run("restore snapshot as dynamically provisioned volume", func(t *testing.T) {
		command := ""
		data1 := "DATA1: " + time.Now().Format(time.RFC3339)
		data2 := "DATA2: " + time.Now().Format(time.RFC1123)
		nginxPodName := "nginx-persistent-volume"
		nginxSnapshotPodName := "nginx-persistent-volume-snapshot-restore"

		testResult.StatusID = 5
		testResult.Comment = "Recovery from snapshot - failed"

		getNginxRunCommand := func(podName, cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", podName, cmd)
		}

		t.Log("deploy first nginx container with persistent volume")
		k8sNginx1, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-persistent-volume.yaml",
			Log:          l,
		})
		defer k8sNginx1.CleanUp()
		defer k8sNginx1.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}
		if err := k8sNginx1.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("write data to the first nginx container")
		command = fmt.Sprintf("echo '%s' > /usr/share/nginx/html/data.txt", data1)
		if _, err := rc.Exec(getNginxRunCommand(nginxPodName, command)); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Cannot write date to nginx volume: %s", err))
		}

		t.Log("validate data in the first nginx container")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data1)
		if _, err := rc.Exec(getNginxRunCommand(nginxPodName, command)); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("create snapshot class")
		k8sSnapshotClass, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/snapshot-class.yaml",
			Log:          l,
		})
		defer k8sSnapshotClass.CleanUp()
		defer k8sSnapshotClass.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s snapshots class deployment: %s", err)
		}
		if err := k8sSnapshotClass.Apply(nil); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("take a snapshot from existing persistent volume")
		k8sSnapshot, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/take-snapshot.yaml",
			Log:          l,
		})
		defer k8sSnapshot.CleanUp()
		defer k8sSnapshot.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s snapshot deployment: %s", err)
		}
		if err := k8sSnapshot.Apply(nil); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		// Some time, snapshot need extra time to be created (?).
		// Some time data gets overwritten before snapshot is taken,
		// so there is a delay before the overwrite.
		// More easily reproducible on SMB share test runs.
		time.Sleep(10 * time.Second)

		t.Log("overwrite the data in the first nginx container")
		command = fmt.Sprintf("echo '%s' > /usr/share/nginx/html/data.txt", data2)
		if _, err := rc.Exec(getNginxRunCommand(nginxPodName, command)); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Cannot write date to nginx volume: %s", err))
		}

		t.Log("validate overwritten data in the first nginx container")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data2)
		if _, err := rc.Exec(getNginxRunCommand(nginxPodName, command)); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("delete the first nginx container")
		if err := k8sNginx1.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("deploy second container using the snapshot")
		k8sNginx2, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-snapshot-volume.yaml",
			Log:          l,
		})
		defer k8sNginx2.CleanUp()
		defer k8sNginx2.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx from snapshot deployment: %s", err)
		}
		if err := k8sNginx2.Apply([]string{nginxSnapshotPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("validate that the second nginx container has data from the snapshot")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data1)
		if _, err := rc.Exec(getNginxRunCommand(nginxSnapshotPodName, command)); err != nil {
			command = fmt.Sprintf("cat /usr/share/nginx/html/data.txt")
			out, readErr := rc.Exec(getNginxRunCommand(nginxSnapshotPodName, command))
			if readErr != nil {
				out = readErr.Error()
			}
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf(
				"Data hasn't been found in the container with volume restored from snapshot, "+
					"expected: '%s', got: '%s', error: %s",
				data1,
				out,
				err,
			))
		}
		if err := k8sNginx2.Delete([]string{nginxSnapshotPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("deploy third container with the same persistent volume")
		k8sNginx3, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-persistent-volume.yaml",
			Log:          l,
		})
		defer k8sNginx3.CleanUp()
		defer k8sNginx3.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}
		if err := k8sNginx3.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("validate the third nginx container has data from persistent volume")
		command = fmt.Sprintf("grep '%s' /usr/share/nginx/html/data.txt", data2)
		if _, err := rc.Exec(getNginxRunCommand(nginxPodName, command)); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}
		if err := k8sNginx3.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Recovery from snapshot - success"
		if _, err := client.AddResultForCase(5151, 706745, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})

	t.Run("volume cloning check", func(t *testing.T) {
		nginxPodName := "nginx-dynamic-volume"
		nginxClonePodName := "nginx-dynamic-volume-clone"

		testResult.StatusID = 5
		testResult.Comment = "Volume clone - failed"

		getNginxRunCommand := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", nginxPodName, cmd)
		}
		getNginxCloneRunCommand := func(cmd string) string {
			return fmt.Sprintf("kubectl exec -c nginx %s -- /bin/bash -c \"%s\"", nginxClonePodName, cmd)
		}

		k8sNginx, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-dynamic-volume.yaml",
			Log:          l,
		})
		defer k8sNginx.CleanUp()
		defer k8sNginx.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}

		t.Log("deploy nginx container with read-write volume")
		if err := k8sNginx.Apply([]string{nginxPodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("write data to the volume")
		if _, err := rc.Exec(getNginxRunCommand("echo 'test' > /usr/share/nginx/html/data.txt")); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Cannot write data to nginx volume: %s", err))
		}

		t.Log("check if the data has been written to the volume")
		if _, err := rc.Exec(getNginxRunCommand("grep 'test' /usr/share/nginx/html/data.txt")); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		k8sNginxClone, err := k8s.NewDeployment(k8s.DeploymentArgs{
			RemoteClient: rc,
			ConfigFile:   "../../examples/kubernetes/nginx-clone-volume.yaml",
			Log:          l,
		})
		defer k8sNginxClone.CleanUp()
		defer k8sNginxClone.Delete(nil)
		if err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatalf("Cannot create K8s nginx deployment: %s", err)
		}

		t.Log("deploy nginx container with cloned volume")
		if err := k8sNginxClone.Apply([]string{nginxClonePodName + ".*Running"}); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("check if the data has been written to the cloned volume")
		if _, err := rc.Exec(getNginxCloneRunCommand("grep 'test' /usr/share/nginx/html/data.txt")); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(fmt.Errorf("Data hasn't been written to nginx container: %s", err))
		}

		t.Log("delete the nginx clone container with read-write volume")
		if err := k8sNginxClone.Delete([]string{nginxClonePodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("delete the nginx container with read-write volume")
		if err := k8sNginx.Delete([]string{nginxPodName}); err != nil {
			if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Volume clone - success"
		if _, err := client.AddResultForCase(5151, 795977, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})

	t.Run("uninstall driver", func(t *testing.T) {
		t.Log("deleting the driver")
		if err := k8sDriver.Delete([]string{"nexentastor-csi-.*"}); err != nil {
			testResult.StatusID = 5
			testResult.Comment = "Uninstallation - failed"
			if _, err := client.AddResultForCase(5151, 706721, testResult); err != nil {
				l.Warn("Can't add test result to TestRail")
			}
			t.Fatal(err)
		}

		t.Log("deleting the secret")
		if err := k8sDriver.DeleteSecret(); err != nil {
			t.Fatal(err)
		}

		testResult.StatusID = 1
		testResult.Comment = "Uninstallation - success"
		if _, err := client.AddResultForCase(5151, 706721, testResult); err != nil {
			l.Warn("Can't add test result to TestRail")
		}

		t.Log("done.")
	})
}
