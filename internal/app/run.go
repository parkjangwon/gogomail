package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	gosmtp "github.com/emersion/go-smtp"
	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/accesspolicy"
	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/apimeter"
	"github.com/gogomail/gogomail/internal/attachmentscan"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/batchlock"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/dedup"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/deltasync"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/dkim"
	"github.com/gogomail/gogomail/internal/dnsbl"
	"github.com/gogomail/gogomail/internal/drive"
	dsnpkg "github.com/gogomail/gogomail/internal/dsn"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/imapgw"
	"github.com/gogomail/gogomail/internal/imapnotify"
	"github.com/gogomail/gogomail/internal/inboundfilter"
	"github.com/gogomail/gogomail/internal/ldapgw"
	"github.com/gogomail/gogomail/internal/mailauth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/milterhook"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/orgchart"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/pop3d"
	"github.com/gogomail/gogomail/internal/pushnotify"
	"github.com/gogomail/gogomail/internal/ratelimit"
	"github.com/gogomail/gogomail/internal/scheduling"
	"github.com/gogomail/gogomail/internal/scim"
	"github.com/gogomail/gogomail/internal/searchindex"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/spamfilter"
	"github.com/gogomail/gogomail/internal/storage"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
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
	case ModeIMAP:
		return runIMAPGateway(ctx, cfg, logger)
	case ModePOP3:
		return runPOP3Gateway(ctx, cfg, logger)
	case ModeCalDAV:
		return runCalDAVGateway(ctx, cfg, logger)
	case ModeCardDAV:
		return runCardDAVGateway(ctx, cfg, logger)
	case ModeWebDAV:
		return runWebDAVGateway(ctx, cfg, logger)
	case ModeLDAPGateway:
		return runLDAPGateway(ctx, cfg, logger)
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
	case ModeDriveCleanup:
		return runDriveCleanupWorker(ctx, cfg, logger)
	case ModeDAVSyncRetention:
		return runDAVSyncRetentionWorker(ctx, cfg, logger)
	case ModeDeliveryWorker:
		return runDeliveryWorker(ctx, cfg, logger)
	case ModeOutboundMTA:
		return runSubmissionMTA(ctx, cfg, logger)
	case ModeBatchWorker:
		return runBatchWorker(ctx, cfg, logger)
	default:
		return errors.New("unsupported mode")
	}
}

func runBatchWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	registry := batchlock.NewJobRegistry()

	scheduledMailRepository := maildb.NewRepository(db)
	registry.Register("scheduled-mail-flusher", func() error {
		stuck, err := scheduledMailRepository.CountStuckScheduledMail(ctx, 10*time.Minute)
		if err != nil {
			logger.Error("scheduled mail flusher check failed", "error", err)
			return err
		}
		if stuck > 0 {
			logger.Warn("scheduled mail entries are stuck in outbox", "count", stuck)
		} else {
			logger.Info("scheduled mail flusher: no stuck entries")
		}
		return nil
	}, 5*time.Minute)

	quotaAlertRepository := maildb.NewRepository(db)
	registry.Register("quota-alert-check", func() error {
		n, err := quotaAlertRepository.ScanAndRecordQuotaAlerts(ctx, 0.80, 0.95)
		if err != nil {
			logger.Error("quota alert check failed", "error", err)
			return err
		}
		logger.Info("quota alert check completed", "alerts", n)
		return nil
	}, 15*time.Minute)

	mfaGraceRepository := maildb.NewRepository(db)
	registry.Register("mfa-grace-period", func() error {
		expired, err := mfaGraceRepository.FindExpiredMFAGraceUsers(ctx, 500)
		if err != nil {
			logger.Error("mfa grace period check failed", "error", err)
			return err
		}
		for _, userID := range expired {
			logger.Warn("MFA grace period expired, enforcement pending", "user_id", userID)
			if err := mfaGraceRepository.ClearMFAGraceDeadline(ctx, userID); err != nil {
				logger.Error("clear mfa grace deadline failed", "user_id", userID, "error", err)
			}
		}
		logger.Info("mfa grace period check completed", "enforced", len(expired))
		return nil
	}, 1*time.Hour)

	tokenCleanupRepository := maildb.NewRepository(db)
	registry.Register("token-cleanup", func() error {
		cutoff := time.Now().UTC()
		na, err := tokenCleanupRepository.PruneExpiredAttachmentShareLinks(ctx, cutoff)
		if err != nil {
			logger.Error("attachment share link cleanup failed", "error", err)
			return err
		}
		nd, err := tokenCleanupRepository.PruneExpiredDriveShareLinks(ctx, cutoff)
		if err != nil {
			logger.Error("drive share link cleanup failed", "error", err)
			return err
		}
		logger.Info("token cleanup completed", "attachment_share_links", na, "drive_share_links", nd)
		return nil
	}, 30*time.Minute)

	totpRepository := maildb.NewRepository(db)
	registry.Register("used-code-cleanup", func() error {
		cutoff := time.Now().UTC().Add(-5 * time.Minute)
		n, err := totpRepository.PruneExpiredTOTPCodes(ctx, cutoff)
		if err != nil {
			logger.Error("TOTP code cleanup failed", "error", err)
			return err
		}
		logger.Info("TOTP code cleanup completed", "pruned", n)
		return nil
	}, 5*time.Minute)

	var orgChartAdapter orgchart.OrgChartSyncAdapter = orgchart.NoopOrgChartAdapter{}
	registry.Register("org-chart-sync", func() error {
		if err := orgChartAdapter.SyncOrgChart(ctx); err != nil {
			logger.Error("org chart sync failed", "error", err)
			return err
		}
		logger.Info("org chart sync completed")
		return nil
	}, 1*time.Hour)

	worker := batchlock.NewWorker(registry, db)
	worker.Start()

	logger.Info("batch worker started", "jobs", len(registry.List()))

	<-ctx.Done()

	logger.Info("shutting down batch worker")
	worker.Stop()
	logger.Info("batch worker stopped")

	return ctx.Err()
}

type attachmentCleanupRunner interface {
	ExpireStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error)
	ExpireAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error)
}

type attachmentCleanupResult struct {
	ExpiredUploads  int
	ExpiredSessions int
}

type driveCleanupRunner interface {
	ExpireUploadSessions(ctx context.Context, req drive.ExpireUploadSessionsRequest) ([]drive.UploadSession, error)
	RetryObjectCleanupFailures(ctx context.Context, req drive.ListObjectCleanupFailuresRequest) (drive.RetryObjectCleanupFailuresResult, error)
}

type driveCleanupResult struct {
	ExpiredSessions int
	ObjectCleanup   drive.RetryObjectCleanupFailuresResult
}

type apiUsageRetentionRunner interface {
	RunAPIUsageLedgerRetention(ctx context.Context, req maildb.APIUsageLedgerRetentionRunRequest) (maildb.APIUsageLedgerRetentionRunView, error)
}

type calDAVSyncRetentionRunner interface {
	PruneCalendarSyncChanges(ctx context.Context, req caldavgw.PruneCalendarSyncChangesRequest) (caldavgw.CalendarSyncChangePruneResult, error)
}

type cardDAVSyncRetentionRunner interface {
	PruneAddressBookChanges(ctx context.Context, req carddavgw.PruneAddressBookChangesRequest) (carddavgw.AddressBookChangePruneResult, error)
}

type davSyncRetentionAuditRecorder interface {
	RecordRun(ctx context.Context, record davsyncretention.RunRecord) (davsyncretention.RunRecord, error)
}

type davSyncRetentionRunners struct {
	CalDAV  calDAVSyncRetentionRunner
	CardDAV cardDAVSyncRetentionRunner
	Audit   davSyncRetentionAuditRecorder
}

type davSyncRetentionResult struct {
	Cutoff         time.Time
	Limit          int
	DryRun         bool
	ConfirmReady   bool
	RunID          string
	Status         davsyncretention.RunStatus
	CalCandidates  int64
	CalDeleted     int64
	CardCandidates int64
	CardDeleted    int64
}

type apiUsageRetentionResult struct {
	RunID          string
	CandidateCount int64
	LimitedCount   int64
	DeletedCount   int64
	Ready          bool
	DryRun         bool
}

type imapGatewayRuntime struct {
	service *mailservice.Service
	store   mailservice.IMAPStoreAdapter
	backend mailservice.IMAPBackendAdapter
	events  *imapgw.MailboxEventBroker
	fanOut  *deltasync.FanOut
}

// fanOutAdapter implements imapnotify.DeltaSyncNotifier using deltasync.FanOut.
type fanOutAdapter struct {
	fanOut *deltasync.FanOut
}

func (a *fanOutAdapter) NotifyMailboxChange(mailboxID string, version int64) {
	a.fanOut.Notify(mailboxID, deltasync.Event{
		MailboxID: mailboxID,
		Type:      "mail.stored",
		Version:   version,
	})
}

