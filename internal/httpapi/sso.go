package httpapi

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/sso"
)

// SSOAdminService manages SSO configurations per domain.
type SSOAdminService interface {
	GetSSOConfig(ctx context.Context, domainID string) (maildb.SSOConfig, error)
	UpsertSSOConfig(ctx context.Context, cfg maildb.SSOConfig) error
	DeleteSSOConfig(ctx context.Context, domainID string) error
}

// SSOFlowService provides the SSO initiation redirect URL.
type SSOFlowService interface {
	GetSSOConfig(ctx context.Context, domainID string) (maildb.SSOConfig, error)
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
func RegisterSSORoutes(mux *http.ServeMux, svc SSOFlowService) {
	// GET /auth/sso/initiate?domain={domainID}&return={url}
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
			state, err := sso.GenerateOIDCState()
			if err != nil {
				http.Error(w, "SSO initiation failed", http.StatusInternalServerError)
				return
			}
			redirectURL := buildOIDCRedirectURL(cfg, state, returnURL)
			http.Redirect(w, r, redirectURL, http.StatusFound)
		default:
			http.Error(w, "unsupported SSO provider", http.StatusBadRequest)
		}
	})

	// POST /auth/sso/saml/acs — SAML Assertion Consumer Service
	mux.HandleFunc("POST /auth/sso/saml/acs", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "SAML ACS: full assertion validation not yet implemented", http.StatusNotImplemented)
	})

	// GET /auth/sso/oidc/callback
	mux.HandleFunc("GET /auth/sso/oidc/callback", func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "OIDC callback: token exchange not yet implemented", http.StatusNotImplemented)
	})
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
	_, err = req.MarshalXML()
	if err != nil {
		return "", err
	}
	_ = returnURL
	return cfg.SSOURL, nil
}

func buildOIDCRedirectURL(cfg maildb.SSOConfig, state, returnURL string) string {
	_ = returnURL
	base := cfg.SSOURL
	if base == "" {
		base = cfg.DiscoveryURL
	}
	if base == "" {
		return ""
	}
	return base + "?client_id=" + cfg.ClientID + "&state=" + state + "&response_type=code&scope=openid+email"
}
