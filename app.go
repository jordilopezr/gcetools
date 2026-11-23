package main

import (
	"context"
	"errors"
	"fmt"
	"gcetools/internal/schema"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/core"
	"gcetools/internal/diff"
	"gcetools/internal/export"
	"gcetools/internal/health"
	"gcetools/internal/history"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
	"gcetools/internal/osopen"
	"gcetools/internal/profile"
	"gcetools/internal/report"
	"gcetools/internal/update"
	"gcetools/internal/view"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the surface exposed to the frontend. Every exported method becomes
// a TypeScript function in frontend/wailsjs/go/main/App.
type App struct {
	ctx context.Context
	ui  uiRuntime

	mu          sync.Mutex
	keyJSON     []byte             // service account key, memory only
	keyPath     string             // where it was read from (saved in profiles; never the content)
	loginCancel context.CancelFunc // pending Google sign-in
	engine      *core.Engine
	projects    map[string]core.Project
	inv         *model.Inventory
	cancel      context.CancelFunc          // running scan
	scanGen     uint64                      // bumped by every scan start, Connect and Disconnect
	scans       sync.WaitGroup              // running scan goroutines (tests wait on it)
	diff        *model.Inventory            // last comparison
	snaps       map[string]*model.Inventory // snapshots loaded for comparison, by path
	snapOrder   []string                    // paths in a.snaps, oldest first
}

// maxSnaps bounds the JSON snapshots kept for comparison: the page only
// compares two at a time, and each one can take hundreds of MB.
const maxSnaps = 2

func NewApp() *App {
	return &App{projects: map[string]core.Project{}, snaps: map[string]*model.Inventory{}}
}

func (a *App) startup(ctx context.Context) { a.ctx, a.ui = ctx, wailsUI{ctx} }

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

// SetLanguage switches the language of backend messages, new scans and
// exports ("es" or "en"). The UI calls it at startup and on every change.
func (a *App) SetLanguage(lang string) { i18n.SetLang(lang) }

func (a *App) DetectADC() ADCStatus {
	found, id := auth.DetectADC(a.ctx)
	return ADCStatus{Found: found, Identity: id}
}

// PickServiceAccountKey opens a file dialog, validates the key and keeps it
// in memory. Only the public identity is returned to the UI.
func (a *App) PickServiceAccountKey() (*auth.KeyInfo, error) {
	path, err := a.ui.OpenFile(runtime.OpenDialogOptions{
		Title:   i18n.T("Selecciona la clave JSON de la service account"),
		Filters: []runtime.FileFilter{{DisplayName: i18n.T("Clave JSON (*.json)"), Pattern: "*.json"}},
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
	FullScope   bool   `json:"fullScope"` // false: no Cloud Asset Inventory, Recommender or OS Config
}

func (a *App) Connect(method string, impersonateSA string) (*SessionInfo, error) {
	a.mu.Lock()
	cfg := auth.Config{Method: auth.Method(method), TargetSA: impersonateSA, KeyJSON: a.keyJSON}
	a.mu.Unlock()
	if cfg.Method == auth.KeyFile && len(cfg.KeyJSON) == 0 {
		return nil, errors.New(i18n.T("selecciona primero un archivo de clave JSON"))
	}
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	sess, err := auth.Connect(ctx, cfg)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.stopScanLocked()
	a.engine = core.NewEngine(sess, 20)
	a.projects = map[string]core.Project{}
	a.inv = nil
	a.mu.Unlock()
	return &SessionInfo{Identity: sess.Identity, Method: string(sess.Method), ProjectHint: sess.ProjectHint, FullScope: sess.FullScope}, nil
}

func (a *App) Disconnect() {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopScanLocked()
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
		return errors.New(i18n.T("selecciona al menos un proyecto"))
	}
	a.mu.Lock()
	if a.cancel != nil {
		a.mu.Unlock()
		return errors.New(i18n.T("ya hay un escaneo en curso"))
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
	a.scanGen++
	gen := a.scanGen
	a.mu.Unlock()

	a.scans.Add(1)
	go func() {
		defer a.scans.Done()
		defer func() {
			a.mu.Lock()
			if a.scanGen == gen {
				a.cancel = nil
			}
			a.mu.Unlock()
			cancel()
		}()
		// Workers report progress concurrently.
		var progressMu sync.Mutex
		var last time.Time
		inv, err := eng.Scan(ctx, list, opt, func(p core.Progress) {
			progressMu.Lock()
			defer progressMu.Unlock()
			// throttle UI updates, but always send project completions
			if p.Step != "done" && time.Since(last) < 150*time.Millisecond {
				return
			}
			a.mu.Lock()
			current := a.scanGen == gen
			a.mu.Unlock()
			if !current {
				return // superseded: the UI already left the scan page
			}
			last = time.Now()
			a.ui.Emit("scan:progress", p)
		})
		a.mu.Lock()
		if a.scanGen != gen {
			// Disconnect or a new sign-in superseded this scan: drop it.
			a.mu.Unlock()
			return
		}
		if err != nil {
			a.mu.Unlock()
			a.ui.Emit("scan:error", err.Error())
			return
		}
		a.inv = inv
		a.mu.Unlock()
		if opt.SaveHistory {
			if _, err := history.Save(inv, opt.Profile); err != nil {
				a.ui.Emit("scan:warning", i18n.Tf("No se pudo guardar en el historial: %v", err))
			}
		}
		a.ui.Emit("scan:done", inv.Meta)
	}()
	return nil
}

// stopScanLocked cancels the running scan and makes sure its result is
// discarded. Callers hold a.mu.
func (a *App) stopScanLocked() {
	if a.cancel != nil {
		a.cancel()
		a.cancel = nil
	}
	a.scanGen++
}

func (a *App) CancelScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cancel != nil {
		a.cancel()
	}
}

// InventorySummary describes the current inventory (sheets, row counts,
// headline figures) without its rows; SheetRows sends one sheet at a time.
// Nil when there is no inventory.
func (a *App) InventorySummary() *view.Summary {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.inv == nil {
		return nil
	}
	schema.Relabel(a.inv)
	s := view.Summarize(a.inv)
	return &s
}

// SheetRows returns the rows of one sheet of the current inventory, without
// the VMs created by GKE, Dataproc, Dataflow or Batch when hideManaged.
func (a *App) SheetRows(name string, hideManaged bool) ([]model.Row, error) {
	a.mu.Lock()
	inv := a.inv
	a.mu.Unlock()
	if inv == nil {
		return nil, errors.New(i18n.T("no hay inventario: escanea o abre uno del historial"))
	}
	rows := view.Rows(inv, name, hideManaged)
	if rows == nil {
		return nil, fmt.Errorf(i18n.T("la pestaña %s no existe en este inventario"), name)
	}
	return rows, nil
}

// Export asks for a destination and writes the last inventory.
// format: "xlsx" | "csv" | "json"; csvSep: "," or ";".
func (a *App) Export(format string, csvSep string) (string, error) {
	a.mu.Lock()
	inv := a.inv
	schema.Relabel(inv)
	a.mu.Unlock()
	if inv == nil {
		return "", errors.New(i18n.T("no hay inventario que exportar"))
	}
	return a.save(inv, i18n.T("Exportar inventario"), "gcetools-"+inv.Meta.StartedAt.Format("20060102-1504"), format, csvSep)
}

// ExportDiff writes the last comparison.
func (a *App) ExportDiff(format string, csvSep string) (string, error) {
	a.mu.Lock()
	res := a.diff
	a.mu.Unlock()
	if res == nil {
		return "", errors.New(i18n.T("no hay comparación que exportar"))
	}
	return a.save(diff.Display(res), i18n.T("Exportar comparación"), "gcetools-diff-"+time.Now().Format("20060102-1504"), format, csvSep)
}

func (a *App) save(inv *model.Inventory, title, name, format, csvSep string) (string, error) {
	f := export.Format(format)
	ext := map[export.Format]string{export.XLSX: "xlsx", export.CSV: "zip", export.JSON: "json"}[f]
	if ext == "" {
		return "", errors.New(i18n.T("formato no soportado"))
	}
	path, err := a.ui.SaveFile(runtime.SaveDialogOptions{
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
		return nil, errors.New(i18n.T("no hay sesión: conéctate primero"))
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
	path, err := a.ui.OpenFile(runtime.OpenDialogOptions{
		Title:   i18n.T("Elegir snapshot JSON de gceTools"),
		Filters: []runtime.FileFilter{{DisplayName: i18n.T("Snapshot JSON (*.json)"), Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return nil, err
	}
	inv, err := export.ReadJSON(path)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	a.snapOrder = append(slices.DeleteFunc(a.snapOrder, func(p string) bool { return p == path }), path)
	a.snaps[path] = inv
	for len(a.snapOrder) > maxSnaps {
		delete(a.snaps, a.snapOrder[0])
		a.snapOrder = a.snapOrder[1:]
	}
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
	return snapshotInfo(CurrentInventory, i18n.T("Inventario actual"), a.inv)
}

// Compare diffs two snapshots picked before (or "current") and keeps the
// result for ExportDiff.
func (a *App) Compare(before, after string) (*model.Inventory, error) {
	a.mu.Lock()
	cur, snaps := a.inv, map[string]*model.Inventory{}
	for _, p := range []string{before, after} {
		if inv, ok := a.snaps[p]; ok {
			snaps[p] = inv
		}
	}
	a.mu.Unlock()
	// History scans are read without holding a.mu: loading a large one takes
	// seconds and would block CancelScan and the rest of the UI.
	get := func(path string) (diff.Input, error) {
		if path == CurrentInventory {
			if cur == nil {
				return diff.Input{}, errors.New(i18n.T("no hay inventario actual: escanea primero"))
			}
			return diff.Input{Inv: cur, Label: i18n.T("Inventario actual")}, nil
		}
		if id, ok := strings.CutPrefix(path, HistoryPrefix); ok {
			inv, err := history.Load(id)
			if err != nil {
				return diff.Input{}, fmt.Errorf(i18n.T("no se pudo leer el escaneo %s del historial: %w"), id, err)
			}
			return diff.Input{Inv: inv, Label: i18n.Tf("Historial %s", id)}, nil
		}
		inv, ok := snaps[path]
		if !ok {
			return diff.Input{}, errors.New(i18n.T("snapshot no cargado: vuelve a elegirlo"))
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
	res := diff.Compare(x, y)
	a.mu.Lock()
	a.diff = res
	a.mu.Unlock()
	return diff.Display(res), nil
}

// ---------- profiles & discovery ----------

func (a *App) ListProfiles() ([]profile.Profile, error) { return profile.List() }

// HealthRules is the gHealth rule catalog for the settings panel.
func (a *App) HealthRules() []health.Rule { return health.LocalizedRules() }

// SaveProfile stores p. For service account keys only the file path is kept,
// so the CLI can reuse the profile; the key itself is never written.
//
// The key path never comes from the UI: it is the file picked in the native
// dialog during this session or, failing that, the one already stored in the
// profile. Otherwise a compromised frontend could make later CLI scans use a
// key of its choice.
func (a *App) SaveProfile(p profile.Profile) error {
	p.KeyPath = ""
	if p.Auth == "sa" {
		a.mu.Lock()
		p.KeyPath = a.keyPath
		a.mu.Unlock()
		if p.KeyPath == "" {
			if old, err := profile.Get(p.Name); err == nil {
				p.KeyPath = old.KeyPath
			}
		}
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

// ---------- history ----------

// HistoryPrefix marks a Compare path that names a history entry ("history:<id>").
const HistoryPrefix = "history:"

func (a *App) ListHistory() ([]history.Summary, error) { return history.List() }

// OpenHistory loads a stored scan as the current inventory and returns its
// summary.
func (a *App) OpenHistory(id string) (*view.Summary, error) {
	inv, err := history.Load(id)
	if err != nil {
		return nil, err
	}
	schema.Relabel(inv)
	a.mu.Lock()
	a.inv = inv
	a.mu.Unlock()
	s := view.Summarize(inv)
	return &s, nil
}

func (a *App) DeleteHistory(id string) error { return history.Delete(id) }

// HistorySnapshot describes a history entry for the comparison page, from
// the index: loading the scan itself takes seconds for large inventories.
func (a *App) HistorySnapshot(id string) (*SnapshotInfo, error) {
	s, err := history.Get(id)
	if err != nil {
		return nil, err
	}
	return &SnapshotInfo{Path: HistoryPrefix + id, Name: i18n.Tf("Historial %s", id), StartedAt: s.StartedAt,
		Identity: s.Identity, Version: s.Version, Projects: s.Projects, VMs: s.VMs}, nil
}

// HistoryDir is shown in the UI so users know where scans are kept.
func (a *App) HistoryDir() string {
	d, err := history.Dir()
	if err != nil {
		return ""
	}
	return d
}

// ExportReport writes the executive HTML summary of the current inventory,
// including the changes since the previous scan kept in the history.
func (a *App) ExportReport() (string, error) {
	a.mu.Lock()
	inv := a.inv
	a.mu.Unlock()
	if inv == nil {
		return "", errors.New(i18n.T("no hay inventario: escanea o abre uno del historial"))
	}
	data, err := report.Render(inv, reportOptions(inv))
	if err != nil {
		return "", err
	}
	path, err := a.ui.SaveFile(runtime.SaveDialogOptions{
		Title:           i18n.T("Guardar informe ejecutivo"),
		DefaultFilename: i18n.Tf("gcetools-informe-%s.html", inv.Meta.StartedAt.Format("20060102-1504")),
		Filters:         []runtime.FileFilter{{DisplayName: "HTML", Pattern: "*.html"}},
	})
	if err != nil || path == "" {
		return "", err
	}
	if err := export.WritePrivate(path, data); err != nil {
		return "", err
	}
	// Not runtime.BrowserOpenURL: Wails rejects file:// URLs.
	_ = osopen.Open(path)
	return path, nil
}

// reportOptions adds the comparison with the previous history entry, if any.
func reportOptions(inv *model.Inventory) report.Options {
	opt := report.Options{Website: update.Website}
	if prev, ok := history.Previous(inv.Meta.StartedAt); ok {
		if old, err := history.Load(prev.ID); err == nil {
			opt.Diff = diff.Compare(diff.Input{Inv: old, Label: i18n.Tf("Historial %s", prev.ID)}, diff.Input{Inv: inv, Label: i18n.T("Actual")})
			opt.DiffSince = prev.StartedAt
		}
	}
	return opt
}

// ---------- sign in with Google (OAuth client of the organization) ----------

func (a *App) OAuthStatus() auth.OAuthStatus { return auth.GetOAuthStatus() }

// OAuthPickClient loads the organization's "Desktop app" client JSON into
// the OS keyring.
func (a *App) OAuthPickClient() (auth.OAuthStatus, error) {
	path, err := a.ui.OpenFile(runtime.OpenDialogOptions{
		Title:   i18n.T("Cliente OAuth de tu organización (client_secret_….json)"),
		Filters: []runtime.FileFilter{{DisplayName: "JSON (*.json)", Pattern: "*.json"}},
	})
	if err != nil || path == "" {
		return auth.GetOAuthStatus(), err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return auth.OAuthStatus{}, err
	}
	c, err := auth.ParseOAuthClient(data)
	if err != nil {
		return auth.OAuthStatus{}, err
	}
	if err := auth.SetOAuthClient(c); err != nil {
		return auth.OAuthStatus{}, err
	}
	return auth.GetOAuthStatus(), nil
}

// OAuthLogin opens the browser and waits (up to 5 minutes) for the consent.
func (a *App) OAuthLogin() (auth.OAuthStatus, error) {
	ctx, cancel := context.WithCancel(a.ctx)
	a.mu.Lock()
	a.loginCancel = cancel
	a.mu.Unlock()
	defer func() {
		cancel()
		a.mu.Lock()
		a.loginCancel = nil
		a.mu.Unlock()
	}()
	if _, err := auth.Login(ctx, func(u string) { a.ui.OpenURL(u) }); err != nil {
		return auth.GetOAuthStatus(), err
	}
	return auth.GetOAuthStatus(), nil
}

// OAuthCancel aborts a pending OAuthLogin.
func (a *App) OAuthCancel() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.loginCancel != nil {
		a.loginCancel()
	}
}

// OAuthLogout signs out; all=true also forgets the organization's client.
func (a *App) OAuthLogout(all bool) (auth.OAuthStatus, error) {
	err := auth.ForgetOAuth(all)
	return auth.GetOAuthStatus(), err
}

// OAuthUseBuiltin goes back to gceTools' own OAuth client.
func (a *App) OAuthUseBuiltin() (auth.OAuthStatus, error) {
	err := auth.UseBuiltinClient()
	return auth.GetOAuthStatus(), err
}
