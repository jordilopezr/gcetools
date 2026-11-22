package model

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNormalizeRoundTrip(t *testing.T) {
	when := time.Date(2026, 11, 3, 10, 30, 0, 0, time.UTC)
	inv := &Inventory{Sheets: []*Sheet{{
		Name:    "s",
		Columns: []Column{{Key: "t", Type: TTime}, {Key: "n", Type: TInt}, {Key: "f", Type: TFloat}, {Key: "e", Type: TTime}},
		Rows:    []Row{{"t": when, "n": int64(42), "f": 1.5}},
	}}}
	data, err := json.Marshal(inv)
	if err != nil {
		t.Fatal(err)
	}
	var back Inventory
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatal(err)
	}
	back.Sheets[0].Rows[0]["e"] = ""
	Normalize(&back)
	r := back.Sheets[0].Rows[0]
	if got, ok := r["t"].(time.Time); !ok || !got.Equal(when) {
		t.Errorf("t = %#v", r["t"])
	}
	if got, ok := r["n"].(int64); !ok || got != 42 {
		t.Errorf("n = %#v", r["n"])
	}
	if got, ok := r["f"].(float64); !ok || got != 1.5 {
		t.Errorf("f = %#v", r["f"])
	}
	if _, ok := r["e"]; ok {
		t.Errorf("empty date kept: %#v", r["e"])
	}
}
