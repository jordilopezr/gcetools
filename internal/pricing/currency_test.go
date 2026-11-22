package pricing

import (
	"context"
	"testing"
	"time"
)

// The currency becomes part of the cache file name: anything but an ISO code
// is rejected before touching the disk or the network.
func TestLoadRejectsBadCurrency(t *testing.T) {
	for _, c := range []string{"X/../../etc", "EURO", "E1R", "../a"} {
		if _, err := Load(context.Background(), c, time.Hour); err == nil {
			t.Errorf("%q accepted", c)
		}
	}
}
