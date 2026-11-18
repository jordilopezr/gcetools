package main

import (
	"context"
	"errors"
	"os"
	"sync"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/core"
	"gcetools/internal/export"
	"gcetools/internal/model"
	"gcetools/internal/update"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the surface exposed to the frontend. Every exported method becomes
// a TypeScript function in frontend/wailsjs/go/main/App.
type App struct {
	ctx context.Context

	mu       sync.Mutex
	keyJSON  []byte // service account key, memory only
	engine   *core.Engine
	projects map[string]core.Project
	inv      *model.Inventory
	cancel   context.CancelFunc
}

func NewApp() *App { return &App{projects: map[string]core.Project{}} }

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
	f := export.Format(format)
	ext := map[export.Format]string{export.XLSX: "xlsx", export.CSV: "zip", export.JSON: "json"}[f]
	if ext == "" {
		return "", errors.New("formato no soportado")
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Exportar inventario",
		DefaultFilename: "gcetools-" + inv.Meta.StartedAt.Format("20060102-1504") + "." + ext,
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
