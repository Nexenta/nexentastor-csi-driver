package rest

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/sirupsen/logrus"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	requestTimeout = 30 * time.Second
)

var requestId = 0

// Client - request client for any REST API
type Client struct {
	address    string
	authToken  string
	httpClient *http.Client
	log        *logrus.Entry
}

// ClientInterface - request client interface
type ClientInterface interface {
	SetAuthToken(string)
	Send(string, string, map[string]interface{}) (int, map[string]interface{}, error)
	BuildURI(string, map[string]string) string
}

//SetAuthToken - set Bearer auth token for all requests
func (client *Client) SetAuthToken(token string) {
	client.authToken = token
}

//Send - send request to REST server
func (client *Client) Send(method, path string, data map[string]interface{}) (
	int,
	map[string]interface{},
	error,
) {
	uri := fmt.Sprintf("%v/%v", client.address, path)
	requestId++
	requestLog := client.log.WithFields(logrus.Fields{
		"req":   fmt.Sprintf("%v %v", method, path),
		"reqId": requestId,
	})
	requestLog.Info("Send request")

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
		client.log.Infof("Data: %v", data) //TODO use debug
	}

	req, _ := http.NewRequest(method, uri, jsonDataReader)

	req.Header.Set("Content-Type", "application/json")
	if len(client.authToken) != 0 {
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %v", client.authToken))
	}

	res, err := client.httpClient.Do(req)
	if err != nil {
		requestLog.Errorf("Request error: %v", err)
		return 0, nil, err
	}

	defer res.Body.Close()

	bodyBytes, err := ioutil.ReadAll(res.Body)
	if err != nil {
		requestLog.Errorf("Cannot read body: %v", err)
		return res.StatusCode, nil, err
	}

	requestLog.Infof("Response status code: %v", res.StatusCode)

	jsonRes := make(map[string]interface{})

	if len(bodyBytes) > 0 {
		jsonErr := json.Unmarshal(bodyBytes, &jsonRes)
		if jsonErr != nil {
			jsonRes["error"] = fmt.Sprintf("Cannot parse json from body: '%v'", string(bodyBytes))
			return res.StatusCode, jsonRes, err
		}
	}

	return res.StatusCode, jsonRes, nil
}

//BuildURI - build request URI using [path?params...] format
func (client *Client) BuildURI(uri string, params map[string]string) string {
	paramsStr := ""
	paramValues := url.Values{}

	for key, val := range params {
		if len(val) != 0 {
			paramValues.Set(key, val)
		}
	}

	paramsStr = paramValues.Encode()
	if len(paramsStr) != 0 {
		uri = fmt.Sprintf("%v?%v", uri, paramsStr)
	}

	return uri
}

// ClientArgs - params to create Client instance
type ClientArgs struct {
	Address string
	Log     *logrus.Entry
}

// NewClient - create new REST client
func NewClient(args ClientArgs) (client ClientInterface, err error) {
	clientLog := args.Log.WithFields(logrus.Fields{
		"cmp": "RestClient",
	})

	clientLog.Debugf("Create for %v", args.Address)

	tr := &http.Transport{
		//IdleConnTimeout: 60 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // don't check certivicate, fix this!
		},
	}

	httpClient := &http.Client{
		Transport: tr,
		Timeout:   requestTimeout,
	}

	client = &Client{
		address:    args.Address,
		authToken:  "",
		httpClient: httpClient,
		log:        clientLog,
	}

	return client, nil
}
