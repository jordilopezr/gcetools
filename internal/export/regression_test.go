package export

import (
	"archive/zip"
	"gcetools/internal/schema"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gcetools/internal/model"

	"github.com/xuri/excelize/v2"
)

// Text that a spreadsheet would run as a formula is neutralized in CSV.
func TestCSVFormulaInjection(t *testing.T) {
	inv := schema.NewInventory()
	inv.Sheet(schema.SVPC).Add(model.Row{"project": "p1", "name": "net", "description": `=HYPERLINK("http://x","clic")`})
	inv.Sheet(schema.SVPC).Add(model.Row{"project": "p1", "name": "@SUM(A1)", "description": "-2+3"})
	path, err := Write(inv, filepath.Join(t.TempDir(), "inv.zip"), CSV, ',')
	if err != nil {
		t.Fatal(err)
	}
	csv := readZipFile(t, path, schema.SVPC+".csv")
	for _, bad := range []string{`,=HYPERLINK`, `,@SUM`, `,-2+3`, `"=HYPERLINK`} {
		if strings.Contains(csv, bad) {
			t.Errorf("unescaped formula %q in %q", bad, csv)
		}
	}
	for _, good := range []string{`'=HYPERLINK`, `'@SUM(A1)`, `'-2+3`} {
		if !strings.Contains(csv, good) {
			t.Errorf("missing %q in %q", good, csv)
		}
	}
	if got := cellString(int64(-5)); got != "-5" {
		t.Errorf("negative numbers must stay numbers: %q", got)
	}
}

// Dates survive a JSON snapshot round trip into XLSX (they used to be blank).
func TestXLSXDatesAfterJSONRoundTrip(t *testing.T) {
	dir := t.TempDir()
	snap := filepath.Join(dir, "snap.json")
	if _, err := Write(sample(), snap, JSON, ','); err != nil {
		t.Fatal(err)
	}
	inv, err := ReadJSON(snap)
	if err != nil {
		t.Fatal(err)
	}
	xlsx := filepath.Join(dir, "inv.xlsx")
	if _, err := Write(inv, xlsx, XLSX, ','); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(xlsx)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	rows, err := f.GetRows(schema.SInfo)
	if err != nil {
		t.Fatal(err)
	}
	col := -1
	for i, h := range rows[0] {
		if h == inv.Sheet(schema.SInfo).Columns[colIndex(inv.Sheet(schema.SInfo), "lastStop")].Title {
			col = i
		}
	}
	if col < 0 || col >= len(rows[1]) || rows[1][col] == "" {
		t.Fatalf("lastStop is blank after the JSON round trip: %v", rows[1])
	}
	if _, ok := inv.Sheet(schema.SInfo).Rows[0]["lastStop"].(time.Time); !ok {
		t.Errorf("lastStop not restored as time.Time")
	}
}

func TestExportsAreOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permissions")
	}
	dir := t.TempDir()
	for _, f := range []Format{XLSX, CSV, JSON} {
		path := filepath.Join(dir, "inv."+string(f))
		if err := os.WriteFile(path, nil, 0o644); err != nil { // existing file keeps no wider mode
			t.Fatal(err)
		}
		got, err := Write(sample(), path, f, ',')
		if err != nil {
			t.Fatal(err)
		}
		if st, _ := os.Stat(got); st.Mode().Perm() != 0o600 {
			t.Errorf("%s: mode %v, want 0600", f, st.Mode().Perm())
		}
	}
	report := filepath.Join(dir, "r.html")
	if err := WritePrivate(report, []byte("x")); err != nil {
		t.Fatal(err)
	}
	if st, _ := os.Stat(report); st.Mode().Perm() != 0o600 {
		t.Errorf("report mode %v", st.Mode().Perm())
	}
}

func colIndex(s *model.Sheet, key string) int {
	for i, c := range s.Columns {
		if c.Key == key {
			return i
		}
	}
	return -1
}

func readZipFile(t *testing.T, path, name string) string {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name == name {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			rc.Close()
			return string(b)
		}
	}
	t.Fatalf("%s not in zip", name)
	return ""
}

// Every sheet keeps its header filter (now a table written on the stream)
// and repeated column titles do not break the table.
func TestXLSXFiltersAndUniqueHeaders(t *testing.T) {
	inv := sample()
	s := inv.Sheet(schema.SInfo)
	s.Columns = append(s.Columns, model.Column{Key: "dup", Title: s.Columns[0].Title, Type: model.TString})
	path := filepath.Join(t.TempDir(), "inv.xlsx")
	if _, err := Write(inv, path, XLSX, ','); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	for _, sh := range inv.Sheets {
		tables, err := f.GetTables(sh.Name)
		if err != nil || len(tables) != 1 {
			t.Errorf("%s: tables = %v, %v", sh.Name, tables, err)
		}
	}
	hdr, _ := f.GetRows(schema.SInfo)
	last := hdr[0][len(hdr[0])-1]
	if last != s.Columns[0].Title+" (2)" {
		t.Errorf("duplicate title = %q", last)
	}
}

func TestUniqueTitles(t *testing.T) {
	got := uniqueTitles([]model.Column{{Title: "VPC"}, {Title: "vpc"}, {Key: "k"}, {Title: "VPC"}})
	want := []string{"VPC", "vpc (2)", "k", "VPC (3)"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}
