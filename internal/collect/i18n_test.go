package collect

import (
	"strings"
	"testing"

	"gcetools/internal/i18n"
)

// neutralTitles are column/sheet titles written the same way in English
// (GCP terms, units, acronyms), so they need no dictionary entry.
var neutralTitles = map[string]bool{
	"VM": true, "ID": true, "Machine type": true, "vCPU": true, "RAM (GB)": true, "CPU platform": true,
	"Auto restart": true, "VPC": true, "GPUs": true, "Service account": true, "Scopes": true, "Labels": true,
	"Network tags": true, "Deletion protection": true, "Secure Boot": true, "vTPM": true, "Confidential": true,
	"MIG": true, "Hostname": true, "Self link": true, "Threads/core": true, "Min CPU platform": true,
	"Custom": true, "Shared core": true, "Nested virt.": true, "GB/vCPU": true, "Extended memory": true,
	"Device": true, "Boot": true, "Auto delete": true, "NIC": true, "Alias IP": true, "Tier": true,
	"Stack": true, "IPv6": true, "IP forwarding": true, "VMs": true, "Snapshot schedule": true,
	"Snapshot": true, "Template": true, "Target size": true, "Autoscaling": true, "Health check": true,
	"Update policy": true, "Routing": true, "MTU": true, "Peerings": true, "CIDR": true, "Gateway": true,
	"Private Google Access": true, "Flow logs": true, "Purpose": true, "Logging": true, "IP": true,
	"Local SSD (GB)": true, "Commitment": true, "Plan": true, "Node group": true, "Node template": true,
	"CPU overcommit": true, "Kernel": true, "Ops Agent": true, "Compute+GPU": true, "Snapshots": true,
	"CPU": true, "CPU p95 (%)": true, "RAM p95 (%)": true, "NICs": true, "MIGs": true, "Templates": true,
	"Instance templates": true, "VPCs": true, "Firewall": true, "IPs": true, "Commitments": true,
	"Sole-tenant": true,
}

func TestSchemaTranslated(t *testing.T) {
	for _, s := range NewInventory().Sheets {
		for _, txt := range []string{s.Title, s.Description} {
			if !i18n.Has(txt) && !neutralTitles[txt] {
				t.Errorf("%s: no English for %q", s.Name, txt)
			}
		}
		for _, c := range s.Columns {
			if !i18n.Has(c.Title) && !neutralTitles[c.Title] {
				t.Errorf("%s.%s: no English for %q", s.Name, c.Key, c.Title)
			}
		}
	}
	for _, v := range []string{"Herramienta", "Otra"} { // licenseCategory values written to rows
		if !i18n.Has(v) {
			t.Errorf("no English for license category %q", v)
		}
	}
}

func TestEnglishInventory(t *testing.T) {
	es := NewInventory()
	i18n.SetLang("en")
	defer i18n.SetLang("es")

	inv := NewInventory()
	info := inv.Sheet(SInfo)
	if info.Title != "Instances" || info.Columns[0].Title != "Project" || inv.Sheet(SDisks).Title != "All disks" ||
		inv.Sheet(SHealth).Title != "Health" || !strings.Contains(info.Description, "equivalent to vInfo") {
		t.Errorf("English schema: %q %q %q", info.Title, info.Columns[0].Title, info.Description)
	}
	Relabel(es)
	if es.Sheet(SInfo).Title != "Instances" || es.Sheet(SInfo).Columns[0].Title != "Project" {
		t.Error("Relabel must bring a Spanish inventory to English")
	}
	// core counts successful projects with strings.HasPrefix(status, "OK").
	if s := i18n.Tf("OK (sin %s)", "osconfig"); !strings.HasPrefix(s, "OK") || s == "OK (sin osconfig)" {
		t.Errorf("English OK status = %q", s)
	}
	if i18n.T(licenseCategory("virtual-disk-import")) != "Tool" || licenseCategory("virtual-disk-import") != "Herramienta" {
		t.Error("license category: internal value must stay, rows get English")
	}
}