type imapServerOptions struct {
	Addr              string
	Backend           imapgw.Backend
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
	MaxConnections    int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

func newIMAPGatewayRuntime(cfg config.Config, repository mailservice.Repository, store storage.Store, authenticator smtpd.SubmissionAuthenticator, quotaAlertEmitter maildb.QuotaWarningEmitterInterface) imapGatewayRuntime {
	events := imapgw.NewMailboxEventBroker(32)
	fanOut := deltasync.NewFanOut()
	service := mailservice.New(repository, store).
		WithMessageBodyCache(cfg.MessageBodyCacheEntries, cfg.MessageBodyCacheTTL).
		WithIMAPMailboxEvents(events)
	if quotaAlertEmitter != nil {
		service = service.WithQuotaAlertEmitter(quotaAlertEmitter)
	}
	return imapGatewayRuntime{
		service: service,
		store:   mailservice.NewIMAPStoreAdapter(service),
		backend: mailservice.NewIMAPBackendAdapter(authenticator, service),
		events:  events,
		fanOut:  fanOut,
	}
}

func imapServerOptionsForConfig(cfg config.Config, backend imapgw.Backend) (imapServerOptions, error) {
	tlsConfig, err := imapTLSConfig(cfg)
	if err != nil {
		return imapServerOptions{}, err
	}
	return imapServerOptions{
		Addr:              strings.TrimSpace(cfg.IMAPAddr),
		Backend:           backend,
		TLSConfig:         tlsConfig,
		AllowInsecureAuth: cfg.IMAPAllowInsecureAuth,
		MaxConnections:    cfg.IMAPMaxConnections,
		ReadTimeout:       cfg.IMAPReadTimeout,
		WriteTimeout:      cfg.IMAPWriteTimeout,
		IdleTimeout:       cfg.IMAPIdleTimeout,
	}, nil
}

func newIMAPServer(opts imapServerOptions) (*imapgw.Server, error) {
	return imapgw.NewServer(imapgw.ServerOptions{
		Addr:              opts.Addr,
		Backend:           opts.Backend,
		TLSConfig:         opts.TLSConfig,
		AllowInsecureAuth: opts.AllowInsecureAuth,
		MaxConnections:    opts.MaxConnections,
		ReadTimeout:       opts.ReadTimeout,
		WriteTimeout:      opts.WriteTimeout,
		IdleTimeout:       opts.IdleTimeout,
	})
}

func newIMAPMailboxEventRouter(uidEnsurer imapnotify.UIDEnsurer, events imapnotify.MailboxEventPublisher, deltaSync imapnotify.DeltaSyncNotifier) (*eventstream.Router, error) {
	router := eventstream.NewRouter()
	handler := imapnotify.NewMailStoredHandler(uidEnsurer).WithMailboxEvents(events)
	if deltaSync != nil {
		handler = handler.WithDeltaSync(deltaSync)
	}
	if err := router.Register("mail.stored", handler); err != nil {
		return nil, err
	}
	return router, nil
}

func runIMAPGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	repository := maildb.NewRepository(db)
	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(runCtx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()
	quotaAlertEmitter := maildb.NewQuotaWarningEmitter(db, redisClient, cfg.EventStream)
	runtime := newIMAPGatewayRuntime(cfg, repository, store, repository, quotaAlertEmitter)
	searchIDSource, err := searchIDSourceForConfig(cfg)
	if err != nil {
		return err
	}
	if searchIDSource != nil {
		runtime.service.WithSearchIDSource(searchIDSource)
	}

	router, err := newIMAPMailboxEventRouter(repository, runtime.events, &fanOutAdapter{fanOut: runtime.fanOut})
	if err != nil {
		return err
	}
	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.IMAPNotifyConsumerGroup,
		Consumer:         cfg.IMAPNotifyConsumerName,
		Count:            int64(cfg.IMAPNotifyConsumerCount),
		Block:            cfg.IMAPNotifyConsumerBlock,
		ClaimIdle:        cfg.IMAPNotifyConsumerClaimIdle,
		MaxDeliveries:    cfg.IMAPNotifyConsumerMaxDeliveries,
		DeadLetterStream: cfg.IMAPNotifyConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
	})
	if err != nil {
		return err
	}

	serverOptions, err := imapServerOptionsForConfig(cfg, runtime.backend)
	if err != nil {
		return err
	}
	server, err := newIMAPServer(serverOptions)
	if err != nil {
		return err
	}
	listener, err := server.Listen()
	if err != nil {
		return err
	}
	errCh := make(chan error, 2)
	go func() {
		logger.Info(
			"imap server listening",
			"mode", ModeIMAP,
			"addr", listener.Addr().String(),
			"tls_configured", serverOptions.TLSConfig != nil,
			"allow_insecure_auth", serverOptions.AllowInsecureAuth,
			"mailbox_event_broker", runtime.events != nil,
			"backend_adapter", "service",
		)
		errCh <- server.Serve(listener)
	}()
	go func() {
		logger.Info(
			"imap mailbox notification consumer started",
			"stream", cfg.EventStream,
			"group", cfg.IMAPNotifyConsumerGroup,
			"consumer", cfg.IMAPNotifyConsumerName,
			"count", cfg.IMAPNotifyConsumerCount,
			"block", cfg.IMAPNotifyConsumerBlock.String(),
			"max_deliveries", cfg.IMAPNotifyConsumerMaxDeliveries,
			"dead_letter_stream", cfg.IMAPNotifyConsumerDeadLetterStream,
		)
		errCh <- consumer.Run(runCtx)
	}()

	select {
	case <-ctx.Done():
		cancel()
		if err := listener.Close(); err != nil {
			logger.Warn("close listener", "error", err)
		}
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		for range 2 {
			err := <-errCh
			if err != nil && !errors.Is(err, imapgw.ErrServerClosed) {
				return err
			}
		}
		return nil
	case err := <-errCh:
		cancel()
		if err := listener.Close(); err != nil {
			logger.Warn("close listener", "error", err)
		}
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		otherErr := <-errCh
		if err == nil || errors.Is(err, imapgw.ErrServerClosed) {
			err = otherErr
		}
		if err != nil && !errors.Is(err, imapgw.ErrServerClosed) {
			return err
		}
		return nil
	}
}

