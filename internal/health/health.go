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
	"gcetools/internal/schema"
	"slices"
	"strings"
	"time"

	"gcetools/internal/i18n"
	"gcetools/internal/model"
	"gcetools/internal/security"
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

// Severities are internal values (the Spanish source texts): logic and
// sorting use them, and gHealth rows get i18n.T of them.
const (
	High   = "Alta"
	Medium = "Media"
	Low    = "Baja"
)

// Level maps a severity as written in a gHealth row, in Spanish or English
// (inventories keep the language of their scan), to High, Medium or Low.
// It returns "" for anything else.
func Level(s string) string {
	switch s {
	case High, "High":
		return High
	case Medium, "Medium":
		return Medium
	case Low, "Low":
		return Low
	}
	return ""
}

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
	{"internet-exposed", "VM expuesta a Internet en puertos sensibles", "Seguridad", High, "", onVM},
	{"project-ssh-keys", "VM que acepta claves SSH de proyecto", "Seguridad", Medium, "", onVM},
	{"serial-port", "Acceso por puerto serie habilitado", "Seguridad", Medium, "", onVM},
	{"no-os-login", "OS Login deshabilitado", "Seguridad", Low, "", onVM},
	{"ip-forward", "IP forwarding habilitado", "Seguridad", Low, "", onVM},
	{"no-shielded-vm", "vTPM o integrity monitoring deshabilitado", "Seguridad", Low, "", onVM},
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
	"snapshot-schedule": true, "no-ops-agent": true, "no-os-login": true, "ip-forward": true,
}

// LocalizedRules returns Rules with their texts in the current language
// (Rules itself keeps the Spanish source texts).
func LocalizedRules() []Rule {
	out := make([]Rule, len(Rules))
	for i, r := range Rules {
		r.Title, r.Category, r.Severity, r.Labels = i18n.T(r.Title), i18n.T(r.Category), i18n.T(r.Severity), i18n.T(r.Labels)
		out[i] = r
	}
	return out
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
	out := inv.Sheet(schema.SHealth)
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

	e := newEvaluator(inv, opt, now, &res)
	for _, check := range []func(){
		e.vms, e.attachedDisks, e.disks, e.snapshotSchedules, e.templates, e.addresses, e.snapshots,
		e.stoppedWithoutDisks, e.quotas, e.reservations, e.commitments, e.performance, e.firewalls,
		e.securityPosture,
	} {
		check()
	}

	fs := e.findings
	order := map[string]int{High: 0, Medium: 1, Low: 2}
	sortFindings(fs, order)
	for _, f := range fs {
		out.Add(model.Row{
			// Severity and category stay internal (Spanish) for sorting and are
			// translated only when written.
			"severity": i18n.T(f.severity), "category": i18n.T(f.category), "rule": f.rule, "ruleId": f.id, "project": f.project,
			"resource": f.resource, "location": f.location, "detail": f.detail,
		})
	}
	res.Findings = len(fs)
	return res
}

// evaluator holds what every check needs: the inventory, the options, the
// labels used for exceptions and the VMs managed by a Google service. Each
// check reads one or a few sheets and reports through add.
type evaluator struct {
	inv *model.Inventory
	opt Options
	now time.Time
	res *Result

	projectLabels map[string]string
	vmLabels      map[string]string
	managedVM     map[string]bool // project/vm created by GKE, Dataproc, a MIG…
	diskLabels    map[string]string

	findings []finding
}

func newEvaluator(inv *model.Inventory, opt Options, now time.Time, res *Result) *evaluator {
	e := &evaluator{inv: inv, opt: opt, now: now, res: res, projectLabels: map[string]string{},
		vmLabels: map[string]string{}, managedVM: map[string]bool{}, diskLabels: map[string]string{}}
	for _, r := range inv.Rows(schema.SProject) {
		e.projectLabels[r.Str("projectId")] = r.Str("labels")
	}
	for _, r := range inv.Rows(schema.SInfo) {
		k := r.Str("project") + "/" + r.Str("name")
		e.vmLabels[k] = r.Str("labels")
		if r.Str("managedBy") != "" || r.Str("mig") != "" {
			e.managedVM[k] = true
		}
	}
	for _, r := range inv.Rows(schema.SDisks) {
		e.diskLabels[r.Str("project")+"/"+r.Str("name")] = r.Str("labels")
	}
	return e
}

