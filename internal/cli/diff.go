package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gcetools/internal/diff"
	"gcetools/internal/export"
	"gcetools/internal/model"
)

const diffUsage = `gcetools diff — compara dos snapshots JSON de gceTools

Uso:
  gcetools diff [opciones] anterior.json nuevo.json

Genera un informe con las pestañas Resumen, Altas, Bajas, Cambios y Metadatos.
Los cambios relevantes (VMs redimensionadas, reglas abiertas a Internet, discos
que quedan huérfanos, cambios de service account…) aparecen primero.

Ejemplos:
  gcetools diff lunes.json viernes.json
  gcetools diff --out cambios.xlsx --fail-on-relevant base.json hoy.json

Códigos de salida: 0 correcto · 1 error · 3 hay cambios relevantes (con --fail-on-relevant)

Opciones:
`

const help = `gceTools — inventario de Google Compute Engine

Comandos:
  scan      escanea proyectos y exporta el inventario (gcetools scan --help)
  diff      compara dos snapshots JSON (gcetools diff --help)
  profiles  lista los perfiles de escaneo guardados
  rules     lista las reglas de gHealth y cómo exceptuar recursos
  login     inicia sesión con Google sin gcloud (cliente OAuth de tu organización)
  logout    cierra la sesión de Google guardada en el llavero
  version   muestra la versión y si hay una más reciente
`

// Dispatch runs a console subcommand. ok is false when args name none, so
// the GUI binary can open its window instead.
func Dispatch(args []string) (code int, ok bool) {
	if len(args) == 0 {
		return 0, false
	}
	switch args[0] {
	case "scan":
		return Run(args[1:]), true
	case "diff":
		return Diff(args[1:]), true
	case "profiles":
		return Profiles(), true
	case "rules":
		return RulesList(), true
	case "login":
		return Login(args[1:]), true
	case "logout":
		return Logout(args[1:]), true
	case "version", "--version":
		return Version(), true
	case "help", "--help", "-h":
		fmt.Print(help)
		return 0, true
	}
	return 0, false
}

func Diff(args []string) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.Usage = func() { fmt.Fprint(fs.Output(), diffUsage); fs.PrintDefaults() }
	out := fs.String("out", "gcetools-diff-"+time.Now().Format("20060102-1504")+".xlsx", "archivo de salida (.xlsx, .zip/.csv, .json)")
	format := fs.String("format", "", "xlsx | csv | json (por defecto según la extensión)")
	sep := fs.String("csv-sep", ",", "separador CSV (usa ';' para Excel en español)")
	failRelevant := fs.Bool("fail-on-relevant", false, "termina con código 3 si hay cambios relevantes")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fs.Usage()
		return 2
	}
	var in [2]diff.Input
	for i, p := range fs.Args() {
		inv, err := export.ReadJSON(p)
		if err != nil {
			return fail("%v", err)
		}
		in[i] = diff.Input{Inv: inv, Label: filepath.Base(p)}
	}
	res := diff.Compare(in[0], in[1])

	f := export.Format(*format)
	if f == "" {
		f = export.FormatFromPath(*out)
	}
	r := []rune(*sep)
	if len(r) != 1 {
		return fail("--csv-sep debe ser un único carácter")
	}
	path, err := export.Write(res, *out, f, r[0])
	if err != nil {
		return fail("error al exportar: %v", err)
	}
	var added, removed, modified, relevant int64
	for _, row := range res.Sheet(diff.SSummary).Rows {
		added += row["added"].(int64)
		removed += row["removed"].(int64)
		modified += row["modified"].(int64)
		relevant += row["relevant"].(int64)
	}
	fmt.Fprintf(os.Stderr, "%d altas, %d bajas, %d recursos modificados (%d cambios relevantes) → %s\n",
		added, removed, modified, relevant, path)
	printRelevant(res)
	if *failRelevant && relevant > 0 {
		return 3
	}
	return 0
}

// printRelevant lists up to 20 relevant changes so cron mails are useful.
func printRelevant(res *model.Inventory) {
	n := 0
	for _, s := range []string{diff.SAdded, diff.SChanged} {
		for _, r := range res.Sheet(s).Rows {
			if r["relevant"] != true {
				continue
			}
			if n++; n > 20 {
				fmt.Fprintln(os.Stderr, "  …")
				return
			}
			line := fmt.Sprintf("  ! %s: %s/%s", r["reason"], r["project"], r["resource"])
			if s == diff.SChanged {
				line += fmt.Sprintf(" (%s: %q → %q)", r["field"], r["before"], r["after"])
			}
			fmt.Fprintln(os.Stderr, line)
		}
	}
}
