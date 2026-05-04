package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/mailservice"
)

const (
	maxJSONBodyBytes       = 1 << 20
	maxHTTPAuthHeaderBytes = 16 << 10
	maxHTTPControlBytes    = 32
	maxHTTPResourceIDBytes = 200
	maxHTTPQueryBytes      = 1024
)

type MessageService interface {
	ListFolders(ctx context.Context, userID string) ([]maildb.Folder, error)
	CreateFolder(ctx context.Context, req maildb.CreateFolderRequest) (maildb.Folder, error)
	RenameFolder(ctx context.Context, userID string, folderID string, name string) (maildb.Folder, error)
	DeleteFolder(ctx context.Context, userID string, folderID string) error
	ListMessages(ctx context.Context, userID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesInFolder(ctx context.Context, userID string, folderID string, limit int) ([]maildb.MessageSummary, error)
	ListMessagesPage(ctx context.Context, userID string, folderID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error)
	ListThreads(ctx context.Context, userID string, limit int) ([]maildb.ThreadSummary, error)
	ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error)
	SearchMessages(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error)
	SearchDrafts(ctx context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error)
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	SetMessageFlag(ctx context.Context, userID string, messageID string, flag string, value bool) error
	BulkSetMessageFlag(ctx context.Context, req maildb.BulkMessageFlagRequest) (int64, error)
	MoveMessage(ctx context.Context, userID string, messageID string, folderID string) error
	BulkMoveMessages(ctx context.Context, req maildb.BulkMessageMoveRequest) (int64, error)
	DeleteMessage(ctx context.Context, userID string, messageID string) error
	BulkDeleteMessages(ctx context.Context, req maildb.BulkMessageDeleteRequest) (int64, error)
	ListPushDevices(ctx context.Context, userID string, limit int) ([]maildb.PushDevice, error)
	UpsertPushDevice(ctx context.Context, req maildb.UpsertPushDeviceRequest) (maildb.PushDevice, error)
	DeletePushDevice(ctx context.Context, userID string, id string) error
	SaveDraft(ctx context.Context, req mailservice.SaveDraftRequest) (maildb.MessageDetail, error)
	DeleteDraft(ctx context.Context, userID string, draftID string) error
	SendDraft(ctx context.Context, userID string, draftID string) (mailservice.SendTextResult, error)
	CreateAttachmentUpload(ctx context.Context, req mailservice.CreateAttachmentUploadRequest) (maildb.Attachment, error)
	UploadAttachment(ctx context.Context, req mailservice.UploadAttachmentRequest) (maildb.Attachment, error)
	CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error)
	CreateAttachmentUploadSession(ctx context.Context, req mailservice.CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error)
	CancelAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error)
	GetAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error)
	StoreAttachmentUploadSessionBody(ctx context.Context, req mailservice.StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error)
	FinalizeAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.Attachment, error)
	ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error)
	OpenAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (mailservice.AttachmentDownload, error)
	SendText(ctx context.Context, req mailservice.SendTextRequest) (mailservice.SendTextResult, error)
	MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (maildb.MessageDeliveryStatusView, error)
}

func RegisterMailRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager) {
	mux.HandleFunc("GET /api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		folders, err := service.ListFolders(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"folders": folders})
	})

	mux.HandleFunc("POST /api/v1/folders", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		folder, err := service.CreateFolder(r.Context(), maildb.CreateFolderRequest{
			UserID: userID,
			Name:   req.Name,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusCreated, map[string]any{"folder": folder})
	})

	mux.HandleFunc("PATCH /api/v1/folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		folderID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}

		var req struct {
			Name string `json:"name"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		folder, err := service.RenameFolder(r.Context(), userID, folderID, req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"folder": folder})
	})

	mux.HandleFunc("DELETE /api/v1/folders/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		folderID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeleteFolder(r.Context(), userID, folderID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"status": "ok"})
	})

	mux.HandleFunc("GET /api/v1/messages", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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
		messages, err := service.ListMessagesPage(r.Context(), userID, folderID, limit, cursor)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		page, err := maildb.NewMessageListPage(messages, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
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
		userID, ok := userIDFromRequest(w, r, tokenManager)
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

	mux.HandleFunc("GET /api/v1/search", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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
		subject, ok := parseBoundedHTTPQuery(w, r, "subject", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		messages, err := service.SearchMessages(r.Context(), maildb.MessageSearchQuery{
			UserID:            userID,
			Query:             queryText,
			FolderID:          folderID,
			From:              from,
			Subject:           subject,
			HasAttachment:     hasAttachment,
			Limit:             limit,
			Sort:              sortMode,
			IncludeRank:       includeRank,
			IncludeHighlights: includeHighlights,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
	})

	mux.HandleFunc("GET /api/v1/threads", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		threads, err := service.ListThreads(r.Context(), userID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"threads": threads})
	})

	mux.HandleFunc("GET /api/v1/threads/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		threadID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		messages, err := service.ListThreadMessages(r.Context(), userID, threadID, limit)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
	})

	mux.HandleFunc("GET /api/v1/messages/{id}/delivery-status", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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

		userID, ok := userIDFromRequest(w, r, tokenManager)
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

		userID, ok := userIDFromRequest(w, r, tokenManager)
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

		userID, ok := userIDFromRequest(w, r, tokenManager)
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

		userID, ok := userIDFromRequest(w, r, tokenManager)
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

	mux.HandleFunc("DELETE /api/v1/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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

		userID, ok := userIDFromRequest(w, r, tokenManager)
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

	mux.HandleFunc("POST /api/v1/drafts", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req mailservice.SaveDraftRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if tokenManager != nil {
			claims, ok := claimsFromRequest(w, r, tokenManager)
			if !ok {
				return
			}
			req.UserID = claims.UserID
		}
		draft, err := service.SaveDraft(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
	})

	mux.HandleFunc("GET /api/v1/drafts/search", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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
		queryText, ok := parseBoundedHTTPQuery(w, r, "q", false, maxHTTPQueryBytes)
		if !ok {
			return
		}
		from, ok := parseBoundedHTTPQuery(w, r, "from", false, maxHTTPQueryBytes)
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
			Subject:       subject,
			HasAttachment: hasAttachment,
			Limit:         limit,
		})
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drafts": drafts})
	})

	mux.HandleFunc("PATCH /api/v1/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

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
		if tokenManager != nil {
			claims, ok := claimsFromRequest(w, r, tokenManager)
			if !ok {
				return
			}
			req.UserID = claims.UserID
		}
		draft, err := service.SaveDraft(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"draft": draft})
	})

	mux.HandleFunc("DELETE /api/v1/drafts/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
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
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		draftID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		result, err := service.SendDraft(r.Context(), userID, draftID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result = mailservice.NormalizeSendTextResult(result)
		writeJSON(w, http.StatusAccepted, map[string]any{"message": result})
	})

	mux.HandleFunc("GET /api/v1/messages/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		messageID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		attachments, err := service.ListAttachments(r.Context(), userID, messageID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"attachments": attachments})
	})

	mux.HandleFunc("POST /api/v1/attachments", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req mailservice.CreateAttachmentUploadRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if tokenManager != nil {
			claims, ok := claimsFromRequest(w, r, tokenManager)
			if !ok {
				return
			}
			req.UserID = claims.UserID
		}
		attachment, err := service.CreateAttachmentUpload(r.Context(), req)
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachment})
	})

	mux.HandleFunc("POST /api/v1/attachments/upload", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, mailservice.MaxAttachmentUploadBytes+(1<<20))
		if err := r.ParseMultipartForm(mailservice.MaxAttachmentUploadBytes); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "attachment upload request is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "invalid multipart attachment upload")
			return
		}
		file, header, ok := singleHTTPMultipartFile(w, r, "file")
		if !ok {
			return
		}
		if file == nil {
			writeError(w, http.StatusBadRequest, "file is required")
			return
		}
		defer file.Close()

		mimeType := strings.TrimSpace(header.Header.Get("Content-Type"))
		if mimeType == "" {
			mimeType = "application/octet-stream"
		}
		if header.Size > mailservice.MaxAttachmentUploadBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "attachment is too large")
			return
		}
		draftID, ok := parseBoundedHTTPFormValue(w, r, "draft_id", false, maxHTTPResourceIDBytes)
		if !ok {
			return
		}
		attachment, err := service.UploadAttachment(r.Context(), mailservice.UploadAttachmentRequest{
			UserID:   userID,
			DraftID:  draftID,
			Filename: header.Filename,
			Size:     header.Size,
			MIMEType: mimeType,
			Body:     file,
		})
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachment})
	})

	mux.HandleFunc("POST /api/v1/attachments/upload-sessions", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			DraftID      string    `json:"draft_id"`
			Filename     string    `json:"filename"`
			DeclaredSize int64     `json:"declared_size"`
			MIMEType     string    `json:"mime_type"`
			ExpiresAt    time.Time `json:"expires_at"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		session, err := service.CreateAttachmentUploadSession(r.Context(), mailservice.CreateAttachmentUploadSessionRequest{
			UserID:       userID,
			DraftID:      req.DraftID,
			Filename:     req.Filename,
			DeclaredSize: req.DeclaredSize,
			MIMEType:     req.MIMEType,
			ExpiresAt:    req.ExpiresAt,
		})
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment_upload_session": session})
	})

	mux.HandleFunc("GET /api/v1/attachments/capabilities", func(w http.ResponseWriter, r *http.Request) {
		if _, ok := userIDFromRequest(w, r, tokenManager); !ok {
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"attachment_upload_capabilities": map[string]any{
				"max_attachment_bytes":       mailservice.MaxAttachmentUploadBytes,
				"max_filename_bytes":         mailservice.MaxAttachmentFilenameBytes,
				"max_session_ttl_seconds":    int64(mailservice.MaxAttachmentUploadSessionTTL.Seconds()),
				"metadata_reservation":       true,
				"direct_multipart_upload":    true,
				"cancel_pending_uploads":     true,
				"upload_sessions":            true,
				"cancel_upload_sessions":     true,
				"upload_session_body":        true,
				"upload_session_checksum":    true,
				"finalize_upload_sessions":   true,
				"resumable_chunked_uploads":  false,
				"requires_declared_size":     true,
				"quota_reserved_on_metadata": true,
			},
		})
	})

	mux.HandleFunc("DELETE /api/v1/attachments/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		attachmentID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		attachment, err := service.CancelAttachmentUpload(r.Context(), userID, attachmentID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment": attachment})
	})

	mux.HandleFunc("DELETE /api/v1/attachments/upload-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		session, err := service.CancelAttachmentUploadSession(r.Context(), userID, sessionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment_upload_session": session})
	})

	mux.HandleFunc("GET /api/v1/attachments/upload-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		session, err := service.GetAttachmentUploadSession(r.Context(), userID, sessionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment_upload_session": session})
	})

	mux.HandleFunc("PUT /api/v1/attachments/upload-sessions/{id}/body", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if strings.TrimSpace(r.Header.Get("Content-Range")) != "" {
			writeError(w, http.StatusBadRequest, "content-range is not supported for upload session body storage")
			return
		}
		body := http.MaxBytesReader(w, r.Body, mailservice.MaxAttachmentUploadBytes+1)
		session, err := service.StoreAttachmentUploadSessionBody(r.Context(), mailservice.StoreAttachmentUploadSessionBodyRequest{
			UserID:                 userID,
			SessionID:              sessionID,
			ExpectedChecksumSHA256: r.Header.Get("X-Content-SHA256"),
			Body:                   body,
		})
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment_upload_session": session})
	})

	mux.HandleFunc("POST /api/v1/attachments/upload-sessions/{id}/finalize", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		attachment, err := service.FinalizeAttachmentUploadSession(r.Context(), userID, sessionID)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachment})
	})

	mux.HandleFunc("GET /api/v1/messages/{id}/attachments/{attachment_id}/download", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		messageID, attachmentID, ok := parseBoundedHTTPPathPair(w, r, "id", "attachment_id")
		if !ok {
			return
		}
		download, err := service.OpenAttachment(r.Context(), userID, messageID, attachmentID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		defer download.Body.Close()

		w.Header().Set("Content-Type", attachmentContentType(download.Attachment.MIMEType))
		w.Header().Set("Content-Disposition", contentDispositionAttachment(download.Attachment.Filename))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if download.Attachment.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(download.Attachment.Size, 10))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, download.Body)
	})

	mux.HandleFunc("GET /api/v1/push-devices", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		devices, err := service.ListPushDevices(r.Context(), userID, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"push_devices": devices})
	})

	mux.HandleFunc("POST /api/v1/push-devices", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req maildb.UpsertPushDeviceRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		req.UserID = userID
		device, err := service.UpsertPushDevice(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"push_device": device})
	})

	mux.HandleFunc("DELETE /api/v1/push-devices/{id}", func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		id, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		if err := service.DeletePushDevice(r.Context(), userID, id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": id})
	})

	mux.HandleFunc("POST /api/v1/messages/send", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		var req mailservice.SendTextRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}

		if tokenManager != nil {
			claims, ok := claimsFromRequest(w, r, tokenManager)
			if !ok {
				return
			}
			req.UserID = claims.UserID
		}
		result, err := service.SendText(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		result = mailservice.NormalizeSendTextResult(result)

		writeJSON(w, http.StatusAccepted, map[string]any{"message": result})
	})
}

func parseQueryLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	raw, ok := singleQueryValue(w, r, "limit")
	if !ok {
		return 0, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, true
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, "limit is too long")
		return 0, false
	}
	limit, err := strconv.Atoi(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, "limit must be an integer")
		return 0, false
	}
	if limit <= 0 {
		writeError(w, http.StatusBadRequest, "limit must be positive")
		return 0, false
	}
	if limit > maildb.MessageListMaxLimit {
		writeError(w, http.StatusBadRequest, "limit must be at most 200")
		return 0, false
	}
	return limit, true
}

func parseOptionalBoolQuery(w http.ResponseWriter, r *http.Request, key string) (*bool, bool) {
	raw, ok := singleQueryValue(w, r, key)
	if !ok {
		return nil, false
	}
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, true
	}
	if strings.ContainsAny(raw, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return nil, false
	}
	if len(raw) > maxHTTPControlBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return nil, false
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, key+" must be a boolean")
		return nil, false
	}
	return &value, true
}

func singleQueryValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	values, ok := r.URL.Query()[key]
	if !ok || len(values) == 0 {
		return "", true
	}
	if len(values) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return "", false
	}
	return values[0], true
}

func parseBoolQueryDefaultFalse(w http.ResponseWriter, r *http.Request, key string) (bool, bool) {
	value, ok := parseOptionalBoolQuery(w, r, key)
	if !ok {
		return false, false
	}
	if value == nil {
		return false, true
	}
	return *value, true
}

