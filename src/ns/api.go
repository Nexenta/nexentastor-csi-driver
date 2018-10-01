package ns

import (
	"fmt"
	"net/url"
)

// LogIn - log in to NexentaStor API and get auth token
func (nsp *Provider) LogIn() error {
	data := make(map[string]interface{})
	data["username"] = nsp.Username
	data["password"] = nsp.Password

	_, resJSON, err := nsp.RestClient.Send("POST", "auth/login", data)
	if err != nil {
		return err
	}

	if token, ok := resJSON["token"]; ok {
		nsp.RestClient.SetAuthToken(fmt.Sprint(token))
		nsp.Log.Info("Login token has been updated")
		return nil
	}

	// try to parse error from rest response
	restError := nsp.parseNefError(resJSON, "Login request")
	if restError != nil {
		code := restError.(*NefError).Code
		if code == "EAUTH" {
			nsp.Log.Errorf(
				"Login to NexentaStor %v failed (username: '%v'), "+
					"please make sure to use correct address and password",
				nsp.Address,
				nsp.Username)
		}
		return restError
	}

	return fmt.Errorf("Login request: No token found in response: %v", resJSON)
}

// GetPools - get NexentaStor pools
func (nsp *Provider) GetPools() ([]string, error) {
	uri := nsp.RestClient.BuildURI("/storage/pools", map[string]string{
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
		nsp.Log.Warnf("response doesn't contain 'data' property: %v", resJSON)
	}

	return pools, nil
}

// GetFilesystems - get NexentaStor filesystems
func (nsp *Provider) GetFilesystems(pool string) ([]string, error) {
	uri := nsp.RestClient.BuildURI("/storage/filesystems", map[string]string{
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
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	data := make(map[string]interface{})
	data["path"] = path

	uri := fmt.Sprintf("/storage/filesystems/%v", url.PathEscape(path))

	_, err := nsp.doAuthRequest("DELETE", uri, nil)

	return err
}

// CreateNfsShare - create NFS share on specified filesystem
// CLI test:
//	 showmount -e HOST
// 	 mkdir -p /mnt/test && sudo mount -v -t nfs HOST:/pool/fs /mnt/test
// 	 findmnt /mnt/test
func (nsp *Provider) CreateNfsShare(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	data := make(map[string]interface{})
	data["filesystem"] = path

	_, err := nsp.doAuthRequest("POST", "nas/nfs", data)

	return err
}

// DeleteNfsShare - destroy filesystem by path
func (nsp *Provider) DeleteNfsShare(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	data := make(map[string]interface{})
	data["path"] = path

	uri := fmt.Sprintf("/nas/nfs/%v", url.PathEscape(path))

	_, err := nsp.doAuthRequest("DELETE", uri, nil)

	return err
}

// GetRsfClusters - get SRF cluster from NexentaStor
func (nsp *Provider) GetRsfClusters() (string, error) {
	return "", nil
}

// IsJobDone - check if job is done by jobId
func (nsp *Provider) IsJobDone(jobID string) (bool, error) {
	uri := fmt.Sprintf("/jobStatus/%v", jobID)

	statusCode, resJSON, err := nsp.RestClient.Send("GET", uri, nil)
	if err != nil { // request failed
		return false, err
	} else if statusCode == 200 || statusCode == 201 { // job is completed
		return true, nil
	} else if statusCode == 202 { // job is in progress
		return false, nil
	}

	// job is failed
	restError := nsp.parseNefError(resJSON, "Job request error")
	if restError != nil {
		err = restError
	} else {
		err = fmt.Errorf(
			"job request returned %v code, but response body doesn't contain explanation: %v",
			statusCode,
			resJSON)
	}
	return false, err
}
