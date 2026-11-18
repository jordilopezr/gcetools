package collect

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"gcetools/internal/model"

	monitoring "google.golang.org/api/monitoring/v3"
	osconfig "google.golang.org/api/osconfig/v1"
	recommender "google.golang.org/api/recommender/v1"
)

// fatal reports whether an error means the whole module is unusable for this
// project (API off or no permission), as opposed to one bad location.
func fatal(err error) bool {
	k, _ := Classify(err)
	return k == model.ErrAPIDisabled || k == model.ErrPermission
}

func sortedKeys(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ---------- OS Config inventory (gTools) ----------

func (c *collector) collectOSInventory() error {
	vmZones := map[string]bool{}
	for _, v := range c.vms {
		vmZones[v.zone] = true
	}
	var lastErr error
	ok := 0
	for _, zone := range sortedKeys(vmZones) {
		parent := fmt.Sprintf("projects/%s/locations/%s/instances/-", c.project, zone)
		err := c.svcs.OSConfig.Projects.Locations.Instances.Inventories.List(parent).View("BASIC").Context(c.ctx).
			Pages(c.ctx, func(page *osconfig.ListInventoriesResponse) error {
				for _, inv := range page.Inventories {
					// name: projects/{num}/locations/{zone}/instances/{id}/inventory
					parts := strings.Split(inv.Name, "/")
					id := ""
					if len(parts) >= 6 {
						id = parts[5]
					}
					vm := c.vms[id]
					row := model.Row{"project": c.project, "vm": vm.name, "zone": zone, "updated": ts(inv.UpdateTime)}
					if vm.name == "" {
						row["vm"] = id
					}
					if o := inv.OsInfo; o != nil {
						row["os"], row["shortName"], row["version"] = o.LongName, o.ShortName, o.Version
						row["kernel"], row["architecture"] = o.KernelRelease, o.Architecture
						row["hostname"], row["agentVersion"] = o.Hostname, o.OsconfigAgentVersion
					}
					c.add(STools, row)
				}
				return nil
			})
		if err != nil {
			if fatal(err) {
				return err
			}
			lastErr = err
			continue
		}
		ok++
	}
	if ok == 0 {
		return lastErr
	}
	return nil
}

// ---------- Cloud Monitoring (gPerf) ----------

func (c *collector) collectMetrics() error {
	if len(c.vms) == 0 {
		return nil
	}
	days := c.svcs.MetricsDays
	if days <= 0 {
		days = 30
	}
	cpu, err := c.series(`metric.type="compute.googleapis.com/instance/cpu/utilization" AND resource.type="gce_instance"`, days, 100)
	if err != nil {
		return err
	}
	// Memory needs the Ops Agent; absence is normal, not an error.
	mem, err := c.series(`metric.type="agent.googleapis.com/memory/percent_used" AND metric.labels.state="used" AND resource.type="gce_instance"`, days, 1)
	if err != nil && fatal(err) {
		return err
	}

	ids := make([]string, 0, len(c.vms))
	for id := range c.vms {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return c.vms[ids[i]].name < c.vms[ids[j]].name })
	for _, id := range ids {
		v := c.vms[id]
		row := model.Row{
			"project": c.project, "vm": v.name, "zone": v.zone, "status": v.status, "machineType": v.machineType,
			"vcpu": v.vcpu, "memoryGB": v.memoryGB, "days": int64(days), "samples": int64(len(cpu[id])),
			"opsAgent": len(mem[id]) > 0,
		}
		if p := cpu[id]; len(p) > 0 {
			row["cpuAvg"], row["cpuP95"], row["cpuMax"] = stats(p)
		}
		if p := mem[id]; len(p) > 0 {
			row["memAvg"], row["memP95"], row["memMax"] = stats(p)
		}
		c.add(SPerf, row)
	}
	return nil
}

// series returns hourly means per instance id over the last days, multiplied by scale.
func (c *collector) series(filter string, days int, scale float64) (map[string][]float64, error) {
	out := map[string][]float64{}
	end := c.now.UTC()
	start := end.Add(-time.Duration(days) * 24 * time.Hour)
	err := c.svcs.Monitoring.Projects.TimeSeries.List("projects/"+c.project).Context(c.ctx).
		Filter(filter).
		IntervalStartTime(start.Format(time.RFC3339)).IntervalEndTime(end.Format(time.RFC3339)).
		AggregationAlignmentPeriod("3600s").AggregationPerSeriesAligner("ALIGN_MEAN").
		Pages(c.ctx, func(page *monitoring.ListTimeSeriesResponse) error {
			for _, s := range page.TimeSeries {
				if s.Resource == nil {
					continue
				}
				id := s.Resource.Labels["instance_id"]
				for _, p := range s.Points {
					if p.Value != nil && p.Value.DoubleValue != nil {
						out[id] = append(out[id], *p.Value.DoubleValue*scale)
					}
				}
			}
			return nil
		})
	return out, err
}

