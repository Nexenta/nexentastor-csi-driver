package nexentastor

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

// Provider NexentaStore provider connecter for API
type Provider struct {
	endpoint         string
	username         string
	password         string
	currentAuthToken string
}

// ProviderInterface NexentaStor providev interface
type ProviderInterface interface {
	GetPools() ([]string, error)
	LogIn() error
}

// LogIn log in to NexentaStor API and get auth token
func (nsp *Provider) LogIn() error {
	data := make(map[string]interface{})
	data["username"] = nsp.username
	data["password"] = nsp.password

	_, resJSON, err := nsp.doRequest("POST", "auth/login", data)
	if err != nil {
		return err
	}

	if token, ok := resJSON["token"]; ok {
		nsp.currentAuthToken = fmt.Sprint(token)
		log.Info("NexentaStor REST API login token is updated")
		return nil
	}

	return fmt.Errorf("NexentaStor REST API: Login request: No token found in response: %v", resJSON)
}

// GetPools get NexentaStor pools
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
	}

	return pools, nil
}

func (nsp *Provider) formatRestErrorMsg(resJSON map[string]interface{}) string {
	var restErrMsg string
	if name, ok := resJSON["name"]; ok {
		restErrMsg = fmt.Sprint(name)
	}
	if message, ok := resJSON["message"]; ok {
		restErrMsg = fmt.Sprintf("%v: %v", restErrMsg, message)
	}
	if code, ok := resJSON["code"]; ok {
		restErrMsg = fmt.Sprintf("%v (code: %v)", restErrMsg, code)
	}
	return restErrMsg
}

func (nsp *Provider) doAuthRequest(method, path string, data map[string]interface{}) (
	map[string]interface{},
	error,
) {
	isUnautorizedUser := func(statusCode int, code interface{}) bool {
		return statusCode == 401 && code == "EAUTH"
	}

	statusCode, resJSON, err := nsp.doRequest(method, path, data)
	if err != nil {
		return resJSON, err
	}

	if isUnautorizedUser(statusCode, resJSON["code"]) {
		// do login call if used is unauthorized in api
		log.Infof("Log in to NexentaStor API as '%v'...", nsp.username)
		loginErr := nsp.LogIn()
		if loginErr != nil {
			return nil, loginErr
		}

		statusCode, resJSON, err = nsp.doRequest(method, path, data)
		if err != nil {
			return resJSON, err
		}
	}

	// check if user is still unathorised and show create a new error
	if isUnautorizedUser(statusCode, resJSON["code"]) {
		err = fmt.Errorf(
			"Login failed to NexentaStor %v with user '%v', "+
				"please make sure to use correct address and password",
			nsp.endpoint,
			nsp.username)
	} else if statusCode != 200 {
		restErrMsg := nsp.formatRestErrorMsg(resJSON)
		if len(restErrMsg) != 0 {
			err = fmt.Errorf("NexentaStor REST API request error: %v", restErrMsg)
		} else {
			err = fmt.Errorf(
				"NexentaStor REST API request returned %v code, "+
					"but response body doesn't contain explanation: %v",
				statusCode,
				resJSON)
		}
	}

	return resJSON, err
}

func (nsp *Provider) doRequest(method, path string, data map[string]interface{}) (
	int,
	map[string]interface{},
	error,
) {
	log.Infof("Request NexentaStor API: %v %v/%v", method, nsp.endpoint, path)

	tr := &http.Transport{
		IdleConnTimeout: 60 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // don't check certivicate, fix this!
		},
	}

	client := &http.Client{
		Transport: tr,
	}

	// send request data as json
	var jsonDataReader io.Reader
	if len(data) == 0 {
		jsonDataReader = nil
	} else {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return 0, nil, err
		}
		jsonDataReader = strings.NewReader(string(jsonData))
		log.Infof("  post data: %v", data) //TODO use debug
	}

	req, _ := http.NewRequest(method, fmt.Sprintf("%v/%v", nsp.endpoint, path), jsonDataReader)

	req.Header.Set("Content-Type", "application/json")
	if len(nsp.currentAuthToken) != 0 {
		log.Infof("set token '%v'", nsp.currentAuthToken)
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", nsp.currentAuthToken))
	}

	res, err := client.Do(req)
	if err != nil {
		log.Errorf("HTTP request error: %v", err)
		return 0, nil, err
	}

	defer res.Body.Close()

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Errorf("HTTP request error: Cannot read body: %v", err)
		return res.StatusCode, nil, err
	}

	jsonRes := make(map[string]interface{})
	jsonErr := json.Unmarshal(bodyBytes, &jsonRes)
	if jsonErr != nil {
		jsonRes["error"] = fmt.Sprintf("Cannot parse json from body: '%v'", string(bodyBytes))
		return res.StatusCode, jsonRes, err
	}

	log.Infof("%v", jsonRes)

	return res.StatusCode, jsonRes, nil
}

// NewProvider create NexentaStor provider instance
func NewProvider(endpoint, username, password string) (nsp ProviderInterface, err error) {
	log.Infof("Create new NexentaStorProvider for %v", endpoint)

	nsp = &Provider{
		endpoint:         endpoint,
		username:         username,
		password:         password,
		currentAuthToken: "",
	}

	return nsp, nil
}
