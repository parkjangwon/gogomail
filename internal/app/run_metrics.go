package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/dkim"
	"github.com/gogomail/gogomail/internal/httpapi"
	ldapgw "github.com/gogomail/gogomail/internal/ldapgw"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/outbound"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func smtpMetrics(cfg config.Config, logger *slog.Logger) smtpd.Metrics {
	switch cfg.MetricsBackend {
	case "slog":
		return observability.NewSlogAdapter(logger)
	case "prometheus":
		return sharedPrometheusAdapter()
	}
	return nil
}

func deliveryMetrics(cfg config.Config, logger *slog.Logger) delivery.Metrics {
	switch cfg.MetricsBackend {
	case "slog":
		return observability.NewSlogAdapter(logger)
	case "prometheus":
		return sharedPrometheusAdapter()
	}
	return nil
}

func ldapMetrics(cfg config.Config, logger *slog.Logger) ldapgw.Metrics {
	switch cfg.MetricsBackend {
	case "slog":
		return observability.NewSlogAdapter(logger)
	case "prometheus":
		return sharedPrometheusAdapter()
	}
	return nil
}

func webDAVMetrics(cfg config.Config, logger *slog.Logger) httpapi.WebDAVMetrics {
	switch cfg.MetricsBackend {
	case "slog":
		return observability.NewSlogAdapter(logger)
	case "prometheus":
		return sharedPrometheusAdapter()
	}
	return nil
}

var (
	prometheusAdapterOnce sync.Once
	prometheusAdapter     *observability.PrometheusAdapter
)

func sharedPrometheusAdapter() *observability.PrometheusAdapter {
	prometheusAdapterOnce.Do(func() {
		prometheusAdapter = observability.NewPrometheusAdapter()
	})
	return prometheusAdapter
}

// serveMetrics starts a lightweight HTTP server on cfg.MetricsAddr that
// exposes Prometheus-format metrics at /metrics.  It runs until ctx is done.
func serveMetrics(ctx context.Context, cfg config.Config, logger *slog.Logger) {
	if strings.ToLower(strings.TrimSpace(cfg.MetricsBackend)) != "prometheus" {
		return
	}
	adapter := sharedPrometheusAdapter()
	mux := http.NewServeMux()
	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		fmt.Fprint(w, adapter.Text())
	})
	srv := &http.Server{
		Addr:              cfg.MetricsAddr,
		Handler:           mux,
		ReadTimeout:       5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
		ReadHeaderTimeout: 2 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	logger.Info("metrics server listening", "addr", cfg.MetricsAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		logger.Error("metrics server error", "error", err)
	}
}

func apiUsageExportManifestSigner(cfg config.Config) apimeter.ExportManifestSigner {
	switch strings.ToLower(strings.TrimSpace(cfg.APIUsageExportManifestSignerBackend)) {
	case "local-hmac":
		return apimeter.HMACExportManifestSigner{
			KeyID:  cfg.APIUsageExportManifestSignerKeyID,
			Secret: []byte(cfg.APIUsageExportManifestSignerSecret),
		}
	case "local-ed25519":
		privateKey, ok := decodeExportManifestKey(cfg.APIUsageExportSignerPrivateKey, ed25519.PrivateKeySize)
		if !ok {
			return nil
		}
		return apimeter.Ed25519ExportManifestSigner{
			KeyID:      cfg.APIUsageExportManifestSignerKeyID,
			PrivateKey: ed25519.PrivateKey(privateKey),
		}
	case "remote-ed25519":
		publicKey, ok := decodeExportManifestKey(cfg.APIUsageExportSignerPublicKey, ed25519.PublicKeySize)
		if !ok {
			return nil
		}
		return apimeter.RemoteEd25519ExportManifestSigner{
			Endpoint:  cfg.APIUsageExportSignerURL,
			Token:     cfg.APIUsageExportSignerToken,
			KeyID:     cfg.APIUsageExportManifestSignerKeyID,
			PublicKey: ed25519.PublicKey(publicKey),
		}
	default:
		return nil
	}
}