// add records a finding unless its rule is disabled, it does not apply to
// managed VMs, or a gcetools-ignore label excludes it.
func (e *evaluator) add(f finding) {
	if slices.Contains(e.opt.Disabled, f.id) || skipManaged[f.id] && e.managedVM[f.project+"/"+f.resource] {
		return
	}
	for _, l := range append(f.labels, e.projectLabels[f.project]) {
		if ignores(l, f.id) {
			e.res.Suppressed++
			return
		}
	}
	e.findings = append(e.findings, f)
}

// VM-level rules from gInfo: stopped, labels, service account, public IP…
func (e *evaluator) vms() {
	for _, r := range e.inv.Rows(schema.SInfo) {
		p, n, z, lb := r.Str("project"), r.Str("name"), r.Str("zone"), []string{r.Str("labels")}
		if r.Str("status") == "TERMINATED" {
			if t, ok := r["lastStop"].(time.Time); ok && e.now.Sub(t) > time.Duration(e.opt.StoppedDays)*24*time.Hour {
				detail := i18n.Tf("Parada desde %s; sus discos siguen facturando", t.Format("2006-01-02"))
				if c, ok := r["costDisks"].(float64); ok && c > 0 {
					detail += i18n.Tf(" (~%.2f %s/mes)", c, e.inv.Meta.Currency)
				}
				e.add(finding{"stopped-vm", Medium, "Coste", i18n.Tf("VM parada más de %d días", e.opt.StoppedDays), p, n, z, detail, lb})
			}
		}
		if sa := r.Str("serviceAccount"); strings.HasSuffix(sa, "-compute@developer.gserviceaccount.com") {
			sev := Medium
			if strings.Contains(r.Str("scopes"), "cloud-platform") {
				sev = High
			}
			e.add(finding{"default-sa", sev, "Seguridad", i18n.T("Usa la service account por defecto de Compute"), p, n, z,
				"SA: " + sa + "; scopes: " + r.Str("scopes"), lb})
		}
		if r.Str("externalIP") != "" {
			e.add(finding{"public-ip", Medium, "Seguridad", i18n.T("VM con IP pública"), p, n, z, r.Str("externalIP"), lb})
		}
		if !r.Bool("secureBoot") {
			e.add(finding{"no-secure-boot", Low, "Seguridad", i18n.T("Shielded VM Secure Boot deshabilitado"), p, n, z, "", lb})
		}
		if r.Str("labels") == "" {
			e.add(finding{"no-labels", Low, "Gobierno", i18n.T("VM sin labels"), p, n, z, i18n.T("Dificulta el reparto de costes"), lb})
		}
		if !r.Bool("deletionProtection") {
			e.add(finding{"no-deletion-protection", Low, "Operación", i18n.T("Deletion protection deshabilitada"), p, n, z, "", lb})
		}
		if f := r.Str("family"); f == "n1" || f == "custom" {
			e.add(finding{"old-family", Low, "Coste", i18n.T("Familia de máquina de generación antigua (N1)"), p, n, z,
				i18n.T("Valorar N2/N2D/N4/C3 (mejor precio/rendimiento)"), lb})
		}
	}
}

// Attached disks (gDisk).
func (e *evaluator) attachedDisks() {
	for _, r := range e.inv.Rows(schema.SDisk) {
		if r.Bool("boot") && r.Str("type") == "pd-standard" {
			p := r.Str("project")
			e.add(finding{"boot-pd-standard", Low, "Rendimiento", i18n.T("Disco de arranque pd-standard"), p, r.Str("disk"),
				r.Str("zone"), i18n.Tf("VM %s; valorar pd-balanced", r.Str("vm")),
				[]string{e.vmLabels[p+"/"+r.Str("vm")], e.diskLabels[p+"/"+r.Str("disk")]}})
		}
	}
}

// Disks (gDisks): orphans and boot disk type.
func (e *evaluator) disks() {
	for _, r := range e.inv.Rows(schema.SDisks) {
		if r.Bool("orphan") {
			detail := i18n.Tf("%.0f GB %s sin VM", r.Float("sizeGB"), r.Str("type"))
			if t, ok := r["lastDetach"].(time.Time); ok {
				detail += i18n.Tf("; último detach %s", t.Format("2006-01-02"))
			}
			if c, ok := r["costMonthly"].(float64); ok {
				detail += i18n.Tf("; ~%.2f %s/mes", c, e.inv.Meta.Currency)
			}
			e.add(finding{"orphan-disk", High, "Coste", i18n.T("Disco huérfano"), r.Str("project"), r.Str("name"), r.Str("location"),
				detail, []string{r.Str("labels")}})
		}
	}
}

