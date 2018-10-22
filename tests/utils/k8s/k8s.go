package k8s

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

const (
	// default wait timeout
	defaultWaitTimeout = 60 * time.Second

	// default wait interval
	defaultWaitInterval = 2 * time.Second

	// default deployment tmp directory name (will be created in /tmp directory)
	defaultDeploymentTmpDirName = "deploment-files"

	// default k8s secret name
	defaultSecretName = "nexentastor-csi-driver-config-tests"
)

// Deployment - k8s deployment
type Deployment struct {
	// RemoteClient - ssh client to connect through
	RemoteClient *remote.Client

	// ConfigFile - path to yaml config file for k8s
	ConfigFile string

	// SecretFile - path to yaml driver secret for k8s
	SecretFile string

	// WaitTimeout - consider waiting commands to fail after this timeout exceeded
	WaitTimeout time.Duration

	// WaitInterval - wait interval between checking pods status
	WaitInterval time.Duration

	deploymentTmpDir string
	secretName       string
}

func (d *Deployment) createFormattedLog(prefix string) func(string) {
	return func(message string) {
		fmt.Printf("%v %v: %v: %v\n", d.RemoteClient, d.ConfigFile, prefix, message)
	}
}

func (d *Deployment) createFormattedError(message string) func(error) error {
	return func(err error) error {
		return fmt.Errorf("K8s deployment failed (%v on %v): %v: %v", d.ConfigFile, d.RemoteClient, message, err)
	}
}

func (d *Deployment) getConfigFileName() string {
	return filepath.Base(d.ConfigFile)
}

func (d *Deployment) getSecretFileName() string {
	return filepath.Base(d.SecretFile)
}

// WaitForPodsMode - mode for WaitForPods() function (wait all pods or when none is presented)
type WaitForPodsMode int64

const (
	// WaitForPodsModeAll - all pods should be presented
	WaitForPodsModeAll WaitForPodsMode = iota

	// WaitForPodsModeNone - none of pods should be presented
	WaitForPodsModeNone WaitForPodsMode = iota
)

// WaitForPods - wait for pods to be presented
func (d *Deployment) WaitForPods(pods []string, mode WaitForPodsMode) (string, error) {
	done := make(chan error)
	timer := time.NewTimer(0)
	timeout := time.After(d.WaitTimeout)
	lastOutput := ""
	failedPodsList := []string{}

	podREs := make([]*regexp.Regexp, len(pods))
	for i, re := range pods {
		podREs[i] = regexp.MustCompile(re)
	}

	go func() {
		startTime := time.Now()
		for {
			select {
			case <-timer.C:
				out, err := d.RemoteClient.Exec("kubectl get pods")
				if err != nil {
					done <- err
					return
				}

				failedPodsList := []string{}
				for _, re := range podREs {
					switch mode {
					case WaitForPodsModeAll:
						if !re.MatchString(out) {
							failedPodsList = append(failedPodsList, re.String())
						}
					case WaitForPodsModeNone:
						if re.MatchString(out) {
							failedPodsList = append(failedPodsList, re.String())
						}
					default:
						done <- fmt.Errorf("WaitForPods() mode '%v' not supported", mode)
						return
					}
				}

				lastOutput = out
				if len(failedPodsList) == 0 {
					done <- nil
					return
				}

				waitingTimeSeconds := time.Since(startTime).Seconds()
				if waitingTimeSeconds >= d.WaitInterval.Seconds() {
					fmt.Printf("...waiting cmd for %.0fs\n", waitingTimeSeconds)
				}
				timer = time.NewTimer(d.WaitInterval)
			case <-timeout:
				timer.Stop()
				done <- fmt.Errorf(
					"Checking cmd output timeout exceeded (%v), "+
						"pods: '%v', mode: %v, last output:\n"+
						"---\n%v---\n",
					d.WaitTimeout,
					failedPodsList,
					mode,
					lastOutput,
				)
				return
			}
		}
	}()

	return lastOutput, <-done
}

