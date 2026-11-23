package pricing

import (
	"gcetools/internal/schema"
	"math"
	"os"
	"testing"

	"gcetools/internal/model"
)

const R = "southamerica-west1"

func sku(desc, fam, usage, unit string, price float64, regions ...string) SKU {
	if len(regions) == 0 {
		regions = []string{R}
	}
	return SKU{Desc: desc, Family: fam, UsageType: usage, Unit: unit, Price: price, Regions: regions}
}

func fakeIndex() *Index {
	c := &Catalog{Currency: "USD", SKUs: []SKU{
		sku("N2 Instance Core running in Santiago", "Compute", "OnDemand", "h", 0.045204),
		sku("N2 Instance Ram running in Santiago", "Compute", "OnDemand", "GiBy.h", 0.006059),
		sku("N2D AMD Instance Core running in Santiago", "Compute", "OnDemand", "h", 0.039328), // must not match n2
		sku("N2 Custom Instance Core running in Santiago", "Compute", "OnDemand", "h", 0.047464),
		sku("N2 Custom Instance Ram running in Santiago", "Compute", "OnDemand", "GiBy.h", 0.006362),
		sku("N2 Custom Extended Instance Ram running in Santiago", "Compute", "OnDemand", "GiBy.h", 0.013657),
		sku("Spot Preemptible N2 Instance Core running in Santiago", "Compute", "Preemptible", "h", 0.01153),
		sku("Spot Preemptible N2 Instance Ram running in Santiago", "Compute", "Preemptible", "GiBy.h", 0.001544),
		sku("E2 Instance Core running in Santiago", "Compute", "OnDemand", "h", 0.031191),
		sku("E2 Instance Ram running in Santiago", "Compute", "OnDemand", "GiBy.h", 0.004181),
		sku("Balanced PD Capacity in Santiago", "Storage", "OnDemand", "GiBy.mo", 0.143),
		sku("Regional Balanced PD Capacity in Santiago", "Storage", "OnDemand", "GiBy.mo", 0.286),
		sku("Hyperdisk Balanced Capacity in Santiago", "Storage", "OnDemand", "GiBy.mo", 0.114),
		sku("Hyperdisk Balanced Capacity Confidential Mode in Santiago", "Storage", "OnDemand", "GiBy.mo", 0.5),
		sku("Hyperdisk Balanced IOPS in Santiago", "Storage", "OnDemand", "mo", 0.007),
		sku("Hyperdisk Balanced Throughput in Santiago", "Storage", "OnDemand", "mo", 0.057),
		sku("Storage PD Snapshot in Santiago", "Storage", "OnDemand", "GiBy.mo", 0.071),
		sku("Storage PD Snapshot", "Storage", "OnDemand", "GiBy.mo", 0.05, "us-central1", "europe-west1", "asia-east1"),
		sku("Static Ip Charge in Santiago", "Network", "OnDemand", "h", 0.015),
		sku("Licensing Fee for Windows Server 2022 Datacenter Edition on VM", "License", "OnDemand", "h", 0.046, "global"),
		sku("Licensing Fee for Ubuntu Pro 22.04 LTS (Jammy Jellyfish) on VM with up to 1 VCPU (CPU cost)", "License", "OnDemand", "h", 0.00166, "global"),
		sku("Licensing Fee for Ubuntu Pro 22.04 LTS (Jammy Jellyfish) on VM with 4 to 5 VCPU (CPU cost)", "License", "OnDemand", "h", 0.006, "global"),
		sku("Licensing Fee for Ubuntu Pro 22.04 LTS (Jammy Jellyfish) on VM with 222 or more VCPU (CPU cost)", "License", "OnDemand", "h", 0.122063, "global"),
		sku("Licensing Fee for Ubuntu Pro 22.04 LTS (Jammy Jellyfish) on VM (RAM cost)", "License", "OnDemand", "GiBy.h", 0.000127, "global"),
		sku("Licensing Fee for Ubuntu Pro 22.04 LTS (Jammy Jellyfish) on VM with 2 to 3 GPU (GPU cost)", "License", "OnDemand", "h", 0.066, "global"),
		sku("Licensing Fee for Ubuntu Pro FIPS 22.04 LTS (Jammy Jellyfish) on VM with 4 to 5 VCPU (CPU cost)", "License", "OnDemand", "h", 0.9, "global"),
		sku("Licensing Fee for Ubuntu 24.04 LTS Minimal Pro (Noble Numbat) on VM with 4 to 5 VCPU (CPU cost)", "License", "OnDemand", "h", 0.007, "global"),
		sku("Licensing Fee for Ubuntu 24.04 LTS Minimal Pro (Noble Numbat) on VM (RAM cost)", "License", "OnDemand", "GiBy.h", 0.0001, "global"),
		sku("Licensing Fee for Windows Server 2022 Datacenter Edition on f1-micro", "License", "OnDemand", "h", 0.023, "global"),
		sku("Licensing Fee for SLES 15 For SAP License on VM with 1 to 3 VCPU", "License", "OnDemand", "h", 0.17, "global"),
		sku("Licensing Fee for SLES 15 For SAP License on VM with 4 VCPU", "License", "OnDemand", "h", 0.34, "global"),
		sku("Licensing Fee for SLES 15 For SAP License on VM with 6 or more VCPU", "License", "OnDemand", "h", 0.41, "global"),
		sku("Licensing Fee for Red Hat Enterprise Linux 9 on VM with up to 8 VCPU", "License", "OnDemand", "h", 0.0144, "global"),
		sku("Licensing Fee for Red Hat Enterprise Linux 9 on VM with 9 to 127 VCPU", "License", "OnDemand", "h", 0.0108, "global"),
		sku("Licensing Fee for SQL Server 2019 Enterprise on VM with 1 to 4 VCPU", "License", "OnDemand", "h", 1.596, "global"),
		sku("Licensing Fee for SQL Server 2019 Enterprise on VM with more than 4 VCPU", "License", "OnDemand", "h", 0.399, "global"),
	}}
	return NewIndex(c)
}