func pop3TLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.POP3TLSCertFile == "" && cfg.POP3TLSKeyFile == "" {
		return nil, nil
	}
	if cfg.POP3TLSCertFile == "" || cfg.POP3TLSKeyFile == "" {
		return nil, errors.New("both POP3 TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.POP3TLSCertFile, cfg.POP3TLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func runPOP3Gateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}

	repository := maildb.NewRepository(db)
	service := mailservice.New(repository, store).WithMessageBodyCache(cfg.MessageBodyCacheEntries, cfg.MessageBodyCacheTTL)

	server, err := pop3ServerForConfig(cfg, repository, service)
	if err != nil {
		return err
	}

	addr := strings.TrimSpace(cfg.POP3Addr)
	if addr == "" {
		addr = ":1110"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("pop3 listen %s: %w", addr, err)
	}

	logger.Info("pop3 server listening", "mode", ModePOP3, "addr", ln.Addr().String(), "tls_configured", server.TLSConfig != nil)

	errCh := make(chan error, 2)
	go func() { errCh <- server.Serve(ln) }()

	pop3sAddr := strings.TrimSpace(cfg.POP3SAddr)
	if pop3sAddr != "" {
		if server.TLSConfig == nil {
			server.Close()
			return errors.New("GOGOMAIL_POP3S_ADDR requires POP3 TLS certificate and key files")
		}
		pop3sLn, err := net.Listen("tcp", pop3sAddr)
		if err != nil {
			server.Close()
			return fmt.Errorf("pop3s listen %s: %w", pop3sAddr, err)
		}
		logger.Info("pop3s server listening", "mode", ModePOP3, "addr", pop3sLn.Addr().String())
		go func() { errCh <- server.Serve(tls.NewListener(pop3sLn, server.TLSConfig)) }()
	}

	select {
	case <-ctx.Done():
		server.Close()
		return nil
	case err := <-errCh:
		return err
	}
}

func pop3ServerForConfig(cfg config.Config, repository *maildb.Repository, service *mailservice.Service) (*pop3d.Server, error) {
	tlsConfig, err := pop3TLSConfig(cfg)
	if err != nil {
		return nil, err
	}
	return &pop3d.Server{
		Store:          mailservice.NewPOP3StoreAdapter(repository, service),
		TLSConfig:      tlsConfig,
		Greeting:       "gogomail POP3 ready",
		IdleTimeout:    cfg.POP3IdleTimeout,
		MaxConnections: cfg.POP3MaxConnections,
	}, nil
}

func runCalDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	calendarRepository := caldavgw.NewRepository(db)
	accountRepository := maildb.NewRepository(db)
	directoryRepository := directory.NewRepository(db)
	resolver := caldavgw.NewBasicAuthResolver(accountRepository, cfg.CalDAVAllowInsecureAuth)
	resolver.TrustForwardedProto = cfg.CalDAVTrustForwardedProto
	resolver, err = resolver.WithTrustedProxies(cfg.CalDAVTrustedProxies)
	if err != nil {
		return fmt.Errorf("invalid caldav trusted proxies: %w", err)
	}
	handler := caldavgw.NewHandler(calendarRepository, resolver.Resolve)
	handler.IncludeScheduling = cfg.CalDAVScheduling
	handler.AccessAuthorizer = caldavgw.DelegatedAccessPolicy{
		Directory: directoryRepository,
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{
			Checker:         directoryRepository,
			AuditRepository: audit.NewPostgresRepository(db),
		},
	}
	server := newCalDAVHTTPServer(cfg, handler)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("caldav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newCalDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.CalDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func runCardDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	addressBookRepository := carddavgw.NewRepository(db)
	accountRepository := maildb.NewRepository(db)
	directoryRepository := directory.NewRepository(db)
	resolver := carddavgw.NewBasicAuthResolver(accountRepository, cfg.CardDAVAllowInsecureAuth)
	resolver.TrustForwardedProto = cfg.CardDAVTrustForwardedProto
	resolver, err = resolver.WithTrustedProxies(cfg.CardDAVTrustedProxies)
	if err != nil {
		return fmt.Errorf("invalid carddav trusted proxies: %w", err)
	}
	handler := carddavgw.NewHandler(addressBookRepository, resolver.Resolve)
	handler.AccessAuthorizer = carddavgw.DelegatedAccessPolicy{
		Directory: directoryRepository,
		Authorizer: accesspolicy.DelegatedAccessAuthorizer{
			Checker:         directoryRepository,
			AuditRepository: audit.NewPostgresRepository(db),
		},
	}
	server := newCardDAVHTTPServer(cfg, handler)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("carddav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newCardDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.CardDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func runWebDAVGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	driveSvc := driveServiceForConfig(db, cfg, store)
	webdavSvc := httpapi.NewWebDAVService(driveSvc)

	tokenManager, err := auth.NewTokenManager(cfg.AuthJWTSecret)
	if err != nil {
		return fmt.Errorf("create token manager: %w", err)
	}

	mux := http.NewServeMux()
	opts := httpapi.WebDAVRouteOptions{
		DepthInfinityEnabled: cfg.WebDAVDepthInfinityEnabled,
		TokenManager:         tokenManager,
	}
	httpapi.RegisterWebDAVRoutes(mux, webdavSvc, opts)
	server := newWebDAVHTTPServer(cfg, mux)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("webdav gateway listening", "addr", server.Addr)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			return err
		}
		return ctx.Err()
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func newWebDAVHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              strings.TrimSpace(cfg.WebDAVAddr),
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func runLDAPGateway(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	auth := maildb.NewRepository(db)
	querier := &ldapDirectoryQuerier{
		repo:       directory.NewRepository(db),
		companyID:  cfg.LDAPCompanyID,
		baseDomain: cfg.LDAPBaseDomain,
	}
	tlsConfig, err := ldapTLSConfig(cfg)
	if err != nil {
		return err
	}
	namingContexts := []string{}
	if strings.TrimSpace(cfg.LDAPBaseDomain) != "" {
		namingContexts = append(namingContexts, strings.TrimSpace(cfg.LDAPBaseDomain))
	}

	errCh := make(chan error, 2)
	var servers []*ldapgw.LDAPServer
	if addr := strings.TrimSpace(cfg.LDAPAddr); addr != "" {
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		srv := ldapgw.NewServerWithOptions(ln, auth, querier, ldapgw.ServerOptions{
			TLSConfig:      tlsConfig,
			NamingContexts: namingContexts,
			ReferralURLs:   cfg.LDAPReferralURLs,
			Metrics:        ldapMetrics(cfg, logger),
		})
		servers = append(servers, srv)
		go func() {
			logger.Info("ldap gateway listening", "mode", ModeLDAPGateway, "addr", ln.Addr().String(), "starttls_configured", tlsConfig != nil)
			errCh <- srv.Serve()
		}()
	}
	if addr := strings.TrimSpace(cfg.LDAPSAddr); addr != "" {
		if tlsConfig == nil {
			return errors.New("GOGOMAIL_LDAPS_ADDR requires LDAP TLS certificate and key files")
		}
		ln, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		tlsLn := tls.NewListener(ln, tlsConfig)
		srv := ldapgw.NewServerWithOptions(tlsLn, auth, querier, ldapgw.ServerOptions{
			NamingContexts: namingContexts,
			ReferralURLs:   cfg.LDAPReferralURLs,
			Metrics:        ldapMetrics(cfg, logger),
		})
		servers = append(servers, srv)
		go func() {
			logger.Info("ldaps gateway listening", "mode", ModeLDAPGateway, "addr", ln.Addr().String())
			errCh <- srv.Serve()
		}()
	}
	if len(servers) == 0 {
		return errors.New("at least one LDAP listener address must be configured")
	}

	select {
	case <-ctx.Done():
		for _, srv := range servers {
			if err := srv.Close(); err != nil {
				logger.Warn("close ldap server", "error", err)
			}
		}
		return ctx.Err()
	case err := <-errCh:
		return err
	}
}

func ldapTLSConfig(cfg config.Config) (*tls.Config, error) {
	if cfg.LDAPTLSCertFile == "" && cfg.LDAPTLSKeyFile == "" {
		return nil, nil
	}
	if cfg.LDAPTLSCertFile == "" || cfg.LDAPTLSKeyFile == "" {
		return nil, errors.New("both LDAP TLS certificate and key files are required")
	}
	cert, err := tls.LoadX509KeyPair(cfg.LDAPTLSCertFile, cfg.LDAPTLSKeyFile)
	if err != nil {
		return nil, err
	}
	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		MinVersion:   tls.VersionTLS12,
	}, nil
}

type ldapDirectoryQuerier struct {
	repo       *directory.Repository
	companyID  string
	baseDomain string
}

func (q *ldapDirectoryQuerier) SearchPrincipals(ctx context.Context, req ldapgw.DirectorySearchRequest) ([]ldapgw.PrincipalEntry, error) {
	query := ldapFilterToQuery(req.Filter)
	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	baseDN := q.baseDomain
	if baseDN == "" {
		baseDN = "dc=local"
	}
	if req.Scope == 0 {
		if kind, id, ok := ldapPrincipalFromDN(req.BaseDN); ok {
			principal, err := q.repo.ResolvePrincipal(ctx, directory.ResolvePrincipalRequest{
				ID:         id,
				Kind:       kind,
				ActiveOnly: true,
			})
			if err != nil {
				return nil, nil
			}
			entry, err := q.ldapPrincipalEntry(ctx, principal, baseDN, req.Attrs)
			if err != nil {
				return nil, err
			}
			return []ldapgw.PrincipalEntry{entry}, nil
		}
	}
	principals, err := q.repo.SearchPrincipals(ctx, directory.SearchPrincipalsRequest{
		CompanyID:  q.companyID,
		Kinds:      req.Kinds,
		Query:      query,
		ActiveOnly: true,
		Limit:      limit,
		Offset:     req.Offset,
	})
	if err != nil {
		return nil, err
	}
	entries := make([]ldapgw.PrincipalEntry, 0, len(principals))
	for _, p := range principals {
		entry, err := q.ldapPrincipalEntry(ctx, p, baseDN, req.Attrs)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (q *ldapDirectoryQuerier) ldapPrincipalEntry(ctx context.Context, p directory.Principal, baseDN string, attrs []string) (ldapgw.PrincipalEntry, error) {
	entry := ldapPrincipalEntry(p, baseDN)
	if p.Kind == directory.PrincipalKindGroup && ldapShouldExpandGroupMembers(attrs) {
		memberships, err := q.repo.ListGroupMemberships(ctx, directory.ListGroupMembershipsRequest{
			CompanyID:  q.companyID,
			GroupID:    p.ID,
			ActiveOnly: true,
			Limit:      directory.MaxGroupMembershipListLimit,
		})
		if err != nil {
			return ldapgw.PrincipalEntry{}, err
		}
		for _, membership := range memberships {
			if memberDN := ldapPrincipalKindIDDN(membership.MemberKind, membership.MemberID, baseDN); memberDN != "" {
				entry.Members = append(entry.Members, memberDN)
			}
		}
	}
	if ldapShouldExpandMemberOf(attrs) {
		memberships, err := q.repo.ListGroupMemberships(ctx, directory.ListGroupMembershipsRequest{
			CompanyID:  q.companyID,
			MemberKind: p.Kind,
			MemberID:   p.ID,
			ActiveOnly: true,
			Limit:      directory.MaxGroupMembershipListLimit,
		})
		if err != nil {
			return ldapgw.PrincipalEntry{}, err
		}
		for _, membership := range memberships {
			if groupDN := ldapPrincipalKindIDDN(directory.PrincipalKindGroup, membership.GroupID, baseDN); groupDN != "" {
				entry.MemberOf = append(entry.MemberOf, groupDN)
			}
		}
	}
	return entry, nil
}

func ldapShouldExpandGroupMembers(attrs []string) bool {
	return ldapAttributeRequested(attrs, "member")
}

func ldapShouldExpandMemberOf(attrs []string) bool {
	return ldapAttributeRequested(attrs, "memberOf")
}

func ldapAttributeRequested(attrs []string, target string) bool {
	if len(attrs) == 0 {
		return true
	}
	for _, attr := range attrs {
		switch strings.ToLower(strings.TrimSpace(attr)) {
		case strings.ToLower(target), "*":
			return true
		case "1.1", "+":
			continue
		}
	}
	return false
}

func ldapPrincipalEntry(p directory.Principal, baseDN string) ldapgw.PrincipalEntry {
	return ldapgw.PrincipalEntry{
		DN:           ldapPrincipalDN(p, baseDN),
		Kind:         p.Kind,
		CN:           p.DisplayName,
		Mail:         p.PrimaryEmail,
		UID:          p.ID,
		OU:           p.DisplayName,
		DisplayName:  p.DisplayName,
		ResourceType: p.ResourceType,
	}
}

func ldapPrincipalKindIDDN(kind, id, baseDN string) string {
	return ldapPrincipalDN(directory.Principal{ID: id, Kind: kind}, baseDN)
}

func ldapPrincipalDN(p directory.Principal, baseDN string) string {
	id := ldapEscapeDNValue(p.ID)
	switch p.Kind {
	case directory.PrincipalKindOrganization:
		return fmt.Sprintf("ou=%s,ou=organizations,%s", id, baseDN)
	case directory.PrincipalKindGroup:
		return fmt.Sprintf("cn=%s,ou=groups,%s", id, baseDN)
	case directory.PrincipalKindResource:
		return fmt.Sprintf("cn=%s,ou=resources,%s", id, baseDN)
	default:
		return fmt.Sprintf("uid=%s,ou=users,%s", id, baseDN)
	}
}

func ldapPrincipalFromDN(dn string) (kind string, id string, ok bool) {
	parts := ldapSplitDN(dn)
	if len(parts) < 2 {
		return "", "", false
	}
	first := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(first) != 2 {
		return "", "", false
	}
	attr := strings.ToLower(strings.TrimSpace(first[0]))
	value := ldapUnescapeDNValue(strings.TrimSpace(first[1]))
	parent := strings.ToLower(strings.TrimSpace(parts[1]))
	switch {
	case attr == "uid" && parent == "ou=users":
		return directory.PrincipalKindUser, value, true
	case attr == "ou" && parent == "ou=organizations":
		return directory.PrincipalKindOrganization, value, true
	case attr == "cn" && parent == "ou=groups":
		return directory.PrincipalKindGroup, value, true
	case attr == "cn" && parent == "ou=resources":
		return directory.PrincipalKindResource, value, true
	default:
		return "", "", false
	}
}

func ldapEscapeDNValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		needsEscape := c == ',' || c == '+' || c == '"' || c == '\\' || c == '<' || c == '>' || c == ';' || c == '=' || c == 0
		if i == 0 && (c == ' ' || c == '#') {
			needsEscape = true
		}
		if i == len(value)-1 && c == ' ' {
			needsEscape = true
		}
		if needsEscape {
			b.WriteString(fmt.Sprintf("\\%02x", c))
			continue
		}
		b.WriteByte(c)
	}
	return b.String()
}

func ldapSplitDN(dn string) []string {
	var parts []string
	var b strings.Builder
	escaped := false
	for _, r := range strings.TrimSpace(dn) {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if r == '\\' {
			b.WriteRune(r)
			escaped = true
			continue
		}
		if r == ',' {
			parts = append(parts, strings.TrimSpace(b.String()))
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	parts = append(parts, strings.TrimSpace(b.String()))
	return parts
}

func ldapUnescapeDNValue(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' || i+1 >= len(value) {
			b.WriteByte(value[i])
			continue
		}
		if i+2 < len(value) && isHex(value[i+1]) && isHex(value[i+2]) {
			b.WriteByte(fromHex(value[i+1])<<4 | fromHex(value[i+2]))
			i += 2
			continue
		}
		i++
		b.WriteByte(value[i])
	}
	return b.String()
}

func isHex(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

func fromHex(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	default:
		return c - 'A' + 10
	}
}

type maildbSCIMUserService struct {
	repo            *maildb.Repository
	defaultDomainID string
}

func (s *maildbSCIMUserService) GetSCIMUser(ctx context.Context, id string) (scim.UserResource, error) {
	user, err := s.repo.GetUser(ctx, id)
	if err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	return maildbUserToSCIM(user), nil
}

func (s *maildbSCIMUserService) ListSCIMUsers(ctx context.Context, filter *scim.Filter, _, count int) ([]scim.UserResource, int, error) {
	users, _, err := s.repo.ListUsers(ctx, maildb.UserListRequest{
		DomainID: s.defaultDomainID,
		Limit:    count,
	})
	if err != nil {
		return nil, 0, err
	}
	var resources []scim.UserResource
	for _, u := range users {
		r := maildbUserToSCIM(u)
		if filter == nil || scim.MatchesFilter(r, filter) {
			resources = append(resources, r)
		}
	}
	return resources, len(resources), nil
}

func (s *maildbSCIMUserService) CreateSCIMUser(ctx context.Context, req scim.UserResource) (scim.UserResource, error) {
	domainID := s.defaultDomainID
	if domainID == "" {
		return scim.UserResource{}, fmt.Errorf("SCIM create requires GOGOMAIL_SCIM_DEFAULT_DOMAIN_ID")
	}
	displayName := req.Name.Formatted
	if displayName == "" {
		displayName = req.UserName
	}
	user, err := s.repo.CreateUser(ctx, maildb.CreateUserRequest{
		DomainID:    domainID,
		Username:    req.UserName,
		DisplayName: displayName,
		Address:     req.UserName,
	})
	if err != nil {
		return scim.UserResource{}, err
	}
	return maildbUserToSCIM(user), nil
}

func (s *maildbSCIMUserService) ReplaceSCIMUser(ctx context.Context, id string, req scim.UserResource) (scim.UserResource, error) {
	status := "active"
	if !req.Active {
		status = "suspended"
	}
	if err := s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status}); err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	return s.GetSCIMUser(ctx, id)
}

