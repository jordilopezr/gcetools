package collect

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"fmt"

	compute "google.golang.org/api/compute/v1"
	monitoring "google.golang.org/api/monitoring/v3"
	"google.golang.org/api/option"
	osconfig "google.golang.org/api/osconfig/v1"
	recommender "google.golang.org/api/recommender/v1"
)

// hourly CPU points: 200 at 2% and 10 at 50% -> avg ~4.3, p95 2.0, max 50
func cpuSeries() string {
	pts := ""
	for i := 0; i < 210; i++ {
		v := 0.02
		if i >= 200 {
			v = 0.5
		}
		if i > 0 {
			pts += ","
		}
		pts += fmt.Sprintf(`{"value":{"doubleValue":%g}}`, v)
	}
	return `{"timeSeries":[{"resource":{"type":"gce_instance","labels":{"instance_id":"42"}},"points":[` + pts + `]}]}`
}

// fakeCompute serves canned Compute API responses keyed by URL path suffix.
func fakeServices(t *testing.T) Services {
	t.Helper()
	routes := map[string]string{
		"/projects/p1/aggregated/disks": `{"items":{"zones/europe-west1-b":{"disks":[
			{"name":"vm-1","selfLink":"https://x/projects/p1/zones/europe-west1-b/disks/vm-1","zone":"https://x/zones/europe-west1-b",
			 "type":"https://x/diskTypes/pd-balanced","sizeGb":"50","users":["https://x/instances/vm-1"],"sourceImage":"https://x/images/debian-12",
			 "status":"READY"},
			{"name":"old","selfLink":"https://x/disks/old","zone":"https://x/zones/europe-west1-b","type":"https://x/diskTypes/pd-ssd","sizeGb":"500","status":"READY"}]}}}`,
		"/projects/p1/aggregated/instances": `{"items":{"zones/europe-west1-b":{"instances":[
			{"name":"vm-1","id":"42","zone":"https://x/zones/europe-west1-b","machineType":"https://x/machineTypes/e2-standard-4",
			 "status":"RUNNING","creationTimestamp":"2024-01-02T03:04:05.000-07:00",
			 "networkInterfaces":[{"name":"nic0","network":"https://x/projects/host/global/networks/shared","subnetwork":"https://x/subnetworks/sn",
			   "networkIP":"10.0.0.2","accessConfigs":[{"natIP":"34.1.1.1","networkTier":"PREMIUM"}]}],
			 "disks":[{"source":"https://x/projects/p1/zones/europe-west1-b/disks/vm-1","boot":true,"deviceName":"boot",
			   "licenses":["https://x/projects/vm-migration/global/licenses/vmmigration-license","https://x/projects/debian-cloud/global/licenses/debian-12-bookworm"]}],
			 "serviceAccounts":[{"email":"1-compute@developer.gserviceaccount.com","scopes":["https://www.googleapis.com/auth/cloud-platform"]}],
			 "labels":{"env":"prod"},"metadata":{"items":[{"key":"created-by","value":"projects/1/zones/z/instanceGroupManagers/web-mig"}]}}]}}}`,
		"/projects/p1/zones/europe-west1-b/machineTypes/e2-standard-4": `{"name":"e2-standard-4","guestCpus":4,"memoryMb":16384}`,
		"/v1/projects/p1/locations/europe-west1-b/instances/-/inventories": `{"inventories":[
			{"name":"projects/123/locations/europe-west1-b/instances/42/inventory","updateTime":"2026-11-17T10:00:00Z",
			 "osInfo":{"longName":"Debian GNU/Linux 12 (bookworm)","shortName":"debian","version":"12","kernelRelease":"6.1.0-26-cloud-amd64",
			 "architecture":"x86_64","hostname":"vm-1","osconfigAgentVersion":"20240923.00"}}]}`,
		"/recommenders/google.compute.instance.MachineTypeRecommender/recommendations": `{"recommendations":[
			{"name":"r1","description":"Save cost by changing machine type from e2-standard-4 to e2-standard-2.",
			 "recommenderSubtype":"CHANGE_MACHINE_TYPE","priority":"P2","stateInfo":{"state":"ACTIVE"},
			 "primaryImpact":{"category":"COST","costProjection":{"cost":{"currencyCode":"USD","units":"-48","nanos":-500000000},"duration":"2592000s"}},
			 "content":{"operationGroups":[{"operations":[{"action":"replace","resource":"//compute.googleapis.com/projects/p1/zones/europe-west1-b/instances/vm-1"}]}]}}]}`,
		"/projects/p1/global/firewalls": `{"items":[{"name":"ssh","network":"https://x/networks/default","direction":"INGRESS","priority":1000,
			"allowed":[{"IPProtocol":"tcp","ports":["22"]}],"sourceRanges":["0.0.0.0/0"]}]}`,
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.Contains(r.URL.Path, "/projects/off/") {
			w.WriteHeader(403)
			w.Write([]byte(`{"error":{"code":403,"message":"Compute Engine API has not been used in project off before or it is disabled.",
				"errors":[{"reason":"accessNotConfigured","message":"disabled"}]}}`))
			return
		}
		if strings.HasSuffix(r.URL.Path, "/v3/projects/p1/timeSeries") {
			if strings.Contains(r.URL.Query().Get("filter"), "cpu/utilization") {
				w.Write([]byte(cpuSeries()))
			} else {
				w.Write([]byte(`{}`)) // no Ops Agent memory series
			}
			return
		}
		for suffix, body := range routes {
			if strings.HasSuffix(r.URL.Path, suffix) {
				w.Write([]byte(body))
				return
			}
		}
		w.Write([]byte(`{}`))
	}))
	t.Cleanup(srv.Close)
	ctx, hc := context.Background(), option.WithHTTPClient(srv.Client())
	var s Services
	var err error
	if s.Compute, err = compute.NewService(ctx, option.WithEndpoint(srv.URL+"/compute/v1/"), hc); err != nil {
		t.Fatal(err)
	}
	if s.OSConfig, err = osconfig.NewService(ctx, option.WithEndpoint(srv.URL+"/"), hc); err != nil {
		t.Fatal(err)
	}
	if s.Monitoring, err = monitoring.NewService(ctx, option.WithEndpoint(srv.URL+"/"), hc); err != nil {
		t.Fatal(err)
	}
	if s.Recommender, err = recommender.NewService(ctx, option.WithEndpoint(srv.URL+"/"), hc); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProjectCollect(t *testing.T) {
	inv := NewInventory()
	st := Project(context.Background(), fakeServices(t), "p1", inv, func(string) {})
	if st.Status != "OK" || st.VMs != 1 || st.VCPU != 4 || st.Disks != 2 {
		t.Fatalf("stats = %+v, errors = %v", st, inv.Errors)
	}
	vm := inv.Sheet(SInfo).Rows[0]
	checks := map[string]any{
		"vcpu": int64(4), "memoryGB": 16.0, "region": "europe-west1", "externalIP": "34.1.1.1",
		"network": "shared", "os": "debian-12-bookworm", "image": "debian-12", "mig": "web-mig", "labels": "env=prod",
		"bootDiskGB": 50.0,
	}
	for k, want := range checks {
		if vm[k] != want {
			t.Errorf("gInfo[%s] = %#v, want %#v", k, vm[k], want)
		}
	}
	if np := inv.Sheet(SNetwork).Rows[0]["networkProject"]; np != "host" {
		t.Errorf("networkProject = %v, want host (shared VPC)", np)
	}
	var orphan bool
	for _, d := range inv.Sheet(SDisks).Rows {
		if d["name"] == "old" {
			orphan = d["orphan"].(bool)
		}
	}
	if !orphan {
		t.Error("disk 'old' should be orphan")
	}
	if fw := inv.Sheet(SFirewall).Rows[0]; fw["openToInternet"] != true || fw["rules"] != "tcp:22" {
		t.Errorf("firewall row = %v", fw)
	}
	lics := inv.Sheet(SLicense).Rows
	if len(lics) != 2 || lics[0]["category"] != "Herramienta" || lics[1]["category"] != "Debian" || lics[1]["licenseProject"] != "debian-cloud" {
		t.Errorf("license rows = %v", lics)
	}
}

