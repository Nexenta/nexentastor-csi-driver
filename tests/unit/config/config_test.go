package config_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
)

var testConfigParams = map[string]string{
	"Address":             "https://10.1.1.1:8443,https://10.1.1.2:8443",
	"Username":            "usr",
	"Password":            "pwd",
	"DefaultDataset":      "poolA/datasetA",
	"DefaultDataIp":       "20.1.1.1",
	"DefaultMountFsType":  "nfs",
	"DefaultMountOptions": "noatime",
}

func testParam(t *testing.T, name, expected, given string) {
	if expected != given {
		t.Errorf("Param '%v' expected to be '%v', but got '%v' instead", name, expected, given)
	}
}

func TestConfig_Full(t *testing.T) {
	path := "./_fixtures/test-config-full"

	c, err := config.New(path)
	if err != nil {
		t.Fatalf("cannot read config file '%v': %v", path, err)
	}

	testParam(t, "Address", testConfigParams["Address"], c.Address)
	testParam(t, "Username", testConfigParams["Username"], c.Username)
	testParam(t, "Password", testConfigParams["Password"], c.Password)
	testParam(t, "DefaultDataset", testConfigParams["DefaultDataset"], c.DefaultDataset)
	testParam(t, "DefaultDataIp", testConfigParams["DefaultDataIp"], c.DefaultDataIP)
	testParam(t, "DefaultMountFsType", testConfigParams["DefaultMountFsType"], c.DefaultMountFsType)
	testParam(t, "DefaultMountOptions", testConfigParams["DefaultMountOptions"], c.DefaultMountOptions)
}

func TestConfig_Short(t *testing.T) {
	path := "./_fixtures/test-config-short"

	c, err := config.New(path)
	if err != nil {
		t.Fatalf("cannot read config file '%v': %v", path, err)
	}

	testParam(t, "Address", testConfigParams["Address"], c.Address)
	testParam(t, "Username", testConfigParams["Username"], c.Username)
	testParam(t, "Password", testConfigParams["Password"], c.Password)
	testParam(t, "DefaultDataset", "", c.DefaultDataset)
	testParam(t, "DefaultDataIp", "", c.DefaultDataIP)
	testParam(t, "DefaultMountFsType", "", c.DefaultMountFsType)
	testParam(t, "DefaultMountOptions", "", c.DefaultMountOptions)
}

func TestConfig_Not_Valid(t *testing.T) {

	t.Run("should return an error if config file if not valid", func(t *testing.T) {
		path := "./_fixtures/test-config-not-valid"
		c, err := config.New(path)
		if err == nil {
			t.Fatalf("not valid '%v' config file should return an error, but got this: %v", path, c)
		}
	})

	t.Run("should return supported filesystem list if 'defaultMountFsType' value is unsupported", func(t *testing.T) {
		path := "./_fixtures/test-config-not-supported-fs"
		c, err := config.New(path)
		if err == nil {
			t.Fatalf(
				"not valid 'defaultMountFsType' (file: %v) value should return an error, but got this: %v",
				path,
				c,
			)
		} else if !strings.Contains(err.Error(), "defaultMountFsType") {
			t.Fatalf(
				"not valid 'defaultMountFsType' (file: %v) error text should contain property name, but got this: %v",
				path,
				err,
			)
		}
	})

	t.Run("should return an error if file not exists", func(t *testing.T) {
		path := "./_fixtures/dir-without-config"
		c, err := config.New(path)
		if err == nil {
			t.Fatalf("not existing config file '%v' returns config: %v", path, c)
		}
	})

	t.Run("should return an error if one of the addresses is invalid", func(t *testing.T) {
		path := "./_fixtures/test-config-not-valid-address"
		c, err := config.New(path)
		if err == nil {
			t.Fatalf("should return an error for file '%v' but returns config: %v", path, c)
		} else if !strings.Contains(err.Error(), "BAD_PORT") {
			t.Fatalf("should return an error with 'BAD_PORT' text for file '%v' but returns this: %v", path, err)
		}
	})
}

func TestConfig_Refresh(t *testing.T) {
	path := "./_fixtures/test-config-short"

	c, err := config.New(path)
	if err != nil {
		t.Fatalf("cannot read config file '%s': %s", path, err)
	}

	t.Run("should return changed:false if config file was not changed", func(t *testing.T) {
		changed, err := c.Refresh()
		if err != nil {
			t.Fatalf("cannot refresh config file '%s': %s", path, err)
		} else if changed == true {
			t.Fatalf("Config.Refresh() indicates that config was changed, but it's not, file '%s'", path)
		}
	})

	t.Run("should return changed:true after config update", func(t *testing.T) {
		err := os.Chtimes(c.GetFilePath(), time.Now(), time.Now())
		if err != nil {
			t.Fatalf("Cannot change atime/mtime for '%s' config file: %s", c.GetFilePath(), err)
		}

		changed, err := c.Refresh()
		if err != nil {
			t.Fatalf("cannot refresh config file '%s': %s", path, err)
		} else if changed == false {
			t.Fatalf("Config.Refresh() does not indicate that config was changed, file '%s'", path)
		}
	})
}
