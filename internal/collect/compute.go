package collect

import (
	"fmt"
	"strings"
	"time"

	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
)

// ---------- disks (gDisks) ----------

func (c *collector) collectDisks() error {
	return c.svc.Disks.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.DiskAggregatedList) error {
			for _, scoped := range page.Items {
				for _, d := range scoped.Disks {
					c.disks[d.SelfLink] = d
					c.addDisk(d)
				}
			}
			return nil
		})
}

func (c *collector) addDisk(d *compute.Disk) {
	loc := last(d.Zone)
	if loc == "" {
		loc = last(d.Region)
	}
	if d.Zone != "" {
		c.zones[last(d.Zone)] = true
		c.regions[regionOfZone(last(d.Zone))] = true
	} else if d.Region != "" {
		c.regions[last(d.Region)] = true
	}
	c.stats.Disks++
	c.stats.DiskGB += float64(d.SizeGb)
	c.add(SDisks, model.Row{
		"project": c.project, "name": d.Name, "location": loc, "type": last(d.Type),
		"sizeGB": float64(d.SizeGb), "status": d.Status, "users": joinLast(d.Users),
		"orphan": len(d.Users) == 0, "created": ts(d.CreationTimestamp),
		"lastAttach": ts(d.LastAttachTimestamp), "lastDetach": ts(d.LastDetachTimestamp),
		"iops": d.ProvisionedIops, "throughput": d.ProvisionedThroughput,
		"encryption": encryption(d.DiskEncryptionKey), "sourceImage": last(d.SourceImage),
		"sourceSnapshot": last(d.SourceSnapshot), "snapshotSchedule": joinLast(d.ResourcePolicies),
		"replicaZones": joinLast(d.ReplicaZones), "architecture": d.Architecture, "labels": labels(d.Labels),
	})
}

// ---------- instances (gInfo, gCPU, gMemory, gDisk, gNetwork, gLicense) ----------

func (c *collector) collectInstances() error {
	return c.svc.Instances.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.InstanceAggregatedList) error {
			for _, scoped := range page.Items {
				for _, in := range scoped.Instances {
					c.addInstance(in)
				}
			}
			return nil
		})
}

// machineType resolves vCPU/RAM for a machine type, caching per zone.
// Custom types are resolved too. A failed lookup is cached as nil.
func (c *collector) machineType(zone, name string) *compute.MachineType {
	key := zone + "/" + name
	if mt, ok := c.mtypes[key]; ok {
		return mt
	}
	mt, err := c.svc.MachineTypes.Get(c.project, zone, name).Context(c.ctx).Do()
	if err != nil {
		mt = nil
	}
	c.mtypes[key] = mt
	return mt
}

