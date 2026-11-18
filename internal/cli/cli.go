// Package cli implements the headless "scan" command so inventories can be
// scheduled (cron / Task Scheduler) without opening the GUI.
package cli

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/core"
	"gcetools/internal/export"
)

const usage = `gcetools scan — inventario de Compute Engine sin interfaz gráfica

Uso:
  gcetools scan [opciones]

Ejemplos:
  gcetools scan --out inventario.xlsx
  gcetools scan --auth sa --key ./sa.json --query "parent:folders/123456" --out inv.zip
  gcetools scan --auth impersonate --impersonate inventory@prj.iam.gserviceaccount.com --projects a,b

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
	if err := fs.Parse(args); err != nil {
		return 2
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
	if *projects != "" {
		for _, id := range strings.Split(*projects, ",") {
			if id = strings.TrimSpace(id); id != "" {
				list = append(list, core.Project{ID: id})
			}
		}
	} else {
		if list, err = eng.ListProjects(ctx, *query); err != nil {
			return fail("%v", err)
		}
	}
	if len(list) == 0 {
		return fail("no hay proyectos que escanear")
	}
	fmt.Fprintf(os.Stderr, "Escaneando %d proyectos…\n", len(list))

	mods := core.Modules{MetricsDays: *metricsDays, Currency: *currency}
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
	inv, err := eng.Scan(ctx, list, core.ScanOptions{Concurrency: *conc, Modules: mods}, func(p core.Progress) {
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
