// Package model holds the tabular inventory produced by a scan.
//
// Every RVTools-like tab is a Sheet with an ordered list of typed columns and
// rows keyed by column key. Keeping the model generic lets the GUI grid and the
// XLSX/CSV exporters render any sheet without per-sheet code.
package model

import (
	"sync"
	"time"
)

type ColType string

const (
	TString ColType = "string"
	TInt    ColType = "int"
	TFloat  ColType = "float"
	TBool   ColType = "bool"
	TTime   ColType = "datetime"
	TGB     ColType = "gb"    // float, displayed with 2 decimals
	TMoney  ColType = "money" // float, currency amount (2 decimals)
)

type Column struct {
	Key   string  `json:"key"`
	Title string  `json:"title"`
	Type  ColType `json:"type"`
	Width float64 `json:"width,omitempty"` // XLSX width in characters
}

// Row values must be one of: string, int64, float64, bool, time.Time or nil.
type Row map[string]any

type Sheet struct {
	Name        string   `json:"name"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Columns     []Column `json:"columns"`
	Rows        []Row    `json:"rows"`

	mu sync.Mutex
}

func (s *Sheet) Add(r Row) {
	s.mu.Lock()
	s.Rows = append(s.Rows, r)
	s.mu.Unlock()
}

type ErrorKind string

const (
	ErrPermission  ErrorKind = "permission_denied"
	ErrAPIDisabled ErrorKind = "api_disabled"
	ErrNotFound    ErrorKind = "not_found"
	ErrOther       ErrorKind = "error"
)

type ScanError struct {
	Project   string    `json:"project"`
	Collector string    `json:"collector"`
	Kind      ErrorKind `json:"kind"`
	Message   string    `json:"message"`
}

type Meta struct {
	Tool       string    `json:"tool"`
	Version    string    `json:"version"`
	Identity   string    `json:"identity"`
	AuthMethod string    `json:"authMethod"`
	StartedAt  time.Time `json:"startedAt"`
	FinishedAt time.Time `json:"finishedAt"`
	Projects   int       `json:"projects"`
	ProjectsOK int       `json:"projectsOk"`
	Errors     int       `json:"errors"`
	Cancelled  bool      `json:"cancelled"`
	Currency   string    `json:"currency"` // set when costs were estimated
}

type Inventory struct {
	Meta   Meta        `json:"meta"`
	Sheets []*Sheet    `json:"sheets"`
	Errors []ScanError `json:"errors"`

	mu sync.Mutex
}

func (inv *Inventory) Sheet(name string) *Sheet {
	for _, s := range inv.Sheets {
		if s.Name == name {
			return s
		}
	}
	return nil
}

func (inv *Inventory) AddError(e ScanError) {
	inv.mu.Lock()
	inv.Errors = append(inv.Errors, e)
	inv.mu.Unlock()
}

func (inv *Inventory) ErrorCount() int {
	inv.mu.Lock()
	defer inv.mu.Unlock()
	return len(inv.Errors)
}
