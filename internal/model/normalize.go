package model

import (
	"math"
	"time"
)

// Normalize restores the Go types of row values after a JSON round trip
// (history, snapshots): dates come back as RFC 3339 strings and every number
// as float64. Values that do not match their column type are left as they are.
func Normalize(inv *Inventory) {
	if inv == nil {
		return
	}
	for _, s := range inv.Sheets {
		for _, c := range s.Columns {
			switch c.Type {
			case TTime:
				for _, r := range s.Rows {
					if v, ok := r[c.Key].(string); ok {
						if v == "" {
							delete(r, c.Key)
						} else if t, err := time.Parse(time.RFC3339Nano, v); err == nil {
							r[c.Key] = t
						}
					}
				}
			case TInt:
				for _, r := range s.Rows {
					if v, ok := r[c.Key].(float64); ok && v == math.Trunc(v) && math.Abs(v) < 1<<53 {
						r[c.Key] = int64(v)
					}
				}
			}
		}
	}
}
