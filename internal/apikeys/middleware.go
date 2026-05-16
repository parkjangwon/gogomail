package apikeys

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"strings"
)

type KeyVerifier interface {
	Verify(ctx context.Context, keyHash string, ip net.IP) (*KeyInfo, error)
}

type KeyInfo struct {
	ID       string
	DomainID string
	Scopes   []string
}

type contextKey struct{}

var keyInfoContextKey = &contextKey{}

func ContextWithKeyInfo(ctx context.Context, info *KeyInfo) context.Context {
	return context.WithValue(ctx, keyInfoContextKey, info)
}

func KeyInfoFromContext(ctx context.Context) (*KeyInfo, bool) {
	info, ok := ctx.Value(keyInfoContextKey).(*KeyInfo)
	return info, ok
}

func Middleware(verifier KeyVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			auth := r.Header.Get("Authorization")
			if auth == "" {
				next.ServeHTTP(w, r)
				return
			}

			parts := strings.SplitN(auth, " ", 2)
			if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
				next.ServeHTTP(w, r)
				return
			}

			token := parts[1]
			if !strings.HasPrefix(token, keyPrefix) {
				next.ServeHTTP(w, r)
				return
			}

			if !VerifyKeyFormat(token) {
				writeError(w, http.StatusUnauthorized, "invalid API key format")
				return
			}

			hash := HashKey(token)
			ip := parseClientIP(r)

			info, err := verifier.Verify(r.Context(), hash, ip)
			if err != nil {
				writeError(w, http.StatusUnauthorized, "invalid or revoked API key")
				return
			}

			if strings.TrimSpace(info.ID) != "" {
				r.Header.Set("X-Gogomail-API-Key-ID", strings.TrimSpace(info.ID))
			}
			if strings.TrimSpace(info.DomainID) != "" {
				r.Header.Set("X-Gogomail-Domain-ID", strings.TrimSpace(info.DomainID))
			}
			next.ServeHTTP(w, r.WithContext(ContextWithKeyInfo(r.Context(), info)))
		})
	}
}

func parseClientIP(r *http.Request) net.IP {
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		if len(parts) > 0 {
			ip := net.ParseIP(strings.TrimSpace(parts[0]))
			if ip != nil {
				return ip
			}
		}
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return net.ParseIP(host)
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`+"\n", message)
}