func decodeJSONBody(r *http.Request, dst any) error {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxJSONBodyBytes+1))
	if err != nil {
		return err
	}
	if len(raw) > maxJSONBodyBytes {
		return errors.New("json body too large")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("body must contain a single JSON value")
		}
		return err
	}
	return nil
}

func parseBoundedHTTPPathValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	value := strings.TrimSpace(r.PathValue(key))
	if value == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxHTTPResourceIDBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func parseBoundedHTTPPathPair(w http.ResponseWriter, r *http.Request, firstKey string, secondKey string) (string, string, bool) {
	first, ok := parseBoundedHTTPPathValue(w, r, firstKey)
	if !ok {
		return "", "", false
	}
	second, ok := parseBoundedHTTPPathValue(w, r, secondKey)
	if !ok {
		return "", "", false
	}
	return first, second, true
}

func parseBoundedHTTPQuery(w http.ResponseWriter, r *http.Request, key string, required bool, maxBytes int) (string, bool) {
	value, ok := singleQueryValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return parseBoundedHTTPValue(w, key, value, required, maxBytes)
}

func parseBoundedHTTPFormValue(w http.ResponseWriter, r *http.Request, key string, required bool, maxBytes int) (string, bool) {
	value, ok := singleHTTPMultipartFormValue(w, r, key)
	if !ok {
		return "", false
	}
	value = strings.TrimSpace(value)
	return parseBoundedHTTPValue(w, key, value, required, maxBytes)
}

func singleHTTPMultipartFormValue(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	if r.MultipartForm == nil || r.MultipartForm.Value == nil {
		return "", true
	}
	values := r.MultipartForm.Value[key]
	if len(values) == 0 {
		return "", true
	}
	if len(values) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return "", false
	}
	return values[0], true
}

func singleHTTPMultipartFile(w http.ResponseWriter, r *http.Request, key string) (multipart.File, *multipart.FileHeader, bool) {
	if r.MultipartForm == nil || r.MultipartForm.File == nil {
		return nil, nil, true
	}
	files := r.MultipartForm.File[key]
	if len(files) == 0 {
		return nil, nil, true
	}
	if len(files) > 1 {
		writeError(w, http.StatusBadRequest, key+" must not be repeated")
		return nil, nil, false
	}
	file, err := files[0].Open()
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart attachment upload")
		return nil, nil, false
	}
	return file, files[0], true
}

