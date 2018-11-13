package driver

import (
	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/context"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
)

// IdentityServer - k8s csi driver identity server
type IdentityServer struct {
	*csiCommon.DefaultIdentityServer

	config *config.Config
	log    *logrus.Entry
}

// Probe - return driver status (ready or not)
func (ids *IdentityServer) Probe(ctx context.Context, req *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	ids.log.WithField("func", "Probe()").Infof("request: %+v", req)

	// read and validate config (do we need it here?)
	_, err := ids.config.Refresh()
	if err != nil {
		return nil, status.Errorf(codes.FailedPrecondition, "Cannot use config file: %v", err)
	}

	return &csi.ProbeResponse{}, nil
}

// NewIdentityServer - create an instance of identity server
func NewIdentityServer(driver *Driver) *IdentityServer {
	l := driver.log.WithField("cmp", "IdentityServer")
	l.Info("create new IdentityServer...")

	return &IdentityServer{
		DefaultIdentityServer: csiCommon.NewDefaultIdentityServer(driver.csiDriver),
		config:                driver.config,
		log:                   l,
	}
}
