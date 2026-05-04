package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/backpressure"
	"github.com/gogomail/gogomail/internal/delivery"
	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/maildb"
)

type adminRouteConfig struct {
	routeCounters *delivery.RouteCounters
}

// AdminRouteOption configures optional capabilities for RegisterAdminRoutes.
type AdminRouteOption func(*adminRouteConfig)

// WithRouteCounters enables the GET /admin/v1/delivery-routes/counters endpoint.
func WithRouteCounters(c *delivery.RouteCounters) AdminRouteOption {
	return func(cfg *adminRouteConfig) { cfg.routeCounters = c }
}

type AdminService interface {
	ListCompanies(ctx context.Context, limit int) ([]maildb.CompanyView, error)
	GetCompany(ctx context.Context, id string) (maildb.CompanyView, error)
	UpdateCompanyQuota(ctx context.Context, req maildb.UpdateCompanyQuotaRequest) error
	ListDomains(ctx context.Context, limit int) ([]maildb.DomainView, error)
	GetDomain(ctx context.Context, id string) (maildb.DomainView, error)
	GetDomainStats(ctx context.Context, id string) (maildb.DomainStatsView, error)
	VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error)
	ListDomainDNSChecks(ctx context.Context, id string, limit int) ([]maildb.DomainDNSCheckView, error)
	CreateDomain(ctx context.Context, req maildb.CreateDomainRequest) (maildb.DomainView, error)
	UpdateDomainStatus(ctx context.Context, req maildb.UpdateDomainStatusRequest) error
	UpdateDomainQuota(ctx context.Context, req maildb.UpdateDomainQuotaRequest) error
	UpdateDomainPolicy(ctx context.Context, req maildb.UpdateDomainPolicyRequest) (maildb.DomainPolicyView, error)
	ListUsers(ctx context.Context, domainID string, limit int) ([]maildb.UserView, error)
	GetUser(ctx context.Context, id string) (maildb.UserView, error)
	CreateUser(ctx context.Context, req maildb.CreateUserRequest) (maildb.UserView, error)
	UpdateUserStatus(ctx context.Context, req maildb.UpdateUserStatusRequest) error
	UpdateUserQuota(ctx context.Context, req maildb.UpdateUserQuotaRequest) error
	ListQueueStats(ctx context.Context) ([]maildb.QueueStat, error)
	ListOutboxEvents(ctx context.Context, req maildb.OutboxEventListRequest) ([]maildb.OutboxEventView, error)
	GetOutboxEvent(ctx context.Context, id string) (maildb.OutboxEventView, error)
	ListQuotaUsage(ctx context.Context, limit int) ([]maildb.QuotaUsageView, error)
	ListAPIUsageDaily(ctx context.Context, limit int) ([]maildb.APIUsageDailyView, error)
	ListAPIUsageMonthly(ctx context.Context, limit int) ([]maildb.APIUsageMonthlyView, error)
	ListAPIUsageLedger(ctx context.Context, req maildb.APIUsageLedgerListRequest) ([]maildb.APIUsageLedgerView, error)
	GetAPIUsageLedgerStats(ctx context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageLedgerStatsView, error)
	GetAPIUsageLedgerRetentionReadiness(ctx context.Context, req maildb.APIUsageLedgerRetentionRequest) (maildb.APIUsageLedgerRetentionReadinessView, error)
	GetAPIUsageExportCapabilities(ctx context.Context) (maildb.APIUsageExportCapabilityView, error)
	CreateAPIUsageExportBatch(ctx context.Context, req maildb.APIUsageLedgerListRequest) (maildb.APIUsageExportBatchView, error)
	ListAPIUsageExportBatches(ctx context.Context, limit int) ([]maildb.APIUsageExportBatchView, error)
	GetAPIUsageExportBatch(ctx context.Context, id string) (maildb.APIUsageExportBatchView, error)
	GetAPIUsageExportHandoff(ctx context.Context, batchID string, deep bool) (maildb.APIUsageExportHandoffView, error)
	CreateAPIUsageExportArtifact(ctx context.Context, req maildb.CreateAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error)
	WriteAPIUsageExportArtifact(ctx context.Context, batchID string, req maildb.WriteAPIUsageExportArtifactRequest) (maildb.APIUsageExportArtifactView, error)
	ListAPIUsageExportArtifacts(ctx context.Context, batchID string, limit int) ([]maildb.APIUsageExportArtifactView, error)
	GetAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, error)
	OpenAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactView, io.ReadCloser, error)
	VerifyAPIUsageExportArtifact(ctx context.Context, batchID string, artifactID string) (maildb.APIUsageExportArtifactVerificationView, error)
	CreateAPIUsageExportManifestDigest(ctx context.Context, batchID string) (maildb.APIUsageExportManifestDigestView, error)
	ListAPIUsageExportManifestDigests(ctx context.Context, batchID string, limit int) ([]maildb.APIUsageExportManifestDigestView, error)
	GetAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestView, error)
	VerifyAPIUsageExportManifestDigest(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestDigestVerificationView, error)
	CreateAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string) (maildb.APIUsageExportManifestSignatureView, error)
	ListAPIUsageExportManifestSignatures(ctx context.Context, batchID string, digestID string, limit int) ([]maildb.APIUsageExportManifestSignatureView, error)
	GetAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureView, error)
	VerifyAPIUsageExportManifestSignature(ctx context.Context, batchID string, digestID string, signatureID string) (maildb.APIUsageExportManifestSignatureVerificationView, error)
	ListQuotaReconciliation(ctx context.Context, limit int) ([]maildb.QuotaReconciliationView, error)
	CorrectQuotaReconciliation(ctx context.Context, req maildb.CorrectQuotaReconciliationRequest) (maildb.QuotaCorrectionResult, error)
	ListDeliveryAttempts(ctx context.Context, req maildb.DeliveryAttemptListRequest) ([]maildb.DeliveryAttemptView, error)
	GetDeliveryAttemptStats(ctx context.Context, req maildb.DeliveryAttemptStatsRequest) (maildb.DeliveryAttemptStatsView, error)
	ListExhaustedAttempts(ctx context.Context, req maildb.ExhaustedAttemptListRequest) ([]maildb.DeliveryAttemptView, error)
	ListPushNotificationAttempts(ctx context.Context, req maildb.PushNotificationAttemptListRequest) ([]maildb.PushNotificationAttemptView, error)
	GetPushNotificationStats(ctx context.Context, req maildb.PushNotificationStatsRequest) (maildb.PushNotificationStatsView, error)
	ListSuppressionEntries(ctx context.Context, limit int) ([]maildb.SuppressionEntry, error)
	ListTrustedRelays(ctx context.Context, limit int) ([]maildb.TrustedRelayView, error)
	CreateTrustedRelay(ctx context.Context, req maildb.CreateTrustedRelayRequest) (maildb.TrustedRelayView, error)
	DeleteTrustedRelay(ctx context.Context, id string) error
	ListDeliveryRoutes(ctx context.Context, limit int) ([]maildb.DeliveryRouteView, error)
	CreateDeliveryRoute(ctx context.Context, req maildb.CreateDeliveryRouteRequest) (maildb.DeliveryRouteView, error)
	ResolveDeliveryRoute(ctx context.Context, domain string) (maildb.DeliveryRouteResolveView, error)
	UpdateDeliveryRouteStatus(ctx context.Context, req maildb.UpdateDeliveryRouteStatusRequest) error
	DeleteDeliveryRoute(ctx context.Context, id string) error
	ListDKIMKeys(ctx context.Context, domainID string, limit int) ([]maildb.DKIMKeyView, error)
	CreateDKIMKey(ctx context.Context, input maildb.CreateDKIMKeyInput) (string, error)
	DeactivateDKIMKey(ctx context.Context, id string) error
	VerifyDKIMKeyDNS(ctx context.Context, keyID string) (maildb.DKIMKeyDNSVerificationResult, error)
	RetryOutbox(ctx context.Context, id string) error
	DeleteSuppressionEntry(ctx context.Context, id string) error
	BackfillIMAPMailboxUIDs(ctx context.Context, userID string, mailboxID string, limit int) ([]maildb.IMAPMessageUID, error)
}

