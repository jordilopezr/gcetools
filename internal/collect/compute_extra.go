package collect

import (
	"fmt"
	"strings"
	"time"

	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
)

// ---------- images & machine images (gImage) ----------

func (c *collector) ageDays(created string) any {
	if t, ok := ts(created).(time.Time); ok {
		return int64(c.now.Sub(t).Hours() / 24)
	}
	return nil
}

func (c *collector) collectImages() error {
	err := c.svc.Images.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.ImageList) error {
		for _, im := range page.Items {
			dep := ""
			if im.Deprecated != nil {
				dep = im.Deprecated.State
			}
			src := last(im.SourceDisk)
			if src == "" {
				src = last(im.SourceImage)
			}
			if src == "" {
				src = last(im.SourceSnapshot)
			}
			c.add(SImage, model.Row{
				"project": c.project, "name": im.Name, "kind": "Image", "family": im.Family, "status": im.Status,
				"deprecated": dep, "diskSizeGB": float64(im.DiskSizeGb),
				"storageGB": round2(float64(im.ArchiveSizeBytes) / (1 << 30)), "created": ts(im.CreationTimestamp),
				"ageDays": c.ageDays(im.CreationTimestamp), "source": src, "licenses": joinLast(im.Licenses),
				"locations": strings.Join(im.StorageLocations, "; "), "architecture": im.Architecture, "labels": labels(im.Labels),
			})
		}
		return nil
	})
	if err != nil {
		return err
	}
	return c.svc.MachineImages.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.MachineImageList) error {
		for _, mi := range page.Items {
			c.add(SImage, model.Row{
				"project": c.project, "name": mi.Name, "kind": "Machine image", "status": mi.Status,
				"storageGB": round2(float64(mi.TotalStorageBytes) / (1 << 30)), "created": ts(mi.CreationTimestamp),
				"ageDays": c.ageDays(mi.CreationTimestamp), "source": last(mi.SourceInstance),
				"locations": strings.Join(mi.StorageLocations, "; "), "labels": labels(mi.Labels),
			})
		}
		return nil
	})
}

// ---------- reservations & commitments ----------

func (c *collector) collectReservations() error {
	return c.svc.Reservations.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.ReservationAggregatedList) error {
			for _, scoped := range page.Items {
				for _, r := range scoped.Reservations {
					row := model.Row{
						"project": c.project, "name": r.Name, "zone": last(r.Zone),
						"specificOnly": r.SpecificReservationRequired, "status": r.Status,
						"commitment": last(r.Commitment), "created": ts(r.CreationTimestamp),
					}
					if r.ShareSettings != nil {
						row["shareType"] = r.ShareSettings.ShareType
					}
					if sr := r.SpecificReservation; sr != nil {
						row["count"], row["inUse"] = sr.Count, sr.InUseCount
						if sr.Count > 0 {
							row["unusedPct"] = round2(float64(sr.Count-sr.InUseCount) * 100 / float64(sr.Count))
						}
						if p := sr.InstanceProperties; p != nil {
							row["machineType"] = p.MachineType
							gpus := []string{}
							for _, g := range p.GuestAccelerators {
								gpus = append(gpus, fmt.Sprintf("%s x%d", last(g.AcceleratorType), g.AcceleratorCount))
							}
							row["gpus"] = strings.Join(gpus, "; ")
							var ssd int64
							for _, l := range p.LocalSsds {
								ssd += l.DiskSizeGb
							}
							row["localSsdGB"] = float64(ssd)
						}
					}
					c.add(SReserv, row)
				}
			}
			return nil
		})
}

func (c *collector) collectCommitments() error {
	return c.svc.RegionCommitments.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.CommitmentAggregatedList) error {
			for _, scoped := range page.Items {
				for _, cm := range scoped.Commitments {
					row := model.Row{
						"project": c.project, "name": cm.Name, "region": last(cm.Region), "plan": cm.Plan,
						"type": cm.Type, "category": cm.Category, "status": cm.Status,
						"start": ts(cm.StartTimestamp), "end": ts(cm.EndTimestamp), "autoRenew": cm.AutoRenew,
					}
					if t, ok := ts(cm.EndTimestamp).(time.Time); ok {
						row["daysLeft"] = int64(t.Sub(c.now).Hours() / 24)
					}
					other := []string{}
					for _, r := range cm.Resources {
						switch r.Type {
						case "VCPU":
							row["vcpu"] = r.Amount
						case "MEMORY":
							row["memoryGB"] = round2(float64(r.Amount) / 1024)
						default:
							name := r.Type
							if r.AcceleratorType != "" {
								name = last(r.AcceleratorType)
							}
							other = append(other, fmt.Sprintf("%s x%d", name, r.Amount))
						}
					}
					row["other"] = strings.Join(other, "; ")
					c.add(SCommit, row)
				}
			}
			return nil
		})
}

// ---------- sole-tenant nodes (gSoleTenant) ----------

func (c *collector) collectSoleTenant() error {
	type group struct {
		zone, name string
		g          *compute.NodeGroup
	}
	var groups []group
	err := c.svc.NodeGroups.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.NodeGroupAggregatedList) error {
			for _, scoped := range page.Items {
				for _, g := range scoped.NodeGroups {
					groups = append(groups, group{last(g.Zone), g.Name, g})
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	for _, gr := range groups {
		autoscale := ""
		if gr.g.AutoscalingPolicy != nil {
			autoscale = gr.g.AutoscalingPolicy.Mode
		}
		err := c.svc.NodeGroups.ListNodes(c.project, gr.zone, gr.name).Context(c.ctx).
			Pages(c.ctx, func(page *compute.NodeGroupsListNodes) error {
				for _, n := range page.Items {
					c.add(SSole, model.Row{
						"project": c.project, "nodeGroup": gr.name, "zone": gr.zone,
						"nodeTemplate": last(gr.g.NodeTemplate), "node": n.Name, "nodeType": n.NodeType,
						"status": n.Status, "vms": int64(len(n.Instances)), "instances": joinLast(n.Instances),
						"cpuOvercommit": n.CpuOvercommitType, "maintenance": gr.g.MaintenancePolicy,
						"autoscaling": autoscale,
					})
				}
				return nil
			})
		if err != nil {
			return err
		}
	}
	return nil
}

// ---------- regional quotas (gQuota) ----------

func (c *collector) collectQuotas() error {
	return c.svc.Regions.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.RegionList) error {
		for _, r := range page.Items {
			for _, q := range r.Quotas {
				if q.Usage <= 0 {
					continue
				}
				var pct any
				if q.Limit > 0 {
					pct = round2(q.Usage * 100 / q.Limit)
				}
				c.add(SQuota, model.Row{
					"project": c.project, "region": r.Name, "metric": q.Metric,
					"usage": q.Usage, "limit": q.Limit, "usagePct": pct,
				})
			}
		}
		return nil
	})
}
