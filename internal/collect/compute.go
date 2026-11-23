package collect

import (
	"fmt"
	"strings"
	"time"

	"gcetools/internal/i18n"
	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
)

// ---------- disks (gDisks) ----------

func (c *collector) collectDisks() error {
	if ok, err := each(c, "Disk", func(d *compute.Disk) { c.disks[d.SelfLink] = d; c.addDisk(d) }); ok {
		return err
	}
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
	if ok, err := each(c, "Instance", c.addInstance); ok {
		return err
	}
	return c.svc.Instances.AggregatedList(c.project).Fields(instanceFields).Context(c.ctx).
		Pages(c.ctx, func(page *compute.InstanceAggregatedList) error {
			for _, scoped := range page.Items {
				for _, in := range scoped.Instances {
					c.addInstance(in)
				}
			}
			return nil
		})
}

// instanceFields limits the instance listing to what addInstance and
// addSecurity read. It leaves out, among others, the UEFI certificates of
// each disk (shieldedInstanceInitialState, several KB per disk), resource
// status and fingerprints. Metadata cannot be filtered by key, so GKE's
// kube-env still comes along. Add a field here when a collector starts
// reading it.
const instanceFields = "nextPageToken,items/*/instances(" +
	"name,id,zone,machineType,status,creationTimestamp,lastStartTimestamp,lastStopTimestamp," +
	"cpuPlatform,minCpuPlatform,hostname,labels,metadata,tags,scheduling,serviceAccounts," +
	"networkInterfaces,guestAccelerators,shieldedInstanceConfig,confidentialInstanceConfig," +
	"advancedMachineFeatures,canIpForward,deletionProtection,selfLink," +
	"disks(boot,deviceName,diskEncryptionKey,diskSizeGb,licenses,source,type,interface,autoDelete,mode))"

// machineType resolves vCPU/RAM for a machine type, caching per zone.
// Custom types are resolved too. A failed lookup is cached as nil.
func (c *collector) machineType(zone, name string) *compute.MachineType {
	key := zone + "/" + name
	if mt, ok := c.mtypes[key]; ok {
		return mt
	}
	if mt := c.svcs.MachineTypes.get(key); mt != nil {
		c.mtypes[key] = mt
		return mt
	}
	mt, err := c.svc.MachineTypes.Get(c.project, zone, name).Context(c.ctx).Do()
	if err != nil {
		mt = nil // failures stay per project: another project may have access
	}
	c.mtypes[key] = mt
	c.svcs.MachineTypes.put(key, mt)
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
	var encs []string
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
		if ad.Type != "SCRATCH" {
			encs = append(encs, enc)
		}
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
				"category": i18n.T(licenseCategory(last(l))), "license": last(l), "licenseProject": projectOf(l),
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
	meta := metadataMap(in.Metadata)
	mig := ""
	if v := meta["created-by"]; strings.Contains(v, "instanceGroupManagers/") {
		mig = last(v)
	}
	mgd := managedBy(in.Labels, meta, mig)
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
		"managedBy": mgd.service, "gkeCluster": mgd.cluster, "gkeNodePool": mgd.pool,
		"hostname": in.Hostname, "selfLink": in.SelfLink,
	})
	c.add(SCPU, model.Row{
		"project": c.project, "name": in.Name, "zone": zone, "machineType": mtName, "vcpu": vcpu,
		"threadsPerCore": threads, "visibleCores": visible, "cpuPlatform": in.CpuPlatform,
		"minCpuPlatform": in.MinCpuPlatform, "custom": custom, "sharedCore": sharedCore, "nestedVirt": nested,
	})
	c.addSecurity(in, zone, meta, mgd, encs)
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
	if ok, err := each(c, "Snapshot", c.addSnapshot); ok {
		return err
	}
	return c.svc.Snapshots.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.SnapshotList) error {
		for _, s := range page.Items {
			c.addSnapshot(s)
		}
		return nil
	})
}