func near(t *testing.T, name string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.01 {
		t.Errorf("%s = %.4f, want %.4f", name, got, want)
	}
}

func TestMachine(t *testing.T) {
	x := fakeIndex()
	cases := []struct {
		mt   string
		vcpu int64
		mem  float64
		spot bool
		want float64
	}{
		{"n2-standard-16", 16, 64, false, (16*0.045204 + 64*0.006059) * 730},
		{"n2-custom-8-16384", 8, 16, false, (8*0.047464 + 16*0.006362) * 730},
		{"n2-custom-4-40960-ext", 4, 40, false, (4*0.047464 + 32*0.006362 + 8*0.013657) * 730},
		{"n2-standard-4", 4, 16, true, (4*0.01153 + 16*0.001544) * 730},
		{"e2-micro", 2, 1, false, (0.25*0.031191 + 1*0.004181) * 730},
	}
	for _, c := range cases {
		got, ok := x.Machine(R, c.mt, c.vcpu, c.mem, c.spot)
		if !ok {
			t.Errorf("%s: no match", c.mt)
			continue
		}
		near(t, c.mt, got, c.want)
	}
	if _, ok := x.Machine(R, "c3-standard-4", 4, 16, false); ok {
		t.Error("c3 should not match a catalog without C3 SKUs")
	}
}

func TestDisksAndStorage(t *testing.T) {
	x := fakeIndex()
	got, _ := x.Disk(R, "pd-balanced", false, 100, 0, 0)
	near(t, "pd-balanced", got, 14.3)
	got, _ = x.Disk(R, "pd-balanced", true, 100, 0, 0)
	near(t, "regional pd-balanced", got, 28.6)
	got, _ = x.Disk(R, "hyperdisk-balanced", false, 100, 5000, 290)
	near(t, "hyperdisk-balanced", got, 100*0.114+2000*0.007+150*0.057)
	got, _ = x.Snapshot("us", false, 10)
	near(t, "multi-region snapshot", got, 0.5)
	got, _ = x.Snapshot(R, false, 10)
	near(t, "regional snapshot", got, 0.71)
	got, _ = x.StaticIP(R)
	near(t, "static ip", got, 0.015*730)
}

