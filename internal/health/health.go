// Package health derives best-practice findings (the gHealth sheet) from
// the collected sheets. Rules only read rows, so they are easy to unit-test
// and to extend.
//
// Every rule has a stable ID. A rule can be switched off for a whole scan
// (Options.Disabled) or for single resources with labels on the VM, disk,
// template or project:
//
//	gcetools-ignore=<rule-id>        one rule
//	gcetools-ignore-<rule-id>=true   one rule (lets you stack several)
//	gcetools-ignore=all              every rule
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
	StoppedDays  int      `json:"stoppedDays"`  // VM stopped longer than this
	SnapshotDays int      `json:"snapshotDays"` // manual snapshot older than this
	ImageDays    int      `json:"imageDays"`    // custom image used by a template older than this
	CPULowPct    float64  `json:"cpuLowPct"`    // CPU p95 below this = underused
	CPUHighPct   float64  `json:"cpuHighPct"`   // CPU p95 above this = saturated
	MemHighPct   float64  `json:"memHighPct"`   // RAM p95 above this = saturated
	QuotaPct     float64  `json:"quotaPct"`     // regional quota usage that raises a finding
	CUDDays      int      `json:"cudDays"`      // commitment ending within this many days
	Disabled     []string `json:"disabled"`     // rule IDs switched off
}

func DefaultOptions() Options {
	return Options{StoppedDays: 30, SnapshotDays: 90, ImageDays: 180, CPULowPct: 10, CPUHighPct: 90,
		MemHighPct: 90, QuotaPct: 80, CUDDays: 60, Disabled: []string{}}
}

// withDefaults fills zero values, so options saved by older versions (or
// partially filled by the CLI) keep working.
func (o Options) withDefaults() Options {
	d := DefaultOptions()
	if o.StoppedDays <= 0 {
		o.StoppedDays = d.StoppedDays
	}
	if o.SnapshotDays <= 0 {
		o.SnapshotDays = d.SnapshotDays
	}
	if o.ImageDays <= 0 {
		o.ImageDays = d.ImageDays
	}
	if o.CPULowPct <= 0 {
		o.CPULowPct = d.CPULowPct
	}
	if o.CPUHighPct <= 0 {
		o.CPUHighPct = d.CPUHighPct
	}
	if o.MemHighPct <= 0 {
		o.MemHighPct = d.MemHighPct
	}
	if o.QuotaPct <= 0 {
		o.QuotaPct = d.QuotaPct
	}
	if o.CUDDays <= 0 {
		o.CUDDays = d.CUDDays
	}
	return o
}

const (
	High   = "Alta"
	Medium = "Media"
	Low    = "Baja"
)

// Rule describes one check for the UI and the docs.
type Rule struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
	Severity string `json:"severity"`        // usual severity; some rules escalate
	Param    string `json:"param,omitempty"` // Options field (JSON name) the rule uses
	Labels   string `json:"labels"`          // where an exception label is read
}

const (
	onVM       = "VM o proyecto"
	onDisk     = "disco o proyecto"
	onTemplate = "template o proyecto"
	onProject  = "proyecto"
)

