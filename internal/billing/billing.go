// Package billing reads the real, net Compute Engine cost of a month from a
// Cloud Billing export table in BigQuery and attaches it to the inventory,
// next to the list-price estimate of the pricing package.
//
// It is the only gceTools feature that can cost money: BigQuery bills the
// bytes a query reads. To keep that bounded, Load filters on the table's
// partitions, estimates the bytes with a dry run first and refuses to run
// above Config.MaxGB.
package billing

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gcetools/internal/i18n"

	bigquery "google.golang.org/api/bigquery/v2"
	"google.golang.org/api/option"
)

// ComputeService is the Cloud Billing service ID of Compute Engine.
const ComputeService = "6F81-5844-456A"

type Config struct {
	Table      string `json:"table"`      // project.dataset.table of the export
	Month      string `json:"month"`      // YYYYMM invoice month; "" = previous month
	JobProject string `json:"jobProject"` // where the query job runs; "" = the table's project
	MaxGB      int    `json:"maxGB"`      // refuse queries that would read more; 0 = 50
}

// Line is the cost of one resource (or of a project, with the standard export).
type Line struct {
	Project    string
	GlobalName string // //compute.googleapis.com/projects/p/zones/z/instances/123 (detailed export)
	Name       string // resource name (detailed export)
	Cost       float64
	Credits    float64 // negative: CUD, SUD, promotions, discounts
}

func (l Line) Net() float64 { return l.Cost + l.Credits }

type Data struct {
	Table     string
	Month     string
	Currency  string
	Detailed  bool // resource-level export: costs can be attributed to VMs and disks
	BytesRead int64
	Lines     []Line
}

var (
	reTable = regexp.MustCompile("^`?([a-z][-a-z0-9]{4,28}[a-z0-9])[.:]([A-Za-z0-9_]+)\\.([A-Za-z0-9_$-]+)`?$")
	reMonth = regexp.MustCompile(`^20\d\d(0[1-9]|1[0-2])$`)
)

// ParseTable splits "project.dataset.table" (also "project:dataset.table").
func ParseTable(s string) (project, dataset, table string, err error) {
	m := reTable.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", "", "", fmt.Errorf(i18n.T("tabla no válida %q: usa proyecto.dataset.tabla (p. ej. finops-prj.billing.gcp_billing_export_resource_v1_0123AB_CDEF12_345678)"), s)
	}
	return m[1], m[2], m[3], nil
}

// PreviousMonth is the last complete invoice month before now (YYYYMM).
func PreviousMonth(now time.Time) string {
	return now.AddDate(0, 0, -now.Day()).Format("200601")
}

// endpoint overrides the BigQuery API base URL (tests).
var endpoint string

// Load runs the cost query for projects (empty = every project in the export).
func Load(ctx context.Context, hc *http.Client, cfg Config, projects []string, now time.Time) (*Data, error) {
	tp, ds, tb, err := ParseTable(cfg.Table)
	if err != nil {
		return nil, err
	}
	month := cfg.Month
	if month == "" {
		month = PreviousMonth(now)
	}
	if !reMonth.MatchString(month) {
		return nil, fmt.Errorf(i18n.T("mes no válido %q: usa AAAAMM"), month)
	}
	maxGB := cfg.MaxGB
	if maxGB <= 0 {
		maxGB = 50
	}
	jobProject := cfg.JobProject
	if jobProject == "" {
		jobProject = tp
	}
	opts := []option.ClientOption{option.WithHTTPClient(hc)}
	if endpoint != "" {
		opts = append(opts, option.WithEndpoint(endpoint))
	}
	svc, err := bigquery.NewService(ctx, opts...)
	if err != nil {
		return nil, err
	}

	meta, err := svc.Tables.Get(tp, ds, tb).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("no se pudo leer la tabla %s: %w"), cfg.Table, err)
	}
	detailed := hasField(meta.Schema, "resource")
	if !hasField(meta.Schema, "invoice") || !hasField(meta.Schema, "credits") {
		return nil, fmt.Errorf(i18n.T("%s no parece una exportación de facturación de Cloud Billing"), cfg.Table)
	}

	from, _ := time.Parse("200601", month)
	query, params := buildQuery(fmt.Sprintf("`%s.%s.%s`", tp, ds, tb), meta.TimePartitioning, detailed, month, from, projects)
	req := &bigquery.QueryRequest{
		Query: query, UseLegacySql: new(bool), ParameterMode: "NAMED", QueryParameters: params,
		Labels: map[string]string{"tool": "gcetools"}, TimeoutMs: 60000,
	}

	// Dry run: free, tells how many bytes the real query would bill.
	dry := *req
	dry.DryRun = true
	est, err := svc.Jobs.Query(jobProject, &dry).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf(i18n.T("BigQuery (estimación): %w"), err)
	}
	limit := int64(maxGB) * 1e9
	if est.TotalBytesProcessed > limit {
		return nil, fmt.Errorf(i18n.T("la consulta leería %.1f GB, más que el límite de %d GB: súbelo o elige otra tabla"), float64(est.TotalBytesProcessed)/1e9, maxGB)
	}
	req.MaximumBytesBilled = limit

	resp, err := svc.Jobs.Query(jobProject, req).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("BigQuery: %w", err)
	}
	data := &Data{Table: cfg.Table, Month: month, Detailed: detailed, BytesRead: resp.TotalBytesProcessed}
	rows, pageToken, complete := resp.Rows, resp.PageToken, resp.JobComplete
	ref := resp.JobReference
	for {
		for _, r := range rows {
			l, cur, err := parseRow(r, detailed)
			if err != nil {
				return nil, err
			}
			if cur != "" {
				data.Currency = cur
			}
			data.Lines = append(data.Lines, l)
		}
		if complete && pageToken == "" {
			break
		}
		if ref == nil {
			return nil, errors.New(i18n.T("BigQuery no devolvió la referencia del job"))
		}
		call := svc.Jobs.GetQueryResults(ref.ProjectId, ref.JobId).Location(ref.Location).TimeoutMs(30000).Context(ctx)
		if complete {
			call = call.PageToken(pageToken)
		}
		page, err := call.Do()
		if err != nil {
			return nil, fmt.Errorf("BigQuery: %w", err)
		}
		if !complete && page.JobComplete && data.BytesRead == 0 {
			data.BytesRead = page.TotalBytesProcessed
		}
		if !page.JobComplete {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			rows, complete = nil, false
			continue
		}
		rows, pageToken, complete = page.Rows, page.PageToken, true
	}
	return data, nil
}

