package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Nexenta/nexentastor-csi-driver/driver"
)

const (
	ENDPOINT = "unix:///var/lib/kubelet/plugins/com.nexenta.nexentastor-csi-plugin/csi.sock"
)

func main() {
	var (
		nodeid   = flag.String("nodeid", "", "Kubernetes node ID")
		endpoint = flag.String("endpoint", ENDPOINT, "CSI endpoint")
		version  = flag.Bool("version", false, "Print driver version")
	)

	flag.Parse()

	if *version {
		fmt.Printf("Version: %s, commit: %s\n", driver.GetVersion(), driver.GetCommit())
		os.Exit(0)
	}

	fmt.Printf("Start driver with endpoint: '%v' and nodeid: '%v'\n", *endpoint, *nodeid)
}
