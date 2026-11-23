package collect

import (
	"go/ast"
	"go/parser"
	"go/token"
	"regexp"
	"strings"
	"testing"
)

// Every Instance field (in.X) and attached-disk field (ad.X) the collectors
// read must be in instanceFields, or the Compute API leaves it out and the
// column comes back empty.
func TestInstanceFieldMask(t *testing.T) {
	top, disk := maskFields(t)
	fset := token.NewFileSet()
	for _, file := range []string{"compute.go", "security.go"} {
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			field := strings.ToLower(sel.Sel.Name[:1]) + sel.Sel.Name[1:]
			switch {
			case id.Name == "in" && !top[field]:
				t.Errorf("%s: in.%s is not in instanceFields", fset.Position(sel.Pos()), sel.Sel.Name)
			case id.Name == "ad" && !disk[field]:
				t.Errorf("%s: ad.%s is not in the disks(...) part of instanceFields", fset.Position(sel.Pos()), sel.Sel.Name)
			}
			return true
		})
	}
}

// maskFields splits instanceFields into top-level and disks(...) names.
func maskFields(t *testing.T) (top, disk map[string]bool) {
	m := regexp.MustCompile(`instances\((.*)\)$`).FindStringSubmatch(instanceFields)
	if m == nil {
		t.Fatalf("unexpected mask %q", instanceFields)
	}
	inner := m[1]
	d := regexp.MustCompile(`disks\(([^)]*)\)`).FindStringSubmatch(inner)
	if d == nil {
		t.Fatal("no disks(...) in the mask")
	}
	top, disk = map[string]bool{"disks": true}, map[string]bool{}
	for _, f := range strings.Split(strings.Replace(inner, d[0], "", 1), ",") {
		if f = strings.TrimSpace(f); f != "" {
			top[f] = true
		}
	}
	for _, f := range strings.Split(d[1], ",") {
		disk[strings.TrimSpace(f)] = true
	}
	return top, disk
}
