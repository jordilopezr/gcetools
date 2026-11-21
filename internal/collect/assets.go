package collect

import (
	"context"
	"encoding/json"
	"fmt"
)

// AssetTypes are the Compute Engine resource types the fast mode reads from
// Cloud Asset Inventory instead of the Compute API. Cloud Asset stores them
// in the same JSON representation as the Compute API, so they decode into
// the compute package types. Keys are the short type names collectors use.
var AssetTypes = []string{
	"Instance", "Disk", "Snapshot", "Image", "MachineImage", "InstanceTemplate",
	"InstanceGroupManager", "Autoscaler", "Network", "Subnetwork", "Firewall",
	"Address", "GlobalAddress",
}

// Assets maps a short asset type to the resources of one project.
type Assets map[string][]json.RawMessage

// AssetSource returns the prefetched resources of one project. Every type in
// AssetTypes must be present in the result (empty when the project has none),
// so collectors can tell "no resources" from "not prefetched".
type AssetSource interface {
	Project(ctx context.Context, projectID string) (Assets, error)
}

// each feeds fn with the prefetched resources of assetType and reports true
// when the scan runs in fast mode; otherwise it returns false and the caller
// queries the Compute API.
func each[T any](c *collector, assetType string, fn func(*T)) (bool, error) {
	raws, ok := c.assets[assetType]
	if !ok {
		return false, nil
	}
	for _, raw := range raws {
		v := new(T)
		if err := json.Unmarshal(raw, v); err != nil {
			return true, fmt.Errorf("Cloud Asset Inventory: %s no válido: %w", assetType, err)
		}
		fn(v)
	}
	return true, nil
}
