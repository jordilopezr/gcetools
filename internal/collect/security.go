package collect

import (
	"gcetools/internal/schema"
	"slices"
	"strings"

	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
)

// collectProjectMetadata reads the project-wide metadata every VM inherits
// (OS Login, SSH keys, serial port). One call per project, also in fast
// mode: Cloud Asset Inventory is not used for it.
func (c *collector) collectProjectMetadata() error {
	p, err := c.svc.Projects.Get(c.project).Context(c.ctx).Do()
	if err != nil {
		return err
	}
	c.meta = metadataMap(p.CommonInstanceMetadata)
	c.stats.OSLogin = metaTrue(c.meta["enable-oslogin"])
	c.stats.SSHKeys = int64(countKeys(c.meta))
	c.stats.SerialPort = metaTrue(c.meta["serial-port-enable"])
	return nil
}

func metadataMap(m *compute.Metadata) map[string]string {
	out := map[string]string{}
	if m == nil {
		return out
	}
	for _, it := range m.Items {
		if it.Value != nil {
			out[it.Key] = *it.Value
		}
	}
	return out
}

// metaTrue reads a boolean metadata value the way the guest environment
// does ("TRUE", "true", "1", "yes"…).
func metaTrue(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "true", "1", "yes", "y", "on":
		return true
	}
	return false
}

// countKeys counts the SSH public keys in ssh-keys and the legacy sshKeys.
func countKeys(meta map[string]string) int {
	n := 0
	for _, k := range []string{"ssh-keys", "sshKeys"} {
		for _, line := range strings.Split(meta[k], "\n") {
			if strings.TrimSpace(line) != "" {
				n++
			}
		}
	}
	return n
}

// metaOverride returns the instance value of key when set, else the
// project-wide one: instance metadata takes precedence.
func (c *collector) metaOverride(inst map[string]string, key string) bool {
	if v, ok := inst[key]; ok && strings.TrimSpace(v) != "" {
		return metaTrue(v)
	}
	return metaTrue(c.meta[key])
}

// addSecurity writes the per-VM security row. Internet exposure (ports open
// by firewall rules) needs every project's firewalls, so the security
// package fills it once the scan ends.
func (c *collector) addSecurity(in *compute.Instance, zone string, meta map[string]string, mgd managed, encs []string) {
	osLogin := c.metaOverride(meta, "enable-oslogin")
	block := metaTrue(meta["block-project-ssh-keys"])
	sa, fullScope := "", false
	if len(in.ServiceAccounts) > 0 {
		sa = in.ServiceAccounts[0].Email
		fullScope = slices.ContainsFunc(in.ServiceAccounts[0].Scopes, func(s string) bool {
			return strings.HasSuffix(s, "/cloud-platform")
		})
	}
	var ext []string
	for _, nic := range in.NetworkInterfaces {
		for _, ac := range nic.AccessConfigs {
			if ac.NatIP != "" {
				ext = append(ext, ac.NatIP)
			}
		}
	}
	secureBoot, vtpm, integrity := false, false, false
	if s := in.ShieldedInstanceConfig; s != nil {
		secureBoot, vtpm, integrity = s.EnableSecureBoot, s.EnableVtpm, s.EnableIntegrityMonitoring
	}
	slices.Sort(encs)
	c.add(schema.SSecurity, model.Row{
		"project": c.project, "vm": in.Name, "zone": zone, "status": in.Status, "managedBy": mgd.service,
		"externalIP": strings.Join(ext, "; "), "internetExposed": false, "exposedPorts": "", "exposingRules": "",
		"serviceAccount": sa, "defaultSA": strings.HasSuffix(sa, "-compute@developer.gserviceaccount.com"),
		"fullScope": fullScope, "osLogin": osLogin,
		"projectSSHKeys":  !osLogin && !block && countKeys(c.meta) > 0,
		"instanceSSHKeys": !osLogin && countKeys(meta) > 0,
		"serialPort":      c.metaOverride(meta, "serial-port-enable"),
		"ipForward":       in.CanIpForward,
		"secureBoot":      secureBoot, "vtpm": vtpm, "integrityMonitoring": integrity,
		"confidential":   in.ConfidentialInstanceConfig != nil && in.ConfidentialInstanceConfig.EnableConfidentialCompute,
		"diskEncryption": strings.Join(slices.Compact(encs), ", "),
		"issues":         int64(0), "notes": "",
	})
}
