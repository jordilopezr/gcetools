// Package history keeps every scan as a compressed JSON snapshot plus a small
// index of summaries, so the app can list scans, draw trends and compare any
// two without the user exporting files by hand.
//
// Layout (Dir): index.json and one <id>.json.gz per scan. The index is only
// a cache: it is rebuilt from the snapshots if it goes missing.
package history

import (
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"gcetools/internal/health"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
	"sync"
	"time"

	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Keep is how many scans are kept; older ones are deleted on Save.
var Keep = 100

// Summary is one row of the history list and one point of the trends.
type Summary struct {
	ID         string    `json:"id"`
	StartedAt  time.Time `json:"startedAt"`
	Identity   string    `json:"identity"`
	Version    string    `json:"version"`
	Profile    string    `json:"profile"`
	Projects   int       `json:"projects"`
	ProjectsOK int       `json:"projectsOk"`
	Cancelled  bool      `json:"cancelled"`
	VMs        int       `json:"vms"`
	Running    int       `json:"running"`
	VCPU       float64   `json:"vcpu"`
	MemoryGB   float64   `json:"memoryGB"`
	Disks      int       `json:"disks"`
	DiskGB     float64   `json:"diskGB"`
	Orphans    int       `json:"orphans"`
	Currency   string    `json:"currency"`
	CostList   float64   `json:"costList"` // list-price estimate per month (gCost TOTAL)
	CostReal   float64   `json:"costReal"` // net real cost per month (gCostReal TOTAL), 0 = not read
	Savings    float64   `json:"savings"`  // Recommender monthly savings
	High       int       `json:"high"`     // gHealth findings by severity
	Medium     int       `json:"medium"`
	Low        int       `json:"low"`
	SizeBytes  int64     `json:"sizeBytes"`
}

// Dir is where history lives: the per-user data directory
// (~/.local/share/gcetools/history, %LocalAppData%\gcetools\history,
// ~/Library/Application Support/gcetools/history). Overridable for tests.
var Dir = func() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("LOCALAPPDATA")
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_DATA_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".local", "share")
		}
	}
	if base == "" {
		return "", errors.New(i18n.T("no se encontró la carpeta de datos del usuario"))
	}
	return filepath.Join(base, "gcetools", "history"), nil
}

var (
	mu   sync.Mutex // serializes index updates within the process
	reID = regexp.MustCompile(`^\d{8}-\d{6}(-\d+)?$`)
)

// Summarize computes the summary of an inventory (also for one not saved).
func Summarize(inv *model.Inventory) Summary {
	m := inv.Meta
	s := Summary{StartedAt: m.StartedAt, Identity: m.Identity, Version: m.Version, Projects: m.Projects,
		ProjectsOK: m.ProjectsOK, Cancelled: m.Cancelled, Currency: m.Currency}
	for _, r := range inv.Rows("gInfo") {
		s.VMs++
		if r["status"] == "RUNNING" {
			s.Running++
		}
		s.VCPU += model.Num(r["vcpu"])
		s.MemoryGB += model.Num(r["memoryGB"])
	}
	for _, r := range inv.Rows("gDisks") {
		s.Disks++
		s.DiskGB += model.Num(r["sizeGB"])
		if r["orphan"] == true {
			s.Orphans++
		}
	}
	// Sum the projects: gCost only has a TOTAL row when several were scanned.
	for _, r := range inv.Rows("gCost") {
		if r["project"] != "TOTAL" {
			s.CostList += model.Num(r["total"])
		}
	}
	for _, r := range inv.Rows("gCostReal") {
		if r["project"] == "TOTAL" {
			s.CostReal = model.Num(r["net"])
		}
	}
	for _, r := range inv.Rows("gRecommend") {
		if v := model.Num(r["savingsMonthly"]); v > 0 {
			s.Savings += v
		}
	}
	for _, r := range inv.Rows("gHealth") {
		switch health.Level(r.Str("severity")) { // either language
		case health.High:
			s.High++
		case health.Medium:
			s.Medium++
		case health.Low:
			s.Low++
		}
	}
	s.MemoryGB = round2(s.MemoryGB)
	s.DiskGB = round2(s.DiskGB)
	s.CostList = round2(s.CostList)
	s.CostReal = round2(s.CostReal)
	s.Savings = round2(s.Savings)
	return s
}

// Save stores inv and returns its summary. profile is informative ("" ok).
func Save(inv *model.Inventory, profile string) (Summary, error) {
	dir, err := Dir()
	if err != nil {
		return Summary{}, err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return Summary{}, err
	}
	mu.Lock()
	defer mu.Unlock()

	s := Summarize(inv)
	s.Profile = profile
	t := inv.Meta.StartedAt
	if t.IsZero() {
		t = time.Now()
	}
	base := t.Local().Format("20060102-150405")
	s.ID = base
	for i := 2; fileExists(filepath.Join(dir, s.ID+".json.gz")); i++ {
		s.ID = fmt.Sprintf("%s-%d", base, i)
	}
	path := filepath.Join(dir, s.ID+".json.gz")
	if err := writeGz(path, inv); err != nil {
		os.Remove(path)
		return Summary{}, err
	}
	if st, err := os.Stat(path); err == nil {
		s.SizeBytes = st.Size()
	}

	idx := indexOrRebuild(dir)
	idx = slices.DeleteFunc(idx, func(o Summary) bool { return o.ID == s.ID }) // rebuild may have found it
	idx = append(idx, s)
	sortNewestFirst(idx)
	for len(idx) > Keep && Keep > 0 {
		old := idx[len(idx)-1]
		os.Remove(filepath.Join(dir, old.ID+".json.gz"))
		idx = idx[:len(idx)-1]
	}
	return s, writeIndex(dir, idx)
}

