package app

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/dedup"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/dkim"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/mailauth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/ratelimit"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
)

func Run(ctx context.Context, mode Mode, cfg config.Config, logger *slog.Logger) error {
	if logger == nil {
		logger = slog.Default()
	}

	logger.Info("starting gogomail", "mode", mode, "env", cfg.Environment)

	switch mode {
	case ModeAllInOne, ModeAuthServer, ModeMailAPI, ModeAdminAPI:
		return runHTTP(ctx, cfg, logger, mode)
	case ModeEdgeMTA:
		return runReceiveMTA(ctx, cfg, logger, receiveMTAOptions{
			Component:              "edge-mta",
			Addr:                   cfg.SMTPAddr,
			EnableAuthVerification: cfg.SMTPAuthVerificationEnabled,
			EnableDMARCEnforcement: cfg.SMTPAuthVerificationEnabled,
			EnableBackpressure:     true,
			EnableRateLimit:        true,
			EnableDedup:            true,
		})
	case ModeInboundMTA:
		return runReceiveMTA(ctx, cfg, logger, receiveMTAOptions{
			Component:              "inbound-mta",
			Addr:                   cfg.InboundSMTPAddr,
			TrustedRelays:          cfg.InboundTrustedRelays,
			EnableAuthVerification: false,
			EnableDMARCEnforcement: false,
			EnableBackpressure:     true,
			EnableRateLimit:        true,
			EnableDedup:            true,
		})
	case ModeOutboxRelay:
		return runOutboxRelay(ctx, cfg, logger)
	case ModeEventWorker:
		return runEventWorker(ctx, cfg, logger)
	case ModeDeliveryWorker:
		return runDeliveryWorker(ctx, cfg, logger)
	case ModeOutboundMTA:
		return runSubmissionMTA(ctx, cfg, logger)
	case ModeBatchWorker:
		return waitForShutdown(ctx, logger, mode)
	default:
		return errors.New("unsupported mode")
	}
}

type receiveMTAOptions struct {
	Component              string
	Addr                   string
	TrustedRelays          []string
	EnableAuthVerification bool
	EnableDMARCEnforcement bool
	EnableBackpressure     bool
	EnableRateLimit        bool
	EnableDedup            bool
}