func buildQuery(table string, tp *bigquery.TimePartitioning, detailed bool, month string, from time.Time, projects []string) (string, []*bigquery.QueryParameter) {
	cols := "project.id AS project, '' AS global_name, '' AS name"
	if detailed {
		cols = "project.id AS project, IFNULL(resource.global_name, '') AS global_name, IFNULL(resource.name, '') AS name"
	}
	where := []string{"invoice.month = @month", "service.id = @service"}
	// Only read the partitions that can hold the month. With ingestion-time
	// partitions late corrections land after the month ends: no upper bound.
	if tp != nil {
		if tp.Field == "" {
			where = append(where, "_PARTITIONTIME >= TIMESTAMP(@from)")
		} else {
			where = append(where, fmt.Sprintf("%s >= TIMESTAMP(@from) AND %s < TIMESTAMP_ADD(TIMESTAMP(@from), INTERVAL 62 DAY)", tp.Field, tp.Field))
		}
	}
	params := []*bigquery.QueryParameter{
		strParam("month", month), strParam("service", ComputeService), strParam("from", from.Format("2006-01-02")),
	}
	if len(projects) > 0 {
		where = append(where, "project.id IN UNNEST(@projects)")
		vals := make([]*bigquery.QueryParameterValue, len(projects))
		for i, p := range projects {
			vals[i] = &bigquery.QueryParameterValue{Value: p}
		}
		params = append(params, &bigquery.QueryParameter{Name: "projects",
			ParameterType:  &bigquery.QueryParameterType{Type: "ARRAY", ArrayType: &bigquery.QueryParameterType{Type: "STRING"}},
			ParameterValue: &bigquery.QueryParameterValue{ArrayValues: vals}})
	}
	q := fmt.Sprintf(`SELECT %s,
  SUM(cost) AS cost,
  SUM(IFNULL((SELECT SUM(c.amount) FROM UNNEST(credits) c), 0)) AS credits,
  ANY_VALUE(currency) AS currency
FROM %s
WHERE %s
GROUP BY 1, 2, 3`, cols, table, strings.Join(where, "\n  AND "))
	return q, params
}

func strParam(name, v string) *bigquery.QueryParameter {
	return &bigquery.QueryParameter{Name: name, ParameterType: &bigquery.QueryParameterType{Type: "STRING"},
		ParameterValue: &bigquery.QueryParameterValue{Value: v}}
}

func parseRow(r *bigquery.TableRow, detailed bool) (Line, string, error) {
	if len(r.F) < 6 {
		return Line{}, "", fmt.Errorf(i18n.T("fila de BigQuery inesperada (%d columnas)"), len(r.F))
	}
	s := func(i int) string { v, _ := r.F[i].V.(string); return v }
	f := func(i int) float64 { v, _ := strconv.ParseFloat(s(i), 64); return v }
	return Line{Project: s(0), GlobalName: s(1), Name: s(2), Cost: f(3), Credits: f(4)}, s(5), nil
}

func hasField(schema *bigquery.TableSchema, name string) bool {
	if schema == nil {
		return false
	}
	for _, f := range schema.Fields {
		if f.Name == name {
			return true
		}
	}
	return false
}
