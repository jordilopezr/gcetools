package i18n

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestSwitch(t *testing.T) {
	defer SetLang(ES)
	add(map[string]string{"__prueba %d": "__test %d"})
	if Tf("__prueba %d", 3) != "__prueba 3" {
		t.Error("Spanish must return the source text")
	}
	SetLang("en_US.UTF-8")
	if Lang() != EN || Tf("__prueba %d", 3) != "__test 3" || T("__sin traducir") != "__sin traducir" {
		t.Error("English lookup or fallback failed")
	}
	if Normalize("es-CL") != ES || Normalize("de_DE") != EN || Normalize("C") != "" {
		t.Error("Normalize")
	}
}

// TestCoverage fails when a string passed to T or Tf anywhere in the module
// has no English translation, or when a translation keeps a different set
// of format verbs than its source.
func TestCoverage(t *testing.T) {
	root := filepath.Join("..", "..")
	fset := token.NewFileSet()
	missing := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && (d.Name() == "node_modules" || d.Name() == "frontend" || d.Name() == "build" || strings.HasPrefix(d.Name(), ".")) {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			name := ""
			switch fn := call.Fun.(type) {
			case *ast.SelectorExpr:
				if x, ok := fn.X.(*ast.Ident); ok && x.Name == "i18n" {
					name = fn.Sel.Name
				}
			case *ast.Ident:
				if f.Name.Name == "i18n" {
					name = fn.Name
				}
			}
			if name != "T" && name != "Tf" {
				return true
			}
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			s, _ := strconv.Unquote(lit.Value)
			if _, ok := en[s]; !ok {
				missing[s] = fset.Position(lit.Pos()).String()
			}
			return true
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for s, pos := range missing {
		t.Errorf("%s: sin traducción: %q", pos, s)
	}
	for k, v := range en {
		if verbs(k) != verbs(v) {
			t.Errorf("verbos distintos: %q → %q", k, v)
		}
	}
}

// verbs lists the fmt verbs of s in order ("%d,%s").
func verbs(s string) string {
	var out []string
	for i := 0; i < len(s); i++ {
		if s[i] != '%' {
			continue
		}
		j := i + 1
		for j < len(s) && strings.ContainsRune("+-# 0123456789.*", rune(s[j])) {
			j++
		}
		if j < len(s) {
			if s[j] != '%' {
				out = append(out, s[i:j+1])
			}
			i = j
		}
	}
	return strings.Join(out, ",")
}