func stats(p []float64) (avg, p95, maxV float64) {
	s := slices.Clone(p)
	slices.Sort(s)
	var sum float64
	for _, v := range s {
		sum += v
	}
	idx := int(math.Ceil(0.95*float64(len(s)))) - 1
	return round1(sum / float64(len(s))), round1(s[max(idx, 0)]), round1(s[len(s)-1])
}

func round1(f float64) float64 { return math.Round(f*10) / 10 }

// ---------- Recommender (gRecommend) ----------

var (
	zonalRecommenders = []string{
		"google.compute.instance.MachineTypeRecommender",
		"google.compute.instance.IdleResourceRecommender",
		"google.compute.disk.IdleResourceRecommender",
	}
	regionalRecommenders = []string{
		"google.compute.address.IdleResourceRecommender",
		"google.compute.commitment.UsageCommitmentRecommender",
	}
	globalRecommenders = []string{"google.compute.image.IdleResourceRecommender"}
)

func (c *collector) collectRecommendations() error {
	type target struct{ loc, rec string }
	var targets []target
	for _, z := range sortedKeys(c.zones) {
		for _, r := range zonalRecommenders {
			targets = append(targets, target{z, r})
		}
	}
	for _, rg := range sortedKeys(c.regions) {
		for _, r := range regionalRecommenders {
			targets = append(targets, target{rg, r})
		}
	}
	for _, r := range globalRecommenders {
		targets = append(targets, target{"global", r})
	}

	var lastErr error
	ok := 0
	for _, t := range targets {
		parent := fmt.Sprintf("projects/%s/locations/%s/recommenders/%s", c.project, t.loc, t.rec)
		err := c.svcs.Recommender.Projects.Locations.Recommenders.Recommendations.List(parent).
			Filter("stateInfo.state=ACTIVE").Context(c.ctx).
			Pages(c.ctx, func(page *recommender.GoogleCloudRecommenderV1ListRecommendationsResponse) error {
				for _, r := range page.Recommendations {
					c.add(SRecomm, c.recommendationRow(r, t.loc, t.rec))
				}
				return nil
			})
		if err != nil {
			if fatal(err) {
				return err
			}
			lastErr = err
			continue
		}
		ok++
	}
	if ok == 0 {
		return lastErr
	}
	return nil
}

func (c *collector) recommendationRow(r *recommender.GoogleCloudRecommenderV1Recommendation, loc, rec string) model.Row {
	short := strings.TrimSuffix(strings.TrimPrefix(rec, "google.compute."), "Recommender")
	row := model.Row{
		"project": c.project, "recommender": short, "subtype": r.RecommenderSubtype, "location": loc,
		"description": r.Description, "priority": r.Priority, "lastRefresh": ts(r.LastRefreshTime),
	}
	if r.StateInfo != nil {
		row["state"] = r.StateInfo.State
	}
	if pi := r.PrimaryImpact; pi != nil {
		row["category"] = pi.Category
		if cp := pi.CostProjection; cp != nil && cp.Cost != nil {
			amount := float64(cp.Cost.Units) + float64(cp.Cost.Nanos)/1e9
			durSec := 30 * 24 * 3600.0
			if d, err := strconv.ParseFloat(strings.TrimSuffix(cp.Duration, "s"), 64); err == nil && d > 0 {
				durSec = d
			}
			// Cost projections are negative when the recommendation saves money.
			row["savingsMonthly"] = round2(-amount * (30 * 24 * 3600) / durSec)
			row["currency"] = cp.Cost.CurrencyCode
		}
	}
	if ct := r.Content; ct != nil {
		for _, g := range ct.OperationGroups {
			for _, op := range g.Operations {
				if op.Resource != "" {
					row["resource"] = last(op.Resource)
					break
				}
			}
			if row["resource"] != nil {
				break
			}
		}
	}
	return row
}
