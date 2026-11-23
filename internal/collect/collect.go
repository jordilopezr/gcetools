// Package collect reads Compute Engine resources for one project and turns
// them into inventory rows. One collector per resource type; each uses
// aggregatedList where available so a project costs one call (plus pages)
// per resource type regardless of how many zones it uses.
//
// Core collectors (Compute API) always run. Optional modules (OS Config,
// Cloud Monitoring, Recommender) run only when their service is set; if their
// API is disabled or not permitted the project is still reported as OK.
package collect

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"gcetools/internal/i18n"
	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
	"google.golang.org/api/googleapi"
	monitoring "google.golang.org/api/monitoring/v3"
	osconfig "google.golang.org/api/osconfig/v1"
	recommender "google.golang.org/api/recommender/v1"
)

// Services groups the API clients. Nil optional services disable their module.
type Services struct {
	Compute     *compute.Service
	OSConfig    *osconfig.Service
	Monitoring  *monitoring.Service
	Recommender *recommender.Service
	MetricsDays int // look-back window for gPerf (default 30)
	// Assets enables the fast mode: the resources in AssetTypes come from
	// Cloud Asset Inventory. If it fails for a project, that project falls
	// back to the Compute API.
	Assets AssetSource
	// MachineTypes shares vCPU/RAM lookups between projects (optional).
	MachineTypes *MachineTypeCache
}

// MachineTypeCache holds machine types by zone/name. A machine type has the
// same shape in every project, so a scan needs one lookup per type and zone.
type MachineTypeCache struct {
	mu sync.Mutex
	m  map[string]*compute.MachineType
}

func NewMachineTypeCache() *MachineTypeCache {
	return &MachineTypeCache{m: map[string]*compute.MachineType{}}
}

func (mc *MachineTypeCache) get(key string) *compute.MachineType {
	if mc == nil {
		return nil
	}
	mc.mu.Lock()
	defer mc.mu.Unlock()
	return mc.m[key]
}

func (mc *MachineTypeCache) put(key string, mt *compute.MachineType) {
	if mc == nil || mt == nil {
		return
	}
	mc.mu.Lock()
	mc.m[key] = mt
	mc.mu.Unlock()
}

// Stats summarises one project for the gProject sheet.
type Stats struct {
	Status     string
	VMs        int64
	VMsRunning int64
	VCPU       int64
	MemoryGB   float64
	Disks      int64
	DiskGB     float64
	// Project-wide metadata (gProject): OS Login and SSH keys every VM of
	// the project inherits unless it overrides or blocks them.
	OSLogin    bool
	SSHKeys    int64
	SerialPort bool
}

type vmRef struct {
	name, zone, machineType, status string
	vcpu                            int64
	memoryGB                        float64
}

type collector struct {
	ctx     context.Context
	svc     *compute.Service
	svcs    Services
	project string
	inv     *model.Inventory
	now     time.Time

	sheets  map[string]*model.Sheet
	disks   map[string]*compute.Disk        // selfLink -> disk
	mtypes  map[string]*compute.MachineType // zone/name -> type (nil = lookup failed)
	vms     map[string]vmRef                // instance id -> summary (for metrics)
	zones   map[string]bool                 // zones holding VMs or disks
	regions map[string]bool                 // regions holding VMs, disks or addresses
	assets  Assets                          // fast mode: prefetched resources (nil = Compute API)
	meta    map[string]string               // project-wide metadata (commonInstanceMetadata)
	stats   Stats
}

type step struct {
	name     string
	fn       func() error
	optional bool
}