func runReceiveMTA(ctx context.Context, cfg config.Config, logger *slog.Logger, opts receiveMTAOptions) error {
	var resolver smtpd.RecipientResolver
	var recorder smtpd.MessageRecorder
	var deduplicator smtpd.Deduplicator
	var rateLimiter smtpd.RateLimiter
	var pressure smtpd.Backpressure
	var relayAuthorizer smtpd.RelayAuthorizer
	var redisClient *redis.Client

	if len(cfg.LocalRecipients) > 0 {
		staticResolver, err := smtpd.StaticResolverFromRecipients(cfg.LocalRecipients)
		if err != nil {
			return err
		}
		resolver = staticResolver
		logger.Info(opts.Component+" using static recipient resolver", "recipients", len(cfg.LocalRecipients))
	} else {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		resolver = repository
		recorder = repository
		logger.Info(opts.Component + " using database recipient resolver and message recorder")
	}

	if opts.EnableDedup && cfg.DedupBackend == "redis" {
		redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
		if err := redisClient.Ping(ctx).Err(); err != nil {
			_ = redisClient.Close()
			return err
		}

		deduplicator = dedup.NewRedisDeduplicator(redisClient, 24*time.Hour)
		logger.Info(opts.Component+" using redis deduplicator", "addr", cfg.RedisAddr)
	}
	if opts.EnableRateLimit && cfg.RateLimitBackend == "redis" {
		if redisClient == nil {
			redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
			if err := redisClient.Ping(ctx).Err(); err != nil {
				_ = redisClient.Close()
				return err
			}
		}
		rateLimiter = ratelimit.NewRedisLimiter(redisClient, int64(cfg.RcptRateLimitPerMinute), time.Minute)
		logger.Info(opts.Component+" using redis rate limiter", "addr", cfg.RedisAddr, "rcpt_per_minute", cfg.RcptRateLimitPerMinute)
	}
	if opts.EnableBackpressure && cfg.BackpressureBackend == "redis" {
		if redisClient == nil {
			redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
			if err := redisClient.Ping(ctx).Err(); err != nil {
				_ = redisClient.Close()
				return err
			}
		}
		pressure = backpressure.NewRedisBackpressure(redisClient, backpressure.DefaultStateKey)
		logger.Info(opts.Component+" using redis backpressure", "addr", cfg.RedisAddr)
	}
	if redisClient != nil {
		defer redisClient.Close()
	}
	if len(opts.TrustedRelays) > 0 {
		authorizer, err := smtpd.NewStaticTrustedRelays(opts.TrustedRelays)
		if err != nil {
			return err
		}
		relayAuthorizer = authorizer
		logger.Info(opts.Component+" using trusted relay policy", "cidrs", len(opts.TrustedRelays))
	}

	var authVerifier smtpd.AuthenticationVerifier
	if opts.EnableAuthVerification {
		authVerifier = mailauth.Verifier{
			AuthservID:           cfg.SMTPAuthservID,
			MaxDKIMVerifications: cfg.SMTPMaxDKIMVerifications,
		}
		logger.Info(
			opts.Component+" authentication verifier enabled",
			"authserv_id", cfg.SMTPAuthservID,
			"max_dkim_verifications", cfg.SMTPMaxDKIMVerifications,
		)
	}
	hooks := []smtpd.Hook(nil)
	if opts.EnableDMARCEnforcement && cfg.SMTPDMARCEnforcement != "monitor" {
		hooks = append(hooks, mailauth.EnforcementHook(mailauth.EnforcementOptions{
			Mode: mailauth.EnforcementMode(cfg.SMTPDMARCEnforcement),
		}))
		logger.Info(opts.Component+" DMARC enforcement configured", "mode", cfg.SMTPDMARCEnforcement)
	}

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:             storage.NewLocalStore(cfg.MailstoreRoot),
		Resolver:          resolver,
		Recorder:          recorder,
		Deduplicator:      deduplicator,
		RateLimiter:       rateLimiter,
		Backpressure:      pressure,
		AuthVerifier:      authVerifier,
		RelayAuthorizer:   relayAuthorizer,
		Metrics:           smtpMetrics(cfg, logger),
		AddReceivedHeader: cfg.SMTPAddReceivedHeader,
		ReceivedDomain:    cfg.SMTPDomain,
		RequireAuth:       cfg.SMTPRequireAuth,
		SupportSMTPUTF8:   cfg.SMTPSupportSMTPUTF8,
		SupportRequireTLS: cfg.SMTPSupportRequireTLS,
		SupportDSN:        cfg.SMTPSupportDSN,
		SupportBinaryMIME: cfg.SMTPSupportBinaryMIME,
		Hooks:             hooks,
		Policy: smtpd.ReceivePolicy{
			MaxRecipientsPerMessage: cfg.SMTPMaxRecipients,
			MaxMessageBytes:         cfg.SMTPMaxMessageBytes,
		},
	})

	return smtpd.RunServer(ctx, smtpd.ServerOptions{
		Addr:             opts.Addr,
		Domain:           cfg.SMTPDomain,
		Receiver:         receiver,
		Logger:           logger,
		ReadTimeout:      cfg.SMTPReadTimeout,
		WriteTimeout:     cfg.SMTPWriteTimeout,
		MaxMessageBytes:  cfg.SMTPMaxMessageBytes,
		MaxRecipients:    cfg.SMTPMaxRecipients,
		EnableSMTPUTF8:   cfg.SMTPSupportSMTPUTF8,
		EnableDSN:        cfg.SMTPSupportDSN,
		EnableRequireTLS: cfg.SMTPSupportRequireTLS,
		EnableBinaryMIME: cfg.SMTPSupportBinaryMIME,
	})
}

