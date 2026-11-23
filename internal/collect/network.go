package collect

import (
	"fmt"
	"gcetools/internal/schema"
	"slices"
	"strings"

	"gcetools/internal/i18n"
	"gcetools/internal/model"

	compute "google.golang.org/api/compute/v1"
)

func (c *collector) collectNetworks() error {
	if ok, err := each(c, "Network", c.addNetwork); ok {
		return err
	}
	return c.svc.Networks.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.NetworkList) error {
		for _, n := range page.Items {
			c.addNetwork(n)
		}
		return nil
	})
}

func (c *collector) addNetwork(n *compute.Network) {
	mode := "custom"
	if n.IPv4Range != "" {
		mode = "legacy"
	} else if n.AutoCreateSubnetworks {
		mode = "auto"
	}
	routing := ""
	if n.RoutingConfig != nil {
		routing = n.RoutingConfig.RoutingMode
	}
	peers := make([]string, 0, len(n.Peerings))
	for _, p := range n.Peerings {
		peers = append(peers, fmt.Sprintf("%s → %s/%s (%s)", p.Name, projectOf(p.Network), last(p.Network), p.State))
	}
	c.add(schema.SVPC, model.Row{
		"project": c.project, "name": n.Name, "mode": mode, "routingMode": routing, "mtu": n.Mtu,
		"subnets": int64(len(n.Subnetworks)), "peerings": strings.Join(peers, "; "),
		"created": ts(n.CreationTimestamp), "description": n.Description,
	})
}

func (c *collector) collectSubnets() error {
	if ok, err := each(c, "Subnetwork", c.addSubnet); ok {
		return err
	}
	return c.svc.Subnetworks.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.SubnetworkAggregatedList) error {
			for _, scoped := range page.Items {
				for _, s := range scoped.Subnetworks {
					c.addSubnet(s)
				}
			}
			return nil
		})
}

func (c *collector) addSubnet(s *compute.Subnetwork) {
	sec := make([]string, 0, len(s.SecondaryIpRanges))
	for _, r := range s.SecondaryIpRanges {
		sec = append(sec, r.RangeName+"="+r.IpCidrRange)
	}
	ipv6 := s.Ipv6CidrRange
	if ipv6 == "" {
		ipv6 = s.ExternalIpv6Prefix
	}
	c.add(schema.SSubnet, model.Row{
		"project": c.project, "name": s.Name, "region": last(s.Region), "network": last(s.Network),
		"cidr": s.IpCidrRange, "secondary": strings.Join(sec, "; "), "gateway": s.GatewayAddress,
		"privateGoogleAccess": s.PrivateIpGoogleAccess,
		"flowLogs":            (s.LogConfig != nil && s.LogConfig.Enable) || s.EnableFlowLogs,
		"purpose":             s.Purpose, "stackType": s.StackType, "ipv6": ipv6,
	})
}

func (c *collector) collectFirewalls() error {
	if ok, err := each(c, "Firewall", c.addFirewall); ok {
		return err
	}
	return c.svc.Firewalls.List(c.project).Context(c.ctx).Pages(c.ctx, func(page *compute.FirewallList) error {
		for _, f := range page.Items {
			c.addFirewall(f)
		}
		return nil
	})
}

func (c *collector) addFirewall(f *compute.Firewall) {
	action, rules := "ALLOW", []string{}
	for _, a := range f.Allowed {
		rules = append(rules, protoPorts(a.IPProtocol, a.Ports))
	}
	if len(f.Denied) > 0 {
		action = "DENY"
		for _, d := range f.Denied {
			rules = append(rules, protoPorts(d.IPProtocol, d.Ports))
		}
	}
	srcTags := append(slices.Clone(f.SourceTags), f.SourceServiceAccounts...)
	targets := append(slices.Clone(f.TargetTags), f.TargetServiceAccounts...)
	if len(targets) == 0 {
		targets = []string{i18n.T("(todas las instancias)")}
	}
	open := f.Direction == "INGRESS" && action == "ALLOW" && !f.Disabled &&
		(slices.Contains(f.SourceRanges, "0.0.0.0/0") || slices.Contains(f.SourceRanges, "::/0"))
	c.add(schema.SFirewall, model.Row{
		"project": c.project, "name": f.Name, "network": last(f.Network), "direction": f.Direction,
		"priority": f.Priority, "action": action, "rules": strings.Join(rules, "; "),
		"sourceRanges": strings.Join(f.SourceRanges, "; "), "sourceTags": strings.Join(srcTags, "; "),
		"destinationRanges": strings.Join(f.DestinationRanges, "; "), "targets": strings.Join(targets, "; "),
		"disabled": f.Disabled, "logging": f.LogConfig != nil && f.LogConfig.Enable, "openToInternet": open,
	})
}

func protoPorts(proto string, ports []string) string {
	if len(ports) == 0 {
		return proto
	}
	return proto + ":" + strings.Join(ports, ",")
}

func (c *collector) collectAddresses() error {
	if ok, err := each(c, "Address", c.addAddress); ok {
		if err != nil {
			return err
		}
		_, err = each(c, "GlobalAddress", c.addAddress)
		return err
	}
	return c.svc.Addresses.AggregatedList(c.project).Context(c.ctx).
		Pages(c.ctx, func(page *compute.AddressAggregatedList) error {
			for _, scoped := range page.Items {
				for _, a := range scoped.Addresses {
					c.addAddress(a)
				}
			}
			return nil
		})
}

func (c *collector) addAddress(a *compute.Address) {
	loc := last(a.Region)
	if loc != "" {
		c.regions[loc] = true
	} else {
		loc = "global"
	}
	c.add(schema.SAddress, model.Row{
		"project": c.project, "name": a.Name, "address": a.Address, "type": a.AddressType,
		"location": loc, "status": a.Status, "users": joinLast(a.Users),
		"networkTier": a.NetworkTier, "purpose": a.Purpose, "created": ts(a.CreationTimestamp),
	})
}
