package httpapi

import (
	"log/slog"
	"net/http"

	"github.com/gogomail/gogomail/internal/maildb"
)

func registerOperationsRoutes(mux *http.ServeMux, service AdminService, cfg adminRouteConfig, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/queue", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		stats, err := service.ListQueueStats(r.Context())
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"queues": stats})
	}))

	if cfg.dlqReader != nil {
		mux.HandleFunc("GET /admin/v1/dlq", adminAuth(func(w http.ResponseWriter, r *http.Request) {
			if !rejectUnknownQueryKeys(w, r, "stream", "count") {
				return
			}
			stream, ok := parseBoundedAdminQuery(w, r, "stream")
			if !ok {
				return
			}
			if stream == "" {
				writeError(w, http.StatusBadRequest, "stream is required")
				return
			}
			count, ok := parseQueryLimit(w, r)
			if !ok {
				return
			}
			entries, err := cfg.dlqReader.ListDLQ(r.Context(), stream, int64(count))
			if err != nil {
				slog.ErrorContext(r.Context(), "admin handler error", "error", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"dlq_entries": entries})
		}))

		mux.HandleFunc("DELETE /admin/v1/dlq/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
			if !rejectUnknownQueryKeys(w, r, "stream") {
				return
			}
			id, ok := parseBoundedAdminPathValue(w, r, "id")
			if !ok {
				return
			}
			stream, ok := parseBoundedAdminQuery(w, r, "stream")
			if !ok {
				return
			}
			if stream == "" {
				writeError(w, http.StatusBadRequest, "stream is required")
				return
			}
			if err := cfg.dlqReader.DeleteDLQEntry(r.Context(), stream, id); err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			w.WriteHeader(http.StatusNoContent)
		}))
	}

	mux.HandleFunc("POST /admin/v1/imap/mailboxes/{id}/uid-backfill", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "limit") {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/outbox-events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since", "topic", "partition_key", "status") {
			return
		}
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
		events, hasMore, err := service.ListOutboxEvents(r.Context(), maildb.OutboxEventListRequest{
			Limit:        limit,
			Topic:        topic,
			PartitionKey: partitionKey,
			Status:       status,
			Since:        since,
			ProbeMore:    true,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"outbox_events": events, "has_more": hasMore})
	}))

	mux.HandleFunc("GET /admin/v1/outbox-events/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
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

	mux.HandleFunc("GET /admin/v1/audit-logs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "category", "action", "action_prefix", "result", "target_type", "company_id", "domain_id", "user_id", "actor_id", "target_id", "since", "before") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseAuditLogListRequest(w, r, limit)
		if !ok {
			return
		}
		before, ok := parseOptionalRFC3339Query(w, r, "before")
		if !ok {
			return
		}
		req.Before = before
		req.ProbeMore = true
		logs, hasMore, err := service.ListAuditLogs(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_logs": logs, "has_more": hasMore})
	}))

	mux.HandleFunc("GET /admin/v1/audit-logs/integrity", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "since") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		view, err := service.CheckAuditLogIntegrity(r.Context(), maildb.AuditLogIntegrityRequest{
			Limit: limit,
			Since: since,
		})
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_log_integrity": view})
	}))

	mux.HandleFunc("GET /admin/v1/audit-logs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		log, err := service.GetAuditLog(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"audit_log": log})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "limit", "direction", "company_id", "domain_id", "user_id", "message_id", "rfc_message_id", "from_addr", "to_addr", "subject", "flow_status", "since", "until") {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		req, ok := parseMailFlowLogListRequest(w, r, limit)
		if !ok {
			return
		}
		logs, err := service.ListMailFlowLogs(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_logs": logs})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "direction", "company_id", "domain_id", "user_id", "since", "until") {
			return
		}
		req, ok := parseMailFlowLogStatsRequest(w, r)
		if !ok {
			return
		}
		stats, err := service.GetMailFlowLogStats(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/daily-stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "direction", "company_id", "domain_id", "user_id", "since", "until") {
			return
		}
		req, ok := parseMailFlowLogDailyStatsRequest(w, r)
		if !ok {
			return
		}
		stats, err := service.GetMailFlowLogDailyStats(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "admin handler error", "error", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_daily_stats": stats})
	}))

	mux.HandleFunc("GET /admin/v1/mail-flow-logs/{id}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		id, ok := parseBoundedAdminPathValue(w, r, "id")
		if !ok {
			return
		}
		log, err := service.GetMailFlowLog(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"mail_flow_log": log})
	}))
}
