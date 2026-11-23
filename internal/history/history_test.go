package history

import (
	"gcetools/internal/schema"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gcetools/internal/model"
)

func store(t *testing.T) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "history")
	old := Dir
	Dir = func() (string, error) { return dir, nil }
	t.Cleanup(func() { Dir = old })
	return dir
}

func inventory(at time.Time, vms int) *model.Inventory {
	inv := schema.NewInventory()
	inv.Meta = model.Meta{Tool: "gceTools", Version: "0.6.0", StartedAt: at, Projects: 2, ProjectsOK: 2, Currency: "USD"}
	for i := 0; i < vms; i++ {
		inv.Sheet(schema.SInfo).Add(model.Row{"name": "vm", "status": "RUNNING", "vcpu": int64(2), "memoryGB": 8.0})
	}
	inv.Sheet(schema.SDisks).Add(model.Row{"sizeGB": 100.0, "orphan": true})
	// CostList sums the projects; the TOTAL row (only with several projects) is skipped.
	inv.Sheet(schema.SCost).Add(model.Row{"project": "p1", "total": 100.456})
	inv.Sheet(schema.SCost).Add(model.Row{"project": "p2", "total": 23.0})
	inv.Sheet(schema.SCost).Add(model.Row{"project": "TOTAL", "total": 123.456})
	inv.Sheet(schema.SHealth).Add(model.Row{"severity": "Alta"})
	inv.Sheet(schema.SHealth).Add(model.Row{"severity": "Baja"})
	return inv
}

func TestSaveListLoadDelete(t *testing.T) {
	dir := store(t)
	if all, err := List(); err != nil || len(all) != 0 {
		t.Fatalf("empty history: %v %v", all, err)
	}
	t0 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)
	s1, err := Save(inventory(t0, 3), "prod")
	if err != nil {
		t.Fatal(err)
	}
	if s1.ID != "20261101-090000" || s1.VMs != 3 || s1.VCPU != 6 || s1.CostList != 123.46 || s1.High != 1 || s1.Low != 1 ||
		s1.Orphans != 1 || s1.Profile != "prod" || s1.SizeBytes == 0 {
		t.Errorf("summary = %+v", s1)
	}
	s2, _ := Save(inventory(t0, 4), "") // same second: unique id
	if s2.ID != "20261101-090000-2" {
		t.Errorf("duplicate id = %s", s2.ID)
	}
	s3, _ := Save(inventory(t0.Add(24*time.Hour), 5), "")

	all, err := List()
	if err != nil || len(all) != 3 || all[0].ID != s3.ID {
		t.Fatalf("list = %+v %v", all, err)
	}
	inv, err := Load(s1.ID)
	if err != nil || len(inv.Sheet(schema.SInfo).Rows) != 3 {
		t.Fatalf("load: %v", err)
	}
	if p, ok := Previous(s3.StartedAt); !ok || !p.StartedAt.Equal(t0) {
		t.Errorf("previous = %+v %v", p, ok)
	}
	if _, err := Load("../../etc/passwd"); err == nil {
		t.Error("path traversal must be rejected")
	}

	// a lost index is rebuilt from the snapshots
	os.Remove(filepath.Join(dir, "index.json"))
	if all, err := List(); err != nil || len(all) != 3 || all[2].VMs == 0 {
		t.Fatalf("rebuild = %+v %v", all, err)
	}
	if err := Delete(s2.ID); err != nil {
		t.Fatal(err)
	}
	if all, _ := List(); len(all) != 2 {
		t.Errorf("after delete: %d", len(all))
	}
}

func TestRetention(t *testing.T) {
	dir := store(t)
	old := Keep
	Keep = 3
	defer func() { Keep = old }()
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.Local)
	for i := 0; i < 5; i++ {
		if _, err := Save(inventory(t0.Add(time.Duration(i)*time.Hour), 1), ""); err != nil {
			t.Fatal(err)
		}
	}
	all, _ := List()
	files, _ := filepath.Glob(filepath.Join(dir, "*.json.gz"))
	if len(all) != 3 || len(files) != 3 || !all[2].StartedAt.Equal(t0.Add(2*time.Hour)) {
		t.Errorf("kept %d entries / %d files: %+v", len(all), len(files), all)
	}
}

// A damaged index is rebuilt on Save instead of being replaced by the new
// entry alone (which hid every older scan), and bad IDs are never used as
// file names.
func TestDamagedIndex(t *testing.T) {
	dir := store(t)
	t1 := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)
	if _, err := Save(inventory(t1, 1), ""); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Save(inventory(t1.Add(time.Hour), 2), ""); err != nil {
		t.Fatal(err)
	}
	all, err := List()
	if err != nil || len(all) != 2 {
		t.Fatalf("List = %d entries, %v; want both scans", len(all), err)
	}

	outside := filepath.Join(filepath.Dir(dir), "victim.json.gz")
	if err := os.WriteFile(outside, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	idx := `[{"id":"../victim","startedAt":"2000-01-01T00:00:00Z"}]`
	if err := os.WriteFile(filepath.Join(dir, "index.json"), []byte(idx), 0o600); err != nil {
		t.Fatal(err)
	}
	old := Keep
	Keep = 1
	defer func() { Keep = old }()
	if _, err := Save(inventory(t1.Add(2*time.Hour), 1), ""); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(outside); err != nil {
		t.Errorf("pruning followed an ID from the index: %v", err)
	}
}

func TestGet(t *testing.T) {
	store(t)
	at := time.Date(2026, 11, 1, 9, 0, 0, 0, time.Local)
	saved, err := Save(inventory(at, 3), "")
	if err != nil {
		t.Fatal(err)
	}
	s, err := Get(saved.ID)
	if err != nil || s.VMs != 3 || !s.StartedAt.Equal(at) {
		t.Fatalf("Get = %+v, %v", s, err)
	}
	if _, err := Get("20000101-000000"); err == nil {
		t.Error("missing scan must fail")
	}
}
