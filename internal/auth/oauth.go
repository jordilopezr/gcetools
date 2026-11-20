package auth

// Sign-in with Google without gcloud: the OAuth 2.0 flow for installed apps
// (loopback redirect + PKCE) with an OAuth client created by the user's own
// organization. An "Internal" client needs no Google verification and keeps
// the consent screen inside the organization.
//
// Nothing is written to disk: the client and the refresh token live in the
// operating system keyring (Windows Credential Manager, macOS Keychain,
// Secret Service on Linux).

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

// OAuth is the sign-in-with-Google method.
const OAuth Method = "oauth"

const (
	keyringService = "gcetools"
	keyringUser    = "google-oauth"
)

// OAuthClient is the organization's "Desktop app" OAuth client.
type OAuthClient struct {
	ClientID     string `json:"clientId"`
	ClientSecret string `json:"clientSecret"` // not confidential for desktop clients, kept in the keyring anyway
	ProjectID    string `json:"projectId"`
}

type stored struct {
	Client       OAuthClient `json:"client"`
	RefreshToken string      `json:"refreshToken,omitempty"`
	Account      string      `json:"account,omitempty"`
}

// OAuthStatus is what the connection page shows.
type OAuthStatus struct {
	Available bool   `json:"available"` // the OS keyring works
	Client    string `json:"client"`    // client ID ("" = none configured)
	ProjectID string `json:"projectId"`
	Account   string `json:"account"` // signed-in account ("" = not signed in)
	Error     string `json:"error,omitempty"`
}

// keyring and endpoint are variables so tests can replace them.
var (
	ring interface {
		Get(service, user string) (string, error)
		Set(service, user, secret string) error
		Delete(service, user string) error
	} = osKeyring{}
	oauthEndpoint = google.Endpoint
)

type osKeyring struct{}

func (osKeyring) Get(s, u string) (string, error) { return keyring.Get(s, u) }
func (osKeyring) Set(s, u, v string) error        { return keyring.Set(s, u, v) }
func (osKeyring) Delete(s, u string) error        { return keyring.Delete(s, u) }

// ParseOAuthClient reads the JSON downloaded from the console
// (APIs & Services → Credentials → OAuth client ID → Desktop app).
func ParseOAuthClient(data []byte) (OAuthClient, error) {
	var f struct {
		Installed *struct {
			ClientID     string `json:"client_id"`
			ClientSecret string `json:"client_secret"`
			ProjectID    string `json:"project_id"`
		} `json:"installed"`
		Web  json.RawMessage `json:"web"`
		Type string          `json:"type"`
	}
	if err := json.Unmarshal(data, &f); err != nil {
		return OAuthClient{}, fmt.Errorf("el archivo no es un JSON válido: %w", err)
	}
	switch {
	case f.Installed != nil && f.Installed.ClientID != "":
		return OAuthClient{ClientID: f.Installed.ClientID, ClientSecret: f.Installed.ClientSecret, ProjectID: f.Installed.ProjectID}, nil
	case f.Web != nil:
		return OAuthClient{}, errors.New("es un cliente de tipo \"Aplicación web\": crea uno de tipo \"App de escritorio\"")
	case f.Type == "service_account":
		return OAuthClient{}, errors.New("es una clave de service account: úsala en la opción \"Service account (JSON)\"")
	}
	return OAuthClient{}, errors.New("no es un cliente OAuth de Google (falta la sección \"installed\")")
}

func load() (stored, error) {
	v, err := ring.Get(keyringService, keyringUser)
	if err != nil {
		return stored{}, err
	}
	var s stored
	if err := json.Unmarshal([]byte(v), &s); err != nil {
		return stored{}, err
	}
	return s, nil
}

func save(s stored) error {
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	if err := ring.Set(keyringService, keyringUser, string(b)); err != nil {
		return fmt.Errorf("no se pudo guardar en el llavero del sistema: %w", err)
	}
	return nil
}

// GetOAuthStatus never fails: problems are reported in Error.
func GetOAuthStatus() OAuthStatus {
	s, err := load()
	switch {
	case err == nil:
		return OAuthStatus{Available: true, Client: s.Client.ClientID, ProjectID: s.Client.ProjectID, Account: s.Account}
	case errors.Is(err, keyring.ErrNotFound):
		return OAuthStatus{Available: true}
	default:
		return OAuthStatus{Error: "Llavero del sistema no disponible: " + err.Error()}
	}
}