func (c *collector) addInstance(in *compute.Instance) {
	zone := last(in.Zone)
	mtName := last(in.MachineType)
	mt := c.machineType(zone, mtName)

	var vcpu int64
	var memGB float64
	sharedCore := false
	if mt != nil {
		vcpu = mt.GuestCpus
		memGB = round2(float64(mt.MemoryMb) / 1024)
		sharedCore = mt.IsSharedCpu
	}
	family := strings.SplitN(mtName, "-", 2)[0]
	custom := strings.Contains(mtName, "custom")

	c.zones[zone] = true
	c.regions[regionOfZone(zone)] = true
	c.vms[fmt.Sprint(in.Id)] = vmRef{name: in.Name, zone: zone, machineType: mtName, status: in.Status, vcpu: vcpu, memoryGB: memGB}
	c.stats.VMs++
	if in.Status == "RUNNING" {
		c.stats.VMsRunning++
	}
	c.stats.VCPU += vcpu
	c.stats.MemoryGB += memGB

	// Scheduling
	provisioning, onHost, autoRestart := "STANDARD", "", true
	if s := in.Scheduling; s != nil {
		if s.ProvisioningModel != "" {
			provisioning = s.ProvisioningModel
		} else if s.Preemptible {
			provisioning = "PREEMPTIBLE"
		}
		onHost = s.OnHostMaintenance
		autoRestart = boolPtr(s.AutomaticRestart, true)
	}

	// Network interfaces
	var internalIP, externalIP, network, subnet string
	for i, nic := range in.NetworkInterfaces {
		ext, tier := "", ""
		for _, ac := range nic.AccessConfigs {
			if ac.NatIP != "" {
				ext, tier = ac.NatIP, ac.NetworkTier
				break
			}
		}
		if i == 0 {
			internalIP, externalIP = nic.NetworkIP, ext
			network, subnet = last(nic.Network), last(nic.Subnetwork)
		}
		aliases := make([]string, 0, len(nic.AliasIpRanges))
		for _, a := range nic.AliasIpRanges {
			aliases = append(aliases, a.IpCidrRange)
		}
		c.add(SNetwork, model.Row{
			"project": c.project, "vm": in.Name, "zone": zone, "nic": nic.Name, "nicType": nic.NicType,
			"network": last(nic.Network), "networkProject": projectOf(nic.Network), "subnet": last(nic.Subnetwork),
			"internalIP": nic.NetworkIP, "aliasRanges": strings.Join(aliases, "; "), "externalIP": ext,
			"networkTier": tier, "stackType": nic.StackType, "ipv6": nic.Ipv6Address, "ipForward": in.CanIpForward,
		})
	}

	// Disks and licenses
	var bootGB, totalGB float64
	var image, osName string
	for _, ad := range in.Disks {
		d := c.disks[ad.Source]
		diskName, dtype := last(ad.Source), ""
		size := float64(ad.DiskSizeGb)
		var iops, thr int64
		enc, srcImage := encryption(ad.DiskEncryptionKey), ""
		if ad.Type == "SCRATCH" {
			dtype, diskName = "local-ssd", ad.DeviceName
		}
		if d != nil {
			dtype = last(d.Type)
			if size == 0 {
				size = float64(d.SizeGb)
			}
			iops, thr = d.ProvisionedIops, d.ProvisionedThroughput
			enc, srcImage = encryption(d.DiskEncryptionKey), last(d.SourceImage)
		}
		totalGB += size
		if ad.Boot {
			bootGB, image = size, srcImage
			for _, l := range ad.Licenses {
				if cat := licenseCategory(last(l)); cat != "SQL Server" && cat != "Herramienta" {
					osName = last(l)
					break
				}
			}
		}
		for _, l := range ad.Licenses {
			c.add(SLicense, model.Row{
				"project": c.project, "vm": in.Name, "zone": zone, "disk": diskName,
				"category": licenseCategory(last(l)), "license": last(l), "licenseProject": projectOf(l),
			})
		}
		c.add(SDisk, model.Row{
			"project": c.project, "vm": in.Name, "zone": zone, "deviceName": ad.DeviceName, "disk": diskName,
			"boot": ad.Boot, "autoDelete": ad.AutoDelete, "mode": ad.Mode, "interface": ad.Interface,
			"type": dtype, "sizeGB": size, "iops": iops, "throughput": thr, "encryption": enc,
			"sourceImage": srcImage, "licenses": joinLast(ad.Licenses),
		})
	}
	if osName == "" {
		osName = image
	}

	gpus := make([]string, 0, len(in.GuestAccelerators))
	for _, g := range in.GuestAccelerators {
		gpus = append(gpus, fmt.Sprintf("%s x%d", last(g.AcceleratorType), g.AcceleratorCount))
	}
	sa, scopes := "", ""
	if len(in.ServiceAccounts) > 0 {
		sa = in.ServiceAccounts[0].Email
		scopes = joinLast(in.ServiceAccounts[0].Scopes)
	}
	tags := ""
	if in.Tags != nil {
		tags = strings.Join(in.Tags.Items, "; ")
	}
	mig := ""
	if in.Metadata != nil {
		for _, it := range in.Metadata.Items {
			if it.Key == "created-by" && it.Value != nil {
				mig = last(*it.Value)
			}
		}
	}
	secureBoot, vtpm := false, false
	if s := in.ShieldedInstanceConfig; s != nil {
		secureBoot, vtpm = s.EnableSecureBoot, s.EnableVtpm
	}
	confidential := in.ConfidentialInstanceConfig != nil && in.ConfidentialInstanceConfig.EnableConfidentialCompute
	var threads, visible int64
	nested := false
	if f := in.AdvancedMachineFeatures; f != nil {
		threads, visible, nested = f.ThreadsPerCore, f.VisibleCoreCount, f.EnableNestedVirtualization
	}

	c.add(SInfo, model.Row{
		"project": c.project, "name": in.Name, "id": fmt.Sprint(in.Id), "zone": zone, "region": regionOfZone(zone),
		"status": in.Status, "machineType": mtName, "family": family, "vcpu": vcpu, "memoryGB": memGB,
		"cpuPlatform": in.CpuPlatform, "provisioning": provisioning, "onHostMaintenance": onHost,
		"automaticRestart": autoRestart, "created": ts(in.CreationTimestamp),
		"lastStart": ts(in.LastStartTimestamp), "lastStop": ts(in.LastStopTimestamp),
		"internalIP": internalIP, "externalIP": externalIP, "network": network, "subnet": subnet,
		"os": osName, "image": image, "disks": int64(len(in.Disks)), "bootDiskGB": bootGB, "totalDiskGB": totalGB,
		"gpus": strings.Join(gpus, "; "), "serviceAccount": sa, "scopes": scopes,
		"labels": labels(in.Labels), "tags": tags, "deletionProtection": in.DeletionProtection,
		"secureBoot": secureBoot, "vtpm": vtpm, "confidential": confidential, "mig": mig,
		"hostname": in.Hostname, "selfLink": in.SelfLink,
	})
	c.add(SCPU, model.Row{
		"project": c.project, "name": in.Name, "zone": zone, "machineType": mtName, "vcpu": vcpu,
		"threadsPerCore": threads, "visibleCores": visible, "cpuPlatform": in.CpuPlatform,
		"minCpuPlatform": in.MinCpuPlatform, "custom": custom, "sharedCore": sharedCore, "nestedVirt": nested,
	})
	perVCPU := 0.0
	if vcpu > 0 {
		perVCPU = round2(memGB / float64(vcpu))
	}
	c.add(SMemory, model.Row{
		"project": c.project, "name": in.Name, "zone": zone, "machineType": mtName, "memoryGB": memGB,
		"gbPerVcpu": perVCPU, "custom": custom, "extended": strings.HasSuffix(mtName, "-ext"),
	})
}

