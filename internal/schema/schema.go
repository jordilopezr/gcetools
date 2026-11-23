// Package schema defines the inventory sheets: names, titles, columns and
// their order. Collectors fill them; the analysis packages (health,
// security, pricing, diff…) read them without depending on the Google API
// clients in collect.
package schema

import (
	"strings"

	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Sheet names. The "g" prefix mirrors RVTools' "v" prefix (vInfo -> gInfo).
const (
	SInfo     = "gInfo"
	SCPU      = "gCPU"
	SMemory   = "gMemory"
	SDisk     = "gDisk"
	SNetwork  = "gNetwork"
	SDisks    = "gDisks"
	SSnapshot = "gSnapshot"
	SLicense  = "gLicense"
	SMIG      = "gMIG"
	STemplate = "gTemplate"
	SVPC      = "gVPC"
	SSubnet   = "gSubnet"
	SFirewall = "gFirewall"
	SAddress  = "gAddress"
	SImage    = "gImage"
	SReserv   = "gReservation"
	SCommit   = "gCommitment"
	SSole     = "gSoleTenant"
	SQuota    = "gQuota"
	STools    = "gTools"
	SPerf     = "gPerf"
	SRecomm   = "gRecommend"
	SCost     = "gCost"
	SCostReal = "gCostReal"
	SProject  = "gProject"
	SHealth   = "gHealth"
	SSecurity = "gSecurity"
	SErrors   = "gErrors"
	SMeta     = "gMetaData"
)

// c builds a column; the Spanish title is translated into the current
// language (TestSchemaTranslated checks every title has an English entry).
func c(key, title string, t model.ColType, width float64) model.Column {
	return model.Column{Key: key, Title: i18n.T(title), Type: t, Width: width}
}

const (
	S = model.TString
	I = model.TInt
	F = model.TFloat
	B = model.TBool
	T = model.TTime
	G = model.TGB
	M = model.TMoney
)

// NewInventory returns an empty inventory with every MVP sheet in display
// order, with titles and descriptions in the current language.
func NewInventory() *model.Inventory {
	sheet := func(name, title, desc string, cols ...model.Column) *model.Sheet {
		return &model.Sheet{Name: name, Title: i18n.T(title), Description: i18n.T(desc), Columns: cols, Rows: []model.Row{}}
	}
	return &model.Inventory{Errors: []model.ScanError{}, Sheets: []*model.Sheet{
		sheet(SInfo, "Instancias", "Una fila por VM (equivale a vInfo)",
			c("project", "Proyecto", S, 24), c("name", "VM", S, 28), c("id", "ID", S, 20),
			c("zone", "Zona", S, 16), c("region", "Región", S, 14), c("status", "Estado", S, 12),
			c("machineType", "Machine type", S, 18), c("family", "Familia", S, 9),
			c("vcpu", "vCPU", I, 7), c("memoryGB", "RAM (GB)", G, 10),
			c("cpuPlatform", "CPU platform", S, 18), c("provisioning", "Aprovisionamiento", S, 14),
			c("onHostMaintenance", "Mantenimiento host", S, 14), c("automaticRestart", "Auto restart", B, 10),
			c("created", "Creada", T, 18), c("lastStart", "Último arranque", T, 18), c("lastStop", "Última parada", T, 18),
			c("internalIP", "IP interna", S, 15), c("externalIP", "IP externa", S, 15),
			c("network", "VPC", S, 18), c("subnet", "Subred", S, 18),
			c("os", "SO (licencia)", S, 24), c("image", "Imagen", S, 30),
			c("disks", "Nº discos", I, 9), c("bootDiskGB", "Disco boot (GB)", G, 12), c("totalDiskGB", "Discos total (GB)", G, 13),
			c("gpus", "GPUs", S, 16), c("serviceAccount", "Service account", S, 34), c("scopes", "Scopes", S, 30),
			c("labels", "Labels", S, 30), c("tags", "Network tags", S, 22),
			c("deletionProtection", "Deletion protection", B, 10), c("secureBoot", "Secure Boot", B, 10),
			c("vtpm", "vTPM", B, 8), c("confidential", "Confidential", B, 10),
			c("mig", "MIG", S, 22), c("managedBy", "Gestionada por", S, 12),
			c("gkeCluster", "Clúster (GKE/Dataproc)", S, 22), c("gkeNodePool", "Node pool GKE", S, 20), c("hostname", "Hostname", S, 24), c("selfLink", "Self link", S, 40),
			c("costCompute", "Compute+GPU/mes (si encendida)", M, 14), c("costLicense", "Licencias/mes", M, 11),
			c("costDisks", "Discos/mes", M, 11), c("costMonthly", "Coste/mes actual", M, 12), c("costNote", "Notas de coste", S, 24),
			c("costReal", "Coste real mes (BigQuery)", M, 13),
		),
		sheet(SCPU, "CPU", "Detalle de CPU por VM (vCPU)",
			c("project", "Proyecto", S, 24), c("name", "VM", S, 28), c("zone", "Zona", S, 16),
			c("machineType", "Machine type", S, 18), c("vcpu", "vCPU", I, 7),
			c("threadsPerCore", "Threads/core", I, 10), c("visibleCores", "Cores visibles", I, 10),
			c("cpuPlatform", "CPU platform", S, 18), c("minCpuPlatform", "Min CPU platform", S, 18),
			c("custom", "Custom", B, 8), c("sharedCore", "Shared core", B, 10), c("nestedVirt", "Nested virt.", B, 10),
		),
		sheet(SMemory, "Memoria", "Detalle de memoria por VM (vMemory)",
			c("project", "Proyecto", S, 24), c("name", "VM", S, 28), c("zone", "Zona", S, 16),
			c("machineType", "Machine type", S, 18), c("memoryGB", "RAM (GB)", G, 10),
			c("gbPerVcpu", "GB/vCPU", G, 9), c("custom", "Custom", B, 8), c("extended", "Extended memory", B, 12),
		),
		sheet(SDisk, "Discos de VM", "Discos conectados a cada VM (vDisk)",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16),
			c("deviceName", "Device", S, 18), c("disk", "Disco", S, 28), c("boot", "Boot", B, 7),
			c("autoDelete", "Auto delete", B, 10), c("mode", "Modo", S, 11), c("interface", "Interfaz", S, 9),
			c("type", "Tipo", S, 16), c("sizeGB", "Tamaño (GB)", G, 11),
			c("iops", "IOPS aprov.", I, 10), c("throughput", "MB/s aprov.", I, 10),
			c("encryption", "Cifrado", S, 10), c("sourceImage", "Imagen origen", S, 30), c("licenses", "Licencias", S, 30),
		),
		sheet(SNetwork, "NICs", "Interfaces de red por VM (vNetwork)",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16), c("nic", "NIC", S, 7),
			c("nicType", "Tipo NIC", S, 9), c("network", "VPC", S, 18), c("networkProject", "Proyecto VPC", S, 20),
			c("subnet", "Subred", S, 18), c("internalIP", "IP interna", S, 15), c("aliasRanges", "Alias IP", S, 20),
			c("externalIP", "IP externa", S, 15), c("networkTier", "Tier", S, 9), c("stackType", "Stack", S, 10),
			c("ipv6", "IPv6", S, 22), c("ipForward", "IP forwarding", B, 10),
		),
		sheet(SDisks, "Todos los discos", "Inventario de discos, incluidos los huérfanos (vDatastore)",
			c("project", "Proyecto", S, 24), c("name", "Disco", S, 28), c("location", "Ubicación", S, 16),
			c("type", "Tipo", S, 16), c("sizeGB", "Tamaño (GB)", G, 11), c("status", "Estado", S, 9),
			c("users", "VMs", S, 28), c("orphan", "Huérfano", B, 9), c("created", "Creado", T, 18),
			c("lastAttach", "Último attach", T, 18), c("lastDetach", "Último detach", T, 18),
			c("iops", "IOPS aprov.", I, 10), c("throughput", "MB/s aprov.", I, 10), c("encryption", "Cifrado", S, 10),
			c("sourceImage", "Imagen origen", S, 30), c("sourceSnapshot", "Snapshot origen", S, 24),
			c("snapshotSchedule", "Snapshot schedule", S, 22), c("replicaZones", "Réplicas", S, 22),
			c("architecture", "Arquitectura", S, 10), c("labels", "Labels", S, 30),
			c("costMonthly", "Coste/mes", M, 10), c("costReal", "Coste real mes (BigQuery)", M, 13),
		),
		sheet(SSnapshot, "Snapshots", "Snapshots de disco (vSnapshot)",
			c("project", "Proyecto", S, 24), c("name", "Snapshot", S, 30), c("sourceDisk", "Disco origen", S, 28),
			c("diskSizeGB", "Tamaño disco (GB)", G, 12), c("storageGB", "Almacenado (GB)", G, 12),
			c("status", "Estado", S, 9), c("created", "Creado", T, 18), c("ageDays", "Antigüedad (días)", I, 10),
			c("type", "Tipo", S, 9), c("autoCreated", "Por schedule", B, 10),
			c("locations", "Ubicación", S, 14), c("encryption", "Cifrado", S, 10), c("labels", "Labels", S, 30),
			c("costMonthly", "Coste/mes", M, 10),
		),
		sheet(SLicense, "Licencias", "Licencias de SO y software en discos (vLicense)",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16), c("disk", "Disco", S, 28),
			c("category", "Categoría", S, 14), c("license", "Licencia", S, 36), c("licenseProject", "Proyecto licencia", S, 22),
		),
		sheet(SMIG, "MIGs", "Managed instance groups y autoscaling (vCluster)",
			c("project", "Proyecto", S, 24), c("name", "MIG", S, 28), c("location", "Ubicación", S, 16),
			c("scope", "Ámbito", S, 9), c("template", "Template", S, 30), c("targetSize", "Target size", I, 10),
			c("stable", "Estable", B, 8), c("autoscaling", "Autoscaling", B, 10), c("autoscalingMode", "Modo", S, 10),
			c("minReplicas", "Mín.", I, 6), c("maxReplicas", "Máx.", I, 6), c("cpuTarget", "CPU objetivo", F, 10),
			c("autoHealing", "Health check", S, 24), c("updateType", "Update policy", S, 12),
			c("baseInstanceName", "Nombre base", S, 20),
		),
		sheet(STemplate, "Templates", "Instance templates",
			c("project", "Proyecto", S, 24), c("name", "Template", S, 30), c("created", "Creado", T, 18),
			c("machineType", "Machine type", S, 18), c("diskType", "Tipo disco", S, 14), c("diskGB", "Disco (GB)", G, 10),
			c("image", "Imagen", S, 30), c("imageProject", "Proyecto imagen", S, 22), c("network", "VPC", S, 18),
			c("serviceAccount", "Service account", S, 34), c("provisioning", "Aprovisionamiento", S, 14), c("labels", "Labels", S, 30),
		),
		sheet(SVPC, "VPCs", "Redes VPC (vSwitch)",
			c("project", "Proyecto", S, 24), c("name", "VPC", S, 22), c("mode", "Modo subredes", S, 11),
			c("routingMode", "Routing", S, 10), c("mtu", "MTU", I, 7), c("subnets", "Nº subredes", I, 9),
			c("peerings", "Peerings", S, 30), c("created", "Creada", T, 18), c("description", "Descripción", S, 30),
		),
		sheet(SSubnet, "Subredes", "Subredes (dvPort)",
			c("project", "Proyecto", S, 24), c("name", "Subred", S, 22), c("region", "Región", S, 14),
			c("network", "VPC", S, 18), c("cidr", "CIDR", S, 16), c("secondary", "Rangos secundarios", S, 30),
			c("gateway", "Gateway", S, 14), c("privateGoogleAccess", "Private Google Access", B, 12),
			c("flowLogs", "Flow logs", B, 9), c("purpose", "Purpose", S, 16), c("stackType", "Stack", S, 10),
			c("ipv6", "IPv6", S, 22),
		),
		sheet(SFirewall, "Firewall", "Reglas de firewall VPC",
			c("project", "Proyecto", S, 24), c("name", "Regla", S, 28), c("network", "VPC", S, 18),
			c("direction", "Dirección", S, 9), c("priority", "Prioridad", I, 9), c("action", "Acción", S, 8),
			c("rules", "Protocolos/puertos", S, 24), c("sourceRanges", "Orígenes", S, 26),
			c("sourceTags", "Tags/SA origen", S, 22), c("destinationRanges", "Destinos", S, 22),
			c("targets", "Tags/SA destino", S, 22), c("disabled", "Deshabilitada", B, 10),
			c("logging", "Logging", B, 8), c("openToInternet", "Abierta a Internet", B, 12),
		),
		sheet(SAddress, "IPs", "Direcciones IP reservadas",
			c("project", "Proyecto", S, 24), c("name", "Nombre", S, 26), c("address", "IP", S, 16),
			c("type", "Tipo", S, 10), c("location", "Ubicación", S, 14), c("status", "Estado", S, 10),
			c("users", "Usada por", S, 34), c("networkTier", "Tier", S, 9), c("purpose", "Purpose", S, 16),
			c("created", "Creada", T, 18),
			c("costMonthly", "Coste/mes", M, 10),
		),
		sheet(SImage, "Imágenes", "Imágenes custom y machine images (vCD)",
			c("project", "Proyecto", S, 24), c("name", "Nombre", S, 30), c("kind", "Tipo", S, 13),
			c("family", "Familia", S, 18), c("status", "Estado", S, 9), c("deprecated", "Deprecación", S, 11),
			c("diskSizeGB", "Tamaño disco (GB)", G, 12), c("storageGB", "Almacenado (GB)", G, 12),
			c("created", "Creada", T, 18), c("ageDays", "Antigüedad (días)", I, 10), c("source", "Origen", S, 28),
			c("licenses", "Licencias", S, 30), c("locations", "Ubicación", S, 14), c("architecture", "Arquitectura", S, 10),
			c("labels", "Labels", S, 30),
			c("costMonthly", "Coste/mes", M, 10),
		),
		sheet(SReserv, "Reservas", "Reservas de capacidad zonal",
			c("project", "Proyecto", S, 24), c("name", "Reserva", S, 28), c("zone", "Zona", S, 16),
			c("machineType", "Machine type", S, 18), c("count", "Reservadas", I, 10), c("inUse", "En uso", I, 8),
			c("unusedPct", "Sin usar (%)", F, 10), c("specificOnly", "Solo específica", B, 11),
			c("gpus", "GPUs", S, 16), c("localSsdGB", "Local SSD (GB)", G, 12), c("status", "Estado", S, 10),
			c("commitment", "Commitment", S, 22), c("shareType", "Compartida", S, 12), c("created", "Creada", T, 18),
		),
		sheet(SCommit, "Commitments", "Committed use discounts (CUD) por recurso",
			c("project", "Proyecto", S, 24), c("name", "Commitment", S, 28), c("region", "Región", S, 14),
			c("plan", "Plan", S, 13), c("type", "Tipo", S, 18), c("category", "Categoría", S, 10), c("status", "Estado", S, 10),
			c("start", "Inicio", T, 18), c("end", "Fin", T, 18), c("daysLeft", "Días restantes", I, 10),
			c("vcpu", "vCPU", I, 7), c("memoryGB", "RAM (GB)", G, 10), c("other", "Otros recursos", S, 24),
			c("autoRenew", "Auto renovación", B, 10),
		),
		sheet(SSole, "Sole-tenant", "Nodos sole-tenant y VMs alojadas (vHost)",
			c("project", "Proyecto", S, 24), c("nodeGroup", "Node group", S, 24), c("zone", "Zona", S, 16),
			c("nodeTemplate", "Node template", S, 24), c("node", "Nodo", S, 28), c("nodeType", "Tipo de nodo", S, 18),
			c("status", "Estado", S, 9), c("vms", "Nº VMs", I, 7), c("instances", "VMs", S, 40),
			c("cpuOvercommit", "CPU overcommit", S, 12), c("maintenance", "Mantenimiento", S, 16), c("autoscaling", "Autoscaling", S, 10),
		),
		sheet(SQuota, "Cuotas", "Cuotas regionales de Compute con uso > 0",
			c("project", "Proyecto", S, 24), c("region", "Región", S, 16), c("metric", "Métrica", S, 30),
			c("usage", "Uso", F, 10), c("limit", "Límite", F, 10), c("usagePct", "Uso (%)", F, 9),
		),
		sheet(STools, "SO y agentes", "Inventario del SO invitado vía OS Config (vTools)",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16),
			c("os", "Sistema operativo", S, 34), c("shortName", "Familia SO", S, 12), c("version", "Versión", S, 14),
			c("kernel", "Kernel", S, 26), c("architecture", "Arquitectura", S, 10), c("hostname", "Hostname", S, 24),
			c("agentVersion", "Agente OS Config", S, 16),
			c("updated", "Actualizado", T, 18),
		),
		sheet(SPerf, "Rendimiento", "CPU y memoria (Cloud Monitoring); memoria requiere Ops Agent",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16), c("status", "Estado", S, 11),
			c("machineType", "Machine type", S, 18), c("vcpu", "vCPU", I, 7), c("memoryGB", "RAM (GB)", G, 10),
			c("days", "Días", I, 6), c("cpuAvg", "CPU media (%)", F, 10), c("cpuP95", "CPU p95 (%)", F, 10),
			c("cpuMax", "CPU máx (%)", F, 10), c("memAvg", "RAM media (%)", F, 10), c("memP95", "RAM p95 (%)", F, 10),
			c("memMax", "RAM máx (%)", F, 10), c("opsAgent", "Ops Agent", B, 9), c("samples", "Muestras", I, 9),
		),
		sheet(SRecomm, "Recomendaciones", "Recomendaciones activas de Google Recommender",
			c("project", "Proyecto", S, 24), c("category", "Impacto", S, 12), c("recommender", "Tipo", S, 22),
			c("subtype", "Subtipo", S, 20), c("resource", "Recurso", S, 30), c("location", "Ubicación", S, 16),
			c("description", "Descripción", S, 60), c("savingsMonthly", "Ahorro mensual", F, 12), c("currency", "Moneda", S, 7),
			c("priority", "Prioridad", S, 9), c("state", "Estado", S, 9), c("lastRefresh", "Actualizada", T, 18),
		),
		sheet(SCost, "Coste", "Coste mensual estimado a precio de lista (sin SUD, CUD ni descuentos)",
			c("project", "Proyecto", S, 26), c("vmsRunning", "VMs encendidas", I, 10), c("compute", "Compute+GPU", M, 12),
			c("licenses", "Licencias", M, 11), c("disks", "Discos", M, 11), c("snapshots", "Snapshots", M, 11),
			c("images", "Imágenes", M, 10), c("ips", "IPs sin uso", M, 10), c("total", "Total/mes", M, 12),
			c("currency", "Moneda", S, 7),
		),
		sheet(SCostReal, "Coste real", "Coste neto real de Compute Engine según la exportación de facturación a BigQuery",
			c("project", "Proyecto", S, 26), c("month", "Mes", S, 8), c("gross", "Coste bruto", M, 12),
			c("credits", "Créditos (CUD, SUD, descuentos)", M, 14), c("net", "Coste neto", M, 12),
			c("attributed", "Atribuido a VMs y discos", M, 13), c("unattributed", "No atribuido (red, IPs, snapshots, commitments…)", M, 16),
			c("estimate", "Estimado lista", M, 12), c("diff", "Neto − estimado", M, 12), c("diffPct", "Diferencia (%)", F, 10),
			c("currency", "Moneda", S, 7),
		),
		sheet(SProject, "Proyectos", "Proyectos escaneados (vSource)",
			c("projectId", "Proyecto", S, 26), c("number", "Número", S, 14), c("name", "Nombre", S, 26),
			c("parent", "Padre", S, 22), c("state", "Estado", S, 9), c("scanStatus", "Resultado escaneo", S, 22),
			c("vms", "VMs", I, 7), c("vmsRunning", "VMs encendidas", I, 10), c("vcpu", "vCPU", I, 7),
			c("memoryGB", "RAM (GB)", G, 10), c("disks", "Discos", I, 7), c("diskGB", "Discos (GB)", G, 11),
			c("labels", "Labels", S, 30),
			c("costMonthly", "Coste/mes", M, 12),
			c("osLogin", "OS Login (proyecto)", B, 11), c("sshKeys", "Claves SSH de proyecto", I, 11),
			c("serialPort", "Puerto serie (proyecto)", B, 11), c("exposedVMs", "VMs expuestas a Internet", I, 11),
		),
		sheet(SHealth, "Salud", "Hallazgos de buenas prácticas (vHealth)",
			c("severity", "Severidad", S, 9), c("category", "Categoría", S, 14), c("rule", "Regla", S, 34),
			c("ruleId", "ID de regla", S, 18), c("project", "Proyecto", S, 24), c("resource", "Recurso", S, 30), c("location", "Ubicación", S, 16),
			c("detail", "Detalle", S, 50),
		),
		sheet(SSecurity, "Seguridad", "Postura de seguridad por VM: exposición a Internet, acceso e identidad",
			c("project", "Proyecto", S, 24), c("vm", "VM", S, 28), c("zone", "Zona", S, 16), c("status", "Estado", S, 12),
			c("managedBy", "Gestionada por", S, 12), c("issues", "Problemas", I, 9),
			c("externalIP", "IP externa", S, 15), c("internetExposed", "Expuesta a Internet", B, 11),
			c("exposedPorts", "Puertos sensibles abiertos", S, 28), c("exposingRules", "Reglas que los abren", S, 30),
			c("serviceAccount", "Service account", S, 34), c("defaultSA", "SA por defecto", B, 10),
			c("fullScope", "Scope cloud-platform", B, 11), c("osLogin", "OS Login", B, 9),
			c("projectSSHKeys", "Acepta claves SSH de proyecto", B, 12), c("instanceSSHKeys", "Claves SSH en la VM", B, 11),
			c("serialPort", "Puerto serie", B, 9), c("ipForward", "IP forwarding", B, 10),
			c("secureBoot", "Secure Boot", B, 10), c("vtpm", "vTPM", B, 8), c("integrityMonitoring", "Integrity monitoring", B, 11),
			c("confidential", "Confidential", B, 10), c("diskEncryption", "Cifrado de discos", S, 14),
			c("notes", "Notas", S, 40),
		),
		sheet(SErrors, "Errores", "Proyectos o APIs que no se pudieron leer",
			c("project", "Proyecto", S, 26), c("collector", "Colector", S, 16), c("kind", "Tipo", S, 18),
			c("message", "Mensaje", S, 80),
		),
		sheet(SMeta, "Metadatos", "Información del escaneo (vMetaData)",
			c("property", "Propiedad", S, 24), c("value", "Valor", S, 60),
		),
	}}
}

