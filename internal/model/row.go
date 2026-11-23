package model

import (
	"math"
	"time"
)

// Typed accessors for row values. They accept the types a value can have in
// memory (int64, float64, time.Time) and after a JSON round trip (float64,
// RFC 3339 strings), and return the zero value when the key is missing or
// holds another type.

// Str returns r[k] as a string.
func (r Row) Str(k string) string {
	s, _ := r[k].(string)
	return s
}

// Bool returns r[k] as a bool.
func (r Row) Bool(k string) bool {
	b, _ := r[k].(bool)
	return b
}

// Float returns r[k] as a float64.
func (r Row) Float(k string) float64 { return Num(r[k]) }

// Int returns r[k] as an int64 (floats are rounded).
func (r Row) Int(k string) int64 {
	switch v := r[k].(type) {
	case int64:
		return v
	case int:
		return int64(v)
	case float64:
		return int64(math.Round(v))
	}
	return 0
}

// Time returns r[k] as a time; ok is false when it is missing or unparsable.
func (r Row) Time(k string) (t time.Time, ok bool) {
	switch v := r[k].(type) {
	case time.Time:
		return v, !v.IsZero()
	case string:
		t, err := time.Parse(time.RFC3339Nano, v)
		return t, err == nil
	}
	return time.Time{}, false
}

// Num converts a numeric row value to float64 (0 for anything else).
func Num(v any) float64 {
	switch x := v.(type) {
	case float64:
		return x
	case int64:
		return float64(x)
	case int:
		return float64(x)
	}
	return 0
}

// Rows returns the rows of a sheet, or nil when the sheet does not exist.
func (inv *Inventory) Rows(name string) []Row {
	if s := inv.Sheet(name); s != nil {
		return s.Rows
	}
	return nil
}
