package diff

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gcetools/internal/collect"
	"gcetools/internal/export"
	"gcetools/internal/model"
)

var created = time.Date(2026, 1, 10, 9, 30, 0, 0, time.UTC)

func baseInventory() *model.Inventory {
	inv := collect.NewInventory()
	inv.Meta = model.Meta{Tool: "gceTools", Version: "0.3.0", Currency: "USD", StartedAt: created}
	add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
	for _, p := range []string{"p1", "p2"} {
		add(collect.SProject, model.Row{"projectId": p, "scanStatus": "OK", "vms": int64(2)})
	}
	add(collect.SInfo, model.Row{"project": "p1", "zone": "z", "name": "app", "machineType": "e2-standard-2",
		"vcpu": int64(2), "memoryGB": 8.0, "status": "RUNNING", "created": created, "lastStart": created,
		"secureBoot": true, "costMonthly": 50.0})
	add(collect.SInfo, model.Row{"project": "p1", "zone": "z", "name": "old", "machineType": "n1-standard-1", "status": "TERMINATED"})
	add(collect.SInfo, model.Row{"project": "p2", "zone": "z", "name": "db", "machineType": "n2-standard-4", "status": "RUNNING"})
	add(collect.SFirewall, model.Row{"project": "p1", "name": "allow-internal", "network": "default", "rules": "tcp:80",
		"sourceRanges": "10.0.0.0/8", "openToInternet": false})
	add(collect.SSnapshot, model.Row{"project": "p1", "name": "sched-1", "autoCreated": true, "ageDays": int64(1)})
	add(collect.SCost, model.Row{"project": "p1", "total": 50.0})
	return inv
}

// roundTrip writes and reads back a JSON snapshot, as the GUI and CLI do.
func roundTrip(t *testing.T, inv *model.Inventory) *model.Inventory {
	t.Helper()
	path := filepath.Join(t.TempDir(), "snap.json")
	if _, err := export.Write(inv, path, export.JSON, ','); err != nil {
		t.Fatal(err)
	}
	out, err := export.ReadJSON(path)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func rowsOf(inv *model.Inventory, sheet string) []model.Row { return inv.Sheet(sheet).Rows }

func TestIdenticalSnapshotsHaveNoChanges(t *testing.T) {
	a := baseInventory()
	res := Compare(Input{a, "a"}, Input{roundTrip(t, baseInventory()), "b"})
	for _, s := range []string{SSummary, SAdded, SRemoved, SChanged} {
		if n := len(rowsOf(res, s)); n != 0 {
			t.Errorf("%s: %d rows, want 0: %v", s, n, rowsOf(res, s))
		}
	}
}

func TestCompare(t *testing.T) {
	a := roundTrip(t, baseInventory())
	b := baseInventory()
	info := b.Sheet(collect.SInfo)
	info.Rows[0]["machineType"] = "e2-standard-4" // resize: relevant
	info.Rows[0]["lastStart"] = time.Now()        // volatile: ignored
	info.Rows[0]["costMonthly"] = 100.0           // same currency: compared
	info.Rows = without(info.Rows, "old")         // deleted
	info.Rows = append(info.Rows, model.Row{"project": "p1", "zone": "z", "name": "new", "externalIP": "34.1.2.3"})
	b.Sheet(collect.SFirewall).Rows[0]["openToInternet"] = true
	b.Sheet(collect.SFirewall).Rows[0]["sourceRanges"] = "0.0.0.0/0"
	b.Sheet(collect.SSnapshot).Rows[0]["name"] = "sched-2" // scheduled rotation: ignored
	b.Errors = append(b.Errors, model.ScanError{Project: "p2", Collector: "instances", Kind: model.ErrPermission})
	info.Rows = without(info.Rows, "db") // p2 instances unreadable in b: must not be a deletion
	b.Sheet(collect.SCost).Rows[0]["total"] = 100.0

	res := Compare(Input{a, "a.json"}, Input{b, "b.json"})

	added := rowsOf(res, SAdded)
	if len(added) != 1 || added[0]["resource"] != "new" || added[0]["relevant"] != true {
		t.Errorf("added = %v", added)
	}
	removed := rowsOf(res, SRemoved)
	if len(removed) != 1 || removed[0]["resource"] != "old" {
		t.Errorf("removed = %v (db in p2 must be skipped, not removed)", removed)
	}
	changes := map[string]model.Row{}
	for _, r := range rowsOf(res, SChanged) {
		changes[r["sheet"].(string)+"/"+r["field"].(string)] = r
	}
	if r := changes["gInfo/Machine type"]; r == nil || r["before"] != "e2-standard-2" || r["after"] != "e2-standard-4" || r["relevant"] != true {
		t.Errorf("machine type change = %v", r)
	}
	if _, ok := changes["gInfo/Último arranque"]; ok {
		t.Error("lastStart is volatile and must be ignored")
	}
	if r := changes["gInfo/Coste/mes actual"]; r == nil || r["after"] != "100" {
		t.Errorf("cost change = %v", r)
	}
	if r := changes["gFirewall/Abierta a Internet"]; r == nil || r["relevant"] != true {
		t.Errorf("firewall open change = %v", r)
	}
	if len(rowsOf(res, SChanged)) != len(changes) {
		t.Errorf("duplicate change rows: %v", rowsOf(res, SChanged))
	}
	if first := rowsOf(res, SChanged)[0]; first["relevant"] != true {
		t.Error("relevant changes must come first")
	}

	var meta []string
	for _, r := range rowsOf(res, SMeta) {
		meta = append(meta, r["property"].(string)+"="+r["value"].(string))
	}
	all := strings.Join(meta, "\n")
	for _, want := range []string{"No comparado: gInfo=p2", "Diferencia de coste/mes=+50.00 USD"} {
		if !strings.Contains(all, want) {
			t.Errorf("metadata missing %q:\n%s", want, all)
		}
	}
}

func TestCostsIgnoredWithDifferentCurrency(t *testing.T) {
	a, b := baseInventory(), baseInventory()
	b.Meta.Currency = "EUR"
	b.Sheet(collect.SInfo).Rows[0]["costMonthly"] = 46.0
	res := Compare(Input{a, "a"}, Input{b, "b"})
	if n := len(rowsOf(res, SChanged)); n != 0 {
		t.Errorf("money columns must be skipped across currencies: %v", rowsOf(res, SChanged))
	}
}

func without(rows []model.Row, name string) []model.Row {
	var out []model.Row
	for _, r := range rows {
		if r["name"] != name {
			out = append(out, r)
		}
	}
	return out
}