func (c *collector) addSnapshot(s *compute.Snapshot) {
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

// ---------- managed instance groups ----------

func (c *collector) collectMIGs() error {
	autoscalers := map[string]*compute.Autoscaler{} // target MIG selfLink -> autoscaler
	if ok, err := each(c, "Autoscaler", func(a *compute.Autoscaler) { autoscalers[a.Target] = a }); ok {
		if err != nil {
			return err
		}
		_, err = each(c, "InstanceGroupManager", func(m *compute.InstanceGroupManager) { c.addMIG(m, autoscalers) })
		return err
	}
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
					c.addMIG(m, autoscalers)
				}
			}
			return nil
		})
}

func (c *collector) addMIG(m *compute.InstanceGroupManager, autoscalers map[string]*compute.Autoscaler) {
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

// ---------- instance templates ----------

func (c *collector) collectTemplates() error {
	if ok, err := each(c, "InstanceTemplate", c.addTemplate); ok {
		return err
	}
	return c.svc.InstanceTemplates.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.InstanceTemplateList) error {
		for _, t := range page.Items {
			c.addTemplate(t)
		}
		return nil
	})
}

func (c *collector) addTemplate(t *compute.InstanceTemplate) {
	row := model.Row{"project": c.project, "name": t.Name, "created": ts(t.CreationTimestamp)}
	if p := t.Properties; p != nil {
		row["machineType"] = p.MachineType
		row["labels"] = labels(p.Labels)
		for _, d := range p.Disks {
			if d.Boot && d.InitializeParams != nil {
				row["diskType"] = d.InitializeParams.DiskType
				row["diskGB"] = float64(d.InitializeParams.DiskSizeGb)
				row["image"], row["imageProject"] = templateImage(d.InitializeParams.SourceImage, c.project)
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

// templateImage splits a template's source image into its name and project.
// Image families come back as "family/<name>": they always resolve to the
// newest non-deprecated image, so they must not match a concrete image.
func templateImage(src, project string) (name, imageProject string) {
	if src == "" {
		return "", ""
	}
	imageProject = projectOf("/" + strings.TrimPrefix(src, "/")) // also "projects/x/..." without host
	if imageProject == "" {
		imageProject = project // relative URL: global/images/<name>
	}
	name = last(src)
	if strings.Contains(src, "/family/") {
		name = "family/" + name
	}
	return name, imageProject
}

// managed describes the Google service that creates and owns a VM.
type managed struct{ service, cluster, pool string }

// managedBy recognises VMs created by Google services from the labels and
// metadata those services set. GKE nodes are also MIG members, so the more
// specific service wins.
func managedBy(lb, meta map[string]string, mig string) managed {
	has := func(keys ...string) bool {
		for _, k := range keys {
			if _, ok := lb[k]; ok {
				return true
			}
		}
		return false
	}
	if has("goog-gke-node", "goog-k8s-cluster-name") || meta["cluster-name"] != "" && meta["kube-labels"] != "" {
		m := managed{service: "GKE", cluster: lb["goog-k8s-cluster-name"], pool: lb["goog-k8s-node-pool-name"]}
		if m.cluster == "" {
			m.cluster = meta["cluster-name"]
		}
		if m.pool == "" {
			for _, kv := range strings.Split(meta["kube-labels"], ",") {
				if k, v, ok := strings.Cut(kv, "="); ok && k == "cloud.google.com/gke-nodepool" {
					m.pool = v
				}
			}
		}
		return m
	}
	switch {
	case has("goog-dataproc-cluster-name"):
		return managed{service: "Dataproc", cluster: lb["goog-dataproc-cluster-name"]}
	case has("goog-dataflow-job-id", "dataflow_job_id"):
		return managed{service: "Dataflow"}
	case has("goog-batch-job-uid", "batch-job-id"):
		return managed{service: "Batch"}
	case mig != "":
		return managed{service: "MIG"}
	}
	return managed{}
}
