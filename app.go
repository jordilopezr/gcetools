package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/core"
	"gcetools/internal/diff"
	"gcetools/internal/export"
	"gcetools/internal/health"
	"gcetools/internal/model"
	"gcetools/internal/profile"
	"gcetools/internal/update"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the surface exposed to the frontend. Every exported method becomes
// a TypeScript function in frontend/wailsjs/go/main/App.
type App struct {
	ctx context.Context

	mu       sync.Mutex
	keyJSON  []byte // service account key, memory only
	keyPath  string // where it was read from (saved in profiles; never the content)
	engine   *core.Engine
	projects map[string]core.Project
	inv      *model.Inventory
	cancel   context.CancelFunc
	diff     *model.Inventory            // last comparison
	snaps    map[string]*model.Inventory // snapshots loaded for comparison, by path
}

func NewApp() *App {
	return &App{projects: map[string]core.Project{}, snaps: map[string]*model.Inventory{}}
}

func (a *App) startup(ctx context.Context) { a.ctx = ctx }

func (a *App) shutdown(ctx context.Context) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	clear(a.keyJSON)
}

type ADCStatus struct {
	Found    bool   `json:"found"`
	Identity string `json:"identity"`
}

func (a *App) Version() string { return core.Version }

func (a *App) DetectADC() ADCStatus {
	found, id := auth.DetectADC(a.ctx)
	return ADCStatus{Found: found, Identity: id}
}

