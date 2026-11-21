// Package profile stores named scan configurations (which projects, which
// modules, how to authenticate) so a recurring inventory is one click or one
// flag away. Profiles never contain secrets: a service account key is
// referenced by path and read only when the scan starts.
package profile

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gcetools/internal/core"
	"gcetools/internal/health"
	"gcetools/internal/i18n"
)

type Profile struct {
	Name string `json:"name"`

	// Authentication (used by the CLI; the app authenticates on its own page).
	Auth        string `json:"auth,omitempty"`        // adc | sa | impersonate
	KeyPath     string `json:"keyPath,omitempty"`     // path to the SA key, never its content
	Impersonate string `json:"impersonate,omitempty"` // service account email

	// Which projects.
	Query    string   `json:"query,omitempty"`    // Resource Manager filter
	Projects []string `json:"projects,omitempty"` // explicit IDs; empty = every match
	Discover string   `json:"discover,omitempty"` // CAI scope: only projects with Compute resources

	// How to scan.
	Concurrency int            `json:"concurrency,omitempty"`
	Modules     core.Modules   `json:"modules"`
	Health      health.Options `json:"health"`

	Updated time.Time `json:"updated"`
}

type file struct {
	Version  int       `json:"version"`
	Profiles []Profile `json:"profiles"`
}

// Path is where profiles live: <user config dir>/gcetools/profiles.json
// (~/.config on Linux, %AppData% on Windows, ~/Library/Application Support
// on macOS). Overridable for tests.
var Path = func() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "gcetools", "profiles.json"), nil
}

// List returns all profiles sorted by name. A missing file is not an error.
func List() ([]Profile, error) {
	p, err := Path()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return []Profile{}, nil
	}
	if err != nil {
		return nil, err
	}
	var f file
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, fmt.Errorf(i18n.T("%s está dañado: %w"), p, err)
	}
	if f.Profiles == nil {
		f.Profiles = []Profile{}
	}
	slices.SortFunc(f.Profiles, func(a, b Profile) int { return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)) })
	return f.Profiles, nil
}

// Get returns the profile with that name (case-insensitive).
func Get(name string) (Profile, error) {
	all, err := List()
	if err != nil {
		return Profile{}, err
	}
	for _, p := range all {
		if strings.EqualFold(p.Name, name) {
			return p, nil
		}
	}
	names := make([]string, len(all))
	for i, p := range all {
		names[i] = p.Name
	}
	if len(names) == 0 {
		return Profile{}, fmt.Errorf(i18n.T("no existe el perfil %q (no hay perfiles guardados)"), name)
	}
	return Profile{}, fmt.Errorf(i18n.T("no existe el perfil %q (disponibles: %s)"), name, strings.Join(names, ", "))
}

// Save creates or replaces the profile with the same name.
func Save(p Profile) error {
	p.Name = strings.TrimSpace(p.Name)
	if p.Name == "" {
		return errors.New(i18n.T("el perfil necesita un nombre"))
	}
	if len(p.Name) > 60 {
		return errors.New(i18n.T("el nombre del perfil es demasiado largo (máximo 60 caracteres)"))
	}
	if p.Discover != "" && !core.ValidScope(p.Discover) {
		return fmt.Errorf(i18n.T("ámbito de descubrimiento no válido %q"), p.Discover)
	}
	p.Updated = time.Now()
	all, err := List()
	if err != nil {
		return err
	}
	all = slices.DeleteFunc(all, func(x Profile) bool { return strings.EqualFold(x.Name, p.Name) })
	return write(append(all, p))
}

// Delete removes a profile; deleting a missing one is not an error.
func Delete(name string) error {
	all, err := List()
	if err != nil {
		return err
	}
	return write(slices.DeleteFunc(all, func(x Profile) bool { return strings.EqualFold(x.Name, name) }))
}

func write(all []Profile) error {
	p, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(file{Version: 1, Profiles: all}, "", "  ")
	if err != nil {
		return err
	}
	// write-then-rename so a crash never leaves a half-written file
	tmp := p + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p)
}