func parseBoundedHTTPValue(w http.ResponseWriter, key string, value string, required bool, maxBytes int) (string, bool) {
	if value == "" {
		if required {
			writeError(w, http.StatusBadRequest, key+" is required")
			return "", false
		}
		return "", true
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, key+" must not contain CR or LF")
		return "", false
	}
	if len(value) > maxBytes {
		writeError(w, http.StatusBadRequest, key+" is too long")
		return "", false
	}
	return value, true
}

func userIDFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (string, bool) {
	if tokenManager == nil {
		return parseBoundedHTTPQuery(w, r, "user_id", true, maxHTTPResourceIDBytes)
	}
	claims, ok := claimsFromRequest(w, r, tokenManager)
	if !ok {
		return "", false
	}
	return claims.UserID, true
}

func claimsFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (auth.Claims, bool) {
	token, ok := bearerToken(w, r)
	if !ok {
		return auth.Claims{}, false
	}
	if token == "" {
		writeError(w, http.StatusUnauthorized, "bearer token is required")
		return auth.Claims{}, false
	}
	claims, err := tokenManager.Verify(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, err.Error())
		return auth.Claims{}, false
	}
	return claims, true
}

func bearerToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if len(authHeader) > maxHTTPAuthHeaderBytes {
		writeError(w, http.StatusBadRequest, "Authorization is too long")
		return "", false
	}
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[len("bearer "):]), true
	}
	return "", true
}

func contentDispositionAttachment(filename string) string {
	utf8Name := strings.NewReplacer("\\", "_", `"`, "_", "\r", "_", "\n", "_").Replace(strings.TrimSpace(filename))
	if utf8Name == "" {
		utf8Name = "attachment"
	}
	utf8Name = truncateRunes(utf8Name, 180)
	asciiName := asciiAttachmentFilename(utf8Name)
	return `attachment; filename="` + asciiName + `"; filename*=UTF-8''` + url.PathEscape(utf8Name)
}

func truncateRunes(value string, max int) string {
	if max <= 0 {
		return ""
	}
	for i := range value {
		if max == 0 {
			return value[:i]
		}
		max--
	}
	return value
}

func asciiAttachmentFilename(filename string) string {
	var builder strings.Builder
	for _, r := range filename {
		if r >= 0x20 && r <= 0x7e {
			builder.WriteRune(r)
			continue
		}
		builder.WriteByte('_')
	}
	if builder.Len() == 0 {
		return "attachment"
	}
	return builder.String()
}

func attachmentContentType(mimeType string) string {
	return safeContentType(mimeType, "application/octet-stream")
}

func safeContentType(mimeType string, fallback string) string {
	mimeType = strings.TrimSpace(mimeType)
	if mimeType == "" || strings.ContainsAny(mimeType, "\r\n") {
		return fallback
	}
	mediaType, _, err := mime.ParseMediaType(mimeType)
	if err != nil {
		return fallback
	}
	typeName, subType, ok := strings.Cut(mediaType, "/")
	if !ok || strings.TrimSpace(typeName) == "" || strings.TrimSpace(subType) == "" {
		return fallback
	}
	return mimeType
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func writeNDJSON[T any](w http.ResponseWriter, status int, rows []T) {
	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	encoder := json.NewEncoder(w)
	for _, row := range rows {
		_ = encoder.Encode(row)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	code := "internal_error"
	switch status {
	case http.StatusBadRequest:
		code = "bad_request"
	case http.StatusUnauthorized:
		code = "unauthorized"
	case http.StatusForbidden:
		code = "forbidden"
	case http.StatusNotFound:
		code = "not_found"
	case http.StatusConflict:
		code = "conflict"
	case http.StatusRequestEntityTooLarge:
		code = "payload_too_large"
	case http.StatusInsufficientStorage:
		code = "insufficient_storage"
	}
	writeJSON(w, status, map[string]any{
		"error": map[string]any{
			"code":        code,
			"message":     message,
			"status":      status,
			"status_text": http.StatusText(status),
		},
		"error_message": message,
	})
}

func writeMailServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, mail.ErrMailboxFull) {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}
	var maxBytesErr *http.MaxBytesError
	if errors.As(err, &maxBytesErr) {
		writeError(w, http.StatusRequestEntityTooLarge, "attachment upload request is too large")
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}
