package httpapi

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/mailservice"
)

func registerAttachmentRoutes(mux *http.ServeMux, service MessageService, tokenManager *auth.TokenManager, opts MailRouteOptions) {
	mux.HandleFunc("GET /api/v1/messages/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
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
		attachments, err := service.ListAttachments(r.Context(), userID, messageID)
		if err != nil {
			writeInternalServerError(w)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"attachments": attachments})
	})

	mux.HandleFunc("POST /api/v1/attachments", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		var req mailservice.CreateAttachmentUploadRequest
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
		attachment, err := service.CreateAttachmentUpload(r.Context(), req)
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"attachment": attachment})
	})

	mux.HandleFunc("POST /api/v1/attachments/upload", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
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

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
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
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		if _, ok := userIDFromRequest(w, r, tokenManager, service); !ok {
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
				"resumable_chunked_uploads":  true,
				"requires_declared_size":     true,
				"quota_reserved_on_metadata": true,
			},
		})
	})

	mux.HandleFunc("DELETE /api/v1/attachments/{id}", func(w http.ResponseWriter, r *http.Request) {
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

		if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager, service)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		contentRangeHdr, ok := singleHTTPHeaderValue(w, r, "Content-Range", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		var contentRange *mailservice.ContentRange
		if contentRangeHdr != "" {
			cr, err := parseContentRange(contentRangeHdr)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			contentRange = cr
		}
		checksum, ok := singleHTTPHeaderValue(w, r, "X-Content-SHA256", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		body := http.MaxBytesReader(w, r.Body, mailservice.MaxAttachmentUploadBytes+1)
		session, err := service.StoreAttachmentUploadSessionBody(r.Context(), mailservice.StoreAttachmentUploadSessionBodyRequest{
			UserID:                 userID,
			SessionID:              sessionID,
			ExpectedChecksumSHA256: checksum,
			ContentRange:           contentRange,
			Body:                   body,
		})
		if err != nil {
			writeMailServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"attachment_upload_session": session})
	})

	mux.HandleFunc("POST /api/v1/attachments/upload-sessions/{id}/finalize", func(w http.ResponseWriter, r *http.Request) {
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

	attachmentDownloadLimiter := NewAdminIPRateLimiter(60, time.Minute)
	mux.HandleFunc("GET /api/v1/messages/{id}/attachments/{attachment_id}/download", func(w http.ResponseWriter, r *http.Request) {
		if !attachmentDownloadLimiter.allow(adminClientIP(r)) {
			writeError(w, http.StatusTooManyRequests, "too many download requests")
			return
		}
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

		writeAttachmentDownloadHeaders(w, download.Attachment)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, download.Body)
	})

	mux.HandleFunc("HEAD /api/v1/messages/{id}/attachments/{attachment_id}/download", func(w http.ResponseWriter, r *http.Request) {
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
		messageID, attachmentID, ok := parseBoundedHTTPPathPair(w, r, "id", "attachment_id")
		if !ok {
			return
		}
		metadata, err := service.StatAttachment(r.Context(), userID, messageID, attachmentID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeAttachmentDownloadHeaders(w, attachmentWithStatSize(metadata.Attachment, metadata.Object.Size))
		w.WriteHeader(http.StatusOK)
	})
}
