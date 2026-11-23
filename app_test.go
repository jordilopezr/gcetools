package main

import (
	"context"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"gcetools/internal/auth"
	"gcetools/internal/core"
	"gcetools/internal/export"
	"gcetools/internal/profile"
	"gcetools/internal/schema"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/oauth2"
)

// fakeUI records events and answers dialogs from a queue of paths.
type fakeUI struct {
	mu     sync.Mutex
	events []string
	files  []string
}

func (f *fakeUI) Emit(event string, _ ...any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}
func (f *fakeUI) next() (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.files) == 0 {
		return "", nil
	}
	p := f.files[0]
	f.files = f.files[1:]
	return p, nil
}
func (f *fakeUI) OpenFile(runtime.OpenDialogOptions) (string, error) { return f.next() }
func (f *fakeUI) SaveFile(runtime.SaveDialogOptions) (string, error) { return f.next() }
func (f *fakeUI) OpenURL(string)                                     {}
func (f *fakeUI) saw(event string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range f.events {
		if e == event {
			return true
		}
	}
	return false
}

// computeStub answers Compute calls without a network: one VM per project
// and empty lists for the rest. While gate is open (non-nil) calls wait on it.
type computeStub struct{ gate chan struct{} }

func (c *computeStub) RoundTrip(r *http.Request) (*http.Response, error) {
	if c.gate != nil {
		select {
		case <-c.gate:
		case <-r.Context().Done():
			return nil, r.Context().Err()
		}
	}
	body := `{}`
	if strings.HasSuffix(r.URL.Path, "/aggregated/instances") {
		body = `{"items":{"zones/z-a":{"instances":[{"name":"vm-1","id":"1","zone":"https://x/zones/z-a","status":"RUNNING"}]}}}`
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}},
		Body: io.NopCloser(strings.NewReader(body)), Request: r}, nil
}

func newTestApp(t *testing.T, stub *computeStub) (*App, *fakeUI) {
	t.Helper()
	ui := &fakeUI{}
	a := NewApp()
	a.ctx, a.ui = context.Background(), ui
	sess := &auth.Session{TokenSource: oauth2.StaticTokenSource(&oauth2.Token{AccessToken: "x"}), Method: auth.ADC, FullScope: true}
	a.engine = &core.Engine{Session: sess, Client: &http.Client{Transport: stub}}
	return a, ui
}

func TestScanPublishesInventory(t *testing.T) {
	a, ui := newTestApp(t, &computeStub{})
	if err := a.StartScan([]string{"p1", "p2"}, core.ScanOptions{Concurrency: 2}); err != nil {
		t.Fatal(err)
	}
	a.scans.Wait()
	if !ui.saw("scan:done") {
		t.Fatalf("events = %v", ui.events)
	}
	s := a.InventorySummary()
	if s == nil || s.Stats.VMs != 2 {
		t.Fatalf("summary = %+v", s)
	}
	rows, err := a.SheetRows(schema.SInfo, false)
	if err != nil || len(rows) != 2 {
		t.Errorf("rows = %v, %v", rows, err)
	}
	if _, err := a.SheetRows("nope", false); err == nil {
		t.Error("unknown sheet must fail")
	}
}

// Disconnecting during a scan drops its result instead of publishing it.
func TestDisconnectDiscardsRunningScan(t *testing.T) {
	stub := &computeStub{gate: make(chan struct{})}
	a, ui := newTestApp(t, stub)
	if err := a.StartScan([]string{"p1"}, core.ScanOptions{Concurrency: 1}); err != nil {
		t.Fatal(err)
	}
	if err := a.StartScan([]string{"p1"}, core.ScanOptions{}); err == nil {
		t.Error("a second scan must be refused while one runs")
	}
	a.Disconnect()
	close(stub.gate)
	a.scans.Wait()
	if ui.saw("scan:done") || ui.saw("scan:error") {
		t.Errorf("superseded scan emitted %v", ui.events)
	}
	if a.InventorySummary() != nil {
		t.Error("inventory resurrected after Disconnect")
	}
}

// The key path of a profile comes from the native dialog or the stored
// profile, never from the UI.
func TestSaveProfileIgnoresKeyPathFromUI(t *testing.T) {
	dir := t.TempDir()
	old := profile.Path
	profile.Path = func() (string, error) { return filepath.Join(dir, "profiles.json"), nil }
	defer func() { profile.Path = old }()

	a, _ := newTestApp(t, &computeStub{})
	if err := a.SaveProfile(profile.Profile{Name: "x", Auth: "sa", KeyPath: "/tmp/evil.json"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := profile.Get("x"); p.KeyPath != "" {
		t.Fatalf("key path from UI stored: %q", p.KeyPath)
	}
	a.keyPath = "/home/u/key.json"
	if err := a.SaveProfile(profile.Profile{Name: "x", Auth: "sa"}); err != nil {
		t.Fatal(err)
	}
	a.keyPath = ""
	if err := a.SaveProfile(profile.Profile{Name: "x", Auth: "sa", KeyPath: "/tmp/evil.json"}); err != nil {
		t.Fatal(err)
	}
	if p, _ := profile.Get("x"); p.KeyPath != "/home/u/key.json" {
		t.Errorf("stored key path = %q", p.KeyPath)
	}
}

// Only the last two picked JSON snapshots are kept in memory.
func TestPickedSnapshotsAreBounded(t *testing.T) {
	dir := t.TempDir()
	a, ui := newTestApp(t, &computeStub{})
	for _, n := range []string{"a", "b", "c"} {
		path := filepath.Join(dir, n+".json")
		inv := schema.NewInventory()
		inv.Meta.Tool = "gceTools"
		if _, err := export.Write(inv, path, export.JSON, ','); err != nil {
			t.Fatal(err)
		}
		ui.files = append(ui.files, path)
		if _, err := a.PickSnapshot(); err != nil {
			t.Fatal(err)
		}
	}
	if len(a.snaps) != 2 || a.snaps[filepath.Join(dir, "a.json")] != nil {
		t.Errorf("snaps = %v", a.snapOrder)
	}
	if _, err := a.Compare(filepath.Join(dir, "b.json"), filepath.Join(dir, "c.json")); err != nil {
		t.Errorf("compare: %v", err)
	}
	if _, err := a.Compare(filepath.Join(dir, "a.json"), filepath.Join(dir, "c.json")); err == nil {
		t.Error("an evicted snapshot must be picked again")
	}
}
