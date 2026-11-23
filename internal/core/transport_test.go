package core

import "testing"

// Each API host gets its own limiter; Compute gets three times the rate.
func TestHostLimiters(t *testing.T) {
	h := newHostLimiters(10)
	c, m := h.get("compute.googleapis.com"), h.get("monitoring.googleapis.com")
	if c == m || h.get("compute.googleapis.com") != c {
		t.Fatal("limiters must be per host and reused")
	}
	if c.Limit() != 30 || m.Limit() != 10 {
		t.Errorf("limits = %v, %v", c.Limit(), m.Limit())
	}
}
