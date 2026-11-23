// Package diff compares two inventories (usually two JSON snapshots) and
// returns the result as another inventory with Resumen, Altas, Bajas and
// Cambios sheets, so the GUI grid and the XLSX/CSV exporters work unchanged.
//
// Resources are matched by stable identity columns (project, location, name),
// volatile columns (last start, ages, quota usage…) are ignored, and a
// resource is only compared when its collector succeeded in both scans, so a
// permission error never shows up as hundreds of deletions.
package diff

import (
	"fmt"
	"gcetools/internal/schema"
	"math"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"gcetools/internal/health"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Output sheet names (machine values: titles are translated, names are not).
const (
	SSummary = "Resumen"
	SAdded   = "Altas"
	SRemoved = "Bajas"
	SChanged = "Cambios"
	SMeta    = "Metadatos"
)

type spec struct {
	sheet     string
	keys      []string // identity columns
	project   string   // project column ("" when the sheet has none)
	loc       string   // location column shown apart ("" none)
	summary   []string // columns described for added/removed resources
	ignore    []string // volatile columns, never compared
	collector string   // collector that fills the sheet; "*" = derived from all of them
	skip      func(model.Row) bool
}

// specs lists the compared sheets in display order. gCPU/gMemory repeat
// gInfo, and gQuota, gPerf, gRecommend, gCost, gErrors and gMetaData change
// on every scan, so they are not compared row by row.
var specs = []spec{
	{sheet: schema.SInfo, keys: []string{"project", "zone", "name"}, project: "project", loc: "zone",
		summary: []string{"machineType", "status", "os", "externalIP"},
		ignore:  []string{"lastStart", "lastStop", "cpuPlatform", "costNote", "costReal"}, collector: "instances"},
	{sheet: schema.SDisks, keys: []string{"project", "location", "name"}, project: "project", loc: "location",
		summary: []string{"type", "sizeGB", "users"}, ignore: []string{"lastAttach", "lastDetach", "costReal"}, collector: "disks"},
	{sheet: schema.SDisk, keys: []string{"project", "zone", "vm", "deviceName"}, project: "project", loc: "zone",
		summary: []string{"disk", "type", "sizeGB"}, collector: "instances"},
	{sheet: schema.SNetwork, keys: []string{"project", "zone", "vm", "nic"}, project: "project", loc: "zone",
		summary: []string{"network", "internalIP", "externalIP"}, collector: "instances"},
	{sheet: schema.SSnapshot, keys: []string{"project", "name"}, project: "project",
		summary: []string{"sourceDisk", "storageGB"}, ignore: []string{"ageDays"}, collector: "snapshots",
		// scheduled snapshots rotate every day: comparing them is pure noise
		skip: func(r model.Row) bool { return truthy(r["autoCreated"]) }},
	{sheet: schema.SImage, keys: []string{"project", "kind", "name"}, project: "project",
		summary: []string{"family", "status", "deprecated"}, ignore: []string{"ageDays"}, collector: "images"},
	{sheet: schema.SLicense, keys: []string{"project", "zone", "vm", "disk", "license"}, project: "project", loc: "zone",
		summary: []string{"category"}, collector: "instances"},
	{sheet: schema.SMIG, keys: []string{"project", "location", "name"}, project: "project", loc: "location",
		summary: []string{"template", "targetSize", "autoscaling"}, collector: "migs"},
	{sheet: schema.STemplate, keys: []string{"project", "name"}, project: "project",
		summary: []string{"machineType", "image"}, collector: "templates"},
	{sheet: schema.SReserv, keys: []string{"project", "zone", "name"}, project: "project", loc: "zone",
		summary: []string{"machineType", "count"}, ignore: []string{"inUse", "unusedPct"}, collector: "reservations"},
	{sheet: schema.SCommit, keys: []string{"project", "region", "name"}, project: "project", loc: "region",
		summary: []string{"plan", "type", "end"}, ignore: []string{"daysLeft"}, collector: "commitments"},
	{sheet: schema.SSole, keys: []string{"project", "zone", "node"}, project: "project", loc: "zone",
		summary: []string{"nodeGroup", "nodeType"}, ignore: []string{"instances", "vms"}, collector: "soletenant"},
	{sheet: schema.SVPC, keys: []string{"project", "name"}, project: "project",
		summary: []string{"mode", "routingMode"}, collector: "networks"},
	{sheet: schema.SSubnet, keys: []string{"project", "region", "name"}, project: "project", loc: "region",
		summary: []string{"network", "cidr"}, collector: "subnets"},
	{sheet: schema.SFirewall, keys: []string{"project", "name"}, project: "project", loc: "network",
		summary: []string{"direction", "action", "rules", "sourceRanges", "targets"}, collector: "firewalls"},
	{sheet: schema.SAddress, keys: []string{"project", "location", "name"}, project: "project", loc: "location",
		summary: []string{"address", "type", "status"}, collector: "addresses"},
	{sheet: schema.STools, keys: []string{"project", "zone", "vm"}, project: "project", loc: "zone",
		summary: []string{"os", "version"}, ignore: []string{"updated"}, collector: "osconfig"},
	{sheet: schema.SProject, keys: []string{"projectId"}, project: "projectId",
		summary: []string{"name", "parent", "vms"}, ignore: []string{"scanStatus"}},
	{sheet: schema.SSecurity, keys: []string{"project", "zone", "vm"}, project: "project", loc: "zone",
		summary: []string{"internetExposed", "exposedPorts", "issues"}, ignore: []string{"status", "notes"}, collector: "*"},
	{sheet: schema.SHealth, keys: []string{"rule", "project", "resource", "location"}, project: "project", loc: "location",
		summary: []string{"severity", "detail"}, ignore: []string{"detail"}, collector: "*"},
}

// Input identifies one side of the comparison.
type Input struct {
	Inv   *model.Inventory
	Label string // file name or "Inventario actual"
}

// c builds a column; the Spanish title is translated into the current
// language (TestSchemaTranslated checks every title has an English entry).
func c(key, title string, t model.ColType, w float64) model.Column {
	return model.Column{Key: key, Title: i18n.T(title), Type: t, Width: w}
}

func newResult() *model.Inventory {
	sheet := func(name, title, desc string, cols ...model.Column) *model.Sheet {
		return &model.Sheet{Name: name, Title: i18n.T(title), Description: i18n.T(desc), Columns: cols, Rows: []model.Row{}}
	}
	common := []model.Column{
		c("relevant", "Relevante", model.TBool, 10), c("reason", "Motivo", model.TString, 30),
		c("sheet", "Pestaña", model.TString, 12), c("kind", "Tipo de recurso", model.TString, 18),
		c("project", "Proyecto", model.TString, 24), c("resource", "Recurso", model.TString, 32),
		c("location", "Ubicación", model.TString, 16),
	}
	with := func(extra ...model.Column) []model.Column { return append(slices.Clone(common), extra...) }
	return &model.Inventory{Errors: []model.ScanError{}, Sheets: []*model.Sheet{
		sheet(SSummary, "Resumen", "Altas, bajas y cambios por tipo de recurso",
			c("sheet", "Pestaña", model.TString, 12), c("kind", "Tipo de recurso", model.TString, 22),
			c("added", "Altas", model.TInt, 8), c("removed", "Bajas", model.TInt, 8),
			c("modified", "Recursos modificados", model.TInt, 12), c("fields", "Campos cambiados", model.TInt, 12),
			c("relevant", "Relevantes", model.TInt, 10)),
		sheet(SAdded, "Altas", "Recursos que existen en el inventario nuevo y no en el anterior",
			with(c("detail", "Detalle", model.TString, 60))...),
		sheet(SRemoved, "Bajas", "Recursos del inventario anterior que ya no existen",
			with(c("detail", "Detalle", model.TString, 60))...),
		sheet(SChanged, "Cambios", "Un campo por fila: valor anterior y valor nuevo",
			with(c("field", "Campo", model.TString, 20), c("before", "Antes", model.TString, 30), c("after", "Después", model.TString, 30))...),
		sheet(SMeta, "Metadatos", "Inventarios comparados y alcance de la comparación",
			c("property", "Propiedad", model.TString, 28), c("value", "Valor", model.TString, 80)),
	}}
}

// coverage says which (project, collector) pairs a scan actually read.
type coverage struct {
	all      bool            // no gProject sheet: trust every row
	scanned  map[string]bool // projects scanned (OK or partial)
	listed   map[string]bool // projects present at all
	failures map[string][]string
}

func newCoverage(inv *model.Inventory) coverage {
	cv := coverage{scanned: map[string]bool{}, listed: map[string]bool{}, failures: map[string][]string{}}
	ps := inv.Sheet(schema.SProject)
	if ps == nil || len(ps.Rows) == 0 {
		cv.all = true
	} else {
		for _, r := range ps.Rows {
			id := r.Str("projectId")
			cv.listed[id] = true
			st := r.Str("scanStatus")
			if schema.ScanComparable(st) {
				cv.scanned[id] = true
			}
		}
	}
	for _, e := range inv.Errors {
		cv.failures[e.Project] = append(cv.failures[e.Project], e.Collector)
	}
	for p := range cv.failures {
		slices.Sort(cv.failures[p])
	}
	return cv
}

func (cv coverage) ok(project, collector string) bool {
	if !cv.all && !cv.scanned[project] {
		return false
	}
	return collector == "" || collector == "*" || !slices.Contains(cv.failures[project], collector)
}

// comparable reports whether rows of a sheet in project can be compared.
func comparable(a, b coverage, project, collector string) bool {
	if !a.ok(project, collector) || !b.ok(project, collector) {
		return false
	}
	if collector == "*" { // derived sheet: both scans must have read the same things
		return slices.Equal(a.failures[project], b.failures[project])
	}
	return true
}

var reSnapCount = regexp.MustCompile(`^\d+ snapshots$`)

// Compare returns the differences going from a (before) to b (after).
func Compare(a, b Input) *model.Inventory {
	out := newResult()
	add := func(name string, r model.Row) { out.Sheet(name).Add(r) }
	cova, covb := newCoverage(a.Inv), newCoverage(b.Inv)
	money := a.Inv.Meta.Currency != "" && a.Inv.Meta.Currency == b.Inv.Meta.Currency
	skipped := map[string]map[string]bool{} // sheet -> projects not comparable

	var added, removed, changed []model.Row
	for _, sp := range specs {
		sa, sb := a.Inv.Sheet(sp.sheet), b.Inv.Sheet(sp.sheet)
		if sa == nil || sb == nil {
			continue
		}
		cols := sb.Columns
		colOf := map[string]model.Column{}
		for _, col := range cols {
			colOf[col.Key] = col
		}
		// gHealth findings are matched by rule ID: the title is translated and
		// embeds thresholds. Scans from before ruleId existed use the title.
		byRuleID := sp.sheet == schema.SHealth && hasColumn(sa, "ruleId") && hasColumn(sb, "ruleId")
		index := func(s *model.Sheet) (map[string]model.Row, []string) {
			m, order, seen := map[string]model.Row{}, []string{}, map[string]int{}
			for _, r := range s.Rows {
				if sp.skip != nil && sp.skip(r) {
					continue
				}
				parts := make([]string, len(sp.keys))
				for i, k := range sp.keys {
					parts[i] = norm(colOf[k], r[k])
					if byRuleID && k == "rule" {
						parts[i] = r.Str("ruleId")
					}
					if sp.sheet == schema.SHealth && k == "resource" && reSnapCount.MatchString(parts[i]) {
						parts[i] = "snapshots" // aggregated finding: the count is not identity
					}
				}
				k := strings.Join(parts, "\x00")
				if n := seen[k]; n > 0 {
					seen[k]++
					k += "#" + strconv.Itoa(n)
				} else {
					seen[k] = 1
				}
				m[k] = r
				order = append(order, k)
			}
			return m, order
		}
		// Only columns both scans have: a column added in a newer version must
		// not show up as a change on every row.
		inA := map[string]bool{}
		for _, col := range sa.Columns {
			inA[col.Key] = true
		}
		var common []model.Column
		for _, col := range cols {
			if inA[col.Key] {
				common = append(common, col)
			}
		}
		ma, oa := index(sa)
		mb, ob := index(sb)

		proj := func(r model.Row) string {
			if sp.project == "" {
				return ""
			}
			return norm(colOf[sp.project], r[sp.project])
		}
		usable := func(r model.Row) bool {
			p := proj(r)
			if p == "" || comparable(cova, covb, p, sp.collector) {
				return true
			}
			if skipped[sp.sheet] == nil {
				skipped[sp.sheet] = map[string]bool{}
			}
			skipped[sp.sheet][p] = true
			return false
		}
		base := func(r model.Row) model.Row {
			var name []string
			for _, k := range sp.keys {
				if k != sp.project && k != sp.loc {
					name = append(name, norm(colOf[k], r[k]))
				}
			}
			row := model.Row{"sheet": sp.sheet, "kind": sb.Title, "project": proj(r), "resource": strings.Join(name, " · ")}
			if sp.loc != "" {
				row["location"] = norm(colOf[sp.loc], r[sp.loc])
			}
			return row
		}
		describe := func(r model.Row) string {
			var parts []string
			for _, k := range sp.summary {
				if v := norm(colOf[k], r[k]); v != "" {
					parts = append(parts, colOf[k].Title+": "+v)
				}
			}
			return strings.Join(parts, "; ")
		}
		var nAdd, nRem, nMod, nFields, nRel int

		for _, k := range ob {
			r := mb[k]
			if _, ok := ma[k]; ok || !usable(r) {
				continue
			}
			o := base(r)
			o["detail"] = describe(r)
			if why := addedReason(sp.sheet, r); why != "" {
				o["relevant"], o["reason"] = true, why
				nRel++
			} else {
				o["relevant"] = false
			}
			added = append(added, o)
			nAdd++
		}
		for _, k := range oa {
			r := ma[k]
			if _, ok := mb[k]; ok || !usable(r) {
				continue
			}
			o := base(r)
			o["detail"] = describe(r)
			o["relevant"] = false
			removed = append(removed, o)
			nRem++
		}
		for _, k := range ob {
			rb, ra := mb[k], ma[k]
			if ra == nil || !usable(rb) {
				continue
			}
			modified := false
			for _, col := range common {
				if slices.Contains(sp.keys, col.Key) || slices.Contains(sp.ignore, col.Key) ||
					(col.Type == model.TMoney && !money) {
					continue
				}
				va, vb := norm(col, ra[col.Key]), norm(col, rb[col.Key])
				if va == vb {
					continue
				}
				modified = true
				nFields++
				o := base(rb)
				o["field"], o["before"], o["after"] = col.Title, va, vb
				if why := changedReason(sp.sheet, col.Key, va, vb, rb); why != "" {
					o["relevant"], o["reason"] = true, why
					nRel++
				} else {
					o["relevant"] = false
				}
				changed = append(changed, o)
			}
			if modified {
				nMod++
			}
		}
		if nAdd+nRem+nMod > 0 {
			add(SSummary, model.Row{"sheet": sp.sheet, "kind": sb.Title, "added": int64(nAdd), "removed": int64(nRem),
				"modified": int64(nMod), "fields": int64(nFields), "relevant": int64(nRel)})
		}
	}

	relevantFirst := func(rows []model.Row) {
		slices.SortStableFunc(rows, func(x, y model.Row) int {
			bx, _ := x["relevant"].(bool)
			by, _ := y["relevant"].(bool)
			switch {
			case bx == by:
				return 0
			case bx:
				return -1
			}
			return 1
		})
	}
	for _, s := range []struct {
		name string
		rows []model.Row
	}{{SAdded, added}, {SRemoved, removed}, {SChanged, changed}} {
		relevantFirst(s.rows)
		out.Sheet(s.name).Rows = append(out.Sheet(s.name).Rows, s.rows...)
	}

	// Metadata: what was compared and what could not be.
	meta := func(k, v string) { add(SMeta, model.Row{"property": k, "value": v}) }
	side := func(label string, in Input) {
		m := in.Inv.Meta
		meta(label, in.Label)
		meta(label+": "+i18n.T("fecha"), fmtTime(m.StartedAt))
		meta(label+": "+i18n.T("identidad"), m.Identity)
		meta(label+": "+i18n.T("proyectos"), fmt.Sprintf("%d (%d OK)", m.Projects, m.ProjectsOK))
		meta(label+": "+i18n.T("versión"), m.Version)
	}
	side(i18n.T("Anterior"), a)
	side(i18n.T("Nuevo"), b)
	if !cova.all && !covb.all {
		var onlyA, onlyB []string
		for p := range cova.listed {
			if !covb.listed[p] {
				onlyA = append(onlyA, p)
			}
		}
		for p := range covb.listed {
			if !cova.listed[p] {
				onlyB = append(onlyB, p)
			}
		}
		slices.Sort(onlyA)
		slices.Sort(onlyB)
		if len(onlyA) > 0 {
			meta(i18n.T("Proyectos solo en el anterior"), strings.Join(onlyA, ", "))
		}
		if len(onlyB) > 0 {
			meta(i18n.T("Proyectos solo en el nuevo"), strings.Join(onlyB, ", "))
		}
	}
	for _, sp := range specs {
		if ps := skipped[sp.sheet]; len(ps) > 0 {
			list := make([]string, 0, len(ps))
			for p := range ps {
				list = append(list, p)
			}
			slices.Sort(list)
			meta(i18n.Tf("No comparado: %s", sp.sheet), i18n.Tf("%s (el proyecto no se leyó completo en alguno de los dos escaneos)", strings.Join(list, ", ")))
		}
	}
	if money {
		ta, tb := totalCost(a.Inv), totalCost(b.Inv)
		cur := b.Inv.Meta.Currency
		meta(i18n.T("Coste/mes estimado anterior"), fmt.Sprintf("%.2f %s", ta, cur))
		meta(i18n.T("Coste/mes estimado nuevo"), fmt.Sprintf("%.2f %s", tb, cur))
		meta(i18n.T("Diferencia de coste/mes"), fmt.Sprintf("%+.2f %s", tb-ta, cur))
	} else {
		meta(i18n.T("Costes"), i18n.T("No comparados: algún inventario no tiene costes o usan monedas distintas"))
	}
	meta(i18n.T("No se comparan"), i18n.T("gCPU y gMemory (repiten gInfo); gQuota, gPerf, gRecommend, gCost, gErrors y gMetaData (cambian en cada escaneo); snapshots creados por schedule (rotan a diario); fechas de último arranque/parada y antigüedades"))

	out.Meta = model.Meta{Tool: "gceTools", Version: b.Inv.Meta.Version, Identity: b.Inv.Meta.Identity,
		StartedAt: a.Inv.Meta.StartedAt, FinishedAt: b.Inv.Meta.StartedAt, Projects: b.Inv.Meta.Projects,
		ProjectsOK: b.Inv.Meta.ProjectsOK}
	if money {
		out.Meta.Currency = b.Inv.Meta.Currency
	}
	return out
}

func addedReason(sheet string, r model.Row) string {
	switch sheet {
	case schema.SInfo:
		if r.Str("externalIP") != "" {
			return i18n.T("VM nueva con IP pública")
		}
	case schema.SFirewall:
		if truthy(r["openToInternet"]) && !strings.EqualFold(r.Str("action"), "deny") {
			return i18n.T("Regla nueva abierta a Internet")
		}
	case schema.SSecurity:
		if truthy(r["internetExposed"]) {
			return i18n.T("VM nueva expuesta a Internet")
		}
	case schema.SDisks:
		if truthy(r["orphan"]) {
			return i18n.T("Disco huérfano nuevo")
		}
	case schema.SHealth:
		if health.Level(r.Str("severity")) == health.High {
			return i18n.T("Hallazgo nuevo de severidad alta")
		}
	case schema.SCommit:
		return i18n.T("Commitment (CUD) nuevo")
	}
	return ""
}

// changedReason receives before/after rendered by norm, so booleans come in
// the current language (yes).
func changedReason(sheet, field, before, after string, r model.Row) string {
	yes := i18n.T("Sí")
	switch sheet {
	case schema.SInfo:
		switch field {
		case "machineType":
			return i18n.T("Cambio de tamaño de la VM")
		case "serviceAccount", "scopes":
			return i18n.T("Cambio de identidad (service account)")
		case "secureBoot", "vtpm", "confidential":
			return i18n.T("Cambio en Shielded/Confidential VM")
		case "externalIP":
			if before == "" {
				return i18n.T("IP pública añadida")
			}
		case "deletionProtection":
			if after != "" && after != yes {
				return i18n.T("Deletion protection desactivada")
			}
		}
	case schema.SNetwork:
		if field == "externalIP" && before == "" {
			return i18n.T("IP pública añadida")
		}
	case schema.SFirewall:
		if field == "openToInternet" && after == yes {
			return i18n.T("Regla abierta a Internet")
		}
		if truthy(r["openToInternet"]) && slices.Contains([]string{"rules", "sourceRanges", "targets", "disabled", "action"}, field) {
			return i18n.T("Cambio en una regla abierta a Internet")
		}
	case schema.SSecurity:
		switch field {
		case "internetExposed":
			if after == yes {
				return i18n.T("VM que pasa a estar expuesta a Internet")
			}
		case "exposedPorts":
			if after != "" {
				return i18n.T("Cambio en los puertos abiertos a Internet")
			}
		case "projectSSHKeys", "serialPort":
			if after == yes {
				return i18n.T("Acceso a la VM ampliado")
			}
		}
	case schema.SDisks:
		if field == "orphan" && after == yes {
			return i18n.T("Disco que se quedó huérfano")
		}
	case schema.SHealth:
		if field == "severity" && health.Level(after) == health.High {
			return i18n.T("Hallazgo que pasa a severidad alta")
		}
	case schema.SCommit:
		if field == "status" {
			return i18n.T("Commitment cambia de estado")
		}
	}
	return ""
}

func totalCost(inv *model.Inventory) float64 {
	s := inv.Sheet(schema.SCost)
	if s == nil {
		return 0
	}
	t := 0.0
	for _, r := range s.Rows {
		if r.Str("project") != "TOTAL" {
			t += model.Num(r["total"])
		}
	}
	return t
}

// norm renders a value canonically so in-memory rows (time.Time, int64) and
// rows decoded from JSON (strings, float64) compare equal.
func norm(col model.Column, v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case bool:
		if x {
			return i18n.T("Sí")
		}
		return i18n.T("No")
	case time.Time:
		return fmtTime(x)
	case string:
		if col.Type == model.TTime {
			if t, err := time.Parse(time.RFC3339, x); err == nil {
				return fmtTime(t)
			}
		}
		return x
	case int64:
		return strconv.FormatInt(x, 10)
	case int:
		return strconv.Itoa(x)
	case float64:
		if col.Type == model.TInt {
			return strconv.FormatInt(int64(math.Round(x)), 10)
		}
		return strconv.FormatFloat(math.Round(x*100)/100, 'f', -1, 64)
	}
	return fmt.Sprint(v)
}

func fmtTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Local().Format("2006-01-02 15:04")
}

func hasColumn(s *model.Sheet, key string) bool {
	return slices.ContainsFunc(s.Columns, func(c model.Column) bool { return c.Key == key })
}

func truthy(v any) bool {
	b, _ := v.(bool)
	return b
}

// Display returns res with its sheet names in the current language, for the
// UI and exports. Compare keeps the Spanish names because the report and the
// CLI look sheets up by them.
func Display(res *model.Inventory) *model.Inventory {
	if res == nil {
		return nil
	}
	out := &model.Inventory{Meta: res.Meta, Errors: res.Errors}
	for _, s := range res.Sheets {
		out.Sheets = append(out.Sheets, &model.Sheet{Name: sheetName(s.Name), Title: s.Title,
			Description: s.Description, Columns: s.Columns, Rows: s.Rows})
	}
	return out
}

func sheetName(name string) string {
	switch name {
	case SSummary:
		return i18n.T("Resumen")
	case SAdded:
		return i18n.T("Altas")
	case SRemoved:
		return i18n.T("Bajas")
	case SChanged:
		return i18n.T("Cambios")
	case SMeta:
		return i18n.T("Metadatos")
	}
	return name
}
