package pricing

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
)

// Index answers price lookups against a catalog. SKU descriptions are the
// only stable join key the catalog offers, so matching is by description
// prefix ("N2 Instance Core running in …") plus the SKU's service regions.
//
// A scan asks the same few questions thousands of times (one per VM, disk
// and snapshot), so descriptions are lowercased once and answers are
// memoized: without that, pricing 20,000 VMs took 12-15 s and 3.9 GB.
type Index struct {
	Currency string
	skus     []SKU
	lower    []string // lowercase descriptions, same order as skus
	byRegion map[string][]int

	mu    sync.Mutex
	finds map[string]found
	other map[string]found // License and GPU answers
}

type found struct {
	sku   SKU
	price float64
	ok    bool
}

func NewIndex(c *Catalog) *Index {
	x := &Index{Currency: c.Currency, skus: c.SKUs, lower: make([]string, len(c.SKUs)), byRegion: map[string][]int{},
		finds: map[string]found{}, other: map[string]found{}}
	for i, s := range c.SKUs {
		x.lower[i] = strings.ToLower(s.Desc)
		for _, r := range s.Regions {
			x.byRegion[r] = append(x.byRegion[r], i)
		}
	}
	return x
}

// memo returns the cached answer for key or computes and stores it.
func (x *Index) memo(cache map[string]found, key string, compute func() found) found {
	x.mu.Lock()
	f, ok := cache[key]
	x.mu.Unlock()
	if ok {
		return f
	}
	f = compute()
	x.mu.Lock()
	cache[key] = f
	x.mu.Unlock()
	return f
}

// find returns the first SKU in region whose usage type matches and whose
// description starts with one of the prefixes (case-insensitive), trying
// prefixes in order of preference.
func (x *Index) find(region, usage string, prefixes ...string) (SKU, bool) {
	key := region + "\x00" + usage + "\x00" + strings.Join(prefixes, "\x00")
	f := x.memo(x.finds, key, func() found {
		s, ok := x.findUncached(region, usage, prefixes...)
		return found{sku: s, ok: ok}
	})
	return f.sku, f.ok
}

func (x *Index) findUncached(region, usage string, prefixes ...string) (SKU, bool) {
	cands := x.byRegion[region]
	for _, p := range prefixes {
		p = strings.ToLower(p)
		// Older regions (us-central1…) publish SKUs without the location
		// suffix: "Balanced PD Capacity" instead of "… Capacity in Iowa".
		bare := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(p, " "), " in"), " running")
		for _, i := range cands {
			s, d := x.skus[i], x.lower[i]
			if (usage == "" || s.UsageType == usage) && (strings.HasPrefix(d, p) || d == bare) {
				return s, true
			}
		}
	}
	return SKU{}, false
}

// findMulti matches multi-region SKUs (snapshots/images stored in "us", "eu",
// "asia"). The catalog publishes them without a location suffix, e.g. the
// description is exactly "Storage PD Snapshot" and lists several regions.
func (x *Index) findMulti(multi, usage, desc string) (SKU, bool) {
	switch strings.ToLower(multi) {
	case "us", "eu", "asia":
	default:
		return SKU{}, false
	}
	key := "multi\x00" + usage + "\x00" + strings.ToLower(desc) // same SKU for us, eu and asia
	f := x.memo(x.finds, key, func() found {
		d := strings.ToLower(desc)
		for i, s := range x.skus {
			if len(s.Regions) > 1 && (usage == "" || s.UsageType == usage) && x.lower[i] == d {
				return found{sku: s, ok: true}
			}
		}
		return found{}
	})
	return f.sku, f.ok
}

// ---------- machines ----------

var familyLabel = map[string]string{
	"e2": "E2", "n1": "N1 Predefined", "n2": "N2", "n2d": "N2D AMD", "n4": "N4", "n4d": "N4D", "n4a": "N4A",
	"c2": "Compute optimized", "c2d": "C2D AMD", "c3": "C3", "c3d": "C3D", "c4": "C4", "c4a": "C4A", "c4d": "C4D",
	"t2d": "T2D AMD", "t2a": "T2A Arm", "m1": "Memory-optimized", "m2": "Memory-optimized", "m3": "M3 Memory-optimized",
	"m4": "M4", "a2": "A2", "a3": "A3", "a4": "A4", "g2": "G2", "g4": "G4", "h3": "H3", "h4d": "H4D", "z3": "Z3", "x4": "X4",
}

