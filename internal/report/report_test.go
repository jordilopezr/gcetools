package report

import (
	"gcetools/internal/schema"
	"strings"
	"testing"
	"time"

	"gcetools/internal/diff"
	"gcetools/internal/model"
)

func sample() *model.Inventory {
	inv := schema.NewInventory()
	inv.Meta = model.Meta{Tool: "gceTools", Version: "0.6.0", Identity: "ana@example.com", Projects: 3, ProjectsOK: 2,
		Currency: "USD", StartedAt: time.Date(2026, 11, 18, 9, 30, 0, 0, time.Local)}
	add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
	add(schema.SInfo, model.Row{"project": "erp", "name": "app", "status": "RUNNING", "vcpu": int64(8), "memoryGB": 32.0})
	add(schema.SInfo, model.Row{"project": "erp", "name": "old", "status": "TERMINATED", "vcpu": int64(2), "memoryGB": 8.0})
	add(schema.SDisks, model.Row{"project": "erp", "name": "d1", "sizeGB": 2048.0, "orphan": true})
	add(schema.SCost, model.Row{"project": "erp", "total": 1234.5, "vmsRunning": int64(1)})
	add(schema.SCost, model.Row{"project": "TOTAL", "total": 1234.5})
	add(schema.SHealth, model.Row{"severity": "Alta", "rule": "Disco huérfano", "project": "erp", "resource": "d1", "detail": "<script>alert(1)</script>"})
	add(schema.SHealth, model.Row{"severity": "Baja", "rule": "VM sin labels", "project": "erp", "resource": "app"})
	add(schema.SProject, model.Row{"projectId": "erp", "scanStatus": "OK"})
	add(schema.SProject, model.Row{"projectId": "qa", "scanStatus": "Sin permisos"})
	add(schema.SRecomm, model.Row{"savingsMonthly": 99.9, "currency": "USD"})
	return inv
}

func TestRender(t *testing.T) {
	out, err := Render(sample(), Options{Website: "https://gcetools.jordilopezr.com", Now: time.Date(2026, 11, 19, 8, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	for _, want := range []string{
		"Escaneo del 18/11/2026 09:30", "2 de 3 proyectos", "1 encendidas", "Discos huérfanos", "2.048 GB sin VM",
		"1.235 USD", "Ahorro posible/mes", "100 USD", "Disco huérfano", "qa: Sin permisos", "gceTools 0.6.0",
		"1 de 2 hallazgos",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("report lacks %q", want)
		}
	}
	if strings.Contains(html, "<script>alert") {
		t.Error("finding detail must be HTML-escaped")
	}
	if strings.Contains(html, "VM sin labels") {
		t.Error("low findings must not be in the top list")
	}
	if strings.Contains(html, "<script src") || strings.Contains(html, "http://") {
		t.Error("report must be self-contained")
	}
}

func TestRenderWithDiff(t *testing.T) {
	a, b := sample(), sample()
	b.Sheet(schema.SInfo).Rows[0]["machineType"] = "n2-standard-16"
	a.Sheet(schema.SInfo).Rows[0]["machineType"] = "n2-standard-8"
	d := diff.Compare(diff.Input{Inv: a, Label: "a"}, diff.Input{Inv: b, Label: "b"})
	out, err := Render(b, Options{Diff: d, DiffSince: a.Meta.StartedAt})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	for _, want := range []string{"Cambios desde el escaneo del 18/11/2026", "1 modificados", "Cambio de tamaño de la VM", "n2-standard-8 → n2-standard-16"} {
		if !strings.Contains(html, want) {
			t.Errorf("diff section lacks %q", want)
		}
	}
}

func TestGroup(t *testing.T) {
	for _, c := range []struct {
		v    float64
		d    int
		want string
	}{{0, 0, "0"}, {1234567.891, 0, "1.234.568"}, {1234.5, 1, "1.234,5"}, {-12.345, 2, "-12,35"}, {999, 0, "999"}, {0.004, 2, "0,00"}} {
		if got := group(c.v, c.d); got != c.want {
			t.Errorf("group(%v, %d) = %q, want %q", c.v, c.d, got, c.want)
		}
	}
}

// With a single project gCost has no TOTAL row: the report sums projects.
func TestSingleProjectCost(t *testing.T) {
	inv := sample()
	s := inv.Sheet(schema.SCost)
	s.Rows = s.Rows[:1] // drop TOTAL
	out, err := Render(inv, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "1.235 USD") {
		t.Errorf("estimated cost missing from the report")
	}
}
