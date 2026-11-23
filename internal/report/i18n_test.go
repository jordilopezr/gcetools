package report

import (
	"gcetools/internal/schema"
	"regexp"
	"strings"
	"testing"
	"time"

	"gcetools/internal/diff"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

var reTemplateText = regexp.MustCompile(`\{\{\s*tf?\s+"((?:[^"\\]|\\.)*)"`)

// TestTemplateTranslated checks every {{t "…"}} / {{tf "…"}} literal of the
// template has an English entry (TestCoverage only reads Go files).
func TestTemplateTranslated(t *testing.T) {
	b, err := files.ReadFile("report.html.tmpl")
	if err != nil {
		t.Fatal(err)
	}
	m := reTemplateText.FindAllStringSubmatch(string(b), -1)
	if len(m) < 20 {
		t.Fatalf("found only %d template texts", len(m))
	}
	for _, s := range m {
		if !i18n.Has(s[1]) {
			t.Errorf("template text without English: %q", s[1])
		}
	}
}

func TestRenderEnglish(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("es")
	inv := sample() // health rows keep the Spanish severities of an older scan
	inv.Sheet(schema.SHealth).Add(model.Row{"severity": "High", "rule": "Orphaned disk", "project": "erp", "resource": "d2"})
	inv.Sheet(schema.SCostReal).Add(model.Row{"project": "TOTAL", "net": 1500.25, "currency": "USD", "month": "202610"})
	a := sample()
	b := sample()
	b.Sheet(schema.SInfo).Rows[0]["machineType"] = "n2-standard-16"
	d := diff.Compare(diff.Input{Inv: a, Label: "a"}, diff.Input{Inv: b, Label: "b"})
	out, err := Render(inv, Options{Diff: d, DiffSince: a.Meta.StartedAt, Now: time.Date(2026, 11, 19, 8, 0, 0, 0, time.Local)})
	if err != nil {
		t.Fatal(err)
	}
	html := string(out)
	for _, want := range []string{
		`<html lang="en">`, "Compute Engine inventory report", "Scan of Nov 18, 2026 09:30 · 2 of 3 projects read",
		"Generated on Nov 19, 2026 08:00", "1 running", "Orphaned disks", "2,048 GB with no VM", "1,235 USD",
		"Actual cost October 2026", "1,500 USD", "Potential savings/month", "Top findings", "2 of 3 findings (2 high, 0 medium, 1 low)",
		`class="sev Alta">Alta<`, `class="sev Alta">High<`, "Changes since the scan of Nov 18, 2026", "VM resized",
		"Monthly cost by project", "Generated with gceTools 0.6.0",
	} {
		if !strings.Contains(html, want) {
			t.Errorf("English report lacks %q", want)
		}
	}
	for _, spanish := range []string{
		"Informe", "Escaneo del", "encendidas", "Discos huérfanos", "sin VM", "Hallazgos", "Coste", "Proyecto",
		"Generado", "Cambios desde", "Cambio de tamaño", "octubre", "Ahorro", "según", "medios", "bajos", "/mes", "leídos",
	} {
		if strings.Contains(html, spanish) {
			t.Errorf("English report still contains %q", spanish)
		}
	}
}