// standard memory per vCPU before "extended memory" pricing applies
var maxGBPerVCPU = map[string]float64{"n1": 6.5, "n2": 8, "n2d": 8, "e2": 8, "n4": 8, "n4d": 8}

// Machine returns the monthly list price of a VM's vCPU and memory.
func (x *Index) Machine(region, machineType string, vcpu int64, memGB float64, spot bool) (float64, bool) {
	usage, pre := "OnDemand", ""
	if spot {
		usage, pre = "Preemptible", "Spot Preemptible "
	}
	hourly := func(prefixes ...string) (float64, bool) {
		for i := range prefixes {
			prefixes[i] = pre + prefixes[i] + " running in "
		}
		s, ok := x.find(region, usage, prefixes...)
		return s.Price, ok
	}

	switch machineType {
	case "f1-micro":
		p, ok := hourly("Micro Instance with burstable CPU")
		return p * HoursPerMonth, ok
	case "g1-small":
		p, ok := hourly("Small Instance with 1 VCPU")
		return p * HoursPerMonth, ok
	}

	fam := strings.SplitN(machineType, "-", 2)[0]
	custom := strings.Contains(machineType, "custom")
	if fam == "custom" { // legacy N1 custom: custom-4-16384
		fam = "n1"
	}
	label, known := familyLabel[fam]
	if !known {
		return 0, false
	}
	cores := float64(vcpu)
	switch machineType { // E2 shared-core: fractional vCPU
	case "e2-micro":
		cores = 0.25
	case "e2-small":
		cores = 0.5
	case "e2-medium":
		cores = 1
	}

	var corePfx, ramPfx, extPfx []string
	if custom {
		cl := label + " Custom"
		if fam == "n1" {
			cl = "Custom"
		}
		corePfx = []string{cl + " Instance Core", cl + " Core"}
		ramPfx = []string{cl + " Instance Ram", cl + " Ram"}
		extPfx = []string{cl + " Extended Instance Ram", cl + " Extended Ram"}
	} else {
		corePfx = []string{label + " Instance Core", label + " Core"}
		ramPfx = []string{label + " Instance Ram", label + " Ram"}
	}
	core, ok1 := hourly(corePfx...)
	ram, ok2 := hourly(ramPfx...)
	if !ok1 || !ok2 {
		return 0, false
	}
	stdMem, extMem := memGB, 0.0
	if strings.HasSuffix(machineType, "-ext") && maxGBPerVCPU[fam] > 0 {
		stdMem = math.Min(memGB, cores*maxGBPerVCPU[fam])
		extMem = memGB - stdMem
	}
	total := cores*core + stdMem*ram
	if extMem > 0 {
		ext, ok := hourly(extPfx...)
		if !ok {
			return 0, false
		}
		total += extMem * ext
	}
	if fam == "m2" && !spot {
		if p, ok := hourly("Memory Optimized Upgrade Premium for Memory-optimized Instance Core"); ok {
			total += cores * p
		}
		if p, ok := hourly("Memory Optimized Upgrade Premium for Memory-optimized Instance Ram"); ok {
			total += memGB * p
		}
	}
	return total * HoursPerMonth, true
}

// GPU returns the monthly price of count accelerators of the given type
// (e.g. "nvidia-tesla-t4", "nvidia-l4").
func (x *Index) GPU(region, accelType string, count int64, spot bool) (float64, bool) {
	f := x.memo(x.other, fmt.Sprintf("gpu\x00%s\x00%s\x00%v", region, accelType, spot), func() found {
		p, ok := x.gpuHourly(region, accelType, spot)
		return found{price: p, ok: ok}
	})
	return f.price * float64(count) * HoursPerMonth, f.ok
}

