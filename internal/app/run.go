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
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"

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
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/directory"
	"github.com/gogomail/gogomail/internal/dkim"
	dmpkg "github.com/gogomail/gogomail/internal/dm"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/eventstream"
	"github.com/gogomail/gogomail/internal/httpapi"
	"github.com/gogomail/gogomail/internal/jmap"
	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/ldapgw"
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
			if shutErr := tp.Shutdown(context.Background()); shutErr != nil {
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
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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

func openDatabase(ctx context.Context, cfg config.Config) (*sql.DB, error) {
	return database.Open(ctx, cfg.DatabaseURL, database.Options{
		MaxOpenConns:    cfg.DBMaxOpenConns,
		MaxIdleConns:    cfg.DBMaxIdleConns,
		ConnMaxLifetime: cfg.DBConnMaxLifetime,
		ConnMaxIdleTime: cfg.DBConnMaxIdleTime,
	})
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
