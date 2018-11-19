package ns

import (
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strings"
)

//TODO create *Request/*Respose data types for all methods

// License - NexentaStor license
type License struct {
	Valid   bool
	Expires string
}

// Filesystem - NexentaStor filesystem
type Filesystem struct {
	Path          string
	MountPoint    string
	SharedOverNfs bool
	SharedOverSmb bool
	QuotaSize     int64
}

// GetDefaultSmbShareName - get default SMB share name (all slashes get replaced by underscore)
// Converts '/pool/dataset/fs' to 'pool_dataset_fs'
func (fs *Filesystem) GetDefaultSmbShareName() string {
	return strings.Replace(strings.TrimPrefix(fs.Path, "/"), "/", "_", -1)
}

// LogIn - log in to NexentaStor API and get auth token
func (nsp *Provider) LogIn() error {
	l := nsp.Log.WithField("func", "LogIn()")

	data := make(map[string]interface{})
	data["username"] = nsp.Username
	data["password"] = nsp.Password

	_, resJSON, err := nsp.RestClient.Send("POST", "auth/login", data)
	if err != nil {
		return err
	}

	if token, ok := resJSON["token"]; ok {
		nsp.RestClient.SetAuthToken(fmt.Sprint(token))
		l.Debugf("login token has been updated")
		return nil
	}

	// try to parse error from rest response
	restError := nsp.parseNefError(resJSON, "Login request")
	if restError != nil {
		code := restError.(*NefError).Code
		if code == "EAUTH" {
			l.Errorf(
				"login to NexentaStor %v failed (username: '%v'), "+
					"please make sure to use correct address and password",
				nsp.Address,
				nsp.Username)
		}
		return restError
	}

	return fmt.Errorf("Login request: No token found in response: %v", resJSON)
}

// GetLicense - return NexentaStor license
func (nsp *Provider) GetLicense() (*License, error) {
	resJSON, err := nsp.doAuthRequest("GET", "/settings/license", nil)
	if err != nil {
		return nil, err
	}

	if err := mapHasProps(resJSON, []string{"expires", "valid"}); err != nil {
		return nil, err
	}

	return &License{
		Valid:   resJSON["valid"].(bool),
		Expires: resJSON["expires"].(string),
	}, nil
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

// GetFilesystemAvailableCapacity - get NexentaStor filesystem available size by its path
func (nsp *Provider) GetFilesystemAvailableCapacity(path string) (int64, error) {
	fields := []string{"quotaSize", "bytesAvailable"}
	uri := nsp.RestClient.BuildURI("/storage/filesystems", map[string]string{
		"path":   path,
		"fields": strings.Join(fields, ","),
	})

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return 0, err
	}

	if err = mapHasProps(resJSON, []string{"data"}); err != nil {
		return 0, fmt.Errorf("/storage/filesystems response: %+v", err)
	}

	//TODO what is the right limit for fs?

	var quotaSize, availableSize float64
	if dataArray, ok := resJSON["data"].([]interface{}); ok && len(dataArray) != 0 {
		filesystem := dataArray[0].(map[string]interface{})
		if err := mapHasProps(filesystem, fields); err != nil {
			return 0, fmt.Errorf("/storage/filesystems response: %+v", err)
		}

		quotaSize = filesystem["quotaSize"].(float64)
		availableSize = filesystem["bytesAvailable"].(float64)
	}

	return int64(math.Min(availableSize, quotaSize)), nil
}

