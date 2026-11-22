// Package security completes the gSecurity sheet once every project has
// been read: which VMs are reachable from the Internet on sensitive ports
// (their external IP plus the VPC firewall rules that apply to them, shared
// VPC host projects included) and how many security problems each VM has.
//
// Only VPC firewall rules are evaluated. Hierarchical firewall policies and
// network firewall policies are not read, so a VM reported as exposed may
// still be protected by an organization policy, and load balancers are not
// considered.
package security

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"gcetools/internal/collect"
	"gcetools/internal/i18n"
	"gcetools/internal/model"
)

// Port is a TCP port worth reporting when open to the whole Internet.
type Port struct {
	Number  int
	Service string
}

// Sensitive lists remote administration, file sharing and database ports.
var Sensitive = []Port{
	{22, "SSH"}, {3389, "RDP"}, {23, "Telnet"}, {21, "FTP"}, {5900, "VNC"},
	{445, "SMB"}, {135, "RPC"}, {139, "NetBIOS"}, {2375, "Docker"},
	{1433, "SQL Server"}, {1521, "Oracle"}, {3306, "MySQL"}, {5432, "PostgreSQL"},
	{27017, "MongoDB"}, {6379, "Redis"}, {9200, "Elasticsearch"}, {11211, "Memcached"},
}

type rule struct {
	project, name string
	priority      int64
	deny          bool
	allTargets    bool
	targets       []string // network tags and service account emails
	protos        []protoPorts
}

type protoPorts struct {
	proto  string
	ranges [][2]int // empty = every port
}

// Evaluate fills the exposure and problem columns of gSecurity and the
// exposed VM count of gProject. Inventories without gSecurity (scanned with
// older versions) are left untouched.
func Evaluate(inv *model.Inventory) {
	sec := inv.Sheet(collect.SSecurity)
	if sec == nil {
		return
	}
	rows := func(name string) []model.Row {
		if s := inv.Sheet(name); s != nil {
			return s.Rows
		}
		return nil
	}

	// Firewall rules that let traffic in from anywhere, by host project/VPC.
	byNet := map[string][]rule{}
	known := map[string]bool{} // projects whose firewall rules were read
	for _, r := range rows(collect.SFirewall) {
		p := str(r, "project")
		known[p] = true
		if str(r, "direction") != "INGRESS" || boolean(r, "disabled") || !fromAnywhere(str(r, "sourceRanges")) {
			continue
		}
		fr := rule{project: p, name: str(r, "name"), priority: integer(r, "priority"),
			deny: strings.EqualFold(str(r, "action"), "DENY"), protos: parseRules(str(r, "rules"))}
		for _, t := range split(str(r, "targets")) {
			if allInstances(t) {
				fr.allTargets = true
			} else {
				fr.targets = append(fr.targets, t)
			}
		}
		if len(fr.targets) == 0 {
			fr.allTargets = true
		}
		k := p + "/" + str(r, "network")
		byNet[k] = append(byNet[k], fr)
	}
	for _, r := range rows(collect.SProject) {
		if strings.HasPrefix(str(r, "scanStatus"), "OK") {
			known[str(r, "projectId")] = true
		}
	}

	type vmInfo struct {
		tags []string
		sa   string
	}
	vms := map[string]vmInfo{}
	for _, r := range rows(collect.SInfo) {
		vms[str(r, "project")+"/"+str(r, "zone")+"/"+str(r, "name")] = vmInfo{split(str(r, "tags")), str(r, "serviceAccount")}
	}
	type nic struct{ network, host string }
	nics := map[string][]nic{} // VM -> NICs with an external IP
	for _, r := range rows(collect.SNetwork) {
		if str(r, "externalIP") == "" {
			continue
		}
		k := str(r, "project") + "/" + str(r, "zone") + "/" + str(r, "vm")
		host := str(r, "networkProject")
		if host == "" {
			host = str(r, "project")
		}
		nics[k] = append(nics[k], nic{str(r, "network"), host})
	}

	exposed := map[string]int64{}
	for _, r := range sec.Rows {
		p := str(r, "project")
		k := p + "/" + str(r, "zone") + "/" + str(r, "vm")
		vm := vms[k]
		open := map[int]bool{}
		var by []string
		allPorts := true
		var notes []string
		for _, n := range nics[k] {
			if !known[n.host] {
				notes = append(notes, i18n.Tf("Sin datos del firewall de la VPC %s/%s (proyecto no escaneado o sin permisos)", n.host, n.network))
				continue
			}
			var applied []rule
			for _, fr := range byNet[n.host+"/"+n.network] {
				if fr.allTargets || slices.ContainsFunc(fr.targets, func(t string) bool { return t == vm.sa || slices.Contains(vm.tags, t) }) {
					applied = append(applied, fr)
				}
			}
			for _, sp := range Sensitive {
				if w := winner(applied, sp.Number); w != nil && !w.deny {
					open[sp.Number] = true
					name := w.name
					if w.project != p {
						name = w.project + "/" + w.name
					}
					if !slices.Contains(by, name) {
						by = append(by, name)
					}
					if !w.everyPort() {
						allPorts = false
					}
				}
			}
		}
		var ports []string
		for _, sp := range Sensitive {
			if open[sp.Number] {
				ports = append(ports, fmt.Sprintf("%d (%s)", sp.Number, sp.Service))
			}
		}
		if len(ports) == len(Sensitive) && allPorts {
			ports = []string{i18n.T("Todos los puertos TCP")}
		}
		isExposed := len(ports) > 0
		r["internetExposed"] = isExposed
		r["exposedPorts"] = strings.Join(ports, ", ")
		r["exposingRules"] = strings.Join(by, "; ")
		r["notes"] = strings.Join(notes, "; ")
		if isExposed {
			exposed[p]++
		}
		r["issues"] = Issues(r)
	}
	for _, r := range rows(collect.SProject) {
		r["exposedVMs"] = exposed[str(r, "projectId")]
	}
}

