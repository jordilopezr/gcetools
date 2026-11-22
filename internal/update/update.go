// Package update holds the product's public links and checks GitHub for a
// newer release. The check sends no user data: one anonymous GET to the
// GitHub REST API with a generic User-Agent.
package update

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gcetools/internal/i18n"
)

const (
	Repo       = "jordilopezr/gcetools"
	Website    = "https://gcetools.jordilopezr.com"
	Donate     = "https://buymeacoffee.com/jordimlopezr"
	License    = "GPL-3.0"
	LicenseURL = "https://www.gnu.org/licenses/gpl-3.0.html"
	ReleaseURL = "https://github.com/" + Repo + "/releases"
)

// API is the endpoint queried for the latest release (overridable in tests).
var API = "https://api.github.com/repos/" + Repo + "/releases/latest"

type Info struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
	Notes     string `json:"notes"`
	Published string `json:"published"`
}

// Check asks GitHub for the latest published (non-draft, non-prerelease)
// release and compares it with current.
func Check(ctx context.Context, current string) (Info, error) {
	info := Info{Current: current, URL: ReleaseURL}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, API, nil)
	if err != nil {
		return info, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	req.Header.Set("User-Agent", "gcetools/"+current)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return info, fmt.Errorf(i18n.T("no se pudo consultar GitHub: %w"), err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return info, errors.New(i18n.T("no hay releases publicadas todavía (o el repositorio no es público)"))
	case http.StatusForbidden, http.StatusTooManyRequests:
		return info, errors.New(i18n.T("GitHub limitó las consultas; inténtalo más tarde"))
	default:
		return info, fmt.Errorf(i18n.T("GitHub respondió %s"), resp.Status)
	}
	var rel struct {
		TagName     string `json:"tag_name"`
		HTMLURL     string `json:"html_url"`
		Body        string `json:"body"`
		PublishedAt string `json:"published_at"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return info, err
	}
	info.Latest = strings.TrimPrefix(rel.TagName, "v")
	// The UI opens this URL: accept only release pages of this repository.
	if strings.HasPrefix(rel.HTMLURL, ReleaseURL+"/") {
		info.URL = rel.HTMLURL
	}
	info.Published = rel.PublishedAt
	notes := rel.Body
	if r := []rune(notes); len(r) > 2000 {
		notes = string(r[:2000]) + "…"
	}
	info.Notes = notes
	info.Available = Newer(info.Latest, current)
	return info, nil
}

// Newer reports whether version a is strictly newer than b (semver
// major.minor.patch; pre-release suffixes sort before the release).
func Newer(a, b string) bool {
	pa, preA, okA := parse(a)
	pb, preB, okB := parse(b)
	if !okA || !okB {
		return false
	}
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	// 1.2.0 > 1.2.0-rc1
	return preA == "" && preB != ""
}

func parse(v string) ([3]int, string, bool) {
	var out [3]int
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre, v = v[i+1:], v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) != 3 {
		return out, "", false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return out, "", false
		}
		out[i] = n
	}
	return out, pre, true
}
