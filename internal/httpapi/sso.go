package httpapi

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/sso"
)

// SSOAdminService manages SSO configurations per domain.
type SSOAdminService interface {
	GetSSOConfig(ctx context.Context, domainID string) (maildb.SSOConfig, error)
	UpsertSSOConfig(ctx context.Context, cfg maildb.SSOConfig) error
	DeleteSSOConfig(ctx context.Context, domainID string) error
}

// SSOFlowService provides the SSO initiation and assertion handling.
type SSOFlowService interface {
	GetSSOConfig(ctx context.Context, domainID string) (maildb.SSOConfig, error)
	GetUserByEmail(ctx context.Context, email string) (maildb.SSOUserInfo, error)
	JITCreateSSOUser(ctx context.Context, email, domainID, displayName string) (maildb.SSOUserInfo, error)
}

// ssoTokenResponse is the JSON body returned after a successful SSO login.
type ssoTokenResponse struct {
	Token     string `json:"token"`
	ExpiresIn int    `json:"expires_in"`
}

// RegisterSSOAdminRoutes mounts /admin/v1/sso-configurations CRUD on mux.
func RegisterSSOAdminRoutes(mux *http.ServeMux, svc SSOAdminService, token string) {
	mux.HandleFunc("GET /admin/v1/sso-configurations/{domain_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		domainID := r.PathValue("domain_id")
		cfg, err := svc.GetSSOConfig(r.Context(), domainID)
		if err != nil {
			writeError(w, http.StatusNotFound, "sso configuration not found")
			return
		}
		writeJSON(w, http.StatusOK, cfg)
	}))

	mux.HandleFunc("PUT /admin/v1/sso-configurations/{domain_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		domainID := r.PathValue("domain_id")
		var cfg maildb.SSOConfig
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cfg.DomainID = domainID
		if err := svc.UpsertSSOConfig(r.Context(), cfg); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	mux.HandleFunc("DELETE /admin/v1/sso-configurations/{domain_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		domainID := r.PathValue("domain_id")
		if err := svc.DeleteSSOConfig(r.Context(), domainID); err != nil {
			writeError(w, http.StatusNotFound, "sso configuration not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
}

// RegisterSSORoutes mounts /auth/sso/* flow endpoints on mux.
// tm may be nil; in that case ACS and callback endpoints return 503.
func RegisterSSORoutes(mux *http.ServeMux, svc SSOFlowService, tm *auth.TokenManager) {
	// GET /auth/sso/initiate?domain={domainID}
	mux.HandleFunc("GET /auth/sso/initiate", func(w http.ResponseWriter, r *http.Request) {
		domainID := r.URL.Query().Get("domain")
		if domainID == "" {
			http.Error(w, "domain parameter required", http.StatusBadRequest)
			return
		}
		returnURL := r.URL.Query().Get("return")

		cfg, err := svc.GetSSOConfig(r.Context(), domainID)
		if err != nil {
			http.Error(w, "SSO not configured for this domain", http.StatusNotFound)
			return
		}

		switch cfg.Provider {
		case "saml":
			redirectURL, err := buildSAMLRedirectURL(cfg, returnURL)
			if err != nil {
				http.Error(w, "SSO initiation failed", http.StatusInternalServerError)
				return
			}
			http.Redirect(w, r, redirectURL, http.StatusFound)
		case "oidc":
			state, codeVerifier, err := sso.GenerateOIDCStateWithPKCE(domainID)
			if err != nil {
				http.Error(w, "SSO initiation failed", http.StatusInternalServerError)
				return
			}
			redirectURL := buildOIDCRedirectURL(cfg, state, codeVerifier, returnURL)
			http.Redirect(w, r, redirectURL, http.StatusFound)
		default:
			http.Error(w, "unsupported SSO provider", http.StatusBadRequest)
		}
	})

	// POST /auth/sso/saml/acs — SAML Assertion Consumer Service
	mux.HandleFunc("POST /auth/sso/saml/acs", func(w http.ResponseWriter, r *http.Request) {
		if tm == nil {
			http.Error(w, "token manager not configured", http.StatusServiceUnavailable)
			return
		}
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form body", http.StatusBadRequest)
			return
		}
		encoded := r.FormValue("SAMLResponse")
		if encoded == "" {
			http.Error(w, "SAMLResponse is required", http.StatusBadRequest)
			return
		}
		domainID := r.FormValue("RelayState")

		xmlData, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			http.Error(w, "invalid SAMLResponse encoding", http.StatusBadRequest)
			return
		}
		if len(xmlData) > sso.SAMLMaxResponseBytes {
			http.Error(w, "SAMLResponse too large", http.StatusRequestEntityTooLarge)
			return
		}

		email, err := sso.ParseSAMLNameID(xmlData)
		if err != nil {
			http.Error(w, "could not extract NameID from SAML assertion", http.StatusBadRequest)
			return
		}

		info, err := ssoLookupOrProvision(r.Context(), svc, email, domainID)
		if err != nil {
			http.Error(w, fmt.Sprintf("SSO user resolution failed: %v", err), http.StatusUnauthorized)
			return
		}

		samlCfg, _ := svc.GetSSOConfig(r.Context(), domainID)
		ttl := ssoSessionTTL(samlCfg)
		token, err := tm.Sign(auth.Claims{
			UserID:   info.UserID,
			DomainID: info.DomainID,
			Role:     "user",
		}, ttl)
		if err != nil {
			http.Error(w, "token issuance failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, ssoTokenResponse{Token: token, ExpiresIn: int(ttl.Seconds())})
	})

	// GET /auth/sso/oidc/callback?code=&state=
	mux.HandleFunc("GET /auth/sso/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		if tm == nil {
			http.Error(w, "token manager not configured", http.StatusServiceUnavailable)
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code parameter is required", http.StatusBadRequest)
			return
		}
		state := r.URL.Query().Get("state")
		domainID, codeVerifier, err := sso.ParseOIDCStateFields(state)
		if err != nil {
			http.Error(w, "invalid state parameter", http.StatusBadRequest)
			return
		}

		cfg, err := svc.GetSSOConfig(r.Context(), domainID)
		if err != nil {
			http.Error(w, "SSO not configured for this domain", http.StatusNotFound)
			return
		}

		tokenEndpoint := cfg.DiscoveryURL
		if tokenEndpoint == "" {
			http.Error(w, "OIDC token endpoint not configured", http.StatusInternalServerError)
			return
		}

		idToken, err := exchangeOIDCCode(r.Context(), tokenEndpoint, cfg.ClientID, cfg.ClientSecret, code, codeVerifier)
		if err != nil {
			http.Error(w, fmt.Sprintf("OIDC code exchange failed: %v", err), http.StatusUnauthorized)
			return
		}

		email, err := sso.VerifyAndParseIDToken(idToken, cfg.ClientSecret, cfg.ClientID, time.Now())
		if err != nil {
			http.Error(w, "could not verify ID token: "+err.Error(), http.StatusUnauthorized)
			return
		}

		info, err := ssoLookupOrProvision(r.Context(), svc, email, domainID)
		if err != nil {
			http.Error(w, fmt.Sprintf("SSO user resolution failed: %v", err), http.StatusUnauthorized)
			return
		}

		ttl := ssoSessionTTL(cfg)
		token, err := tm.Sign(auth.Claims{
			UserID:   info.UserID,
			DomainID: info.DomainID,
			Role:     "user",
		}, ttl)
		if err != nil {
			http.Error(w, "token issuance failed", http.StatusInternalServerError)
			return
		}
		writeJSON(w, http.StatusOK, ssoTokenResponse{Token: token, ExpiresIn: int(ttl.Seconds())})
	})
}

// ssoSessionTTL returns the per-domain JWT lifetime from the SSO config.
// Defaults to 15 minutes when SessionTTLSeconds is zero.
func ssoSessionTTL(cfg maildb.SSOConfig) time.Duration {
	if cfg.SessionTTLSeconds > 0 {
		return time.Duration(cfg.SessionTTLSeconds) * time.Second
	}
	return 15 * time.Minute
}

// ssoLookupOrProvision finds the user by email; if not found and the SSO config
// allows JIT provisioning, creates the account on demand.
func ssoLookupOrProvision(ctx context.Context, svc SSOFlowService, email, domainID string) (maildb.SSOUserInfo, error) {
	info, err := svc.GetUserByEmail(ctx, email)
	if err == nil {
		return info, nil
	}
	if domainID == "" {
		return maildb.SSOUserInfo{}, fmt.Errorf("user not found and no domain for JIT provisioning")
	}
	cfg, err2 := svc.GetSSOConfig(ctx, domainID)
	if err2 != nil {
		return maildb.SSOUserInfo{}, fmt.Errorf("user not found: %w", err)
	}
	if !cfg.JITProvisioning {
		return maildb.SSOUserInfo{}, fmt.Errorf("user not found and JIT provisioning is disabled")
	}
	return svc.JITCreateSSOUser(ctx, email, domainID, "")
}

// oidcCodeResponse is the JSON body returned by an OIDC token endpoint.
type oidcCodeResponse struct {
	IDToken string `json:"id_token"`
}

// exchangeOIDCCode sends a code exchange request to the OIDC token endpoint
// and returns the raw ID token string. codeVerifier is included when PKCE was
// used in the authorization request (RFC 7636 §4.5).
func exchangeOIDCCode(ctx context.Context, tokenURL, clientID, clientSecret, code, codeVerifier string) (string, error) {
	form := url.Values{
		"grant_type":   {"authorization_code"},
		"code":         {code},
		"client_id":    {clientID},
		"client_secret": {clientSecret},
	}
	if codeVerifier != "" {
		form.Set("code_verifier", codeVerifier)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("build token request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token endpoint request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", fmt.Errorf("read token response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, body)
	}

	var tr oidcCodeResponse
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", fmt.Errorf("parse token response: %w", err)
	}
	if tr.IDToken == "" {
		return "", fmt.Errorf("token endpoint did not return id_token")
	}
	return tr.IDToken, nil
}

func buildSAMLRedirectURL(cfg maildb.SSOConfig, returnURL string) (string, error) {
	id, err := sso.GenerateSAMLRequestID()
	if err != nil {
		return "", err
	}
	req := sso.AuthnRequest{
		ID:          id,
		Destination: cfg.SSOURL,
		Issuer:      cfg.EntityID,
	}
	_, err = req.BuildXML()
	if err != nil {
		return "", err
	}
	_ = returnURL
	return cfg.SSOURL, nil
}

func buildOIDCRedirectURL(cfg maildb.SSOConfig, state, codeVerifier, returnURL string) string {
	_ = returnURL
	base := cfg.SSOURL
	if base == "" {
		base = cfg.DiscoveryURL
	}
	if base == "" {
		return ""
	}
	u := base + "?client_id=" + cfg.ClientID + "&state=" + state + "&response_type=code&scope=openid+email"
	if codeVerifier != "" {
		challenge := sso.PKCEChallenge(codeVerifier)
		u += "&code_challenge=" + challenge + "&code_challenge_method=S256"
	}
	return u
}