func apiUsageExportManifestVerifier(cfg config.Config) apimeter.ExportManifestSignatureVerifier {
	switch strings.ToLower(strings.TrimSpace(cfg.APIUsageExportManifestSignerBackend)) {
	case "local-hmac":
		return apimeter.HMACExportManifestSignatureVerifier{
			KeyID:  cfg.APIUsageExportManifestSignerKeyID,
			Secret: []byte(cfg.APIUsageExportManifestSignerSecret),
		}
	case "local-ed25519":
		publicKey, ok := decodeExportManifestKey(cfg.APIUsageExportSignerPublicKey, ed25519.PublicKeySize)
		if !ok {
			return nil
		}
		return apimeter.Ed25519ExportManifestSignatureVerifier{
			KeyID:     cfg.APIUsageExportManifestSignerKeyID,
			PublicKey: ed25519.PublicKey(publicKey),
		}
	case "remote-ed25519":
		publicKey, ok := decodeExportManifestKey(cfg.APIUsageExportSignerPublicKey, ed25519.PublicKeySize)
		if !ok {
			return nil
		}
		return apimeter.Ed25519ExportManifestSignatureVerifier{
			KeyID:     cfg.APIUsageExportManifestSignerKeyID,
			PublicKey: ed25519.PublicKey(publicKey),
		}
	default:
		return nil
	}
}

func decodeExportManifestKey(value string, size int) ([]byte, bool) {
	value = strings.TrimSpace(value)
	if len(value) > base64.StdEncoding.EncodedLen(size) {
		return nil, false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil || len(decoded) != size {
		return nil, false
	}
	return decoded, true
}

type dkimKeyRepository interface {
	ActiveDKIMKey(ctx context.Context, domainID string) (maildb.DKIMKey, error)
}

type dkimKeyProvider struct {
	repository dkimKeyRepository
}

func (p dkimKeyProvider) DKIMKey(ctx context.Context, job delivery.Job) (dkim.Key, error) {
	key, err := p.repository.ActiveDKIMKey(ctx, job.DomainID)
	if err != nil {
		return dkim.Key{}, err
	}
	return dkim.Key{
		Domain:        key.DomainName,
		Selector:      key.Selector,
		PrivateKeyPEM: key.PrivateKeyPEM,
	}, nil
}

func apiMeteringHandler(next http.Handler, cfg config.Config, logger *slog.Logger, outboxDB *sql.DB, tokenManager *auth.TokenManager, adminToken string) http.Handler {
	opts := []apimeter.Option{
		apimeter.WithTimeout(cfg.APIMeteringTimeout),
		apimeter.WithIdentityResolver(meteringIdentityResolver(tokenManager, adminToken)),
	}
	switch strings.ToLower(strings.TrimSpace(cfg.APIMeteringBackend)) {
	case "", "none":
		return next
	case "slog":
		if logger != nil {
			logger.Info("api metering enabled", "backend", "slog", "timeout", cfg.APIMeteringTimeout.String())
		}
		return apimeter.Handler(next, apimeter.SlogSink{Logger: logger}, opts...)
	case "outbox":
		if outboxDB == nil {
			return next
		}
		if logger != nil {
			logger.Info("api metering enabled", "backend", "outbox", "timeout", cfg.APIMeteringTimeout.String())
		}
		return apimeter.Handler(next, apimeter.NewPostgresOutboxSink(outboxDB), opts...)
	default:
		return next
	}
}

func meteringIdentityResolver(tokenManager *auth.TokenManager, adminToken string) apimeter.IdentityResolver {
	return func(r *http.Request) apimeter.Identity {
		if r == nil {
			return apimeter.Identity{AuthSource: apimeter.AuthSourceAnonymous}
		}
		id := apimeter.Identity{
			TenantID:    r.Header.Get("X-Gogomail-Tenant-ID"),
			CompanyID:   r.Header.Get("X-Gogomail-Company-ID"),
			DomainID:    r.Header.Get("X-Gogomail-Domain-ID"),
			UserID:      firstNonEmptyString(r.Header.Get("X-Gogomail-Resolved-User-ID"), r.Header.Get("X-Gogomail-User-ID"), r.URL.Query().Get("user_id")),
			APIKeyID:    r.Header.Get("X-Gogomail-API-Key-ID"),
			PrincipalID: r.Header.Get("X-Gogomail-Principal-ID"),
			AuthSource:  apimeter.AuthSourceAnonymous,
		}
		if info, ok := apikeys.KeyInfoFromContext(r.Context()); ok && info != nil {
			id.DomainID = firstNonEmptyString(info.DomainID, id.DomainID)
			id.APIKeyID = firstNonEmptyString(info.ID, id.APIKeyID)
			id.AuthSource = apimeter.AuthSourceAPIKey
			return id.Normalize()
		}
		bearer := meteringBearerToken(r)
		if tokenManager != nil && bearer != "" {
			if claims, err := tokenManager.Verify(bearer); err == nil {
				id.UserID = claims.UserID
				id.DomainID = claims.DomainID
				id.AuthSource = apimeter.AuthSourceBearer
				return id.Normalize()
			}
			id.AuthSource = apimeter.AuthSourceBearer
			return id.Normalize()
		}
		if meteringAdminTokenMatches(r, adminToken) {
			id.AuthSource = apimeter.AuthSourceAdminToken
			return id.Normalize()
		}
		if bearer != "" {
			id.AuthSource = apimeter.AuthSourceBearer
			return id.Normalize()
		}
		if strings.TrimSpace(id.UserID) != "" {
			id.AuthSource = apimeter.AuthSourceQueryUserID
		}
		return id.Normalize()
	}
}

func meteringBearerToken(r *http.Request) string {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("bearer "):])
	}
	return ""
}

