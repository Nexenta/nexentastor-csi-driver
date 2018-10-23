package config

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v2"
)

// Config - driver config from file
type Config struct {
	filePath       string
	Address        string `yaml:"restIp"`
	Username       string `yaml:"username"`
	Password       string `yaml:"password"`
	DefaultDataset string `yaml:"defaultDataset,omitempty"`
	DefaultDataIP  string `yaml:"defaultDataIp,omitempty"`
	Debug          bool   `yaml:"debug,omitempty"`
}

// GetFilePath - get filepath of found config file
func (c *Config) GetFilePath() string {
	return c.filePath
}

// Refresh - read and validate config
func (c *Config) Refresh() error {
	if c.filePath == "" {
		return fmt.Errorf("Cannot read config file, filePath not specified")
	}
	content, err := ioutil.ReadFile(c.filePath)
	if err != nil {
		return fmt.Errorf("Cannot read '%v' config file: %v", c.filePath, err)
	} else if err := yaml.Unmarshal(content, c); err != nil {
		return fmt.Errorf("Cannot parse yaml in '%v' config file: %v", c.filePath, err)
	} else if err := c.Validate(); err != nil {
		return err
	}

	return nil
}

// Validate - validate current config
func (c *Config) Validate() error {
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

// New - find config file and create config instance
func New(lookUpDir string) (*Config, error) {
	// look up for config file
	configFilePath := ""
	fileList := []string{}
	err := filepath.Walk(lookUpDir, func(path string, info os.FileInfo, err error) error {
		if info.IsDir() {
			return nil
		}
		fileList = append(fileList, path)
		ext := filepath.Ext(path)
		if ext == ".yaml" || ext == ".yml" {
			configFilePath = path
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("Cannot read config directory '%v'", lookUpDir)
	} else if configFilePath == "" {
		return nil, fmt.Errorf("Cannot find .yaml config file in '%v' directory, found: %v", lookUpDir, fileList)
	}

	// read config file
	config := &Config{filePath: configFilePath}
	if err := config.Refresh(); err != nil {
		return nil, fmt.Errorf("Cannot refresh config from file '%v': %v", configFilePath, err)
	}

	return config, nil
}