var Rules = []Rule{
	{"orphan-disk", "Disco huérfano", "Coste", High, "", onDisk},
	{"stopped-vm", "VM parada más de N días", "Coste", Medium, "stoppedDays", onVM},
	{"unused-ip", "IP externa reservada sin uso", "Coste", Medium, "", onProject},
	{"unused-reservation", "Reserva con capacidad sin usar", "Coste", Medium, "", onProject},
	{"cud-expiring", "Commitment (CUD) que vence pronto sin renovación", "Coste", Medium, "cudDays", onProject},
	{"cpu-low", "VM infrautilizada (CPU p95 baja)", "Coste", Medium, "cpuLowPct", onVM},
	{"old-family", "Familia de máquina antigua (N1)", "Coste", Low, "", onVM},
	{"old-snapshots", "Snapshots manuales antiguos", "Coste", Low, "snapshotDays", onProject},
	{"firewall-open", "Regla de firewall abierta a Internet", "Seguridad", High, "", onProject},
	{"default-sa", "Service account por defecto de Compute", "Seguridad", Medium, "", onVM},
	{"public-ip", "VM con IP pública", "Seguridad", Medium, "", onVM},
	{"no-secure-boot", "Secure Boot deshabilitado", "Seguridad", Low, "", onVM},
	{"template-image", "Template con imagen obsoleta, deprecada o antigua", "Seguridad", Medium, "imageDays", onTemplate},
	{"snapshot-schedule", "VM con discos sin snapshot schedule", "Protección de datos", Low, "", onVM},
	{"cpu-high", "CPU saturada", "Rendimiento", Medium, "cpuHighPct", onVM},
	{"mem-high", "Memoria saturada", "Rendimiento", Medium, "memHighPct", onVM},
	{"boot-pd-standard", "Disco de arranque pd-standard", "Rendimiento", Low, "", onVM},
	{"quota", "Cuota regional casi agotada", "Capacidad", Medium, "quotaPct", onProject},
	{"no-labels", "VM sin labels", "Gobierno", Low, "", onVM},
	{"stopped-no-disks", "VM parada sin discos", "Gobierno", Low, "", onVM},
	{"no-deletion-protection", "Deletion protection deshabilitada", "Operación", Low, "", onVM},
	{"no-ops-agent", "Sin Ops Agent", "Operación", Low, "", onVM},
}

// skipManaged lists rules that do not apply to VMs created and replaced by a
// Google service (GKE, Dataproc, Dataflow, Batch) or by a MIG: those VMs are
// disposable, so protecting or monitoring each one is the service's job.
var skipManaged = map[string]bool{
	"stopped-vm": true, "stopped-no-disks": true, "no-deletion-protection": true,
	"snapshot-schedule": true, "no-ops-agent": true,
}

// Result summarizes what was left out, for gMetaData.
type Result struct {
	Findings   int      `json:"findings"`
	Suppressed int      `json:"suppressed"` // hidden by gcetools-ignore labels
	Disabled   []string `json:"disabled"`   // rules switched off for the scan
}

type finding struct {
	id, severity, category, rule, project, resource, location, detail string
	labels                                                            []string // resource labels ("k=v; k=v"), checked for exceptions
}

