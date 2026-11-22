package security

import "testing"

func TestOpensAdminOrAllPorts(t *testing.T) {
	for rules, want := range map[string]bool{
		"tcp:80,443,22": true, "tcp:20-30": true, "tcp:0-65535": true, "all": true, "tcp": true, "6:3389": true,
		"tcp:2222": false, "udp:3389": false, "tcp:443; udp:22": false, "": false,
	} {
		if got := OpensAdminOrAllPorts(rules); got != want {
			t.Errorf("%q = %v, want %v", rules, got, want)
		}
	}
}
