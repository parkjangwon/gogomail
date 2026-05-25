package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/webauthn"
)

// RegisterWebAuthnRoutes registers WebAuthn/Passkey MFA endpoints.
// All routes require a valid user JWT (verified via tokenManager).
//
//   - POST /api/v1/mfa/webauthn/register/begin      → {publicKeyCredentialCreationOptions}
//   - POST /api/v1/mfa/webauthn/register/complete   → {id, name, createdAt}
//   - POST /api/v1/mfa/webauthn/authenticate/begin  → {publicKeyCredentialRequestOptions}
//   - POST /api/v1/mfa/webauthn/authenticate/complete → {token}  (MFA-verified JWT)
//   - GET  /api/v1/mfa/webauthn/credentials         → [{id, name, createdAt, lastUsedAt}]
//   - DELETE /api/v1/mfa/webauthn/credentials/{id}  → 204
func RegisterWebAuthnRoutes(mux *http.ServeMux, svc *webauthn.Service, tokenManager *auth.TokenManager) {
	// POST /api/v1/mfa/webauthn/register/begin
	mux.HandleFunc("POST /api/v1/mfa/webauthn/register/begin", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		var req struct {
			Username    string `json:"username"`
			DisplayName string `json:"display_name"`
		}
		// Body is optional — fall back to user ID as name.
		_ = decodeJSONBody(r, &req)
		if req.Username == "" {
			req.Username = claims.UserID
		}
		if req.DisplayName == "" {
			req.DisplayName = claims.UserID
		}

		optionsJSON, err := svc.BeginRegistration(r.Context(), claims.UserID, req.Username, req.DisplayName)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to begin webauthn registration")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(optionsJSON)
	})

	// POST /api/v1/mfa/webauthn/register/complete
	mux.HandleFunc("POST /api/v1/mfa/webauthn/register/complete", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			name = "Security Key"
		}

		body, err := readRequestBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		cred, err := svc.CompleteRegistration(r.Context(), claims.UserID, name, body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "webauthn registration failed: "+sanitizeError(err))
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{
			"id":         cred.ID,
			"name":       cred.Name,
			"created_at": cred.CreatedAt.Format(time.RFC3339),
		})
	})

	// POST /api/v1/mfa/webauthn/authenticate/begin
	mux.HandleFunc("POST /api/v1/mfa/webauthn/authenticate/begin", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		optionsJSON, err := svc.BeginAuthentication(r.Context(), claims.UserID)
		if err != nil {
			if strings.Contains(err.Error(), "no credentials registered") {
				writeError(w, http.StatusUnprocessableEntity, "no webauthn credentials registered")
				return
			}
			writeError(w, http.StatusInternalServerError, "failed to begin webauthn authentication")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(optionsJSON)
	})

	// POST /api/v1/mfa/webauthn/authenticate/complete
	mux.HandleFunc("POST /api/v1/mfa/webauthn/authenticate/complete", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		body, err := readRequestBody(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, "failed to read request body")
			return
		}

		_, err = svc.CompleteAuthentication(r.Context(), claims.UserID, body)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "webauthn authentication failed")
			return
		}

		// Issue a full MFA-verified token (reuses fullTokenTTL from mail_mfa.go).
		fullClaims := auth.Claims{
			UserID:         claims.UserID,
			DomainID:       claims.DomainID,
			CompanyID:      claims.CompanyID,
			SessionVersion: claims.SessionVersion,
			MFAVerified:    true,
		}
		token, err := tokenManager.Sign(fullClaims, fullTokenTTL)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "failed to issue token")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"token":      token,
			"expires_at": time.Now().UTC().Add(fullTokenTTL).Format(time.RFC3339),
		})
	})

	// GET /api/v1/mfa/webauthn/credentials
	mux.HandleFunc("GET /api/v1/mfa/webauthn/credentials", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		// We need the store to list credentials; expose it through the service.
		// Use an internal accessor defined below.
		creds, err := svc.ListCredentials(r.Context(), claims.UserID)
		if err != nil {
			writeInternalServerError(w)
			return
		}

		type credView struct {
			ID          string  `json:"id"`
			Name        string  `json:"name"`
			CreatedAt   string  `json:"created_at"`
			LastUsedAt  *string `json:"last_used_at,omitempty"`
		}
		out := make([]credView, 0, len(creds))
		for _, c := range creds {
			v := credView{
				ID:        c.ID,
				Name:      c.Name,
				CreatedAt: c.CreatedAt.Format(time.RFC3339),
			}
			if c.LastUsedAt != nil {
				s := c.LastUsedAt.Format(time.RFC3339)
				v.LastUsedAt = &s
			}
			out = append(out, v)
		}
		writeJSON(w, http.StatusOK, map[string]any{"credentials": out})
	})

	// DELETE /api/v1/mfa/webauthn/credentials/{id}
	mux.HandleFunc("DELETE /api/v1/mfa/webauthn/credentials/{id}", func(w http.ResponseWriter, r *http.Request) {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		credID := r.PathValue("id")
		if credID == "" {
			writeError(w, http.StatusBadRequest, "credential id is required")
			return
		}

		if err := svc.DeleteCredential(r.Context(), claims.UserID, credID); err != nil {
			if strings.Contains(err.Error(), "not found") {
				writeError(w, http.StatusNotFound, "credential not found")
				return
			}
			writeInternalServerError(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// readRequestBody reads the full request body and returns the bytes.
func readRequestBody(r *http.Request) ([]byte, error) {
	if r.Body == nil {
		return nil, errors.New("empty request body")
	}
	defer r.Body.Close()

	buf := make([]byte, 0, 4096)
	tmp := make([]byte, 512)
	for {
		n, err := r.Body.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			break
		}
	}
	return buf, nil
}

// sanitizeError returns a safe error message, stripping internal prefixes.
func sanitizeError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	// Strip "webauthn: " prefix if present for cleaner client messages.
	msg = strings.TrimPrefix(msg, "webauthn: ")
	return msg
}
