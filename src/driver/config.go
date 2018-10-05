package driver

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"strings"
)

const defaultConfigFile = "/config/nexentastor-csi-driver-config.yaml"

// Config - driver config from file
type Config struct {
	Address        string `yaml:"restIp"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	DefaultDataset string `yaml:"defaultDataset,omitempty"`
	DefaultDataIP  string `yaml:"defaultDataIp,omitempty"`
}

func (c *Config) validate() error {
	var errors []string

	//TODO validate address schema too
	if c.Address == "" {
		errors = append(errors, fmt.Sprintf("parameter 'restIp' is missed"))
	}
	if c.Username == "" {
		errors = append(errors, fmt.Sprintf("parameter 'username' is missed"))
	}
	if c.Password == "" {
		errors = append(errors, fmt.Sprintf("parameter 'password' is missed"))
	}

	if len(errors) != 0 {
		return fmt.Errorf("Bad format, fix following issues: %v", strings.Join(errors, ", "))
	}

	return nil
}

// Read - read config from default config file
func Read() (*Config, error) {
	return ReadFromFile(defaultConfigFile)
}

// ReadFromFile - read specific config file
func ReadFromFile(path string) (*Config, error) {
	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("Cannot read '%v' config file: %v", path, err)
	}

	var config Config

	if err := yaml.Unmarshal(content, &config); err != nil {
		return nil, fmt.Errorf("Cannot parse yaml in '%v' config file: %v", path, err)
	}

	if err := config.validate(); err != nil {
		return nil, err
	}

	return &config, nil
}