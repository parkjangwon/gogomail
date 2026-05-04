package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/attachmentscan"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/dedup"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/dkim"
	dsnpkg "github.com/gogomail/gogomail/internal/dsn"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/imapnotify"
	"github.com/gogomail/gogomail/internal/mailauth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/pushnotify"
	"github.com/gogomail/gogomail/internal/ratelimit"
	"github.com/gogomail/gogomail/internal/searchindex"
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
	case ModeSearchIndexWorker:
		return runSearchIndexWorker(ctx, cfg, logger)
	case ModeAPIMeteringWorker:
		return runAPIMeteringWorker(ctx, cfg, logger)
	case ModeAPIUsageRetention:
		return runAPIUsageRetentionWorker(ctx, cfg, logger)
	case ModePushWorker:
		return runPushNotificationWorker(ctx, cfg, logger)
	case ModeAttachmentCleanup:
		return runAttachmentCleanupWorker(ctx, cfg, logger)
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

type attachmentCleanupRunner interface {
	ExpireStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error)
	ExpireAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error)
}

type attachmentCleanupResult struct {
	ExpiredUploads  int
	ExpiredSessions int
}

type apiUsageRetentionRunner interface {
	RunAPIUsageLedgerRetention(ctx context.Context, req maildb.APIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunView, error)
}

type apiUsageRetentionResult struct {
	RunID          string
	CandidateCount int64
	LimitedCount   int64
	DeletedCount   int64
	Ready          bool
	DryRun         bool
}

func runAttachmentCleanupWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	service := mailservice.New(maildb.NewRepository(db), storage.NewLocalStore(cfg.MailstoreRoot))
	if cfg.AttachmentCleanupRunOnce {
		_, err := cleanupStaleAttachmentUploadsOnce(ctx, service, time.Now, cfg.AttachmentCleanupStaleAge, cfg.AttachmentCleanupBatchSize, logger)
		return err
	}
	return runAttachmentCleanupLoop(ctx, service, cfg.AttachmentCleanupInterval, cfg.AttachmentCleanupStaleAge, cfg.AttachmentCleanupBatchSize, logger)
}

func runAPIUsageRetentionWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	repository := maildb.NewRepository(db)
	if cfg.APIUsageRetentionRunOnce {
		_, err := runAPIUsageRetentionOnce(ctx, repository, time.Now, cfg, logger)
		return err
	}
	if _, err := runAPIUsageRetentionOnce(ctx, repository, time.Now, cfg, logger); err != nil && logger != nil {
		logger.Error("api usage retention failed", "error", err)
	}
	ticker := time.NewTicker(cfg.APIUsageRetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := runAPIUsageRetentionOnce(ctx, repository, time.Now, cfg, logger); err != nil && logger != nil {
				logger.Error("api usage retention failed", "error", err)
			}
		}
	}
}

func runAPIUsageRetentionOnce(ctx context.Context, runner apiUsageRetentionRunner, now func() time.Time, cfg config.Config, logger *slog.Logger) (apiUsageRetentionResult, error) {
	if runner == nil {
		return apiUsageRetentionResult{}, fmt.Errorf("api usage retention runner is required")
	}
	if now == nil {
		now = time.Now
	}
	if cfg.APIUsageRetentionCutoffAge <= 0 {
		return apiUsageRetentionResult{}, fmt.Errorf("api usage retention cutoff age must be positive")
	}
	cutoff := now().UTC().Add(-cfg.APIUsageRetentionCutoffAge)
	run, err := runner.RunAPIUsageLedgerRetention(ctx, maildb.APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     cfg.APIUsageRetentionTenantID,
		PrincipalID:  cfg.APIUsageRetentionPrincipalID,
		Limit:        cfg.APIUsageRetentionBatchSize,
		DryRun:       cfg.APIUsageRetentionDryRun,
		ConfirmReady: cfg.APIUsageRetentionConfirmReady,
	})
	if err != nil {
		return apiUsageRetentionResult{}, err
	}
	result := apiUsageRetentionResult{
		RunID:          run.ID,
		CandidateCount: run.CandidateCount,
		LimitedCount:   run.LimitedCount,
		DeletedCount:   run.DeletedCount,
		Ready:          run.Ready,
		DryRun:         run.DryRun,
	}
	if logger != nil {
		logger.Info(
			"api usage retention completed",
			"run_id", result.RunID,
			"cutoff", cutoff.Format(time.RFC3339),
			"tenant_id", strings.TrimSpace(cfg.APIUsageRetentionTenantID),
			"principal_id", strings.TrimSpace(cfg.APIUsageRetentionPrincipalID),
			"limit", cfg.APIUsageRetentionBatchSize,
			"dry_run", result.DryRun,
			"ready", result.Ready,
			"candidates", result.CandidateCount,
			"limited", result.LimitedCount,
			"deleted", result.DeletedCount,
		)
	}
	return result, nil
}