type adminIMAPUIDBackfillItem struct {
	MessageID string `json:"message_id"`
	MailboxID string `json:"mailbox_id"`
	UID       uint32 `json:"uid"`
	ModSeq    uint64 `json:"modseq"`
}

type AdminBackpressureService interface {
	GetBackpressure(ctx context.Context) (backpressure.State, error)
	UpdateBackpressure(ctx context.Context, req backpressure.StateUpdate) (backpressure.State, error)
}

func RegisterAdminRoutes(mux *http.ServeMux, service AdminService, token string, opts ...AdminRouteOption) {
	var cfg adminRouteConfig
	for _, opt := range opts {
		opt(&cfg)
	}
	if cfg.routeCounters != nil {
		mux.HandleFunc("GET /admin/v1/delivery-routes/counters", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]any{"route_counters": cfg.routeCounters.Snapshot()})
		}))
	}

	mux.HandleFunc("GET /admin/v1/companies", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companies, err := service.ListCompanies(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"companies": companies})
	}))

	mux.HandleFunc("GET /admin/v1/companies/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		company, err := service.GetCompany(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"company": company})
	}))

	mux.HandleFunc("PATCH /admin/v1/companies/{id}/quota", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateCompanyQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateCompanyQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/domains", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domains, err := service.ListDomains(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domains": domains})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		domain, err := service.GetDomain(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/stats", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		stats, err := service.GetDomainStats(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-check", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		report, err := service.VerifyDomainDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_check": report})
	}))

	mux.HandleFunc("GET /admin/v1/domains/{id}/dns-checks", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		checks, err := service.ListDomainDNSChecks(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dns_checks": checks})
	}))

	mux.HandleFunc("POST /admin/v1/domains", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateDomainRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		domain, err := service.CreateDomain(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"domain": domain})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDomainStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/quota", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDomainQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/domains/{id}/policy", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDomainPolicyRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		policy, err := service.UpdateDomainPolicy(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"domain_policy": policy})
	}))

	mux.HandleFunc("GET /admin/v1/users", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		users, err := service.ListUsers(r.Context(), domainID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"users": users})
	}))

	mux.HandleFunc("GET /admin/v1/users/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		user, err := service.GetUser(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"user": user})
	}))

	mux.HandleFunc("POST /admin/v1/users", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateUserRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		user, err := service.CreateUser(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"user": user})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateUserStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("PATCH /admin/v1/users/{id}/quota", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateUserQuotaRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateUserQuota(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/queue", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		stats, err := service.ListQueueStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"queues": stats})
	}))

	mux.HandleFunc("POST /admin/v1/imap/mailboxes/{id}/uid-backfill", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		mailboxID, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if userID == "" {
			writeError(w, http.StatusBadRequest, "user_id is required")
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		assigned, err := service.BackfillIMAPMailboxUIDs(r.Context(), userID, mailboxID, limit)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		items := make([]adminIMAPUIDBackfillItem, 0, len(assigned))
		for _, item := range assigned {
			items = append(items, adminIMAPUIDBackfillItem{
				MessageID: string(item.MessageID),
				MailboxID: string(item.MailboxID),
				UID:       uint32(item.UID),
				ModSeq:    item.ModSeq,
			})
		}
		writeJSON(w, http.StatusOK, map[string]any{"imap_uid_backfill": items})
	}))

	mux.HandleFunc("GET /admin/v1/outbox-events", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		topic, ok := parseBoundedAdminQuery(w, r, "topic")
		if !ok {
			return
		}
		partitionKey, ok := parseBoundedAdminQuery(w, r, "partition_key")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		events, err := service.ListOutboxEvents(r.Context(), maildb.OutboxEventListRequest{
			Limit:        limit,
			Topic:        topic,
			PartitionKey: partitionKey,
			Status:       status,
			Since:        since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbox_events": events})
	}))

	mux.HandleFunc("GET /admin/v1/outbox-events/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		event, err := service.GetOutboxEvent(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbox_event": event})
	}))

	mux.HandleFunc("GET /admin/v1/backpressure", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		state, err := backpressureService.GetBackpressure(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))

	mux.HandleFunc("PATCH /admin/v1/backpressure", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		backpressureService, ok := service.(AdminBackpressureService)
		if !ok {
			writeError(w, http.StatusNotFound, "backpressure backend is not configured")
			return
		}
		var req backpressure.StateUpdate
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		state, err := backpressureService.UpdateBackpressure(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"backpressure": state})
	}))

	mux.HandleFunc("GET /admin/v1/quota-usage", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		usages, err := service.ListQuotaUsage(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_usage": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/daily", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageDaily(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_daily": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/monthly", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageMonthly(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_monthly": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/export", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, limit)
		if !ok {
			return
		}
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/stats", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		req, ok := parseAPIUsageLedgerListRequest(w, r, 0)
		if !ok {
			return
		}
		stats, err := service.GetAPIUsageLedgerStats(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-readiness", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		req, ok := parseAPIUsageLedgerRetentionRequest(w, r)
		if !ok {
			return
		}
		readiness, err := service.GetAPIUsageLedgerRetentionReadiness(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_readiness": readiness})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-capabilities", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		capabilities, err := service.GetAPIUsageExportCapabilities(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_capabilities": capabilities})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		req, ok := parseAPIUsageLedgerListRequest(w, r, 0)
		if !ok {
			return
		}
		if req.From.IsZero() || req.To.IsZero() {
			writeError(w, http.StatusBadRequest, "from and to are required")
			return
		}
		batch, err := service.CreateAPIUsageExportBatch(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_batch": batch})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		batches, err := service.ListAPIUsageExportBatches(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_batches": batches})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		batch, err := service.GetAPIUsageExportBatch(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_batch": batch})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/handoff-readiness", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		deep, ok := parseBoolQueryDefaultFalse(w, r, "deep")
		if !ok {
			return
		}
		handoff, err := service.GetAPIUsageExportHandoff(r.Context(), id, deep)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_handoff_readiness": handoff})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/export", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		batch, err := service.GetAPIUsageExportBatch(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		req := apiUsageLedgerRequestFromBatch(batch, limit)
		usages, err := service.ListAPIUsageLedger(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.CreateAPIUsageExportArtifactRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.BatchID = id
		artifact, err := service.CreateAPIUsageExportArtifact(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		artifacts, err := service.ListAPIUsageExportArtifacts(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifacts": artifacts})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts/write", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		var req maildb.WriteAPIUsageExportArtifactRequest
		if r.ContentLength != 0 {
			if err := decodeJSONBody(r, &req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid JSON body")
				return
			}
		}
		artifact, err := service.WriteAPIUsageExportArtifact(r.Context(), id, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		artifact, err := service.GetAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifact": artifact})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/download", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		artifact, body, err := service.OpenAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", safeContentType(artifact.ContentType, "application/x-ndjson"))
		if sha256Hex := safeSHA256Header(artifact.SHA256Hex); sha256Hex != "" {
			w.Header().Set("X-Gogomail-Artifact-SHA256", sha256Hex)
		}
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, body)
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/verification", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, artifactID, ok := parseBoundedAdminPathPair(w, r, "id", "artifact_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportArtifact(r.Context(), id, artifactID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifact_verification": verification})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		digest, err := service.CreateAPIUsageExportManifestDigest(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_manifest_digest": digest})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		digests, err := service.ListAPIUsageExportManifestDigests(r.Context(), id, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digests": digests})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		digest, err := service.GetAPIUsageExportManifestDigest(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digest": digest})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportManifestDigest(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digest_verification": verification})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		signature, err := service.CreateAPIUsageExportManifestSignature(r.Context(), id, digestID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_manifest_signature": signature})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, ok := parseBoundedAdminPathPair(w, r, "id", "digest_id")
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		signatures, err := service.ListAPIUsageExportManifestSignatures(r.Context(), id, digestID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signatures": signatures})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, signatureID, ok := parseBoundedAdminPathTriple(w, r, "id", "digest_id", "signature_id")
		if !ok {
			return
		}
		signature, err := service.GetAPIUsageExportManifestSignature(r.Context(), id, digestID, signatureID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signature": signature})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, digestID, signatureID, ok := parseBoundedAdminPathTriple(w, r, "id", "digest_id", "signature_id")
		if !ok {
			return
		}
		verification, err := service.VerifyAPIUsageExportManifestSignature(r.Context(), id, digestID, signatureID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signature_verification": verification})
	}))

	mux.HandleFunc("GET /admin/v1/quota-reconciliation", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		reconciliation, err := service.ListQuotaReconciliation(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_reconciliation": reconciliation})
	}))

	mux.HandleFunc("POST /admin/v1/quota-reconciliation/corrections", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CorrectQuotaReconciliationRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		result, err := service.CorrectQuotaReconciliation(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_correction": result})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		attempts, err := service.ListDeliveryAttempts(r.Context(), maildb.DeliveryAttemptListRequest{
			Limit:           limit,
			Status:          status,
			RecipientDomain: recipientDomain,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/stats", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		stats, err := service.GetDeliveryAttemptStats(r.Context(), maildb.DeliveryAttemptStatsRequest{
			Status:          status,
			RecipientDomain: recipientDomain,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_attempt_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-attempts/exhausted", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		recipientDomain, ok := parseBoundedAdminQuery(w, r, "recipient_domain")
		if !ok {
			return
		}
		attempts, err := service.ListExhaustedAttempts(r.Context(), maildb.ExhaustedAttemptListRequest{
			Limit:           limit,
			RecipientDomain: recipientDomain,
			Since:           since,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"exhausted_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-attempts", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		status, ok := parseBoundedAdminQuery(w, r, "status")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		platform, ok := parseBoundedAdminQuery(w, r, "platform")
		if !ok {
			return
		}
		deviceID, ok := parseBoundedAdminQuery(w, r, "device_id")
		if !ok {
			return
		}
		providerStatus, ok := parseBoundedAdminQuery(w, r, "provider_status")
		if !ok {
			return
		}
		providerMessageID, ok := parseBoundedAdminQuery(w, r, "provider_message_id")
		if !ok {
			return
		}
		attempts, err := service.ListPushNotificationAttempts(r.Context(), maildb.PushNotificationAttemptListRequest{
			Limit:             limit,
			Status:            status,
			UserID:            userID,
			Platform:          platform,
			DeviceID:          deviceID,
			ProviderStatus:    providerStatus,
			ProviderMessageID: providerMessageID,
			Since:             since,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_attempts": attempts})
	}))

	mux.HandleFunc("GET /admin/v1/push-notification-stats", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		stats, err := service.GetPushNotificationStats(r.Context(), maildb.PushNotificationStatsRequest{
			UserID: userID,
			Since:  since,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_notification_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/suppression-list", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		entries, err := service.ListSuppressionEntries(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"suppression_list": entries})
	}))

	mux.HandleFunc("GET /admin/v1/trusted-relays", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		relays, err := service.ListTrustedRelays(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"trusted_relays": relays})
	}))

	mux.HandleFunc("POST /admin/v1/trusted-relays", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateTrustedRelayRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		relay, err := service.CreateTrustedRelay(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"trusted_relay": relay})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-routes", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		routes, err := service.ListDeliveryRoutes(r.Context(), limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_routes": routes})
	}))

	mux.HandleFunc("POST /admin/v1/delivery-routes", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.CreateDeliveryRouteRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		route, err := service.CreateDeliveryRoute(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"delivery_route": route})
	}))

	mux.HandleFunc("GET /admin/v1/delivery-routes/resolve", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		domain, ok := parseBoundedAdminQuery(w, r, "domain")
		if !ok {
			return
		}
		result, err := service.ResolveDeliveryRoute(r.Context(), domain)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_route_resolution": result})
	}))

	mux.HandleFunc("PATCH /admin/v1/delivery-routes/{id}/status", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req maildb.UpdateDeliveryRouteStatusRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		req.ID = id
		if err := service.UpdateDeliveryRouteStatus(r.Context(), req); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": req.ID})
	}))

	mux.HandleFunc("GET /admin/v1/dkim-keys", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		keys, err := service.ListDKIMKeys(r.Context(), domainID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_keys": keys})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var input maildb.CreateDKIMKeyInput
		if err := decodeJSONBody(r, &input); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		id, err := service.CreateDKIMKey(r.Context(), input)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/dkim-keys/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeactivateDKIMKey(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("POST /admin/v1/dkim-keys/{id}/verify-dns", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		result, err := service.VerifyDKIMKeyDNS(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dkim_verification": result})
	}))

	mux.HandleFunc("POST /admin/v1/outbox/{id}/retry", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.RetryOutbox(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/suppression-list/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteSuppressionEntry(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/trusted-relays/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteTrustedRelay(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))

	mux.HandleFunc("DELETE /admin/v1/delivery-routes/{id}", adminAuth(token, func(w http.ResponseWriter, r *http.Request) {
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteDeliveryRoute(r.Context(), id); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	}))
}

func adminAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	token = strings.TrimSpace(token)
	if token == "" {
		return next
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !constantTimeTokenEqual(adminTokenFromRequest(r), token) {
			writeError(w, http.StatusUnauthorized, "admin token is required")
			return
		}
		next(w, r)
	}
}

func constantTimeTokenEqual(got string, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	if got == "" || want == "" {
		return false
	}
	gotHash := sha256.Sum256([]byte(got))
	wantHash := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gotHash[:], wantHash[:]) == 1
}

func safeSHA256Header(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 64 {
		return ""
	}
	for _, r := range value {
		if (r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') {
			continue
		}
		return ""
	}
	return value
}

func parseAPIUsageLedgerListRequest(w http.ResponseWriter, r *http.Request, limit int) (maildb.APIUsageLedgerListRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}
	from, ok := parseOptionalRFC3339Query(w, r, "from")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	to, ok := parseOptionalRFC3339Query(w, r, "to")
	if !ok {
		return maildb.APIUsageLedgerListRequest{}, false
	}
	req.From = from
	req.To = to
	if !req.From.IsZero() && !req.To.IsZero() && !req.From.Before(req.To) {
		writeError(w, http.StatusBadRequest, "from must be before to")
		return maildb.APIUsageLedgerListRequest{}, false
	}
	return req, true
}