func (e *evaluator) snapshotSchedules() {
	// Disks of VMs with no snapshot schedule. One finding per VM; managed VMs
	// are skipped (stateless by design) and orphans already have their own rule.
	type vmDisks struct {
		project, vm, zone string
		disks             []string
	}
	unprotected := map[string]*vmDisks{}
	var unprotectedOrder []string
	for _, r := range e.inv.Rows(schema.SDisks) {
		if r.Bool("orphan") || r.Str("snapshotSchedule") != "" {
			continue
		}
		p := r.Str("project")
		for _, vm := range strings.Split(r.Str("users"), "; ") {
			if vm == "" || e.managedVM[p+"/"+vm] {
				continue
			}
			k := p + "/" + vm
			v := unprotected[k]
			if v == nil {
				v = &vmDisks{project: p, vm: vm, zone: r.Str("location")}
				unprotected[k] = v
				unprotectedOrder = append(unprotectedOrder, k)
			}
			v.disks = append(v.disks, r.Str("name"))
		}
	}
	for _, k := range unprotectedOrder {
		v := unprotected[k]
		e.add(finding{"snapshot-schedule", Low, "Protección de datos", i18n.T("VM con discos sin snapshot schedule"), v.project, v.vm, v.zone,
			i18n.Tf("Discos: %s. Si se protege con otra herramienta (Backup and DR, Veeam…), desactiva la regla o usa el label gcetools-ignore=snapshot-schedule",
				strings.Join(v.disks, ", ")),
			[]string{e.vmLabels[k]}})
	}
}

func (e *evaluator) templates() {
	// Instance templates that create VMs from deprecated or old custom images.
	type imageInfo struct {
		deprecated string
		age        int64
	}
	images := map[string]imageInfo{}
	for _, r := range e.inv.Rows(schema.SImage) {
		if r.Str("kind") == "Image" {
			age, _ := r["ageDays"].(int64)
			images[r.Str("project")+"/"+r.Str("name")] = imageInfo{r.Str("deprecated"), age}
		}
	}
	templateMIGs := map[string][]string{}
	for _, r := range e.inv.Rows(schema.SMIG) {
		k := r.Str("project") + "/" + r.Str("template")
		templateMIGs[k] = append(templateMIGs[k], r.Str("name"))
	}
	for _, r := range e.inv.Rows(schema.STemplate) {
		img, ok := images[r.Str("imageProject")+"/"+r.Str("image")]
		if !ok {
			continue // public image, image family or image outside the scan
		}
		p, t, lb := r.Str("project"), r.Str("name"), []string{r.Str("labels")}
		detail := i18n.Tf("Imagen %s/%s", r.Str("imageProject"), r.Str("image"))
		if migs := templateMIGs[p+"/"+t]; len(migs) > 0 {
			detail += i18n.Tf("; usado por el MIG %s", strings.Join(migs, ", "))
		}
		switch img.deprecated {
		case "OBSOLETE", "DELETED":
			e.add(finding{"template-image", High, "Operación", i18n.T("Template con imagen obsoleta o eliminada"), p, t, "",
				i18n.Tf("%s (%s): no se podrán crear VMs nuevas", detail, img.deprecated), lb})
		case "DEPRECATED":
			e.add(finding{"template-image", Medium, "Operación", i18n.T("Template con imagen deprecada"), p, t, "", detail, lb})
		default:
			if img.age > int64(e.opt.ImageDays) {
				e.add(finding{"template-image", Low, "Seguridad", i18n.Tf("Template con imagen custom de más de %d días", e.opt.ImageDays), p, t, "",
					i18n.Tf("%s, creada hace %d días: probablemente sin parches recientes", detail, img.age), lb})
			}
		}
	}
}

// Reserved external IPs not in use.
func (e *evaluator) addresses() {
	for _, r := range e.inv.Rows(schema.SAddress) {
		if r.Str("status") == "RESERVED" && r.Str("type") == "EXTERNAL" {
			e.add(finding{"unused-ip", Medium, "Coste", i18n.T("IP externa reservada sin uso"), r.Str("project"), r.Str("name"),
				r.Str("location"), r.Str("address"), nil})
		}
	}
}

