package security

import (
	"gcetools/internal/schema"
	"testing"

	"gcetools/internal/model"
)

// inv builds an inventory with one VM per entry of vms (project/zone/name,
// external IP, network host project, tags, SA) and the given firewall rows.
func inv(t *testing.T, fws []model.Row, vms ...model.Row) *model.Inventory {
	t.Helper()
	in := schema.NewInventory()
	for _, v := range vms {
		p, z, n := v["project"].(string), "z1", v["vm"].(string)
		in.Sheet(schema.SInfo).Add(model.Row{"project": p, "zone": z, "name": n, "tags": v["tags"], "serviceAccount": v["sa"]})
		in.Sheet(schema.SNetwork).Add(model.Row{"project": p, "zone": z, "vm": n, "network": "net",
			"networkProject": v["host"], "externalIP": v["ip"]})
		in.Sheet(schema.SSecurity).Add(model.Row{"project": p, "zone": z, "vm": n, "externalIP": v["ip"],
			"osLogin": true, "secureBoot": true, "vtpm": true, "integrityMonitoring": true})
	}
	for _, f := range fws {
		row := model.Row{"direction": "INGRESS", "action": "ALLOW", "network": "net", "sourceRanges": "0.0.0.0/0",
			"targets": "(todas las instancias)", "priority": int64(1000)}
		for k, v := range f {
			row[k] = v
		}
		in.Sheet(schema.SFirewall).Add(row)
	}
	in.Sheet(schema.SProject).Add(model.Row{"projectId": "p1", "scanStatus": "OK"})
	return in
}

func sec(in *model.Inventory, i int) model.Row { return in.Sheet(schema.SSecurity).Rows[i] }

func TestExposure(t *testing.T) {
	in := inv(t, []model.Row{
		{"project": "p1", "name": "ssh-web", "rules": "tcp:22,80; udp:53", "targets": "web"},
		{"project": "p1", "name": "db", "rules": "tcp:3300-3310", "targets": "db-sa@p1.iam.gserviceaccount.com"},
		{"project": "p1", "name": "deny-ssh", "rules": "tcp:22", "action": "DENY", "priority": int64(100), "targets": "locked"},
		{"project": "p1", "name": "internal", "rules": "all", "sourceRanges": "10.0.0.0/8"},
		{"project": "p1", "name": "off", "rules": "all", "disabled": true},
	},
		model.Row{"project": "p1", "vm": "web", "ip": "34.0.0.1", "host": "p1", "tags": "web; http"},
		model.Row{"project": "p1", "vm": "db", "ip": "34.0.0.2", "host": "p1", "sa": "db-sa@p1.iam.gserviceaccount.com"},
		model.Row{"project": "p1", "vm": "locked", "ip": "34.0.0.3", "host": "p1", "tags": "web; locked"},
		model.Row{"project": "p1", "vm": "private", "ip": "", "host": "p1", "tags": "web"},
	)
	Evaluate(in)
	cases := []struct {
		i     int
		ports string
		rules string
	}{
		{0, "22 (SSH)", "ssh-web"},
		{1, "3306 (MySQL)", "db"},
		{2, "", ""}, // deny with a higher priority wins
		{3, "", ""}, // no external IP
	}
	for _, c := range cases {
		r := sec(in, c.i)
		if r["exposedPorts"] != c.ports || r["exposingRules"] != c.rules || r["internetExposed"] != (c.ports != "") {
			t.Errorf("%s: ports %q rules %q exposed %v", r["vm"], r["exposedPorts"], r["exposingRules"], r["internetExposed"])
		}
	}
	if n := in.Sheet(schema.SProject).Rows[0]["exposedVMs"]; n != int64(2) {
		t.Errorf("exposedVMs = %v", n)
	}
	if n := sec(in, 0)["issues"]; n != int64(1) {
		t.Errorf("issues = %v, want 1 (exposed)", n)
	}
}

func TestExposureTieAndAllPorts(t *testing.T) {
	in := inv(t, []model.Row{
		{"project": "p1", "name": "all", "rules": "all"},
		{"project": "p1", "name": "tie-deny", "rules": "tcp:22", "action": "DENY"},
	}, model.Row{"project": "p1", "vm": "a", "ip": "34.0.0.1", "host": "p1"})
	Evaluate(in)
	if p := sec(in, 0)["exposedPorts"]; p == "" || p == "Todos los puertos TCP" {
		t.Errorf("deny should beat allow on a priority tie for 22: %q", p)
	}

	in = inv(t, []model.Row{{"project": "p1", "name": "any", "rules": "tcp:0-65535"}},
		model.Row{"project": "p1", "vm": "a", "ip": "34.0.0.1", "host": "p1"})
	Evaluate(in)
	if p := sec(in, 0)["exposedPorts"]; p != "Todos los puertos TCP" {
		t.Errorf("all ports: %q", p)
	}
}

func TestSharedVPC(t *testing.T) {
	in := inv(t, []model.Row{{"project": "host", "name": "rdp", "rules": "tcp:3389"}},
		model.Row{"project": "p1", "vm": "win", "ip": "34.0.0.1", "host": "host"},
		model.Row{"project": "p1", "vm": "other", "ip": "34.0.0.2", "host": "unscanned"})
	Evaluate(in)
	if r := sec(in, 0); r["exposedPorts"] != "3389 (RDP)" || r["exposingRules"] != "host/rdp" {
		t.Errorf("shared VPC: %v / %v", r["exposedPorts"], r["exposingRules"])
	}
	if r := sec(in, 1); r["internetExposed"] != false || r["notes"] == "" {
		t.Errorf("unknown host project should leave a note: %v", r)
	}
}

func TestIssuesSkipManaged(t *testing.T) {
	r := model.Row{"managedBy": "GKE", "osLogin": false, "ipForward": true, "secureBoot": true, "vtpm": true, "integrityMonitoring": false}
	if n := Issues(r); n != 1 {
		t.Errorf("GKE node issues = %d, want 1 (integrity monitoring)", n)
	}
	r["managedBy"] = ""
	if n := Issues(r); n != 3 {
		t.Errorf("issues = %d, want 3", n)
	}
}

func TestParseRules(t *testing.T) {
	r := rule{protos: parseRules("udp:53; tcp:80,8000-8100")}
	if !r.covers(8080) || r.covers(53) || r.covers(22) || r.everyPort() {
		t.Error("parseRules/covers")
	}
}