func runAttachmentCleanupLoop(ctx context.Context, cleaner attachmentCleanupRunner, interval time.Duration, staleAge time.Duration, batchSize int, logger *slog.Logger) error {
	if cleaner == nil {
		return fmt.Errorf("attachment cleanup service is required")
	}
	if interval <= 0 {
		return fmt.Errorf("attachment cleanup interval must be positive")
	}
	if staleAge <= 0 {
		return fmt.Errorf("attachment cleanup stale age must be positive")
	}
	if batchSize <= 0 {
		return fmt.Errorf("attachment cleanup batch size must be positive")
	}

	if _, err := cleanupStaleAttachmentUploadsOnce(ctx, cleaner, time.Now, staleAge, batchSize, logger); err != nil {
		if logger != nil {
			logger.Error("attachment cleanup failed", "error", err)
		}
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := cleanupStaleAttachmentUploadsOnce(ctx, cleaner, time.Now, staleAge, batchSize, logger); err != nil && logger != nil {
				logger.Error("attachment cleanup failed", "error", err)
			}
		}
	}
}

func cleanupStaleAttachmentUploadsOnce(ctx context.Context, cleaner attachmentCleanupRunner, now func() time.Time, staleAge time.Duration, batchSize int, logger *slog.Logger) (attachmentCleanupResult, error) {
	if now == nil {
		now = time.Now
	}
	before := now().UTC().Add(-staleAge)
	expired, err := cleaner.ExpireStaleAttachmentUploads(ctx, before, batchSize)
	if err != nil {
		return attachmentCleanupResult{}, err
	}
	expiredSessions, err := cleaner.ExpireAttachmentUploadSessions(ctx, before, batchSize)
	if err != nil {
		return attachmentCleanupResult{}, err
	}
	result := attachmentCleanupResult{
		ExpiredUploads:  len(expired),
		ExpiredSessions: len(expiredSessions),
	}
	if logger != nil {
		logger.Info("attachment cleanup completed", "expired", result.ExpiredUploads, "expired_sessions", result.ExpiredSessions, "before", before.Format(time.RFC3339), "limit", batchSize)
	}
	return result, nil
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
	var maildbRepo *maildb.Repository

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

		maildbRepo = maildb.NewRepository(db)
		resolver = maildbRepo
		recorder = maildbRepo
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
	attachmentHooks, err := attachmentScanHooksForConfig(cfg, logger, opts.Component)
	if err != nil {
		return err
	}
	hooks = append(hooks, attachmentHooks...)

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:              storage.NewLocalStore(cfg.MailstoreRoot),
		Resolver:           resolver,
		Recorder:           recorder,
		Deduplicator:       deduplicator,
		RateLimiter:        rateLimiter,
		Backpressure:       pressure,
		AuthVerifier:       authVerifier,
		RelayAuthorizer:    relayAuthorizer,
		DomainPolicyLookup: maildbRepo,
		Metrics:            smtpMetrics(cfg, logger),
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
	hooks, err := attachmentScanHooksForConfig(cfg, logger, "outbound submission mta")
	if err != nil {
		return err
	}
	receiver := smtpd.NewSubmissionReceiver(smtpd.SubmissionOptions{
		Store:              storage.NewLocalStore(cfg.MailstoreRoot),
		Authenticator:      repository,
		Recorder:           repository,
		DomainPolicyLookup: repository,
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
	default:
		return nil, fmt.Errorf("unsupported attachment scanner backend %q", cfg.AttachmentScanBackend)
	}
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
	if err := router.Register("mail.stored", eventstream.MultiHandler{
		imapnotify.NewMailStoredHandler(maildb.NewRepository(db)),
		audit.NewMailStoredHandler(auditRepository),
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivered", audit.NewDeliveryStatusHandler(auditRepository)); err != nil {
		return err
	}
	if err := router.Register("mail.bounced", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		dsnpkg.NewBounceHandler(dsnpkg.HandlerOptions{
			Store:        storage.NewLocalStore(cfg.MailstoreRoot),
			Queue:        dsnpkg.NewPostgresOutboxQueue(db),
			ReportingMTA: cfg.SMTPDomain,
			Postmaster:   cfg.DSNPostmaster,
			Farm:         outbound.FarmGeneral,
		}),
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_failed", audit.NewDeliveryStatusHandler(auditRepository)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:    redisClient,
		Stream:    cfg.EventStream,
		Group:     cfg.EventConsumerGroup,
		Consumer:  cfg.EventConsumerName,
		Count:     int64(cfg.EventConsumerCount),
		Block:     cfg.EventConsumerBlock,
		ClaimIdle: cfg.EventConsumerClaimIdle,
		Handler:   router,
		Logger:    logger,
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

func runSearchIndexWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "disabled") {
		return waitForShutdown(ctx, logger, ModeSearchIndexWorker)
	}

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

	repository := maildb.NewRepository(db)
	indexer, err := searchIndexerForConfig(cfg, repository)
	if err != nil {
		return err
	}
	if err := maybeBootstrapSearchIndex(ctx, cfg, indexer); err != nil {
		return err
	}
	router := eventstream.NewRouter()
	if err := router.Register("mail.stored", searchindex.NewHandler(
		searchindex.NewStorageStoreReader(storage.NewLocalStore(cfg.MailstoreRoot)),
		indexer,
		searchindex.HandlerOptions{MaxTextBodyBytes: cfg.SearchIndexMaxBodyBytes},
	)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:    redisClient,
		Stream:    cfg.EventStream,
		Group:     cfg.SearchIndexConsumerGroup,
		Consumer:  cfg.SearchIndexConsumerName,
		Count:     int64(cfg.SearchIndexConsumerCount),
		Block:     cfg.SearchIndexConsumerBlock,
		ClaimIdle: cfg.SearchIndexConsumerClaimIdle,
		Handler:   router,
		Logger:    logger,
	})
	if err != nil {
		return err
	}

	logger.Info("search index worker started", searchIndexWorkerLogFields(cfg)...)
	return consumer.Run(ctx)
}

func searchIndexWorkerLogFields(cfg config.Config) []any {
	fields := []any{
		"stream", cfg.EventStream,
		"group", cfg.SearchIndexConsumerGroup,
		"consumer", cfg.SearchIndexConsumerName,
		"backend", strings.ToLower(strings.TrimSpace(cfg.SearchIndexBackend)),
		"max_body_bytes", cfg.SearchIndexMaxBodyBytes,
	}
	if strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		fields = append(fields,
			"opensearch_index", strings.TrimSpace(cfg.SearchIndexOpenSearchIndex),
			"opensearch_bootstrap", cfg.SearchIndexOpenSearchBootstrap,
		)
	}
	return fields
}

