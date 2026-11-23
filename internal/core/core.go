// Package core is the UI-agnostic engine shared by the desktop app and the
// CLI: connect, discover projects, scan, and build the inventory.
package core

import (
	"context"
	"fmt"
	"math"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"gcetools/internal/auth"
	"gcetools/internal/billing"
	"gcetools/internal/collect"
	"gcetools/internal/health"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
	"gcetools/internal/pricing"
	"gcetools/internal/security"

	crm "google.golang.org/api/cloudresourcemanager/v3"
	compute "google.golang.org/api/compute/v1"
	monitoring "google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"
	osconfig "google.golang.org/api/osconfig/v1"
	recommender "google.golang.org/api/recommender/v1"
)

// Version is overridden at release build time:
//
//	-ldflags "-X gcetools/internal/core.Version=1.2.3"
var Version = "0.9.2"

type Engine struct {
	Session *auth.Session
	Client  *http.Client
}

func NewEngine(s *auth.Session, rps float64) *Engine {
	return &Engine{Session: s, Client: NewHTTPClient(s.TokenSource, rps)}
}

type Project struct {
	ID      string            `json:"id"`
	Number  string            `json:"number"`
	Name    string            `json:"name"`
	Parent  string            `json:"parent"`
	State   string            `json:"state"`
	Created string            `json:"created"`
	Labels  map[string]string `json:"labels"`
}

