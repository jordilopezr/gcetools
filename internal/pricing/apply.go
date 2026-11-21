package pricing

import (
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gcetools/internal/collect"
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

func str(r model.Row, k string) string { s, _ := r[k].(string); return s }
func num(r model.Row, k string) float64 {
	switch v := r[k].(type) {
	case float64:
		return v
	case int64:
		return float64(v)
	}
	return 0
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
	sheet := func(n string) []model.Row {
		if s := inv.Sheet(n); s != nil {
			return s.Rows
		}
		return nil
	}
	pc := map[string]*projectCost{}
	proj := func(p string) *projectCost {
		if pc[p] == nil {
			pc[p] = &projectCost{}
		}
		return pc[p]
	}

	// Persistent disks
	diskCost := map[string]float64{} // project/location/name
	for _, r := range sheet(collect.SDisks) {
		loc := str(r, "location")
		cost, ok := x.Disk(regionOf(loc), str(r, "type"), str(r, "replicaZones") != "", num(r, "sizeGB"),
			int64(num(r, "iops")), int64(num(r, "throughput")))
		if !ok {
			miss(i18n.Tf("disco %s", str(r, "type")))
			continue
		}
		r["costMonthly"] = money(cost)
		diskCost[str(r, "project")+"/"+loc+"/"+str(r, "name")] = cost
		proj(str(r, "project")).disks += cost
	}

	// Per-VM disks (persistent from gDisks, local SSD priced here)
	type vmKey struct{ project, zone, name string }
	vmDisks := map[vmKey]float64{}
	vmMachine := map[vmKey]string{}
	for _, r := range sheet(collect.SInfo) {
		vmMachine[vmKey{str(r, "project"), str(r, "zone"), str(r, "name")}] = str(r, "machineType")
	}
	for _, r := range sheet(collect.SDisk) {
		k := vmKey{str(r, "project"), str(r, "zone"), str(r, "vm")}
		if str(r, "type") == "local-ssd" {
			if c, ok := x.LocalSSD(regionOf(k.zone), vmMachine[k], num(r, "sizeGB")); ok {
				vmDisks[k] += c
				proj(k.project).disks += c
			} else {
				miss("local SSD")
			}
			continue
		}
		c, ok := diskCost[k.project+"/"+k.zone+"/"+str(r, "disk")]
		if !ok {
			c = diskCost[k.project+"/"+regionOf(k.zone)+"/"+str(r, "disk")]
		}
		vmDisks[k] += c
	}

	// Licenses per VM
	vmLic := map[vmKey][]string{}
	for _, r := range sheet(collect.SLicense) {
		k := vmKey{str(r, "project"), str(r, "zone"), str(r, "vm")}
		vmLic[k] = append(vmLic[k], str(r, "license"))
	}

	// VMs
	for _, r := range sheet(collect.SInfo) {
		k := vmKey{str(r, "project"), str(r, "zone"), str(r, "name")}
		region := regionOf(k.zone)
		spot := str(r, "provisioning") == "SPOT" || str(r, "provisioning") == "PREEMPTIBLE"
		vcpu := int64(num(r, "vcpu"))
		var notes []string

		compute, ok := x.Machine(region, str(r, "machineType"), vcpu, num(r, "memoryGB"), spot)
		if !ok {
			notes = append(notes, i18n.T("machine type sin tarifa"))
			miss("machine type " + str(r, "machineType"))
		}
		for _, g := range strings.Split(str(r, "gpus"), "; ") {
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
			c, ok := x.License(l, vcpu, num(r, "memoryGB"))
			if !ok {
				notes = append(notes, i18n.Tf("licencia %s sin tarifa", l))
				miss(i18n.Tf("licencia %s", l))
			}
			lic += c
		}
		disks := vmDisks[k]
		total := disks
		if str(r, "status") == "RUNNING" {
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
	for _, r := range sheet(collect.SSnapshot) {
		loc := strings.Split(str(r, "locations"), "; ")[0]
		if c, ok := x.Snapshot(loc, str(r, "type") == "ARCHIVE", num(r, "storageGB")); ok {
			r["costMonthly"] = money(c)
			proj(str(r, "project")).snaps += c
		} else if loc != "" {
			miss(i18n.Tf("snapshots en %s", loc))
		}
	}
	for _, r := range sheet(collect.SImage) {
		loc := strings.Split(str(r, "locations"), "; ")[0]
		if c, ok := x.Image(loc, str(r, "kind") == "Machine image", num(r, "storageGB")); ok {
			r["costMonthly"] = money(c)
			proj(str(r, "project")).images += c
		} else if loc != "" {
			miss(i18n.Tf("imágenes en %s", loc))
		}
	}
	for _, r := range sheet(collect.SAddress) {
		if str(r, "status") != "RESERVED" || str(r, "type") != "EXTERNAL" || str(r, "location") == "global" {
			continue
		}
		if c, ok := x.StaticIP(str(r, "location")); ok {
			r["costMonthly"] = money(c)
			proj(str(r, "project")).ips += c
		}
	}

	// Summary
	projects := make([]string, 0, len(pc))
	for p := range pc {
		projects = append(projects, p)
	}
	sort.Strings(projects)
	out := inv.Sheet(collect.SCost)
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
	for _, r := range sheet(collect.SProject) {
		if c := pc[str(r, "projectId")]; c != nil {
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
