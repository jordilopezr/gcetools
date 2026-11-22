// Package cli implements the headless "scan" command so inventories can be
// scheduled (cron / Task Scheduler) without opening the GUI.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"slices"
	"strings"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/billing"
	"gcetools/internal/core"
	"gcetools/internal/diff"
	"gcetools/internal/export"
	"gcetools/internal/health"
	"gcetools/internal/history"
	"gcetools/internal/i18n"
	"gcetools/internal/profile"
	"gcetools/internal/report"
	"gcetools/internal/update"
)

const usage = `gcetools scan — inventario de Compute Engine sin interfaz gráfica

Uso:
  gcetools scan [opciones]

Ejemplos:
  gcetools scan --out inventario.xlsx
  gcetools scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip
  gcetools scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b
  gcetools scan --discover organizations/123456 --out inventario.json
  gcetools scan --discover organizations/123456 --fast --out inventario.xlsx
  gcetools scan --profile produccion --out inventario.xlsx
  gcetools scan --save-history --report informe.html --out inventario.xlsx
  gcetools scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXX --out inventario.xlsx

Con --profile se usan los valores guardados en la app (Proyectos → Perfil);
las opciones que pases explícitamente tienen prioridad.
Idioma de los mensajes y del inventario: --lang es|en (o GCETOOLS_LANG).

Opciones:
`