// Relabel rewrites the sheet titles, descriptions and column titles of inv
// in the current language, matching sheets by name and columns by key.
// Inventories loaded from history or JSON keep the language of the scan
// that produced them; this keeps headers consistent with the UI. Row values
// are left as they are.
func Relabel(inv *model.Inventory) {
	if inv == nil {
		return
	}
	ref := map[string]*model.Sheet{}
	for _, s := range NewInventory().Sheets {
		ref[s.Name] = s
	}
	for _, s := range inv.Sheets {
		r := ref[s.Name]
		if r == nil {
			continue
		}
		s.Title, s.Description = r.Title, r.Description
		titles := make(map[string]string, len(r.Columns))
		for _, c := range r.Columns {
			titles[c.Key] = c.Title
		}
		for i, c := range s.Columns {
			if t, ok := titles[c.Key]; ok {
				s.Columns[i].Title = t
			}
		}
	}
}

// ScanOK reports whether a gProject scanStatus means the project was read
// ("OK", or "OK (sin osconfig)" when only an optional module failed). Every
// language keeps the "OK" prefix, so inventories in either language work.
func ScanOK(status string) bool { return strings.HasPrefix(status, "OK") }

// ScanComparable reports whether a project's data can be compared: read
// completely or partially ("Parcial (2 errores)" / "Partial (2 errors)").
func ScanComparable(status string) bool {
	return ScanOK(status) || strings.HasPrefix(status, "Parcial") || strings.HasPrefix(status, "Partial")
}