func Evaluate(inv *model.Inventory, opt Options, now time.Time) Result {
	out := inv.Sheet(collect.SHealth)
	if out == nil {
		return Result{}
	}
	opt = opt.withDefaults()
	res := Result{Disabled: []string{}}
	for _, r := range Rules {
		if slices.Contains(opt.Disabled, r.ID) {
			res.Disabled = append(res.Disabled, r.ID)
		}
	}

	rows := func(name string) []model.Row {
		if s := inv.Sheet(name); s != nil {
			return s.Rows
		}
		return nil
	}
	projectLabels := map[string]string{}
	for _, r := range rows(collect.SProject) {
		projectLabels[str(r, "projectId")] = str(r, "labels")
	}
	vmLabels := map[string]string{}
	managedVM := map[string]bool{} // project/vm created by GKE, Dataproc, a MIG…
	for _, r := range rows(collect.SInfo) {
		k := str(r, "project") + "/" + str(r, "name")
		vmLabels[k] = str(r, "labels")
		if str(r, "managedBy") != "" || str(r, "mig") != "" {
			managedVM[k] = true
		}
	}
	diskLabels := map[string]string{}
	for _, r := range rows(collect.SDisks) {
		diskLabels[str(r, "project")+"/"+str(r, "name")] = str(r, "labels")
	}

	var fs []finding
	add := func(f finding) {
		if slices.Contains(opt.Disabled, f.id) || skipManaged[f.id] && managedVM[f.project+"/"+f.resource] {
			return
		}
		for _, l := range append(f.labels, projectLabels[f.project]) {
			if ignores(l, f.id) {
				res.Suppressed++
				return
			}
		}
		fs = append(fs, f)
	}

	for _, r := range rows(collect.SInfo) {
		p, n, z, lb := str(r, "project"), str(r, "name"), str(r, "zone"), []string{str(r, "labels")}
		if str(r, "status") == "TERMINATED" {
			if t, ok := r["lastStop"].(time.Time); ok && now.Sub(t) > time.Duration(opt.StoppedDays)*24*time.Hour {
				detail := fmt.Sprintf("Parada desde %s; sus discos siguen facturando", t.Format("2006-01-02"))
				if c, ok := r["costDisks"].(float64); ok && c > 0 {
					detail += fmt.Sprintf(" (~%.2f %s/mes)", c, inv.Meta.Currency)
				}
				add(finding{"stopped-vm", Medium, "Coste", fmt.Sprintf("VM parada más de %d días", opt.StoppedDays), p, n, z, detail, lb})
			}
		}
		if sa := str(r, "serviceAccount"); strings.HasSuffix(sa, "-compute@developer.gserviceaccount.com") {
			sev := Medium
			if strings.Contains(str(r, "scopes"), "cloud-platform") {
				sev = High
			}
			add(finding{"default-sa", sev, "Seguridad", "Usa la service account por defecto de Compute", p, n, z,
				"SA: " + sa + "; scopes: " + str(r, "scopes"), lb})
		}
		if str(r, "externalIP") != "" {
			add(finding{"public-ip", Medium, "Seguridad", "VM con IP pública", p, n, z, str(r, "externalIP"), lb})
		}
		if !boolean(r, "secureBoot") {
			add(finding{"no-secure-boot", Low, "Seguridad", "Shielded VM Secure Boot deshabilitado", p, n, z, "", lb})
		}
		if str(r, "labels") == "" {
			add(finding{"no-labels", Low, "Gobierno", "VM sin labels", p, n, z, "Dificulta el reparto de costes", lb})
		}
		if !boolean(r, "deletionProtection") {
			add(finding{"no-deletion-protection", Low, "Operación", "Deletion protection deshabilitada", p, n, z, "", lb})
		}
		if f := str(r, "family"); f == "n1" || f == "custom" {
			add(finding{"old-family", Low, "Coste", "Familia de máquina de generación antigua (N1)", p, n, z,
				"Valorar N2/N2D/N4/C3 (mejor precio/rendimiento)", lb})
		}
	}

	for _, r := range rows(collect.SDisk) {
		if boolean(r, "boot") && str(r, "type") == "pd-standard" {
			p := str(r, "project")
			add(finding{"boot-pd-standard", Low, "Rendimiento", "Disco de arranque pd-standard", p, str(r, "disk"),
				str(r, "zone"), "VM " + str(r, "vm") + "; valorar pd-balanced",
				[]string{vmLabels[p+"/"+str(r, "vm")], diskLabels[p+"/"+str(r, "disk")]}})
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
			add(finding{"orphan-disk", High, "Coste", "Disco huérfano", str(r, "project"), str(r, "name"), str(r, "location"),
				detail, []string{str(r, "labels")}})
		}
	}

	// Disks of VMs with no snapshot schedule. One finding per VM; managed VMs
	// are skipped (stateless by design) and orphans already have their own rule.
	type vmDisks struct {
		project, vm, zone string
		disks             []string
	}
	unprotected := map[string]*vmDisks{}
	var unprotectedOrder []string
	for _, r := range rows(collect.SDisks) {
		if boolean(r, "orphan") || str(r, "snapshotSchedule") != "" {
			continue
		}
		p := str(r, "project")
		for _, vm := range strings.Split(str(r, "users"), "; ") {
			if vm == "" || managedVM[p+"/"+vm] {
				continue
			}
			k := p + "/" + vm
			v := unprotected[k]
			if v == nil {
				v = &vmDisks{project: p, vm: vm, zone: str(r, "location")}
				unprotected[k] = v
				unprotectedOrder = append(unprotectedOrder, k)
			}
			v.disks = append(v.disks, str(r, "name"))
		}
	}
	for _, k := range unprotectedOrder {
		v := unprotected[k]
		add(finding{"snapshot-schedule", Low, "Protección de datos", "VM con discos sin snapshot schedule", v.project, v.vm, v.zone,
			"Discos: " + strings.Join(v.disks, ", ") + ". Si se protege con otra herramienta (Backup and DR, Veeam…), " +
				"desactiva la regla o usa el label gcetools-ignore=snapshot-schedule",
			[]string{vmLabels[k]}})
	}

	// Instance templates that create VMs from deprecated or old custom images.
	type imageInfo struct {
		deprecated string
		age        int64
	}
	images := map[string]imageInfo{}
	for _, r := range rows(collect.SImage) {
		if str(r, "kind") == "Image" {
			age, _ := r["ageDays"].(int64)
			images[str(r, "project")+"/"+str(r, "name")] = imageInfo{str(r, "deprecated"), age}
		}
	}
	templateMIGs := map[string][]string{}
	for _, r := range rows(collect.SMIG) {
		k := str(r, "project") + "/" + str(r, "template")
		templateMIGs[k] = append(templateMIGs[k], str(r, "name"))
	}
	for _, r := range rows(collect.STemplate) {
		img, ok := images[str(r, "imageProject")+"/"+str(r, "image")]
		if !ok {
			continue // public image, image family or image outside the scan
		}
		p, t, lb := str(r, "project"), str(r, "name"), []string{str(r, "labels")}
		detail := "Imagen " + str(r, "imageProject") + "/" + str(r, "image")
		if migs := templateMIGs[p+"/"+t]; len(migs) > 0 {
			detail += "; usado por el MIG " + strings.Join(migs, ", ")
		}
		switch img.deprecated {
		case "OBSOLETE", "DELETED":
			add(finding{"template-image", High, "Operación", "Template con imagen obsoleta o eliminada", p, t, "",
				detail + " (" + img.deprecated + "): no se podrán crear VMs nuevas", lb})
		case "DEPRECATED":
			add(finding{"template-image", Medium, "Operación", "Template con imagen deprecada", p, t, "", detail, lb})
		default:
			if img.age > int64(opt.ImageDays) {
				add(finding{"template-image", Low, "Seguridad", fmt.Sprintf("Template con imagen custom de más de %d días", opt.ImageDays), p, t, "",
					fmt.Sprintf("%s, creada hace %d días: probablemente sin parches recientes", detail, img.age), lb})
			}
		}
	}

	for _, r := range rows(collect.SAddress) {
		if str(r, "status") == "RESERVED" && str(r, "type") == "EXTERNAL" {
			add(finding{"unused-ip", Medium, "Coste", "IP externa reservada sin uso", str(r, "project"), str(r, "name"),
				str(r, "location"), str(r, "address"), nil})
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
		if !ok || age <= int64(opt.SnapshotDays) || boolean(r, "autoCreated") || ignores(str(r, "labels"), "old-snapshots") {
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
		add(finding{"old-snapshots", Low, "Coste", fmt.Sprintf("Snapshots manuales de más de %d días", opt.SnapshotDays), p,
			fmt.Sprintf("%d snapshots", a.count), "",
			fmt.Sprintf("%.0f GB almacenados; el más antiguo tiene %d días (detalle en gSnapshot)", a.gb, a.oldest), nil})
	}

	// VMs stopped with no disk attached are usually leftovers of migrations.
	for _, r := range rows(collect.SInfo) {
		if str(r, "status") == "TERMINATED" && num(r, "disks") == 0 {
			add(finding{"stopped-no-disks", Low, "Gobierno", "VM parada sin discos", str(r, "project"), str(r, "name"), str(r, "zone"),
				"Posible resto de una migración; revisar si sus discos aparecen como huérfanos", []string{str(r, "labels")}})
		}
	}

	for _, r := range rows(collect.SQuota) {
		if pct := num(r, "usagePct"); pct >= opt.QuotaPct {
			sev := Medium
			if pct >= 95 {
				sev = High
			}
			add(finding{"quota", sev, "Capacidad", "Cuota regional casi agotada", str(r, "project"), str(r, "metric"),
				str(r, "region"), fmt.Sprintf("%.0f de %.0f (%.0f%%)", num(r, "usage"), num(r, "limit"), pct), nil})
		}
	}

	for _, r := range rows(collect.SReserv) {
		if num(r, "unusedPct") > 0 && str(r, "status") == "READY" {
			add(finding{"unused-reservation", Medium, "Coste", "Reserva con capacidad sin usar", str(r, "project"), str(r, "name"), str(r, "zone"),
				fmt.Sprintf("%.0f de %.0f %s sin usar: se facturan igualmente", num(r, "count")-num(r, "inUse"),
					num(r, "count"), str(r, "machineType")), nil})
		}
	}

	for _, r := range rows(collect.SCommit) {
		left, ok := r["daysLeft"].(int64)
		if ok && str(r, "status") == "ACTIVE" && left >= 0 && left <= int64(opt.CUDDays) && !boolean(r, "autoRenew") {
			add(finding{"cud-expiring", Medium, "Coste", fmt.Sprintf("Commitment (CUD) vence en menos de %d días", opt.CUDDays),
				str(r, "project"), str(r, "name"), str(r, "region"),
				fmt.Sprintf("Vence en %d días y no se renueva automáticamente", left), nil})
		}
	}

	for _, r := range rows(collect.SPerf) {
		if str(r, "status") != "RUNNING" || num(r, "samples") < 24*7 {
			continue // need at least a week of data to judge
		}
		p, n, z := str(r, "project"), str(r, "vm"), str(r, "zone")
		lb := []string{vmLabels[p+"/"+n]}
		if cpu := num(r, "cpuP95"); cpu < opt.CPULowPct {
			add(finding{"cpu-low", Medium, "Coste", fmt.Sprintf("VM infrautilizada (CPU p95 < %g%%)", opt.CPULowPct), p, n, z,
				fmt.Sprintf("CPU p95 %.1f%% en %d días con %d vCPU (%s)", cpu, int(num(r, "days")), int(num(r, "vcpu")), str(r, "machineType")), lb})
		} else if cpu > opt.CPUHighPct {
			add(finding{"cpu-high", Medium, "Rendimiento", fmt.Sprintf("CPU saturada (p95 > %g%%)", opt.CPUHighPct), p, n, z,
				fmt.Sprintf("CPU p95 %.1f%%", cpu), lb})
		}
		if mem, ok := r["memP95"].(float64); ok && mem > opt.MemHighPct {
			add(finding{"mem-high", Medium, "Rendimiento", fmt.Sprintf("Memoria saturada (p95 > %g%%)", opt.MemHighPct), p, n, z,
				fmt.Sprintf("RAM p95 %.1f%%", mem), lb})
		}
		if !boolean(r, "opsAgent") {
			add(finding{"no-ops-agent", Low, "Operación", "Sin Ops Agent: no hay métricas de memoria ni disco", p, n, z, "", lb})
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
		add(finding{"firewall-open", sev, "Seguridad", rule, str(r, "project"), str(r, "name"), str(r, "network"),
			rules + " desde " + str(r, "sourceRanges") + " hacia " + str(r, "targets"), nil})
	}

	order := map[string]int{High: 0, Medium: 1, Low: 2}
	sortFindings(fs, order)
	for _, f := range fs {
		out.Add(model.Row{
			"severity": f.severity, "category": f.category, "rule": f.rule, "ruleId": f.id, "project": f.project,
			"resource": f.resource, "location": f.location, "detail": f.detail,
		})
	}
	res.Findings = len(fs)
	return res
}

// ignores reports whether a "k=v; k=v" label string holds an exception for
// rule id: gcetools-ignore=<id>|all or gcetools-ignore-<id>=<anything>.
func ignores(labels, id string) bool {
	if labels == "" {
		return false
	}
	for _, kv := range strings.Split(labels, "; ") {
		k, v, _ := strings.Cut(kv, "=")
		switch {
		case k == "gcetools-ignore" && (v == id || v == "all"):
			return true
		case k == "gcetools-ignore-"+id:
			return true
		}
	}
	return false
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
