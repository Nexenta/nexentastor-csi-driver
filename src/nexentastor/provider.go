package nexentastor

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"net/url"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/src/rest"
)

// Provider - NexentaStore provider connecter for API
type Provider struct {
	address    string
	username   string
	password   string
	restClient rest.ClientInterface
	log        *logrus.Entry
}

// ProviderInterface - NexentaStor providev interface
type ProviderInterface interface {
	GetPools() ([]string, error)
	GetFilesystems(pool string) ([]string, error)
	CreateFilesystem(path string) error
	DestroyFilesystem(path string) error
	LogIn() error
}

// LogIn - log in to NexentaStor API and get auth token
func (nsp *Provider) LogIn() error {
	data := make(map[string]interface{})
	data["username"] = nsp.username
	data["password"] = nsp.password

	_, resJSON, err := nsp.restClient.Send("POST", "auth/login", data)
	if err != nil {
		return err
	}

	if token, ok := resJSON["token"]; ok {
		nsp.restClient.SetAuthToken(fmt.Sprint(token))
		nsp.log.Info("Login token has been updated")
		return nil
	}

	// try to parse error from rest response
	restError := nsp.parseNefError(resJSON, "Login request")
	if restError != nil {
		code := restError.(*NefError).Code
		if code == "EAUTH" {
			nsp.log.Errorf(
				"Login to NexentaStor %v failed (username: '%v'), "+
					"please make sure to use correct address and password",
				nsp.address,
				nsp.username)
		}
		return restError
	}

	return fmt.Errorf("Login request: No token found in response: %v", resJSON)
}

// GetPools - get NexentaStor pools
func (nsp *Provider) GetPools() ([]string, error) {
	uri := nsp.restClient.BuildURI("/storage/pools", map[string]string{
		"fields": "poolName,health,status",
	})

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	pools := []string{}

	if data, ok := resJSON["data"]; ok {
		for _, val := range data.([]interface{}) {
			pool := val.(map[string]interface{})
			pools = append(pools, fmt.Sprint(pool["poolName"]))
		}
	} else {
		nsp.log.Warnf("response doesn't contain 'data' property: %v", resJSON)
	}

	return pools, nil
}

// GetFilesystems - get NexentaStor filesystems
func (nsp *Provider) GetFilesystems(pool string) ([]string, error) {
	uri := nsp.restClient.BuildURI("/storage/filesystems", map[string]string{
		"pool":   pool,
		"fields": "path",
	})

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	filesystems := []string{}

	if data, ok := resJSON["data"]; ok {
		for _, val := range data.([]interface{}) {
			filesystem := val.(map[string]interface{})
			filesystems = append(filesystems, fmt.Sprint(filesystem["path"]))
		}
	}

	return filesystems, nil
}

// CreateFilesystem - create filesystem by path
func (nsp *Provider) CreateFilesystem(path string) error {
	data := make(map[string]interface{})
	data["path"] = path

	_, err := nsp.doAuthRequest("POST", "/storage/filesystems", data)

	return err
}

// DestroyFilesystem - destroy filesystem by path
func (nsp *Provider) DestroyFilesystem(path string) error {
	data := make(map[string]interface{})
	data["path"] = path

	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	uri := fmt.Sprintf("/storage/filesystems/%v", url.PathEscape(path))

	_, err := nsp.doAuthRequest("DELETE", uri, nil)

	return err
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

		err = nsp.waitForAsyncJob(href)
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

func (nsp *Provider) waitForAsyncJob(uri string) (err error) {
	jobLog := nsp.log.WithFields(logrus.Fields{
		"job": uri,
	})
	sleepTime := 3 * time.Second
	attemptsLimit := 30 //>1.5min

	for attempts := 0; attempts <= attemptsLimit; attempts++ {
		statusCode, resJSON, err := nsp.restClient.Send("GET", uri, nil)
		if err != nil { // request failed
			return err
		} else if statusCode == 200 || statusCode == 201 { // job is completed
			return nil
		} else if statusCode != 202 { // job is failed
			restError := nsp.parseNefError(resJSON, "Job request error")
			if restError != nil {
				err = restError
			} else {
				err = fmt.Errorf(
					"job request returned %v code, but response body doesn't contain explanation: %v",
					statusCode,
					resJSON)
			}
			jobLog.Error(err)
			return err
		}

		jobLog.Info("Waiting for job")
		time.Sleep(sleepTime)
	}

	err = fmt.Errorf("Exceeded timeout for job status")
	jobLog.Error(err)
	return err
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
