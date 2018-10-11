package ns

import (
	"fmt"
	"net/url"
	"strings"
)

// Filesystem - NexentaStor filesystem
type Filesystem struct {
	Path          string
	MountPoint    string
	SharedOverNfs bool
	QuotaSize     int64
}

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
		return nil, fmt.Errorf("/storage/pools response doesn't contain 'data' property: %v", resJSON)
	}

	return pools, nil
}

// GetFilesystem - get NexentaStor filesystem by its path
func (nsp *Provider) GetFilesystem(path string) (*Filesystem, error) {
	fields := []string{"path", "quotaSize", "mountPoint", "sharedOverNfs"}
	uri := nsp.RestClient.BuildURI("/storage/filesystems", map[string]string{
		"path":   path,
		"fields": strings.Join(fields, ","),
	})

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	if err = mapHasProps(resJSON, []string{"data"}); err != nil {
		return nil, fmt.Errorf("/storage/filesystems response: %+v", err)
	}

	if dataArray, ok := resJSON["data"].([]interface{}); ok && len(dataArray) != 0 {
		filesystem := dataArray[0].(map[string]interface{})
		if err := mapHasProps(filesystem, fields); err != nil {
			return nil, fmt.Errorf("/storage/filesystems response: %+v", err)
		}
		return &Filesystem{
			Path:          filesystem["path"].(string),
			MountPoint:    filesystem["mountPoint"].(string),
			SharedOverNfs: filesystem["sharedOverNfs"].(bool),
			QuotaSize:     int64(filesystem["quotaSize"].(float64)),
		}, nil
	}

	return nil, nil
}

// GetFilesystems - get all NexentaStor filesystems by parent filesystem
func (nsp *Provider) GetFilesystems(parent string) ([]*Filesystem, error) {
	fields := []string{"path", "quotaSize", "mountPoint", "sharedOverNfs"}
	uri := nsp.RestClient.BuildURI("/storage/filesystems", map[string]string{
		"parent": parent,
		"fields": strings.Join(fields, ","),
	})

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return nil, err
	}

	filesystems := []*Filesystem{}

	if err = mapHasProps(resJSON, []string{"data"}); err != nil {
		return nil, fmt.Errorf("/storage/filesystems response: %+v", err)
	}

	for _, val := range resJSON["data"].([]interface{}) {
		filesystem := val.(map[string]interface{})
		if err := mapHasProps(filesystem, fields); err != nil {
			return nil, fmt.Errorf("/storage/filesystems response: %+v", err)
		}
		filesystemPath := filesystem["path"].(string)
		if filesystemPath != parent {
			filesystems = append(filesystems, &Filesystem{
				Path:          filesystemPath,
				MountPoint:    filesystem["mountPoint"].(string),
				SharedOverNfs: filesystem["sharedOverNfs"].(bool),
				QuotaSize:     int64(filesystem["quotaSize"].(float64)),
			})
		}
	}

	return filesystems, nil
}

// CreateFilesystem - create filesystem by path
func (nsp *Provider) CreateFilesystem(path string, params map[string]interface{}) error {
	data := make(map[string]interface{})
	data["path"] = path

	for key, val := range params {
		data[key] = val
	}

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
	data["anon"] = "root" //TODO this should be configurable

	//TODO add host access options

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
	restError := nsp.parseNefError(resJSON, "Job was finished with error")
	if restError != nil {
		err = restError
	} else {
		err = fmt.Errorf(
			"Job request returned %v code, but response body doesn't contain explanation: %v",
			statusCode,
			resJSON)
	}
	return false, err
}

func mapHasProps(m map[string]interface{}, props []string) error {
	var missedProps []string
	for _, prop := range props {
		if _, ok := m[prop]; !ok {
			missedProps = append(missedProps, prop)
		}
	}
	if len(missedProps) != 0 {
		return fmt.Errorf("Properties missed: %v", missedProps)
	}
	return nil
}
