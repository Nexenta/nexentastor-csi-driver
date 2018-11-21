package ns

import (
	"fmt"
	"strings"

	"github.com/sirupsen/logrus"
)

// Resolver - NexentaStor cluster API provider
type Resolver struct {
	Nodes []ProviderInterface
	Log   *logrus.Entry
}

// Resolve - get one NS from the list of NSs by provided pool/dataset/fs path
func (nsr *Resolver) Resolve(path string) (resolvedNS ProviderInterface, lastError error) {
	l := nsr.Log.WithField("func", "Resolve()")

	if path == "" {
		return nil, fmt.Errorf("Resolved was called with empty pool/dataset path")
	}

	//TODO do non-block requests to all NSs in the list, select first one responded
	for _, ns := range nsr.Nodes {
		_, err := ns.GetFilesystem(path)
		if err != nil {
			lastError = err
		} else {
			resolvedNS = ns
			break
		}
	}

	if resolvedNS != nil {
		l.Debugf("resolve '%v' to '%v'", path, resolvedNS)
		return resolvedNS, nil
	}

	message := fmt.Sprintf("No NexentaStor(s) found with pool/dataset: '%v'", path)
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

// NewResolver - create NexentaStor resolver instance based on configuration
func NewResolver(args ResolverArgs) (nsr *Resolver, err error) {
	if len(args.Address) == 0 {
		return nil, fmt.Errorf("NexentaStor address not specified: %v", args.Address)
	}

	l := args.Log.WithFields(logrus.Fields{
		"cmp": "NSResolver",
		"ns":  args.Address,
	})

	l.Debugf("created for %v", args.Address)

	var nodes []ProviderInterface
	addressList := strings.Split(args.Address, ",")
	for _, address := range addressList {
		nsProvider, err := NewProvider(ProviderArgs{
			Address:  address,
			Username: args.Username,
			Password: args.Password,
			Log:      l,
		})
		if err != nil {
			return nil, fmt.Errorf("Cannot create provider for %v NexentaStor: %v", address, err)
		}
		nodes = append(nodes, nsProvider)
	}

	nsr = &Resolver{
		Nodes: nodes,
		Log:   l,
	}

	return nsr, nil
}