func TestLicenses(t *testing.T) {
	x := fakeIndex()
	cases := []struct {
		lic  string
		vcpu int64
		want float64
	}{
		{"windows-server-2022-dc", 8, 8 * 0.046 * 730},
		{"sles-sap-15", 2, 0.17 * 730},
		{"sles-sap-15", 32, 0.41 * 730},
		{"sles-sap-15-byos", 32, 0},
		{"rhel-9", 4, 4 * 0.0144 * 730},
		{"rhel-9", 16, 16 * 0.0108 * 730},
		{"sql-server-2019-enterprise", 4, 1.596 * 730},
		{"sql-server-2019-enterprise", 8, 8 * 0.399 * 730},
		{"debian-12-bookworm", 4, 0},
		{"ubuntu-pro-2204-lts", 4, (0.006 + 0.000127*16) * 730},
		{"ubuntu-pro-2204-lts", 1, (0.00166 + 0.000127*16) * 730},
		{"ubuntu-pro-2204-lts", 240, (0.122063 + 0.000127*16) * 730},
		{"ubuntu-pro-fips-2204-lts", 4, 0.9 * 730},
		{"ubuntu-minimal-pro-2404-lts", 4, (0.007 + 0.0001*16) * 730},
		{"ubuntu-2204-jammy-v20240501", 4, 0},
	}
	for _, c := range cases {
		got, ok := x.License(c.lic, c.vcpu, 16)
		if !ok {
			t.Errorf("%s/%d: no match", c.lic, c.vcpu)
			continue
		}
		near(t, c.lic, got, c.want)
	}
	if _, ok := x.License("windows-server-2025-dc", 4, 16); ok {
		t.Error("2025 must not match 2022 SKUs")
	}
}

func TestApply(t *testing.T) {
	inv := schema.NewInventory()
	inv.Sheet(schema.SInfo).Add(model.Row{"project": "p", "name": "vm", "zone": R + "-a", "status": "RUNNING",
		"machineType": "n2-standard-16", "vcpu": int64(16), "memoryGB": 64.0, "provisioning": "STANDARD"})
	inv.Sheet(schema.SInfo).Add(model.Row{"project": "p", "name": "off", "zone": R + "-a", "status": "TERMINATED",
		"machineType": "n2-standard-16", "vcpu": int64(16), "memoryGB": 64.0, "provisioning": "STANDARD"})
	inv.Sheet(schema.SDisks).Add(model.Row{"project": "p", "name": "vm", "location": R + "-a", "type": "pd-balanced", "sizeGB": 100.0})
	inv.Sheet(schema.SDisks).Add(model.Row{"project": "p", "name": "off", "location": R + "-a", "type": "pd-balanced", "sizeGB": 50.0})
	inv.Sheet(schema.SDisk).Add(model.Row{"project": "p", "vm": "vm", "zone": R + "-a", "disk": "vm", "type": "pd-balanced", "sizeGB": 100.0})
	inv.Sheet(schema.SDisk).Add(model.Row{"project": "p", "vm": "off", "zone": R + "-a", "disk": "off", "type": "pd-balanced", "sizeGB": 50.0})
	inv.Sheet(schema.SLicense).Add(model.Row{"project": "p", "vm": "vm", "zone": R + "-a", "license": "windows-server-2022-dc"})

	res := Apply(inv, fakeIndex())
	if len(res.Unpriced) != 0 {
		t.Errorf("unpriced: %v", res.Unpriced)
	}
	vm, off := inv.Sheet(schema.SInfo).Rows[0], inv.Sheet(schema.SInfo).Rows[1]
	compute := (16*0.045204 + 64*0.006059) * 730
	lic := 16 * 0.046 * 730
	near(t, "running total", vm["costMonthly"].(float64), compute+lic+14.3)
	near(t, "stopped total (disks only)", off["costMonthly"].(float64), 7.15)
	near(t, "stopped compute shown", off["costCompute"].(float64), compute)
	sum := inv.Sheet(schema.SCost).Rows
	if len(sum) != 1 {
		t.Fatalf("gCost rows = %v", sum)
	}
	near(t, "project total", sum[0]["total"].(float64), compute+lic+14.3+7.15)
}

