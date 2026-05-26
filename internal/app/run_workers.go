package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/batchlock"
	"github.com/gogomail/gogomail/internal/caldavgw"
	"github.com/gogomail/gogomail/internal/carddavgw"
	"github.com/gogomail/gogomail/internal/config"
	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/davsyncretention"
	"github.com/gogomail/gogomail/internal/directory"
	dsnpkg "github.com/gogomail/gogomail/internal/dsn"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/imapnotify"
	"github.com/gogomail/gogomail/internal/inboundfilter"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailflow"
	"github.com/gogomail/gogomail/internal/mailservice"
	"github.com/gogomail/gogomail/internal/observability"
	"github.com/gogomail/gogomail/internal/orgchart"
	"github.com/gogomail/gogomail/internal/outbound"
	"github.com/gogomail/gogomail/internal/outbox"
	"github.com/gogomail/gogomail/internal/scheduling"
	"github.com/gogomail/gogomail/internal/searchindex"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

func runBatchWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
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
	quotaSystemEmailSender := mailservice.NewSMTPSystemEmailSender(cfg.SystemEmail.From, cfg.SystemEmail.SMTPAddr, cfg.SystemEmail.SMTPUser, cfg.SystemEmail.SMTPPass)
	registry.Register("quota-alert-check", func() error {
		n, err := quotaAlertRepository.ScanAndRecordQuotaAlerts(ctx, 0.80, 0.95)
		if err != nil {
			logger.Error("quota alert check failed", "error", err)
			return err
		}
		emailAlerts, err := quotaAlertRepository.ListPendingUserQuotaAlertEmails(ctx, 100)
		if err != nil {
			logger.Error("quota alert email lookup failed", "error", err)
			return err
		}
		sent := 0
		for _, alert := range emailAlerts {
			if err := quotaSystemEmailSender.SendQuotaAlert(ctx, alert.Email, alert.Pct); err != nil {
				logger.Warn("quota alert email failed", "alert_id", alert.ID, "error", err)
				continue
			}
			if err := quotaAlertRepository.MarkQuotaAlertNotified(ctx, alert.ID); err != nil {
				logger.Warn("quota alert notify mark failed", "alert_id", alert.ID, "error", err)
				continue
			}
			sent++
		}
		logger.Info("quota alert check completed", "alerts", n, "emails_sent", sent)
		return nil
	}, 15*time.Minute)

	if cfg.AutoPurgeEnabled {
		autoPurgeRepository := maildb.NewRepository(db)
		registry.Register("auto-purge", func() error {
			result, err := autoPurgeRepository.RunAutoPurge(ctx, cfg.AutoPurgeBatchSize)
			if err != nil {
				logger.Error("auto purge failed", "error", err)
				return err
			}
			logger.Info("auto purge completed", "companies", result.CompaniesScanned, "messages_deleted", result.MessagesDeleted, "audit_logs_deleted", result.AuditLogsDeleted)
			return nil
		}, cfg.AutoPurgeInterval)
	}

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

	// org-chart-sync job is not registered: no OrgChartSyncAdapter implementation
	// is wired into the runtime yet. When an adapter is added, register the job
	// here (interval 1h, handler invokes adapter.SyncOrgChart(ctx)).
	_ = orgchart.OrgChartSyncAdapter(nil)

	worker := batchlock.NewWorker(registry, db)
	worker.Start()

	logger.Info("batch worker started", "jobs", len(registry.List()))

	<-ctx.Done()

	logger.Info("shutting down batch worker")
	worker.Stop()
	logger.Info("batch worker stopped")

	return ctx.Err()
}

func runAttachmentCleanupWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
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
	db, err := openDatabase(ctx, cfg)
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
	db, err := openDatabase(ctx, cfg)
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

func runAPIUsageRetentionWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
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
func runOutboxRelay(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	tp, err := observability.InitTracing(ctx, observability.TracingConfig{
		Enabled:          cfg.OTelEnabled,
		ExporterEndpoint: cfg.OTelEndpoint,
		ServiceName:      cfg.OTelServiceName + "-outbox",
		ServiceVersion:   cfg.OTelServiceVersion,
	})
	if err != nil {
		logger.Warn("tracing init failed in outbox relay", "error", err)
	} else {
		defer func() { _ = tp.Shutdown(context.Background()) }()
	}

	db, err := openDatabase(ctx, cfg)
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

	// Choose store: sharded (for partition-ordered multi-process scaling) or plain.
	var store outbox.Store
	if cfg.OutboxRelayShardTotal > 1 {
		sharded, err := outbox.NewShardedPostgresStore(db, cfg.OutboxRelayMaxAttempts, cfg.OutboxRelayShardTotal, cfg.OutboxRelayShardIndex)
		if err != nil {
			return fmt.Errorf("create sharded outbox store: %w", err)
		}
		store = sharded
		logger.Info("outbox relay sharding enabled",
			"shard_index", cfg.OutboxRelayShardIndex,
			"shard_total", cfg.OutboxRelayShardTotal,
		)
	} else {
		store = outbox.NewPostgresStore(db, cfg.OutboxRelayMaxAttempts)
	}

	relay, err := outbox.NewRelay(outbox.RelayOptions{
		Store:        store,
		Publisher:    outbox.NewRedisStreamPublisher(redisClient, cfg.EventStream),
		BatchSize:    cfg.OutboxRelayBatchSize,
		PollInterval: cfg.OutboxRelayPollInterval,
		WorkerCount:  cfg.OutboxRelayWorkerCount,
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
		"workers", cfg.OutboxRelayWorkerCount,
	)
	return relay.Run(ctx)
}

func runEventWorker(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	db, err := openDatabase(ctx, cfg)
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