func parseAPIUsageLedgerRetentionRequest(w http.ResponseWriter, r *http.Request) (maildb.APIUsageLedgerRetentionRequest, bool) {
	tenantID, ok := parseBoundedAdminQuery(w, r, "tenant_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	principalID, ok := parseBoundedAdminQuery(w, r, "principal_id")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	cutoff, ok := parseOptionalRFC3339Query(w, r, "cutoff")
	if !ok {
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.IsZero() {
		writeError(w, http.StatusBadRequest, "cutoff is required")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	if cutoff.After(time.Now().UTC()) {
		writeError(w, http.StatusBadRequest, "cutoff must not be in the future")
		return maildb.APIUsageLedgerRetentionRequest{}, false
	}
	return maildb.APIUsageLedgerRetentionRequest{
		Cutoff:      cutoff,
		TenantID:    tenantID,
		PrincipalID: principalID,
	}, true
}

func apiUsageLedgerRequestFromBatch(batch maildb.APIUsageExportBatchView, limit int) maildb.APIUsageLedgerListRequest {
	req := maildb.APIUsageLedgerListRequest{
		Limit:       limit,
		TenantID:    batch.TenantID,
		PrincipalID: batch.PrincipalID,
	}
	if batch.WindowStart != nil {
		req.From = batch.WindowStart.UTC()
	}
	if batch.WindowEnd != nil {
		req.To = batch.WindowEnd.UTC()
	}
	return req
}

func parseOptionalRFC3339Query(w http.ResponseWriter, r *http.Request, key string) (time.Time, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get(key))
	if raw == "" {
		return time.Time{}, true
	}
	value, err := time.Parse(time.RFC3339Nano, raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, key+" must be RFC3339 timestamp")
		return time.Time{}, false
	}
	return value.UTC(), true
}

const maxAdminQueryFilterBytes = 1024

func parseBoundedAdminQuery(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.URL.Query().Get(key))
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxAdminQueryFilterBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedAdminPathPair(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string) (string, string, bool) {
	first, ok := parseBoundedAdminPathValue(w, r, firstKey)
	if !ok {
		return "", "", false
	}
	second, ok := parseBoundedAdminPathValue(w, r, secondKey)
	if !ok {
		return "", "", false
	}
	return first, second, true
}

func parseBoundedAdminPathTriple(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string, thirdKey string) (string, string, string, bool) {
	first, second, ok := parseBoundedAdminPathPair(w, r, firstKey, secondKey)
	if !ok {
		return "", "", "", false
	}
	third, ok := parseBoundedAdminPathValue(w, r, thirdKey)
	if !ok {
		return "", "", "", false
	}
	return first, second, third, true
}

func adminTokenFromRequest(r *http.Request) string {
	if value := strings.TrimSpace(r.Header.Get("X-Admin-Token")); value != "" {
		return value
	}
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(auth), "bearer ") {
		return strings.TrimSpace(auth[len("bearer "):])
	}
	return ""
}
