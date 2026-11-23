package billing

import (
	"context"
	"encoding/json"
	"gcetools/internal/schema"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"gcetools/internal/model"
)

type fakeBQ struct {
	partitionField *string // nil = not partitioned
	detailed       bool
	dryBytes       int64
	queries        []map[string]any
}

func (f *fakeBQ) server(t *testing.T) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/finops/datasets/billing/tables/export"):
			fields := `[{"name":"invoice"},{"name":"credits"},{"name":"cost"}` + map[bool]string{true: `,{"name":"resource"}`}[f.detailed] + `]`
			tp := ""
			if f.partitionField != nil {
				tp = `,"timePartitioning":{"type":"DAY","field":"` + *f.partitionField + `"}`
			}
			w.Write([]byte(`{"schema":{"fields":` + fields + `}` + tp + `}`))
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/projects/finops/queries"):
			var q map[string]any
			json.NewDecoder(r.Body).Decode(&q)
			f.queries = append(f.queries, q)
			if q["dryRun"] == true {
				w.Write([]byte(`{"jobComplete":true,"totalBytesProcessed":"` + itoa(f.dryBytes) + `"}`))
				return
			}
			w.Write([]byte(`{"jobComplete":true,"totalBytesProcessed":"123000000","pageToken":"p2",
				"jobReference":{"projectId":"finops","jobId":"j1","location":"EU"},
				"rows":[
				 {"f":[{"v":"erp"},{"v":"//compute.googleapis.com/projects/erp/zones/z/instances/111"},{"v":"app-1"},{"v":"100.5"},{"v":"-30.5"},{"v":"EUR"}]},
				 {"f":[{"v":"erp"},{"v":"//compute.googleapis.com/projects/erp/zones/z/disks/app-1-data"},{"v":"app-1-data"},{"v":"20"},{"v":"0"},{"v":"EUR"}]}]}`))
		case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/projects/finops/queries/j1"):
			if r.URL.Query().Get("pageToken") != "p2" || r.URL.Query().Get("location") != "EU" {
				t.Errorf("page request: %s", r.URL.RawQuery)
			}
			w.Write([]byte(`{"jobComplete":true,"rows":[
				 {"f":[{"v":"erp"},{"v":""},{"v":""},{"v":"15"},{"v":"-5"},{"v":"EUR"}]},
				 {"f":[{"v":"web"},{"v":"//compute.googleapis.com/projects/web/zones/z/instances/999"},{"v":"gone"},{"v":"7"},{"v":"0"},{"v":"EUR"}]}]}`))
		default:
			t.Errorf("unexpected %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
}

func itoa(n int64) string { b, _ := json.Marshal(n); return string(b) }

func withServer(t *testing.T, f *fakeBQ) {
	srv := f.server(t)
	t.Cleanup(srv.Close)
	old := endpoint
	endpoint = srv.URL + "/"
	t.Cleanup(func() { endpoint = old })
}

