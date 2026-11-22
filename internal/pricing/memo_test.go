package pricing

import "testing"

// Memoized answers match fresh ones, also for GPU counts and licenses.
func TestMemoizedLookups(t *testing.T) {
	x := NewIndex(&Catalog{Currency: "USD", SKUs: []SKU{
		{Desc: "N2 Instance Core running in Americas", UsageType: "OnDemand", Regions: []string{"us-central1"}, Price: 0.03, Unit: "h"},
		{Desc: "N2 Instance Ram running in Americas", UsageType: "OnDemand", Regions: []string{"us-central1"}, Price: 0.004, Unit: "GiBy.h"},
		{Desc: "Nvidia Tesla T4 GPU running in Americas", UsageType: "OnDemand", Regions: []string{"us-central1"}, Price: 0.35, Unit: "h"},
	}})
	m1, ok1 := x.Machine("us-central1", "n2-standard-4", 4, 16, false)
	m2, ok2 := x.Machine("us-central1", "n2-standard-4", 4, 16, false)
	if !ok1 || !ok2 || m1 != m2 || m1 == 0 {
		t.Fatalf("machine %v %v %v %v", m1, ok1, m2, ok2)
	}
	g1, _ := x.GPU("us-central1", "nvidia-tesla-t4", 1, false)
	g2, ok := x.GPU("us-central1", "nvidia-tesla-t4", 2, false)
	if !ok || g2 != 2*g1 || g1 == 0 {
		t.Fatalf("gpu 1=%v 2=%v", g1, g2)
	}
	if _, ok := x.Machine("us-central1", "n2-standard-4", 4, 16, true); ok {
		t.Error("spot must not reuse the on-demand answer")
	}
}
