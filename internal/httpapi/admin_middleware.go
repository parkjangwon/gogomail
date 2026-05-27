package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
)

func RequestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := strings.TrimSpace(r.Header.Get("X-Request-ID"))
		if requestID == "" || len(requestID) > 128 || strings.ContainsAny(requestID, "\r\n") {
			requestID = newRequestID()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(ContextWithRequestID(r.Context(), requestID)))
	})
}

func newRequestID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// internalProxyHeaders lists headers that are set by internal middleware only.
// Any value sent by an external caller is stripped before processing to prevent
// metering and billing attribution spoofing.
var internalProxyHeaders = []string{
	"X-Gogomail-Resolved-User-ID",
	"X-Gogomail-Tenant-ID",
	"X-Gogomail-Company-ID",
	"X-Gogomail-Domain-ID",
	"X-Gogomail-Principal-ID",
	"X-Gogomail-API-Key-ID",
}

// StripInternalHeadersMiddleware removes internal X-Gogomail-* headers from every
// inbound request. This prevents external callers from spoofing metering or billing
// attribution by injecting these headers.
func StripInternalHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for _, h := range internalProxyHeaders {
			r.Header.Del(h)
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adds defensive HTTP security headers to every response.
// It should wrap the outermost handler in production deployments.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Frame-Options", "DENY")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-XSS-Protection", "0") // modern browsers ignore this; CSP is the real defence
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; frame-ancestors 'none'")
		h.Set("Strict-Transport-Security", "max-age=63072000; includeSubDomains; preload")
		next.ServeHTTP(w, r)
	})
}

// CORSMiddleware returns a middleware that adds CORS headers to every response.
// allowedOrigins is a comma-separated list of allowed origins; pass "*" to
// allow all origins.  OPTIONS preflight requests are answered immediately with
// 204 No Content.
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	origins := parseCORSOrigins(allowedOrigins)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if origin != "" && len(origins) > 0 {
				if allowed, normalized := matchCORSOrigin(origins, origin); allowed {
					w.Header().Set("Access-Control-Allow-Origin", normalized)
					w.Header().Set("Vary", "Origin")
					// Browsers reject credentials with wildcard origin; only emit when origin is explicit.
					if normalized != "*" {
						w.Header().Set("Access-Control-Allow-Credentials", "true")
					}
					w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
					w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type, X-Requested-With, X-Admin-Token, X-Gogomail-User-ID, X-Gogomail-User-Email")
					w.Header().Set("Access-Control-Max-Age", "86400")
				}
			}
			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func parseCORSOrigins(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func matchCORSOrigin(allowed []string, origin string) (bool, string) {
	origin = strings.TrimSpace(origin)
	for _, a := range allowed {
		if a == "*" {
			return true, "*"
		}
		if strings.EqualFold(a, origin) {
			return true, a
		}
	}
	return false, ""
}

// AdminIPRateLimiter provides in-process per-IP rate limiting for admin API endpoints.
// Use NewAdminIPRateLimiter to construct and Middleware to obtain the http.Handler wrapper.
type AdminIPRateLimiter struct {
	mu      sync.Mutex
	windows map[string][]time.Time
	limit   int
	window  time.Duration
}

// NewAdminIPRateLimiter returns a limiter that allows up to limit requests per window per IP.
func NewAdminIPRateLimiter(limit int, window time.Duration) *AdminIPRateLimiter {
	if limit <= 0 {
		limit = 600
	}
	if window <= 0 {
		window = time.Minute
	}
	return &AdminIPRateLimiter{windows: make(map[string][]time.Time), limit: limit, window: window}
}

func (l *AdminIPRateLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-l.window)
	prev := l.windows[ip]
	fresh := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	if len(fresh) >= l.limit {
		l.windows[ip] = fresh
		return false
	}
	// Evict map entry when all timestamps expired to prevent unbounded growth.
	if len(fresh) == 0 {
		delete(l.windows, ip)
	}
	l.windows[ip] = append(fresh, now)
	return true
}

