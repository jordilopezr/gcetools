package core

import (
	"context"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"

	"gcetools/internal/collect"
	"gcetools/internal/i18n"
	"gcetools/internal/model"

	cloudasset "google.golang.org/api/cloudasset/v1"
	"google.golang.org/api/option"
)

// Footprint is the Compute Engine presence of one project as seen by Cloud
// Asset Inventory. Projects without footprint (no VMs, disks, snapshots,
// images…) are not worth a full scan: in large organizations that is most
// of them, and skipping them saves thousands of Compute API calls.
type Footprint struct {
	ProjectID string `json:"projectId"`
	Number    string `json:"number"`
	VMs       int    `json:"vms"`
	Running   int    `json:"running"`
	Disks     int    `json:"disks"`
	Other     int    `json:"other"` // snapshots, images, templates, reservations, commitments
}

func (f Footprint) Total() int { return f.VMs + f.Disks + f.Other }

// discoverTypes are the asset types that make a project worth scanning.
// Networks and firewalls are left out on purpose: almost every project has a
// default VPC even when it never ran a VM.
var discoverTypes = []string{
	"compute.googleapis.com/Instance",
	"compute.googleapis.com/Disk",
	"compute.googleapis.com/Snapshot",
	"compute.googleapis.com/Image",
	"compute.googleapis.com/MachineImage",
	"compute.googleapis.com/InstanceTemplate",
	"compute.googleapis.com/Reservation",
	"compute.googleapis.com/Commitment",
}

var reScope = regexp.MustCompile(`^(organizations|folders)/\d+$|^projects/[a-z][-a-z0-9]{4,28}[a-z0-9]$`)

// ValidScope reports whether s is organizations/N, folders/N or projects/ID.
func ValidScope(s string) bool { return reScope.MatchString(s) }

// Discover runs one paged Cloud Asset Inventory search over scope and returns
// the projects that hold Compute resources, sorted by project ID. It needs
// roles/cloudasset.viewer on the scope and the Cloud Asset API enabled in the
// quota project (user credentials) or in the service account's project.
func (e *Engine) Discover(ctx context.Context, scope string) ([]Footprint, error) {
	scope = strings.TrimSpace(scope)
	if !ValidScope(scope) {
		return nil, fmt.Errorf(i18n.T("ámbito no válido %q: usa organizations/123, folders/456 o projects/mi-proyecto"), scope)
	}
	// With user credentials Cloud Asset needs a quota project; try it first
	// and fall back to the plain client (service accounts bill their own).
	clients := []*http.Client{e.Client}
	if e.Session.QuotaProject != "" {
		clients = []*http.Client{withQuotaProject(e.Client, e.Session.QuotaProject), e.Client}
	}
	var firstErr error
	for _, hc := range clients {
		out, err := discover(ctx, hc, scope)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if firstErr == nil {
			firstErr = err // the quota-project attempt explains the real cause
		}
	}
	kind, msg := collect.Classify(firstErr)
	switch kind {
	case model.ErrPermission:
		msg += i18n.Tf(". Hace falta roles/cloudasset.viewer en %s", scope)
	case model.ErrAPIDisabled:
		msg += i18n.T(". Habilita la Cloud Asset API en el quota project (o en el proyecto de la service account)")
	}
	return nil, fmt.Errorf("Cloud Asset Inventory: %s", msg)
}

// assetEndpoint overrides the Cloud Asset API base URL (tests).
var assetEndpoint string

func discover(ctx context.Context, hc *http.Client, scope string) ([]Footprint, error) {
	opts := []option.ClientOption{option.WithHTTPClient(hc)}
	if assetEndpoint != "" {
		opts = append(opts, option.WithEndpoint(assetEndpoint))
	}
	svc, err := cloudasset.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}
	byID := map[string]*Footprint{}
	err = svc.V1.SearchAllResources(scope).AssetTypes(discoverTypes...).
		ReadMask("name,assetType,project,state").PageSize(500).Context(ctx).
		Pages(ctx, func(r *cloudasset.SearchAllResourcesResponse) error {
			for _, res := range r.Results {
				id := projectFromName(res.Name)
				if id == "" {
					continue
				}
				f := byID[id]
				if f == nil {
					f = &Footprint{ProjectID: id, Number: strings.TrimPrefix(res.Project, "projects/")}
					byID[id] = f
				}
				switch res.AssetType {
				case "compute.googleapis.com/Instance":
					f.VMs++
					if res.State == "RUNNING" {
						f.Running++
					}
				case "compute.googleapis.com/Disk":
					f.Disks++
				default:
					f.Other++
				}
			}
			return nil
		})
	if err != nil {
		return nil, err
	}
	out := make([]Footprint, 0, len(byID))
	for _, f := range byID {
		out = append(out, *f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ProjectID < out[j].ProjectID })
	return out, nil
}

// projectFromName extracts the project ID from a Compute full resource name:
// //compute.googleapis.com/projects/<id>/zones/<z>/instances/<vm>.
func projectFromName(name string) string {
	const marker = "/projects/"
	i := strings.Index(name, marker)
	if i < 0 {
		return ""
	}
	rest := name[i+len(marker):]
	if j := strings.IndexByte(rest, '/'); j >= 0 {
		rest = rest[:j]
	}
	return rest
}
