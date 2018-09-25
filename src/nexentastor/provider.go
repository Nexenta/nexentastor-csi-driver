package nexentastor

import (
	"fmt"
	"github.com/sirupsen/logrus"

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
		nsp.log.Info("Login token was updated")
		return nil
	}

	if restErrorMessage, ok := nsp.formatRestErrorMessage(resJSON); ok {
		return fmt.Errorf("Login request: %v", restErrorMessage)
	}

	return fmt.Errorf("Login request: No token found in response: %v", resJSON)
}

// GetPools - get NexentaStor pools
func (nsp *Provider) GetPools() ([]string, error) {
	resJSON, err := nsp.doAuthRequest("GET", "storage/pools?fields=poolName,health,status", nil)
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

func (nsp *Provider) formatRestErrorMessage(resJSON map[string]interface{}) (string, bool) {
	var restErrorMessage string
	if name, ok := resJSON["name"]; ok {
		restErrorMessage = fmt.Sprint(name)
	}
	if message, ok := resJSON["message"]; ok {
		restErrorMessage = fmt.Sprintf("%v: %v", restErrorMessage, message)
	}
	if code, ok := resJSON["code"]; ok {
		restErrorMessage = fmt.Sprintf("%v (code: %v)", restErrorMessage, code)
	}

	if len(restErrorMessage) > 0 {
		return restErrorMessage, true
	}

	return "", false
}

func (nsp *Provider) doAuthRequest(method, path string, data map[string]interface{}) (
	map[string]interface{},
	error,
) {
	isUnautorizedUser := func(statusCode int, code interface{}) bool {
		return statusCode == 401 && code == "EAUTH"
	}

	statusCode, resJSON, err := nsp.restClient.Send(method, path, data)
	if err != nil {
		return resJSON, err
	}

	if isUnautorizedUser(statusCode, resJSON["code"]) {
		// do login call if used is unauthorized in api
		nsp.log.Infof("Log in as '%v'...", nsp.username)
		loginErr := nsp.LogIn()
		if loginErr != nil {
			return nil, loginErr
		}

		statusCode, resJSON, err = nsp.restClient.Send(method, path, data)
		if err != nil {
			return resJSON, err
		}
	}

	// check if user is still unathorised and show create a new error
	if isUnautorizedUser(statusCode, resJSON["code"]) {
		err = fmt.Errorf(
			"Login to NexentaStor %v failed (username: '%v'), "+
				"please make sure to use correct address and password",
			nsp.address,
			nsp.username)
	} else if statusCode != 200 {
		if restErrorMessage, ok := nsp.formatRestErrorMessage(resJSON); ok {
			err = fmt.Errorf("request error: %v", restErrorMessage)
		} else {
			err = fmt.Errorf(
				"request returned %v code, but response body doesn't contain explanation: %v",
				statusCode,
				resJSON)
		}
	}

	return resJSON, err
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