func searchIndexerForConfig(cfg config.Config, repository *maildb.Repository) (searchindex.Indexer, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.SearchIndexBackend)) {
	case "postgres":
		return searchindex.NewPostgresIndexer(repository), nil
	case "opensearch":
		return searchindex.NewOpenSearchIndexer(openSearchOptionsForConfig(cfg))
	default:
		return nil, fmt.Errorf("unsupported search index backend %q", cfg.SearchIndexBackend)
	}
}

type searchIndexBootstrapper interface {
	EnsureIndex(ctx context.Context) error
}

func maybeBootstrapSearchIndex(ctx context.Context, cfg config.Config, indexer any) error {
	if !cfg.SearchIndexOpenSearchBootstrap {
		return nil
	}
	if !strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		return nil
	}
	bootstrapper, ok := indexer.(searchIndexBootstrapper)
	if !ok {
		return fmt.Errorf("search index backend %q does not support bootstrap", cfg.SearchIndexBackend)
	}
	return bootstrapper.EnsureIndex(ctx)
}

func searchIDSourceForConfig(cfg config.Config) (mailservice.SearchIDSource, error) {
	if !strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		return nil, nil
	}
	return searchindex.NewOpenSearchSearcher(openSearchOptionsForConfig(cfg))
}

func openSearchOptionsForConfig(cfg config.Config) searchindex.OpenSearchOptions {
	timeout := cfg.SearchIndexOpenSearchTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return searchindex.OpenSearchOptions{
		Endpoint: cfg.SearchIndexOpenSearchEndpoint,
		Index:    cfg.SearchIndexOpenSearchIndex,
		Client:   &http.Client{Timeout: timeout},
		Username: cfg.SearchIndexOpenSearchUsername,
		Password: cfg.SearchIndexOpenSearchPassword,
	}
}

func runAPIMeteringWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if strings.EqualFold(strings.TrimSpace(cfg.APIMeteringAggregateBackend), "disabled") {
		return waitForShutdown(ctx, logger, ModeAPIMeteringWorker)
	}

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
	if err := router.Register(apimeter.EventAPIUsage, apimeter.NewUsageHandler(apimeter.NewPostgresAggregateStore(db))); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:    redisClient,
		Stream:    cfg.APIMeteringStream,
		Group:     cfg.APIMeteringConsumerGroup,
		Consumer:  cfg.APIMeteringConsumerName,
		Count:     int64(cfg.APIMeteringConsumerCount),
		Block:     cfg.APIMeteringConsumerBlock,
		ClaimIdle: cfg.APIMeteringConsumerClaimIdle,
		Handler:   router,
		Logger:    logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"api metering worker started",
		"stream", cfg.APIMeteringStream,
		"group", cfg.APIMeteringConsumerGroup,
		"consumer", cfg.APIMeteringConsumerName,
		"backend", cfg.APIMeteringAggregateBackend,
	)
	return consumer.Run(ctx)
}

func runPushNotificationWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	backend := strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend))
	if backend == "" || backend == "none" {
		return waitForShutdown(ctx, logger, ModePushWorker)
	}
	sink, err := pushNotificationSinkForConfig(cfg, logger)
	if err != nil {
		return err
	}

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

	repository := maildb.NewRepository(db)
	pushRecorder := pushnotify.NewPostgresRecorder(db)
	router := eventstream.NewRouter()
	if err := router.Register(pushnotify.EventMailStored, pushnotify.NewHandler(
		sink,
		pushnotify.WithTargetResolver(pushnotify.NewDeviceResolver(repository, cfg.PushNotifyDeviceLimit)),
		pushnotify.WithCandidateRecorder(pushRecorder),
		pushnotify.WithOutcomeRecorder(pushRecorder),
	)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:    redisClient,
		Stream:    cfg.EventStream,
		Group:     cfg.PushNotifyConsumerGroup,
		Consumer:  cfg.PushNotifyConsumerName,
		Count:     int64(cfg.PushNotifyConsumerCount),
		Block:     cfg.PushNotifyConsumerBlock,
		ClaimIdle: cfg.PushNotifyConsumerClaimIdle,
		Handler:   router,
		Logger:    logger,
	})
	if err != nil {
		return err
	}

	logger.Info(
		"push notification worker started",
		"stream", cfg.EventStream,
		"group", cfg.PushNotifyConsumerGroup,
		"consumer", cfg.PushNotifyConsumerName,
		"backend", backend,
		"device_limit", cfg.PushNotifyDeviceLimit,
	)
	return consumer.Run(ctx)
}

