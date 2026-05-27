package httpapi

import (
	"io"
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/maildb"
)

func registerUsageAndQuotaRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/api-usage/daily", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageAggregateQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageAggregateListRequest(w, r, limit)
		if !ok {
			return
		}
		if err := requiresCompanyAccess(r.Context(), req.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		usages, err := service.ListAPIUsageDaily(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_daily": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/monthly", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageAggregateQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageAggregateListRequest(w, r, limit)
		if !ok {
			return
		}
		if err := requiresCompanyAccess(r.Context(), req.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		usages, err := service.ListAPIUsageMonthly(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_monthly": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerQuery(w, r) {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger": usages})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerQuery(w, r) {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageLedgerStatsQuery(w, r) {
			return
		}
		req, ok := parseAPIUsageLedgerListRequest(w, r, 0)
		if !ok {
			return
		}
		stats, err := service.GetAPIUsageLedgerStats(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageRetentionReadinessQuery(w, r) {
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRequest(w, r)
		if !ok {
			return
		}
		readiness, err := service.GetAPIUsageLedgerRetentionReadiness(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_readiness": readiness})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/ledger/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()

		var body adminAPIUsageLedgerRetentionRunRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRunRequest(w, body)
		if !ok {
			return
		}
		run, err := service.RunAPIUsageLedgerRetention(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageRetentionRunListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageLedgerRetentionRunListRequest(w, r, limit)
		if !ok {
			return
		}
		runs, err := service.ListAPIUsageLedgerRetentionRuns(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_runs": runs})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/ledger/retention-runs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		run, err := service.GetAPIUsageLedgerRetentionRun(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_ledger_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownDAVSyncRetentionReadinessQuery(w, r) {
			return
		}
		req, ok := parseDAVSyncRetentionReadinessRequest(w, r)
		if !ok {
			return
		}
		readiness, err := service.GetDAVSyncRetentionReadiness(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_readiness": readiness})
	}))

	mux.HandleFunc("POST /admin/v1/dav-sync/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		defer r.Body.Close()

		var body adminDAVSyncRetentionRunRequest
		if err := decodeJSONBody(r, &body); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		req, ok := parseDAVSyncRetentionRunRequest(w, body)
		if !ok {
			return
		}
		run, err := service.RunDAVSyncRetention(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-runs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownDAVSyncRetentionRunListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseDAVSyncRetentionRunListRequest(w, r, limit)
		if !ok {
			return
		}
		runs, err := service.ListDAVSyncRetentionRuns(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_runs": runs})
	}))

	mux.HandleFunc("GET /admin/v1/dav-sync/retention-runs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		run, err := service.GetDAVSyncRetentionRun(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"dav_sync_retention_run": run})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-capabilities", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		capabilities, err := service.GetAPIUsageExportCapabilities(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_capabilities": capabilities})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownAPIUsageExportBatchCreateQuery(w, r) {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"api_usage_export_batch": batch})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownAPIUsageExportBatchListQuery(w, r) {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAPIUsageExportBatchListRequest(w, r, limit)
		if !ok {
			return
		}
		batches, err := service.ListAPIUsageExportBatches(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_batches": batches})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/handoff-readiness", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "deep") {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/export", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		writeNDJSON(w, http.StatusOK, usages)
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_artifacts": artifacts})
	}))

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/artifacts/write", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/download", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/artifacts/{artifact_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_digests": digests})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("POST /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
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
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"api_usage_export_manifest_signatures": signatures})
	}))

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/api-usage/export-batches/{id}/manifest-digests/{digest_id}/signatures/{signature_id}/verification", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/quota-reconciliation", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		reconciliation, err := service.ListQuotaReconciliation(r.Context(), limit)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_reconciliation": reconciliation})
	}))

	mux.HandleFunc("POST /admin/v1/quota-reconciliation/corrections", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/quota-alert-thresholds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "scope") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		if err := requiresCompanyAccess(r.Context(), companyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		scope, ok := parseBoundedAdminQuery(w, r, "scope")
		if !ok {
			return
		}
		thresholds, err := service.ListQuotaAlertThresholds(r.Context(), maildb.QuotaAlertThresholdListRequest{
			Limit:     limit,
			CompanyID: companyID,
			Scope:     scope,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_thresholds": thresholds})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		threshold, err := service.GetQuotaAlertThreshold(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), threshold.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("POST /admin/v1/quota-alert-thresholds", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		var req maildb.CreateQuotaAlertThresholdRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := requiresCompanyAccess(r.Context(), req.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		threshold, err := service.CreateQuotaAlertThreshold(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("PATCH /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetQuotaAlertThreshold(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		var req maildb.UpdateQuotaAlertThresholdRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.ID = id
		threshold, err := service.UpdateQuotaAlertThreshold(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert_threshold": threshold})
	}))

	mux.HandleFunc("DELETE /admin/v1/quota-alert-thresholds/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		existing, err := service.GetQuotaAlertThreshold(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), existing.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		if err := service.DeleteQuotaAlertThreshold(r.Context(), id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alerts", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "company_id", "domain_id", "user_id", "scope", "alert_type", "since", "until") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		companyID, ok := parseBoundedAdminQuery(w, r, "company_id")
		if !ok {
			return
		}
		if err := requiresCompanyAccess(r.Context(), companyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		domainID, ok := parseBoundedAdminQuery(w, r, "domain_id")
		if !ok {
			return
		}
		userID, ok := parseBoundedAdminQuery(w, r, "user_id")
		if !ok {
			return
		}
		scope, ok := parseBoundedAdminQuery(w, r, "scope")
		if !ok {
			return
		}
		alertType, ok := parseBoundedAdminQuery(w, r, "alert_type")
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		until, ok := parseOptionalRFC3339Query(w, r, "until")
		if !ok {
			return
		}
		alerts, err := service.ListQuotaAlerts(r.Context(), maildb.QuotaAlertListRequest{
			Limit:     limit,
			CompanyID: companyID,
			DomainID:  domainID,
			UserID:    userID,
			Scope:     scope,
			AlertType: alertType,
			Since:     since,
			Until:     until,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alerts": alerts})
	}))

	mux.HandleFunc("GET /admin/v1/quota-alerts/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		alert, err := service.GetQuotaAlert(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		if err := requiresCompanyAccess(r.Context(), alert.CompanyID); err != nil {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"quota_alert": alert})
	}))
}
