// Package export writes an inventory to XLSX (one worksheet per sheet),
// CSV (one file per sheet inside a ZIP) or JSON (full snapshot, the basis
// for comparing two scans later).
package export

import (
	"archive/zip"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gcetools/internal/model"

	"github.com/xuri/excelize/v2"
)

type Format string

const (
	XLSX Format = "xlsx"
	CSV  Format = "csv"
	JSON Format = "json"
)

// FormatFromPath infers the format from the file extension.
func FormatFromPath(path string) Format {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".csv", ".zip":
		return CSV
	case ".json":
		return JSON
	}
	return XLSX
}

// Write exports inv and returns the path actually written (CSV always ends in .zip).
func Write(inv *model.Inventory, path string, f Format, csvSep rune) (string, error) {
	switch f {
	case XLSX:
		return path, writeXLSX(inv, path)
	case CSV:
		if !strings.EqualFold(filepath.Ext(path), ".zip") {
			path = strings.TrimSuffix(path, filepath.Ext(path)) + ".zip"
		}
		return path, writeCSVZip(inv, path, csvSep)
	case JSON:
		return path, writeJSON(inv, path)
	}
	return "", fmt.Errorf("formato no soportado: %s", f)
}

// ---------- XLSX ----------

func writeXLSX(inv *model.Inventory, path string) error {
	f := excelize.NewFile()
	defer f.Close()

	header, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF"},
		Fill:      excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"0B57D0"}},
		Alignment: &excelize.Alignment{Vertical: "center"},
	})
	if err != nil {
		return err
	}
	dateFmt := "yyyy-mm-dd hh:mm"
	dateStyle, _ := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	gbStyle, _ := f.NewStyle(&excelize.Style{NumFmt: 2}) // 0.00

	for i, sh := range inv.Sheets {
		if i == 0 {
			if err := f.SetSheetName("Sheet1", sh.Name); err != nil {
				return err
			}
		} else if _, err := f.NewSheet(sh.Name); err != nil {
			return err
		}
		sw, err := f.NewStreamWriter(sh.Name)
		if err != nil {
			return err
		}
		for ci, col := range sh.Columns {
			w := col.Width
			if w == 0 {
				w = 16
			}
			if err := sw.SetColWidth(ci+1, ci+1, w); err != nil {
				return err
			}
		}
		if err := sw.SetPanes(&excelize.Panes{Freeze: true, YSplit: 1, TopLeftCell: "A2", ActivePane: "bottomLeft"}); err != nil {
			return err
		}
		hdr := make([]any, len(sh.Columns))
		for ci, col := range sh.Columns {
			hdr[ci] = excelize.Cell{StyleID: header, Value: col.Title}
		}
		if err := sw.SetRow("A1", hdr, excelize.RowOpts{Height: 20}); err != nil {
			return err
		}
		for ri, row := range sh.Rows {
			vals := make([]any, len(sh.Columns))
			for ci, col := range sh.Columns {
				v := row[col.Key]
				switch col.Type {
				case model.TTime:
					if t, ok := v.(time.Time); ok {
						vals[ci] = excelize.Cell{StyleID: dateStyle, Value: t.Local()}
					}
				case model.TGB, model.TMoney:
					if v != nil {
						vals[ci] = excelize.Cell{StyleID: gbStyle, Value: v}
					}
				case model.TBool:
					if b, ok := v.(bool); ok {
						vals[ci] = map[bool]string{true: "Sí", false: "No"}[b]
					}
				default:
					vals[ci] = v
				}
			}
			cell, _ := excelize.CoordinatesToCellName(1, ri+2)
			if err := sw.SetRow(cell, vals); err != nil {
				return err
			}
		}
		if err := sw.Flush(); err != nil {
			return err
		}
		lastCol, _ := excelize.ColumnNumberToName(len(sh.Columns))
		rng := fmt.Sprintf("A1:%s%d", lastCol, len(sh.Rows)+1)
		if err := f.AutoFilter(sh.Name, rng, nil); err != nil {
			return err
		}
	}
	// Not f.SaveAs: excelize rejects paths longer than 207 characters, which
	// deep folders (and most Windows profiles with OneDrive) easily exceed.
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	if _, err := f.WriteTo(out); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// ---------- CSV (zip) ----------

func writeCSVZip(inv *model.Inventory, path string, sep rune) error {
	if sep == 0 {
		sep = ','
	}
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	zw := zip.NewWriter(out)
	for _, sh := range inv.Sheets {
		w, err := zw.Create(sh.Name + ".csv")
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte{0xEF, 0xBB, 0xBF}); err != nil { // BOM: Excel opens UTF-8 correctly
			return err
		}
		cw := csv.NewWriter(w)
		cw.Comma = sep
		hdr := make([]string, len(sh.Columns))
		for i, col := range sh.Columns {
			hdr[i] = col.Title
		}
		if err := cw.Write(hdr); err != nil {
			return err
		}
		rec := make([]string, len(sh.Columns))
		for _, row := range sh.Rows {
			for i, col := range sh.Columns {
				rec[i] = cellString(row[col.Key])
			}
			if err := cw.Write(rec); err != nil {
				return err
			}
		}
		cw.Flush()
		if err := cw.Error(); err != nil {
			return err
		}
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return out.Close()
}

func cellString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case string:
		return x
	case bool:
		if x {
			return "true"
		}
		return "false"
	case int64:
		return strconv.FormatInt(x, 10)
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case time.Time:
		return x.Local().Format("2006-01-02 15:04:05")
	}
	return fmt.Sprint(v)
}

// ---------- JSON ----------

func writeJSON(inv *model.Inventory, path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	if err := enc.Encode(inv); err != nil {
		return err
	}
	return out.Close()
}

// ReadJSON loads a snapshot written by Write(..., JSON). Row values come back
// as JSON types (dates as RFC 3339 strings, numbers as float64).
func ReadJSON(path string) (*model.Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var inv model.Inventory
	if err := json.NewDecoder(f).Decode(&inv); err != nil {
		return nil, fmt.Errorf("%s no es un snapshot JSON de gceTools: %w", filepath.Base(path), err)
	}
	if inv.Meta.Tool == "" && len(inv.Sheets) == 0 {
		return nil, fmt.Errorf("%s no es un snapshot JSON de gceTools", filepath.Base(path))
	}
	return &inv, nil
}
