package driver

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path"
	"path/filepath"

	csi "github.com/container-storage-interface/spec/lib/go/csi/v0"
	csiCommon "github.com/kubernetes-csi/drivers/pkg/csi-common"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc"

	"github.com/Nexenta/nexentastor-csi-driver/src/config"
	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

// Name - driver name
var Name = "com.nexenta.nexentastor-csi-driver"

// Version - driver version, to set version set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Version=0.0.1"
var Version string

// Commit - driver last commit, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.Commit=..."
var Commit string

// DateTime - driver build datetime, to set commit set flags:
// go build -ldflags "-X github.com/Nexenta/nexentastor-csi-driver/src/driver.DateTime=..."
var DateTime string

// Driver - K8s CSI driver for NexentaStor
type Driver struct {
	role      Role
	endpoint  string
	config    *config.Config
	server    *grpc.Server
	csiDriver *csiCommon.CSIDriver
	log       *logrus.Entry
}

// Run - run the driver
func (d *Driver) Run() error {
	d.log.Info("run")

	parsedURL, err := url.Parse(d.endpoint)
	if err != nil {
		return fmt.Errorf("Failed to parse endpoint: %s", d.endpoint)
	}

	if parsedURL.Scheme != "unix" {
		return fmt.Errorf("Only unix domain sockets supported")
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
		return fmt.Errorf("Failed to create socket listener: %v", err)
	}

	d.server = grpc.NewServer(grpc.UnaryInterceptor(d.grpcErrorHandler))

	// IdentityServer - should be running on both controller and node pods
	csi.RegisterIdentityServer(d.server, NewIdentityServer(d))

	if d.role.IsController() {
		controllerServer, err := NewControllerServer(d)
		if err != nil {
			return fmt.Errorf("Failed to create ControllerServer: %v", err)
		}
		csi.RegisterControllerServer(d.server, controllerServer)
	}

	if d.role.IsNode() {
		nodeServer, err := NewNodeServer(d)
		if err != nil {
			return fmt.Errorf("Failed to create NodeServer: %v", err)
		}
		csi.RegisterNodeServer(d.server, nodeServer)
	}

	return d.server.Serve(listener)
}

// Validate - validate driver configuration:
// - check NS connection
// - check NS license
func (d *Driver) Validate() error {
	nsResolver, err := ns.NewResolver(ns.ResolverArgs{
		Address:  d.config.Address,
		Username: d.config.Username,
		Password: d.config.Password,
		Log:      d.log,
	})
	if err != nil {
		return fmt.Errorf("Driver validation failed, cannot create NexentaStor(s) resolver: %v", err)
	}

	for _, nsProvider := range nsResolver.Nodes {
		license, err := nsProvider.GetLicense()
		if err != nil {
			return fmt.Errorf("Driver validation failed: %v", err)
		} else if !license.Valid {
			return fmt.Errorf(
				"Driver validation failed, NexentaStor %v has invalid license (expired: %v)",
				nsProvider,
				license.Expires,
			)
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
	l := args.Log.WithField("cmp", "Driver")

	if args.Config == nil {
		return nil, fmt.Errorf("args.Config is required")
	} else if args.Log == nil {
		return nil, fmt.Errorf("args.Log is required")
	}

	l.Infof("create new driver: %v@%v-%v (%v)", Name, Version, Commit, DateTime)

	csiDriver := csiCommon.NewCSIDriver(Name, Version, args.NodeID)

	csiDriver.AddControllerServiceCapabilities(
		[]csi.ControllerServiceCapability_RPC_Type{
			csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
			csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
			//csi.ControllerServiceCapability_RPC_GET_CAPACITY, //TODO
			//csi.ControllerServiceCapability_RPC_LIST_SNAPSHOTS, //TODO
			//csi.ControllerServiceCapability_RPC_CREATE_DELETE_SNAPSHOT, //TODO
		},
	)

	csiDriver.AddVolumeCapabilityAccessModes(
		[]csi.VolumeCapability_AccessMode_Mode{csi.VolumeCapability_AccessMode_MULTI_NODE_MULTI_WRITER},
	)

	d := &Driver{
		role:      args.Role,
		endpoint:  args.Endpoint,
		config:    args.Config,
		log:       l,
		csiDriver: csiDriver,
	}

	return d, nil
}
