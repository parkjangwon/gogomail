package httpapi

import (
	"net/http"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

func registerMessageRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	mux.HandleFunc("GET /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
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
		cursor, err := maildb.DecodeMessageListCursor(cursorRaw)
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
		messages, err := service.ListMessagesPage(r.Context(), userID, folderID, limit, cursor, maildb.MessageListFilter{
			Read:          read,
			Starred:       starred,
			HasAttachment: hasAttachment,
			Sort:          sortMode,
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

		writeJSON(w, http.StatusOK, map[string]any{
			"messages":    page.Messages,
			"limit":       page.Limit,
			"has_more":    page.HasMore,
			"next_cursor": page.NextCursor,
		})
	})

	mux.HandleFunc("GET /api/v1/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
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

		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		message, err := service.GetMessage(r.Context(), userID, messageID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"message": message})
	})

	mux.HandleFunc("GET /api/v1/messages/{id}/delivery-status", func(w http.ResponseWriter, r *http.Request) {
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
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		status, err := service.MessageDeliveryStatus(r.Context(), userID, messageID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"delivery_status": status})
	})

	mux.HandleFunc("PATCH /api/v1/messages/{id}/flags", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req struct {
			Flag  string `json:"flag"`
			Value bool   `json:"value"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.SetMessageFlag(r.Context(), userID, messageID, req.Flag, req.Value); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("PATCH /api/v1/messages/bulk/flags", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkMessageFlagRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkSetMessageFlag(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("PATCH /api/v1/messages/{id}/folder", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req struct {
			FolderID string `json:"folder_id"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.MoveMessage(r.Context(), userID, messageID, req.FolderID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("PATCH /api/v1/messages/bulk/folder", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkMessageMoveRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkMoveMessages(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("PATCH /api/v1/threads/bulk/folder", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkThreadMoveRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkMoveThreads(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("DELETE /api/v1/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
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
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteMessage(r.Context(), userID, messageID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/messages/bulk/delete", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkMessageDeleteRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkDeleteMessages(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("POST /api/v1/threads/bulk/delete", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkThreadDeleteRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkDeleteThreads(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("POST /api/v1/messages/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
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
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.RestoreMessage(r.Context(), userID, messageID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("POST /api/v1/messages/bulk/restore", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkMessageRestoreRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkRestoreMessages(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})

	mux.HandleFunc("POST /api/v1/threads/bulk/restore", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}

		var req maildb.BulkThreadRestoreRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		updated, err := service.BulkRestoreThreads(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "updated": updated})
	})
}