// GetFilesystem - get NexentaStor filesystem by its path
func (nsp *Provider) GetFilesystem(path string) (*Filesystem, error) {
	if len(path) == 0 {
		return nil, fmt.Errorf("Filesystem path is empty")
	}

	fields := []string{"path", "quotaSize", "mountPoint", "sharedOverNfs", "sharedOverSmb"}
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
			SharedOverSmb: filesystem["sharedOverSmb"].(bool),
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

// CreateFilesystemParams - params to create filesystem
type CreateFilesystemParams struct {
	Path      string `json:"path,omitempty"`
	QuotaSize int64  `json:"quotaSize,omitempty"` //TODO use "referencedQuotaSize" instead
}

// CreateFilesystem - create filesystem by path
func (nsp *Provider) CreateFilesystem(params CreateFilesystemParams) error {
	if params.Path == "" {
		return fmt.Errorf("Parameter 'CreateFilesystemParams.Path' is required")
	}

	_, err := nsp.doAuthRequest("POST", "/storage/filesystems", params)

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

	type ParamsSecurityContext struct {
		SecurityModes []string `json:"securityModes"`
	}

	type Params struct {
		Filesystem       string                   `json:"filesystem"`
		Anon             string                   `json:"anon"`
		SecurityContexts []*ParamsSecurityContext `json:"securityContexts"`
	}

	data := &Params{
		Filesystem: path,
		Anon:       "root",
		SecurityContexts: []*ParamsSecurityContext{
			&ParamsSecurityContext{
				SecurityModes: []string{"sys"},
			},
		},
	}

	_, err := nsp.doAuthRequest("POST", "nas/nfs", data)

	return err
}

// DeleteNfsShare - destroy NFS chare by filesystem path
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

// CreateSmbShare - create SMB share (cifs) on specified filesystem
// Leave shareName empty to generate default value
// CLI test:
// 	 mkdir -p /mnt/test && sudo mount -v -t cifs -o username=admin,password=Nexenta@1 //HOST//pool_fs /mnt/test
// 	 findmnt /mnt/test
func (nsp *Provider) CreateSmbShare(path, shareName string) error {
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	type Params struct {
		Filesystem string `json:"filesystem"`
		ShareName  string `json:"shareName,omitempty"`
	}

	data := &Params{
		Filesystem: path,
		ShareName:  shareName,
	}

	_, err := nsp.doAuthRequest("POST", "nas/smb", data)

	return err
}

// GetSmbShareName - get share name for filesystem that shared over SMB
func (nsp *Provider) GetSmbShareName(path string) (string, error) {
	if len(path) == 0 {
		return "", fmt.Errorf("Filesystem path is empty")
	}

	fields := []string{"shareName", "shareState"} //TODO check shareState value?
	uri := nsp.RestClient.BuildURI(
		fmt.Sprintf("/nas/smb/%v", url.PathEscape(path)),
		map[string]string{
			"fields": strings.Join(fields, ","),
		},
	)

	resJSON, err := nsp.doAuthRequest("GET", uri, nil)
	if err != nil {
		return "", err
	}

	if err = mapHasProps(resJSON, fields); err != nil {
		return "", fmt.Errorf("/nas/smb response: %+v", err)
	}

	shareName := resJSON["shareName"].(string)

	return shareName, nil
}

// DeleteSmbShare - destroy SMB share by filesystem path
func (nsp *Provider) DeleteSmbShare(path string) error {
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	data := make(map[string]interface{})
	data["path"] = path

	uri := fmt.Sprintf("/nas/smb/%v", url.PathEscape(path))

	_, err := nsp.doAuthRequest("DELETE", uri, nil)

	return err
}

// ACLRuleSet - filesystem ACL rule set
type ACLRuleSet int64

const (
	// ACLReadOnly - apply read only set of rules to filesystem
	ACLReadOnly ACLRuleSet = iota

	// ACLReadWrite - apply full access set of rules to filesystem
	ACLReadWrite
)

// SetFilesystemACL - set filesystem ACL, so NFS share can allow user to write w/o checking UNIX user uid
func (nsp *Provider) SetFilesystemACL(path string, aclRuleSet ACLRuleSet) error {
	if len(path) == 0 {
		return fmt.Errorf("Filesystem path is empty")
	}

	permissions := []string{}
	if aclRuleSet == ACLReadOnly {
		permissions = append(permissions, "read_set")
	} else {
		permissions = append(permissions, "full_set")
	}

	type Params struct {
		Type        string   `json:"type"`
		Principal   string   `json:"principal"`
		Flags       []string `json:"flags"`
		Permissions []string `json:"permissions"`
	}

	data := &Params{
		Type:      "allow",
		Principal: "everyone@",
		Flags: []string{
			"file_inherit",
			"dir_inherit",
		},
		Permissions: permissions,
	}

	uri := fmt.Sprintf("/storage/filesystems/%v/acl", url.PathEscape(path))
	_, err := nsp.doAuthRequest("POST", uri, data)

	return err
}

// IsJobDone - check if job is done by jobId
func (nsp *Provider) IsJobDone(jobID string) (bool, error) {
	uri := fmt.Sprintf("/jobStatus/%v", jobID)

	statusCode, resJSON, err := nsp.RestClient.Send("GET", uri, nil)
	if err != nil { // request failed
		return false, err
	} else if statusCode == http.StatusOK || statusCode == http.StatusCreated { // job is completed
		return true, nil
	} else if statusCode == http.StatusAccepted { // job is in progress (202)
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
