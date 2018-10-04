package ns

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"strings"
)

// Resolver - NexentaStor cluster API provider
type Resolver struct {
	Nodes []ProviderInterface
	Log   *logrus.Entry
}

// Resolve - get one NS from the pool of NSs by provided pool or dataset path
func (nsr *Resolver) Resolve(path string) (resolvedNS ProviderInterface, lastError error) {
	if path == "" {
		return nil, fmt.Errorf("Resolved was called with empty pool/dataset path")
	}

	for _, ns := range nsr.Nodes {
		filesystem, err := ns.GetFilesystem(path)
		if err != nil {
			lastError = err
		} else if filesystem != "" {
			resolvedNS = ns
			break
		}
	}

	if resolvedNS != nil {
		nsr.Log.Debugf("Resolve '%v' to %v", path, resolvedNS)
		return resolvedNS, nil
	}

	message := fmt.Sprintf("No NexentaStors found with pool/dataset: '%v'", path)
	if lastError != nil {
		return nil, fmt.Errorf("%v, last error: %v", message, lastError)
	}
	return nil, fmt.Errorf(message)
}

// ResolverArgs - params to create resolver instance from config
type ResolverArgs struct {
	Address  string
	Username string
	Password string
	Log      *logrus.Entry
}

// NewResolver - create NexentaStor resolver instance based on confiuration
func NewResolver(args ResolverArgs) (nsr *Resolver, err error) {
	if len(args.Address) == 0 {
		return nil, fmt.Errorf("NexentaStor address not specified: %v", args.Address)
	}

	resolverLog := args.Log.WithFields(logrus.Fields{
		"cmp": "NSResolver",
		"ns":  args.Address,
	})

	resolverLog.Debugf("Create for %v", args.Address)

	var nodes []ProviderInterface
	addressList := strings.Split(args.Address, ",")
	for _, address := range addressList {
		nsProvider, err := NewProvider(ProviderArgs{
			Address:  address,
			Username: args.Username,
			Password: args.Password,
			Log:      resolverLog,
		})
		if err != nil {
			return nil, fmt.Errorf("Cannot create provider for %v NexentaStor: %v", address, err)
		}
		nodes = append(nodes, nsProvider)
	}

	nsr = &Resolver{
		Nodes: nodes,
		Log:   resolverLog,
	}

	return nsr, nil
}
