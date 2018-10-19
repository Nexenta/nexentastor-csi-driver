package driver_test

import (
	"testing"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/k8s"
	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

const (
	// ConnectionString - user@host for ssh command
	ConnectionString = "root@10.3.199.250"

	// CMDWaitInterval - run command every N seconds to check the output
	CMDWaitInterval = 2 * time.Second

	// CMDWaitTimeout - consider command to fail after this timeout exceeded
	CMDWaitTimeout = 60 * time.Second

	DeploymentTmpDirName = "deploment-files"
)

func TestDriver_deploy(t *testing.T) {
	rc := &remote.Client{
		ConnectionString: ConnectionString,
		CMDWaitInterval:  CMDWaitInterval,
		CMDWaitTimeout:   CMDWaitTimeout,
	}

	t.Run("install/uninstall driver", func(t *testing.T) {
		deploymentFile := "nexentastor-csi-driver-master-local.yaml"

		k8sDeployment := k8s.NewDeployment(rc, deploymentFile)

		err := k8sDeployment.Apply([]string{
			"nexentastor-csi-attacher-.*Running",
			"nexentastor-csi-provisioner-.*Running",
			"nexentastor-csi-driver-.*Running",
		})
		if err != nil {
			k8sDeployment.CleanUp()
			t.Error(err)
			return
		}

		err = k8sDeployment.Delete([]string{
			"nexentastor-csi-attacher-.*",
			"nexentastor-csi-provisioner-.*",
			"nexentastor-csi-driver-.*",
		})
		if err != nil {
			k8sDeployment.CleanUp()
			t.Error(err)
			return
		}
	})
}
