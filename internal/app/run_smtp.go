package app

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/attachmentscan"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/dedup"
	"github.com/gogomail/gogomail/internal/dnsbl"
	"github.com/gogomail/gogomail/internal/mailauth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/milterhook"
	"github.com/gogomail/gogomail/internal/ratelimit"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/spamfilter"
)

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
	var maildbRepo *maildb.Repository
	var runtimeConfigStore *configstore.PostgresConfigStore
	var mailFlowWriter *maildb.MailFlowLogWriter

	if len(cfg.LocalRecipients) > 0 {
		staticResolver, err := smtpd.StaticResolverFromRecipients(cfg.LocalRecipients)
		if err != nil {
			return err
		}
		resolver = staticResolver
		logger.Info(opts.Component+" using static recipient resolver", "recipients", len(cfg.LocalRecipients))
	} else {
		db, err := openDatabase(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		maildbRepo = maildb.NewRepository(db)
		resolver = maildbRepo
		recorder = maildbRepo
		runtimeConfigStore = configstore.NewPostgresConfigStore(db)
		if err := runtimeConfigStore.Start(ctx); err != nil {
			return fmt.Errorf("start receive mta config store: %w", err)
		}
		mailFlowWriter = maildb.NewMailFlowLogWriter(db)
		logger.Info(opts.Component + " using database recipient resolver and message recorder")
	}

	if opts.EnableDedup && cfg.DedupBackend == "redis" {
		redisClient = newRedisClient(cfg)
		if err := redisClient.Ping(ctx).Err(); err != nil {
			if err := redisClient.Close(); err != nil {
				logger.Warn("close redis client", "error", err)
			}
			return err
		}

		deduplicator = dedup.NewRedisDeduplicator(redisClient, 24*time.Hour)
		logger.Info(opts.Component+" using redis deduplicator", "addr", cfg.RedisAddr)
	}
	if opts.EnableRateLimit && cfg.RateLimitBackend == "redis" {
		if redisClient == nil {
			redisClient = newRedisClient(cfg)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				if err := redisClient.Close(); err != nil {
					logger.Warn("close redis client", "error", err)
				}
				return err
			}
		}
		rateLimiter = ratelimit.NewRedisLimiter(redisClient, int64(cfg.RcptRateLimitPerMinute), time.Minute)
		logger.Info(opts.Component+" using redis rate limiter", "addr", cfg.RedisAddr, "rcpt_per_minute", cfg.RcptRateLimitPerMinute)
	}
	if opts.EnableBackpressure && cfg.BackpressureBackend == "redis" {
		if redisClient == nil {
			redisClient = newRedisClient(cfg)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				if err := redisClient.Close(); err != nil {
					logger.Warn("close redis client", "error", err)
				}
				return err
			}
		}
		pressure = backpressure.NewRedisBackpressure(redisClient, backpressure.DefaultStateKey)
		logger.Info(opts.Component+" using redis backpressure", "addr", cfg.RedisAddr)
	}
	if redisClient != nil {
		defer redisClient.Close()
	}

	// Auto backpressure manager
	if opts.EnableBackpressure && cfg.AutoBackpressureEnabled && redisClient != nil {
		if redisBP, ok := pressure.(*backpressure.RedisBackpressure); ok {
			monitorStreams := []string{}
			if cfg.DeliveryStream != "" {
				monitorStreams = append(monitorStreams, cfg.DeliveryStream)
			}
			autoMgr := backpressure.NewAutoBackpressureManager(redisClient, redisBP, backpressure.AutoBackpressureConfig{
				CheckInterval:      cfg.AutoBackpressureCheckInterval,
				WarningThreshold:   cfg.AutoBackpressureMemWarn,
				DangerThreshold:    cfg.AutoBackpressureMemDanger,
				CriticalThreshold:  cfg.AutoBackpressureMemCritical,
				QueueWarningDepth:  cfg.AutoBackpressureQueueWarn,
				QueueDangerDepth:   cfg.AutoBackpressureQueueDanger,
				QueueCriticalDepth: cfg.AutoBackpressureQueueCritical,
				MonitorStreams:     monitorStreams,
				InstanceID:         cfg.AutoBackpressureInstanceID,
			})
			autoCtx, autoCancel := context.WithCancel(ctx)
			defer autoCancel()
			autoMgr.Start(autoCtx)
			logger.Info(opts.Component + " auto backpressure manager started")
		}
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
	attachmentHooks, err := attachmentScanHooksForConfig(cfg, logger, opts.Component)
	if err != nil {
		return err
	}
	hooks = append(hooks, attachmentHooks...)
	if runtimeConfigStore != nil {
		engine := spamfilter.NewEngine()
		hooks = append(hooks, spamfilter.Hook(spamfilter.Options{
			Resolver: runtimeConfigStore,
			Logger:   mailFlowWriter,
			Engine:   engine,
		}))
		recorder = spamfilter.Recorder{
			Next:     recorder,
			Resolver: runtimeConfigStore,
			Logger:   mailFlowWriter,
			Engine:   engine,
		}
		logger.Info(opts.Component + " built-in spam filter enabled")
	}
	if cfg.MilterEnabled {
		hooks = append(hooks, milterhook.Hook(milterhook.HookOptions{
			Dialer:     milterhook.PoolDialer(cfg.MilterAddr, cfg.MilterTimeout, cfg.MilterMaxConns),
			ShadowMode: cfg.MilterShadowMode,
		}))
		logger.Info(opts.Component+" milter filter enabled", "addr", cfg.MilterAddr, "timeout", cfg.MilterTimeout, "maxConns", cfg.MilterMaxConns, "shadowMode", cfg.MilterShadowMode)
	}
	if len(cfg.DNSBLZones) > 0 {
		resolver := dnsbl.NewResolverWithTimeout(cfg.DNSBLTimeout)
		checker := dnsbl.NewChecker(cfg.DNSBLZones, resolver)
		hooks = append(hooks, dnsbl.Hook(dnsbl.HookOptions{
			Checker: checker,
			Policy:  dnsbl.Policy(cfg.DNSBLPolicy),
			Logger:  logger,
		}))
		logger.Info(opts.Component+" DNSBL enabled", "zones", cfg.DNSBLZones, "policy", cfg.DNSBLPolicy)
	}
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}

	var latencyTracker *smtpd.LatencyTracker
	if cfg.SMTPLatencyTrackingEnabled {
		latencyTracker = smtpd.NewLatencyTracker(cfg.SMTPLatencyWindowSize)
		logger.Info(opts.Component + " SMTP latency tracking enabled")
	}

	// Guard against the nil-interface trap: a typed nil *maildb.Repository assigned
	// to a DomainPolicyLookup interface is not a nil interface, so the receiver's
	// nil-check won't fire and the method call panics.  Only wire it up when the
	// repo was actually created (i.e. when DB mode, not static-resolver mode).
	var domainPolicyLookup smtpd.DomainPolicyLookup
	if maildbRepo != nil {
		domainPolicyLookup = maildbRepo
	}

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:              store,
		Resolver:           resolver,
		Recorder:           recorder,
		Deduplicator:       deduplicator,
		RateLimiter:        rateLimiter,
		Backpressure:       pressure,
		AuthVerifier:       authVerifier,
		RelayAuthorizer:    relayAuthorizer,
		DomainPolicyLookup: domainPolicyLookup,
		Metrics:            smtpMetrics(cfg, logger),
		Logger:             logger,
		AddReceivedHeader:  cfg.SMTPAddReceivedHeader,
		ReceivedDomain:     cfg.SMTPDomain,
		RequireAuth:        cfg.SMTPRequireAuth,
		SupportSMTPUTF8:    cfg.SMTPSupportSMTPUTF8,
		SupportRequireTLS:  cfg.SMTPSupportRequireTLS,
		SupportDSN:         cfg.SMTPSupportDSN,
		SupportBinaryMIME:  cfg.SMTPSupportBinaryMIME,
		Hooks:              hooks,
		Policy: smtpd.ReceivePolicy{
			MaxRecipientsPerMessage: cfg.SMTPMaxRecipients,
			MaxMessageBytes:         cfg.SMTPMaxMessageBytes,
		},
		LatencyTracker: latencyTracker,
	})

	tlsCfg, err := smtpTLSConfig(cfg)
	if err != nil {
		return fmt.Errorf("receive MTA TLS config: %w", err)
	}

	receiver.SetBaseContext(ctx)

	go serveMetrics(ctx, cfg, logger)

	return smtpd.RunServer(ctx, smtpd.ServerOptions{
		Addr:             opts.Addr,
		Domain:           cfg.SMTPDomain,
		Receiver:         receiver,
		Logger:           logger,
		TLSConfig:        tlsCfg,
		ReadTimeout:      cfg.SMTPReadTimeout,
		WriteTimeout:     cfg.SMTPWriteTimeout,
		MaxConnections:   cfg.SMTPMaxConnections,
		MaxMessageBytes:  cfg.SMTPMaxMessageBytes,
		MaxRecipients:    cfg.SMTPMaxRecipients,
		EnableSMTPUTF8:   cfg.SMTPSupportSMTPUTF8,
		EnableDSN:        cfg.SMTPSupportDSN,
		EnableRequireTLS: cfg.SMTPSupportRequireTLS,
		EnableBinaryMIME: cfg.SMTPSupportBinaryMIME,
	})
}