func (x *Index) gpuHourly(region, accelType string, spot bool) (float64, bool) {
	tokens := strings.Split(strings.ToLower(accelType), "-")
	usage := "OnDemand"
	if spot {
		usage = "Preemptible"
	}
	for _, i := range x.byRegion[region] {
		s, d := x.skus[i], x.lower[i]
		if s.UsageType != usage || !strings.Contains(d, "gpu running in") ||
			strings.Contains(d, "commit") || strings.Contains(d, "dws") || strings.Contains(d, "sole tenancy") ||
			strings.Contains(d, "workstation") || strings.Contains(d, "reserved") || (!spot && strings.Contains(d, "spot")) {
			continue
		}
		all := true
		for _, t := range tokens {
			if !strings.Contains(d, t) {
				all = false
				break
			}
		}
		if all {
			return s.Price, true
		}
	}
	return 0, false
}

// ---------- disks, snapshots, images, IPs ----------

type diskSpec struct {
	capacity   string
	iops       string
	throughput string
	freeIOPS   int64
	freeMBps   int64
}

var diskSpecs = map[string]diskSpec{
	"pd-standard":                          {capacity: "Storage PD Capacity"},
	"pd-balanced":                          {capacity: "Balanced PD Capacity"},
	"pd-ssd":                               {capacity: "SSD backed PD Capacity"},
	"pd-extreme":                           {capacity: "Extreme PD Capacity", iops: "Extreme PD IOPS"},
	"hyperdisk-balanced":                   {capacity: "Hyperdisk Balanced Capacity", iops: "Hyperdisk Balanced IOPS", throughput: "Hyperdisk Balanced Throughput", freeIOPS: 3000, freeMBps: 140},
	"hyperdisk-balanced-high-availability": {capacity: "Hyperdisk Balanced High Availability Capacity", iops: "Hyperdisk Balanced High Availability Iops", throughput: "Hyperdisk Balanced High Availability Throughput", freeIOPS: 3000, freeMBps: 140},
	"hyperdisk-extreme":                    {capacity: "Hyperdisk Extreme Capacity", iops: "Hyperdisk Extreme IOPS"},
	"hyperdisk-throughput":                 {capacity: "Hyperdisk Throughput Capacity", throughput: "Hyperdisk Throughput Throughput"},
	"hyperdisk-ml":                         {capacity: "Hyperdisk ML Capacity", throughput: "Hyperdisk ML Throughput"},
}

// Disk returns the monthly price of a persistent disk. regional selects the
// synchronously replicated (regional PD) SKUs.
func (x *Index) Disk(region, diskType string, regional bool, sizeGB float64, iops, mbps int64) (float64, bool) {
	spec, ok := diskSpecs[diskType]
	if !ok {
		return 0, false
	}
	pfx := ""
	if regional && strings.HasPrefix(diskType, "pd-") {
		pfx = "Regional "
	}
	capSKU, ok := x.find(region, "OnDemand", pfx+spec.capacity+" in ")
	if !ok {
		return 0, false
	}
	total := sizeGB * capSKU.Price
	if spec.iops != "" && iops > spec.freeIOPS {
		if s, ok := x.find(region, "OnDemand", spec.iops+" in "); ok {
			total += float64(iops-spec.freeIOPS) * s.Price
		}
	}
	if spec.throughput != "" && mbps > spec.freeMBps {
		if s, ok := x.find(region, "OnDemand", spec.throughput+" in "); ok {
			total += float64(mbps-spec.freeMBps) * s.Price
		}
	}
	return total, true
}

// LocalSSD prices local SSD capacity attached to a VM of the given family.
func (x *Index) LocalSSD(region, machineType string, sizeGB float64) (float64, bool) {
	fam := strings.SplitN(machineType, "-", 2)[0]
	pfx := []string{"SSD backed Local Storage in ", "SSD backed Local Storage attached to"}
	if l, ok := familyLabel[fam]; ok {
		pfx = append([]string{l + " Instance Local SSD running in "}, pfx...)
	}
	s, ok := x.find(region, "OnDemand", pfx...)
	if !ok {
		return 0, false
	}
	if s.Unit == "GiBy.h" {
		return sizeGB * s.Price * HoursPerMonth, true
	}
	return sizeGB * s.Price, true
}

func (x *Index) storage(location, prefix string, gb float64) (float64, bool) {
	if strings.Contains(location, "-") { // a region
		s, ok := x.find(location, "OnDemand", prefix+" in ")
		return gb * s.Price, ok
	}
	s, ok := x.findMulti(location, "OnDemand", prefix)
	if !ok && prefix == "Storage Machine Image" {
		s, ok = x.findMulti(location, "OnDemand", "Storage Image")
	}
	return gb * s.Price, ok
}