// Middleware wraps next and returns 429 when the per-IP limit is exceeded.
func (l *AdminIPRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := adminClientIP(r)
		if !l.allow(ip) {
			w.Header().Set("Retry-After", "60")
			http.Error(w, `{"error":"rate limit exceeded"}`, http.StatusTooManyRequests)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// MaxRequestBodyMiddleware returns a middleware that enforces a max request body size.
// Requests with a body larger than maxBytes receive 413 Entity Too Large.
// Pass 0 to use the default of 4 MiB.
// Upload-body endpoints (attachment / drive upload-session PUT bodies) are
// exempted because they enforce their own per-endpoint MaxBytesReader limits.
func MaxRequestBodyMiddleware(maxBytes int64) func(http.Handler) http.Handler {
	if maxBytes <= 0 {
		maxBytes = 4 * 1024 * 1024
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if isUploadBodyPath(r.Method, r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}
			if r.ContentLength > maxBytes {
				http.Error(w, `{"error":"request body too large"}`, http.StatusRequestEntityTooLarge)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
			next.ServeHTTP(w, r)
		})
	}
}

// isUploadBodyPath returns true for endpoints that accept large binary bodies
// and enforce their own size limit via MaxBytesReader.
func isUploadBodyPath(method, path string) bool {
	if method != http.MethodPut && method != http.MethodPost {
		return false
	}
	// Attachment upload bodies and drive upload-session bodies / staged objects.
	if strings.Contains(path, "/attachments/upload-sessions/") && strings.HasSuffix(path, "/body") {
		return true
	}
	if strings.Contains(path, "/drive/upload-sessions/") && strings.HasSuffix(path, "/body") {
		return true
	}
	if strings.Contains(path, "/drive/staged-objects") {
		return true
	}
	return false
}

// adminClientIP extracts the best client IP from the request.
// X-Real-IP and X-Forwarded-For are only honored when the TCP peer is a loopback
// or RFC1918 private address (i.e., a trusted reverse proxy on the same host or
// internal network). Public peers cannot spoof their client IP via these headers.
func adminClientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer := net.ParseIP(host)
	if peer != nil && isTrustedProxyIP(peer) {
		if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
			if ip := net.ParseIP(xri); ip != nil {
				return ip.String()
			}
		}
		if xff := strings.TrimSpace(r.Header.Get("X-Forwarded-For")); xff != "" {
			// leftmost entry is the original client
			if comma := strings.Index(xff, ","); comma > 0 {
				xff = strings.TrimSpace(xff[:comma])
			}
			if ip := net.ParseIP(xff); ip != nil {
				return ip.String()
			}
		}
	}
	return host
}

// isTrustedProxyIP returns true for loopback and RFC1918 private addresses.
func isTrustedProxyIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if ip.IsLoopback() || ip.IsPrivate() {
		return true
	}
	return false
}

func adminClaimsFromCtx(ctx context.Context) (auth.Claims, bool) {
	c, ok := ctx.Value(adminContextKey{}).(auth.Claims)
	return c, ok
}

// requiresCompanyAccess returns a non-nil error if the caller is a company_admin
// attempting to access data belonging to a different company.
// system_admin callers always pass. Callers with no claims in context also pass
// because static-token authentication proves access but does not carry tenant scope.
func requiresCompanyAccess(ctx context.Context, companyID string) error {
	claims, ok := adminClaimsFromCtx(ctx)
	if !ok {
		return nil // static token or unauthenticated-dev mode — no restriction
	}
	if claims.Role == "system_admin" {
		return nil
	}
	if claims.Role == "company_admin" && claims.CompanyID != companyID {
		return fmt.Errorf("access denied")
	}
	return nil
}

func adminJWTOrStaticAuth(token string, tokenMgr *auth.TokenManager, next http.HandlerFunc) http.HandlerFunc {
	return adminJWTOrStaticAuthWithEnvironment(token, tokenMgr, "production", next)
}

func adminJWTOrStaticAuthWithEnvironment(token string, tokenMgr *auth.TokenManager, environment string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	environment = strings.TrimSpace(strings.ToLower(environment))
	return func(w http.ResponseWriter, r *http.Request) {
		if token == "" && tokenMgr == nil {
			if environment != "development" && environment != "test" {
				writeError(w, http.StatusUnauthorized, "admin authentication is not configured")
				return
			}
			if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
				return
			}
			next(w, r)
			return
		}

		got, ok := adminTokenFromRequest(w, r)
		if !ok {
			return
		}
		authorized := false
		if tokenMgr != nil && got != "" {
			if claims, err := verifyAdminJWTClaims(r.Context(), tokenMgr, got); err == nil {
				if claims.Role == "company_admin" || claims.Role == "system_admin" {
					// Reject MFA-pending tokens — the holder must complete MFA first.
					if claims.TokenType == "mfa_pending" {
						writeError(w, http.StatusUnauthorized, "mfa verification required")
						return
					}
					r = r.WithContext(context.WithValue(r.Context(), adminContextKey{}, claims))
					authorized = true
				}
			}
		}
		if !authorized && token != "" && constantTimeTokenEqual(got, token) {
			authorized = true
		}
		if !authorized {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		if (r.Method == http.MethodGet || r.Method == http.MethodDelete) && !rejectBodylessRequestPayload(w, r) {
			return
		}
		next(w, r)
	}
}
