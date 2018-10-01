package nscluster

import (
	"fmt"
	"github.com/sirupsen/logrus"

	"github.com/Nexenta/nexentastor-csi-driver/src/ns"
)

// Provider - NexentaStor cluster API provider
type Provider struct {
	NSProvider1 *ns.Provider
	NSProvider2 *ns.Provider
	Log         *logrus.Entry
}

// ProviderArgs - params to create cluster provider instanse
type ProviderArgs struct {
	NSProvider1 *ns.Provider
	NSProvider2 *ns.Provider
	Log         *logrus.Entry
}

// NewProvider - create NexentaStor cluster provider instance
func NewProvider(args ProviderArgs) (nscp *Provider, err error) {
	clusterAddresses := fmt.Sprintf("%v,%v", args.NSProvider1.Address, args.NSProvider2.Address)
	providerLog := args.Log.WithFields(logrus.Fields{
		"cmp": "ns-cluster-provider",
		"ns":  clusterAddresses,
	})

	providerLog.Debugf("Create for %v", clusterAddresses)

	if args.NSProvider1.Address == args.NSProvider2.Address {
		return nil, fmt.Errorf("Cluster nodes cannot have same address: %v", args.NSProvider1.Address)
	}

	nscp = &Provider{
		NSProvider1: args.NSProvider1,
		NSProvider2: args.NSProvider2,
		Log:         providerLog,
	}

	return nscp, nil
}