func (s *maildbSCIMUserService) PatchSCIMUser(ctx context.Context, id string, ops []scim.PatchOperation) (scim.UserResource, error) {
	// Verify the user exists first.
	if _, err := s.repo.GetUser(ctx, id); err != nil {
		return scim.UserResource{}, httpapi.ErrSCIMUserNotFound
	}
	for _, op := range ops {
		switch strings.ToLower(op.Op) {
		case "replace":
			// Handle path-less replace with a value object.
			if op.Path == "" {
				var attrs map[string]json.RawMessage
				if err := json.Unmarshal(op.Value, &attrs); err != nil {
					continue
				}
				if raw, ok := attrs["active"]; ok {
					var active bool
					if err := json.Unmarshal(raw, &active); err == nil {
						status := "active"
						if !active {
							status = "suspended"
						}
						_ = s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status})
					}
				}
				continue
			}
			// Handle path-targeted replace.
			switch strings.ToLower(op.Path) {
			case "active":
				var active bool
				if err := json.Unmarshal(op.Value, &active); err != nil {
					continue
				}
				status := "active"
				if !active {
					status = "suspended"
				}
				_ = s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: status})
			}
		}
	}
	return s.GetSCIMUser(ctx, id)
}

func (s *maildbSCIMUserService) DeleteSCIMUser(ctx context.Context, id string) error {
	if err := s.repo.UpdateUserStatus(ctx, maildb.UpdateUserStatusRequest{ID: id, Status: "suspended"}); err != nil {
		return httpapi.ErrSCIMUserNotFound
	}
	return nil
}

func maildbUserToSCIM(u maildb.UserView) scim.UserResource {
	return scim.UserResource{
		Schemas:  []string{scim.SchemaUser},
		ID:       u.ID,
		UserName: u.Username,
		Name:     scim.Name{Formatted: u.DisplayName},
		Active:   u.Status == "active",
		Meta: &scim.Meta{
			ResourceType: "User",
			Created:      u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			LastModified: u.CreatedAt.Format("2006-01-02T15:04:05Z"),
			Location:     "/scim/v2/Users/" + u.ID,
		},
	}
}

func ldapFilterToQuery(filter string) string {
	filter = strings.TrimSpace(filter)
	filter = strings.TrimPrefix(filter, "(")
	filter = strings.TrimSuffix(filter, ")")
	if idx := strings.Index(filter, "="); idx >= 0 {
		attr := strings.ToLower(strings.TrimSpace(filter[:idx]))
		switch attr {
		case "cn", "mail", "uid", "displayname", "givenname", "sn", "ou", "description", "name", "canonicalname", "samaccountname", "userprincipalname", "mailnickname", "proxyaddresses":
		default:
			return ""
		}
		val := filter[idx+1:]
		val = strings.Trim(val, "*")
		if attr == "canonicalname" {
			if idx := strings.LastIndex(val, "/"); idx >= 0 {
				val = val[idx+1:]
			}
		}
		return val
	}
	return ""
}

func runAttachmentCleanupWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	service := mailservice.New(maildb.NewRepository(db), store)
	if cfg.AttachmentCleanupRunOnce {
		_, err := cleanupStaleAttachmentUploadsOnce(ctx, service, time.Now, cfg.AttachmentCleanupStaleAge, cfg.AttachmentCleanupBatchSize, logger)
		return err
	}
	return runAttachmentCleanupLoop(ctx, service, cfg.AttachmentCleanupInterval, cfg.AttachmentCleanupStaleAge, cfg.AttachmentCleanupBatchSize, logger)
}

func runDriveCleanupWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	service := driveServiceForConfig(db, cfg, store)
	if cfg.DriveCleanupRunOnce {
		_, err := cleanupDriveOnce(ctx, service, time.Now, cfg.DriveCleanupBatchSize, logger)
		return err
	}
	return runDriveCleanupLoop(ctx, service, cfg.DriveCleanupInterval, cfg.DriveCleanupBatchSize, logger)
}

func runDAVSyncRetentionWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	runners := davSyncRetentionRunners{
		CalDAV:  caldavgw.NewRepository(db),
		CardDAV: carddavgw.NewRepository(db),
		Audit:   davsyncretention.NewRepository(db),
	}
	if cfg.DAVSyncRetentionRunOnce {
		_, err := runDAVSyncRetentionOnce(ctx, runners, time.Now, cfg, logger)
		return err
	}
	if _, err := runDAVSyncRetentionOnce(ctx, runners, time.Now, cfg, logger); err != nil && logger != nil {
		logger.Error("DAV sync retention failed", "error", err)
	}
	ticker := time.NewTicker(cfg.DAVSyncRetentionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := runDAVSyncRetentionOnce(ctx, runners, time.Now, cfg, logger); err != nil && logger != nil {
				logger.Error("DAV sync retention failed", "error", err)
			}
		}
	}
}

func driveServiceForConfig(db *sql.DB, cfg config.Config, store storage.Store) *drive.Service {
	return drive.NewService(drive.NewRepository(db), storageStoresForConfig(cfg, store)).WithDefaultStorageBackend(normalizedStorageBackend(cfg.StorageBackend))
}

func orgChartServiceForDB(db *sql.DB) httpapi.OrgChartService {
	return orgchart.NewService(orgchart.NewRepository(db), nil)
}

func storageStoresForConfig(cfg config.Config, store storage.Store) map[string]storage.Store {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	stores := map[string]storage.Store{
		backend: store,
	}
	if backend == "local" || backend == "nfs" {
		stores["local"] = store
		stores["nfs"] = store
	}
	if backend == "s3" || backend == "minio" {
		stores["s3"] = store
		stores["minio"] = store
	}
	for _, label := range cfg.StorageBackendCompatLabels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		stores[label] = store
	}
	return stores
}