// List returns the summaries, newest first. A missing or damaged index is
// rebuilt from the snapshot files.
func List() ([]Summary, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	mu.Lock()
	defer mu.Unlock()
	idx, err := readIndex(dir)
	if err == nil {
		return idx, nil
	}
	if errors.Is(err, os.ErrNotExist) && !fileExists(dir) {
		return []Summary{}, nil
	}
	return rebuild(dir)
}

// Get returns the index entry of one stored scan.
func Get(id string) (Summary, error) {
	all, err := List()
	if err != nil {
		return Summary{}, err
	}
	for _, s := range all {
		if s.ID == id {
			return s, nil
		}
	}
	return Summary{}, fmt.Errorf(i18n.T("no existe el escaneo %s en el historial"), id)
}

// Load reads one stored inventory.
func Load(id string) (*model.Inventory, error) {
	if !reID.MatchString(id) {
		return nil, fmt.Errorf(i18n.T("identificador de historial no válido: %q"), id)
	}
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	return readGz(filepath.Join(dir, id+".json.gz"))
}

// Delete removes one stored scan.
func Delete(id string) error {
	if !reID.MatchString(id) {
		return fmt.Errorf(i18n.T("identificador de historial no válido: %q"), id)
	}
	dir, err := Dir()
	if err != nil {
		return err
	}
	mu.Lock()
	defer mu.Unlock()
	if err := os.Remove(filepath.Join(dir, id+".json.gz")); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	idx := indexOrRebuild(dir)
	return writeIndex(dir, slices.DeleteFunc(idx, func(s Summary) bool { return s.ID == id }))
}

// Previous returns the newest stored scan older than t, if any.
func Previous(t time.Time) (Summary, bool) {
	all, err := List()
	if err != nil {
		return Summary{}, false
	}
	for _, s := range all { // newest first
		if s.StartedAt.Before(t) && !s.Cancelled {
			return s, true
		}
	}
	return Summary{}, false
}

func rebuild(dir string) ([]Summary, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := []Summary{}
	for _, e := range entries {
		id, ok := strings.CutSuffix(e.Name(), ".json.gz")
		if !ok || !reID.MatchString(id) {
			continue
		}
		inv, err := readGz(filepath.Join(dir, e.Name()))
		if err != nil {
			continue // unreadable snapshot: skip, do not fail the list
		}
		s := Summarize(inv)
		s.ID = id
		if info, err := e.Info(); err == nil {
			s.SizeBytes = info.Size()
		}
		out = append(out, s)
	}
	sortNewestFirst(out)
	return out, writeIndex(dir, out)
}

// indexOrRebuild returns the index, rebuilding it from the snapshot files
// when it is missing or damaged, so Save and Delete never rewrite it with a
// partial list (which would hide, and never prune, the older scans).
func indexOrRebuild(dir string) []Summary {
	if idx, err := readIndex(dir); err == nil {
		return idx
	}
	idx, _ := rebuild(dir)
	return idx
}

// readIndex reads index.json. Entries with an invalid ID are dropped: the ID
// becomes a file name when pruning or deleting.
func readIndex(dir string) ([]Summary, error) {
	data, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		return nil, err
	}
	var idx []Summary
	if err := json.Unmarshal(data, &idx); err != nil {
		return nil, err
	}
	return slices.DeleteFunc(append([]Summary{}, idx...), func(s Summary) bool { return !reID.MatchString(s.ID) }), nil
}

func writeIndex(dir string, idx []Summary) error {
	data, err := json.MarshalIndent(idx, "", " ")
	if err != nil {
		return err
	}
	tmp := filepath.Join(dir, "index.json.tmp")
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, filepath.Join(dir, "index.json"))
}

func writeGz(path string, inv *model.Inventory) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	zw := gzip.NewWriter(f)
	if err := json.NewEncoder(zw).Encode(inv); err != nil {
		f.Close()
		return err
	}
	if err := zw.Close(); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

func readGz(path string) (*model.Inventory, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		return nil, err
	}
	defer zr.Close()
	var inv model.Inventory
	if err := json.NewDecoder(zr).Decode(&inv); err != nil {
		return nil, err
	}
	model.Normalize(&inv)
	return &inv, nil
}

func sortNewestFirst(s []Summary) {
	slices.SortFunc(s, func(a, b Summary) int { return b.StartedAt.Compare(a.StartedAt) })
}

func fileExists(p string) bool { _, err := os.Stat(p); return err == nil }

func round2(f float64) float64 { return math.Round(f*100) / 100 }