func (e *evaluator) snapshots() {
	// Old manual snapshots are aggregated per project: backup tools create
	// thousands of them and one row each would drown every other finding.
	type snapAgg struct {
		count  int
		gb     float64
		oldest int64
	}
	snaps := map[string]*snapAgg{}
	var snapOrder []string
	for _, r := range e.inv.Rows(schema.SSnapshot) {
		age, ok := r["ageDays"].(int64)
		if !ok || age <= int64(e.opt.SnapshotDays) || r.Bool("autoCreated") || ignores(r.Str("labels"), "old-snapshots") {
			continue
		}
		p := r.Str("project")
		a := snaps[p]
		if a == nil {
			a = &snapAgg{}
			snaps[p] = a
			snapOrder = append(snapOrder, p)
		}
		a.count++
		a.gb += r.Float("storageGB")
		a.oldest = max(a.oldest, age)
	}
	for _, p := range snapOrder {
		a := snaps[p]
		e.add(finding{"old-snapshots", Low, "Coste", i18n.Tf("Snapshots manuales de más de %d días", e.opt.SnapshotDays), p,
			fmt.Sprintf("%d snapshots", a.count), "",
			i18n.Tf("%.0f GB almacenados; el más antiguo tiene %d días (detalle en gSnapshot)", a.gb, a.oldest), nil})
	}
}

func (e *evaluator) stoppedWithoutDisks() {
	// VMs stopped with no disk attached are usually leftovers of migrations.
	for _, r := range e.inv.Rows(schema.SInfo) {
		if r.Str("status") == "TERMINATED" && r.Float("disks") == 0 {
			e.add(finding{"stopped-no-disks", Low, "Gobierno", i18n.T("VM parada sin discos"), r.Str("project"), r.Str("name"), r.Str("zone"),
				i18n.T("Posible resto de una migración; revisar si sus discos aparecen como huérfanos"), []string{r.Str("labels")}})
		}
	}
}

// Regional quotas close to the limit.
func (e *evaluator) quotas() {
	for _, r := range e.inv.Rows(schema.SQuota) {
		if pct := r.Float("usagePct"); pct >= e.opt.QuotaPct {
			sev := Medium
			if pct >= 95 {
				sev = High
			}
			e.add(finding{"quota", sev, "Capacidad", i18n.T("Cuota regional casi agotada"), r.Str("project"), r.Str("metric"),
				r.Str("region"), i18n.Tf("%.0f de %.0f (%.0f%%)", r.Float("usage"), r.Float("limit"), pct), nil})
		}
	}
}

// Reservations not used.
func (e *evaluator) reservations() {
	for _, r := range e.inv.Rows(schema.SReserv) {
		if r.Float("unusedPct") > 0 && r.Str("status") == "READY" {
			e.add(finding{"unused-reservation", Medium, "Coste", i18n.T("Reserva con capacidad sin usar"), r.Str("project"), r.Str("name"), r.Str("zone"),
				i18n.Tf("%.0f de %.0f %s sin usar: se facturan igualmente", r.Float("count")-r.Float("inUse"),
					r.Float("count"), r.Str("machineType")), nil})
		}
	}
}

// Commitments about to expire.
func (e *evaluator) commitments() {
	for _, r := range e.inv.Rows(schema.SCommit) {
		left, ok := r["daysLeft"].(int64)
		if ok && r.Str("status") == "ACTIVE" && left >= 0 && left <= int64(e.opt.CUDDays) && !r.Bool("autoRenew") {
			e.add(finding{"cud-expiring", Medium, "Coste", i18n.Tf("Commitment (CUD) vence en menos de %d días", e.opt.CUDDays),
				r.Str("project"), r.Str("name"), r.Str("region"),
				i18n.Tf("Vence en %d días y no se renueva automáticamente", left), nil})
		}
	}
}

