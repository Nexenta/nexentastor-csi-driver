package config_test

import (
	"testing"

	"github.com/Nexenta/nexentastor-csi-driver/src/driver"
)

var testConfigParams = map[string]string{
	"Address":        "https://10.1.1.1:8443,https://10.1.1.2:8443",
	"Username":       "usr",
	"Password":       "pwd",
	"DefaultDataset": "pl/dtst",
	"DefaultDataIp":  "20.1.1.1",
}

func testParam(t *testing.T, name, expected, given string) {
	if expected != given {
		t.Errorf("Param '%v' expected to be '%v', but got '%v' instead", name, expected, given)
	}
}

func TestDriver_Config_Full(t *testing.T) {
	path := "./_fixtures/test-config-full.yaml"

	c, err := driver.ReadFromFile(path)
	if err != nil {
		t.Errorf("cannot read config file '%v': %v", path, err)
		return
	}

	testParam(t, "Address", testConfigParams["Address"], c.Address)
	testParam(t, "Username", testConfigParams["Username"], c.Username)
	testParam(t, "Password", testConfigParams["Password"], c.Password)
	testParam(t, "DefaultDataset", testConfigParams["DefaultDataset"], c.DefaultDataset)
	testParam(t, "DefaultDataIp", testConfigParams["DefaultDataIp"], c.DefaultDataIP)
}

func TestDriver_Config_Short(t *testing.T) {
	path := "./_fixtures/test-config-short.yaml"

	c, err := driver.ReadFromFile(path)
	if err != nil {
		t.Errorf("cannot read config file '%v': %v", path, err)
		return
	}

	testParam(t, "Address", testConfigParams["Address"], c.Address)
	testParam(t, "Username", testConfigParams["Username"], c.Username)
	testParam(t, "Password", testConfigParams["Password"], c.Password)
	testParam(t, "DefaultDataset", "", c.DefaultDataset)
	testParam(t, "DefaultDataIp", "", c.DefaultDataIP)
}

func TestDriver_Config_Not_Valid(t *testing.T) {

	t.Run("should return an error if config file if not valid", func(t *testing.T) {
		path := "./_fixtures/test-config-not-valid.yaml"
		c, err := driver.ReadFromFile(path)
		if err == nil {
			t.Errorf("not valid '%v' config file should return an error, but got this: %v", path, c)
			return
		}
	})

	t.Run("should return nan error if file not exists", func(t *testing.T) {
		path := "./_fixtures/not-existing-test-config.yaml"
		c, err := driver.ReadFromFile(path)
		if err == nil {
			t.Errorf("not existing config file '%v' returns config: %v", path, c)
			return
		}
	})
}