func runSubmissionMTA(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	repository := maildb.NewRepository(db)
	receiver := smtpd.NewSubmissionReceiver(smtpd.SubmissionOptions{
		Store:             storage.NewLocalStore(cfg.MailstoreRoot),
		Authenticator:     repository,
		Recorder:          repository,
		AddReceivedHeader: cfg.SubmissionAddReceivedHeader,
		ReceivedDomain:    cfg.SMTPDomain,
		SupportSMTPUTF8:   cfg.SubmissionSupportSMTPUTF8,
		SupportRequireTLS: cfg.SubmissionSupportRequireTLS,
		SupportDSN:        cfg.SubmissionSupportDSN,
		SupportBinaryMIME: cfg.SubmissionSupportBinaryMIME,
		Policy: smtpd.ReceivePolicy{
			MaxRecipientsPerMessage: cfg.SubmissionMaxRecipients,
			MaxMessageBytes:         cfg.SubmissionMaxMessageBytes,
		},
	})
	tlsConfig, err := smtpTLSConfig(cfg)
	if err != nil {
		return err
	}

	logger.Info(
		"outbound submission mta configured",
		"addr", cfg.SubmissionAddr,
		"smtps_addr", cfg.SubmissionSMTPSAddr,
		"tls_enabled", tlsConfig != nil,
		"allow_insecure_auth", cfg.SubmissionAllowInsecureAuth,
	)
	return runSubmissionServers(ctx, cfg, logger, receiver, tlsConfig)
}

func runSubmissionServers(ctx context.Context, cfg config.Config, logger *slog.Logger, backend gosmtp.Backend, tlsConfig *tls.Config) error {
	if strings.TrimSpace(cfg.SubmissionSMTPSAddr) == "" {
		return smtpd.RunServer(ctx, submissionServerOptions(cfg, logger, backend, tlsConfig, false))
	}
	if tlsConfig == nil {
		return errors.New("submission SMTPS requires SMTP TLS certificate and key files")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() {
		errCh <- smtpd.RunServer(ctx, submissionServerOptions(cfg, logger, backend, tlsConfig, false))
	}()
	go func() {
		errCh <- smtpd.RunServer(ctx, submissionServerOptions(cfg, logger, backend, tlsConfig, true))
	}()
	err := <-errCh
	cancel()
	return err
}

func submissionServerOptions(cfg config.Config, logger *slog.Logger, backend gosmtp.Backend, tlsConfig *tls.Config, implicitTLS bool) smtpd.ServerOptions {
	addr := strings.TrimSpace(cfg.SubmissionAddr)
	if implicitTLS {
		addr = strings.TrimSpace(cfg.SubmissionSMTPSAddr)
	}
	return smtpd.ServerOptions{
		Addr:              addr,
		Domain:            cfg.SMTPDomain,
		Backend:           backend,
		Logger:            logger,
		TLSConfig:         tlsConfig,
		ReadTimeout:       cfg.SMTPReadTimeout,
		WriteTimeout:      cfg.SMTPWriteTimeout,
		MaxMessageBytes:   cfg.SubmissionMaxMessageBytes,
		MaxRecipients:     cfg.SubmissionMaxRecipients,
		AllowInsecureAuth: cfg.SubmissionAllowInsecureAuth,
		EnableSMTPUTF8:    cfg.SubmissionSupportSMTPUTF8,
		EnableDSN:         cfg.SubmissionSupportDSN,
		EnableRequireTLS:  cfg.SubmissionSupportRequireTLS,
		EnableBinaryMIME:  cfg.SubmissionSupportBinaryMIME,
		ImplicitTLS:       implicitTLS,
	}
}

func smtpTLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.SMTPTLSCertFile == "" && cfg.SMTPTLSKeyFile == "" {
		return nil, nil
	}
	if cfg.SMTPTLSCertFile == "" || cfg.SMTPTLSKeyFile == "" {
		return nil, errors.New("both SMTP TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.SMTPTLSCertFile, cfg.SMTPTLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ServerName:   cfg.SMTPDomain,
	}, nil
}

func runOutboxRelay(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	relay, err := outbox.NewRelay(outbox.RelayOptions{
		Store:        outbox.NewPostgresStore(db, cfg.OutboxRelayMaxAttempts),
		Publisher:    outbox.NewRedisStreamPublisher(redisClient),
		BatchSize:    cfg.OutboxRelayBatchSize,
		PollInterval: cfg.OutboxRelayPollInterval,
		Logger:       logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"outbox relay started",
		"redis_addr", cfg.RedisAddr,
		"batch_size", cfg.OutboxRelayBatchSize,
		"poll_interval", cfg.OutboxRelayPollInterval.String(),
		"max_attempts", cfg.OutboxRelayMaxAttempts,
	)
	return relay.Run(ctx)
}

func runEventWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	router := eventstream.NewRouter()
	auditRepository := audit.NewPostgresRepository(db)
	if err := router.Register("mail.stored", audit.NewMailStoredHandler(auditRepository)); err != nil {
		return err
	}
	if err := router.Register("mail.delivered", audit.NewDeliveryStatusHandler(auditRepository)); err != nil {
		return err
	}
	if err := router.Register("mail.bounced", audit.NewDeliveryStatusHandler(auditRepository)); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_failed", audit.NewDeliveryStatusHandler(auditRepository)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:   redisClient,
		Stream:   cfg.EventStream,
		Group:    cfg.EventConsumerGroup,
		Consumer: cfg.EventConsumerName,
		Count:    int64(cfg.EventConsumerCount),
		Block:    cfg.EventConsumerBlock,
		Handler:  router,
		Logger:   logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"event worker started",
		"stream", cfg.EventStream,
		"group", cfg.EventConsumerGroup,
		"consumer", cfg.EventConsumerName,
		"count", cfg.EventConsumerCount,
		"block", cfg.EventConsumerBlock.String(),
	)
	return consumer.Run(ctx)
}

func runDeliveryWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
	if err := redisClient.Ping(ctx).Err(); err != nil {
		_ = redisClient.Close()
		return err
	}
	defer redisClient.Close()

	transport := delivery.NewDirectSMTPTransport()
	transport.Hello = cfg.DeliverySMTPHello
	transport.Timeout = cfg.DeliveryTimeout
	transport.TLSMode = delivery.DeliveryTLSMode(cfg.DeliveryTLSMode)
	if router := deliveryRouterFromConfig(cfg); router != nil {
		transport.Router = router
		logger.Info("delivery worker using smart-host relay", "host", cfg.DeliverySmartHost, "port", cfg.DeliverySmartHostPort, "tls_mode", cfg.DeliverySmartHostTLSMode, "auth_configured", strings.TrimSpace(cfg.DeliverySmartHostUsername) != "")
	}
	retryPolicy := delivery.DefaultRetryPolicy()
	retryPolicy.Delays = cfg.DeliveryRetryDelays
	retryPolicy.JitterRatio = cfg.DeliveryRetryJitterRatio
	retryPolicy.MaxDelay = cfg.DeliveryRetryMaxDelay
	if cfg.DKIMEnabled {
		repository := maildb.NewRepository(db)
		transport.Transformers = append(transport.Transformers, dkim.Transformer{
			Signer: dkim.RFC6376Signer{KeyProvider: dkimKeyProvider{repository: repository}},
		})
		logger.Info("delivery worker enabled DKIM signing transformer")
	}
	handler := delivery.NewHandler(
		storage.NewLocalStore(cfg.MailstoreRoot),
		transport,
		delivery.NewPostgresRecorder(db),
		delivery.NewPostgresRetryScheduler(db, retryPolicy),
	)
	if cfg.DeliveryThrottleEnabled {
		handler.WithThrottler(delivery.NewInMemoryThrottler(delivery.ThrottlePolicy{
			FarmMaxConcurrent:   deliveryFarmLimits(cfg.DeliveryFarmConcurrency),
			DomainMaxConcurrent: cfg.DeliveryDomainConcurrency,
			DefaultConcurrent:   cfg.DeliveryDefaultConcurrency,
		}))
		logger.Info(
			"delivery throttling enabled",
			"default_concurrency", cfg.DeliveryDefaultConcurrency,
			"farm_limits", cfg.DeliveryFarmConcurrency,
			"domain_limits", cfg.DeliveryDomainConcurrency,
		)
	}
	handler.WithMetrics(deliveryMetrics(cfg, logger))

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:   redisClient,
		Stream:   cfg.DeliveryStream,
		Group:    cfg.DeliveryConsumerGroup,
		Consumer: cfg.DeliveryConsumerName,
		Count:    int64(cfg.DeliveryConsumerCount),
		Block:    cfg.DeliveryConsumerBlock,
		Handler:  handler,
		Logger:   logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"delivery worker started",
		"stream", cfg.DeliveryStream,
		"group", cfg.DeliveryConsumerGroup,
		"consumer", cfg.DeliveryConsumerName,
		"count", cfg.DeliveryConsumerCount,
		"block", cfg.DeliveryConsumerBlock.String(),
	)
	return consumer.Run(ctx)
}