func TestLoadAndApply(t *testing.T) {
	empty := ""
	f := &fakeBQ{partitionField: &empty, detailed: true, dryBytes: 2e9}
	withServer(t, f)

	now := time.Date(2026, 11, 19, 10, 0, 0, 0, time.UTC)
	d, err := Load(context.Background(), http.DefaultClient, Config{Table: "finops.billing.export"}, []string{"erp", "web"}, now)
	if err != nil {
		t.Fatal(err)
	}
	if d.Month != "202610" || d.Currency != "EUR" || !d.Detailed || len(d.Lines) != 4 || d.BytesRead != 123000000 {
		t.Fatalf("data = %+v", d)
	}
	if len(f.queries) != 2 || f.queries[0]["dryRun"] != true {
		t.Fatalf("expected a dry run first: %v", f.queries)
	}
	q := f.queries[1]["query"].(string)
	for _, want := range []string{"_PARTITIONTIME >= TIMESTAMP(@from)", "service.id = @service", "project.id IN UNNEST(@projects)", "resource.global_name"} {
		if !strings.Contains(q, want) {
			t.Errorf("query lacks %q:\n%s", want, q)
		}
	}
	if f.queries[1]["maximumBytesBilled"] != "50000000000" {
		t.Errorf("maximumBytesBilled = %v", f.queries[1]["maximumBytesBilled"])
	}

	inv := schema.NewInventory()
	inv.Meta.Currency = "EUR"
	inv.Sheet(schema.SInfo).Add(model.Row{"project": "erp", "name": "app-1", "id": "111"})
	inv.Sheet(schema.SDisks).Add(model.Row{"project": "erp", "name": "app-1-data"})
	inv.Sheet(schema.SCost).Add(model.Row{"project": "erp", "total": 120.0})
	res := Apply(inv, d)

	if v := inv.Sheet(schema.SInfo).Rows[0]["costReal"]; v != 70.0 {
		t.Errorf("VM costReal = %v, want 70 (100.5 - 30.5)", v)
	}
	if v := inv.Sheet(schema.SDisks).Rows[0]["costReal"]; v != 20.0 {
		t.Errorf("disk costReal = %v", v)
	}
	if res.Net != 107 || res.Attributed != 90 {
		t.Errorf("result = %+v", res)
	}
	rows := inv.Sheet(schema.SCostReal).Rows
	if len(rows) != 3 || rows[2]["project"] != "TOTAL" {
		t.Fatalf("gCostReal = %v", rows)
	}
	erp := rows[0]
	if erp["net"] != 100.0 || erp["attributed"] != 90.0 || erp["unattributed"] != 10.0 || erp["estimate"] != 120.0 ||
		erp["diff"] != -20.0 || erp["diffPct"] != -16.7 {
		t.Errorf("erp row = %v", erp)
	}
	if web := rows[1]; web["unattributed"] != 7.0 || web["estimate"] != nil {
		t.Errorf("web row (deleted VM, no estimate) = %v", web)
	}
}

func TestByteLimitAndColumnPartition(t *testing.T) {
	field := "usage_start_time"
	f := &fakeBQ{partitionField: &field, dryBytes: 80e9}
	withServer(t, f)
	_, err := Load(context.Background(), http.DefaultClient, Config{Table: "finops.billing.export", Month: "202609"}, nil, time.Now())
	if err == nil || !strings.Contains(err.Error(), "80.0 GB") {
		t.Fatalf("expected byte-limit error, got %v", err)
	}
	if len(f.queries) != 1 {
		t.Errorf("must stop after the dry run: %d queries", len(f.queries))
	}
	q := f.queries[0]["query"].(string)
	if !strings.Contains(q, "usage_start_time >= TIMESTAMP(@from)") || strings.Contains(q, "_PARTITIONTIME") ||
		strings.Contains(q, "resource.global_name") || strings.Contains(q, "UNNEST(@projects)") {
		t.Errorf("standard export / column partition query wrong:\n%s", q)
	}
}

func TestParseTableAndMonth(t *testing.T) {
	for in, ok := range map[string]bool{
		"finops-prj.billing.gcp_billing_export_resource_v1_0123AB_CDEF12_345678": true,
		"`finops-prj.billing.export`":                                            true,
		"finops-prj:billing.export":                                              true,
		"billing.export":                                                         false,
		"finops-prj.billing.export; --":                                          false,
	} {
		if _, _, _, err := ParseTable(in); (err == nil) != ok {
			t.Errorf("ParseTable(%q) err = %v", in, err)
		}
	}
	if m := PreviousMonth(time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC)); m != "202512" {
		t.Errorf("PreviousMonth = %s", m)
	}
	if _, err := Load(context.Background(), http.DefaultClient, Config{Table: "a-proj.d.t", Month: "2026-10"}, nil, time.Now()); err == nil {
		t.Error("bad month must fail before any request")
	}
}
