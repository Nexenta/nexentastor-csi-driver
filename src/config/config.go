package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"gopkg.in/yaml.v2"

	"github.com/Nexenta/nexentastor-csi-driver/src/arrays"
)

const addressRegExp = "^https?://[^:]+:[0-9]{1,5}$"

// supported mount filesystem types
const (
	// FsTypeNFS - to mount NS filesystem over NFS
	FsTypeNFS string = "nfs"

	// FsTypeCIFS - to mount NS filesystem over SMB
	FsTypeCIFS string = "cifs"
)

// SuppertedFsTypeList - list of supported filesystem types to mount
var SuppertedFsTypeList = []string{FsTypeNFS, FsTypeCIFS}

// Config - driver config from file
type Config struct {
	Address             string `yaml:"restIp"`
	Username            string `yaml:"username"`
	Password            string `yaml:"password"`
	DefaultDataset      string `yaml:"defaultDataset,omitempty"`
	DefaultDataIP       string `yaml:"defaultDataIp,omitempty"`
	DefaultMountFsType  string `yaml:"defaultMountFsType,omitempty"`
	DefaultMountOptions string `yaml:"defaultMountOptions,omitempty"`
	Debug               bool   `yaml:"debug,omitempty"`

	filePath    string
	lastMobTime time.Time
}

// GetFilePath - get filepath of found config file
func (c *Config) GetFilePath() string {
	return c.filePath
}

// Refresh - read and validate config, return `true` if config has been changed
func (c *Config) Refresh() (changed bool, err error) {
	if c.filePath == "" {
		return false, fmt.Errorf("Cannot read config file, filePath not specified")
	}

	fileInfo, err := os.Stat(c.filePath)
	if err != nil {
		return false, fmt.Errorf("Cannot get stats for '%s' config file: %s", c.filePath, err)
	}

	changed = c.lastMobTime != fileInfo.ModTime()

	if changed {
		c.lastMobTime = fileInfo.ModTime()

		content, err := ioutil.ReadFile(c.filePath)
		if err != nil {
			return changed, fmt.Errorf("Cannot read '%s' config file: %s", c.filePath, err)
		}

		if err := yaml.Unmarshal(content, c); err != nil {
			return changed, fmt.Errorf("Cannot parse yaml in '%s' config file: %s", c.filePath, err)
		}

		if err := c.Validate(); err != nil {
			return changed, err
		}
	}

	return changed, nil
}

// Validate - validate current config
func (c *Config) Validate() error {
	var errors []string

	if c.Address == "" {
		errors = append(errors, fmt.Sprintf("parameter 'restIp' is missed"))
	} else {
		addresses := strings.Split(c.Address, ",")
		for _, address := range addresses {
			if !regexp.MustCompile(addressRegExp).MatchString(address) {
				errors = append(
					errors,
					fmt.Sprintf(
						"parameter 'restIp' has invalid address: '%s', should be 'schema://host:port'",
						address,
					),
				)
			}
		}
	}
	if c.Username == "" {
		errors = append(errors, fmt.Sprintf("parameter 'username' is missed"))
	}
	if c.Password == "" {
		errors = append(errors, fmt.Sprintf("parameter 'password' is missed"))
	}
	if c.DefaultMountFsType != "" && !arrays.ContainsString(SuppertedFsTypeList, c.DefaultMountFsType) {
		errors = append(
			errors,
			fmt.Sprintf("parameter 'defaultMountFsType' must be omitted or one of: [%s, %s]", FsTypeNFS, FsTypeCIFS),
		)
	}

	if len(errors) != 0 {
		return fmt.Errorf("Bad format, fix following issues: %s", strings.Join(errors, "; "))
	}

	return nil
}

// findConfigFile - look up for config file in a directory
func findConfigFile(lookUpDir string) (configFilePath string, err error) {
	err = filepath.Walk(lookUpDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" {
			configFilePath = path
			return filepath.SkipDir
		}
		return nil
	})
	return configFilePath, err
}

// New - find config file and create config instance
func New(lookUpDir string) (*Config, error) {
	configFilePath, err := findConfigFile(lookUpDir)
	if err != nil {
		return nil, fmt.Errorf("Cannot read config directory '%s': %s", lookUpDir, err)
	} else if configFilePath == "" {
		return nil, fmt.Errorf("Cannot find .yaml config file in '%s' directory", lookUpDir)
	}

	// read config file
	config := &Config{filePath: configFilePath}
	if _, err := config.Refresh(); err != nil {
		return nil, fmt.Errorf("Cannot refresh config from file '%s': %s", configFilePath, err)
	}

	return config, nil
}
