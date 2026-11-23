package view

import (
	"testing"

	"gcetools/internal/collect"
	"gcetools/internal/model"
)

func sample() *model.Inventory {
	inv := collect.NewInventory()
	inv.Meta.Currency = "USD"
	add := func(s string, r model.Row) { inv.Sheet(s).Add(r) }
	add(collect.SInfo, model.Row{"project": "p", "name": "web", "status": "RUNNING", "vcpu": int64(2), "memoryGB": 8.0})
	add(collect.SInfo, model.Row{"project": "p", "name": "node", "status": "RUNNING", "vcpu": int64(4), "memoryGB": 16.0, "managedBy": "GKE"})
	add(collect.SInfo, model.Row{"project": "p", "name": "mig-1", "status": "TERMINATED", "managedBy": "MIG"})
	add(collect.SHealth, model.Row{"project": "p", "resource": "node", "severity": "Alta"})
	add(collect.SHealth, model.Row{"project": "p", "resource": "web", "severity": "Baja"})
	add(collect.SSecurity, model.Row{"project": "p", "vm": "web", "internetExposed": true})
	add(collect.SDisks, model.Row{"project": "p", "name": "d", "sizeGB": 100.0, "orphan": true})
	add(collect.SCost, model.Row{"project": "p", "total": 12.5})
	add(collect.SRecomm, model.Row{"savingsMonthly": 3.0, "currency": "EUR"})
	return inv
}

func TestSummarize(t *testing.T) {
	s := Summarize(sample())
	if s.Managed != 1 {
		t.Errorf("managed = %d", s.Managed)
	}
	if s.Sheets[0].Name != collect.SInfo || s.Sheets[1].Name != collect.SHealth || s.Sheets[2].Name != collect.SSecurity {
		t.Errorf("order = %s %s %s", s.Sheets[0].Name, s.Sheets[1].Name, s.Sheets[2].Name)
	}
	for _, sh := range s.Sheets {
		if sh.Name == collect.SInfo && (sh.Rows != 3 || sh.RowsUnmanaged != 2) {
			t.Errorf("gInfo counts = %d/%d", sh.Rows, sh.RowsUnmanaged)
		}
		if sh.Name == collect.SDisks && sh.RowsUnmanaged != 1 {
			t.Errorf("gDisks is not per VM: %d", sh.RowsUnmanaged)
		}
	}
	st := s.Stats
	if st.VMs != 3 || st.Running != 2 || st.VCPU != 6 || st.High != 1 || !st.HasSecurity || st.Exposed != 1 ||
		st.Orphans != 1 || st.CostList != 12.5 || !st.HasSavings || st.SavingsCurrency != "EUR" || st.HasCostReal {
		t.Errorf("stats = %+v", st)
	}
}

func TestRows(t *testing.T) {
	inv := sample()
	if got := Rows(inv, collect.SHealth, true); len(got) != 1 || got[0]["resource"] != "web" {
		t.Errorf("gHealth without managed = %v", got)
	}
	if got := Rows(inv, collect.SHealth, false); len(got) != 2 {
		t.Errorf("gHealth = %v", got)
	}
	if got := Rows(inv, collect.SVPC, false); got == nil || len(got) != 0 {
		t.Errorf("empty sheet must be an empty list, got %#v", got)
	}
	if Rows(inv, "nope", false) != nil {
		t.Error("unknown sheet")
	}
}