func storageCapabilitiesForConfig(cfg config.Config) storage.BackendCapabilities {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	labels := []string{backend}
	if backend == "local" || backend == "nfs" {
		labels = append(labels, "local", "nfs")
	}
	if backend == "s3" || backend == "minio" {
		labels = append(labels, "s3", "minio")
	}
	labels = append(labels, cfg.StorageBackendCompatLabels...)
	activeLabels := make([]string, 0, len(labels))
	seen := map[string]struct{}{}
	for _, label := range labels {
		label = strings.ToLower(strings.TrimSpace(label))
		if label == "" {
			continue
		}
		if _, ok := seen[label]; ok {
			continue
		}
		seen[label] = struct{}{}
		activeLabels = append(activeLabels, label)
	}
	sort.Strings(activeLabels)
	supportsLocalNFS, supportsMinIO, supportsAWSCompatible := storage.SupportMatrixForLabels(activeLabels)

	capabilities := storage.BackendCapabilities{
		ContractVersion:       httpapi.BackendContractVersion,
		ConfiguredBackend:     backend,
		BackendClass:          backend,
		ActiveLabels:          activeLabels,
		Operations:            []string{"put", "get", "get_range", "stat", "copy", "move", "list", "delete"},
		LocalFilesystem:       backend == "local" || backend == "nfs",
		S3Compatible:          backend == "s3" || backend == "minio",
		PathStyleAddressing:   false,
		CompatLabelsEnabled:   len(cfg.StorageBackendCompatLabels) > 0,
		ReadinessProbe:        true,
		SecretsRedacted:       true,
		SupportsBackendSwitch: true,
		SupportsLocalNFS:      supportsLocalNFS,
		SupportsMinIO:         supportsMinIO,
		SupportsAWSCompatible: supportsAWSCompatible,
		RequiresByteMigration: true,
	}
	if capabilities.S3Compatible {
		capabilities.BackendClass = "s3_compatible"
		capabilities.Region = strings.TrimSpace(cfg.StorageS3Region)
		capabilities.Bucket = strings.TrimSpace(cfg.StorageS3Bucket)
		capabilities.Prefix = strings.Trim(strings.TrimSpace(cfg.StorageS3Prefix), "/")
		endpointValue := strings.TrimSpace(cfg.StorageS3Endpoint)
		if endpointValue == "" && capabilities.Region != "" {
			endpointValue = "https://s3." + capabilities.Region + ".amazonaws.com"
		}
		if endpoint, err := storage.ValidateS3Endpoint(endpointValue); err == nil {
			capabilities.EndpointOrigin = endpoint.Scheme + "://" + endpoint.Host
			if endpoint.Path != "" && endpoint.Path != "/" {
				capabilities.EndpointOrigin += endpoint.EscapedPath()
			}
			capabilities.PathStyleAddressing = cfg.StorageS3ForcePathStyle || backend == "minio" || storage.S3BucketNeedsPathStyle(endpoint, capabilities.Bucket)
		} else {
			capabilities.PathStyleAddressing = cfg.StorageS3ForcePathStyle || backend == "minio"
		}
	} else if capabilities.LocalFilesystem {
		capabilities.BackendClass = "local"
	}
	return capabilities
}

func normalizedStorageBackend(value string) string {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" {
		return "local"
	}
	return backend
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

func runDAVSyncRetentionOnce(ctx context.Context, runners davSyncRetentionRunners, now func() time.Time, cfg config.Config, logger *slog.Logger) (davSyncRetentionResult, error) {
	if runners.CalDAV == nil {
		return davSyncRetentionResult{}, fmt.Errorf("CalDAV sync retention runner is required")
	}
	if runners.CardDAV == nil {
		return davSyncRetentionResult{}, fmt.Errorf("CardDAV sync retention runner is required")
	}
	if now == nil {
		now = time.Now
	}
	if cfg.DAVSyncRetentionCutoffAge <= 0 {
		return davSyncRetentionResult{}, fmt.Errorf("DAV sync retention cutoff age must be positive")
	}
	if cfg.DAVSyncRetentionBatchSize <= 0 {
		return davSyncRetentionResult{}, fmt.Errorf("DAV sync retention batch size must be positive")
	}
	if !cfg.DAVSyncRetentionDryRun && !cfg.DAVSyncRetentionConfirmReady {
		return davSyncRetentionResult{}, fmt.Errorf("DAV sync retention confirm_ready is required when dry-run is disabled")
	}
	cutoff := now().UTC().Add(-cfg.DAVSyncRetentionCutoffAge)
	calResult, err := runners.CalDAV.PruneCalendarSyncChanges(ctx, caldavgw.PruneCalendarSyncChangesRequest{
		Cutoff: cutoff,
		Limit:  cfg.DAVSyncRetentionBatchSize,
		DryRun: cfg.DAVSyncRetentionDryRun,
	})
	if err != nil {
		result := davSyncRetentionResult{
			Cutoff:       cutoff,
			Limit:        cfg.DAVSyncRetentionBatchSize,
			DryRun:       cfg.DAVSyncRetentionDryRun,
			ConfirmReady: cfg.DAVSyncRetentionConfirmReady,
			Status:       davsyncretention.RunStatusFailed,
		}
		result, auditErr := recordDAVSyncRetentionRun(ctx, runners.Audit, result, err)
		return result, errors.Join(err, auditErr)
	}
	cardResult, err := runners.CardDAV.PruneAddressBookChanges(ctx, carddavgw.PruneAddressBookChangesRequest{
		Cutoff: cutoff,
		Limit:  cfg.DAVSyncRetentionBatchSize,
		DryRun: cfg.DAVSyncRetentionDryRun,
	})
	if err != nil {
		result := davSyncRetentionResult{
			Cutoff:        cutoff,
			Limit:         cfg.DAVSyncRetentionBatchSize,
			DryRun:        cfg.DAVSyncRetentionDryRun,
			ConfirmReady:  cfg.DAVSyncRetentionConfirmReady,
			Status:        davsyncretention.RunStatusFailed,
			CalCandidates: calResult.CandidateCount,
			CalDeleted:    calResult.DeletedCount,
		}
		result, auditErr := recordDAVSyncRetentionRun(ctx, runners.Audit, result, err)
		return result, errors.Join(err, auditErr)
	}
	result := davSyncRetentionResult{
		Cutoff:         cutoff,
		Limit:          cfg.DAVSyncRetentionBatchSize,
		DryRun:         cfg.DAVSyncRetentionDryRun,
		ConfirmReady:   cfg.DAVSyncRetentionConfirmReady,
		Status:         davsyncretention.RunStatusCompleted,
		CalCandidates:  calResult.CandidateCount,
		CalDeleted:     calResult.DeletedCount,
		CardCandidates: cardResult.CandidateCount,
		CardDeleted:    cardResult.DeletedCount,
	}
	result, err = recordDAVSyncRetentionRun(ctx, runners.Audit, result, nil)
	if err != nil {
		return result, err
	}
	if logger != nil {
		logger.Info("DAV sync retention completed",
			"run_id", result.RunID,
			"cutoff", result.Cutoff,
			"limit", result.Limit,
			"dry_run", result.DryRun,
			"confirm_ready", result.ConfirmReady,
			"status", result.Status,
			"caldav_candidates", result.CalCandidates,
			"caldav_deleted", result.CalDeleted,
			"carddav_candidates", result.CardCandidates,
			"carddav_deleted", result.CardDeleted,
		)
	}
	return result, nil
}