// Snapshot prices snapshot storage at its storage location (region or multi-region).
func (x *Index) Snapshot(location string, archive bool, gb float64) (float64, bool) {
	if archive {
		return x.storage(location, "Regional Archive Snapshot Data Storage", gb)
	}
	return x.storage(location, "Storage PD Snapshot", gb)
}

// Image prices custom image (or machine image) storage.
func (x *Index) Image(location string, machineImage bool, gb float64) (float64, bool) {
	if machineImage {
		return x.storage(location, "Storage Machine Image", gb)
	}
	return x.storage(location, "Storage Image", gb)
}

// StaticIP prices a reserved, unused regional external IP for a month.
func (x *Index) StaticIP(region string) (float64, bool) {
	s, ok := x.find(region, "OnDemand", "Static Ip Charge in ")
	return s.Price * HoursPerMonth, ok
}

// ---------- OS / software licenses ----------

var (
	reYear  = regexp.MustCompile(`(19|20)\d\d`)
	reMajor = regexp.MustCompile(`-(\d{1,2})(?:-|$)`)
	reTiers = []struct {
		re   *regexp.Regexp
		tier func(m []string) (int64, int64)
	}{
		{regexp.MustCompile(`with up to (\d+) vcpu`), func(m []string) (int64, int64) { return 1, atoi(m[1]) }},
		{regexp.MustCompile(`with (\d+) to (\d+) vcpu`), func(m []string) (int64, int64) { return atoi(m[1]), atoi(m[2]) }},
		{regexp.MustCompile(`with (\d+) (?:vcpu )?or more`), func(m []string) (int64, int64) { return atoi(m[1]), math.MaxInt32 }},
		{regexp.MustCompile(`with more than (\d+) vcpu`), func(m []string) (int64, int64) { return atoi(m[1]) + 1, math.MaxInt32 }},
		{regexp.MustCompile(`with (\d+) vcpu`), func(m []string) (int64, int64) { return atoi(m[1]), atoi(m[1]) }},
	}
)

func atoi(s string) int64 { n, _ := strconv.ParseInt(s, 10, 64); return n }

// FreeLicense reports licenses that never carry a fee (BYOL/BYOS, free distros, tooling).
func FreeLicense(name string) bool {
	n := strings.ToLower(name)
	for _, k := range []string{"byol", "byos", "debian", "rocky", "centos", "cos-", "container-vm", "vmmigration",
		"virtual-disk-import", "almalinux", "fedora", "freebsd", "opensuse"} {
		if strings.Contains(n, k) {
			return true
		}
	}
	return strings.HasPrefix(n, "ubuntu-") && !strings.Contains(n, "pro")
}

var reUbuntuVer = regexp.MustCompile(`(\d{2})(\d{2})`)

// License returns the monthly fee of a premium license for a VM with vcpu and
// memGB. matched is false when the license is premium but no SKU could be found.
func (x *Index) License(name string, vcpu int64, memGB float64) (fee float64, matched bool) {
	f := x.memo(x.other, fmt.Sprintf("lic\x00%s\x00%d\x00%g", name, vcpu, memGB), func() found {
		p, ok := x.license(name, vcpu, memGB)
		return found{price: p, ok: ok}
	})
	return f.price, f.ok
}