// PickServiceAccountKey opens a file dialog, validates the key and keeps it
// in memory. Only the public identity is returned to the UI.
func (a *App) PickServiceAccountKey() (*auth.KeyInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Selecciona la clave JSON de la service account",
		Filters: []runtime.FileFilter{{DisplayName: "Clave JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	info, err := auth.ParseKey(data)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	clear(a.keyJSON)
	a.keyJSON = data
	a.keyPath = path
	a.mu.Unlock()
	return &info, nil
}

type SessionInfo struct {
	Identity    string `json:"identity"`
	Method      string `json:"method"`
	ProjectHint string `json:"projectHint"`
}

func (a *App) Connect(method string, impersonateSA string) (*SessionInfo, error) {
	a.mu.Lock()
	cfg := auth.Config{Method: auth.Method(method), TargetSA: impersonateSA, KeyJSON: a.keyJSON}
	a.mu.Unlock()
	if cfg.Method == auth.KeyFile && len(cfg.KeyJSON) == 0 {
		return nil, errors.New("selecciona primero un archivo de clave JSON")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	sess, err := auth.Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.engine = core.NewEngine(sess, 20)
	a.projects = map[string]core.Project{}
	a.inv = nil
	a.mu.Unlock()
	return &SessionInfo{Identity: sess.Identity, Method: string(sess.Method), ProjectHint: sess.ProjectHint}, nil
}

func (a *App) Disconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
	a.engine, a.inv = nil, nil
	clear(a.keyJSON)
	a.keyJSON = nil
}

func (a *App) ListProjects(query string) ([]core.Project, error) {
	eng, err := a.eng()
	if err != nil {
		return nil, err
	}
	ps, err := eng.ListProjects(a.ctx, query)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	for _, p := range ps {
		a.projects[p.ID] = p
	}
	a.mu.Unlock()
	return ps, nil
}

// StartScan runs in the background and reports through events:
// "scan:progress" (core.Progress) and "scan:done" (summary) / "scan:error".
func (a *App) StartScan(projectIDs []string, opt core.ScanOptions) error {
	eng, err := a.eng()
	if err != nil {
		return err
	}
	if len(projectIDs) == 0 {
		return errors.New("selecciona al menos un proyecto")
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return errors.New("ya hay un escaneo en curso")
	}
	list := make([]core.Project, len(projectIDs))
	for i, id := range projectIDs {
		p, ok := a.projects[id]
		if !ok {
			p = core.Project{ID: id} // added manually by the user
		}
		list[i] = p
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.cancel = cancel
	a.mu.Unlock()

	go func() {
		defer func() {
			a.mu.Lock()
			a.cancel = nil
			a.mu.Unlock()
			cancel()
		}()
		var last time.Time
		inv, err := eng.Scan(ctx, list, opt, func(p core.Progress) {
			// throttle UI updates, but always send project completions
			if p.Step != "done" && time.Since(last) < 150*time.Millisecond {
				return
			}
			last = time.Now()
			runtime.EventsEmit(a.ctx, "scan:progress", p)
		})
		if err != nil {
			runtime.EventsEmit(a.ctx, "scan:error", err.Error())
			return
		}
		a.mu.Lock()
		a.inv = inv
		a.mu.Unlock()
		runtime.EventsEmit(a.ctx, "scan:done", inv.Meta)
	}()
	return nil
}

func (a *App) CancelScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

func (a *App) GetInventory() *model.Inventory {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.inv
}

// Export asks for a destination and writes the last inventory.
// format: "xlsx" | "csv" | "json"; csvSep: "," or ";".
func (a *App) Export(format string, csvSep string) (string, error) {
	a.mu.Lock()
	inv := a.inv
	a.mu.Unlock()
	if inv == nil {
		return "", errors.New("no hay inventario que exportar")
	}
	return a.save(inv, "Exportar inventario", "gcetools-"+inv.Meta.StartedAt.Format("20060102-1504"), format, csvSep)
}

// ExportDiff writes the last comparison.
func (a *App) ExportDiff(format string, csvSep string) (string, error) {
	a.mu.Lock()
	res := a.diff
	a.mu.Unlock()
	if res == nil {
		return "", errors.New("no hay comparación que exportar")
	}
	return a.save(res, "Exportar comparación", "gcetools-diff-"+time.Now().Format("20060102-1504"), format, csvSep)
}

func (a *App) save(inv *model.Inventory, title, name, format, csvSep string) (string, error) {
	f := export.Format(format)
	ext := map[export.Format]string{export.XLSX: "xlsx", export.CSV: "zip", export.JSON: "json"}[f]
	if ext == "" {
		return "", errors.New("formato no soportado")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           title,
		DefaultFilename: name + "." + ext,
		Filters:         []runtime.FileFilter{{DisplayName: ext, Pattern: "*." + ext}},
	})
	if err != nil || path == "" {
		return "", err
	}
	sep := ','
	if csvSep == ";" {
		sep = ';'
	}
	return export.Write(inv, path, f, sep)
}

func (a *App) eng() (*core.Engine, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.engine == nil {
		return nil, errors.New("no hay sesión: conéctate primero")
	}
	return a.engine, nil
}

// AboutInfo is what the "Acerca de" page shows.
type AboutInfo struct {
	Version    string `json:"version"`
	Website    string `json:"website"`
	Donate     string `json:"donate"`
	License    string `json:"license"`
	LicenseURL string `json:"licenseUrl"`
	Releases   string `json:"releases"`
}

func (a *App) AppInfo() AboutInfo {
	return AboutInfo{
		Version: core.Version, Website: update.Website, Donate: update.Donate,
		License: update.License, LicenseURL: update.LicenseURL, Releases: update.ReleaseURL,
	}
}

// CheckUpdate asks GitHub for the latest release.
func (a *App) CheckUpdate() (update.Info, error) {
	return update.Check(a.ctx, core.Version)
}

// SnapshotInfo summarizes a JSON snapshot chosen for comparison.
type SnapshotInfo struct {
	Path      string    `json:"path"`
	Name      string    `json:"name"`
	StartedAt time.Time `json:"startedAt"`
	Identity  string    `json:"identity"`
	Version   string    `json:"version"`
	Projects  int       `json:"projects"`
	VMs       int       `json:"vms"`
}

// CurrentInventory is the pseudo-path that selects the inventory in memory.
const CurrentInventory = "current"

// PickSnapshot opens a file dialog and loads a JSON snapshot.
func (a *App) PickSnapshot() (*SnapshotInfo, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title:   "Elegir snapshot JSON de gceTools",
		Filters: []runtime.FileFilter{{DisplayName: "Snapshot JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	inv, err := export.ReadJSON(path)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.snaps[path] = inv
	a.mu.Unlock()
	return snapshotInfo(path, filepath.Base(path), inv), nil
}

func snapshotInfo(path, name string, inv *model.Inventory) *SnapshotInfo {
	vms := 0
	if s := inv.Sheet("gInfo"); s != nil {
		vms = len(s.Rows)
	}
	m := inv.Meta
	return &SnapshotInfo{Path: path, Name: name, StartedAt: m.StartedAt, Identity: m.Identity,
		Version: m.Version, Projects: m.Projects, VMs: vms}
}

// CurrentSnapshot describes the inventory in memory, if any.
func (a *App) CurrentSnapshot() *SnapshotInfo {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.inv == nil {
		return nil
	}
	return snapshotInfo(CurrentInventory, "Inventario actual", a.inv)
}

// Compare diffs two snapshots picked before (or "current") and keeps the
// result for ExportDiff.
func (a *App) Compare(before, after string) (*model.Inventory, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	get := func(path string) (diff.Input, error) {
		if path == CurrentInventory {
			if a.inv == nil {
				return diff.Input{}, errors.New("no hay inventario actual: escanea primero")
			}
			return diff.Input{Inv: a.inv, Label: "Inventario actual"}, nil
		}
		inv, ok := a.snaps[path]
		if !ok {
			return diff.Input{}, errors.New("snapshot no cargado: vuelve a elegirlo")
		}
		return diff.Input{Inv: inv, Label: filepath.Base(path)}, nil
	}
	x, err := get(before)
	if err != nil {
		return nil, err
	}
	y, err := get(after)
	if err != nil {
		return nil, err
	}
	a.diff = diff.Compare(x, y)
	return a.diff, nil
}

// ---------- profiles & discovery ----------

func (a *App) ListProfiles() ([]profile.Profile, error) { return profile.List() }

// HealthRules is the gHealth rule catalog for the settings panel.
func (a *App) HealthRules() []health.Rule { return health.Rules }

// SaveProfile stores p. For service account keys only the file path is kept,
// so the CLI can reuse the profile; the key itself is never written.
func (a *App) SaveProfile(p profile.Profile) error {
	if p.Auth == "sa" && p.KeyPath == "" {
		a.mu.Lock()
		p.KeyPath = a.keyPath
		a.mu.Unlock()
	}
	return profile.Save(p)
}

func (a *App) DeleteProfile(name string) error { return profile.Delete(name) }

// Discover asks Cloud Asset Inventory which projects in scope hold Compute
// resources.
func (a *App) Discover(scope string) ([]core.Footprint, error) {
	eng, err := a.eng()
	if err != nil {
		return nil, err
	}
	return eng.Discover(a.ctx, scope)
}
