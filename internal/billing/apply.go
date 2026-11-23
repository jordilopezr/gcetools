package billing

import (
	"gcetools/internal/schema"
	"math"
	"sort"
	"strings"

	"gcetools/internal/model"
)

// Result tells how much of the real cost could be tied to inventory rows.
type Result struct {
	Net        float64 // total net Compute Engine cost of the month
	Attributed float64 // part matched to a VM or disk of the inventory
}

// Apply fills costReal on gInfo and gDisks (detailed export only) and builds
// gCostReal: per project, list estimate vs real gross, credits and net.
func Apply(inv *model.Inventory, d *Data) Result {
	type key struct{ project, kind, ref string }
	byRes := map[key]float64{}
	type agg struct{ gross, credits, attributed float64 }
	perProject := map[string]*agg{}
	var res Result
	for _, l := range d.Lines {
		a := perProject[l.Project]
		if a == nil {
			a = &agg{}
			perProject[l.Project] = a
		}
		a.gross += l.Cost
		a.credits += l.Credits
		res.Net += l.Net()
		if kind, id := resourceRef(l.GlobalName); kind != "" {
			byRes[key{l.Project, kind, id}] += l.Net()
			if l.Name != "" && l.Name != id {
				byRes[key{l.Project, kind + "-name", l.Name}] += l.Net()
			}
		}
	}

	attribute := func(project string, v float64) {
		if a := perProject[project]; a != nil {
			a.attributed += v
		}
		res.Attributed += v
	}
	if d.Detailed {
		if s := inv.Sheet(schema.SInfo); s != nil {
			for _, r := range s.Rows {
				p, _ := r["project"].(string)
				id, _ := r["id"].(string)
				name, _ := r["name"].(string)
				v, ok := byRes[key{p, "instances", id}]
				if !ok {
					v, ok = byRes[key{p, "instances-name", name}]
				}
				if ok {
					r["costReal"] = round2(v)
					attribute(p, v)
				}
			}
		}
		if s := inv.Sheet(schema.SDisks); s != nil {
			for _, r := range s.Rows {
				p, _ := r["project"].(string)
				name, _ := r["name"].(string)
				if v, ok := byRes[key{p, "disks-name", name}]; ok {
					r["costReal"] = round2(v)
					attribute(p, v)
				} else if v, ok := byRes[key{p, "disks", name}]; ok {
					r["costReal"] = round2(v)
					attribute(p, v)
				}
			}
		}
	}

	estimate := map[string]float64{}
	sameCurrency := inv.Meta.Currency != "" && strings.EqualFold(inv.Meta.Currency, d.Currency)
	if s := inv.Sheet(schema.SCost); s != nil && sameCurrency {
		for _, r := range s.Rows {
			p, _ := r["project"].(string)
			if t, ok := r["total"].(float64); ok {
				estimate[p] = t
			}
		}
	}
	out := inv.Sheet(schema.SCostReal)
	if out == nil {
		return res
	}
	projects := make([]string, 0, len(perProject))
	for p := range perProject {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	var tot agg
	var totEst float64
	row := func(p string, a agg, est float64, hasEst bool) model.Row {
		r := model.Row{"project": p, "month": d.Month, "gross": round2(a.gross), "credits": round2(a.credits),
			"net": round2(a.gross + a.credits), "currency": d.Currency}
		if d.Detailed {
			r["attributed"] = round2(a.attributed)
			r["unattributed"] = round2(a.gross + a.credits - a.attributed)
		}
		if hasEst {
			r["estimate"] = round2(est)
			r["diff"] = round2(a.gross + a.credits - est)
			if est > 0 {
				r["diffPct"] = math.Round((a.gross+a.credits-est)/est*1000) / 10
			}
		}
		return r
	}
	for _, p := range projects {
		a := *perProject[p]
		est, hasEst := estimate[p]
		out.Add(row(p, a, est, hasEst))
		tot.gross += a.gross
		tot.credits += a.credits
		tot.attributed += a.attributed
		totEst += est
	}
	if len(projects) > 0 {
		out.Add(row("TOTAL", tot, totEst, len(estimate) > 0))
	}
	return res
}

// resourceRef extracts ("instances"|"disks", id-or-name) from a Compute
// global name: //compute.googleapis.com/projects/p/zones/z/instances/123.
func resourceRef(global string) (kind, id string) {
	parts := strings.Split(global, "/")
	if len(parts) < 2 {
		return "", ""
	}
	kind = parts[len(parts)-2]
	if kind != "instances" && kind != "disks" {
		return "", ""
	}
	return kind, parts[len(parts)-1]
}

func round2(f float64) float64 { return math.Round(f*100) / 100 }
