package k8s

import (
	"fmt"
	"path/filepath"
	"regexp"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/tests/utils/remote"
)

const (
	// default wait timeout
	defaultWaitTimeout = 120 * time.Second

	// default wait interval
	defaultWaitInterval = 2 * time.Second

	// default deployment tmp directory name (will be created in /tmp directory)
	defaultDeploymentTmpDirName = "deployment-files"

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
	log              *logrus.Entry
}

func (d *Deployment) createFormattedError(message string) func(error) error {
	return func(err error) error {
		return fmt.Errorf("K8s deployment failed (%s on %v): %s: %s", d.ConfigFile, d.RemoteClient, message, err)
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

func (mode WaitForPodsMode) String() string {
	switch mode {
	case WaitForPodsModeAllToMatch:
		return "waiting until all pods are presented"
	case WaitForPodsModeNoneToMatch:
		return "waiting until no pods are presented"
	default:
		return "BAD_VALUE"
	}
}

const (
	// WaitForPodsModeAllToMatch - all pods should be presented
	WaitForPodsModeAllToMatch WaitForPodsMode = iota

	// WaitForPodsModeNoneToMatch - none of pods should be presented
	WaitForPodsModeNoneToMatch WaitForPodsMode = iota
)

// WaitForPods - wait for pods to be presented
func (d *Deployment) WaitForPods(pods []string, mode WaitForPodsMode) (string, error) {
	l := d.log.WithField("func", "WaitForPods()")
	l.Infof("pods: %v, mode: %v", pods, mode)

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

				failedPodsList = []string{}
				for _, re := range podREs {
					switch mode {
					case WaitForPodsModeAllToMatch:
						if !re.MatchString(out) {
							failedPodsList = append(failedPodsList, re.String())
						}
					case WaitForPodsModeNoneToMatch:
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
					msg := fmt.Sprintf("waiting for %.0fs...", waitingTimeSeconds)
					if waitingTimeSeconds > d.WaitTimeout.Seconds()/2 {
						l.Warn(msg)
					} else {
						l.Infof(msg)
					}
				}
				timer = time.NewTimer(d.WaitInterval)
			case <-timeout:
				timer.Stop()
				done <- fmt.Errorf(
					"Checking cmd output timeout exceeded (%v), "+
						"failed pods: '%v', mode: %v, last output:\n"+
						"---\n%s---\n",
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
	l := d.log.WithField("func", "Apply()")
	fail := d.createFormattedError("Apply()")

	l.Info("run...")

	if err := d.RemoteClient.CopyFiles(d.ConfigFile, d.deploymentTmpDir); err != nil {
		return fail(err)
	}

	applyCommand := fmt.Sprintf("cd %s; kubectl apply -f %s", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(applyCommand); err != nil {
		return fail(err)
	}

	if pods != nil {
		out, err := d.WaitForPods(pods, WaitForPodsModeAllToMatch)
		if err != nil {
			return fail(err)
		}
		l.Infof("pods are running:\n---\n%s---", out)

		grepCommand := fmt.Sprintf("cd %s; grep \"image:\" %s", d.deploymentTmpDir, d.getConfigFileName())
		out, err = d.RemoteClient.Exec(grepCommand)
		if err != nil {
			l.Warnf("cannot get list of used images: %s; out: %s", err, out)
		} else {
			l.Infof("used images:\n---\n%s---", out)
		}
	}

	l.Info("deployment file has been successfully deployed")

	return nil
}

// Delete - run 'kubectl delete' for current deployment
// pods - wait for pods to be shutted down, nil to skip this step
func (d *Deployment) Delete(pods []string) error {
	l := d.log.WithField("func", "Delete()")
	fail := d.createFormattedError("Delete()")

	l.Info("run...")

	deleteCommand := fmt.Sprintf("cd %s; kubectl delete -f %s", d.deploymentTmpDir, d.getConfigFileName())
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		return fail(fmt.Errorf("Failed to delete k8s deployment: %s", err))
	}

	if pods != nil {
		out, err := d.WaitForPods(pods, WaitForPodsModeNoneToMatch)
		if err != nil {
			return fail(fmt.Errorf("Failed to wait for pods to be shutted down: %s", err))
		}
		l.Infof("pods after deletion:\n---\n%s---", out)
	}

	l.Info("deployment has been successfully deleted")

	return nil
}

// CreateSecret - run 'kubectl create secret' for current deployment
func (d *Deployment) CreateSecret() error {
	l := d.log.WithField("func", "CreateSecret()")
	fail := d.createFormattedError("CreateSecret()")

	l.Info("run...")

	if d.SecretFile == "" {
		return fail(fmt.Errorf("an attempt to create secret without config Deployment.SecretFile configured"))
	}

	if err := d.RemoteClient.CopyFiles(d.SecretFile, d.deploymentTmpDir); err != nil {
		return fail(err)
	}

	applyCommand := fmt.Sprintf(
		"cd %s; kubectl create secret generic %s --from-file=%s",
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

	l.Infof("kubernetes secrets:\n---\n%s---", out)
	l.Info("secret has been successfully created")

	return nil
}

// DeleteSecret - run 'kubectl delete secret' for current deployment
func (d *Deployment) DeleteSecret() error {
	l := d.log.WithField("func", "DeleteSecret()")
	fail := d.createFormattedError("DeleteSecret()")

	l.Info("run...")

	if _, err := d.RemoteClient.Exec(fmt.Sprintf("kubectl delete secret %s", d.secretName)); err != nil {
		return fail(err)
	}

	l.Info("secret has been successfully deleted")

	return nil
}

// CleanUp - silently delete k8s deployment and tmp folder
func (d *Deployment) CleanUp() {
	l := d.log.WithField("func", "CleanUp()")
	l.Info("run...")

	deleteCommand := fmt.Sprintf(
		"cd %s; kubectl delete --force --grace-period=0 -f %s | true",
		d.deploymentTmpDir,
		d.getConfigFileName(),
	)
	if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
		l.Errorf("failed run command on remote client: %s\n", err)
	}

	// wait all pods to be terminated
	t := 3 * time.Second
	d.log.Infof("sleep for %.0fs...", t.Seconds())
	time.Sleep(t)
	re := regexp.MustCompile(`.*Terminating.*`)
	if err := d.RemoteClient.ExecAndWaitRegExp("kubectl get pods", re, true); err != nil {
		l.Errorf("failed to shutdown pods: %s\n", err)
	}

	if d.secretName != "" {
		deleteCommand = fmt.Sprintf("kubectl delete secret %s | true", d.secretName)
		if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
			l.Errorf("failed to delete secret: %s\n", err)
		}

		if d.deploymentTmpDir != "" {
			deleteCommand = fmt.Sprintf("rm -f %s/%s | true", d.deploymentTmpDir, d.getSecretFileName())
			if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
				l.Errorf("failed to remove secret from tmp directory: %s\n", err)
			}
		}
	}

	if d.deploymentTmpDir != "" {
		deleteCommand = fmt.Sprintf("rm -f %s/%s | true", d.deploymentTmpDir, d.getConfigFileName())
		if _, err := d.RemoteClient.Exec(deleteCommand); err != nil {
			l.Errorf("failed to remove config from tmp directory: %s\n", err)
		}
	}

	l.Info("done.")
}

// DeploymentArgs - arguments for deployment
type DeploymentArgs struct {
	RemoteClient *remote.Client
	ConfigFile   string
	SecretFile   string
	SecretName   string
	Log          *logrus.Entry
}

// NewDeployment - create new k8s deployment
func NewDeployment(args DeploymentArgs) (*Deployment, error) {
	if args.RemoteClient == nil {
		return nil, fmt.Errorf("args.RemoteClient is required")
	} else if args.ConfigFile == "" {
		return nil, fmt.Errorf("args.ConfigFile is required")
	} else if args.Log == nil {
		return nil, fmt.Errorf("args.Log is required")
	}

	l := args.Log.WithFields(logrus.Fields{
		"address": args.RemoteClient.ConnectionString,
		"cmp":     "k8s",
		"name":    filepath.Base(args.ConfigFile),
	})

	deploymentTmpDir := filepath.Join("/tmp", defaultDeploymentTmpDirName)

	if _, err := args.RemoteClient.Exec(fmt.Sprintf("mkdir -p %s", deploymentTmpDir)); err != nil {
		return nil, fmt.Errorf(
			"NewDeployment(): cannot create '%s' directory on %+v",
			deploymentTmpDir,
			args.RemoteClient,
		)
	}

	secretName := ""
	if args.SecretFile != "" {
		if args.SecretName != "" {
			secretName = args.SecretName
		} else {
			secretName = defaultSecretName
		}
	}

	return &Deployment{
		RemoteClient:     args.RemoteClient,
		ConfigFile:       args.ConfigFile,
		SecretFile:       args.SecretFile,
		WaitTimeout:      defaultWaitTimeout,
		WaitInterval:     defaultWaitInterval,
		deploymentTmpDir: deploymentTmpDir,
		secretName:       secretName,
		log:              l,
	}, nil
}