func deliveryRouterFromConfig(cfg config.Config) delivery.Router {
	if strings.TrimSpace(cfg.DeliverySmartHost) == "" {
		return nil
	}
	return delivery.StaticRouter{RouteConfig: delivery.Route{
		Hosts:   []string{cfg.DeliverySmartHost},
		Port:    cfg.DeliverySmartHostPort,
		TLSMode: delivery.DeliveryTLSMode(cfg.DeliverySmartHostTLSMode),
		Auth: delivery.RouteAuth{
			Identity: cfg.DeliverySmartHostIdentity,
			Username: cfg.DeliverySmartHostUsername,
			Password: cfg.DeliverySmartHostPassword,
		},
	}}
}

func deliveryFarmLimits(values map[string]int) map[outbound.Farm]int {
	result := make(map[outbound.Farm]int, len(values))
	for farm, limit := range values {
		result[outbound.Farm(farm)] = limit
	}
	return result
}

func smtpMetrics(cfg config.Config, logger *slog.Logger) smtpd.Metrics {
	if cfg.MetricsBackend == "slog" {
		return observability.NewSlogAdapter(logger)
	}
	return nil
}

func deliveryMetrics(cfg config.Config, logger *slog.Logger) delivery.Metrics {
	if cfg.MetricsBackend == "slog" {
		return observability.NewSlogAdapter(logger)
	}
	return nil
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

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	mux := http.NewServeMux()
	httpapi.RegisterHealthRoutes(mux)

	if mode == ModeMailAPI {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		service := mailservice.New(repository, storage.NewLocalStore(cfg.MailstoreRoot))
		var tokenManager *auth.TokenManager
		if cfg.AuthJWTSecret != "" {
			tokenManager, err = auth.NewTokenManager(cfg.AuthJWTSecret)
			if err != nil {
				return err
			}
		}
		httpapi.RegisterMailRoutes(mux, service, tokenManager)
		logger.Info("mail api routes registered")
	}
	if mode == ModeAdminAPI {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		httpapi.RegisterAdminRoutes(mux, repository, cfg.AdminToken)
		logger.Info("admin api routes registered")
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "mode", mode)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func waitForShutdown(ctx context.Context, logger *slog.Logger, mode Mode) error {
	logger.Info("mode scaffold is ready; component implementation will be added next", "mode", mode)
	<-ctx.Done()
	return nil
}
