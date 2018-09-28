package nexentastor

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/src/rest"
)

const (
	checkJobStatusInterval = 3 * time.Second
	checkJobStatusTimeout  = 60 * time.Second
)

// Provider - NexentaStore API provider
type Provider struct {
	address    string
	username   string
	password   string
	restClient rest.ClientInterface
	log        *logrus.Entry
}

// ProviderInterface - NexentaStor provider interface
type ProviderInterface interface {
	LogIn() error
	GetPools() ([]string, error)
	GetFilesystems(string) ([]string, error)
	CreateFilesystem(string) error
	DestroyFilesystem(string) error
	CreateNfsShare(string) error
	DeleteNfsShare(string) error
	IsJobDone(string) (bool, error)
}

func (nsp *Provider) parseNefError(resJSON map[string]interface{}, prefix string) error {
	var restErrorMessage string
	var restErrorCode string

	if name, ok := resJSON["name"]; ok {
		restErrorMessage = fmt.Sprint(name)
	}
	if message, ok := resJSON["message"]; ok {
		restErrorMessage = fmt.Sprintf("%v: %v", restErrorMessage, message)
	}
	if code, ok := resJSON["code"]; ok {
		restErrorCode = code.(string)
	}

	if len(restErrorMessage) > 0 {
		return &NefError{fmt.Errorf("%v: %v", prefix, restErrorMessage), restErrorCode}
	}

	return nil
}

func (nsp *Provider) doAuthRequest(method, path string, data map[string]interface{}) (
	map[string]interface{},
	error,
) {
	statusCode, resJSON, err := nsp.restClient.Send(method, path, data)
	if err != nil {
		return resJSON, err
	}

	// log in again if user is not logged in
	if statusCode == 401 && resJSON["code"] == "EAUTH" {
		// do login call if used is not authorized in api
		nsp.log.Infof("Log in as '%v'...", nsp.username)

		err = nsp.LogIn()
		if err != nil {
			return nil, err
		}

		// send original request again
		statusCode, resJSON, err = nsp.restClient.Send(method, path, data)
		if err != nil {
			return resJSON, err
		}
	}

	if statusCode == 202 {
		// this is an async job
		var href string
		href, err = nsp.getAsyncJobHref(resJSON)
		if err != nil {
			return resJSON, err
		}

		err = nsp.waitForAsyncJob(strings.TrimPrefix(href, "/jobStatus/"))
		if err != nil {
			nsp.log.Error(err)
		}
	} else if statusCode >= 300 {
		restError := nsp.parseNefError(resJSON, "request error")
		if restError != nil {
			err = restError
		} else {
			err = fmt.Errorf(
				"request returned %v code, but response body doesn't contain explanation: %v",
				statusCode,
				resJSON)
		}
	}

	return resJSON, err
}

func (nsp *Provider) getAsyncJobHref(resJSON map[string]interface{}) (string, error) {
	noFieldError := func(field string) error {
		return fmt.Errorf(
			"request return an async job, but links response doesn't contain '%v' field: %v",
			field,
			resJSON)
	}

	if links, ok := resJSON["links"].([]interface{}); ok && len(links) != 0 {
		link := links[0].(map[string]interface{})
		if rel, ok := link["rel"]; ok && rel == "monitor" {
			if val, ok := link["href"]; ok {
				return val.(string), nil
			}
			return "", noFieldError("href")
		}
		return "", noFieldError("rel")
	}

	return "", fmt.Errorf(
		"request return an async job, but response doesn't contain any links: %v",
		resJSON)
}

// waitForAsyncJob - keep asking for job status while it's not completed, return an error if timeout exceeded
func (nsp *Provider) waitForAsyncJob(jobID string) (err error) {
	jobLog := nsp.log.WithFields(logrus.Fields{
		"job": jobID,
	})

	done := make(chan error)
	timer := time.NewTimer(0)
	timeout := time.After(checkJobStatusTimeout)

	go func() {
		startTime := time.Now()
		for {
			select {
			case <-timer.C:
				jobDone, err := nsp.IsJobDone(jobID)
				if err != nil { // request failed
					done <- err
					return
				} else if jobDone { // job is completed
					done <- nil
					return
				}
				jobLog.Infof("Waiting job for %.0fs...", time.Since(startTime).Seconds())
				timer = time.NewTimer(checkJobStatusInterval)
			case <-timeout:
				timer.Stop()
				done <- fmt.Errorf("Exceeded timeout for checking job status")
				return
			}
		}
	}()

	return <-done
}

// ProviderArgs - params to create Provider instanse
type ProviderArgs struct {
	Address  string
	Username string
	Password string
	Log      *logrus.Entry
}

// NewProvider - create NexentaStor provider instance
func NewProvider(args ProviderArgs) (nsp ProviderInterface, err error) {
	providerLog := args.Log.WithFields(logrus.Fields{
		"cmp": "NexentaStorAPIProvider",
		"ns":  fmt.Sprint(args.Address),
	})

	providerLog.Debugf("Create for %v", args.Address)

	restClient, err := rest.NewClient(rest.ClientArgs{
		Address: args.Address,
		Log:     providerLog,
	})
	if err != nil {
		providerLog.Errorf("Cannot create REST client for: %v", args.Address)
	}

	nsp = &Provider{
		address:    args.Address,
		username:   args.Username,
		password:   args.Password,
		restClient: restClient,
		log:        providerLog,
	}

	return nsp, nil
}