// Project runs every enabled collector against one project. report is called
// with the collector name before it starts. Errors are recorded in inv.
func Project(ctx context.Context, svcs Services, projectID string, inv *model.Inventory, report func(step string)) Stats {
	c := &collector{
		ctx: ctx, svc: svcs.Compute, svcs: svcs, project: projectID, inv: inv, now: time.Now(),
		sheets:  map[string]*model.Sheet{},
		disks:   map[string]*compute.Disk{},
		mtypes:  map[string]*compute.MachineType{},
		vms:     map[string]vmRef{},
		zones:   map[string]bool{},
		regions: map[string]bool{},
	}
	for _, s := range inv.Sheets {
		c.sheets[s.Name] = s
	}

	if svcs.Assets != nil {
		report("cloudasset")
		a, err := svcs.Assets.Project(ctx, projectID)
		if err != nil && ctx.Err() == nil {
			kind, msg := Classify(err)
			inv.AddError(model.ScanError{Project: projectID, Collector: "cloudasset", Kind: kind,
				Message: i18n.Tf("%s (se usó la Compute API)", msg)})
		}
		c.assets = a
	}

	steps := []step{
		{name: "disks", fn: c.collectDisks},             // first: instances need the disk map
		{name: "project", fn: c.collectProjectMetadata}, // before instances: they inherit it
		{name: "instances", fn: c.collectInstances},
		{name: "snapshots", fn: c.collectSnapshots},
		{name: "images", fn: c.collectImages},
		{name: "migs", fn: c.collectMIGs},
		{name: "templates", fn: c.collectTemplates},
		{name: "reservations", fn: c.collectReservations},
		{name: "commitments", fn: c.collectCommitments},
		{name: "soletenant", fn: c.collectSoleTenant},
		{name: "networks", fn: c.collectNetworks},
		{name: "subnets", fn: c.collectSubnets},
		{name: "firewalls", fn: c.collectFirewalls},
		{name: "addresses", fn: c.collectAddresses},
		{name: "quotas", fn: c.collectQuotas},
	}
	if svcs.OSConfig != nil {
		steps = append(steps, step{name: "osconfig", fn: c.collectOSInventory, optional: true})
	}
	if svcs.Monitoring != nil {
		steps = append(steps, step{name: "monitoring", fn: c.collectMetrics, optional: true})
	}
	if svcs.Recommender != nil {
		steps = append(steps, step{name: "recommender", fn: c.collectRecommendations, optional: true})
	}

	failed := 0
	var skipped []string
	var optional []step
	for i, st := range steps {
		if st.optional {
			optional = append(optional, st)
			continue
		}
		if ctx.Err() != nil {
			c.stats.Status = i18n.T("Cancelado")
			return c.stats
		}
		report(st.name)
		err := st.fn()
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			c.stats.Status = i18n.T("Cancelado")
			return c.stats
		}
		kind, msg := Classify(err)
		inv.AddError(model.ScanError{Project: projectID, Collector: st.name, Kind: kind, Message: msg})
		failed++
		// If the very first call is refused, the rest will be too: stop early.
		if i == 0 && kind != model.ErrOther {
			c.stats.Status = map[model.ErrorKind]string{
				model.ErrAPIDisabled: i18n.T("Compute API deshabilitada"),
				model.ErrPermission:  i18n.T("Sin permisos"),
				model.ErrNotFound:    i18n.T("No encontrado"),
			}[kind]
			return c.stats
		}
	}
	// The optional modules only read what the Compute steps collected and
	// call different APIs, so they run at the same time.
	errs := make([]error, len(optional))
	var wg sync.WaitGroup
	for i, st := range optional {
		if ctx.Err() != nil {
			break
		}
		report(st.name)
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = st.fn()
		}()
	}
	wg.Wait()
	if ctx.Err() != nil {
		c.stats.Status = i18n.T("Cancelado")
		return c.stats
	}
	for i, st := range optional {
		if errs[i] == nil {
			continue
		}
		kind, msg := Classify(errs[i])
		inv.AddError(model.ScanError{Project: projectID, Collector: st.name, Kind: kind, Message: msg})
		skipped = append(skipped, st.name)
	}
	switch {
	case failed > 0:
		c.stats.Status = i18n.Tf("Parcial (%d errores)", failed)
	case len(skipped) > 0:
		// English keeps the "OK" prefix: core counts projects OK with it.
		c.stats.Status = i18n.Tf("OK (sin %s)", strings.Join(skipped, ", "))
	default:
		c.stats.Status = "OK"
	}
	return c.stats
}