func meteringAdminTokenMatches(r *http.Request, token string) bool {
	token = strings.TrimSpace(token)
	if token == "" {
		return false
	}
	got := strings.TrimSpace(r.Header.Get("X-Admin-Token"))
	if got == "" {
		got = meteringBearerToken(r)
	}
	if got == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(token))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func deliveryRouterFromConfig(cfg config.Config) delivery.Router {
	if strings.TrimSpace(cfg.DeliverySmartHost) == "" {
		return nil
	}
	return delivery.StaticRouter{RouteConfig: delivery.Route{
		Hosts:       []string{cfg.DeliverySmartHost},
		Port:        cfg.DeliverySmartHostPort,
		TLSMode:     delivery.DeliveryTLSMode(cfg.DeliverySmartHostTLSMode),
		ImplicitTLS: cfg.DeliverySmartHostImplicitTLS,
		Auth: delivery.RouteAuth{
			Identity: cfg.DeliverySmartHostIdentity,
			Username: cfg.DeliverySmartHostUsername,
			Password: cfg.DeliverySmartHostPassword,
		},
	}}
}

type deliveryRouteRepository interface {
	DeliveryRouteForDomain(ctx context.Context, domain string) (maildb.DeliveryRouteView, error)
}

type postgresDeliveryRouter struct {
	repository      deliveryRouteRepository
	fallbackTLSMode delivery.DeliveryTLSMode
}

func (r postgresDeliveryRouter) Route(ctx context.Context, _ delivery.Job, domain string) (delivery.Route, error) {
	if r.repository == nil {
		return delivery.Route{TLSMode: r.fallbackTLSMode}, nil
	}
	route, err := r.repository.DeliveryRouteForDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, maildb.ErrDeliveryRouteNotFound) {
			return delivery.Route{TLSMode: r.fallbackTLSMode}, nil
		}
		return delivery.Route{}, err
	}
	return delivery.Route{
		Farm:        outbound.Farm(route.Farm),
		Domain:      domain,
		Hosts:       route.Hosts,
		Port:        route.Port,
		Hello:       route.SMTPHello,
		TLSMode:     delivery.DeliveryTLSMode(route.TLSMode),
		ImplicitTLS: route.ImplicitTLS,
		PoolName:    route.PoolName,
		Auth: delivery.RouteAuth{
			Identity: route.AuthIdentity,
			Username: route.AuthUsername,
			Password: route.AuthPassword,
		},
	}, nil
}

func deliveryFarmLimits(values map[string]int) map[outbound.Farm]int {
	result := make(map[outbound.Farm]int, len(values))
	for farm, limit := range values {
		result[outbound.Farm(farm)] = limit
	}
	return result
}

func deliveryDomainBackoffFromConfig(cfg config.Config, redisClient *redis.Client) delivery.DomainBackoff {
	if !cfg.DeliveryDomainBackoffEnabled {
		return nil
	}
	policy := delivery.DomainBackoffPolicy{
		BaseDelay: cfg.DeliveryDomainBackoffBaseDelay,
		MaxDelay:  cfg.DeliveryDomainBackoffMaxDelay,
		Scope:     delivery.DomainBackoffScope(cfg.DeliveryDomainBackoffScope),
	}
	if strings.EqualFold(strings.TrimSpace(cfg.DeliveryDomainBackoffBackend), "redis") {
		return delivery.NewRedisDomainBackoff(redisClient, "gogomail:delivery:domain_backoff", policy)
	}
	return delivery.NewInMemoryDomainBackoff(policy)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return ""
}
