package health

import (
	"strings"
	"testing"
	"time"

	"gcetools/internal/collect"
	"gcetools/internal/model"
)

func findings(inv *model.Inventory) []model.Row { return inv.Sheet(collect.SHealth).Rows }

func has(rows []model.Row, rule, resource string) (model.Row, bool) {
	for _, r := range rows {
		if strings.HasPrefix(r["rule"].(string), rule) && r["resource"] == resource {
			return r, true
		}
	}
	return nil, false
}

func TestDisksWithoutSchedule(t *testing.T) {
	inv := collect.NewInventory()
	add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
	add(collect.SInfo, model.Row{"project": "p", "name": "db", "zone": "z", "status": "RUNNING", "secureBoot": true, "labels": "a=b", "deletionProtection": true})
	add(collect.SInfo, model.Row{"project": "p", "name": "web-1", "zone": "z", "status": "RUNNING", "mig": "web"})
	add(collect.SDisks, model.Row{"project": "p", "name": "db-boot", "location": "z", "users": "db", "snapshotSchedule": "daily"})
	add(collect.SDisks, model.Row{"project": "p", "name": "db-data", "location": "z", "users": "db"})
	add(collect.SDisks, model.Row{"project": "p", "name": "db-logs", "location": "z", "users": "db"})
	add(collect.SDisks, model.Row{"project": "p", "name": "web-1", "location": "z", "users": "web-1"})
	add(collect.SDisks, model.Row{"project": "p", "name": "old", "location": "z", "orphan": true, "sizeGB": 10.0})

	Evaluate(inv, DefaultOptions(), time.Now())
	rows := findings(inv)
	r, ok := has(rows, "VM con discos sin snapshot schedule", "db")
	if !ok {
		t.Fatalf("missing finding for db: %v", rows)
	}
	if d := r["detail"].(string); !strings.Contains(d, "db-data, db-logs") || strings.Contains(d, "db-boot") {
		t.Errorf("detail = %q", d)
	}
	if _, ok := has(rows, "VM con discos sin snapshot schedule", "web-1"); ok {
		t.Error("MIG members must be skipped")
	}
	n := 0
	for _, r := range rows {
		if r["rule"] == "VM con discos sin snapshot schedule" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("want one finding (orphans excluded), got %d", n)
	}
}

func TestTemplateImages(t *testing.T) {
	inv := collect.NewInventory()
	add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
	add(collect.SImage, model.Row{"project": "img", "name": "golden-v1", "kind": "Image", "deprecated": "DEPRECATED", "ageDays": int64(30)})
	add(collect.SImage, model.Row{"project": "img", "name": "golden-v0", "kind": "Image", "deprecated": "OBSOLETE", "ageDays": int64(400)})
	add(collect.SImage, model.Row{"project": "p", "name": "base", "kind": "Image", "ageDays": int64(365)})
	add(collect.SImage, model.Row{"project": "p", "name": "fresh", "kind": "Image", "ageDays": int64(10)})
	add(collect.STemplate, model.Row{"project": "p", "name": "t-dep", "image": "golden-v1", "imageProject": "img"})
	add(collect.STemplate, model.Row{"project": "p", "name": "t-obs", "image": "golden-v0", "imageProject": "img"})
	add(collect.STemplate, model.Row{"project": "p", "name": "t-old", "image": "base", "imageProject": "p"})
	add(collect.STemplate, model.Row{"project": "p", "name": "t-fresh", "image": "fresh", "imageProject": "p"})
	add(collect.STemplate, model.Row{"project": "p", "name": "t-public", "image": "family/debian-12", "imageProject": "debian-cloud"})
	add(collect.SMIG, model.Row{"project": "p", "name": "web", "template": "t-obs"})

	Evaluate(inv, DefaultOptions(), time.Now())
	rows := findings(inv)
	if r, ok := has(rows, "Template con imagen obsoleta", "t-obs"); !ok || r["severity"] != High || !strings.Contains(r["detail"].(string), "MIG web") {
		t.Errorf("t-obs: %v %v", ok, r)
	}
	if r, ok := has(rows, "Template con imagen deprecada", "t-dep"); !ok || r["severity"] != Medium {
		t.Errorf("t-dep: %v %v", ok, r)
	}
	if r, ok := has(rows, "Template con imagen custom de más de 180 días", "t-old"); !ok || r["severity"] != Low {
		t.Errorf("t-old: %v %v", ok, r)
	}
	for _, n := range []string{"t-fresh", "t-public"} {
		for _, r := range rows {
			if r["resource"] == n {
				t.Errorf("unexpected finding for %s: %v", n, r)
			}
		}
	}
}

func TestSeverityOrder(t *testing.T) {
	inv := collect.NewInventory()
	inv.Sheet(collect.SInfo).Add(model.Row{"project": "p", "name": "vm", "zone": "z", "status": "RUNNING",
		"serviceAccount": "1-compute@developer.gserviceaccount.com", "scopes": "cloud-platform"})
	Evaluate(inv, DefaultOptions(), time.Now())
	rows := findings(inv)
	if len(rows) == 0 || rows[0]["severity"] != High {
		t.Fatalf("first finding must be High: %v", rows)
	}
	last := 0
	order := map[string]int{High: 0, Medium: 1, Low: 2}
	for _, r := range rows {
		o := order[r["severity"].(string)]
		if o < last {
			t.Fatalf("findings not sorted by severity: %v", rows)
		}
		last = o
	}
}