// ListProjects returns every project the identity can see, optionally
// narrowed by a Resource Manager search query (e.g. "parent:folders/123").
func (e *Engine) ListProjects(ctx context.Context, query string) ([]Project, error) {
	svc, err := crm.NewService(ctx, option.WithHTTPClient(e.Client))
	if err != nil {
		return nil, err
	}
	q := strings.TrimSpace(query)
	if !strings.Contains(q, "state:") {
		q = strings.TrimSpace("state:ACTIVE " + q)
	}
	var out []Project
	err = svc.Projects.Search().Query(q).Context(ctx).Pages(ctx, func(r *crm.SearchProjectsResponse) error {
		for _, p := range r.Projects {
			out = append(out, Project{
				ID: p.ProjectId, Number: strings.TrimPrefix(p.Name, "projects/"), Name: p.DisplayName,
				Parent: p.Parent, State: p.State, Created: p.CreateTime, Labels: p.Labels,
			})
		}
		return nil
	})
	if err != nil {
		_, msg := collect.Classify(err)
		return nil, fmt.Errorf(i18n.T("no se pudieron listar proyectos: %s"), msg)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// Modules toggles the optional, non-Compute data sources.
type Modules struct {
	OSConfig    bool `json:"osconfig"`
	Monitoring  bool `json:"monitoring"`
	Recommender bool `json:"recommender"`
	MetricsDays int  `json:"metricsDays"`
	// Pricing estimates monthly list-price cost from the Cloud Billing Catalog.
	Pricing  bool   `json:"pricing"`
	Currency string `json:"currency"` // ISO 4217, default USD
	// Billing reads the real net cost from a Cloud Billing export in BigQuery
	// (empty table = off). The only module that can cost money: see billing.
	BillingTable string `json:"billingTable"`
	BillingMonth string `json:"billingMonth"` // YYYYMM; empty = previous month
	BillingMaxGB int    `json:"billingMaxGB"` // refuse queries above this; 0 = 50 GB
	// Fast reads instances, disks, snapshots, images, templates, MIGs and
	// networks from Cloud Asset Inventory instead of the Compute API. With an
	// organization or folder in FastScope, one listing covers every project.
	Fast      bool   `json:"fast"`
	FastScope string `json:"fastScope"`
}

func AllModules() Modules {
	return Modules{OSConfig: true, Monitoring: true, Recommender: true, MetricsDays: 30}
}

type ScanOptions struct {
	Concurrency int `json:"concurrency"`
	// RPS overrides the requests per second per API for this scan (0 = the
	// engine's own, 20 by default; Compute gets three times that).
	RPS     float64        `json:"rps,omitempty"`
	Health  health.Options `json:"health"`
	Modules Modules        `json:"modules"`
	// SaveHistory stores the result in the local history (the app does it,
	// the CLI with --save-history); Profile labels that history entry.
	SaveHistory bool   `json:"saveHistory"`
	Profile     string `json:"profile"`
}

type Progress struct {
	Total   int    `json:"total"`
	Done    int    `json:"done"`
	Project string `json:"project"`
	Step    string `json:"step"`
	VMs     int64  `json:"vms"`
	Errors  int    `json:"errors"`
	Message string `json:"message"`
}

// Scan inventories the given projects concurrently. It always returns an
// inventory (possibly partial if ctx is cancelled).
func (e *Engine) Scan(ctx context.Context, projects []Project, opt ScanOptions, onProgress func(Progress)) (*model.Inventory, error) {
	if opt.Concurrency <= 0 {
		opt.Concurrency = 8
	}
	if opt.RPS > 0 {
		e = &Engine{Session: e.Session, Client: NewHTTPClient(e.Session.TokenSource, opt.RPS)}
	}
	// Without cloud-platform these APIs refuse every call: switch them off
	// and say why once, instead of one permission error per project.
	var unavailable []string
	if !e.Session.FullScope {
		for _, m := range []struct {
			on   *bool
			name string
		}{{&opt.Modules.Fast, "cloudasset"}, {&opt.Modules.OSConfig, "osconfig"}, {&opt.Modules.Recommender, "recommender"}} {
			if *m.on {
				*m.on = false
				unavailable = append(unavailable, m.name)
			}
		}
	}
	svcs, err := e.services(ctx, opt.Modules)
	if err != nil {
		return nil, err
	}

	inv := collect.NewInventory()
	inv.Meta = model.Meta{
		Tool: "gceTools", Version: Version, Identity: e.Session.Identity,
		AuthMethod: string(e.Session.Method), StartedAt: time.Now(), Projects: len(projects),
	}
	for _, name := range unavailable {
		inv.AddError(model.ScanError{Project: "(all)", Collector: name, Kind: model.ErrPermission,
			Message: i18n.T("no disponible con inicio de sesión con Google, clave o impersonación de service account: esta API solo acepta el permiso completo de Google Cloud. Usa gcloud (ADC) para incluirlo")})
	}

	var (
		mu    sync.Mutex
		done  int
		vms   int64
		stats = make(map[string]collect.Stats, len(projects))
	)
	emit := func(p Progress) {
		if onProgress == nil {
			return
		}
		mu.Lock()
		p.Total, p.Done, p.VMs = len(projects), done, vms
		mu.Unlock()
		p.Errors = inv.ErrorCount()
		onProgress(p)
	}

	jobs := make(chan Project)
	var wg sync.WaitGroup
	for w := 0; w < opt.Concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for p := range jobs {
				st := collect.Project(ctx, svcs, p.ID, inv, func(step string) {
					emit(Progress{Project: p.ID, Step: step})
				})
				mu.Lock()
				stats[p.ID] = st
				done++
				vms += st.VMs
				mu.Unlock()
				emit(Progress{Project: p.ID, Step: "done", Message: st.Status})
			}
		}()
	}
feed:
	for _, p := range projects {
		select {
		case jobs <- p:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()

	inv.Meta.FinishedAt = time.Now()
	inv.Meta.Cancelled = ctx.Err() != nil
	e.finish(ctx, inv, projects, stats, opt)
	return inv, nil
}

func (e *Engine) finish(ctx context.Context, inv *model.Inventory, projects []Project, stats map[string]collect.Stats, opt ScanOptions) {
	ps := inv.Sheet(collect.SProject)
	for _, p := range projects {
		st, scanned := stats[p.ID]
		status := st.Status
		if !scanned {
			status = i18n.T("No escaneado")
		}
		// Also "OK (sin osconfig)" / "OK (without osconfig)": optional modules do
		// not make a project fail. Statuses start with "OK" in every language.
		if strings.HasPrefix(status, "OK") {
			inv.Meta.ProjectsOK++
		}
		lbl := make([]string, 0, len(p.Labels))
		for k, v := range p.Labels {
			lbl = append(lbl, k+"="+v)
		}
		sort.Strings(lbl)
		ps.Add(model.Row{
			"projectId": p.ID, "number": p.Number, "name": p.Name, "parent": p.Parent, "state": p.State,
			"scanStatus": status, "vms": st.VMs, "vmsRunning": st.VMsRunning, "vcpu": st.VCPU,
			"memoryGB": math.Round(st.MemoryGB*100) / 100, "disks": st.Disks, "diskGB": st.DiskGB,
			"labels":  strings.Join(lbl, "; "),
			"osLogin": st.OSLogin, "sshKeys": st.SSHKeys, "serialPort": st.SerialPort,
		})
	}

	var priced *pricing.Result
	if opt.Modules.Pricing && ctx.Err() == nil {
		cat, err := pricing.Load(ctx, opt.Modules.Currency, 24*time.Hour,
			e.Client, withQuotaProject(e.Client, e.Session.QuotaProject))
		if err != nil {
			kind, msg := collect.Classify(err)
			inv.AddError(model.ScanError{Project: i18n.T("(todos)"), Collector: "pricing", Kind: kind, Message: msg})
		} else {
			r := pricing.Apply(inv, pricing.NewIndex(cat))
			priced = &r
			inv.Meta.Currency = cat.Currency
		}
	}

	var billed *billing.Data
	var billedRes billing.Result
	if opt.Modules.BillingTable != "" && ctx.Err() == nil {
		var ids []string
		for _, p := range projects {
			ids = append(ids, p.ID)
		}
		d, err := billing.Load(ctx, e.Client, billing.Config{Table: opt.Modules.BillingTable, Month: opt.Modules.BillingMonth,
			MaxGB: opt.Modules.BillingMaxGB}, ids, time.Now())
		if err != nil {
			kind, msg := collect.Classify(err)
			inv.AddError(model.ScanError{Project: i18n.T("(todos)"), Collector: "billing", Kind: kind, Message: msg})
		} else {
			billed, billedRes = d, billing.Apply(inv, d)
		}
	}

	security.Evaluate(inv)
	hres := health.Evaluate(inv, opt.Health, inv.Meta.FinishedAt)

	es := inv.Sheet(collect.SErrors)
	for _, er := range inv.Errors {
		es.Add(model.Row{"project": er.Project, "collector": er.Collector, "kind": string(er.Kind), "message": er.Message})
	}

	inv.Meta.Errors = len(inv.Errors)
	m := inv.Meta
	meta := inv.Sheet(collect.SMeta)
	for _, kv := range [][2]string{
		{i18n.T("Herramienta"), m.Tool + " " + m.Version},
		{i18n.T("Identidad"), m.Identity},
		{i18n.T("Método de autenticación"), m.AuthMethod},
		{i18n.T("Scope OAuth"), scopesText(e.Session)},
		{i18n.T("Módulos opcionales"), modulesText(opt.Modules)},
		{i18n.T("Quota project"), e.Session.QuotaProject},
		{i18n.T("Moneda de costes"), m.Currency},
		{i18n.T("Base de los costes"), map[bool]string{true: i18n.T("Precio de lista on-demand del Cloud Billing Catalog, 730 h/mes; sin SUD, CUD ni descuentos negociados"), false: i18n.T("No estimados")}[priced != nil]},
		{i18n.T("Sin tarifa"), unpricedText(priced)},
		{i18n.T("Coste real (BigQuery)"), billingText(billed, billedRes)},
		{i18n.T("Reglas de salud desactivadas"), strings.Join(hres.Disabled, ", ")},
		{i18n.T("Hallazgos ocultos por labels gcetools-ignore"), fmt.Sprint(hres.Suppressed)},
		{i18n.T("Inicio"), m.StartedAt.Format(time.RFC3339)},
		{i18n.T("Fin"), m.FinishedAt.Format(time.RFC3339)},
		{i18n.T("Duración"), m.FinishedAt.Sub(m.StartedAt).Round(time.Second).String()},
		{i18n.T("Proyectos seleccionados"), fmt.Sprint(m.Projects)},
		{i18n.T("Proyectos OK"), fmt.Sprint(m.ProjectsOK)},
		{i18n.T("Errores"), fmt.Sprint(m.Errors)},
		{i18n.T("Cancelado"), map[bool]string{true: i18n.T("Sí"), false: i18n.T("No")}[m.Cancelled]},
	} {
		meta.Add(model.Row{"property": kv[0], "value": kv[1]})
	}
}

func (e *Engine) services(ctx context.Context, m Modules) (collect.Services, error) {
	var s collect.Services
	var err error
	if s.Compute, err = compute.NewService(ctx, option.WithHTTPClient(e.Client)); err != nil {
		return s, err
	}
	s.MetricsDays = m.MetricsDays
	s.MachineTypes = collect.NewMachineTypeCache()
	if m.Fast {
		s.Assets = e.newAssetSource(m.FastScope)
	}
	if m.OSConfig {
		if s.OSConfig, err = osconfig.NewService(ctx, option.WithHTTPClient(e.Client)); err != nil {
			return s, err
		}
	}
	if m.Monitoring {
		if s.Monitoring, err = monitoring.NewService(ctx, option.WithHTTPClient(e.Client)); err != nil {
			return s, err
		}
	}
	if m.Recommender {
		rc := withQuotaProject(e.Client, e.Session.QuotaProject)
		if s.Recommender, err = recommender.NewService(ctx, option.WithHTTPClient(rc)); err != nil {
			return s, err
		}
	}
	return s, nil
}

func scopesText(s *auth.Session) string {
	if s.FullScope {
		return auth.ScopeCloudAll + " (gcloud)"
	}
	return strings.Join(auth.Scopes, " ")
}

func modulesText(m Modules) string {
	var on []string
	if m.Fast {
		mode := i18n.T("modo rápido con Cloud Asset Inventory (puede reflejar cambios con unos minutos de retraso)")
		if m.FastScope != "" {
			mode = i18n.Tf("%s en %s", mode, m.FastScope)
		}
		on = append(on, mode)
	}
	if m.OSConfig {
		on = append(on, "OS Config")
	}
	if m.Monitoring {
		on = append(on, i18n.Tf("Cloud Monitoring (%d días)", m.MetricsDays))
	}
	if m.Recommender {
		on = append(on, "Recommender")
	}
	if m.Pricing {
		on = append(on, i18n.T("coste estimado"))
	}
	if m.BillingTable != "" {
		on = append(on, i18n.T("coste real (BigQuery)"))
	}
	if len(on) == 0 {
		return i18n.T("ninguno")
	}
	return strings.Join(on, ", ")
}

func billingText(d *billing.Data, r billing.Result) string {
	if d == nil {
		return ""
	}
	kind := i18n.T("exportación estándar: solo por proyecto")
	if d.Detailed {
		kind = i18n.Tf("exportación detallada: %.0f%% atribuido a VMs y discos", 100*r.Attributed/max(r.Net, 0.01))
	}
	return i18n.Tf("%s, mes %s, neto %.2f %s (%s); BigQuery leyó %.2f GB", d.Table, d.Month, r.Net, d.Currency,
		kind, float64(d.BytesRead)/1e9)
}

func unpricedText(r *pricing.Result) string {
	if r == nil || len(r.Unpriced) == 0 {
		return ""
	}
	return strings.Join(r.Unpriced, "; ")
}
