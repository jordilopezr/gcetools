package pricing

import (
	"gcetools/internal/schema"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

var reZone = regexp.MustCompile(`^[a-z]+-[a-z]+\d+-[a-z]$`)

func regionOf(loc string) string {
	if reZone.MatchString(loc) {
		return loc[:strings.LastIndex(loc, "-")]
	}
	return loc
}

func money(f float64) float64 { return math.Round(f*100) / 100 }

type projectCost struct {
	running                                      int64
	compute, licenses, disks, snaps, images, ips float64
}

// Result summarises how much of the inventory could be priced.
type Result struct {
	Unpriced []string // distinct items without a matching SKU
}

// Apply fills the cost columns of an inventory and the gCost summary sheet.
func Apply(inv *model.Inventory, x *Index) Result {
	unpriced := map[string]bool{}
	miss := func(what string) { unpriced[what] = true }
	pc := map[string]*projectCost{}
	proj := func(p string) *projectCost {
		if pc[p] == nil {
			pc[p] = &projectCost{}
		}
		return pc[p]
	}

	// Persistent disks
	diskCost := map[string]float64{} // project/location/name
	for _, r := range inv.Rows(schema.SDisks) {
		loc := r.Str("location")
		cost, ok := x.Disk(regionOf(loc), r.Str("type"), r.Str("replicaZones") != "", r.Float("sizeGB"),
			int64(r.Float("iops")), int64(r.Float("throughput")))
		if !ok {
			miss(i18n.Tf("disco %s", r.Str("type")))
			continue
		}
		r["costMonthly"] = money(cost)
		diskCost[r.Str("project")+"/"+loc+"/"+r.Str("name")] = cost
		proj(r.Str("project")).disks += cost
	}

	// Per-VM disks (persistent from gDisks, local SSD priced here)
	type vmKey struct{ project, zone, name string }
	vmDisks := map[vmKey]float64{}
	vmMachine := map[vmKey]string{}
	for _, r := range inv.Rows(schema.SInfo) {
		vmMachine[vmKey{r.Str("project"), r.Str("zone"), r.Str("name")}] = r.Str("machineType")
	}
	for _, r := range inv.Rows(schema.SDisk) {
		k := vmKey{r.Str("project"), r.Str("zone"), r.Str("vm")}
		if r.Str("type") == "local-ssd" {
			if c, ok := x.LocalSSD(regionOf(k.zone), vmMachine[k], r.Float("sizeGB")); ok {
				vmDisks[k] += c
				proj(k.project).disks += c
			} else {
				miss("local SSD")
			}
			continue
		}
		c, ok := diskCost[k.project+"/"+k.zone+"/"+r.Str("disk")]
		if !ok {
			c = diskCost[k.project+"/"+regionOf(k.zone)+"/"+r.Str("disk")]
		}
		vmDisks[k] += c
	}

	// Licenses per VM
	vmLic := map[vmKey][]string{}
	for _, r := range inv.Rows(schema.SLicense) {
		k := vmKey{r.Str("project"), r.Str("zone"), r.Str("vm")}
		vmLic[k] = append(vmLic[k], r.Str("license"))
	}

	// VMs
	for _, r := range inv.Rows(schema.SInfo) {
		k := vmKey{r.Str("project"), r.Str("zone"), r.Str("name")}
		region := regionOf(k.zone)
		spot := r.Str("provisioning") == "SPOT" || r.Str("provisioning") == "PREEMPTIBLE"
		vcpu := int64(r.Float("vcpu"))
		var notes []string

		compute, ok := x.Machine(region, r.Str("machineType"), vcpu, r.Float("memoryGB"), spot)
		if !ok {
			notes = append(notes, i18n.T("machine type sin tarifa"))
			miss("machine type " + r.Str("machineType"))
		}
		for _, g := range strings.Split(r.Str("gpus"), "; ") {
			if g == "" {
				continue
			}
			parts := strings.Split(g, " x")
			n, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
			if c, ok := x.GPU(region, parts[0], n, spot); ok {
				compute += c
			} else {
				notes = append(notes, i18n.T("GPU sin tarifa"))
				miss("GPU " + parts[0])
			}
		}
		var lic float64
		seen := map[string]bool{}
		for _, l := range vmLic[k] {
			if seen[l] {
				continue
			}
			seen[l] = true
			c, ok := x.License(l, vcpu, r.Float("memoryGB"))
			if !ok {
				notes = append(notes, i18n.Tf("licencia %s sin tarifa", l))
				miss(i18n.Tf("licencia %s", l))
			}
			lic += c
		}
		disks := vmDisks[k]
		total := disks
		if r.Str("status") == "RUNNING" {
			total += compute + lic
			p := proj(k.project)
			p.running++
			p.compute += compute
			p.licenses += lic
		}
		r["costCompute"], r["costLicense"], r["costDisks"] = money(compute), money(lic), money(disks)
		r["costMonthly"] = money(total)
		r["costNote"] = strings.Join(notes, "; ")
	}

	// Snapshots, images, IPs
	for _, r := range inv.Rows(schema.SSnapshot) {
		loc := strings.Split(r.Str("locations"), "; ")[0]
		if c, ok := x.Snapshot(loc, r.Str("type") == "ARCHIVE", r.Float("storageGB")); ok {
			r["costMonthly"] = money(c)
			proj(r.Str("project")).snaps += c
		} else if loc != "" {
			miss(i18n.Tf("snapshots en %s", loc))
		}
	}
	for _, r := range inv.Rows(schema.SImage) {
		loc := strings.Split(r.Str("locations"), "; ")[0]
		if c, ok := x.Image(loc, r.Str("kind") == "Machine image", r.Float("storageGB")); ok {
			r["costMonthly"] = money(c)
			proj(r.Str("project")).images += c
		} else if loc != "" {
			miss(i18n.Tf("imágenes en %s", loc))
		}
	}
	for _, r := range inv.Rows(schema.SAddress) {
		if r.Str("status") != "RESERVED" || r.Str("type") != "EXTERNAL" || r.Str("location") == "global" {
			continue
		}
		if c, ok := x.StaticIP(r.Str("location")); ok {
			r["costMonthly"] = money(c)
			proj(r.Str("project")).ips += c
		}
	}

	// Summary
	projects := make([]string, 0, len(pc))
	for p := range pc {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	out := inv.Sheet(schema.SCost)
	var all projectCost
	for _, p := range projects {
		c := pc[p]
		out.Add(costRow(p, c, x.Currency))
		all.running += c.running
		all.compute += c.compute
		all.licenses += c.licenses
		all.disks += c.disks
		all.snaps += c.snaps
		all.images += c.images
		all.ips += c.ips
	}
	if len(projects) > 1 {
		out.Add(costRow("TOTAL", &all, x.Currency))
	}
	for _, r := range inv.Rows(schema.SProject) {
		if c := pc[r.Str("projectId")]; c != nil {
			r["costMonthly"] = money(c.compute + c.licenses + c.disks + c.snaps + c.images + c.ips)
		}
	}

	res := Result{}
	for k := range unpriced {
		res.Unpriced = append(res.Unpriced, k)
	}
	sort.Strings(res.Unpriced)
	return res
}

func costRow(p string, c *projectCost, cur string) model.Row {
	return model.Row{
		"project": p, "vmsRunning": c.running, "compute": money(c.compute), "licenses": money(c.licenses),
		"disks": money(c.disks), "snapshots": money(c.snaps), "images": money(c.images), "ips": money(c.ips),
		"total":    money(c.compute + c.licenses + c.disks + c.snaps + c.images + c.ips),
		"currency": cur,
	}
}
