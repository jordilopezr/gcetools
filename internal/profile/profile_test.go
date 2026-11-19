package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"gcetools/internal/core"
)

func tempStore(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gcetools", "profiles.json")
	old := Path
	Path = func() (string, error) { return p, nil }
	t.Cleanup(func() { Path = old })
	return p
}

func TestCRUD(t *testing.T) {
	path := tempStore(t)
	if all, err := List(); err != nil || len(all) != 0 {
		t.Fatalf("empty store: %v %v", all, err)
	}
	if err := Save(Profile{Name: "  "}); err == nil {
		t.Error("blank name must fail")
	}
	if err := Save(Profile{Name: "x", Discover: "org/1"}); err == nil {
		t.Error("bad scope must fail")
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(Save(Profile{Name: "Producción", Query: "parent:folders/1", Modules: core.Modules{Pricing: true, Currency: "EUR"}}))
	must(Save(Profile{Name: "backup", Projects: []string{"a", "b"}, Discover: "folders/115617811679"}))
	must(Save(Profile{Name: "producción", Query: "parent:folders/2"})) // same name, other case: replaces

	all, err := List()
	must(err)
	if len(all) != 2 || all[0].Name != "backup" || all[1].Query != "parent:folders/2" {
		t.Fatalf("list = %+v", all)
	}
	p, err := Get("BACKUP")
	must(err)
	if len(p.Projects) != 2 || p.Updated.IsZero() {
		t.Errorf("get = %+v", p)
	}
	if _, err := Get("nada"); err == nil || !strings.Contains(err.Error(), "backup") {
		t.Errorf("missing profile error should list names: %v", err)
	}
	must(Delete("backup"))
	if all, _ := List(); len(all) != 1 {
		t.Errorf("after delete: %+v", all)
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		must(err)
		if st.Mode().Perm() != 0o600 {
			t.Errorf("profiles.json mode = %v, want 0600", st.Mode().Perm())
		}
	}
}

func TestCorruptFile(t *testing.T) {
	path := tempStore(t)
	os.MkdirAll(filepath.Dir(path), 0o700)
	os.WriteFile(path, []byte("{no es json"), 0o600)
	if _, err := List(); err == nil {
		t.Error("corrupt file must be reported, not silently overwritten")
	}
}