// ---------- snapshots ----------

func (c *collector) collectSnapshots() error {
	return c.svc.Snapshots.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.SnapshotList) error {
		for _, s := range page.Items {
			var age any
			if t, ok := ts(s.CreationTimestamp).(time.Time); ok {
				age = int64(c.now.Sub(t).Hours() / 24)
			}
			c.add(SSnapshot, model.Row{
				"project": c.project, "name": s.Name, "sourceDisk": last(s.SourceDisk),
				"diskSizeGB": float64(s.DiskSizeGb), "storageGB": round2(float64(s.StorageBytes) / (1 << 30)),
				"status": s.Status, "created": ts(s.CreationTimestamp), "ageDays": age, "type": s.SnapshotType,
				"autoCreated": s.AutoCreated, "locations": strings.Join(s.StorageLocations, "; "),
				"encryption": encryption(s.SnapshotEncryptionKey), "labels": labels(s.Labels),
			})
		}
		return nil
	})
}

// ---------- managed instance groups ----------

func (c *collector) collectMIGs() error {
	autoscalers := map[string]*compute.Autoscaler{} // target MIG selfLink -> autoscaler
	err := c.svc.Autoscalers.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.AutoscalerAggregatedList) error {
			for _, scoped := range page.Items {
				for _, a := range scoped.Autoscalers {
					autoscalers[a.Target] = a
				}
			}
			return nil
		})
	if err != nil {
		return err
	}
	return c.svc.InstanceGroupManagers.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.InstanceGroupManagerAggregatedList) error {
			for _, scoped := range page.Items {
				for _, m := range scoped.InstanceGroupManagers {
					loc, scope := last(m.Zone), "zonal"
					if loc == "" {
						loc, scope = last(m.Region), "regional"
					}
					row := model.Row{
						"project": c.project, "name": m.Name, "location": loc, "scope": scope,
						"template": last(m.InstanceTemplate), "targetSize": m.TargetSize,
						"stable": m.Status != nil && m.Status.IsStable, "autoscaling": false,
						"baseInstanceName": m.BaseInstanceName,
					}
					hcs := make([]string, 0, len(m.AutoHealingPolicies))
					for _, h := range m.AutoHealingPolicies {
						hcs = append(hcs, last(h.HealthCheck))
					}
					row["autoHealing"] = strings.Join(hcs, "; ")
					if m.UpdatePolicy != nil {
						row["updateType"] = m.UpdatePolicy.Type
					}
					if a := autoscalers[m.SelfLink]; a != nil && a.AutoscalingPolicy != nil {
						p := a.AutoscalingPolicy
						row["autoscaling"] = p.Mode != "OFF"
						row["autoscalingMode"] = p.Mode
						row["minReplicas"] = p.MinNumReplicas
						row["maxReplicas"] = p.MaxNumReplicas
						if p.CpuUtilization != nil {
							row["cpuTarget"] = p.CpuUtilization.UtilizationTarget
						}
					}
					c.add(SMIG, row)
				}
			}
			return nil
		})
}

// ---------- instance templates ----------

func (c *collector) collectTemplates() error {
	return c.svc.InstanceTemplates.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.InstanceTemplateList) error {
		for _, t := range page.Items {
			row := model.Row{"project": c.project, "name": t.Name, "created": ts(t.CreationTimestamp)}
			if p := t.Properties; p != nil {
				row["machineType"] = p.MachineType
				row["labels"] = labels(p.Labels)
				for _, d := range p.Disks {
					if d.Boot && d.InitializeParams != nil {
						row["diskType"] = d.InitializeParams.DiskType
						row["diskGB"] = float64(d.InitializeParams.DiskSizeGb)
						row["image"] = last(d.InitializeParams.SourceImage)
					}
				}
				if len(p.NetworkInterfaces) > 0 {
					row["network"] = last(p.NetworkInterfaces[0].Network)
				}
				if len(p.ServiceAccounts) > 0 {
					row["serviceAccount"] = p.ServiceAccounts[0].Email
				}
				row["provisioning"] = "STANDARD"
				if s := p.Scheduling; s != nil {
					if s.ProvisioningModel != "" {
						row["provisioning"] = s.ProvisioningModel
					} else if s.Preemptible {
						row["provisioning"] = "PREEMPTIBLE"
					}
				}
			}
			c.add(STemplate, row)
		}
		return nil
	})
}