// Classify maps an API error to a kind the UI can explain.
func Classify(err error) (model.ErrorKind, string) {
	var gerr *googleapi.Error
	if !errors.As(err, &gerr) {
		return model.ErrOther, err.Error()
	}
	msg := gerr.Message
	if msg == "" {
		msg = gerr.Error()
	}
	switch gerr.Code {
	case 403:
		for _, e := range gerr.Errors {
			if e.Reason == "accessNotConfigured" {
				return model.ErrAPIDisabled, msg
			}
		}
		if strings.Contains(msg, "has not been used") || strings.Contains(msg, "is disabled") ||
			strings.Contains(gerr.Body, "SERVICE_DISABLED") {
			return model.ErrAPIDisabled, msg
		}
		if strings.Contains(msg, "insufficient authentication scopes") {
			msg += i18n.T(" (el permiso concedido no cubre esta API: con inicio de sesión con Google, vuelve a iniciar sesión y acepta todos los permisos)")
		}
		return model.ErrPermission, msg
	case 404:
		return model.ErrNotFound, msg
	}
	return model.ErrOther, msg
}

func (c *collector) add(sheet string, r model.Row) {
	c.sheets[sheet].Add(r)
}

// ---------- small helpers ----------

// last returns the final path segment of a resource URL.
func last(u string) string {
	if i := strings.LastIndex(u, "/"); i >= 0 {
		return u[i+1:]
	}
	return u
}

// projectOf extracts the project id from a resource URL.
func projectOf(u string) string {
	const marker = "/projects/"
	i := strings.Index(u, marker)
	if i < 0 {
		return ""
	}
	rest := u[i+len(marker):]
	if j := strings.Index(rest, "/"); j >= 0 {
		return rest[:j]
	}
	return rest
}

func regionOfZone(zone string) string {
	if i := strings.LastIndex(zone, "-"); i > 0 {
		return zone[:i]
	}
	return zone
}

func ts(s string) any {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return t
}

func joinLast(urls []string) string {
	out := make([]string, len(urls))
	for i, u := range urls {
		out[i] = last(u)
	}
	return strings.Join(out, "; ")
}

func labels(m map[string]string) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = k + "=" + m[k]
	}
	return strings.Join(parts, "; ")
}

func encryption(k *compute.CustomerEncryptionKey) string {
	switch {
	case k == nil:
		return "Google"
	case k.KmsKeyName != "":
		return "CMEK"
	case k.Sha256 != "" || k.RsaEncryptedKey != "":
		return "CSEK"
	}
	return "Google"
}

func round2(f float64) float64 {
	return math.Round(f*100) / 100
}

func boolPtr(b *bool, def bool) bool {
	if b == nil {
		return def
	}
	return *b
}

// licenseCategory groups a license name into a billing-relevant family.
// The result is an internal (Spanish) value used for OS detection; rows get
// i18n.T of it ("Herramienta" and "Otra" have English entries).
func licenseCategory(name string) string {
	n := strings.ToLower(name)
	switch {
	// Licenses added by migration/import tooling, not an OS: skip them for OS detection.
	case strings.Contains(n, "vmmigration") || strings.Contains(n, "virtual-disk-import") ||
		strings.Contains(n, "migrate-to-vms"):
		return "Herramienta"
	case strings.Contains(n, "sql"):
		return "SQL Server"
	case strings.Contains(n, "windows"):
		return "Windows"
	case strings.Contains(n, "rhel"):
		return "RHEL"
	case strings.Contains(n, "sles"):
		return "SLES"
	case strings.Contains(n, "ubuntu-pro"):
		return "Ubuntu Pro"
	case strings.Contains(n, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(n, "debian"):
		return "Debian"
	case strings.Contains(n, "rocky"):
		return "Rocky Linux"
	case strings.Contains(n, "centos"):
		return "CentOS"
	case strings.HasPrefix(n, "cos"):
		return "Container-Optimized OS"
	}
	return "Otra"
}