// SetOAuthClient stores the organization's client. Changing the client signs out.
func SetOAuthClient(c OAuthClient) error {
	s, err := load()
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("llavero del sistema no disponible: %w", err)
	}
	if s.Client.ClientID != c.ClientID {
		s = stored{}
	}
	s.Client = c
	return save(s)
}

// ForgetOAuth signs out; with all=true it also removes the client.
func ForgetOAuth(all bool) error {
	s, err := load()
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if all {
		return ring.Delete(keyringService, keyringUser)
	}
	return save(stored{Client: s.Client})
}

func oauthConfig(c OAuthClient, redirect string) *oauth2.Config {
	return &oauth2.Config{ClientID: c.ClientID, ClientSecret: c.ClientSecret, Endpoint: oauthEndpoint,
		RedirectURL: redirect, Scopes: []string{Scope, "openid", "email"}}
}

// Login runs the browser flow and stores the refresh token. open receives
// the consent URL (the app opens the browser; the CLI also prints it).
func Login(ctx context.Context, open func(url string)) (account string, err error) {
	s, err := load()
	if errors.Is(err, keyring.ErrNotFound) || (err == nil && s.Client.ClientID == "") {
		return "", errors.New("primero configura el cliente OAuth de tu organización")
	}
	if err != nil {
		return "", fmt.Errorf("llavero del sistema no disponible: %w", err)
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", err
	}
	defer ln.Close()
	redirect := fmt.Sprintf("http://127.0.0.1:%d/", ln.Addr().(*net.TCPAddr).Port)
	cfg := oauthConfig(s.Client, redirect)
	verifier := oauth2.GenerateVerifier()
	state := randomHex(16)

	type result struct{ code, err string }
	done := make(chan result, 1)
	srv := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		if q.Get("state") != state {
			http.Error(w, "estado inválido", http.StatusBadRequest)
			return
		}
		res := result{code: q.Get("code"), err: q.Get("error")}
		msg := "Sesión iniciada. Ya puedes cerrar esta pestaña y volver a gceTools."
		if res.err != "" || res.code == "" {
			msg = "No se completó el inicio de sesión (" + html.EscapeString(res.err) + "). Vuelve a gceTools e inténtalo de nuevo."
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		fmt.Fprintf(w, `<!doctype html><meta charset="utf-8"><title>gceTools</title><body style="font:16px system-ui;margin:48px;max-width:560px"><h2>gceTools</h2><p>%s</p></body>`, msg)
		select {
		case done <- res:
		default:
		}
	})}
	go srv.Serve(ln)
	defer srv.Close()

	open(cfg.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.S256ChallengeOption(verifier),
		oauth2.SetAuthURLParam("prompt", "consent select_account")))

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	var res result
	select {
	case res = <-done:
	case <-ctx.Done():
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return "", errors.New("se agotó el tiempo de espera del inicio de sesión (5 minutos)")
		}
		return "", errors.New("inicio de sesión cancelado")
	}
	if res.err != "" {
		if res.err == "access_denied" {
			return "", errors.New("se denegó el acceso en la pantalla de consentimiento")
		}
		return "", fmt.Errorf("Google devolvió el error %q", res.err)
	}
	tok, err := cfg.Exchange(ctx, res.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return "", fmt.Errorf("no se pudo completar el inicio de sesión: %w", err)
	}
	if tok.RefreshToken == "" {
		return "", errors.New("Google no devolvió un token de refresco; revoca el acceso de la app en tu cuenta y vuelve a intentarlo")
	}
	account = emailFromIDToken(tok)
	if account == "" {
		account = emailFromTokenInfo(ctx, tok.AccessToken)
	}
	s.RefreshToken, s.Account = tok.RefreshToken, account
	return account, save(s)
}

// oauthSession builds the token source from the stored refresh token.
func oauthSession(ctx context.Context) (oauth2.TokenSource, string, string, error) {
	s, err := load()
	if err != nil || s.RefreshToken == "" {
		return nil, "", "", errors.New("no has iniciado sesión con Google en gceTools")
	}
	ts := oauthConfig(s.Client, "").TokenSource(context.WithoutCancel(ctx), &oauth2.Token{RefreshToken: s.RefreshToken})
	return ts, s.Account, s.Client.ProjectID, nil
}

func emailFromIDToken(tok *oauth2.Token) string {
	raw, _ := tok.Extra("id_token").(string)
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return ""
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return ""
	}
	var c struct {
		Email string `json:"email"`
	}
	if json.Unmarshal(payload, &c) != nil {
		return ""
	}
	return c.Email
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}
