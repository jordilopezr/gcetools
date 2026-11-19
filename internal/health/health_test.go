package health

import (
	"os"
	"regexp"
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

func TestRulesCatalog(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range Rules {
		if seen[r.ID] {
			t.Errorf("duplicate rule id %s", r.ID)
		}
		seen[r.ID] = true
	}
	// every ID the evaluator emits must be in the catalog (and so in the UI)
	data, err := os.ReadFile("health.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range regexp.MustCompile(`add\(finding\{"([a-z0-9-]+)"`).FindAllStringSubmatch(string(data), -1) {
		if !seen[m[1]] {
			t.Errorf("rule %s emitted but missing from Rules", m[1])
		}
	}
}

func TestDisabledAndExceptions(t *testing.T) {
	build := func() *model.Inventory {
		inv := collect.NewInventory()
		add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
		add(collect.SProject, model.Row{"projectId": "veeam", "labels": "gcetools-ignore=snapshot-schedule"})
		add(collect.SProject, model.Row{"projectId": "p"})
		for _, vm := range []struct{ p, n, labels string }{
			{"p", "plain", "app=x"},
			{"p", "ignored-one", "app=x; gcetools-ignore=public-ip"},
			{"p", "ignored-stack", "gcetools-ignore-public-ip=true; gcetools-ignore-snapshot-schedule=yes"},
			{"p", "ignored-all", "gcetools-ignore=all"},
			{"veeam", "proxy", "app=veeam"},
		} {
			add(collect.SInfo, model.Row{"project": vm.p, "name": vm.n, "zone": "z", "status": "RUNNING", "externalIP": "1.2.3.4",
				"secureBoot": true, "deletionProtection": true, "labels": vm.labels})
			add(collect.SDisks, model.Row{"project": vm.p, "name": vm.n + "-boot", "location": "z", "users": vm.n})
		}
		return inv
	}
	count := func(rows []model.Row, id, resource string) int {
		n := 0
		for _, r := range rows {
			if r["ruleId"] == id && (resource == "" || r["resource"] == resource) {
				n++
			}
		}
		return n
	}

	inv := build()
	res := Evaluate(inv, DefaultOptions(), time.Now())
	rows := findings(inv)
	if count(rows, "public-ip", "plain") != 1 || count(rows, "public-ip", "ignored-one") != 0 ||
		count(rows, "public-ip", "ignored-stack") != 0 || count(rows, "public-ip", "proxy") != 1 {
		t.Errorf("public-ip exceptions wrong: %v", rows)
	}
	if count(rows, "snapshot-schedule", "proxy") != 0 || count(rows, "snapshot-schedule", "ignored-stack") != 0 ||
		count(rows, "snapshot-schedule", "plain") != 1 {
		t.Errorf("project/stacked snapshot exceptions wrong: %v", rows)
	}
	for _, r := range rows {
		if r["resource"] == "ignored-all" {
			t.Errorf("gcetools-ignore=all must hide everything: %v", r)
		}
	}
	if res.Suppressed != 6 || res.Findings != len(rows) {
		t.Errorf("result = %+v (rows %d)", res, len(rows))
	}

	inv = build()
	opt := DefaultOptions()
	opt.Disabled = []string{"public-ip", "no-such-rule"}
	res = Evaluate(inv, opt, time.Now())
	if count(findings(inv), "public-ip", "") != 0 || len(res.Disabled) != 1 || res.Disabled[0] != "public-ip" {
		t.Errorf("disabled: %+v", res)
	}
}

func TestThresholds(t *testing.T) {
	inv := collect.NewInventory()
	inv.Sheet(collect.SPerf).Add(model.Row{"project": "p", "vm": "a", "zone": "z", "status": "RUNNING", "samples": 500.0,
		"cpuP95": 15.0, "opsAgent": true})
	inv.Sheet(collect.SQuota).Add(model.Row{"project": "p", "metric": "CPUS", "region": "r", "usagePct": 70.0})

	Evaluate(inv, DefaultOptions(), time.Now())
	if len(findings(inv)) != 0 {
		t.Fatalf("defaults should not fire: %v", findings(inv))
	}
	inv.Sheet(collect.SHealth).Rows = nil
	opt := Options{CPULowPct: 20, QuotaPct: 60} // zero fields fall back to defaults
	Evaluate(inv, opt, time.Now())
	if _, ok := has(findings(inv), "VM infrautilizada (CPU p95 < 20%)", "a"); !ok {
		t.Errorf("cpu threshold: %v", findings(inv))
	}
	if _, ok := has(findings(inv), "Cuota regional", "CPUS"); !ok {
		t.Errorf("quota threshold: %v", findings(inv))
	}
}
