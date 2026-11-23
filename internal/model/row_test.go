package model

import (
	"testing"
	"time"
)

func TestAccessors(t *testing.T) {
	when := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	r := Row{"s": "x", "b": true, "i": int64(7), "f": 2.6, "n": 3, "t": when, "ts": "2026-01-02T03:04:05Z", "bad": "nope"}
	if r.Str("s") != "x" || r.Str("i") != "" || !r.Bool("b") || r.Bool("s") {
		t.Error("str/bool")
	}
	if r.Int("i") != 7 || r.Int("f") != 3 || r.Int("n") != 3 || r.Int("s") != 0 {
		t.Error("int")
	}
	if r.Float("i") != 7 || r.Float("f") != 2.6 || r.Float("n") != 3 || r.Float("missing") != 0 {
		t.Error("float")
	}
	if v, ok := r.Time("t"); !ok || !v.Equal(when) {
		t.Error("time")
	}
	if v, ok := r.Time("ts"); !ok || !v.Equal(when) {
		t.Error("time from JSON")
	}
	if _, ok := r.Time("bad"); ok {
		t.Error("unparsable time")
	}
	inv := &Inventory{Sheets: []*Sheet{{Name: "a", Rows: []Row{r}}}}
	if len(inv.Rows("a")) != 1 || inv.Rows("b") != nil {
		t.Error("rows")
	}
}