func TestProjectAPIDisabled(t *testing.T) {
	inv := NewInventory()
	st := Project(context.Background(), fakeServices(t), "off", inv, func(string) {})
	if st.Status != "Compute API deshabilitada" {
		t.Fatalf("status = %q", st.Status)
	}
	if len(inv.Errors) != 1 || inv.Errors[0].Kind != "api_disabled" {
		t.Fatalf("errors = %+v (should stop after first collector)", inv.Errors)
	}
}

func TestOptionalModules(t *testing.T) {
	inv := NewInventory()
	Project(context.Background(), fakeServices(t), "p1", inv, func(string) {})

	tools := inv.Sheet(STools).Rows
	if len(tools) != 1 || tools[0]["vm"] != "vm-1" || tools[0]["os"] != "Debian GNU/Linux 12 (bookworm)" {
		t.Errorf("gTools = %v", tools)
	}
	perf := inv.Sheet(SPerf).Rows
	if len(perf) != 1 {
		t.Fatalf("gPerf = %v", perf)
	}
	if p := perf[0]; p["cpuP95"] != 2.0 || p["cpuMax"] != 50.0 || p["samples"] != int64(210) || p["opsAgent"] != false {
		t.Errorf("gPerf row = %v", p)
	}
	rec := inv.Sheet(SRecomm).Rows
	if len(rec) != 1 || rec[0]["savingsMonthly"] != 48.5 || rec[0]["resource"] != "vm-1" || rec[0]["recommender"] != "instance.MachineType" {
		t.Errorf("gRecommend = %v", rec)
	}
}

func TestOptionalModuleDisabledKeepsProjectOK(t *testing.T) {
	s := fakeServices(t)
	// Point OS Config at a project-less path so it gets a 403 SERVICE_DISABLED-like error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(403)
		w.Write([]byte(`{"error":{"code":403,"message":"OS Config API has not been used in project p1 before or it is disabled.","errors":[{"reason":"accessNotConfigured"}]}}`))
	}))
	t.Cleanup(srv.Close)
	var err error
	s.OSConfig, err = osconfig.NewService(context.Background(), option.WithEndpoint(srv.URL+"/"), option.WithHTTPClient(srv.Client()))
	if err != nil {
		t.Fatal(err)
	}
	inv := NewInventory()
	st := Project(context.Background(), s, "p1", inv, func(string) {})
	if st.Status != "OK (sin osconfig)" {
		t.Fatalf("status = %q, errors = %+v", st.Status, inv.Errors)
	}
}
