package core

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"gcetools/internal/auth"
	"gcetools/internal/schema"

	"golang.org/x/oauth2"
)

// fakeCompute answers every project with one running VM; everything else
// is an empty list. Calls counts the requests.
func fakeCompute(t *testing.T) (*Engine, *atomic.Int64) {
	t.Helper()
	var calls atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		path := r.URL.Path
		switch {
		case strings.HasSuffix(path, "/aggregated/instances"):
			_, rest, _ := strings.Cut(path, "/projects/")
			p, _, _ := strings.Cut(rest, "/")
			w.Write([]byte(`{"items":{"zones/europe-west1-b":{"instances":[{"name":"vm-` + p + `","id":"1",
				"zone":"https://x/zones/europe-west1-b","machineType":"https://x/machineTypes/e2-standard-2","status":"RUNNING"}]}}}`))
		case strings.Contains(path, "/machineTypes/"):
			w.Write([]byte(`{"guestCpus":2,"memoryMb":8192}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	old := computeEndpoint
	computeEndpoint = srv.URL + "/"
	t.Cleanup(func() { computeEndpoint = old })
	sess := &auth.Session{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "x"}), Method: auth.ADC, FullScope: true}
	return &Engine{Session: sess, Client: srv.Client()}, &calls
}

func projects(ids ...string) []Project {
	out := make([]Project, len(ids))
	for i, id := range ids {
		out[i] = Project{ID: id}
	}
	return out
}

// The worker pool reads every project once and progress is reported
// concurrently (run with -race).
func TestScanWorkers(t *testing.T) {
	e, _ := fakeCompute(t)
	var mu sync.Mutex
	done := map[string]bool{}
	inv, err := e.Scan(context.Background(), projects("p1", "p2", "p3", "p4"), ScanOptions{Concurrency: 2}, func(p Progress) {
		if p.Step == "done" {
			mu.Lock()
			done[p.Project] = true
			mu.Unlock()
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 4 || inv.Meta.Projects != 4 || inv.Meta.ProjectsOK != 4 || inv.Meta.Cancelled {
		t.Errorf("done=%v meta=%+v", done, inv.Meta)
	}
	if n := len(inv.Rows(schema.SInfo)); n != 4 {
		t.Errorf("gInfo rows = %d", n)
	}
}

// Cancelling stops feeding projects and returns a partial inventory.
func TestScanCancel(t *testing.T) {
	e, _ := fakeCompute(t)
	ctx, cancel := context.WithCancel(context.Background())
	var once sync.Once
	ids := make([]string, 50)
	for i := range ids {
		ids[i] = "p" + string(rune('a'+i%26)) + string(rune('a'+i/26))
	}
	inv, err := e.Scan(ctx, projects(ids...), ScanOptions{Concurrency: 1}, func(p Progress) {
		if p.Step == "done" {
			once.Do(cancel)
		}
	})
	if err != nil {
		t.Fatal(err)
	}
	if !inv.Meta.Cancelled || inv.Meta.ProjectsOK >= 50 {
		t.Errorf("meta = %+v", inv.Meta)
	}
}

// Without cloud-platform, CAI, OS Config and Recommender are switched off
// up front with one note each, and no call is made to them.
func TestScanReadOnlyScopes(t *testing.T) {
	e, _ := fakeCompute(t)
	e.Session.FullScope = false
	opt := ScanOptions{Concurrency: 1, Modules: Modules{OSConfig: true, Recommender: true, Fast: true}}
	inv, err := e.Scan(context.Background(), projects("p1"), opt, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, er := range inv.Errors {
		if er.Project == "(all)" {
			got[er.Collector] = true
		}
	}
	if !got["cloudasset"] || !got["osconfig"] || !got["recommender"] || len(inv.Rows(schema.STools)) != 0 {
		t.Errorf("errors = %v", inv.Errors)
	}
	if inv.Meta.ProjectsOK != 1 {
		t.Errorf("project must still be OK: %+v", inv.Meta)
	}
}
