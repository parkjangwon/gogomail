package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/gogomail/gogomail/internal/admin"
	"github.com/gogomail/gogomail/internal/apikeys"
	"github.com/gogomail/gogomail/internal/attachmentscan"
	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/directory"
	dmpkg "github.com/gogomail/gogomail/internal/dm"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/jmap"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/orgchart"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/ratelimit"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	"github.com/gogomail/gogomail/internal/storage"
	wapkg "github.com/gogomail/gogomail/internal/webauthn"
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

func runHTTP(ctx context.Context, cfg config.Config, logger *slog.Logger, mode Mode) error {
	// Initialise OTel tracing.  Shutdown is deferred so buffered spans are
	// exported before the process exits even under normal shutdown paths.
	tp, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:        cfg.OTelEnabled,
		ExporterEndpoint: cfg.OTelEndpoint,
		ServiceName:    cfg.OTelServiceName,
		ServiceVersion: cfg.OTelServiceVersion,
	})
	if err != nil {
		logger.Warn("tracing init failed, continuing without traces", "error", err)
	} else {
		defer func() {
			if shutErr := tp.Shutdown(context.WithoutCancel(ctx)); shutErr != nil {
				logger.Warn("tracing shutdown error", "error", shutErr)
			}
		}()
		if cfg.OTelEnabled {
			logger.Info("opentelemetry tracing enabled", "endpoint", cfg.OTelEndpoint, "service", cfg.OTelServiceName)
		}
	}

	mux := http.NewServeMux()
	var readinessChecks []httpapi.ReadinessCheckFunc

	// bgTracker tracks fire-and-forget goroutines spawned by HTTP handlers
	// (invite/welcome email sends, password reset token issue) so graceful
	// shutdown can drain them before http.Server.Shutdown returns.
	bgTracker := httpapi.NewBackgroundTracker(logger)

	var tokenManager *auth.TokenManager
	var apiKeyVerifier apikeys.PostgresVerifier
	var apiKeyVerifierConfigured bool
	if modeIncludesMailAPI(mode) {
		db, err := openDatabase(ctx, cfg)
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
			SessionRevoker:        repository,
			Authenticator:         repository,
			MFAStore:              repository,
			ConfigResolver:        mailConfigStore,
			RefreshTokenStore:     repository,
			WebPushVAPIDPublicKey: cfg.WebPushVAPIDPublicKey,
		}
		if strings.EqualFold(strings.TrimSpace(cfg.MailMutationRateLimitBackend), "redis") {
			mailRedisClient := newRedisClient(cfg)
			if err := mailRedisClient.Ping(ctx).Err(); err != nil {
				if err := mailRedisClient.Close(); err != nil {
					logger.Warn("close mail mutation redis client", "error", err)
				}
				return err
			}
			defer mailRedisClient.Close()
			readinessChecks = append(readinessChecks, redisReadinessCheck("mail_mutation_rate_limit_redis", mailRedisClient))
			mailOpts.MutationLimiter = ratelimit.NewRedisFixedWindowLimiter(mailRedisClient, "mail_mutation", int64(cfg.MailMutationRateLimitPerMinute), time.Minute)
			logger.Info("mail mutation rate limiting enabled", "backend", "redis", "per_minute", cfg.MailMutationRateLimitPerMinute)
		}
		httpapi.RegisterMailRoutesWithOptions(mux, service, tokenManager, mailOpts)
		if strings.TrimSpace(cfg.DMMasterKey) != "" {
			dmMasterKey, err := dmpkg.ParseMasterKey(cfg.DMMasterKey)
			if err != nil {
				return err
			}
			dmCrypto, err := dmpkg.NewCrypto(dmMasterKey)
			if err != nil {
				return err
			}
			httpapi.RegisterDMRoutes(mux, dmpkg.NewService(dmpkg.NewPostgresStore(db), dmCrypto).WithAttachmentStore(store), tokenManager, cfg.PublicBaseURL)
			logger.Info("dm routes registered")
		} else {
			logger.Warn("dm routes disabled; GOGOMAIL_DM_MASTER_KEY is not configured")
		}
		httpapi.RegisterMFARoutes(mux, tokenManager, mailOpts)
		httpapi.RegisterNotificationPreferenceRoutes(mux, httpapi.NewMaildbNotificationPreferenceAdapter(maildb.NewRepository(db)), tokenManager)
		httpapi.RegisterTrackingRoutes(mux, trackingRepo, tokenManager)
		httpapi.RegisterDriveRoutesWithOptions(mux, driveServiceForConfig(db, cfg, store), tokenManager, driveRouteOptions)
		httpapi.RegisterContactRoutes(mux, httpapi.NewContactHandler(
			carddavgw.NewRepository(db),
			directory.NewRepository(db),
		).WithOrgProfiler(orgchart.NewService(orgchart.NewRepository(db), nil)), tokenManager)
		calendarHandler := httpapi.NewCalendarHandler(caldavgw.NewRepository(db), service)
		httpapi.RegisterCalendarRoutes(mux, calendarHandler, tokenManager)
		httpapi.RegisterCalendarSubscriptionRoutes(mux, calendarHandler, tokenManager)
		httpapi.RegisterPasswordResetRoutes(
			mux,
			httpapi.NewMaildbPasswordResetAdapter(repository),
			mailservice.NewSMTPSystemEmailSender(cfg.SystemEmail.From, cfg.SystemEmail.SMTPAddr, cfg.SystemEmail.SMTPUser, cfg.SystemEmail.SMTPPass),
			cfg.PublicBaseURL,
			bgTracker,
		)
		logger.Info("mail api routes registered")

		if cfg.WebAuthnEnabled {
			rpid := strings.TrimSpace(cfg.WebAuthnRPID)
			if rpid == "" {
				// Derive RPID from PublicBaseURL hostname.
				if u, err2 := parsePublicHostname(cfg.PublicBaseURL); err2 == nil {
					rpid = u
				}
			}
			waStore := wapkg.NewStore(db)
			waSvc, err := wapkg.NewService(wapkg.Config{
				RPDisplayName: cfg.WebAuthnRPDisplayName,
				RPID:          rpid,
				RPOrigins:     cfg.WebAuthnRPOrigins,
			}, waStore)
			if err != nil {
				logger.Warn("webauthn init failed, feature disabled", "error", err)
			} else {
				httpapi.RegisterWebAuthnRoutes(mux, waSvc, tokenManager)
				logger.Info("webauthn mfa enabled", "rpid", rpid)
			}
		}

		httpapi.RegisterJMAPRoutes(mux, jmapHandler(cfg, repository, store, tokenManager, service))
		logger.Info("jmap routes registered")
	}
	if modeIncludesAdminAPI(mode) {
		db, err := openDatabase(ctx, cfg)
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
			httpapi.WithSystemEmailSender(mailservice.NewSMTPSystemEmailSender(cfg.SystemEmail.From, cfg.SystemEmail.SMTPAddr, cfg.SystemEmail.SMTPUser, cfg.SystemEmail.SMTPPass), cfg.PublicBaseURL),
			httpapi.WithAdminBootstrap(cfg.AdminBootstrap.Email, cfg.AdminBootstrap.Password),
			httpapi.WithBackgroundTracker(bgTracker),
		}
		if redisClient != nil {
			if dlqReader, err := eventstream.NewRedisDLQReader(redisClient); err == nil {
				adminRouteOpts = append(adminRouteOpts, httpapi.WithDLQReader(dlqReader))
			}
			adminRouteOpts = append(adminRouteOpts, httpapi.WithRedisLoginLimiter(redisClient))
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
			idpConfigRepo:               idprovider.NewConfigRepository(db),
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
		db, err := openDatabase(ctx, cfg)
		if err != nil {
			return err
		}
		defer db.Close()
		meteringDB = db
		readinessChecks = append(readinessChecks, databaseReadinessCheck("api_metering_database", db, cfg.MigrationDir))
	}
	httpapi.RegisterWellKnownRoutes(mux, cfg.WellKnownCalDAVURL, cfg.WellKnownCardDAVURL)
	if strings.EqualFold(strings.TrimSpace(cfg.AttachmentScanBackend), "clamav") {
		if sc, err2 := attachmentscan.NewClamAVScanner(attachmentscan.ClamAVOptions{
			Addr:                cfg.AttachmentScanClamAVAddr,
			Timeout:             cfg.AttachmentScanTimeout,
			MaxConcurrency:      cfg.AttachmentScanMaxConcurrency,
			MaxScanBytes:        cfg.AttachmentScanMaxBytes,
			FailureThreshold:    cfg.AttachmentScanFailureThreshold,
			CircuitOpenDuration: cfg.AttachmentScanCircuitOpenDuration,
		}); err2 == nil {
			sc := sc
			readinessChecks = append(readinessChecks, func(ctx context.Context) httpapi.ReadinessCheck {
				if err := sc.Ping(ctx); err != nil {
					return httpapi.ReadinessCheck{Name: "clamav", Status: "unhealthy", Detail: err.Error()}
				}
				return httpapi.ReadinessCheck{Name: "clamav", Status: "ok"}
			})
			logger.Info("clamav readiness check registered", "addr", cfg.AttachmentScanClamAVAddr)
		}
	}
	httpapi.RegisterHealthRoutesWithChecks(mux, readinessChecks...)

	handler := apiMeteringHandler(mux, cfg, logger, meteringDB, tokenManager, cfg.AdminToken)
	if apiKeyVerifierConfigured {
		handler = apikeys.Middleware(apiKeyVerifier, cfg.TrustedProxyCIDRs)(handler)
	}
	handler = httpapi.NewAdminIPRateLimiter(600, time.Minute).Middleware(handler)
	handler = httpapi.MaxRequestBodyMiddleware(4 * 1024 * 1024)(handler)
	if cfg.CORSAllowedOrigins != "" {
		handler = httpapi.CORSMiddleware(cfg.CORSAllowedOrigins)(handler)
	}
	handler = httpapi.StripInternalHeadersMiddleware(handler)
	handler = httpapi.SecurityHeadersMiddleware(handler)
	handler = httpapi.AccessLogMiddleware(logger, handler)
	handler = httpapi.RequestIDMiddleware(handler)
	if cfg.OTelEnabled {
		handler = observability.OTelHTTPMiddleware(cfg.OTelServiceName)(handler)
	}
	if strings.EqualFold(strings.TrimSpace(cfg.MetricsBackend), "prometheus") {
		handler = httpapi.MetricsMiddleware(sharedPrometheusAdapter(), handler)
	}
	server := newHTTPServer(cfg, handler)

	go serveMetrics(ctx, cfg, logger)

	errCh := make(chan error, 1)
	go func() {
		logger.Info("http server listening", "addr", cfg.HTTPAddr, "mode", mode)
		errCh <- server.ListenAndServe()
	}()

	select {
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Second)
		defer cancel()
		// Drain background goroutines (invite/welcome/password-reset emails)
		// before closing the HTTP server so in-flight sends are not lost.
		if err := bgTracker.Wait(shutdownCtx); err != nil {
			logger.Warn("background tracker drain incomplete", "error", err)
		}
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

