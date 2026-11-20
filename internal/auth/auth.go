// Package auth builds a read-only Google Cloud token source from one of four
// methods: Application Default Credentials (gcloud user login or a key pointed
// to by GOOGLE_APPLICATION_CREDENTIALS), a service account JSON key,
// impersonation of a service account by the ADC identity, or sign-in with
// Google through the organization's own OAuth client (oauth.go).
package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/impersonate"
	"google.golang.org/api/option"
)

// Scope is the only scope ever requested: the tool never writes anything.
const Scope = "https://www.googleapis.com/auth/cloud-platform.read-only"

type Method string

const (
	ADC         Method = "adc"
	KeyFile     Method = "sa"
	Impersonate Method = "impersonate"
)

type Config struct {
	Method   Method
	KeyJSON  []byte // KeyFile only; kept in memory, never written to disk
	TargetSA string // Impersonate only
}

type Session struct {
	TokenSource oauth2.TokenSource
	Method      Method
	Identity    string
	ProjectHint string // project_id from the key / ADC, if any
	// QuotaProject is the ADC quota_project_id. Some APIs (Recommender) reject
	// user credentials unless requests name a project to bill quota to.
	QuotaProject string
}

// KeyInfo is the non-secret part of a service account key.
type KeyInfo struct {
	ClientEmail string `json:"clientEmail"`
	ProjectID   string `json:"projectId"`
	KeyID       string `json:"keyId"`
}

// ParseKey validates that data is a service account key and returns its
// public identity. The private key is not inspected or returned.
func ParseKey(data []byte) (KeyInfo, error) {
	var k struct {
		Type         string `json:"type"`
		ClientEmail  string `json:"client_email"`
		ProjectID    string `json:"project_id"`
		PrivateKeyID string `json:"private_key_id"`
		PrivateKey   string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &k); err != nil {
		return KeyInfo{}, fmt.Errorf("el archivo no es un JSON válido: %w", err)
	}
	if k.Type != "service_account" {
		return KeyInfo{}, fmt.Errorf("tipo de credencial %q no soportado: se esperaba \"service_account\"", k.Type)
	}
	if k.ClientEmail == "" || k.PrivateKey == "" {
		return KeyInfo{}, errors.New("la clave no contiene client_email o private_key")
	}
	return KeyInfo{ClientEmail: k.ClientEmail, ProjectID: k.ProjectID, KeyID: k.PrivateKeyID}, nil
}

func Connect(ctx context.Context, cfg Config) (*Session, error) {
	s := &Session{Method: cfg.Method}
	switch cfg.Method {
	case ADC:
		creds, err := google.FindDefaultCredentials(ctx, Scope)
		if err != nil {
			return nil, fmt.Errorf("no se encontraron credenciales ADC (ejecuta `gcloud auth application-default login`): %w", err)
		}
		s.TokenSource = creds.TokenSource
		s.ProjectHint = creds.ProjectID
		s.QuotaProject = quotaProjectFromJSON(creds.JSON)
		s.Identity = identityFromCredsJSON(creds.JSON)

	case KeyFile:
		info, err := ParseKey(cfg.KeyJSON)
		if err != nil {
			return nil, err
		}
		creds, err := google.CredentialsFromJSONWithType(ctx, cfg.KeyJSON, google.ServiceAccount, Scope)
		if err != nil {
			return nil, fmt.Errorf("clave de service account inválida: %w", err)
		}
		s.TokenSource = creds.TokenSource
		s.Identity = info.ClientEmail
		s.ProjectHint = info.ProjectID

	case Impersonate:
		target := strings.TrimSpace(cfg.TargetSA)
		if !strings.Contains(target, "@") {
			return nil, errors.New("indica el email de la service account a impersonar")
		}
		base, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
		if err != nil {
			return nil, fmt.Errorf("la impersonación necesita credenciales ADC: %w", err)
		}
		ts, err := impersonate.CredentialsTokenSource(ctx, impersonate.CredentialsConfig{
			TargetPrincipal: target,
			Scopes:          []string{Scope},
		}, option.WithCredentials(base))
		if err != nil {
			return nil, fmt.Errorf("no se pudo impersonar %s: %w", target, err)
		}
		s.TokenSource = ts
		s.Identity = target

	case OAuth:
		// Quota for user tokens of an OAuth client goes to the client's
		// project, so no quota project header is needed.
		ts, account, project, err := oauthSession(ctx)
		if err != nil {
			return nil, err
		}
		s.TokenSource, s.Identity, s.ProjectHint = ts, account, project

	default:
		return nil, fmt.Errorf("método de autenticación desconocido: %q", cfg.Method)
	}

	s.TokenSource = oauth2.ReuseTokenSource(nil, s.TokenSource)
	tok, err := s.TokenSource.Token()
	if err != nil {
		if cfg.Method == OAuth && strings.Contains(err.Error(), "invalid_grant") {
			return nil, errors.New("la sesión de Google ha caducado o se revocó el acceso: vuelve a iniciar sesión en Conexión")
		}
		if cfg.Method != KeyFile && (strings.Contains(err.Error(), "invalid_rapt") || strings.Contains(err.Error(), "invalid_grant")) {
			return nil, errors.New("la sesión de gcloud ha caducado o tu organización exige volver a autenticarse. " +
				"Ejecuta `gcloud auth application-default login` y vuelve a conectar")
		}
		return nil, fmt.Errorf("no se pudo obtener un token: %w", err)
	}
	if s.Identity == "" {
		s.Identity = emailFromTokenInfo(ctx, tok.AccessToken)
	}
	if s.Identity == "" {
		s.Identity = "Usuario (ADC)"
	}
	return s, nil
}

// DetectADC reports whether ADC is available without fetching a token.
func DetectADC(ctx context.Context) (found bool, identity string) {
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	creds, err := google.FindDefaultCredentials(ctx, Scope)
	if err != nil {
		return false, ""
	}
	id := identityFromCredsJSON(creds.JSON)
	if id == "" {
		id = "Usuario de gcloud"
	}
	return true, id
}

func identityFromCredsJSON(data []byte) string {
	if len(data) == 0 {
		return "Cuenta de servicio del entorno (metadata server)"
	}
	var c struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
		ImpURL      string `json:"service_account_impersonation_url"`
	}
	if json.Unmarshal(data, &c) != nil {
		return ""
	}
	if c.ClientEmail != "" {
		return c.ClientEmail
	}
	if c.ImpURL != "" {
		// .../serviceAccounts/sa@proj.iam.gserviceaccount.com:generateAccessToken
		p := c.ImpURL[strings.LastIndex(c.ImpURL, "/")+1:]
		return strings.TrimSuffix(p, ":generateAccessToken")
	}
	return ""
}

// emailFromTokenInfo asks Google's tokeninfo endpoint for the token's email.
// The token travels in the POST body, never in the URL. Best effort.
func emailFromTokenInfo(ctx context.Context, accessToken string) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	form := url.Values{"access_token": {accessToken}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		"https://oauth2.googleapis.com/tokeninfo", strings.NewReader(form.Encode()))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var ti struct {
		Email string `json:"email"`
	}
	if resp.StatusCode != http.StatusOK || json.NewDecoder(resp.Body).Decode(&ti) != nil {
		return ""
	}
	return ti.Email
}

func quotaProjectFromJSON(data []byte) string {
	var c struct {
		QuotaProject string `json:"quota_project_id"`
	}
	if len(data) == 0 || json.Unmarshal(data, &c) != nil {
		return ""
	}
	return c.QuotaProject
}
