package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"gcetools/internal/auth"
)

func TestDiscover(t *testing.T) {
	var quotaHeader, scopePath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		quotaHeader = r.Header.Get("X-Goog-User-Project")
		scopePath = r.URL.Path
		if !strings.Contains(r.URL.RawQuery, "assetTypes=compute.googleapis.com%2FInstance") {
			t.Errorf("asset types missing: %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("pageToken") == "" {
			w.Write([]byte(`{"nextPageToken":"p2","results":[
				{"name":"//compute.googleapis.com/projects/erp-prod/zones/z/instances/a","assetType":"compute.googleapis.com/Instance","project":"projects/111","state":"RUNNING"},
				{"name":"//compute.googleapis.com/projects/erp-prod/zones/z/instances/b","assetType":"compute.googleapis.com/Instance","project":"projects/111","state":"TERMINATED"},
				{"name":"//compute.googleapis.com/projects/erp-prod/zones/z/disks/a","assetType":"compute.googleapis.com/Disk","project":"projects/111"}]}`))
			return
		}
		w.Write([]byte(`{"results":[
			{"name":"//compute.googleapis.com/projects/backup/global/snapshots/s1","assetType":"compute.googleapis.com/Snapshot","project":"projects/222"}]}`))
	}))
	defer srv.Close()
	old := assetEndpoint
	assetEndpoint = srv.URL + "/"
	defer func() { assetEndpoint = old }()

	e := &Engine{Session: &auth.Session{QuotaProject: "billing-prj"}, Client: srv.Client()}
	got, err := e.Discover(context.Background(), "organizations/123")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(scopePath, "/v1/organizations/123:searchAllResources") {
		t.Errorf("path = %s", scopePath)
	}
	if quotaHeader != "billing-prj" {
		t.Errorf("quota project header = %q", quotaHeader)
	}
	if len(got) != 2 {
		t.Fatalf("got %+v", got)
	}
	if f := got[1]; f.ProjectID != "erp-prod" || f.Number != "111" || f.VMs != 2 || f.Running != 1 || f.Disks != 1 {
		t.Errorf("erp-prod = %+v", f)
	}
	if f := got[0]; f.ProjectID != "backup" || f.Other != 1 || f.Total() != 1 {
		t.Errorf("backup = %+v", f)
	}
}

func TestValidScope(t *testing.T) {
	for s, want := range map[string]bool{
		"organizations/123": true, "folders/456": true, "projects/my-prj-1": true,
		"organizations/abc": false, "folders/": false, "123": false, "projects/X": false,
	} {
		if ValidScope(s) != want {
			t.Errorf("ValidScope(%q) = %v", s, !want)
		}
	}
}