func farmCoordinatorFromConfig(cfg config.Config, redisClient *redis.Client) smtpd.FarmCoordinator {
	if strings.EqualFold(cfg.FarmCoordinatorBackend, "redis") && redisClient != nil {
		return smtpd.NewRedisFarmCoordinator(redisClient, smtpd.RedisFarmCoordinatorOptions{
			NodeHeartbeatTTL:     cfg.FarmCoordinatorHeartbeatTTL,
			JobVisibilityTimeout: cfg.FarmCoordinatorJobVisibilityTimeout,
		})
	}
	return smtpd.NewNoOpFarmCoordinator()
}

func runSubmissionMTA(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
	if err != nil {
		return err
	}
	defer db.Close()

	var bulkLimiter *smtpd.BulkSenderLimiter
	if cfg.SubmissionBulkSenderEnabled {
		bulkLimiter = smtpd.NewBulkSenderLimiter(cfg.SubmissionBulkSenderRate, cfg.SubmissionBulkSenderRole)
		logger.Info("submission bulk sender limiter enabled",
			"rate", cfg.SubmissionBulkSenderRate,
			"role", cfg.SubmissionBulkSenderRole,
		)
	}

	repository := maildb.NewRepository(db)
	hooks, err := attachmentScanHooksForConfig(cfg, logger, "outbound submission mta")
	if err != nil {
		return err
	}
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	receiver := smtpd.NewSubmissionReceiver(smtpd.SubmissionOptions{
		Store:              store,
		Authenticator:      repository,
		Recorder:           repository,
		DomainPolicyLookup: repository,
		Logger:             logger,
		Hooks:              hooks,
		AddReceivedHeader:  cfg.SubmissionAddReceivedHeader,
		ReceivedDomain:     cfg.SMTPDomain,
		SupportSMTPUTF8:    cfg.SubmissionSupportSMTPUTF8,
		SupportRequireTLS:  cfg.SubmissionSupportRequireTLS,
		SupportDSN:         cfg.SubmissionSupportDSN,
		SupportBinaryMIME:  cfg.SubmissionSupportBinaryMIME,
		Policy: smtpd.ReceivePolicy{
			MaxRecipientsPerMessage: cfg.SubmissionMaxRecipients,
			MaxMessageBytes:         cfg.SubmissionMaxMessageBytes,
		},
		BulkSenderLimiter: bulkLimiter,
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
		MaxConnections:    cfg.SubmissionMaxConnections,
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

func imapTLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.IMAPTLSCertFile == "" && cfg.IMAPTLSKeyFile == "" {
		return nil, nil
	}
	if cfg.IMAPTLSCertFile == "" || cfg.IMAPTLSKeyFile == "" {
		return nil, errors.New("both IMAP TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.IMAPTLSCertFile, cfg.IMAPTLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
		ServerName:   imapTLSServerName(cfg),
	}, nil
}

func imapTLSServerName(cfg config.Config) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(cfg.IMAPAddr))
	if err == nil && strings.TrimSpace(host) != "" {
		return strings.Trim(host, "[]")
	}
	return strings.TrimSpace(cfg.SMTPDomain)
}

func attachmentScanHooksForConfig(cfg config.Config, logger *slog.Logger, component string) ([]smtpd.Hook, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.AttachmentScanBackend)) {
	case "", "none":
		return nil, nil
	case "webhook":
		scanner, err := attachmentscan.NewWebhookScanner(attachmentscan.WebhookOptions{
			Endpoint: strings.TrimSpace(cfg.AttachmentScanWebhookURL),
			Token:    cfg.AttachmentScanWebhookToken,
			Client:   &http.Client{Timeout: cfg.AttachmentScanTimeout},
		})
		if err != nil {
			return nil, err
		}
		if logger != nil {
			logger.Info(component+" attachment scanner configured", "backend", "webhook", "timeout", cfg.AttachmentScanTimeout.String())
		}
		return []smtpd.Hook{attachmentscan.Hook(attachmentscan.HookOptions{Scanner: scanner})}, nil
	case "clamav":
		scanner, err := attachmentscan.NewClamAVScanner(attachmentscan.ClamAVOptions{
			Addr:                cfg.AttachmentScanClamAVAddr,
			Timeout:             cfg.AttachmentScanTimeout,
			MaxConcurrency:      cfg.AttachmentScanMaxConcurrency,
			MaxScanBytes:        cfg.AttachmentScanMaxBytes,
			FailureThreshold:    cfg.AttachmentScanFailureThreshold,
			CircuitOpenDuration: cfg.AttachmentScanCircuitOpenDuration,
		})
		if err != nil {
			return nil, err
		}
		if logger != nil {
			logger.Info(component+" attachment scanner configured", "backend", "clamav", "addr", cfg.AttachmentScanClamAVAddr, "timeout", cfg.AttachmentScanTimeout.String())
		}
		return []smtpd.Hook{attachmentscan.StreamHook(attachmentscan.StreamHookOptions{Scanner: scanner})}, nil
	default:
		return nil, fmt.Errorf("unsupported attachment scanner backend %q", cfg.AttachmentScanBackend)
	}
}
