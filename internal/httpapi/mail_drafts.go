package httpapi

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

func registerDraftRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	searchLimiter := NewAdminIPRateLimiter(30, time.Minute)
	_ = searchLimiter // used in drafts/search handler below
	mux.HandleFunc("POST /api/v1/drafts", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		var req mailservice.SaveDraftRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if !allowAPIKeyRequest(w, r, opts.APIKeyLimiter) {
			return
		}
		if !bindRequestUserID(w, r, tokenManager, service, &req.UserID, req.UserEmail) {
			return
		}
		if !allowMailMutationRequest(w, r, opts, req.UserID, "save_draft") {
			return
		}
		draft, err := service.SaveDraft(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
	})

	mux.HandleFunc("GET /api/v1/drafts/search", func(w http.ResponseWriter, r *http.Request) {
		if !searchLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many search requests")
			return
		}
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email", "limit", "cursor", "has_attachment", "q", "from", "to", "cc", "bcc", "subject") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		cursorRaw, ok := singleQueryValue(w, r, "cursor")
		if !ok {
			return
		}
		cursor, err := maildb.DecodeMessageListCursor(cursorRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		hasAttachment, ok := parseOptionalBoolQuery(w, r, "has_attachment")
		if !ok {
			return
		}
		queryText, ok := parseBoundedHTTPQuery(w, r, "q", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		from, ok := parseBoundedHTTPQuery(w, r, "from", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		to, ok := parseBoundedHTTPQuery(w, r, "to", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		cc, ok := parseBoundedHTTPQuery(w, r, "cc", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		bcc, ok := parseBoundedHTTPQuery(w, r, "bcc", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		subject, ok := parseBoundedHTTPQuery(w, r, "subject", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		drafts, err := service.SearchDrafts(r.Context(), maildb.DraftSearchQuery{
			UserID:        userID,
			Query:         queryText,
			From:          from,
			To:            to,
			Cc:            cc,
			Bcc:           bcc,
			Subject:       subject,
			HasAttachment: hasAttachment,
			Limit:         limit,
			Cursor:        cursor,
		})
		if err != nil {
			writeInternalServerError(w)
			return
		}
		page, err := maildb.NewDraftListPage(drafts, limit)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"drafts":      page.Drafts,
			"limit":       page.Limit,
			"has_more":    page.HasMore,
			"next_cursor": page.NextCursor,
		})
	})

	mux.HandleFunc("PATCH /api/v1/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		var req mailservice.SaveDraftRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		draftID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		req.DraftID = draftID
		if !allowAPIKeyRequest(w, r, opts.APIKeyLimiter) {
			return
		}
		if !bindRequestUserID(w, r, tokenManager, service, &req.UserID, req.UserEmail) {
			return
		}
		if !allowMailMutationRequest(w, r, opts, req.UserID, "update_draft") {
			return
		}
		draft, err := service.SaveDraft(r.Context(), req)
		if err != nil {
			if errors.Is(err, maildb.ErrDraftConflict) {
				writeError(w, http.StatusConflict, "Draft was modified by another session")
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
	})

	mux.HandleFunc("DELETE /api/v1/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		draftID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteDraft(r.Context(), userID, draftID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/drafts/{id}/send", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		draftID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		ctx := r.Context()
		if notice, ok, err := userMCPGeneratedNotice(ctx, service, r, userID); err != nil {
			writeError(w, http.StatusInternalServerError, "failed to apply mcp settings")
			return
		} else if ok {
			ctx = mailservice.ContextWithMCPGeneratedNotice(ctx, notice)
		}
		ctx, ok = userMCPSendPolicyContext(w, r, service, ctx, userID, nil)
		if !ok {
			return
		}
		result, err := service.SendDraft(ctx, userID, draftID)
		if err != nil {
			if strings.HasPrefix(err.Error(), "mcp ") {
				writeError(w, http.StatusForbidden, err.Error())
				return
			}
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		recordUserMCPSend(r, userID)
		result = mailservice.NormalizeSendTextResult(result)
		writeJSON(w, http.StatusAccepted, map[string]any{"message": result})
	})
}
