package health

import (
	"testing"
	"time"

	"gcetools/internal/collect"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// The firewall-open severity parses the ports instead of matching text.
func TestFirewallOpenSeverity(t *testing.T) {
	cases := map[string]string{
		"tcp:80,443,22": High,
		"tcp:20-30":     High,
		"tcp:1-65535":   High,
		"all":           High,
		"tcp":           High,
		"tcp:3389":      High,
		"tcp:2222":      Medium,
		"udp:3389":      Medium,
		"tcp:443":       Medium,
	}
	for rules, want := range cases {
		inv := collect.NewInventory()
		inv.Sheet(collect.SFirewall).Add(model.Row{"project": "p", "name": "fw", "network": "n", "rules": rules,
			"openToInternet": true, "sourceRanges": "0.0.0.0/0"})
		Evaluate(inv, DefaultOptions(), time.Now())
		var r model.Row
		for _, f := range findings(inv) {
			if f["ruleId"] == "firewall-open" {
				r = f
			}
		}
		if r == nil {
			t.Fatalf("%s: no finding", rules)
		}
		if r["severity"] != i18n.T(want) {
			t.Errorf("%s: severity %v, want %s", rules, r["severity"], want)
		}
	}
}
