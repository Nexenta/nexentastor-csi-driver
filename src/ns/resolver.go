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

func arrayContains(array []string, value string) bool {
	for _, v := range array {
		if v == value {
			return true
		}
	}
	return false
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

// ResolverArgs - params to create resolver instanse
type ResolverArgs struct {
	Nodes []ProviderInterface
	Log   *logrus.Entry
}

// NewResolver - create NexentaStor resolver instance
func NewResolver(args ResolverArgs) (nsr *Resolver, err error) {
	var clusterAddressesArray []string
	for _, ns := range args.Nodes {
		address := fmt.Sprint(ns)
		if arrayContains(clusterAddressesArray, address) {
			return nil, fmt.Errorf("Duplicated NexentaStor address: %v", address)
		}
		clusterAddressesArray = append(clusterAddressesArray, address)
	}
	clusterAddresses := strings.Join(clusterAddressesArray, ",")

	resolverLog := args.Log.WithFields(logrus.Fields{
		"cmp": "NSResolver",
		"ns":  clusterAddresses,
	})

	resolverLog.Debugf("Create for %v", clusterAddresses)

	nsr = &Resolver{
		Nodes: args.Nodes,
		Log:   resolverLog,
	}

	return nsr, nil
}
