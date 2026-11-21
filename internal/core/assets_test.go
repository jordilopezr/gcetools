package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"gcetools/internal/auth"
)

func TestAssetSource(t *testing.T) {
	var mu sync.Mutex
	var parents []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		parents = append(parents, strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/v1/"), "/assets"))
		mu.Unlock()
		if r.URL.Query().Get("contentType") != "RESOURCE" || !strings.Contains(r.URL.RawQuery, "compute.googleapis.com%2FGlobalAddress") {
			t.Errorf("query = %s", r.URL.RawQuery)
		}
		if r.Header.Get("X-Goog-User-Project") != "billing-prj" {
			t.Errorf("quota project header missing")
		}
		switch {
		case strings.Contains(r.URL.Path, "folders/9") && r.URL.Query().Get("pageToken") == "":
			w.Write([]byte(`{"nextPageToken":"p2","assets":[
				{"name":"//compute.googleapis.com/projects/erp/zones/z/instances/a","assetType":"compute.googleapis.com/Instance","resource":{"data":{"name":"a","id":"1"}}},
				{"name":"//compute.googleapis.com/projects/erp/zones/z/disks/a","assetType":"compute.googleapis.com/Disk","resource":{"data":{"name":"a"}}}]}`))
		case strings.Contains(r.URL.Path, "folders/9"):
			w.Write([]byte(`{"assets":[{"name":"//compute.googleapis.com/projects/web/global/networks/default","assetType":"compute.googleapis.com/Network","resource":{"data":{"name":"default"}}}]}`))
		default:
			w.Write([]byte(`{}`)) // per-project listing of a project with nothing
		}
	}))
	defer srv.Close()
	old := assetEndpoint
	assetEndpoint = srv.URL + "/"
	defer func() { assetEndpoint = old }()

	e := &Engine{Session: &auth.Session{QuotaProject: "billing-prj"}, Client: srv.Client()}
	src := e.newAssetSource("folders/9")
	ctx := context.Background()
	erp, err := src.Project(ctx, "erp")
	if err != nil || len(erp["Instance"]) != 1 || len(erp["Disk"]) != 1 {
		t.Fatalf("erp = %v, %v", erp, err)
	}
	if _, ok := erp["Snapshot"]; !ok {
		t.Error("every asset type must be present, even when empty")
	}
	web, _ := src.Project(ctx, "web")
	if len(web["Network"]) != 1 {
		t.Errorf("web = %v", web)
	}
	other, err := src.Project(ctx, "outside")
	if err != nil || len(other["Instance"]) != 0 {
		t.Errorf("outside = %v, %v", other, err)
	}
	if _, ok := other["Instance"]; !ok {
		t.Error("empty project must still mark the types as prefetched")
	}
	want := []string{"folders/9", "folders/9", "projects/outside"}
	if strings.Join(parents, ",") != strings.Join(want, ",") {
		t.Errorf("listings = %v, want %v (the scope must be read once)", parents, want)
	}
}