// TestRealCatalog validates matching against a downloaded catalog:
// GCETOOLS_CATALOG=~/.cache/gcetools/compute-skus-USD.json go test ./internal/pricing -run Real -v
func TestRealCatalog(t *testing.T) {
	path := os.Getenv("GCETOOLS_CATALOG")
	if path == "" {
		t.Skip("set GCETOOLS_CATALOG to a cached catalog file")
	}
	c, err := readCache(path)
	if err != nil {
		t.Fatal(err)
	}
	x := NewIndex(c)
	region := "us-central1"
	for _, m := range []struct {
		mt   string
		vcpu int64
		mem  float64
	}{
		{"n1-standard-4", 4, 15}, {"custom-2-4096", 2, 4}, {"n2-standard-4", 4, 16}, {"n2d-standard-4", 4, 16},
		{"n2-custom-4-40960-ext", 4, 40}, {"e2-medium", 2, 4}, {"e2-standard-8", 8, 32}, {"c2-standard-4", 4, 16},
		{"c2d-standard-4", 4, 16}, {"c3-standard-4", 4, 16}, {"c3d-standard-4", 4, 16}, {"c4-standard-4", 4, 15},
		{"n4-standard-4", 4, 16}, {"t2d-standard-4", 4, 16}, {"m1-megamem-96", 96, 1433.6}, {"m2-ultramem-208", 208, 5888},
		{"m3-megamem-64", 64, 976}, {"g2-standard-4", 4, 16}, {"a2-highgpu-1g", 12, 85}, {"f1-micro", 1, 0.6}, {"g1-small", 1, 1.7},
	} {
		p, ok := x.Machine(region, m.mt, m.vcpu, m.mem, false)
		t.Logf("%-24s %10.2f %s/mes %v", m.mt, p, c.Currency, ok)
		if !ok {
			t.Errorf("%s not priced", m.mt)
		}
	}
	for _, g := range []string{"nvidia-tesla-t4", "nvidia-l4", "nvidia-tesla-a100"} {
		p, ok := x.GPU(region, g, 1, false)
		t.Logf("GPU %-20s %10.2f %v", g, p, ok)
		if !ok {
			t.Errorf("GPU %s not priced", g)
		}
	}
	for _, d := range []string{"pd-standard", "pd-balanced", "pd-ssd", "pd-extreme", "hyperdisk-balanced", "hyperdisk-extreme", "hyperdisk-throughput"} {
		p, ok := x.Disk(region, d, false, 100, 4000, 200)
		t.Logf("disk %-22s %8.2f %v", d, p, ok)
		if !ok {
			t.Errorf("disk %s not priced", d)
		}
	}
	for _, l := range []struct {
		name string
		vcpu int64
	}{{"windows-server-2022-dc", 4}, {"windows-server-2019-dc", 8}, {"sles-15", 4}, {"sles-sap-15", 8}, {"sles-sap-12", 2},
		{"ubuntu-pro-2204-lts", 4}, {"ubuntu-pro-fips-2004-lts", 2}, {"ubuntu-minimal-pro-2404-lts", 8},
		{"rhel-9", 4}, {"rhel-8-sap", 16}, {"sql-server-2019-standard", 8}, {"sql-server-2022-enterprise", 4}} {
		p, ok := x.License(l.name, l.vcpu, 16)
		t.Logf("lic %-28s %8.2f %v", l.name, p, ok)
		if !ok {
			t.Errorf("license %s not priced", l.name)
		}
	}
}
