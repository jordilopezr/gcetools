// Package i18n translates gceTools' user-facing text. Spanish is the source
// language: the Spanish text itself is the key, so code stays readable and a
// missing translation falls back to Spanish instead of showing an ID.
//
//	i18n.T("Proyecto")                       // "Project" in English
//	i18n.Tf("%d proyectos leídos", n)        // the format string is the key
//
// The language is process-wide: the app sets it from the UI, the CLI from
// --lang or the system locale. Data produced by a scan (column titles, rule
// texts) is written in the language active during the scan; Relabel brings
// the schema texts of an older inventory to the current language.
package i18n

import (
	"fmt"
	"os"
	"strings"
	"sync/atomic"
)

const (
	ES = "es"
	EN = "en"
)

var lang atomic.Value

func init() { lang.Store(ES) }

// Lang returns the active language ("es" or "en").
func Lang() string { return lang.Load().(string) }

// SetLang switches the language; anything but English selects Spanish.
func SetLang(l string) {
	if Normalize(l) == EN {
		lang.Store(EN)
	} else {
		lang.Store(ES)
	}
}

// Normalize maps a locale ("en_US.UTF-8", "en-GB", "es-CL") to "en" or "es".
// Unknown locales get English: the project is public and English is the
// common ground.
func Normalize(locale string) string {
	l := strings.ToLower(strings.TrimSpace(locale))
	if l == "" || l == "c" || l == "posix" {
		return ""
	}
	if strings.HasPrefix(l, "es") {
		return ES
	}
	return EN
}

// FromEnv returns the language of the system locale (LC_ALL, LC_MESSAGES,
// LANG), or Spanish when it is not set.
func FromEnv() string {
	for _, k := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if l := Normalize(os.Getenv(k)); l != "" {
			return l
		}
	}
	return ES
}

// T translates s into the active language.
func T(s string) string {
	if Lang() == EN {
		if t, ok := en[s]; ok {
			return t
		}
	}
	return s
}

// Tf translates the format string and formats it.
func Tf(format string, args ...any) string {
	return fmt.Sprintf(T(format), args...)
}

// en holds every translation. Several files fill it (en_*.go) so the
// dictionary can be split by area.
var en = map[string]string{}

// conflicts records keys added twice with different translations
// (dictionary files are split by area; TestCoverage reports them).
var conflicts []string

func add(m map[string]string) {
	for k, v := range m {
		if old, ok := en[k]; ok && old != v {
			conflicts = append(conflicts, fmt.Sprintf("%q: %q / %q", k, old, v))
		}
		en[k] = v
	}
}

// Has reports whether s has an English translation (template coverage tests).
func Has(s string) bool {
	_, ok := en[s]
	return ok
}