func modeIncludesMailAPI(mode Mode) bool {
	return mode == ModeMailAPI || mode == ModeAllInOne
}

// jmapDraftSender adapts mailservice.Service to jmap.DraftSender.
type jmapDraftSender struct {
	svc *mailservice.Service
}

func (a *jmapDraftSender) SendDraft(ctx context.Context, userID, draftID string) error {
	_, err := a.svc.SendDraft(ctx, userID, draftID)
	return err
}

// jmapHandler creates a JMAP handler wired with real DB, storage, auth, and mail-send deps.
func jmapHandler(cfg config.Config, repo *maildb.Repository, store storage.Store, tm *auth.TokenManager, svc *mailservice.Service) *jmap.Handler {
	base := cfg.PublicBaseURL
	if base == "" {
		base = "http://localhost" + cfg.HTTPAddr
	}
	var sender jmap.DraftSender
	if svc != nil {
		sender = &jmapDraftSender{svc: svc}
	}
	deps := jmap.Deps{
		Repo:   repo,
		Store:  store,
		Auth:   tm,
		Sender: sender,
	}
	return jmap.NewHandler(deps, func(ctx context.Context, userID, accountID string) (*jmap.Session, error) {
		return jmap.BuildSession(userID, accountID, base), nil
	})
}

func modeIncludesAdminAPI(mode Mode) bool {
	return mode == ModeAdminAPI || mode == ModeAllInOne
}

