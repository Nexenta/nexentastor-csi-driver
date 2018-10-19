package k8s

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

const waitInterval = 2 * time.Second

// Deployment - k8s deployment
type Deployment struct {
	// RemoteClient - ssh client to connect through
	RemoteClient *remote.Client

	// ConfigFile - path to yaml config file for k8s
	ConfigFile string

	// WaitTimeout - consider waiting commands to fail after this timeout exceeded
	WaitTimeout time.Duration

	deploymentTmpDir string
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
				if waitingTimeSeconds >= waitInterval.Seconds() {
					fmt.Printf("...waiting cmd for %.0fs\n", waitingTimeSeconds)
				}
				timer = time.NewTimer(waitInterval)
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

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("rm -rf %v | true", d.deploymentTmpDir)); err != nil {
		return fail(err)
	}

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("mkdir -p %v", d.deploymentTmpDir)); err != nil {
		return fail(err)
	}

	if err := d.RemoteClient.CopyFiles(d.ConfigFile, d.deploymentTmpDir); err != nil {
		return fail(err)
	}

	applyCommand := fmt.Sprintf("cd %v; kubectl apply -f %v", d.deploymentTmpDir, d.ConfigFile)
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

	deleteCommand := fmt.Sprintf("cd %v; kubectl delete  -f %v", d.deploymentTmpDir, d.ConfigFile)
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

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("rm -rf %v", d.deploymentTmpDir)); err != nil {
		return fail(fmt.Errorf("Failed to remove tmp directory: %v", err))
	}

	return nil
}

// CleanUp - silently delete k8s deployment and tmp folder
func (d *Deployment) CleanUp() {
	deleteCommand := fmt.Sprintf("cd %v; kubectl delete  -f %v", d.deploymentTmpDir, d.ConfigFile)
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		fmt.Printf("CleanUp(): failed to delete pods: %v", err)
	}

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("rm -rf %v | true", d.deploymentTmpDir)); err != nil {
		fmt.Printf("CleanUp(): failed to remove tmp directory: %v", err)
	}
}

// NewDeployment - create new k8s deployment
func NewDeployment(remoteClient *remote.Client, configFile string) *Deployment {
	deploymentTmpDirName := "deploment-files" //TODO configurable?
	waitTimeout := 30 * time.Second           //TODO configurable?

	return &Deployment{
		RemoteClient:     remoteClient,
		ConfigFile:       configFile,
		WaitTimeout:      waitTimeout,
		deploymentTmpDir: filepath.Join("/tmp", deploymentTmpDirName),
	}
}