// CPU and memory from Cloud Monitoring (gPerf).
func (e *evaluator) performance() {
	for _, r := range e.inv.Rows(schema.SPerf) {
		if r.Str("status") != "RUNNING" || r.Float("samples") < 24*7 {
			continue // need at least a week of data to judge
		}
		p, n, z := r.Str("project"), r.Str("vm"), r.Str("zone")
		lb := []string{e.vmLabels[p+"/"+n]}
		if cpu := r.Float("cpuP95"); cpu < e.opt.CPULowPct {
			e.add(finding{"cpu-low", Medium, "Coste", i18n.Tf("VM infrautilizada (CPU p95 < %g%%)", e.opt.CPULowPct), p, n, z,
				i18n.Tf("CPU p95 %.1f%% en %d días con %d vCPU (%s)", cpu, int(r.Float("days")), int(r.Float("vcpu")), r.Str("machineType")), lb})
		} else if cpu > e.opt.CPUHighPct {
			e.add(finding{"cpu-high", Medium, "Rendimiento", i18n.Tf("CPU saturada (p95 > %g%%)", e.opt.CPUHighPct), p, n, z,
				fmt.Sprintf("CPU p95 %.1f%%", cpu), lb})
		}
		if mem, ok := r["memP95"].(float64); ok && mem > e.opt.MemHighPct {
			e.add(finding{"mem-high", Medium, "Rendimiento", i18n.Tf("Memoria saturada (p95 > %g%%)", e.opt.MemHighPct), p, n, z,
				fmt.Sprintf("RAM p95 %.1f%%", mem), lb})
		}
		if !r.Bool("opsAgent") {
			e.add(finding{"no-ops-agent", Low, "Operación", i18n.T("Sin Ops Agent: no hay métricas de memoria ni disco"), p, n, z, "", lb})
		}
	}
}

// Firewall rules open to the Internet.
func (e *evaluator) firewalls() {
	for _, r := range e.inv.Rows(schema.SFirewall) {
		if !r.Bool("openToInternet") {
			continue
		}
		rules := r.Str("rules")
		sev, rule := Medium, i18n.T("Regla de firewall abierta a Internet")
		if security.OpensAdminOrAllPorts(rules) {
			sev, rule = High, i18n.T("SSH/RDP o todos los puertos abiertos a Internet")
		}
		e.add(finding{"firewall-open", sev, "Seguridad", rule, r.Str("project"), r.Str("name"), r.Str("network"),
			i18n.Tf("%s desde %s hacia %s", rules, r.Str("sourceRanges"), r.Str("targets")), nil})
	}
}

func (e *evaluator) securityPosture() {
	// Per-VM security posture (gSecurity, filled by the security package).
	for _, r := range e.inv.Rows(schema.SSecurity) {
		p, n, z := r.Str("project"), r.Str("vm"), r.Str("zone")
		lb := []string{e.vmLabels[p+"/"+n]}
		if r.Bool("internetExposed") {
			e.add(finding{"internet-exposed", High, "Seguridad", i18n.T("VM expuesta a Internet en puertos sensibles"), p, n, z,
				i18n.Tf("%s abiertos en %s por %s", r.Str("exposedPorts"), r.Str("externalIP"), r.Str("exposingRules")), lb})
		}
		if r.Bool("projectSSHKeys") {
			e.add(finding{"project-ssh-keys", Medium, "Seguridad", i18n.T("VM que acepta claves SSH de proyecto"), p, n, z,
				i18n.T("Quien tenga una clave en los metadatos del proyecto entra en esta VM; activa OS Login o block-project-ssh-keys"), lb})
		}
		if r.Bool("serialPort") {
			e.add(finding{"serial-port", Medium, "Seguridad", i18n.T("Acceso por puerto serie habilitado"), p, n, z,
				i18n.T("serial-port-enable=TRUE en la VM o en el proyecto"), lb})
		}
		if !r.Bool("osLogin") {
			e.add(finding{"no-os-login", Low, "Seguridad", i18n.T("OS Login deshabilitado"), p, n, z,
				i18n.T("El acceso SSH se gestiona con claves en metadatos en lugar de IAM. Si una política de organización exige OS Login, desactiva la regla"), lb})
		}
		if r.Bool("ipForward") {
			e.add(finding{"ip-forward", Low, "Seguridad", i18n.T("IP forwarding habilitado"), p, n, z,
				i18n.T("Solo necesario en appliances de red, NAT o VPN"), lb})
		}
		if !r.Bool("vtpm") || !r.Bool("integrityMonitoring") {
			var off []string
			if !r.Bool("vtpm") {
				off = append(off, "vTPM")
			}
			if !r.Bool("integrityMonitoring") {
				off = append(off, "integrity monitoring")
			}
			e.add(finding{"no-shielded-vm", Low, "Seguridad", i18n.T("vTPM o integrity monitoring deshabilitado"), p, n, z,
				i18n.Tf("Deshabilitado: %s", strings.Join(off, ", ")), lb})
		}
	}
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