func waitForShutdown(ctx context.Context, logger *slog.Logger, mode Mode) error {
	logger.Info("mode scaffold is ready; component implementation will be added next", "mode", mode)
	<-ctx.Done()
	return nil
}

type localDeliveryAdapter struct {
	repository *maildb.Repository
}

func (a localDeliveryAdapter) ResolveLocalRecipient(ctx context.Context, address string) (delivery.LocalRecipientLookup, error) {
	if a.repository == nil {
		return delivery.LocalRecipientLookup{}, fmt.Errorf("local delivery repository is required")
	}
	mailbox, domainLocal, err := a.repository.ResolveLocalRecipient(ctx, address)
	if err != nil {
		return delivery.LocalRecipientLookup{}, err
	}
	return delivery.LocalRecipientLookup{
		DomainLocal:     domainLocal,
		RecipientExists: domainLocal && mailbox.UserID != "",
		Mailbox: delivery.LocalMailbox{
			CompanyID: mailbox.CompanyID,
			DomainID:  mailbox.DomainID,
			UserID:    mailbox.UserID,
			Address:   mailbox.Address,
		},
	}, nil
}

func (a localDeliveryAdapter) DeliverLocal(ctx context.Context, job delivery.Job, _ outbound.Address, mailbox delivery.LocalMailbox) error {
	if a.repository == nil {
		return fmt.Errorf("local delivery repository is required")
	}
	body, err := job.OpenMessage(ctx)
	if err != nil {
		return err
	}
	defer body.Close()
	parsed, err := message.ParseEML(body)
	if err != nil {
		return err
	}
	_, err = a.repository.Record(ctx, smtpd.ReceivedMessage{
		EnvelopeFrom: strings.TrimSpace(job.From.Email),
		Mailbox: smtpd.Mailbox{
			CompanyID: mailbox.CompanyID,
			DomainID:  mailbox.DomainID,
			UserID:    mailbox.UserID,
			Address:   mailbox.Address,
		},
		StoragePath:      job.StoragePath,
		Parsed:           parsed,
		ReceivedAt:       time.Now().UTC(),
		Size:             job.Size,
		FolderSystemType: "inbox",
	})
	return err
}

// parsePublicHostname extracts the hostname from a public base URL.
// Returns an error if rawURL is empty or unparseable.
func parsePublicHostname(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("empty URL")
	}
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", err
	}
	host := parsed.Hostname()
	if host == "" {
		return "", fmt.Errorf("no hostname in URL %q", rawURL)
	}
	return host, nil
}
