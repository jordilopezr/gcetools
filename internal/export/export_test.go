package export

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gcetools/internal/collect"
	"gcetools/internal/health"
	"gcetools/internal/model"

	"github.com/xuri/excelize/v2"
)

func sample() *model.Inventory {
	inv := collect.NewInventory()
	now := time.Date(2026, 11, 18, 10, 0, 0, 0, time.UTC)
	inv.Sheet(collect.SInfo).Add(model.Row{
		"project": "p1", "name": "vm-1", "zone": "europe-west1-b", "status": "TERMINATED", "vcpu": int64(4),
		"memoryGB": 16.0, "lastStop": now.Add(-60 * 24 * time.Hour), "externalIP": "34.1.2.3",
		"serviceAccount": "123-compute@developer.gserviceaccount.com", "scopes": "cloud-platform",
		"family": "n1", "secureBoot": false, "deletionProtection": false,
	})
	inv.Sheet(collect.SDisks).Add(model.Row{"project": "p1", "name": "orphan", "sizeGB": 500.0, "type": "pd-ssd", "orphan": true})
	inv.Sheet(collect.SFirewall).Add(model.Row{"project": "p1", "name": "allow-ssh", "rules": "tcp:22", "openToInternet": true, "sourceRanges": "0.0.0.0/0"})
	inv.Sheet(collect.SAddress).Add(model.Row{"project": "p1", "name": "ip", "status": "RESERVED", "type": "EXTERNAL"})
	health.Evaluate(inv, health.DefaultOptions(), now)
	return inv
}

func TestHealthFindings(t *testing.T) {
	rows := sample().Sheet(collect.SHealth).Rows
	want := []string{"Disco huérfano", "SSH/RDP o todos los puertos abiertos a Internet", "VM con IP pública",
		"IP externa reservada sin uso", "VM parada más de 30 días", "Usa la service account por defecto de Compute"}
	got := map[string]string{}
	for _, r := range rows {
		got[r["rule"].(string)] = r["severity"].(string)
	}
	for _, w := range want {
		if _, ok := got[w]; !ok {
			t.Errorf("missing finding %q; got %v", w, got)
		}
	}
	if rows[0]["severity"] != health.High {
		t.Errorf("findings not sorted by severity: first is %v", rows[0]["severity"])
	}
}

func TestXLSX(t *testing.T) {
	inv := sample()
	path := filepath.Join(t.TempDir(), "inv.xlsx")
	if _, err := Write(inv, path, XLSX, ','); err != nil {
		t.Fatal(err)
	}
	f, err := excelize.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if n := len(f.GetSheetList()); n != len(inv.Sheets) {
		t.Fatalf("sheets = %d, want %d", n, len(inv.Sheets))
	}
	rows, _ := f.GetRows(collect.SInfo)
	if len(rows) != 2 || rows[0][0] != "Proyecto" || rows[1][1] != "vm-1" {
		t.Fatalf("unexpected gInfo rows: %v", rows)
	}
}

func TestCSVZip(t *testing.T) {
	path, err := Write(sample(), filepath.Join(t.TempDir(), "inv.csv"), CSV, ';')
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(path, ".zip") {
		t.Fatalf("path = %s, want .zip", path)
	}
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()
	for _, f := range zr.File {
		if f.Name != collect.SDisks+".csv" {
			continue
		}
		rc, _ := f.Open()
		b, _ := io.ReadAll(rc)
		rc.Close()
		if !bytes.HasPrefix(b, []byte{0xEF, 0xBB, 0xBF}) || !strings.Contains(string(b), "p1;orphan;") {
			t.Fatalf("unexpected csv: %q", b)
		}
		return
	}
	t.Fatal("gDisks.csv not found in zip")
}

func TestXLSXLongPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), strings.Repeat("carpeta-muy-larga-", 6), strings.Repeat("subcarpeta-", 6))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "inventario-con-un-nombre-bastante-largo.xlsx")
	if len(path) <= 207 {
		t.Fatalf("test path too short: %d", len(path))
	}
	if _, err := Write(collect.NewInventory(), path, XLSX, ','); err != nil {
		t.Fatalf("long path: %v", err)
	}
}
