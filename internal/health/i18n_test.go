package health

import (
	"gcetools/internal/schema"
	"strings"
	"testing"
	"time"

	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Rules keeps the Spanish source texts and LocalizedRules translates them
// through variables, which TestCoverage cannot see: check them here.
func TestRulesTranslated(t *testing.T) {
	texts := []string{High, Medium, Low, onVM, onDisk, onTemplate, onProject}
	for _, r := range Rules {
		texts = append(texts, r.Title, r.Category, r.Severity, r.Labels)
	}
	for _, s := range texts {
		if !i18n.Has(s) {
			t.Errorf("no English for %q", s)
		}
	}
}

func TestLevel(t *testing.T) {
	defer i18n.SetLang("es")
	for _, lang := range []string{"es", "en"} {
		i18n.SetLang(lang)
		for _, l := range []string{High, Medium, Low} {
			if Level(i18n.T(l)) != l {
				t.Errorf("%s: Level(%q) != %q", lang, i18n.T(l), l)
			}
		}
	}
	if Level("Critical") != "" {
		t.Error("unknown severity must map to empty")
	}
}

func TestEnglishFindings(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("es")
	inv := schema.NewInventory()
	inv.Sheet(schema.SDisks).Add(model.Row{"project": "p", "name": "d1", "location": "z", "orphan": true, "sizeGB": 10.0, "type": "pd-ssd"})
	inv.Sheet(schema.SInfo).Add(model.Row{"project": "p", "name": "vm", "zone": "z", "status": "RUNNING", "secureBoot": true,
		"labels": "a=b", "deletionProtection": true, "externalIP": "34.1.2.3"})
	Evaluate(inv, DefaultOptions(), time.Now())
	rows := findings(inv)
	if len(rows) == 0 {
		t.Fatal("no findings")
	}
	first := rows[0] // sorted by internal severity: High first
	if first["severity"] != "High" || first["category"] != "Cost" || first["rule"] != "Orphaned disk" ||
		first["detail"] != "10 GB pd-ssd with no VM" {
		t.Errorf("English finding = %v", first)
	}
	r, ok := has(rows, "VM with a public IP", "vm")
	if !ok || r["severity"] != "Medium" || r["category"] != "Security" {
		t.Errorf("public-ip finding = %v", r)
	}
	for _, row := range rows {
		for _, k := range []string{"severity", "category", "rule", "detail"} {
			if s, _ := row[k].(string); strings.ContainsAny(s, "áéíóúñ") {
				t.Errorf("Spanish text left in %s: %q", k, s)
			}
		}
	}
	if lr := LocalizedRules(); lr[0].Title != "Orphaned disk" || lr[0].Severity != "High" || Rules[0].Title != "Disco huérfano" {
		t.Errorf("LocalizedRules = %v", lr[0])
	}
}