// Apply - run 'kubectl apply' for current deployment
// pods - wait for pods to be running, nil to skip this step
func (d *Deployment) Apply(pods []string) error {
	log := d.createFormattedLog("Apply()")
	fail := d.createFormattedError("Apply()")

	log("run...")

	if err := d.RemoteClient.CopyFiles(d.ConfigFile, d.deploymentTmpDir); err != nil {
		return fail(err)
	}

	applyCommand := fmt.Sprintf("cd %v; kubectl apply -f %v", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(applyCommand); err != nil {
		return fail(err)
	}

	if pods != nil {
		out, err := d.WaitForPods(pods, WaitForPodsModeAll)
		if err != nil {
			return fail(err)
		}

		log(fmt.Sprintf("deployment file has been successfully deployed, pods are running:\n---\n%v---", out))

		return nil
	}

	log("deployment file has been successfully deployed")

	return nil
}

// Delete - run 'kubectl delete' for current deployment
// pods - wait for pods to be shutted down, nil to skip this step
func (d *Deployment) Delete(pods []string) error {
	log := d.createFormattedLog("Delete()")
	fail := d.createFormattedError("Delete()")

	log("run...")

	deleteCommand := fmt.Sprintf("cd %v; kubectl delete  -f %v", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		return fail(fmt.Errorf("Failed to delete k8s deployment: %v", err))
	}

	if pods != nil {
		out, err := d.WaitForPods(pods, WaitForPodsModeNone)
		if err != nil {
			return fail(fmt.Errorf("Failed to wait for pods to be shutted down: %v", err))
		}
		log(fmt.Sprintf("pods after deletion:\n---\n%v---", out))
	}

	return nil
}

// CreateSecret - run 'kubectl create secret' for current deployment
func (d *Deployment) CreateSecret() error {
	log := d.createFormattedLog("CreateSecret()")
	fail := d.createFormattedError("CreateSecret()")

	log("run...")

	if d.SecretFile == "" {
		return fail(fmt.Errorf("an attempt to create secret without config Deployment.SecretFile configured"))
	}

	if err := d.RemoteClient.CopyFiles(d.SecretFile, d.deploymentTmpDir); err != nil {
		return fail(err)
	}

	applyCommand := fmt.Sprintf(
		"cd %v; kubectl create secret generic %v --from-file=%v",
		d.deploymentTmpDir,
		d.secretName,
		d.getSecretFileName(),
	)
	if _, err := d.RemoteClient.Exec(applyCommand); err != nil {
		return fail(err)
	}

	out, err := d.RemoteClient.Exec("kubectl get secrets")
	if err != nil {
		return fail(err)
	}
	log(fmt.Sprintf("kubernetis secrets:\n---\n%v---", out))

	log("secret has been successfully created")

	return nil
}

// DeleteSecret - run 'kubectl delete secret' for current deployment
func (d *Deployment) DeleteSecret() error {
	log := d.createFormattedLog("DeleteSecret()")
	fail := d.createFormattedError("DeleteSecret()")

	log("run...")

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("kubectl delete secret %v", d.secretName)); err != nil {
		return fail(err)
	}

	log("secret has been successfully deleted")

	return nil
}

// CleanUp - silently delete k8s deployment and tmp folder
func (d *Deployment) CleanUp() {
	deleteCommand := fmt.Sprintf("cd %v; kubectl delete -f %v | true", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		fmt.Printf("CleanUp(): failed to delete pods: %v\n", err)
	}

	// wait all pods to be terminated
	time.Sleep(3 * time.Second)
	re := regexp.MustCompile(`.*Terminating.*`)
	if err := d.RemoteClient.ExecAndWaitRegExp("kubectl get pods", re, true); err != nil {
		fmt.Printf("CleanUp(): failed to shutdown pods: %v\n", err)
	}

	if d.secretName != "" {
		deleteCommand = fmt.Sprintf("kubectl delete secret %v | true", d.secretName)
		if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
			fmt.Printf("CleanUp(): failed to delete secret: %v\n", err)
		}
	}

	deleteCommand = fmt.Sprintf("rm -f %v/%v | true", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		fmt.Printf("CleanUp(): failed to remove config from tmp directory: %v\n", err)
	}

	deleteCommand = fmt.Sprintf("rm -f %v/%v | true", d.deploymentTmpDir, d.getSecretFileName())
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		fmt.Printf("CleanUp(): failed to remove secret from tmp directory: %v\n", err)
	}
}

// NewDeployment - create new k8s deployment
func NewDeployment(remoteClient *remote.Client, configFile string, secretFile string) (*Deployment, error) {
	deploymentTmpDir := filepath.Join("/tmp", defaultDeploymentTmpDirName)

	if _, err := remoteClient.Exec(fmt.Sprintf("mkdir -p %v", deploymentTmpDir)); err != nil {
		return nil, fmt.Errorf("NewDeployment(): cannot create '%v' directory on %v", deploymentTmpDir, remoteClient)
	}

	secretName := ""
	if secretFile != "" {
		secretName = defaultSecretName
	}

	return &Deployment{
		RemoteClient:     remoteClient,
		ConfigFile:       configFile,
		SecretFile:       secretFile,
		WaitTimeout:      defaultWaitTimeout,
		WaitInterval:     defaultWaitInterval,
		deploymentTmpDir: deploymentTmpDir,
		secretName:       secretName,
	}, nil
}