func pushNotificationSinkForConfig(cfg config.Config, logger *slog.Logger) (pushnotify.Sink, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.PushNotifyBackend)) {
	case "slog":
		return pushnotify.SlogSink{Logger: logger}, nil
	case "webhook":
		return pushnotify.NewWebhookSink(pushnotify.WebhookOptions{
			Endpoint: strings.TrimSpace(cfg.PushNotifyWebhookURL),
			Token:    cfg.PushNotifyWebhookToken,
			Client:   &http.Client{Timeout: cfg.PushNotifyWebhookTimeout},
		})
	default:
		return nil, errors.New("unsupported push notification backend")
	}
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
	repository := maildb.NewRepository(db)
	if strings.EqualFold(strings.TrimSpace(cfg.DeliveryRouteBackend), "postgres") {
		transport.Router = postgresDeliveryRouter{repository: repository, fallbackTLSMode: delivery.DeliveryTLSMode(cfg.DeliveryTLSMode)}
		logger.Info("delivery worker using postgres delivery routes")
	} else if router := deliveryRouterFromConfig(cfg); router != nil {
		transport.Router = router
		logger.Info("delivery worker using smart-host relay", "host", cfg.DeliverySmartHost, "port", cfg.DeliverySmartHostPort, "tls_mode", cfg.DeliverySmartHostTLSMode, "implicit_tls", cfg.DeliverySmartHostImplicitTLS, "auth_configured", strings.TrimSpace(cfg.DeliverySmartHostUsername) != "")
	}
	retryPolicy := delivery.DefaultRetryPolicy()
	retryPolicy.Delays = cfg.DeliveryRetryDelays
	retryPolicy.JitterRatio = cfg.DeliveryRetryJitterRatio
	retryPolicy.MaxDelay = cfg.DeliveryRetryMaxDelay
	if cfg.DKIMEnabled {
		transport.Transformers = append(transport.Transformers, dkim.Transformer{
			Signer: dkim.RFC6376Signer{KeyProvider: dkimKeyProvider{repository: repository}},
		})
		logger.Info("delivery worker enabled DKIM signing transformer")
	}
	deliveryRecorder := delivery.NewPostgresRecorder(db)
	handler := delivery.NewHandler(
		storage.NewLocalStore(cfg.MailstoreRoot),
		transport,
		deliveryRecorder,
		delivery.NewPostgresRetryScheduler(db, retryPolicy),
	).WithExhaustionHook(deliveryRecorder)
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
		Client:    redisClient,
		Stream:    cfg.DeliveryStream,
		Group:     cfg.DeliveryConsumerGroup,
		Consumer:  cfg.DeliveryConsumerName,
		Count:     int64(cfg.DeliveryConsumerCount),
		Block:     cfg.DeliveryConsumerBlock,
		ClaimIdle: cfg.DeliveryConsumerClaimIdle,
		Handler:   handler,
		Logger:    logger,
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

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	mux := http.NewServeMux()
	httpapi.RegisterHealthRoutes(mux)

	var tokenManager *auth.TokenManager
	if mode == ModeMailAPI {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()

		repository := maildb.NewRepository(db)
		service := mailservice.New(repository, storage.NewLocalStore(cfg.MailstoreRoot))
		searchIDSource, err := searchIDSourceForConfig(cfg)
		if err != nil {
			return err
		}
		if searchIDSource != nil {
			service.WithSearchIDSource(searchIDSource)
		}
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

		var redisClient *redis.Client
		var pressure backpressureStore
		if cfg.BackpressureBackend == "redis" {
			redisClient = redis.NewClient(&redis.Options{Addr: cfg.RedisAddr})
			if err := redisClient.Ping(ctx).Err(); err != nil {
				_ = redisClient.Close()
				return err
			}
			defer redisClient.Close()
			pressure = backpressure.NewRedisBackpressure(redisClient, backpressure.DefaultStateKey)
		}

		repository := maildb.NewRepository(db)
		httpapi.RegisterAdminRoutes(mux, adminService{
			Repository:                  repository,
			backpressure:                pressure,
			exportStore:                 storage.NewLocalStore(cfg.MailstoreRoot),
			exportManifestSigner:        apiUsageExportManifestSigner(cfg),
			exportManifestSignerBackend: cfg.APIUsageExportManifestSignerBackend,
			exportManifestVerifier:      apiUsageExportManifestVerifier(cfg),
			attachmentCleanup:           mailservice.New(repository, storage.NewLocalStore(cfg.MailstoreRoot)),
		}, cfg.AdminToken)
		logger.Info("admin api routes registered")
	}

	var meteringDB *sql.DB
	if strings.EqualFold(strings.TrimSpace(cfg.APIMeteringBackend), "outbox") {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		meteringDB = db
	}

	handler := apiMeteringHandler(mux, cfg, logger, meteringDB, tokenManager, cfg.AdminToken)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
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
			UserID:      r.URL.Query().Get("user_id"),
			APIKeyID:    r.Header.Get("X-Gogomail-API-Key-ID"),
			PrincipalID: r.Header.Get("X-Gogomail-Principal-ID"),
			AuthSource:  apimeter.AuthSourceAnonymous,
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

func waitForShutdown(ctx context.Context, logger *slog.Logger, mode Mode) error {
	logger.Info("mode scaffold is ready; component implementation will be added next", "mode", mode)
	<-ctx.Done()
	return nil
}
