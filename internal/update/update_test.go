package update

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewer(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"0.3.0", "0.2.0", true},
		{"v0.2.1", "0.2.0", true},
		{"1.0.0", "0.9.9", true},
		{"0.2.0", "0.2.0", false},
		{"0.1.9", "0.2.0", false},
		{"0.2.0", "0.2.0-rc1", true},
		{"0.2.0-rc2", "0.2.0", false},
		{"0.10.0", "0.9.0", true},
		{"garbage", "0.2.0", false},
	}
	for _, c := range cases {
		if got := Newer(c.a, c.b); got != c.want {
			t.Errorf("Newer(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestCheck(t *testing.T) {
	var ua string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ua = r.Header.Get("User-Agent")
		w.Write([]byte(`{"tag_name":"v0.3.0","html_url":"https://github.com/x/releases/tag/v0.3.0","body":"Novedades","published_at":"2026-12-01T10:00:00Z"}`))
	}))
	defer srv.Close()
	old := API
	API = srv.URL
	defer func() { API = old }()

	info, err := Check(context.Background(), "0.2.0")
	if err != nil {
		t.Fatal(err)
	}
	if !info.Available || info.Latest != "0.3.0" || info.URL != "https://github.com/x/releases/tag/v0.3.0" {
		t.Errorf("info = %+v", info)
	}
	if ua != "gcetools/0.2.0" {
		t.Errorf("User-Agent = %q", ua)
	}
	if info, _ := Check(context.Background(), "0.3.0"); info.Available {
		t.Error("same version must not be reported as an update")
	}
}

func TestCheckNoReleases(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(404) }))
	defer srv.Close()
	old := API
	API = srv.URL
	defer func() { API = old }()
	if _, err := Check(context.Background(), "0.2.0"); err == nil {
		t.Error("expected an error on 404")
	}
}
