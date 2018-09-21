package driver

var (
	version string
	commit  string
)

// To set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/driver/driver.version=0.0.1"
func GetVersion() string {
	if version == "" {
		return "-"
	}
	return version
}

// To set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/driver/driver.commit=asdf"
func GetCommit() string {
	if commit == "" {
		return "-"
	}
	return commit
}