func recordDAVSyncRetentionRun(ctx context.Context, recorder davSyncRetentionAuditRecorder, result davSyncRetentionResult, runErr error) (davSyncRetentionResult, error) {
	if recorder == nil {
		return result, nil
	}
	status := result.Status
	if status == "" {
		status = davsyncretention.RunStatusCompleted
	}
	errorMessage := ""
	if runErr != nil {
		status = davsyncretention.RunStatusFailed
		errorMessage = runErr.Error()
	}
	record, err := recorder.RecordRun(ctx, davsyncretention.RunRecord{
		Cutoff:            result.Cutoff,
		Limit:             result.Limit,
		DryRun:            result.DryRun,
		ConfirmReady:      result.ConfirmReady,
		Status:            status,
		ErrorMessage:      errorMessage,
		CalDAVCandidates:  result.CalCandidates,
		CalDAVDeleted:     result.CalDeleted,
		CardDAVCandidates: result.CardCandidates,
		CardDAVDeleted:    result.CardDeleted,
	})
	if err != nil {
		return result, fmt.Errorf("record DAV sync retention run: %w", err)
	}
	result.RunID = record.ID
	result.Status = record.Status
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

func runDriveCleanupLoop(ctx context.Context, cleaner driveCleanupRunner, interval time.Duration, batchSize int, logger *slog.Logger) error {
	if cleaner == nil {
		return fmt.Errorf("drive cleanup service is required")
	}
	if interval <= 0 {
		return fmt.Errorf("drive cleanup interval must be positive")
	}
	if batchSize <= 0 {
		return fmt.Errorf("drive cleanup batch size must be positive")
	}
	if _, err := cleanupDriveOnce(ctx, cleaner, time.Now, batchSize, logger); err != nil && logger != nil {
		logger.Error("drive cleanup failed", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if _, err := cleanupDriveOnce(ctx, cleaner, time.Now, batchSize, logger); err != nil && logger != nil {
				logger.Error("drive cleanup failed", "error", err)
			}
		}
	}
}

func cleanupDriveOnce(ctx context.Context, cleaner driveCleanupRunner, now func() time.Time, batchSize int, logger *slog.Logger) (driveCleanupResult, error) {
	if cleaner == nil {
		return driveCleanupResult{}, fmt.Errorf("drive cleanup service is required")
	}
	if batchSize <= 0 {
		return driveCleanupResult{}, fmt.Errorf("drive cleanup batch size must be positive")
	}
	if now == nil {
		now = time.Now
	}
	before := now().UTC()
	expired, err := cleaner.ExpireUploadSessions(ctx, drive.ExpireUploadSessionsRequest{
		Before: before,
		Limit:  batchSize,
	})
	if err != nil {
		return driveCleanupResult{}, err
	}
	objectCleanup, err := retryDriveObjectCleanupOnce(ctx, cleaner, batchSize, logger)
	result := driveCleanupResult{
		ExpiredSessions: len(expired),
		ObjectCleanup:   objectCleanup,
	}
	if logger != nil {
		logger.Info(
			"drive upload session cleanup completed",
			"expired_sessions", result.ExpiredSessions,
			"before", before.Format(time.RFC3339),
			"limit", batchSize,
		)
	}
	return result, err
}

func retryDriveObjectCleanupOnce(ctx context.Context, cleaner driveCleanupRunner, batchSize int, logger *slog.Logger) (drive.RetryObjectCleanupFailuresResult, error) {
	if cleaner == nil {
		return drive.RetryObjectCleanupFailuresResult{}, fmt.Errorf("drive cleanup service is required")
	}
	if batchSize <= 0 {
		return drive.RetryObjectCleanupFailuresResult{}, fmt.Errorf("drive cleanup batch size must be positive")
	}
	result, err := cleaner.RetryObjectCleanupFailures(ctx, drive.ListObjectCleanupFailuresRequest{
		Status: drive.ObjectCleanupFailureStatusPending,
		Limit:  batchSize,
	})
	if logger != nil {
		logger.Info(
			"drive cleanup completed",
			"scanned", result.Scanned,
			"deleted", result.Deleted,
			"resolved", result.Resolved,
			"failed", result.Failed,
			"limit", batchSize,
		)
	}
	return result, err
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
		db, err := database.Open(ctx, cfg.DatabaseURL)
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

	receiver := smtpd.NewReceiver(smtpd.ReceiverOptions{
		Store:              store,
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
		LatencyTracker: latencyTracker,
	})

	return smtpd.RunServer(ctx, smtpd.ServerOptions{
		Addr:             opts.Addr,
		Domain:           cfg.SMTPDomain,
		Receiver:         receiver,
		Logger:           logger,
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
	db, err := database.Open(ctx, cfg.DatabaseURL)
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

func runOutboxRelay(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	relay, err := outbox.NewRelay(outbox.RelayOptions{
		Store:        outbox.NewPostgresStore(db, cfg.OutboxRelayMaxAttempts),
		Publisher:    outbox.NewRedisStreamPublisher(redisClient, cfg.EventStream),
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
		"stream", cfg.EventStream,
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

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	router := eventstream.NewRouter()
	auditRepository := audit.NewPostgresRepository(db)
	mailFlowHandler := mailflow.NewHandler(db)

	if cfg.MailFlowOpenSearchBootstrap && strings.EqualFold(strings.TrimSpace(cfg.SearchIndexBackend), "opensearch") {
		opts := mailFlowOpenSearchOptionsForConfig(cfg)
		indexer, err := searchindex.NewMailFlowIndexer(opts)
		if err != nil {
			return fmt.Errorf("create mail flow indexer: %w", err)
		}
		if err := indexer.EnsureIndex(ctx); err != nil {
			return fmt.Errorf("bootstrap mail flow index: %w", err)
		}
		mailFlowHandler = mailflow.NewHandlerWithIndexer(db, &indexer)
	}
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	// Build webhook dispatcher early so it can be included in MultiHandlers below.
	var webhookStoredHandler, webhookDeliveredHandler, webhookBouncedHandler eventstream.Handler
	if cfg.WebhookDispatchEnabled {
		webhookConfigStore := configstore.NewPostgresConfigStore(db)
		webhookLoader := webhookguard.NewConfigStoreLoader(webhookConfigStore)
		webhookDispatcher := webhookguard.NewWebhookDispatcher(webhookguard.WebhookDispatcherOptions{
			Loader: webhookLoader,
			Logger: logger,
		})
		webhookStoredHandler = webhookguard.NewMailStoredWebhookHandler(webhookDispatcher)
		webhookDeliveredHandler = webhookguard.NewMailSentWebhookHandler(webhookDispatcher)
		webhookBouncedHandler = webhookguard.NewMailBouncedWebhookHandler(webhookDispatcher)
		logger.Info("webhook dispatcher enabled")
	}

	if err := router.Register("mail.stored", eventstream.MultiHandler{
		imapnotify.NewMailStoredHandler(maildb.NewRepository(db)),
		audit.NewMailStoredHandler(auditRepository),
		mailFlowHandler,
		inboundfilter.NewHandler(mailservice.New(maildb.NewRepository(db), store)),
		webhookStoredHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivered", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		mailFlowHandler,
		webhookDeliveredHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.bounced", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		dsnpkg.NewBounceHandler(dsnpkg.HandlerOptions{
			Store:        store,
			Queue:        dsnpkg.NewPostgresOutboxQueue(db),
			ReportingMTA: cfg.SMTPDomain,
			Postmaster:   cfg.DSNPostmaster,
			Farm:         outbound.FarmGeneral,
		}),
		mailFlowHandler,
		webhookBouncedHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_failed", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		mailFlowHandler,
	}); err != nil {
		return err
	}
	if err := router.Register("mail.delivery_exhausted", eventstream.MultiHandler{
		audit.NewDeliveryStatusHandler(auditRepository),
		dsnpkg.NewBounceHandler(dsnpkg.HandlerOptions{
			Store:        store,
			Queue:        dsnpkg.NewPostgresOutboxQueue(db),
			ReportingMTA: cfg.SMTPDomain,
			Postmaster:   cfg.DSNPostmaster,
			Farm:         outbound.FarmGeneral,
		}),
		mailFlowHandler,
	}); err != nil {
		return err
	}
	davAuditHandler := audit.NewDAVChangeHandler(auditRepository)
	if err := router.Register(audit.DAVChangeEventCalendar, davAuditHandler); err != nil {
		return err
	}
	if err := router.Register(audit.DAVChangeEventContacts, davAuditHandler); err != nil {
		return err
	}

	schedulingObjectStore, err := objectStoreForConfig(cfg)
	if err != nil {
		return fmt.Errorf("create scheduling object store: %w", err)
	}
	schedulingQueue := scheduling.NewDeliveryQueue(redisClient)
	directoryRepo := directory.NewRepository(db)
	carddavRepo := carddavgw.NewRepository(db)
	attendeeResolver := scheduling.NewDefaultAttendeeResolver(directoryRepo, carddavRepo)
	schedulingHandler := scheduling.NewHandler(logger, schedulingQueue, schedulingObjectStore, attendeeResolver)
	if err := router.Register("scheduling.outbox", schedulingHandler); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.EventConsumerGroup,
		Consumer:         cfg.EventConsumerName,
		Count:            int64(cfg.EventConsumerCount),
		Block:            cfg.EventConsumerBlock,
		ClaimIdle:        cfg.EventConsumerClaimIdle,
		MaxDeliveries:    cfg.EventConsumerMaxDeliveries,
		DeadLetterStream: cfg.EventConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
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
		"max_deliveries", cfg.EventConsumerMaxDeliveries,
		"dead_letter_stream", cfg.EventConsumerDeadLetterStream,
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

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
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
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	router := eventstream.NewRouter()
	if err := router.Register("mail.stored", searchindex.NewHandler(
		searchindex.NewStorageStoreReader(store),
		indexer,
		searchindex.HandlerOptions{MaxTextBodyBytes: cfg.SearchIndexMaxBodyBytes},
	)); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.SearchIndexConsumerGroup,
		Consumer:         cfg.SearchIndexConsumerName,
		Count:            int64(cfg.SearchIndexConsumerCount),
		Block:            cfg.SearchIndexConsumerBlock,
		ClaimIdle:        cfg.SearchIndexConsumerClaimIdle,
		MaxDeliveries:    cfg.SearchIndexConsumerMaxDeliveries,
		DeadLetterStream: cfg.SearchIndexConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
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
		"max_deliveries", cfg.SearchIndexConsumerMaxDeliveries,
		"dead_letter_stream", cfg.SearchIndexConsumerDeadLetterStream,
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

func mailFlowOpenSearchOptionsForConfig(cfg config.Config) searchindex.OpenSearchOptions {
	timeout := cfg.SearchIndexOpenSearchTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	return searchindex.OpenSearchOptions{
		Endpoint: cfg.SearchIndexOpenSearchEndpoint,
		Index:    cfg.MailFlowOpenSearchIndex,
		Client:   &http.Client{Timeout: timeout},
		Username: cfg.SearchIndexOpenSearchUsername,
		Password: cfg.SearchIndexOpenSearchPassword,
	}
}

func mailFlowStatsProviderForConfig(cfg config.Config, repo *maildb.Repository) mailflow.MailFlowStatsProvider {
	backend := strings.ToLower(strings.TrimSpace(cfg.MailFlowStatsBackend))
	if backend == "" {
		backend = "auto"
	}
	switch backend {
	case "postgres":
		return mailflow.NewPostgresMailFlowStatsProvider(repo)
	case "opensearch":
		searcher, err := searchindex.NewMailFlowStatsSearcher(mailFlowOpenSearchOptionsForConfig(cfg))
		if err != nil {
			logger := slog.Default()
			logger.Warn("failed to create mail flow OpenSearch stats searcher, falling back to postgres", "error", err)
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		return mailflow.NewOpenSearchMailFlowStatsProvider(&searcher)
	case "auto":
		if !cfg.MailFlowOpenSearchBootstrap {
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		searcher, err := searchindex.NewMailFlowStatsSearcher(mailFlowOpenSearchOptionsForConfig(cfg))
		if err != nil {
			logger := slog.Default()
			logger.Warn("failed to create mail flow OpenSearch stats searcher, falling back to postgres", "error", err)
			return mailflow.NewPostgresMailFlowStatsProvider(repo)
		}
		return mailflow.NewOpenSearchMailFlowStatsProvider(&searcher)
	default:
		logger := slog.Default()
		logger.Warn("unknown mail flow stats backend, using postgres", "backend", backend)
		return mailflow.NewPostgresMailFlowStatsProvider(repo)
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

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	router := eventstream.NewRouter()
	if err := router.Register(apimeter.EventAPIUsage, apimeter.NewUsageHandler(apimeter.NewPostgresAggregateStore(db))); err != nil {
		return err
	}

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.APIMeteringStream,
		Group:            cfg.APIMeteringConsumerGroup,
		Consumer:         cfg.APIMeteringConsumerName,
		Count:            int64(cfg.APIMeteringConsumerCount),
		Block:            cfg.APIMeteringConsumerBlock,
		ClaimIdle:        cfg.APIMeteringConsumerClaimIdle,
		MaxDeliveries:    cfg.APIMeteringConsumerMaxDeliveries,
		DeadLetterStream: cfg.APIMeteringConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
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
		"max_deliveries", cfg.APIMeteringConsumerMaxDeliveries,
		"dead_letter_stream", cfg.APIMeteringConsumerDeadLetterStream,
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

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
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
		Client:           redisClient,
		Stream:           cfg.EventStream,
		Group:            cfg.PushNotifyConsumerGroup,
		Consumer:         cfg.PushNotifyConsumerName,
		Count:            int64(cfg.PushNotifyConsumerCount),
		Block:            cfg.PushNotifyConsumerBlock,
		ClaimIdle:        cfg.PushNotifyConsumerClaimIdle,
		MaxDeliveries:    cfg.PushNotifyConsumerMaxDeliveries,
		DeadLetterStream: cfg.PushNotifyConsumerDeadLetterStream,
		Handler:          router,
		Logger:           logger,
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
		"max_deliveries", cfg.PushNotifyConsumerMaxDeliveries,
		"dead_letter_stream", cfg.PushNotifyConsumerDeadLetterStream,
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
			Client:   webhookguard.GuardedHTTPClient(&http.Client{Timeout: cfg.PushNotifyWebhookTimeout}, webhookguard.OutboundURLGuardOptions{}),
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

	redisClient := newRedisClient(cfg)
	if err := redisClient.Ping(ctx).Err(); err != nil {
		if err := redisClient.Close(); err != nil {
			logger.Warn("close redis client", "error", err)
		}
		return err
	}
	defer redisClient.Close()

	transport := delivery.NewDirectSMTPTransport()
	transport.Hello = cfg.DeliverySMTPHello
	transport.Timeout = cfg.DeliveryTimeout
	transport.TLSMode = delivery.DeliveryTLSMode(cfg.DeliveryTLSMode)
	transport.MaxRecipientsPerBatch = cfg.DeliveryRecipientBatchSize
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
	store, err := objectStoreForConfig(cfg)
	if err != nil {
		return err
	}
	var deliveryTransport delivery.Transport = transport
	if cfg.DeliveryCircuitBreakerEnabled {
		cb := delivery.NewCircuitBreakerTransport(transport, cfg.DeliveryCircuitBreakerMax)
		cb.FailureThreshold = cfg.DeliveryCircuitBreakerThreshold
		cb.HalfOpenTimeout = cfg.DeliveryCircuitBreakerTimeout
		deliveryTransport = cb
		logger.Info("delivery circuit breaker enabled",
			"max", cfg.DeliveryCircuitBreakerMax,
			"threshold", cfg.DeliveryCircuitBreakerThreshold,
			"timeout", cfg.DeliveryCircuitBreakerTimeout,
		)
	}

	handler := delivery.NewHandler(
		store,
		deliveryTransport,
		deliveryRecorder,
		delivery.NewPostgresRetryScheduler(db, retryPolicy),
	).WithExhaustionHook(deliveryRecorder)
	if cfg.DeliveryThrottleEnabled {
		throttlePolicy := delivery.ThrottlePolicy{
			FarmMaxConcurrent:   deliveryFarmLimits(cfg.DeliveryFarmConcurrency),
			DomainMaxConcurrent: cfg.DeliveryDomainConcurrency,
			DefaultConcurrent:   cfg.DeliveryDefaultConcurrency,
		}
		if strings.EqualFold(strings.TrimSpace(cfg.DeliveryThrottleBackend), "redis") {
			handler.WithThrottler(delivery.NewCoordinatedThrottler(
				throttlePolicy,
				delivery.NewRedisThrottleCounter(redisClient, "gogomail:delivery:throttle"),
			))
		} else {
			handler.WithThrottler(delivery.NewInMemoryThrottler(throttlePolicy))
		}
		logger.Info(
			"delivery throttling enabled",
			"backend", cfg.DeliveryThrottleBackend,
			"default_concurrency", cfg.DeliveryDefaultConcurrency,
			"farm_limits", cfg.DeliveryFarmConcurrency,
			"domain_limits", cfg.DeliveryDomainConcurrency,
		)
	}
	if backoff := deliveryDomainBackoffFromConfig(cfg, redisClient); backoff != nil {
		handler.WithDomainBackoff(backoff)
		logger.Info(
			"delivery adaptive domain backoff enabled",
			"backend", cfg.DeliveryDomainBackoffBackend,
			"scope", cfg.DeliveryDomainBackoffScope,
			"base_delay", cfg.DeliveryDomainBackoffBaseDelay,
			"max_delay", cfg.DeliveryDomainBackoffMaxDelay,
		)
	}
	handler.WithMetrics(deliveryMetrics(cfg, logger))

	consumer, err := eventstream.NewRedisConsumer(eventstream.RedisConsumerOptions{
		Client:           redisClient,
		Stream:           cfg.DeliveryStream,
		Group:            cfg.DeliveryConsumerGroup,
		Consumer:         cfg.DeliveryConsumerName,
		Count:            int64(cfg.DeliveryConsumerCount),
		Block:            cfg.DeliveryConsumerBlock,
		ClaimIdle:        cfg.DeliveryConsumerClaimIdle,
		MaxDeliveries:    cfg.DeliveryConsumerMaxDeliveries,
		DeadLetterStream: cfg.DeliveryConsumerDeadLetterStream,
		Handler:          handler,
		Logger:           logger,
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
		"max_deliveries", cfg.DeliveryConsumerMaxDeliveries,
		"dead_letter_stream", cfg.DeliveryConsumerDeadLetterStream,
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	mux := http.NewServeMux()
	var readinessChecks []httpapi.ReadinessCheckFunc

	var tokenManager *auth.TokenManager
	var apiKeyVerifier apikeys.PostgresVerifier
	var apiKeyVerifierConfigured bool
	if modeIncludesMailAPI(mode) {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		readinessChecks = append(readinessChecks, databaseReadinessCheck("mail_database", db, cfg.MigrationDir))

		store, err := objectStoreForConfig(cfg)
		if err != nil {
			return err
		}
		readinessChecks = append(readinessChecks, storageReadinessCheck("mail_storage", store))
		repository := maildb.NewRepository(db)
		service := mailservice.New(repository, store).WithMessageBodyCache(cfg.MessageBodyCacheEntries, cfg.MessageBodyCacheTTL)
		searchIDSource, err := searchIDSourceForConfig(cfg)
		if err != nil {
			return err
		}
		if searchIDSource != nil {
			service.WithSearchIDSource(searchIDSource)
		}
		if cfg.AuthJWTSecret != "" {
			tokenManager, err = tokenManagerForConfig(cfg, repository)
			if err != nil {
				return err
			}
		}
		driveRouteOptions := httpapi.DriveRouteOptions{}
		driveRouteOptions.PublicShareAudit = drivePublicShareAuditRecorder{audit: audit.NewPostgresRepository(db)}
		apiKeyVerifier = apikeys.NewPostgresVerifier(db)
		apiKeyVerifierConfigured = true
		if strings.EqualFold(strings.TrimSpace(cfg.DriveShareRateLimitBackend), "redis") {
			redisClient := newRedisClient(cfg)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				if err := redisClient.Close(); err != nil {
					logger.Warn("close redis client", "error", err)
				}
				return err
			}
			defer redisClient.Close()
			readinessChecks = append(readinessChecks, redisReadinessCheck("drive_share_rate_limit_redis", redisClient))
			driveRouteOptions.PublicShareLimiter = ratelimit.NewRedisFixedWindowLimiter(redisClient, "drive_share_public", int64(cfg.DriveShareRateLimitPerMinute), time.Minute)
			logger.Info("drive public share rate limiting enabled", "backend", "redis", "per_minute", cfg.DriveShareRateLimitPerMinute)
		}
		trackingRepo := maildb.NewRepository(db)
		service.WithTrackingRepo(trackingRepo, cfg.PublicBaseURL)
		mailConfigStore := configstore.NewPostgresConfigStore(db)
		if err := mailConfigStore.Start(ctx); err != nil {
			logger.Warn("runtime config store unavailable for mail api", "error", err)
		}
		mailOpts := httpapi.MailRouteOptions{
			SessionRevoker: repository,
			Authenticator:  repository,
			MFAStore:       repository,
			ConfigResolver: mailConfigStore,
		}
		httpapi.RegisterMailRoutesWithOptions(mux, service, tokenManager, mailOpts)
		httpapi.RegisterMFARoutes(mux, tokenManager, mailOpts)
		httpapi.RegisterTrackingRoutes(mux, trackingRepo, tokenManager)
		httpapi.RegisterDriveRoutesWithOptions(mux, driveServiceForConfig(db, cfg, store), tokenManager, driveRouteOptions)
		httpapi.RegisterContactRoutes(mux, httpapi.NewContactHandler(
			carddavgw.NewRepository(db),
			directory.NewRepository(db),
		), tokenManager)
		calendarHandler := httpapi.NewCalendarHandler(caldavgw.NewRepository(db), service)
		httpapi.RegisterCalendarRoutes(mux, calendarHandler, tokenManager)
		httpapi.RegisterCalendarSubscriptionRoutes(mux, calendarHandler, tokenManager)
		httpapi.RegisterPasswordResetRoutes(
			mux,
			httpapi.NewMaildbPasswordResetAdapter(repository),
			mailservice.NewSMTPSystemEmailSenderFromEnv(),
			cfg.PublicBaseURL,
		)
		logger.Info("mail api routes registered")
	}
	if modeIncludesAdminAPI(mode) {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		readinessChecks = append(readinessChecks, databaseReadinessCheck("admin_database", db, cfg.MigrationDir))

		if err := initializeIdPProvider(ctx, db); err != nil {
			return fmt.Errorf("initialize identity provider: %w", err)
		}

		var redisClient *redis.Client
		var pressure backpressureStore
		if cfg.BackpressureBackend == "redis" {
			redisClient = newRedisClient(cfg)
			if err := redisClient.Ping(ctx).Err(); err != nil {
				if err := redisClient.Close(); err != nil {
					logger.Warn("close redis client", "error", err)
				}
				return err
			}
			defer redisClient.Close()
			readinessChecks = append(readinessChecks, redisReadinessCheck("backpressure_redis", redisClient))
			pressure = backpressure.NewRedisBackpressure(redisClient, backpressure.DefaultStateKey)
		}

		store, err := objectStoreForConfig(cfg)
		if err != nil {
			return err
		}
		readinessChecks = append(readinessChecks, storageReadinessCheck("admin_storage", store))
		repository := maildb.NewRepository(db)
		if tokenManager == nil && cfg.AuthJWTSecret != "" {
			tokenManager, err = tokenManagerForConfig(cfg, repository)
			if err != nil {
				return err
			}
		}
		mailFlowStatsProvider := mailFlowStatsProviderForConfig(cfg, repository)
		configStore := configstore.NewPostgresConfigStore(db)
		if err := configStore.Start(ctx); err != nil {
			return fmt.Errorf("start config store: %w", err)
		}
		adminRepo := admin.NewRepository(db)
		adminSvc := admin.NewService(adminRepo)
		orgChartService := orgChartServiceForDB(db)
		adminRouteOpts := []httpapi.AdminRouteOption{
			httpapi.WithStorageCapabilities(storageCapabilitiesForConfig(cfg)),
			httpapi.WithConfigNotifier(configStore),
			httpapi.WithTokenManager(tokenManager),
			httpapi.WithEnvironment(cfg.Environment),
			httpapi.WithAdminMFAStore(repository),
			httpapi.WithAdminMFARequired(cfg.AdminMFARequired),
			httpapi.WithAdminConfigResolver(configStore),
		}
		if redisClient != nil {
			if dlqReader, err := eventstream.NewRedisDLQReader(redisClient); err == nil {
				adminRouteOpts = append(adminRouteOpts, httpapi.WithDLQReader(dlqReader))
			}
		}
		httpapi.RegisterAdminRoutes(mux, adminService{
			Repository:                  repository,
			adminSvc:                    adminSvc,
			backpressure:                pressure,
			audit:                       audit.NewPostgresRepository(db),
			exportStore:                 store,
			exportManifestSigner:        apiUsageExportManifestSigner(cfg),
			exportManifestSignerBackend: cfg.APIUsageExportManifestSignerBackend,
			exportManifestVerifier:      apiUsageExportManifestVerifier(cfg),
			directory:                   directory.NewRepository(db),
			drive:                       driveServiceForConfig(db, cfg, store),
			davSyncRetention:            davsyncretention.NewRepository(db),
			calDAVSyncRetention:         caldavgw.NewRepository(db),
			cardDAVSyncRetention:        carddavgw.NewRepository(db),
			attachmentCleanup:           mailservice.New(repository, store),
			mailFlowStats:               mailFlowStatsProvider,
			configStore:                 configStore,
		}, cfg.AdminToken, adminRouteOpts...)
		logger.Info("admin api routes registered")
		httpapi.RegisterOrgChartRoutes(mux, orgChartService, cfg.AdminToken)
		logger.Info("organization routes registered")
		if cfg.SCIMToken != "" {
			httpapi.RegisterSCIMRoutes(mux, &maildbSCIMUserService{
				repo:            repository,
				defaultDomainID: cfg.SCIMDefaultDomainID,
			}, cfg.SCIMToken)
			logger.Info("scim routes registered")
		}
		httpapi.RegisterSSOAdminRoutes(mux, repository, cfg.AdminToken)
		httpapi.RegisterSSORoutes(mux, repository, tokenManager)
		logger.Info("sso routes registered")
		httpapi.RegisterAutodiscoveryRoutes(mux, cfg.AdminToken, httpapi.NewDNSDiscoveryChecker())
	}

	var meteringDB *sql.DB
	if strings.EqualFold(strings.TrimSpace(cfg.APIMeteringBackend), "outbox") {
		db, err := database.Open(ctx, cfg.DatabaseURL)
		if err != nil {
			return err
		}
		defer db.Close()
		meteringDB = db
		readinessChecks = append(readinessChecks, databaseReadinessCheck("api_metering_database", db, cfg.MigrationDir))
	}
	httpapi.RegisterWellKnownRoutes(mux, cfg.WellKnownCalDAVURL, cfg.WellKnownCardDAVURL)
	httpapi.RegisterHealthRoutesWithChecks(mux, readinessChecks...)

	handler := apiMeteringHandler(mux, cfg, logger, meteringDB, tokenManager, cfg.AdminToken)
	if apiKeyVerifierConfigured {
		handler = apikeys.Middleware(apiKeyVerifier)(handler)
	}
	handler = httpapi.NewAdminIPRateLimiter(600, time.Minute).Middleware(handler)
	handler = httpapi.MaxRequestBodyMiddleware(4*1024*1024)(handler)
	if cfg.CORSAllowedOrigins != "" {
		handler = httpapi.CORSMiddleware(cfg.CORSAllowedOrigins)(handler)
	}
	handler = httpapi.SecurityHeadersMiddleware(handler)
	server := newHTTPServer(cfg, handler)

	go serveMetrics(ctx, cfg, logger)

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

func newHTTPServer(cfg config.Config, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadTimeout:       cfg.HTTPReadTimeout,
		WriteTimeout:      cfg.HTTPWriteTimeout,
		IdleTimeout:       cfg.HTTPIdleTimeout,
		ReadHeaderTimeout: cfg.HTTPReadHeaderTimeout,
		MaxHeaderBytes:    cfg.HTTPMaxHeaderBytes,
	}
}

func tokenManagerForConfig(cfg config.Config, checker auth.RevocationChecker) (*auth.TokenManager, error) {
	if strings.TrimSpace(cfg.AuthJWTSecret) == "" {
		return nil, nil
	}
	tokenManager, err := auth.NewTokenManager(cfg.AuthJWTSecret)
	if err != nil {
		return nil, err
	}
	if checker != nil {
		tokenManager.SetRevocationChecker(checker)
	}
	return tokenManager, nil
}

func modeIncludesMailAPI(mode Mode) bool {
	return mode == ModeMailAPI || mode == ModeAllInOne
}

func modeIncludesAdminAPI(mode Mode) bool {
	return mode == ModeAdminAPI || mode == ModeAllInOne
}

type configuredObjectStore interface {
	storage.Store
	Check(context.Context) error
}

func objectStoreForConfig(cfg config.Config) (configuredObjectStore, error) {
	backend := normalizedStorageBackend(cfg.StorageBackend)
	switch backend {
	case "local", "nfs":
		return storage.NewLocalStore(cfg.MailstoreRoot), nil
	case "s3", "minio":
		opts, err := s3OptionsForConfig(cfg, backend)
		if err != nil {
			return nil, err
		}
		return storage.NewS3Store(opts)
	default:
		return nil, fmt.Errorf("unsupported storage backend %q", cfg.StorageBackend)
	}
}

func s3OptionsForConfig(cfg config.Config, backend string) (storage.S3Options, error) {
	backend = strings.ToLower(strings.TrimSpace(backend))
	client, err := s3HTTPClientForConfig(cfg)
	if err != nil {
		return storage.S3Options{}, err
	}
	return storage.S3Options{
		Endpoint:        cfg.StorageS3Endpoint,
		Region:          cfg.StorageS3Region,
		Bucket:          cfg.StorageS3Bucket,
		Prefix:          cfg.StorageS3Prefix,
		AccessKeyID:     cfg.StorageS3AccessKeyID,
		SecretAccessKey: cfg.StorageS3SecretAccessKey,
		SessionToken:    cfg.StorageS3SessionToken,
		ForcePathStyle:  cfg.StorageS3ForcePathStyle || backend == "minio",
		HTTPClient:      client,
	}, nil
}

func s3HTTPClientForConfig(cfg config.Config) (*http.Client, error) {
	caCertFile := strings.TrimSpace(cfg.StorageS3CACertFile)
	if caCertFile == "" && !cfg.StorageS3InsecureSkipVerify {
		return nil, nil
	}
	rootCAs, err := x509.SystemCertPool()
	if err != nil || rootCAs == nil {
		rootCAs = x509.NewCertPool()
	}
	if caCertFile != "" {
		data, err := os.ReadFile(caCertFile)
		if err != nil {
			return nil, fmt.Errorf("read S3 CA certificate file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("S3 CA certificate file must contain at least one PEM-encoded certificate")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{
		MinVersion:         tls.VersionTLS12,
		RootCAs:            rootCAs,
		InsecureSkipVerify: cfg.StorageS3InsecureSkipVerify,
	}
	return &http.Client{
		Timeout:   30 * time.Second,
		Transport: transport,
	}, nil
}

func storageReadinessCheck(name string, store interface {
	Check(context.Context) error
}) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if store == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "storage is not configured"}
		}
		if err := store.Check(ctx); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{Name: name, Status: "ok", Detail: "probe ok"}
	}
}

func databaseReadinessCheck(name string, db *sql.DB, migrationDir string) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if db == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "database handle is not configured"}
		}
		if err := db.PingContext(ctx); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		current, expected, err := database.MigrationVersionReady(ctx, db, migrationDir)
		if err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{
			Name:   name,
			Status: "ok",
			Detail: fmt.Sprintf("ping ok; migration version %d/%d", current, expected),
		}
	}
}

func redisReadinessCheck(name string, client *redis.Client) httpapi.ReadinessCheckFunc {
	return func(ctx context.Context) httpapi.ReadinessCheck {
		if client == nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: "redis client is not configured"}
		}
		if err := client.Ping(ctx).Err(); err != nil {
			return httpapi.ReadinessCheck{Name: name, Status: "error", Detail: err.Error()}
		}
		return httpapi.ReadinessCheck{Name: name, Status: "ok", Detail: "ping ok"}
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

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
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

// newRedisClient creates a Redis client. When RedisSentinelAddrs is non-empty a
// failover (Sentinel) client is returned; otherwise a plain single-node client.
func newRedisClient(cfg config.Config) *redis.Client {
	if len(cfg.RedisSentinelAddrs) > 0 {
		return redis.NewFailoverClient(&redis.FailoverOptions{
			MasterName:    cfg.RedisMasterName,
			SentinelAddrs: cfg.RedisSentinelAddrs,
			Password:      cfg.RedisPassword,
		})
	}
	return redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
}

func waitForShutdown(ctx context.Context, logger *slog.Logger, mode Mode) error {
	logger.Info("mode scaffold is ready; component implementation will be added next", "mode", mode)
	<-ctx.Done()
	return nil
}