func Run(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), i18n.T(usage)); fs.PrintDefaults() }
	method := fs.String("auth", "adc", i18n.T("método: adc | sa | impersonate | oauth (tras gcetools login)"))
	keyPath := fs.String("key", "", i18n.T("ruta a la clave JSON de la service account (--auth sa)"))
	target := fs.String("impersonate", "", i18n.T("email de la service account a impersonar (--auth impersonate)"))
	projects := fs.String("projects", "", i18n.T("IDs de proyecto separados por comas (por defecto: todos los visibles)"))
	query := fs.String("query", "", i18n.T(`filtro de Resource Manager, p.ej. "parent:folders/123"`))
	out := fs.String("out", "gcetools-"+time.Now().Format("20060102-1504")+".xlsx", i18n.T("archivo de salida (.xlsx, .zip/.csv, .json)"))
	format := fs.String("format", "", i18n.T("xlsx | csv | json (por defecto según la extensión)"))
	sep := fs.String("csv-sep", ",", i18n.T("separador CSV (usa ';' para Excel en español)"))
	conc := fs.Int("concurrency", 8, i18n.T("proyectos en paralelo"))
	rps := fs.Float64("rps", 20, i18n.T("peticiones por segundo máximas"))
	modules := fs.String("modules", "osconfig,monitoring,recommender,pricing", i18n.T("módulos opcionales separados por comas (\"none\" para desactivarlos)"))
	metricsDays := fs.Int("metrics-days", 30, i18n.T("días de métricas de Cloud Monitoring"))
	currency := fs.String("currency", "USD", i18n.T("moneda de los costes estimados (USD, EUR, CLP…)"))
	disc := fs.String("discover", "", i18n.T("escanea solo proyectos con recursos de Compute según Cloud Asset Inventory (organizations/N, folders/N)"))
	profName := fs.String("profile", "", i18n.T("perfil guardado (gcetools profiles los lista)"))
	billTable := fs.String("billing-table", "", i18n.T("tabla de exportación de facturación en BigQuery (proyecto.dataset.tabla) para el coste real"))
	billMonth := fs.String("billing-month", "", i18n.T("mes de facturación AAAAMM (por defecto, el mes anterior)"))
	billMaxGB := fs.Int("billing-max-gb", 50, i18n.T("no ejecutar la consulta de BigQuery si leería más GB que esto"))
	reportPath := fs.String("report", "", i18n.T("genera además el informe ejecutivo en HTML en esta ruta"))
	saveHistory := fs.Bool("save-history", false, i18n.T("guarda también el resultado en el historial local (el mismo que usa la app)"))
	fast := fs.Bool("fast", false, i18n.T("modo rápido: lee VMs, discos, snapshots, imágenes, templates, MIGs y redes de Cloud Asset Inventory (con --discover, una sola lectura para todo el ámbito)"))
	disableRules := fs.String("disable-rules", "", i18n.T("reglas de gHealth a desactivar, separadas por comas (gcetools rules las lista)"))
	if err := fs.Parse(args); err != nil {
		return 2
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if *billTable != "" {
		if _, _, _, err := billing.ParseTable(*billTable); err != nil {
			return fail(err.Error())
		}
	}
	for _, id := range strings.Split(*disableRules, ",") { // validate before asking for credentials
		if id = strings.TrimSpace(id); id != "" && !slices.ContainsFunc(health.Rules, func(r health.Rule) bool { return r.ID == id }) {
			return fail(i18n.Tf("regla desconocida %q (gcetools rules las lista)", id))
		}
	}
	var prof *profile.Profile
	if *profName != "" {
		p, err := profile.Get(*profName)
		if err != nil {
			return fail(err.Error())
		}
		prof = &p
		def := func(flagName string, dst *string, v string) {
			if !set[flagName] && v != "" {
				*dst = v
			}
		}
		def("auth", method, p.Auth)
		def("key", keyPath, p.KeyPath)
		def("impersonate", target, p.Impersonate)
		def("query", query, p.Query)
		def("projects", projects, strings.Join(p.Projects, ","))
		def("discover", disc, p.Discover)
		if !set["concurrency"] && p.Concurrency > 0 {
			*conc = p.Concurrency
		}
		fmt.Fprint(os.Stderr, i18n.Tf("Perfil: %s\n", p.Name))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := auth.Config{Method: auth.Method(*method), TargetSA: *target}
	if cfg.Method == auth.KeyFile {
		data, err := os.ReadFile(*keyPath)
		if err != nil {
			return fail(i18n.Tf("no se pudo leer la clave: %v", err))
		}
		cfg.KeyJSON = data
	}
	sess, err := auth.Connect(ctx, cfg)
	if err != nil {
		return fail(err.Error())
	}
	fmt.Fprint(os.Stderr, i18n.Tf("Conectado como %s (%s)\n", sess.Identity, sess.Method))
	eng := core.NewEngine(sess, *rps)

	var list []core.Project
	switch {
	case *projects != "":
		for _, id := range strings.Split(*projects, ",") {
			if id = strings.TrimSpace(id); id != "" {
				list = append(list, core.Project{ID: id})
			}
		}
	case *disc != "" && *query == "":
		// discovery alone: no need to enumerate every project first
	default:
		if list, err = eng.ListProjects(ctx, *query); err != nil {
			return fail(err.Error())
		}
	}
	if *disc != "" {
		fps, err := eng.Discover(ctx, *disc)
		if err != nil {
			return fail(err.Error())
		}
		if list == nil {
			for _, f := range fps {
				list = append(list, core.Project{ID: f.ProjectID, Number: f.Number})
			}
		} else {
			found := map[string]bool{}
			for _, f := range fps {
				found[f.ProjectID] = true
			}
			before := len(list)
			list = slices.DeleteFunc(list, func(p core.Project) bool { return !found[p.ID] })
			fmt.Fprint(os.Stderr, i18n.Tf("Cloud Asset Inventory: %d de %d proyectos tienen recursos de Compute\n", len(list), before))
		}
		if *projects == "" && *query == "" {
			fmt.Fprint(os.Stderr, i18n.Tf("Cloud Asset Inventory: %d proyectos con recursos de Compute en %s\n", len(list), *disc))
		}
	}
	if len(list) == 0 {
		return fail(i18n.T("no hay proyectos que escanear"))
	}
	fmt.Fprint(os.Stderr, i18n.Tf("Escaneando %d proyectos…\n", len(list)))

	mods := core.Modules{MetricsDays: *metricsDays, Currency: *currency}
	hopt := health.DefaultOptions()
	if prof != nil {
		if !set["modules"] {
			*modules = "none" // the profile's modules replace the default list
			mods = prof.Modules
		}
		if !set["metrics-days"] && prof.Modules.MetricsDays > 0 {
			mods.MetricsDays = prof.Modules.MetricsDays
		}
		if !set["currency"] && prof.Modules.Currency != "" {
			mods.Currency = prof.Modules.Currency
		}
		hopt = prof.Health // zero fields fall back to defaults in health.Evaluate
	}
	if set["disable-rules"] {
		hopt.Disabled = nil
		for _, id := range strings.Split(*disableRules, ",") {
			if id = strings.TrimSpace(id); id != "" {
				hopt.Disabled = append(hopt.Disabled, id)
			}
		}
	}
	if set["billing-table"] || mods.BillingTable == "" {
		mods.BillingTable = *billTable
	}
	if set["billing-month"] || mods.BillingMonth == "" {
		mods.BillingMonth = *billMonth
	}
	if set["billing-max-gb"] || mods.BillingMaxGB == 0 {
		mods.BillingMaxGB = *billMaxGB
	}
	if set["fast"] {
		mods.Fast = *fast
	}
	if mods.Fast && (set["discover"] || mods.FastScope == "") {
		mods.FastScope = *disc
	}
	for _, m := range strings.Split(*modules, ",") {
		switch strings.TrimSpace(m) {
		case "osconfig":
			mods.OSConfig = true
		case "monitoring":
			mods.Monitoring = true
		case "recommender":
			mods.Recommender = true
		case "pricing":
			mods.Pricing = true
		case "none", "":
		default:
			return fail(i18n.Tf("módulo desconocido %q (osconfig, monitoring, recommender, pricing)", m))
		}
	}
	inv, err := eng.Scan(ctx, list, core.ScanOptions{Concurrency: *conc, Modules: mods, Health: hopt, SaveHistory: *saveHistory}, func(p core.Progress) {
		if p.Step == "done" {
			fmt.Fprintf(os.Stderr, "[%d/%d] %-30s %s\n", p.Done, p.Total, p.Project, p.Message)
		}
	})
	if err != nil {
		return fail(err.Error())
	}

	f := export.Format(*format)
	if f == "" {
		f = export.FormatFromPath(*out)
	}
	r := []rune(*sep)
	if len(r) != 1 {
		return fail(i18n.T("--csv-sep debe ser un único carácter"))
	}
	path, err := export.Write(inv, *out, f, r[0])
	if err != nil {
		return fail(i18n.Tf("error al exportar: %v", err))
	}
	if *reportPath != "" {
		ropt := report.Options{Website: update.Website}
		if prev, ok := history.Previous(inv.Meta.StartedAt); ok { // compare with the last stored scan
			if old, err := history.Load(prev.ID); err == nil {
				ropt.Diff = diff.Compare(diff.Input{Inv: old, Label: prev.ID}, diff.Input{Inv: inv, Label: i18n.T("actual")})
				ropt.DiffSince = prev.StartedAt
			}
		}
		if html, err := report.Render(inv, ropt); err != nil {
			fmt.Fprint(os.Stderr, i18n.Tf("aviso: no se pudo generar el informe: %v\n", err))
		} else if err := os.WriteFile(*reportPath, html, 0o600); err != nil {
			fmt.Fprint(os.Stderr, i18n.Tf("aviso: no se pudo guardar el informe: %v\n", err))
		} else {
			fmt.Fprint(os.Stderr, i18n.Tf("Informe ejecutivo: %s\n", *reportPath))
		}
	}
	if *saveHistory {
		label := ""
		if prof != nil {
			label = prof.Name
		}
		if s, err := history.Save(inv, label); err != nil {
			fmt.Fprint(os.Stderr, i18n.Tf("aviso: no se pudo guardar en el historial: %v\n", err))
		} else {
			fmt.Fprint(os.Stderr, i18n.Tf("Guardado en el historial: %s\n", s.ID))
		}
	}
	fmt.Fprint(os.Stderr, i18n.Tf("Inventario: %d VMs, %d errores → %s\n", len(inv.Sheet("gInfo").Rows), len(inv.Errors), path))
	if inv.Meta.Cancelled {
		return 130
	}
	return 0
}

// fail prints an already translated message and returns exit code 1.
func fail(msg string) int {
	fmt.Fprintln(os.Stderr, "error: "+msg)
	return 1
}

// Version prints the version and whether a newer release exists.
func Version() int {
	fmt.Print(i18n.Tf("gceTools %s · licencia %s · %s\n", core.Version, update.License, update.Website))
	info, err := update.Check(context.Background(), core.Version)
	switch {
	case err != nil:
		fmt.Fprint(os.Stderr, i18n.Tf("No se pudo comprobar si hay versiones nuevas: %v\n", err))
	case info.Available:
		fmt.Print(i18n.Tf("Hay una versión nueva: %s → %s\n", info.Latest, info.URL))
	default:
		fmt.Println(i18n.T("Estás en la última versión."))
	}
	return 0
}

// Profiles lists the saved profiles.
func Profiles() int {
	path, _ := profile.Path()
	all, err := profile.List()
	if err != nil {
		return fail(err.Error())
	}
	if len(all) == 0 {
		fmt.Print(i18n.Tf("No hay perfiles guardados. Créalos en la app (Proyectos → Guardar perfil) o edita %s\n", path))
		return 0
	}
	for _, p := range all {
		var what []string
		switch {
		case len(p.Projects) > 0:
			what = append(what, i18n.Tf("%d proyectos", len(p.Projects)))
		case p.Query != "":
			what = append(what, i18n.Tf("filtro %s", p.Query))
		default:
			what = append(what, i18n.T("todos los proyectos visibles"))
		}
		if p.Discover != "" {
			what = append(what, i18n.Tf("descubrimiento CAI en %s", p.Discover))
		}
		if p.Modules.Fast {
			what = append(what, i18n.T("modo rápido"))
		}
		if p.Auth != "" && p.Auth != "adc" {
			what = append(what, "auth "+p.Auth)
		}
		fmt.Printf("%-24s %s\n", p.Name, strings.Join(what, " · "))
	}
	fmt.Printf("\n%s\n", path)
	return 0
}

// RulesList prints the gHealth rules and how to except a resource.
func RulesList() int {
	for _, r := range health.LocalizedRules() {
		param := ""
		if r.Param != "" {
			param = i18n.Tf(" [umbral: %s]", r.Param)
		}
		fmt.Printf("%-24s %-6s %-20s %s%s\n", r.ID, r.Severity, r.Category, r.Title, param)
	}
	fmt.Print(i18n.T(rulesHelp))
	return 0
}

const rulesHelp = `
Desactivar en un escaneo:   gcetools scan --disable-rules snapshot-schedule,no-labels
Excepción por recurso:      label gcetools-ignore=<id> (o gcetools-ignore-<id>=true, o gcetools-ignore=all)
                            en la VM, el disco, el template o el proyecto
`