func (x *Index) license(name string, vcpu int64, memGB float64) (fee float64, matched bool) {
	if FreeLicense(name) {
		return 0, true
	}
	n := strings.ToLower(name)
	if strings.HasPrefix(n, "ubuntu") && strings.Contains(n, "pro") {
		return x.ubuntuPro(n, vcpu, memGB)
	}
	var need, anyOf []string
	var avoid = []string{"commitment", "f1-micro", "g1-small", "byos", "byol", "(ram cost)", "(gpu cost)", "cloud sql", "extended life"}
	perVCPU := false
	major := ""
	if m := reMajor.FindStringSubmatch(n); m != nil {
		major = m[1]
	}
	switch {
	case strings.HasPrefix(n, "windows-server"):
		y := reYear.FindString(n)
		ed := "datacenter"
		if strings.Contains(n, "std") || strings.Contains(n, "standard") {
			ed = "standard"
		}
		need = []string{"windows server " + y + " " + ed + " edition"}
		anyOf = []string{"on vm", "(cpu cost)"}
		perVCPU = true
	case strings.HasPrefix(n, "sql-server") || strings.HasPrefix(n, "sql-"):
		y := reYear.FindString(n)
		ed := ""
		for _, e := range []string{"enterprise", "standard", "web", "express"} {
			if strings.Contains(n, e) {
				ed = e
			}
		}
		if ed == "express" {
			return 0, true
		}
		need = []string{"sql server " + y + " " + ed + " on vm"}
		avoid = append(avoid, "on linux")
	case strings.HasPrefix(n, "rhel"):
		need = []string{"red hat enterprise linux " + major + " "}
		if strings.Contains(n, "sap") {
			need = append(need, "for sap")
		} else {
			avoid = append(avoid, "sap")
		}
		perVCPU = true
	case strings.HasPrefix(n, "sles-sap") || (strings.HasPrefix(n, "sles") && strings.Contains(n, "sap")):
		need = []string{" " + major + " for sap"}
	case strings.HasPrefix(n, "sles"):
		need = []string{"suse linux enterprise server " + major + " on vm"}
	default:
		return 0, false
	}

	for i, s := range x.skus {
		if s.Family != "License" || s.Unit != "h" {
			continue
		}
		d := x.lower[i]
		ok := true
		for _, k := range need {
			if !strings.Contains(d, k) {
				ok = false
				break
			}
		}
		for _, k := range avoid {
			if ok && strings.Contains(d, k) {
				ok = false
			}
		}
		if ok && len(anyOf) > 0 {
			ok = false
			for _, k := range anyOf {
				if strings.Contains(d, k) {
					ok = true
				}
			}
		}
		if !ok {
			continue
		}
		lo, hi, tiered := int64(1), int64(math.MaxInt32), false
		for _, t := range reTiers {
			if m := t.re.FindStringSubmatch(d); m != nil {
				lo, hi = t.tier(m)
				tiered = true
				break
			}
		}
		if vcpu < lo || vcpu > hi {
			continue
		}
		per := perVCPU || (tiered && strings.Contains(d, "more than"))
		if per {
			return s.Price * float64(vcpu) * HoursPerMonth, true
		}
		return s.Price * HoursPerMonth, true
	}
	return 0, false
}

// ubuntuPro prices Ubuntu Pro (also FIPS and Minimal Pro): a flat hourly fee
// per VM by vCPU tier plus a fee per GiB of RAM. License names look like
// ubuntu-pro-2204-lts, ubuntu-pro-fips-2004-lts or ubuntu-minimal-pro-2404-lts.
func (x *Index) ubuntuPro(n string, vcpu int64, memGB float64) (float64, bool) {
	m := reUbuntuVer.FindStringSubmatch(n)
	if m == nil {
		return 0, false
	}
	ver := m[1] + "." + m[2]
	prefix := "ubuntu pro " + ver + " "
	switch {
	case strings.Contains(n, "minimal"):
		prefix = "ubuntu " + ver + " lts minimal pro "
	case strings.Contains(n, "fips"):
		prefix = "ubuntu pro fips " + ver + " "
	}
	cpu, ram := -1.0, -1.0
	for i, s := range x.skus {
		if s.Family != "License" {
			continue
		}
		d := x.lower[i]
		if !strings.Contains(d, prefix) {
			continue
		}
		switch {
		case s.Unit == "GiBy.h" && strings.Contains(d, "(ram cost)"):
			ram = s.Price
		case s.Unit == "h" && strings.Contains(d, "(cpu cost)") && cpu < 0:
			for _, t := range reTiers {
				if tm := t.re.FindStringSubmatch(d); tm != nil {
					if lo, hi := t.tier(tm); vcpu >= lo && vcpu <= hi {
						cpu = s.Price
					}
					break
				}
			}
		}
	}
	if cpu < 0 {
		return 0, false
	}
	fee := cpu
	if ram > 0 {
		fee += ram * memGB
	}
	return fee * HoursPerMonth, true
}
