// Package health derives best-practice findings (the gHealth sheet) from
// the collected sheets. Rules only read rows, so they are easy to unit-test
// and to extend.
package health

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"gcetools/internal/collect"
	"gcetools/internal/model"
)

type Options struct {
	StoppedDays  int `json:"stoppedDays"`  // VM stopped longer than this
	SnapshotDays int `json:"snapshotDays"` // snapshot older than this
}

func DefaultOptions() Options { return Options{StoppedDays: 30, SnapshotDays: 90} }

const (
	High   = "Alta"
	Medium = "Media"
	Low    = "Baja"
)

type finding struct{ severity, category, rule, project, resource, location, detail string }

func Evaluate(inv *model.Inventory, opt Options, now time.Time) {
	out := inv.Sheet(collect.SHealth)
	if out == nil {
		return
	}
	var fs []finding
	add := func(f finding) { fs = append(fs, f) }

	rows := func(name string) []model.Row {
		if s := inv.Sheet(name); s != nil {
			return s.Rows
		}
		return nil
	}

	for _, r := range rows(collect.SInfo) {
		p, n, z := str(r, "project"), str(r, "name"), str(r, "zone")
		if str(r, "status") == "TERMINATED" {
			if t, ok := r["lastStop"].(time.Time); ok && now.Sub(t) > time.Duration(opt.StoppedDays)*24*time.Hour {
				detail := fmt.Sprintf("Parada desde %s; sus discos siguen facturando", t.Format("2006-01-02"))
				if c, ok := r["costDisks"].(float64); ok && c > 0 {
					detail += fmt.Sprintf(" (~%.2f %s/mes)", c, inv.Meta.Currency)
				}
				add(finding{Medium, "Coste", fmt.Sprintf("VM parada más de %d días", opt.StoppedDays), p, n, z, detail})
			}
		}
		if sa := str(r, "serviceAccount"); strings.HasSuffix(sa, "-compute@developer.gserviceaccount.com") {
			sev := Medium
			if strings.Contains(str(r, "scopes"), "cloud-platform") {
				sev = High
			}
			add(finding{sev, "Seguridad", "Usa la service account por defecto de Compute", p, n, z,
				"SA: " + sa + "; scopes: " + str(r, "scopes")})
		}
		if str(r, "externalIP") != "" {
			add(finding{Medium, "Seguridad", "VM con IP pública", p, n, z, str(r, "externalIP")})
		}
		if !boolean(r, "secureBoot") {
			add(finding{Low, "Seguridad", "Shielded VM Secure Boot deshabilitado", p, n, z, ""})
		}
		if str(r, "labels") == "" {
			add(finding{Low, "Gobierno", "VM sin labels", p, n, z, "Dificulta el reparto de costes"})
		}
		if !boolean(r, "deletionProtection") && str(r, "mig") == "" {
			add(finding{Low, "Operación", "Deletion protection deshabilitada", p, n, z, ""})
		}
		if f := str(r, "family"); f == "n1" || f == "custom" {
			add(finding{Low, "Coste", "Familia de máquina de generación antigua (N1)", p, n, z,
				"Valorar N2/N2D/N4/C3 (mejor precio/rendimiento)"})
		}
	}

	for _, r := range rows(collect.SDisk) {
		if boolean(r, "boot") && str(r, "type") == "pd-standard" {
			add(finding{Low, "Rendimiento", "Disco de arranque pd-standard", str(r, "project"), str(r, "disk"),
				str(r, "zone"), "VM " + str(r, "vm") + "; valorar pd-balanced"})
		}
	}

	for _, r := range rows(collect.SDisks) {
		if boolean(r, "orphan") {
			detail := fmt.Sprintf("%.0f GB %s sin VM", num(r, "sizeGB"), str(r, "type"))
			if t, ok := r["lastDetach"].(time.Time); ok {
				detail += "; último detach " + t.Format("2006-01-02")
			}
			if c, ok := r["costMonthly"].(float64); ok {
				detail += fmt.Sprintf("; ~%.2f %s/mes", c, inv.Meta.Currency)
			}
			add(finding{High, "Coste", "Disco huérfano", str(r, "project"), str(r, "name"), str(r, "location"), detail})
		}
	}

	for _, r := range rows(collect.SAddress) {
		if str(r, "status") == "RESERVED" && str(r, "type") == "EXTERNAL" {
			add(finding{Medium, "Coste", "IP externa reservada sin uso", str(r, "project"), str(r, "name"),
				str(r, "location"), str(r, "address")})
		}
	}

	// Old manual snapshots are aggregated per project: backup tools create
	// thousands of them and one row each would drown every other finding.
	type snapAgg struct {
		count  int
		gb     float64
		oldest int64
	}
	snaps := map[string]*snapAgg{}
	var snapOrder []string
	for _, r := range rows(collect.SSnapshot) {
		age, ok := r["ageDays"].(int64)
		if !ok || age <= int64(opt.SnapshotDays) || boolean(r, "autoCreated") {
			continue
		}
		p := str(r, "project")
		a := snaps[p]
		if a == nil {
			a = &snapAgg{}
			snaps[p] = a
			snapOrder = append(snapOrder, p)
		}
		a.count++
		a.gb += num(r, "storageGB")
		a.oldest = max(a.oldest, age)
	}
	for _, p := range snapOrder {
		a := snaps[p]
		add(finding{Low, "Coste", fmt.Sprintf("Snapshots manuales de más de %d días", opt.SnapshotDays), p,
			fmt.Sprintf("%d snapshots", a.count), "",
			fmt.Sprintf("%.0f GB almacenados; el más antiguo tiene %d días (detalle en gSnapshot)", a.gb, a.oldest)})
	}

	// VMs stopped with no disk attached are usually leftovers of migrations.
	for _, r := range rows(collect.SInfo) {
		if str(r, "status") == "TERMINATED" && num(r, "disks") == 0 {
			add(finding{Low, "Gobierno", "VM parada sin discos", str(r, "project"), str(r, "name"), str(r, "zone"),
				"Posible resto de una migración; revisar si sus discos aparecen como huérfanos"})
		}
	}

	for _, r := range rows(collect.SQuota) {
		if pct := num(r, "usagePct"); pct >= 80 {
			sev := Medium
			if pct >= 95 {
				sev = High
			}
			add(finding{sev, "Capacidad", "Cuota regional casi agotada", str(r, "project"), str(r, "metric"),
				str(r, "region"), fmt.Sprintf("%.0f de %.0f (%.0f%%)", num(r, "usage"), num(r, "limit"), pct)})
		}
	}

	for _, r := range rows(collect.SReserv) {
		if num(r, "unusedPct") > 0 && str(r, "status") == "READY" {
			add(finding{Medium, "Coste", "Reserva con capacidad sin usar", str(r, "project"), str(r, "name"), str(r, "zone"),
				fmt.Sprintf("%.0f de %.0f %s sin usar: se facturan igualmente", num(r, "count")-num(r, "inUse"),
					num(r, "count"), str(r, "machineType"))})
		}
	}

	for _, r := range rows(collect.SCommit) {
		left, ok := r["daysLeft"].(int64)
		if ok && str(r, "status") == "ACTIVE" && left >= 0 && left <= 60 && !boolean(r, "autoRenew") {
			add(finding{Medium, "Coste", "Commitment (CUD) vence en menos de 60 días", str(r, "project"), str(r, "name"),
				str(r, "region"), fmt.Sprintf("Vence en %d días y no se renueva automáticamente", left)})
		}
	}

	for _, r := range rows(collect.SPerf) {
		if str(r, "status") != "RUNNING" || num(r, "samples") < 24*7 {
			continue // need at least a week of data to judge
		}
		p, n, z := str(r, "project"), str(r, "vm"), str(r, "zone")
		if cpu := num(r, "cpuP95"); cpu < 10 {
			add(finding{Medium, "Coste", "VM infrautilizada (CPU p95 < 10%)", p, n, z,
				fmt.Sprintf("CPU p95 %.1f%% en %d días con %d vCPU (%s)", cpu, int(num(r, "days")), int(num(r, "vcpu")), str(r, "machineType"))})
		} else if cpu > 90 {
			add(finding{Medium, "Rendimiento", "CPU saturada (p95 > 90%)", p, n, z, fmt.Sprintf("CPU p95 %.1f%%", cpu)})
		}
		if mem, ok := r["memP95"].(float64); ok && mem > 90 {
			add(finding{Medium, "Rendimiento", "Memoria saturada (p95 > 90%)", p, n, z, fmt.Sprintf("RAM p95 %.1f%%", mem)})
		}
		if !boolean(r, "opsAgent") {
			add(finding{Low, "Operación", "Sin Ops Agent: no hay métricas de memoria ni disco", p, n, z, ""})
		}
	}

	for _, r := range rows(collect.SFirewall) {
		if !boolean(r, "openToInternet") {
			continue
		}
		rules := str(r, "rules")
		sev, rule := Medium, "Regla de firewall abierta a Internet"
		if rules == "all" || strings.Contains(rules, ":22") || strings.Contains(rules, "3389") ||
			rules == "tcp" || strings.Contains(rules, "tcp:0-65535") {
			sev, rule = High, "SSH/RDP o todos los puertos abiertos a Internet"
		}
		add(finding{sev, "Seguridad", rule, str(r, "project"), str(r, "name"), str(r, "network"),
			rules + " desde " + str(r, "sourceRanges") + " hacia " + str(r, "targets")})
	}

	order := map[string]int{High: 0, Medium: 1, Low: 2}
	sortFindings(fs, order)
	for _, f := range fs {
		out.Add(model.Row{
			"severity": f.severity, "category": f.category, "rule": f.rule, "project": f.project,
			"resource": f.resource, "location": f.location, "detail": f.detail,
		})
	}
}

func sortFindings(fs []finding, order map[string]int) {
	// stable: keeps collection order inside a severity
	slices.SortStableFunc(fs, func(a, b finding) int { return order[a.severity] - order[b.severity] })
}

func str(r model.Row, k string) string {
	s, _ := r[k].(string)
	return s
}

func boolean(r model.Row, k string) bool {
	b, _ := r[k].(bool)
	return b
}

func num(r model.Row, k string) float64 {
	switch v := r[k].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	return 0
}
