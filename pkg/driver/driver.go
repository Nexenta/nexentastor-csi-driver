package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/Nexenta/go-nexentastor/pkg/ns"
	"github.com/Nexenta/nexentastor-csi-driver/pkg/config"
)

// Name - driver name
var Name = "nexentastor-csi-driver.nexenta.com"

// Version - driver version, to set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Version=0.0.1"
var Version string

// Commit - driver last commit, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.Commit=..."
var Commit string

// DateTime - driver build datetime, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/pkg/driver.DateTime=..."
var DateTime string

// Driver - K8s CSI driver for NexentaStor
type Driver struct {
	role     Role
	nodeID   string
	endpoint string
	config   *config.Config
	server   *grpc.Server
	log      *logrus.Entry
}

// Run - run the driver
func (d *Driver) Run() error {
	d.log.Info("run")

	parsedURL, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint: %s", d.endpoint)
	}

	if parsedURL.Scheme != "unix" {
		return fmt.Errorf("Only unix domain sockets are supported")
	}

	socket := filepath.FromSlash(parsedURL.Path)
	if parsedURL.Host != "" {
		socket = path.Join(parsedURL.Host, socket)
	}

	d.log.Infof("parsed unix domain socket: %s", socket)

	//remove old socket file if exists
	if err := os.Remove(socket); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("Cannot remove unix domain socket: %s", socket)
	}

	listener, err := net.Listen(parsedURL.Scheme, socket)
	if err != nil {
		return fmt.Errorf("Failed to create socket listener: %s", err)
	}

	d.server = grpc.NewServer(grpc.UnaryInterceptor(d.grpcErrorHandler))

	// IdentityServer - should be running on both controller and node pods
	csi.RegisterIdentityServer(d.server, NewIdentityServer(d))

	if d.role.IsController() {
		controllerServer, err := NewControllerServer(d)
		if err != nil {
			return fmt.Errorf("Failed to create ControllerServer: %s", err)
		}
		csi.RegisterControllerServer(d.server, controllerServer)
	}

	if d.role.IsNode() {
		nodeServer, err := NewNodeServer(d)
		if err != nil {
			return fmt.Errorf("Failed to create NodeServer: %s", err)
		}
		csi.RegisterNodeServer(d.server, nodeServer)
	}

	return d.server.Serve(listener)
}

// Validate - validate driver configuration:
// - check NS connection
// - check NS license
// - in case of cluster, check if provided addresses belong to the same cluster
func (d *Driver) Validate() error {
	for _, cfg := range d.config.NsMap {
		nsResolver, err := ns.NewResolver(ns.ResolverArgs{
			Address:            cfg.Address,
			Username:           cfg.Username,
			Password:           cfg.Password,
			Log:                d.log,
			InsecureSkipVerify: true, //TODO move to config
		})
		if err != nil {
			return fmt.Errorf("Driver validation failed, cannot create NexentaStor(s) resolver: %s", err)
		}

		// check license for each NS node
		//TODO check the license only in case some driver operations are failed
		for _, nsProvider := range nsResolver.Nodes {
			license, err := nsProvider.GetLicense()
			if err != nil {
				d.log.Warnf("Driver license validation failed: %s", err)
			} else if !license.Valid {
				d.log.Warnf(
					"Driver license validation failed, NexentaStor %s has invalid license (expired: %s)",
					nsProvider,
					license.Expires,
				)
			}
		}

		// check if NS nodes belong to one cluster
		if len(nsResolver.Nodes) > 1 {
			isCluster, err := nsResolver.IsCluster()
			if err != nil {
				d.log.Warnf(
					"Provided NexentaStor addresses may not belong to the same cluster, cannot check the cluster: %s",
					err,
				)
			} else if !isCluster {
				d.log.Warn("Provided NexentaStor addresses may not belong to the same cluster")
			}
		}
	}
	return nil
}

func (d *Driver) grpcErrorHandler(
	ctx context.Context,
	req interface{},
	info *grpc.UnaryServerInfo,
	handler grpc.UnaryHandler,
) (interface{}, error) {
	resp, err := handler(ctx, req)
	if err != nil {
		d.log.WithField("func", "grpc").Error(err)
	}
	return resp, err
}

// Args - params to crete new driver
type Args struct {
	Role     Role
	NodeID   string
	Endpoint string
	Config   *config.Config
	Log      *logrus.Entry
}

// NewDriver - new driver instance
func NewDriver(args Args) (*Driver, error) {
	if args.Config == nil {
		return nil, fmt.Errorf("args.Config is required")
	} else if args.Log == nil {
		return nil, fmt.Errorf("args.Log is required")
	}

	l := args.Log.WithField("cmp", "Driver")
	l.Infof("create new driver: %s@%s-%s (%s)", Name, Version, Commit, DateTime)

	d := &Driver{
		role:     args.Role,
		nodeID:   args.NodeID,
		endpoint: args.Endpoint,
		config:   args.Config,
		log:      l,
	}

	return d, nil
}
