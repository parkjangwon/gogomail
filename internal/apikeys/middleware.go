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
	ID             string
	UserID         string
	DomainID       string
	Scopes         []string
	PermissionMode string
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

func Middleware(verifier KeyVerifier, trustedProxyCIDRs []string) func(http.Handler) http.Handler {
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
			if !IsManagedAPIKey(token) {
				next.ServeHTTP(w, r)
				return
			}

			if !VerifyKeyFormat(token) {
				writeError(w, http.StatusUnauthorized, "invalid API key format")
				return
			}

			hash := HashKey(token)
			ip := parseClientIP(r, trustedProxyCIDRs)

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
			if strings.TrimSpace(info.UserID) != "" {
				r.Header.Set("X-Gogomail-Resolved-User-ID", strings.TrimSpace(info.UserID))
			}
			next.ServeHTTP(w, r.WithContext(ContextWithKeyInfo(r.Context(), info)))
		})
	}
}

func parseClientIP(r *http.Request, trustedProxyCIDRs []string) net.IP {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	remoteIP := net.ParseIP(host)
	if !isTrustedForwardingProxy(remoteIP, trustedProxyCIDRs) {
		return remoteIP
	}
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
	return remoteIP
}

func isTrustedForwardingProxy(ip net.IP, trustedProxyCIDRs []string) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() {
		return true
	}
	for _, raw := range trustedProxyCIDRs {
		if candidate := net.ParseIP(raw); candidate != nil && candidate.Equal(ip) {
			return true
		}
		_, cidr, err := net.ParseCIDR(raw)
		if err == nil && cidr.Contains(ip) {
			return true
		}
	}
	return false
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	fmt.Fprintf(w, `{"error":%q}`+"\n", message)
}
