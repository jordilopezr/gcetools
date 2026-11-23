// Package view prepares an inventory for the desktop UI without sending it
// whole: a summary (sheet list, row counts, headline figures) and then one
// sheet at a time. A 20,000-VM inventory is about 160 MB of JSON, which the
// Wails bridge copies several times and WebKit then has to parse.
package view

import (
	"gcetools/internal/history"
	"gcetools/internal/model"
	"gcetools/internal/schema"
)

// SheetInfo describes a sheet without its rows.
type SheetInfo struct {
	Name        string         `json:"name"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Columns     []model.Column `json:"columns"`
	Rows        int            `json:"rows"`
	// RowsUnmanaged is the row count once VMs created by GKE, Dataproc,
	// Dataflow or Batch are hidden (equal to Rows for sheets not per VM).
	RowsUnmanaged int `json:"rowsUnmanaged"`
}

// Stats are the headline figures of the inventory page.
type Stats struct {
	VMs      int     `json:"vms"`
	Running  int     `json:"running"`
	VCPU     float64 `json:"vcpu"`
	MemoryGB float64 `json:"memoryGB"`
	DiskGB   float64 `json:"diskGB"`
	Orphans  int     `json:"orphans"`
	High     int     `json:"high"`

	HasSecurity bool `json:"hasSecurity"`
	Exposed     int  `json:"exposed"`

	CostList float64 `json:"costList"` // set when meta.currency is

	HasCostReal      bool    `json:"hasCostReal"`
	CostReal         float64 `json:"costReal"`
	CostRealMonth    string  `json:"costRealMonth"`
	CostRealCurrency string  `json:"costRealCurrency"`

	HasSavings      bool    `json:"hasSavings"`
	Savings         float64 `json:"savings"`
	SavingsCurrency string  `json:"savingsCurrency"`
}

// Summary is what the inventory page needs before showing any sheet.
type Summary struct {
	Meta    model.Meta  `json:"meta"`
	Sheets  []SheetInfo `json:"sheets"`
	Stats   Stats       `json:"stats"`
	Managed int         `json:"managed"` // VMs created by a Google service
}

// serviceManaged are the services whose VMs can be hidden; MIG members are
// the user's own and stay visible.
var serviceManaged = map[string]bool{"GKE": true, "Dataproc": true, "Dataflow": true, "Batch": true}

// vmColumn is the column holding the VM name in each per-VM sheet.
var vmColumn = map[string]string{
	schema.SInfo: "name", schema.SCPU: "name", schema.SMemory: "name", schema.SDisk: "vm",
	schema.SNetwork: "vm", schema.SPerf: "vm", schema.STools: "vm", schema.SLicense: "vm",
	schema.SHealth: "resource", schema.SSecurity: "vm",
}

// order is the tab order of the inventory page; other sheets go last.
var order = []string{
	schema.SInfo, schema.SHealth, schema.SSecurity, schema.SCost, schema.SCostReal, schema.SRecomm,
	schema.SPerf, schema.STools, schema.SDisks, schema.SDisk, schema.SNetwork, schema.SSnapshot,
	schema.SImage, schema.SCPU, schema.SMemory, schema.SLicense, schema.SMIG, schema.STemplate,
	schema.SReserv, schema.SCommit, schema.SSole, schema.SQuota, schema.SVPC, schema.SSubnet,
	schema.SFirewall, schema.SAddress, schema.SProject, schema.SErrors, schema.SMeta,
}

// Summarize builds the summary of inv.
func Summarize(inv *model.Inventory) Summary {
	managed := managedVMs(inv)
	out := Summary{Meta: inv.Meta, Managed: len(managed)}
	for _, s := range sorted(inv) {
		info := SheetInfo{Name: s.Name, Title: s.Title, Description: s.Description, Columns: s.Columns,
			Rows: len(s.Rows), RowsUnmanaged: len(s.Rows)}
		if len(managed) > 0 && vmColumn[s.Name] != "" {
			info.RowsUnmanaged = len(filter(s, managed))
		}
		out.Sheets = append(out.Sheets, info)
	}

	h := history.Summarize(inv)
	st := Stats{VMs: h.VMs, Running: h.Running, VCPU: h.VCPU, MemoryGB: h.MemoryGB, DiskGB: h.DiskGB,
		Orphans: h.Orphans, High: h.High, CostList: h.CostList}
	if s := inv.Sheet(schema.SSecurity); s != nil {
		st.HasSecurity = true
		for _, r := range s.Rows {
			if r["internetExposed"] == true {
				st.Exposed++
			}
		}
	}
	if s := inv.Sheet(schema.SCostReal); s != nil && len(s.Rows) > 0 {
		st.HasCostReal, st.CostReal = true, h.CostReal
		st.CostRealMonth, _ = s.Rows[0]["month"].(string)
		st.CostRealCurrency, _ = s.Rows[0]["currency"].(string)
	}
	if s := inv.Sheet(schema.SRecomm); s != nil && len(s.Rows) > 0 {
		st.HasSavings, st.Savings, st.SavingsCurrency = true, h.Savings, "USD"
		for _, r := range s.Rows {
			if c, _ := r["currency"].(string); c != "" {
				st.SavingsCurrency = c
				break
			}
		}
	}
	out.Stats = st
	return out
}

// Rows returns the rows of one sheet, without the service-managed VMs when
// hideManaged is set. Nil when the sheet does not exist.
func Rows(inv *model.Inventory, name string, hideManaged bool) []model.Row {
	s := inv.Sheet(name)
	if s == nil {
		return nil
	}
	if hideManaged && vmColumn[name] != "" {
		if managed := managedVMs(inv); len(managed) > 0 {
			return filter(s, managed)
		}
	}
	if s.Rows == nil {
		return []model.Row{}
	}
	return s.Rows
}

func managedVMs(inv *model.Inventory) map[string]bool {
	out := map[string]bool{}
	if s := inv.Sheet(schema.SInfo); s != nil {
		for _, r := range s.Rows {
			if by, _ := r["managedBy"].(string); serviceManaged[by] {
				out[key(r, "name")] = true
			}
		}
	}
	return out
}

func filter(s *model.Sheet, managed map[string]bool) []model.Row {
	col := vmColumn[s.Name]
	out := make([]model.Row, 0, len(s.Rows))
	for _, r := range s.Rows {
		if !managed[key(r, col)] {
			out = append(out, r)
		}
	}
	return out
}

func key(r model.Row, col string) string {
	p, _ := r["project"].(string)
	n, _ := r[col].(string)
	return p + "/" + n
}

func sorted(inv *model.Inventory) []*model.Sheet {
	rank := map[string]int{}
	for i, n := range order {
		rank[n] = i + 1
	}
	out := append([]*model.Sheet(nil), inv.Sheets...)
	pos := func(s *model.Sheet) int {
		if r := rank[s.Name]; r > 0 {
			return r
		}
		return len(order) + 1
	}
	for i := 1; i < len(out); i++ { // stable insertion sort: a few dozen sheets
		for j := i; j > 0 && pos(out[j]) < pos(out[j-1]); j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
