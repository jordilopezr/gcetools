package core

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"gcetools/internal/collect"
	"gcetools/internal/model"

	cloudasset "google.golang.org/api/cloudasset/v1"
	"google.golang.org/api/option"
)

const computeAsset = "compute.googleapis.com/"

// assetSource feeds the fast mode. With an organization or folder scope it
// reads every Compute resource under it in one paged listing, the first
// time a project asks; projects missing from that listing (outside the
// scope, or with no resources) get a listing of their own. Without a scope
// each project costs one listing instead of a dozen Compute API calls.
type assetSource struct {
	clients []*http.Client // quota-project client first, like Discover
	scope   string         // organizations/N or folders/N; "" = per project

	once      sync.Once
	byProject map[string]collect.Assets
	bulkErr   error
}

func (e *Engine) newAssetSource(scope string) *assetSource {
	a := &assetSource{clients: []*http.Client{e.Client}}
	if e.Session.QuotaProject != "" {
		a.clients = []*http.Client{withQuotaProject(e.Client, e.Session.QuotaProject), e.Client}
	}
	if s := strings.TrimSpace(scope); strings.HasPrefix(s, "organizations/") || strings.HasPrefix(s, "folders/") {
		a.scope = s
	}
	return a
}

func (a *assetSource) Project(ctx context.Context, projectID string) (collect.Assets, error) {
	if a.scope != "" {
		a.once.Do(func() { a.byProject, a.bulkErr = a.list(ctx, a.scope) })
		if a.bulkErr == nil {
			if as, ok := a.byProject[projectID]; ok {
				return as, nil
			}
		}
	}
	all, err := a.list(ctx, "projects/"+projectID)
	if err != nil {
		return nil, err
	}
	if as, ok := all[projectID]; ok {
		return as, nil
	}
	return emptyAssets(), nil
}

// list reads every type in collect.AssetTypes under parent, grouped by
// project ID, trying each client until one works.
func (a *assetSource) list(ctx context.Context, parent string) (map[string]collect.Assets, error) {
	var firstErr error
	for _, hc := range a.clients {
		out, err := listAssets(ctx, hc, parent)
		if err == nil {
			return out, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if firstErr == nil {
			firstErr = err
		}
	}
	return nil, assetError(firstErr, parent)
}

func listAssets(ctx context.Context, hc *http.Client, parent string) (map[string]collect.Assets, error) {
	opts := []option.ClientOption{option.WithHTTPClient(hc)}
	if assetEndpoint != "" {
		opts = append(opts, option.WithEndpoint(assetEndpoint))
	}
	svc, err := cloudasset.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}
	types := make([]string, len(collect.AssetTypes))
	for i, t := range collect.AssetTypes {
		types[i] = computeAsset + t
	}
	out := map[string]collect.Assets{}
	err = svc.Assets.List(parent).AssetTypes(types...).ContentType("RESOURCE").PageSize(1000).Context(ctx).
		Pages(ctx, func(r *cloudasset.ListAssetsResponse) error {
			for _, as := range r.Assets {
				id := projectFromName(as.Name)
				if id == "" || as.Resource == nil || len(as.Resource.Data) == 0 {
					continue
				}
				p := out[id]
				if p == nil {
					p = emptyAssets()
					out[id] = p
				}
				t := strings.TrimPrefix(as.AssetType, computeAsset)
				p[t] = append(p[t], json.RawMessage(as.Resource.Data))
			}
			return nil
		})
	return out, err
}

func emptyAssets() collect.Assets {
	a := make(collect.Assets, len(collect.AssetTypes))
	for _, t := range collect.AssetTypes {
		a[t] = nil
	}
	return a
}

func assetError(err error, parent string) error {
	kind, msg := collect.Classify(err)
	switch kind {
	case model.ErrPermission:
		msg += fmt.Sprintf(". Hace falta roles/cloudasset.viewer en %s", parent)
	case model.ErrAPIDisabled:
		msg += ". Habilita la Cloud Asset API en el quota project (o en el proyecto de la service account)"
	}
	return fmt.Errorf("Cloud Asset Inventory: %s", msg)
}
