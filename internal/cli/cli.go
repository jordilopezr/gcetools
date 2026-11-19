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
	"gcetools/internal/export"
	"gcetools/internal/health"
	"gcetools/internal/profile"
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
  gcetools scan --profile produccion --out inventario.xlsx
  gcetools scan --billing-table finops-prj.billing.gcp_billing_export_resource_v1_XXXX --out inventario.xlsx

Con --profile se usan los valores guardados en la app (Proyectos → Perfil);
las opciones que pases explícitamente tienen prioridad.

Opciones:
`

func Run(args []string) int {
	fs := flag.NewFlagSet("scan", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), usage); fs.PrintDefaults() }
	method := fs.String("auth", "adc", "método: adc | sa | impersonate")
	keyPath := fs.String("key", "", "ruta a la clave JSON de la service account (--auth sa)")
	target := fs.String("impersonate", "", "email de la service account a impersonar (--auth impersonate)")
	projects := fs.String("projects", "", "IDs de proyecto separados por comas (por defecto: todos los visibles)")
	query := fs.String("query", "", `filtro de Resource Manager, p.ej. "parent:folders/123"`)
	out := fs.String("out", "gcetools-"+time.Now().Format("20060102-1504")+".xlsx", "archivo de salida (.xlsx, .zip/.csv, .json)")
	format := fs.String("format", "", "xlsx | csv | json (por defecto según la extensión)")
	sep := fs.String("csv-sep", ",", "separador CSV (usa ';' para Excel en español)")
	conc := fs.Int("concurrency", 8, "proyectos en paralelo")
	rps := fs.Float64("rps", 20, "peticiones por segundo máximas")
	modules := fs.String("modules", "osconfig,monitoring,recommender,pricing", "módulos opcionales separados por comas (\"none\" para desactivarlos)")
	metricsDays := fs.Int("metrics-days", 30, "días de métricas de Cloud Monitoring")
	currency := fs.String("currency", "USD", "moneda de los costes estimados (USD, EUR, CLP…)")
	disc := fs.String("discover", "", "escanea solo proyectos con recursos de Compute según Cloud Asset Inventory (organizations/N, folders/N)")
	profName := fs.String("profile", "", "perfil guardado (gcetools profiles los lista)")
	billTable := fs.String("billing-table", "", "tabla de exportación de facturación en BigQuery (proyecto.dataset.tabla) para el coste real")
	billMonth := fs.String("billing-month", "", "mes de facturación AAAAMM (por defecto, el mes anterior)")
	billMaxGB := fs.Int("billing-max-gb", 50, "no ejecutar la consulta de BigQuery si leería más GB que esto")
	disableRules := fs.String("disable-rules", "", "reglas de gHealth a desactivar, separadas por comas (gcetools rules las lista)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if *billTable != "" {
		if _, _, _, err := billing.ParseTable(*billTable); err != nil {
			return fail("%v", err)
		}
	}
	for _, id := range strings.Split(*disableRules, ",") { // validate before asking for credentials
		if id = strings.TrimSpace(id); id != "" && !slices.ContainsFunc(health.Rules, func(r health.Rule) bool { return r.ID == id }) {
			return fail("regla desconocida %q (gcetools rules las lista)", id)
		}
	}
	var prof *profile.Profile
	if *profName != "" {
		p, err := profile.Get(*profName)
		if err != nil {
			return fail("%v", err)
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
		fmt.Fprintf(os.Stderr, "Perfil: %s\n", p.Name)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	cfg := auth.Config{Method: auth.Method(*method), TargetSA: *target}
	if cfg.Method == auth.KeyFile {
		data, err := os.ReadFile(*keyPath)
		if err != nil {
			return fail("no se pudo leer la clave: %v", err)
		}
		cfg.KeyJSON = data
	}
	sess, err := auth.Connect(ctx, cfg)
	if err != nil {
		return fail("%v", err)
	}
	fmt.Fprintf(os.Stderr, "Conectado como %s (%s)\n", sess.Identity, sess.Method)
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
			return fail("%v", err)
		}
	}
	if *disc != "" {
		fps, err := eng.Discover(ctx, *disc)
		if err != nil {
			return fail("%v", err)
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
			fmt.Fprintf(os.Stderr, "Cloud Asset Inventory: %d de %d proyectos tienen recursos de Compute\n", len(list), before)
		}
		if *projects == "" && *query == "" {
			fmt.Fprintf(os.Stderr, "Cloud Asset Inventory: %d proyectos con recursos de Compute en %s\n", len(list), *disc)
		}
	}
	if len(list) == 0 {
		return fail("no hay proyectos que escanear")
	}
	fmt.Fprintf(os.Stderr, "Escaneando %d proyectos…\n", len(list))

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
			return fail("módulo desconocido %q (osconfig, monitoring, recommender, pricing)", m)
		}
	}
	inv, err := eng.Scan(ctx, list, core.ScanOptions{Concurrency: *conc, Modules: mods, Health: hopt}, func(p core.Progress) {
		if p.Step == "done" {
			fmt.Fprintf(os.Stderr, "[%d/%d] %-30s %s\n", p.Done, p.Total, p.Project, p.Message)
		}
	})
	if err != nil {
		return fail("%v", err)
	}

	f := export.Format(*format)
	if f == "" {
		f = export.FormatFromPath(*out)
	}
	r := []rune(*sep)
	if len(r) != 1 {
		return fail("--csv-sep debe ser un único carácter")
	}
	path, err := export.Write(inv, *out, f, r[0])
	if err != nil {
		return fail("error al exportar: %v", err)
	}
	fmt.Fprintf(os.Stderr, "Inventario: %d VMs, %d errores → %s\n", len(inv.Sheet("gInfo").Rows), len(inv.Errors), path)
	if inv.Meta.Cancelled {
		return 130
	}
	return 0
}

func fail(format string, a ...any) int {
	fmt.Fprintf(os.Stderr, "error: "+format+"\n", a...)
	return 1
}

// Version prints the version and whether a newer release exists.
func Version() int {
	fmt.Printf("gceTools %s · licencia %s · %s\n", core.Version, update.License, update.Website)
	info, err := update.Check(context.Background(), core.Version)
	switch {
	case err != nil:
		fmt.Fprintf(os.Stderr, "No se pudo comprobar si hay versiones nuevas: %v\n", err)
	case info.Available:
		fmt.Printf("Hay una versión nueva: %s → %s\n", info.Latest, info.URL)
	default:
		fmt.Println("Estás en la última versión.")
	}
	return 0
}

// Profiles lists the saved profiles.
func Profiles() int {
	path, _ := profile.Path()
	all, err := profile.List()
	if err != nil {
		return fail("%v", err)
	}
	if len(all) == 0 {
		fmt.Printf("No hay perfiles guardados. Créalos en la app (Proyectos → Guardar perfil) o edita %s\n", path)
		return 0
	}
	for _, p := range all {
		var what []string
		switch {
		case len(p.Projects) > 0:
			what = append(what, fmt.Sprintf("%d proyectos", len(p.Projects)))
		case p.Query != "":
			what = append(what, "filtro "+p.Query)
		default:
			what = append(what, "todos los proyectos visibles")
		}
		if p.Discover != "" {
			what = append(what, "descubrimiento CAI en "+p.Discover)
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
	for _, r := range health.Rules {
		param := ""
		if r.Param != "" {
			param = " [umbral: " + r.Param + "]"
		}
		fmt.Printf("%-24s %-6s %-20s %s%s\n", r.ID, r.Severity, r.Category, r.Title, param)
	}
	fmt.Print(`
Desactivar en un escaneo:   gcetools scan --disable-rules snapshot-schedule,no-labels
Excepción por recurso:      label gcetools-ignore=<id> (o gcetools-ignore-<id>=true, o gcetools-ignore=all)
                            en la VM, el disco, el template o el proyecto
`)
	return 0
}
