package diff

import (
	"testing"

	"gcetools/internal/collect"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Titles are translated through c() and sheet(), which TestCoverage cannot
// see: check them here.
func TestSchemaTranslated(t *testing.T) {
	for _, s := range newResult().Sheets {
		for _, txt := range []string{s.Title, s.Description} {
			if !i18n.Has(txt) {
				t.Errorf("%s: no English for %q", s.Name, txt)
			}
		}
		for _, c := range s.Columns {
			if !i18n.Has(c.Title) {
				t.Errorf("%s.%s: no English for %q", s.Name, c.Key, c.Title)
			}
		}
	}
}

func TestEnglishReasons(t *testing.T) {
	i18n.SetLang("en")
	defer i18n.SetLang("es")
	a, b := baseInventory(), baseInventory()
	b.Sheet(collect.SInfo).Rows[0]["machineType"] = "e2-standard-4"
	b.Sheet(collect.SFirewall).Rows[0]["openToInternet"] = true
	b.Sheet(collect.SProject).Rows[1]["scanStatus"] = "Partial (2 errors)"
	// A Spanish scan compared with an English one: severities still match.
	b.Sheet(collect.SHealth).Add(model.Row{"severity": "High", "rule": "Orphaned disk", "project": "p1", "resource": "d"})
	res := Compare(Input{a, "a"}, Input{b, "b"})

	if s := res.Sheet(SChanged); s.Title != "Changes" || s.Columns[0].Title != "Relevant" {
		t.Errorf("English diff schema: %q %q", s.Title, s.Columns[0].Title)
	}
	reasons := map[string]bool{}
	for _, name := range []string{SAdded, SChanged} {
		for _, r := range rowsOf(res, name) {
			if s, _ := r["reason"].(string); s != "" {
				reasons[s] = true
			}
		}
	}
	for _, want := range []string{"VM resized", "Rule opened to the internet", "New high-severity finding"} {
		if !reasons[want] {
			t.Errorf("missing reason %q; got %v", want, reasons)
		}
	}
	for _, r := range rowsOf(res, SMeta) {
		if r["property"] == "Previous: date" {
			return
		}
	}
	t.Errorf("English metadata labels missing: %v", rowsOf(res, SMeta))
}