// Issues counts the security problems of one gSecurity row. OS Login and IP
// forwarding are not counted for VMs managed by GKE, Dataproc, Dataflow,
// Batch or a MIG: those services set them on purpose.
func Issues(r model.Row) int64 {
	managed := str(r, "managedBy") != ""
	n := int64(0)
	for _, bad := range []bool{
		boolean(r, "internetExposed"),
		boolean(r, "defaultSA"),
		!managed && !boolean(r, "osLogin"),
		boolean(r, "projectSSHKeys"),
		boolean(r, "serialPort"),
		!managed && boolean(r, "ipForward"),
		!boolean(r, "secureBoot"),
		!boolean(r, "vtpm") || !boolean(r, "integrityMonitoring"),
	} {
		if bad {
			n++
		}
	}
	return n
}

// winner returns the rule that decides traffic to a TCP port: the lowest
// priority number wins and, on a tie, deny beats allow. Nil when no rule
// applies (the implied deny).
func winner(rules []rule, port int) *rule {
	var w *rule
	for i := range rules {
		fr := &rules[i]
		if !fr.covers(port) {
			continue
		}
		if w == nil || fr.priority < w.priority || fr.priority == w.priority && fr.deny && !w.deny {
			w = fr
		}
	}
	return w
}

func (fr *rule) covers(port int) bool {
	for _, pp := range fr.protos {
		if pp.proto != "tcp" && pp.proto != "all" && pp.proto != "6" {
			continue
		}
		if len(pp.ranges) == 0 {
			return true
		}
		for _, rg := range pp.ranges {
			if port >= rg[0] && port <= rg[1] {
				return true
			}
		}
	}
	return false
}

// everyPort reports whether the rule opens the whole TCP range.
func (fr *rule) everyPort() bool {
	for _, pp := range fr.protos {
		if pp.proto != "tcp" && pp.proto != "all" && pp.proto != "6" {
			continue
		}
		if len(pp.ranges) == 0 {
			return true
		}
		for _, rg := range pp.ranges {
			if rg[0] <= 1 && rg[1] >= 65535 {
				return true
			}
		}
	}
	return false
}

// OpensAdminOrAllPorts reports whether a gFirewall "rules" value ("tcp:22,
// 80-90; udp; all") lets in TCP 22 (SSH), TCP 3389 (RDP) or every TCP port.
func OpensAdminOrAllPorts(rules string) bool {
	fr := rule{protos: parseRules(rules)}
	return fr.covers(22) || fr.covers(3389) || fr.everyPort()
}

// parseRules reads the gFirewall "rules" column: "tcp:22,80-90; udp; all".
func parseRules(s string) []protoPorts {
	var out []protoPorts
	for _, part := range split(s) {
		proto, ports, _ := strings.Cut(part, ":")
		pp := protoPorts{proto: strings.ToLower(strings.TrimSpace(proto))}
		for _, p := range strings.Split(ports, ",") {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			lo, hi, isRange := strings.Cut(p, "-")
			a, err1 := strconv.Atoi(lo)
			b := a
			var err2 error
			if isRange {
				b, err2 = strconv.Atoi(hi)
			}
			if err1 == nil && err2 == nil {
				pp.ranges = append(pp.ranges, [2]int{a, b})
			}
		}
		out = append(out, pp)
	}
	return out
}

func fromAnywhere(ranges string) bool {
	for _, r := range split(ranges) {
		if r == "0.0.0.0/0" || r == "::/0" {
			return true
		}
	}
	return false
}

// allInstances recognizes the gFirewall placeholder for rules without target
// tags or service accounts, in the scan's language or in Spanish.
func allInstances(t string) bool {
	return t == "(todas las instancias)" || t == i18n.T("(todas las instancias)")
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ";") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

func str(r model.Row, k string) string {
	s, _ := r[k].(string)
	return s
}

func boolean(r model.Row, k string) bool {
	b, _ := r[k].(bool)
	return b
}

func integer(r model.Row, k string) int64 {
	switch v := r[k].(type) {
	case int64:
		return v
	case float64:
		return int64(v)
	}
	return 0
}
