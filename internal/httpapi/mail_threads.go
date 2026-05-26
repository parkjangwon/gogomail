package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerThreadRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	searchLimiter := NewAdminIPRateLimiter(30, time.Minute)
	mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		if !searchLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many search requests")
			return
		}
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email", "limit", "cursor", "has_attachment", "include_rank", "include_highlights", "sort", "q", "folder_id", "from", "to", "cc", "bcc", "subject", "since", "until") {
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
		hasAttachment, ok := parseOptionalBoolQuery(w, r, "has_attachment")
		if !ok {
			return
		}
		includeRank, ok := parseBoolQueryDefaultFalse(w, r, "include_rank")
		if !ok {
			return
		}
		includeHighlights, ok := parseBoolQueryDefaultFalse(w, r, "include_highlights")
		if !ok {
			return
		}
		sortMode, ok := parseBoundedHTTPQuery(w, r, "sort", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		sortMode = strings.ToLower(sortMode)
		if sortMode == "" {
			sortMode = maildb.MessageSearchSortDate
		}
		if sortMode != maildb.MessageSearchSortDate && sortMode != maildb.MessageSearchSortRelevance {
			writeError(w, http.StatusBadRequest, "sort must be date or relevance")
			return
		}
		cursorRaw, ok := singleQueryValue(w, r, "cursor")
		if !ok {
			return
		}
		if cursorRaw != "" && sortMode == maildb.MessageSearchSortRelevance {
			writeError(w, http.StatusBadRequest, "cursor is not supported for relevance sort")
			return
		}
		cursor, err := maildb.DecodeMessageListCursor(cursorRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		queryText, ok := parseBoundedHTTPQuery(w, r, "q", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		folderID, ok := parseBoundedHTTPQuery(w, r, "folder_id", false, maxHTTPResourceIDBytes)
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
		since, ok := parseOptionalRFC3339Query(w, r, "since")
		if !ok {
			return
		}
		until, ok := parseOptionalRFC3339Query(w, r, "until")
		if !ok {
			return
		}
		sinceStr := ""
		if !since.IsZero() {
			sinceStr = since.Format(time.RFC3339Nano)
		}
		untilStr := ""
		if !until.IsZero() {
			untilStr = until.Format(time.RFC3339Nano)
		}
		messages, err := service.SearchMessages(r.Context(), maildb.MessageSearchQuery{
			UserID:            userID,
			Query:             queryText,
			FolderID:          folderID,
			From:              from,
			To:                to,
			Cc:                cc,
			Bcc:               bcc,
			Subject:           subject,
			HasAttachment:     hasAttachment,
			Since:             sinceStr,
			Until:             untilStr,
			Limit:             limit,
			Sort:              sortMode,
			Cursor:            cursor,
			IncludeRank:       includeRank,
			IncludeHighlights: includeHighlights,
		})
		if err != nil {
			writeInternalServerError(w)
			return
		}
		page, err := maildb.NewMessageListPage(messages, limit)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		if sortMode == maildb.MessageSearchSortRelevance {
			page.NextCursor = ""
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"messages":    page.Messages,
			"limit":       page.Limit,
			"has_more":    page.HasMore,
			"next_cursor": page.NextCursor,
		})
	})

	mux.HandleFunc("GET /api/v1/threads", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email", "limit", "cursor", "folder_id", "read", "starred", "has_attachment", "sort") {
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
		cursor, err := maildb.DecodeThreadListCursor(cursorRaw)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		folderID, ok := parseBoundedHTTPQuery(w, r, "folder_id", false, maxHTTPResourceIDBytes)
		if !ok {
			return
		}
		read, ok := parseOptionalBoolQuery(w, r, "read")
		if !ok {
			return
		}
		starred, ok := parseOptionalBoolQuery(w, r, "starred")
		if !ok {
			return
		}
		hasAttachment, ok := parseOptionalBoolQuery(w, r, "has_attachment")
		if !ok {
			return
		}
		sortMode, ok := parseBoundedHTTPQuery(w, r, "sort", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		sortMode, valid := maildb.NormalizeListSort(sortMode)
		if !valid {
			writeError(w, http.StatusBadRequest, "sort must be newest or oldest")
			return
		}
		threads, err := service.ListThreadsPage(r.Context(), userID, limit, cursor, maildb.ThreadListFilter{
			FolderID:      folderID,
			Read:          read,
			Starred:       starred,
			HasAttachment: hasAttachment,
			Sort:          sortMode,
		})
		if err != nil {
			writeInternalServerError(w)
			return
		}
		page, err := maildb.NewThreadListPage(threads, limit)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"threads":     page.Threads,
			"limit":       page.Limit,
			"has_more":    page.HasMore,
			"next_cursor": page.NextCursor,
		})
	})

	mux.HandleFunc("GET /api/v1/threads/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email", "limit", "cursor") {
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
		threadID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		messages, err := service.ListThreadMessagesPage(r.Context(), userID, threadID, limit, cursor)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		page, err := maildb.NewMessageListPage(messages, limit)
		if err != nil {
			writeInternalServerError(w)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"messages":    page.Messages,
			"limit":       page.Limit,
			"has_more":    page.HasMore,
			"next_cursor": page.NextCursor,
		})
	})

	mux.HandleFunc("PATCH /api/v1/threads/bulk/flags", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkThreadFlagRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkSetThreadFlag(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})
}
