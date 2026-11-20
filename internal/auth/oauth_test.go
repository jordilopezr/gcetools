package auth

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

type memRing struct {
	mu sync.Mutex
	m  map[string]string
}

func (r *memRing) Get(s, u string) (string, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.m[s+"/"+u]
	if !ok {
		return "", keyring.ErrNotFound
	}
	return v, nil
}
func (r *memRing) Set(s, u, v string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.m[s+"/"+u] = v
	return nil
}
func (r *memRing) Delete(s, u string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.m, s+"/"+u)
	return nil
}

func useMemRing(t *testing.T) *memRing {
	r := &memRing{m: map[string]string{}}
	old := ring
	ring = r
	t.Cleanup(func() { ring = old })
	return r
}

func TestParseOAuthClient(t *testing.T) {
	c, err := ParseOAuthClient([]byte(`{"installed":{"client_id":"123.apps.googleusercontent.com","client_secret":"GOCSPX-x","project_id":"org-tools"}}`))
	if err != nil || c.ClientID != "123.apps.googleusercontent.com" || c.ProjectID != "org-tools" {
		t.Fatalf("%+v %v", c, err)
	}
	for in, want := range map[string]string{
		`{"web":{"client_id":"x"}}`:  "Aplicación web",
		`{"type":"service_account"}`: "service account",
		`{"foo":1}`:                  "no es un cliente OAuth",
		`no json`:                    "JSON válido",
	} {
		if _, err := ParseOAuthClient([]byte(in)); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("%s: err = %v, want %q", in, err, want)
		}
	}
}

func TestLoginFlow(t *testing.T) {
	r := useMemRing(t)
	if st := GetOAuthStatus(); !st.Available || st.Client != "" {
		t.Fatalf("empty status = %+v", st)
	}
	if _, err := Login(context.Background(), func(string) {}); err == nil {
		t.Fatal("login without client must fail")
	}
	idTok := "x." + base64.RawURLEncoding.EncodeToString([]byte(`{"email":"ana@example.com"}`)) + ".y"
	var gotVerifier, gotRedirect string
	tokenSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		gotVerifier, gotRedirect = r.Form.Get("code_verifier"), r.Form.Get("redirect_uri")
		if r.Form.Get("code") != "the-code" {
			w.WriteHeader(400)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"access_token":"at","refresh_token":"rt-1","expires_in":3600,"token_type":"Bearer","id_token":"` + idTok + `"}`))
	}))
	defer tokenSrv.Close()
	old := oauthEndpoint
	oauthEndpoint = oauth2.Endpoint{AuthURL: "https://accounts.example/auth", TokenURL: tokenSrv.URL}
	defer func() { oauthEndpoint = old }()

	if err := SetOAuthClient(OAuthClient{ClientID: "cid", ClientSecret: "sec", ProjectID: "org-tools"}); err != nil {
		t.Fatal(err)
	}
	// the "browser": follow the consent URL straight to the loopback redirect
	browser := func(consent string) {
		u, _ := url.Parse(consent)
		q := u.Query()
		if q.Get("code_challenge_method") != "S256" || q.Get("access_type") != "offline" ||
			!strings.Contains(q.Get("scope"), "cloud-platform.read-only") || strings.Contains(q.Get("scope"), "auth/cloud-platform ") {
			t.Errorf("consent URL = %s", consent)
		}
		go func() {
			bad, _ := http.Get(q.Get("redirect_uri") + "?state=forged&code=evil")
			if bad != nil && bad.StatusCode != 400 {
				t.Errorf("forged state accepted: %d", bad.StatusCode)
			}
			http.Get(q.Get("redirect_uri") + "?state=" + q.Get("state") + "&code=the-code")
		}()
	}
	account, err := Login(context.Background(), browser)
	if err != nil {
		t.Fatal(err)
	}
	if account != "ana@example.com" || gotVerifier == "" || !strings.HasPrefix(gotRedirect, "http://127.0.0.1:") {
		t.Errorf("account=%q verifier=%q redirect=%q", account, gotVerifier, gotRedirect)
	}
	st := GetOAuthStatus()
	if st.Account != "ana@example.com" || st.Client != "cid" || st.ProjectID != "org-tools" {
		t.Errorf("status = %+v", st)
	}
	if !strings.Contains(r.m["gcetools/google-oauth"], `"refreshToken":"rt-1"`) {
		t.Errorf("refresh token not in keyring: %s", r.m["gcetools/google-oauth"])
	}
	// changing the client signs out; forgetting keeps nothing
	SetOAuthClient(OAuthClient{ClientID: "other"})
	if st := GetOAuthStatus(); st.Account != "" || st.Client != "other" {
		t.Errorf("after client change = %+v", st)
	}
	ForgetOAuth(true)
	if len(r.m) != 0 {
		t.Errorf("keyring not empty: %v", r.m)
	}
}

func TestLoginCancelled(t *testing.T) {
	useMemRing(t)
	SetOAuthClient(OAuthClient{ClientID: "cid"})
	ctx, cancel := context.WithCancel(context.Background())
	_, err := Login(ctx, func(string) { cancel() })
	if err == nil || !strings.Contains(err.Error(), "cancelado") {
		t.Errorf("err = %v", err)
	}
}
