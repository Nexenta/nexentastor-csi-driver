package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nexenta/nexentastor-csi-driver/pkg/config"
)

// IdentityServer - k8s csi driver identity server
type IdentityServer struct {
	config *config.Config
	log    *logrus.Entry
}

// GetPluginInfo - return plugin info
func (ids *IdentityServer) GetPluginInfo(ctx context.Context, req *csi.GetPluginInfoRequest) (
	*csi.GetPluginInfoResponse,
	error,
) {
	ids.log.WithField("func", "GetPluginInfo()").Infof("request: '%+v'", req)

	return &csi.GetPluginInfoResponse{
		Name:          Name,
		VendorVersion: Version,
	}, nil
}

// Probe - return driver status (ready or not)
func (ids *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	ids.log.WithField("func", "Probe()").Infof("request: '%+v'", req)

	// read and validate config (do we need it here?)
	_, err := ids.config.Refresh()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %s", err)
	}

	return &csi.ProbeResponse{}, nil
}

// GetPluginCapabilities - get plugin capabilities
func (ids *IdentityServer) GetPluginCapabilities(ctx context.Context, req *csi.GetPluginCapabilitiesRequest) (
	*csi.GetPluginCapabilitiesResponse,
	error,
) {
	ids.log.WithField("func", "GetPluginCapabilities()").Infof("request: '%+v'", req)

	return &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
		},
	}, nil
}

// NewIdentityServer - create an instance of identity server
func NewIdentityServer(driver *Driver) *IdentityServer {
	l := driver.log.WithField("cmp", "IdentityServer")
	l.Info("create new IdentityServer...")

	return &IdentityServer{
		config: driver.config,
		log:    l,
	}
}
