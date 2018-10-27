package ns

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/rest"
)

const (
	checkJobStatusInterval = 3 * time.Second
	checkJobStatusTimeout  = 60 * time.Second
)

// ProviderInterface - NexentaStor provider interface
type ProviderInterface interface {
	LogIn() error
	GetPools() ([]string, error)
	GetFilesystem(string) (*Filesystem, error)
	GetFilesystems(string) ([]*Filesystem, error)
	CreateFilesystem(string, map[string]interface{}) error
	DestroyFilesystem(string) error
	CreateNfsShare(string) error
	DeleteNfsShare(string) error
	SetFilesystemACL(string, ACLRuleSet) error
	IsJobDone(string) (bool, error)
}

// Provider - NexentaStor API provider
type Provider struct {
	Address    string
	Username   string
	Password   string
	RestClient rest.ClientInterface
	Log        *logrus.Entry
}

func (nsp *Provider) String() string {
	return nsp.Address
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
	if errors, ok := resJSON["errors"]; ok {
		restErrorMessage = fmt.Sprintf("%v, errors: [%v]", restErrorMessage, errors)
	}
	if code, ok := resJSON["code"]; ok {
		restErrorCode = code.(string)
	}

	if len(restErrorMessage) > 0 {
		return &NefError{fmt.Errorf("%v: %v", prefix, restErrorMessage), restErrorCode}
	}

	return nil
}

func (nsp *Provider) doAuthRequest(method, path string, data interface{}) (
	map[string]interface{},
	error,
) {
	l := nsp.Log.WithField("func", "doAuthRequest()")

	statusCode, resJSON, err := nsp.RestClient.Send(method, path, data)
	if err != nil {
		return resJSON, err
	}

	// log in again if user is not logged in
	if statusCode == 401 && resJSON["code"] == "EAUTH" {
		// do login call if used is not authorized in api
		l.Debugf("log in as '%v'...", nsp.Username)

		err = nsp.LogIn()
		if err != nil {
			return nil, err
		}

		// send original request again
		statusCode, resJSON, err = nsp.RestClient.Send(method, path, data)
		if err != nil {
			return resJSON, err
		}
	}

	if statusCode == http.StatusAccepted {
		// this is an async job
		var href string
		href, err = nsp.getAsyncJobHref(resJSON)
		if err != nil {
			return resJSON, err
		}

		err = nsp.waitForAsyncJob(strings.TrimPrefix(href, "/jobStatus/"))
		if err != nil {
			l.Error(err)
		}
	} else if statusCode >= 300 {
		restError := nsp.parseNefError(resJSON, "request error")
		if restError != nil {
			err = restError
		} else {
			err = fmt.Errorf(
				"Request returned %v code, but response body doesn't contain explanation: %v",
				statusCode,
				resJSON)
		}
	}

	return resJSON, err
}

func (nsp *Provider) getAsyncJobHref(resJSON map[string]interface{}) (string, error) {
	noFieldError := func(field string) error {
		return fmt.Errorf(
			"Request return an async job, but links response doesn't contain '%v' field: %v",
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
		"Request return an async job, but response doesn't contain any links: %v",
		resJSON)
}

// waitForAsyncJob - keep asking for job status while it's not completed, return an error if timeout exceeded
func (nsp *Provider) waitForAsyncJob(jobID string) (err error) {
	l := nsp.Log.WithField("job", jobID)

	timer := time.NewTimer(0)
	timeout := time.After(checkJobStatusTimeout)
	startTime := time.Now()

	for {
		select {
		case <-timer.C:
			jobDone, err := nsp.IsJobDone(jobID)
			if err != nil { // request failed
				return err
			} else if jobDone { // job is completed
				return nil
			} else {
				waitingTimeSeconds := time.Since(startTime).Seconds()
				if waitingTimeSeconds >= checkJobStatusInterval.Seconds() {
					l.Warnf("waiting job for %.0fs...", waitingTimeSeconds)
				}
				timer = time.NewTimer(checkJobStatusInterval)
			}
		case <-timeout:
			timer.Stop()
			return fmt.Errorf("Checking job status timeout exceeded (%vs)", checkJobStatusTimeout)
		}
	}
}

// ProviderArgs - params to create Provider instance
type ProviderArgs struct {
	Address  string
	Username string
	Password string
	Log      *logrus.Entry
}

// NewProvider - create NexentaStor provider instance
func NewProvider(args ProviderArgs) (nsp ProviderInterface, err error) {
	l := args.Log.WithFields(logrus.Fields{
		"cmp": "NSProvider",
		"ns":  fmt.Sprint(args.Address),
	})

	l.Debugf("created for %v", args.Address)

	restClient, err := rest.NewClient(rest.ClientArgs{
		Address: args.Address,
		Log:     l,
	})
	if err != nil {
		l.Errorf("cannot create REST client for: %v", args.Address)
	}

	nsp = &Provider{
		Address:    args.Address,
		Username:   args.Username,
		Password:   args.Password,
		RestClient: restClient,
		Log:        l,
	}

	return nsp, nil
}
