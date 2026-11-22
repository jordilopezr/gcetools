// Package report renders a one-page executive summary of an inventory as a
// self-contained HTML file (inline CSS, no scripts, no external requests),
// readable in any browser and printable to PDF.
package report

import (
	"bytes"
	"embed"
	"html/template"
	"math"
	"sort"
	"strings"
	"time"

	"gcetools/internal/health"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

//go:embed report.html.tmpl
var files embed.FS

var tmpl = template.Must(template.New("report.html.tmpl").Funcs(template.FuncMap{
	"n0":   func(v float64) string { return group(math.Round(v), 0) },
	"n2":   func(v float64) string { return group(v, 2) },
	"lc":   strings.ToLower,
	"t":    i18n.T,
	"tf":   i18n.Tf,
	"lang": i18n.Lang,
	"pct": func(a, b float64) string {
		if b == 0 {
			return ""
		}
		return group(100*a/b, 0) + "%"
	},
}).ParseFS(files, "report.html.tmpl"))

type KPI struct {
	Label, Value, Sub string
	Alert             bool
}

// Finding is one row of the top findings; Level is the internal severity
// (health.High…), used as CSS class whatever the language of the scan.
type Finding struct{ Severity, Level, Rule, Project, Resource, Detail string }

type ProjectCost struct {
	Project    string
	VMs        int
	List, Real float64
	HasReal    bool
}

type Change struct{ Reason, Project, Resource, What string }

type Data struct {
	Generated, Scanned       string
	Identity, Version        string
	Projects, ProjectsOK     int
	Currency, RealCurrency   string
	RealMonth                string
	KPIs                     []KPI
	Findings                 []Finding
	FindingsTotal            int
	High, Medium, Low        int
	Costs                    []ProjectCost
	CostsMore                int
	Failed                   []string
	HasDiff                  bool
	DiffSince                string
	Added, Removed, Modified int
	Relevant                 []Change
	CostDelta                string
	Website                  string
}

// Options tune the report; Diff is a comparison against the previous scan
// (diff.Compare output) or nil.
type Options struct {
	Diff      *model.Inventory
	DiffSince time.Time
	Website   string
	Now       time.Time
}

func Render(inv *model.Inventory, opt Options) ([]byte, error) {
	d := build(inv, opt)
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, d); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func build(inv *model.Inventory, opt Options) Data {
	if opt.Now.IsZero() {
		opt.Now = time.Now()
	}
	m := inv.Meta
	d := Data{Generated: opt.Now.Format(dateTimeLayout()), Scanned: m.StartedAt.Local().Format(dateTimeLayout()),
		Identity: m.Identity, Version: m.Version, Projects: m.Projects, ProjectsOK: m.ProjectsOK, Currency: m.Currency,
		Website: opt.Website}
	rows := func(name string) []model.Row {
		if s := inv.Sheet(name); s != nil {
			return s.Rows
		}
		return nil
	}

	var vms, running int
	var vcpu, mem float64
	for _, r := range rows("gInfo") {
		vms++
		if r["status"] == "RUNNING" {
			running++
		}
		vcpu += num(r["vcpu"])
		mem += num(r["memoryGB"])
	}
	var disks, orphans int
	var diskGB, orphanGB float64
	for _, r := range rows("gDisks") {
		disks++
		diskGB += num(r["sizeGB"])
		if r["orphan"] == true {
			orphans++
			orphanGB += num(r["sizeGB"])
		}
	}
	var listTotal, realTotal, savings float64
	byProject := map[string]*ProjectCost{}
	pc := func(p string) *ProjectCost {
		if byProject[p] == nil {
			byProject[p] = &ProjectCost{Project: p}
		}
		return byProject[p]
	}
	for _, r := range rows("gCost") {
		p := str(r, "project")
		if p == "TOTAL" { // only present with several projects: sum them instead
			continue
		}
		c := pc(p)
		c.List = num(r["total"])
		listTotal += c.List
		c.VMs = int(num(r["vmsRunning"]))
	}
	for _, r := range rows("gCostReal") {
		p := str(r, "project")
		d.RealCurrency, d.RealMonth = str(r, "currency"), str(r, "month")
		if p == "TOTAL" {
			realTotal = num(r["net"])
			continue
		}
		c := pc(p)
		c.Real, c.HasReal = num(r["net"]), true
	}
	for _, r := range rows("gRecommend") {
		if v := num(r["savingsMonthly"]); v > 0 {
			savings += v
		}
	}

	d.KPIs = []KPI{
		{Label: "VMs", Value: group(float64(vms), 0), Sub: i18n.Tf("%s encendidas", group(float64(running), 0))},
		{Label: "vCPU / RAM", Value: group(vcpu, 0), Sub: i18n.Tf("%s GB de RAM", group(mem, 0))},
		{Label: i18n.T("Discos"), Value: group(diskGB/1024, 1) + " TB", Sub: i18n.Tf("%s discos", group(float64(disks), 0))},
		{Label: i18n.T("Discos huérfanos"), Value: group(float64(orphans), 0), Sub: i18n.Tf("%s GB sin VM", group(orphanGB, 0)), Alert: orphans > 0},
	}
	if sec := rows("gSecurity"); len(sec) > 0 { // absent in scans made before 0.8
		exposed := 0
		for _, r := range sec {
			if r["internetExposed"] == true {
				exposed++
			}
		}
		d.KPIs = append(d.KPIs, KPI{Label: i18n.T("VMs expuestas a Internet"), Value: group(float64(exposed), 0),
			Sub: i18n.T("puertos sensibles abiertos"), Alert: exposed > 0})
	}
	if m.Currency != "" {
		d.KPIs = append(d.KPIs, KPI{Label: i18n.T("Coste estimado/mes"), Value: group(listTotal, 0) + " " + m.Currency, Sub: i18n.T("precio de lista on-demand")})
	}
	if d.RealCurrency != "" {
		d.KPIs = append(d.KPIs, KPI{Label: i18n.Tf("Coste real %s", monthName(d.RealMonth)), Value: group(realTotal, 0) + " " + d.RealCurrency, Sub: i18n.T("neto, con descuentos")})
	}
	if savings > 0 {
		cur := m.Currency
		for _, r := range rows("gRecommend") {
			if c := str(r, "currency"); c != "" {
				cur = c
				break
			}
		}
		d.KPIs = append(d.KPIs, KPI{Label: i18n.T("Ahorro posible/mes"), Value: group(savings, 0) + " " + cur, Sub: i18n.T("según Google Recommender")})
	}

	for _, r := range rows("gHealth") {
		// The inventory may come from a scan in another language: compare
		// internal severities.
		level := health.Level(str(r, "severity"))
		switch level {
		case health.High:
			d.High++
		case health.Medium:
			d.Medium++
		case health.Low:
			d.Low++
		}
		d.FindingsTotal++
		if len(d.Findings) < 10 && level != health.Low {
			d.Findings = append(d.Findings, Finding{str(r, "severity"), level, str(r, "rule"), str(r, "project"), str(r, "resource"), str(r, "detail")})
		}
	}
	d.KPIs = append(d.KPIs, KPI{Label: i18n.T("Hallazgos altos"), Value: group(float64(d.High), 0),
		Sub: i18n.Tf("%s medios · %s bajos", group(float64(d.Medium), 0), group(float64(d.Low), 0)), Alert: d.High > 0})

	for _, c := range byProject {
		d.Costs = append(d.Costs, *c)
	}
	sort.Slice(d.Costs, func(i, j int) bool {
		a, b := d.Costs[i], d.Costs[j]
		return max(a.Real, a.List) > max(b.Real, b.List)
	})
	if len(d.Costs) > 10 {
		d.CostsMore = len(d.Costs) - 10
		d.Costs = d.Costs[:10]
	}

	for _, r := range rows("gProject") {
		st := str(r, "scanStatus")
		if !strings.HasPrefix(st, "OK") {
			d.Failed = append(d.Failed, str(r, "projectId")+": "+st)
		}
	}

	if opt.Diff != nil {
		d.HasDiff = true
		d.DiffSince = opt.DiffSince.Local().Format(dateLayout())
		for _, r := range opt.Diff.Sheet("Resumen").Rows {
			d.Added += int(num(r["added"]))
			d.Removed += int(num(r["removed"]))
			d.Modified += int(num(r["modified"]))
		}
		for _, s := range []string{"Altas", "Cambios"} {
			for _, r := range opt.Diff.Sheet(s).Rows {
				if r["relevant"] != true || len(d.Relevant) >= 10 {
					continue
				}
				what := ""
				if s == "Cambios" {
					what = str(r, "field") + ": " + str(r, "before") + " → " + str(r, "after")
				}
				d.Relevant = append(d.Relevant, Change{str(r, "reason"), str(r, "project"), str(r, "resource"), what})
			}
		}
		for _, r := range opt.Diff.Sheet("Metadatos").Rows {
			// The comparison is labelled in the current language; accept the
			// Spanish source too.
			if p := str(r, "property"); p == "Diferencia de coste/mes" || p == i18n.T("Diferencia de coste/mes") {
				d.CostDelta = str(r, "value")
			}
		}
	}
	return d
}

func monthName(yyyymm string) string {
	t, err := time.Parse("200601", yyyymm)
	if err != nil {
		return yyyymm
	}
	if i18n.Lang() == i18n.EN {
		return t.Format("January 2006")
	}
	months := []string{"enero", "febrero", "marzo", "abril", "mayo", "junio", "julio", "agosto", "septiembre", "octubre", "noviembre", "diciembre"}
	return months[t.Month()-1] + " " + t.Format("2006")
}

// dateTimeLayout and dateLayout follow the language: 18/11/2026 in Spanish,
// Nov 18, 2026 in English.
func dateTimeLayout() string {
	if i18n.Lang() == i18n.EN {
		return "Jan 2, 2006 15:04"
	}
	return "02/01/2006 15:04"
}

func dateLayout() string {
	if i18n.Lang() == i18n.EN {
		return "Jan 2, 2006"
	}
	return "02/01/2006"
}

// group formats with the separators of the language: 12.345,6 in Spanish,
// 12,345.6 in English.
func group(v float64, decimals int) string {
	thousands, point := byte('.'), ","
	if i18n.Lang() == i18n.EN {
		thousands, point = ',', "."
	}
	neg := v < 0
	v = math.Abs(v)
	p := math.Pow10(decimals)
	v = math.Round(v*p) / p
	ip := int64(v)
	frac := v - float64(ip)
	s := []byte{}
	digits := []byte(itoa(ip))
	for i, c := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			s = append(s, thousands)
		}
		s = append(s, c)
	}
	out := string(s)
	if decimals > 0 {
		f := itoa(int64(math.Round(frac * p)))
		for len(f) < decimals {
			f = "0" + f
		}
		out += point + f
	}
	if neg && out != "0" {
		out = "-" + out
	}
	return out
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func str(r model.Row, k string) string { s, _ := r[k].(string); return s }

func num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	}
	return 0
}
