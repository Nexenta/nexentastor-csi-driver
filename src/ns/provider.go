package ns

import (
	"encoding/json"
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
	CreateFilesystem(params CreateFilesystemParams) error
	CreateNfsShare(params CreateNfsShareParams) error
	CreateSmbShare(params CreateSmbShareParams) error
	DeleteNfsShare(path string) error
	DeleteSmbShare(path string) error
	DestroyFilesystem(path string) error
	GetFilesystem(path string) (Filesystem, error)
	GetFilesystemAvailableCapacity(path string) (int64, error)
	GetFilesystems(parent string) ([]Filesystem, error)
	GetLicense() (License, error)
	GetPools() ([]string, error)
	GetSmbShareName(path string) (string, error) //TODO return *SmbShare
	IsJobDone(jobID string) (bool, error)
	LogIn() error
	SetFilesystemACL(path string, aclRuleSet ACLRuleSet) error
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

func (nsp *Provider) parseNefError(bodyBytes []byte, prefix string) error {
	var restErrorMessage string
	var restErrorCode string

	response := struct {
		Name    string `json:"name,omitempty"`
		Message string `json:"message,omitempty"`
		Errors  string `json:"errors,omitempty"`
		Code    string `json:"code,omitempty"`
	}{}

	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil
	}

	if response.Name != "" {
		restErrorMessage = fmt.Sprint(response.Name)
	}
	if response.Message != "" {
		restErrorMessage = fmt.Sprintf("%v: %v", restErrorMessage, response.Message)
	}
	if response.Errors != "" {
		restErrorMessage = fmt.Sprintf("%v, errors: [%v]", restErrorMessage, response.Errors)
	}
	if response.Code != "" {
		restErrorCode = response.Code
	}

	if len(restErrorMessage) > 0 {
		return &NefError{
			Err:  fmt.Errorf("%v: %v", prefix, restErrorMessage),
			Code: restErrorCode,
		}
	}

	return nil
}

func (nsp *Provider) sendRequestWithStruct(method, path string, data, response interface{}) error {
	bodyBytes, err := nsp.doAuthRequest(method, path, data)
	if err != nil {
		return err
	}

	if response != nil && len(bodyBytes) > 0 {
		if json.Valid(bodyBytes) {
			err := json.Unmarshal(bodyBytes, response)
			if err != nil || response == nil {
				err = fmt.Errorf(
					"Request '%s %s': cannot unmarshal JSON from: '%s' to '%+v': %s",
					method,
					path,
					bodyBytes,
					response,
					err,
				)
			}
		} else {
			err = fmt.Errorf("Request '%s %s' responded with invalid JSON: '%s'", method, path, bodyBytes)
		}
	}

	return err
}

func (nsp *Provider) sendRequest(method, path string, data interface{}) error {
	_, err := nsp.doAuthRequest(method, path, data)
	return err
}

func (nsp *Provider) doAuthRequest(method, path string, data interface{}) ([]byte, error) {
	l := nsp.Log.WithField("func", "doAuthRequest()")

	statusCode, bodyBytes, err := nsp.RestClient.Send(method, path, data)
	if err != nil {
		return bodyBytes, err
	}

	nefError := nsp.parseNefError(bodyBytes, "checking login status")

	// log in again if user is not logged in
	if statusCode == 401 && IsAuthNefError(nefError) {
		// do login call if used is not authorized in api
		l.Debugf("log in as '%v'...", nsp.Username)

		err = nsp.LogIn()
		if err != nil {
			return nil, err
		}

		// send original request again
		statusCode, bodyBytes, err = nsp.RestClient.Send(method, path, data)
		if err != nil {
			return bodyBytes, err
		}
	}

	if statusCode == http.StatusAccepted {
		// this is an async job
		var href string
		href, err = nsp.parseAsyncJobHref(bodyBytes)
		if err != nil {
			return bodyBytes, err
		}

		err = nsp.waitForAsyncJob(strings.TrimPrefix(href, "/jobStatus/"))
		if err != nil {
			l.Debugf("waitForAsyncJob() error: %v", err)
		}
	} else if statusCode >= 300 {
		nefError := nsp.parseNefError(bodyBytes, "request error")
		if nefError != nil {
			err = nefError
		} else {
			err = fmt.Errorf(
				"Request returned %v code, but response body doesn't contain explanation: %v",
				statusCode,
				bodyBytes,
			)
		}
	}

	return bodyBytes, err
}

func (nsp *Provider) parseAsyncJobHref(bodyBytes []byte) (string, error) {
	response := nefJobStatusResponse{}
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return "", fmt.Errorf("Cannot parse NS response '%s' to '%+v'", bodyBytes, response)
	}

	for _, link := range response.Links {
		if link.Rel == "monitor" && link.Href != "" {
			return link.Href, nil
		}
	}

	err := fmt.Errorf("Request return an async job, but response doesn't contain any links: %v", bodyBytes)
	return "", err
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
