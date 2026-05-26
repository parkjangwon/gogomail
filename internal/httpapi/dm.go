package httpapi

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/dm"
)

type DMService interface {
	CreateRoom(ctx context.Context, principal dm.Principal, req dm.CreateRoomRequest) (dm.Room, error)
	ListRooms(ctx context.Context, principal dm.Principal) ([]dm.Room, error)
	ListPublicRooms(ctx context.Context, principal dm.Principal) ([]dm.Room, error)
	AddMembers(ctx context.Context, principal dm.Principal, roomID string, userIDs []string) ([]dm.Message, error)
	RemoveMember(ctx context.Context, principal dm.Principal, roomID string, userID string) (dm.RoomRemoval, error)
	TransferOwner(ctx context.Context, principal dm.Principal, roomID string, userID string) (dm.Message, error)
	CreateInvite(ctx context.Context, principal dm.Principal, roomID string) (dm.Invite, error)
	JoinInvite(ctx context.Context, principal dm.Principal, token string) (dm.Message, error)
	ListMessages(ctx context.Context, principal dm.Principal, roomID string, cursor dm.MessageCursor) ([]dm.Message, error)
	SendMessage(ctx context.Context, principal dm.Principal, roomID string, req dm.SendMessageRequest) (dm.Message, error)
	SendAttachment(ctx context.Context, principal dm.Principal, roomID string, upload dm.AttachmentUpload) (dm.Message, error)
	EditMessage(ctx context.Context, principal dm.Principal, messageID string, body string) (dm.Message, error)
	DeleteMessage(ctx context.Context, principal dm.Principal, messageID string) (dm.Message, error)
	ToggleReaction(ctx context.Context, principal dm.Principal, messageID string, emoji string) error
	MarkRead(ctx context.Context, principal dm.Principal, roomID string, lastMessageID string) error
	Search(ctx context.Context, principal dm.Principal, roomID string, q string, before string, limit int) ([]dm.SearchResult, error)
	ListMedia(ctx context.Context, principal dm.Principal, roomID string, query dm.MediaQuery) ([]dm.MediaItem, error)
	SignAttachmentDownload(messageID string, expiresAt time.Time) (string, error)
	VerifyAttachmentDownload(token string) (string, error)
	OpenAttachment(ctx context.Context, token string) (dm.AttachmentDownload, error)
	ExportRoom(ctx context.Context, principal dm.Principal, roomID string) (dm.RoomExport, error)
	RotateRoomKey(ctx context.Context, principal dm.Principal, roomID string) error
}

func RegisterDMRoutes(mux *http.ServeMux, service DMService, tokenManager *auth.TokenManager, publicBaseURL string) {
	mux.HandleFunc("GET /api/v1/dm/rooms", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		rooms, err := service.ListRooms(r.Context(), principal)
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
	})

	mux.HandleFunc("GET /api/v1/dm/rooms/public", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		rooms, err := service.ListPublicRooms(r.Context(), principal)
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"rooms": rooms})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req dm.CreateRoomRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		room, err := service.CreateRoom(r.Context(), principal, req)
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"room": room})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{id}/members", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			UserIDs []string `json:"user_ids"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		messages, err := service.AddMembers(r.Context(), principal, r.PathValue("id"), req.UserIDs)
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURLs(messages, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
	})

	mux.HandleFunc("DELETE /api/v1/dm/rooms/{id}/members/{user_id}", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		result, err := service.RemoveMember(r.Context(), principal, r.PathValue("id"), r.PathValue("user_id"))
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"removal": result})
	})

	mux.HandleFunc("PATCH /api/v1/dm/rooms/{id}/owner", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			UserID string `json:"user_id"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		message, err := service.TransferOwner(r.Context(), principal, r.PathValue("id"), req.UserID)
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"message": message})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{id}/invites", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		invite, err := service.CreateInvite(r.Context(), principal, r.PathValue("id"))
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"invite":     invite,
			"invite_url": strings.TrimRight(publicBaseURL, "/") + "/dm/join/" + invite.Token,
		})
	})

	mux.HandleFunc("POST /api/v1/dm/join/{token}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		message, err := service.JoinInvite(r.Context(), principal, r.PathValue("token"))
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"message": message})
	})

	mux.HandleFunc("GET /api/v1/dm/rooms/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id", "before", "after", "limit") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := dmLimitFromRequest(w, r, 50, 100)
		if !ok {
			return
		}
		messages, err := service.ListMessages(r.Context(), principal, r.PathValue("id"), dm.MessageCursor{
			BeforeID: r.URL.Query().Get("before"),
			AfterID:  r.URL.Query().Get("after"),
			Limit:    limit,
		})
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURLs(messages, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"messages": messages})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{id}/messages", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req dm.SendMessageRequest
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		message, err := service.SendMessage(r.Context(), principal, r.PathValue("id"), req)
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusCreated, map[string]any{"message": message})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{id}/attachments", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		upload, ok := readDMAttachmentUpload(w, r)
		if !ok {
			return
		}
		message, err := service.SendAttachment(r.Context(), principal, r.PathValue("id"), upload)
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusCreated, map[string]any{"message": message})
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{id}/read", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			LastMessageID string `json:"last_message_id"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := service.MarkRead(r.Context(), principal, r.PathValue("id"), req.LastMessageID); err != nil {
			writeDMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/dm/rooms/{id}/search", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id", "q", "before", "limit") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := dmLimitFromRequest(w, r, 20, 50)
		if !ok {
			return
		}
		results, err := service.Search(r.Context(), principal, r.PathValue("id"), r.URL.Query().Get("q"), r.URL.Query().Get("before"), limit)
		if err != nil {
			writeDMError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"results": results})
	})

	mux.HandleFunc("GET /api/v1/dm/rooms/{id}/media", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id", "type", "before", "limit") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		limit, ok := dmLimitFromRequest(w, r, 30, 100)
		if !ok {
			return
		}
		items, err := service.ListMedia(r.Context(), principal, r.PathValue("id"), dm.MediaQuery{
			Type:     r.URL.Query().Get("type"),
			BeforeID: r.URL.Query().Get("before"),
			Limit:    limit,
		})
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMAttachmentDownloadURLs(items, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"media": items})
	})

	mux.HandleFunc("GET /api/v1/dm/messages/{id}/attachment", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "token") {
			return
		}
		token := strings.TrimSpace(r.URL.Query().Get("token"))
		if token == "" {
			writeError(w, http.StatusBadRequest, "token is required")
			return
		}
		messageID, err := service.VerifyAttachmentDownload(token)
		if err != nil {
			writeDMError(w, err)
			return
		}
		if messageID != strings.TrimSpace(r.PathValue("id")) {
			writeDMError(w, dm.ErrForbidden)
			return
		}
		download, err := service.OpenAttachment(r.Context(), token)
		if err != nil {
			writeDMError(w, err)
			return
		}
		defer download.Body.Close()
		w.Header().Set("Content-Type", attachmentContentType(download.ContentType))
		w.Header().Set("Cache-Control", "private, max-age=3600")
		w.Header().Set("Content-Disposition", contentDispositionAttachment(download.Filename))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if download.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(download.Size, 10))
		}
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, download.Body)
	})

	mux.HandleFunc("PATCH /api/v1/dm/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			Body string `json:"body"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		message, err := service.EditMessage(r.Context(), principal, r.PathValue("id"), req.Body)
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"message": message})
	})

	mux.HandleFunc("DELETE /api/v1/dm/messages/{id}", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		message, err := service.DeleteMessage(r.Context(), principal, r.PathValue("id"))
		if err != nil {
			writeDMError(w, err)
			return
		}
		addDMMessageAttachmentDownloadURL(&message, service, publicBaseURL)
		writeJSON(w, http.StatusOK, map[string]any{"message": message})
	})

	mux.HandleFunc("PUT /api/v1/dm/messages/{id}/reactions/{emoji}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		if err := service.ToggleReaction(r.Context(), principal, r.PathValue("id"), r.PathValue("emoji")); err != nil {
			writeDMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("PUT /api/v1/dm/messages/{id}/reactions", func(w http.ResponseWriter, r *http.Request) {
		principal, ok := dmMutationPrincipal(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			Emoji string `json:"emoji"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		if err := service.ToggleReaction(r.Context(), principal, r.PathValue("id"), req.Emoji); err != nil {
			writeDMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/dm/rooms/{roomID}/export", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id", "timezone") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		roomID := strings.TrimSpace(r.PathValue("roomID"))
		if roomID == "" {
			writeError(w, http.StatusBadRequest, "room_id is required")
			return
		}

		// Parse optional timezone query param (IANA name, e.g. "Asia/Seoul").
		// Falls back to UTC if missing or invalid.
		loc := time.UTC
		if tzParam := strings.TrimSpace(r.URL.Query().Get("timezone")); tzParam != "" {
			if parsed, err := time.LoadLocation(tzParam); err == nil {
				loc = parsed
			}
		}

		export, err := service.ExportRoom(r.Context(), principal, roomID)
		if err != nil {
			writeDMError(w, err)
			return
		}

		// Derive owner email from room members matching the requesting user.
		ownerEmail := principal.UserID // fallback
		for _, m := range export.Room.Members {
			if m.ID == principal.UserID {
				if m.Email != "" {
					ownerEmail = m.Email
				}
				break
			}
		}

		// Build room name from explicit name or participant display names.
		roomName := export.Room.Name
		if roomName == "" {
			names := make([]string, 0, len(export.Room.Members))
			for _, m := range export.Room.Members {
				names = append(names, m.DisplayName)
			}
			roomName = strings.Join(names, "-")
		}
		if roomName == "" {
			roomName = export.Room.ID
		}

		sanitize := func(s string) string {
			return strings.Map(func(r rune) rune {
				switch r {
				case '/', '\\', ':', '*', '?', '"', '<', '>', '|', ' ':
					return '_'
				}
				return r
			}, s)
		}

		// Filename: {ownerEmail}_{roomName}_{YYYYMMDDHHmmss}.txt
		datetime := export.ExportAt.In(loc).Format("20060102150405")
		filename := fmt.Sprintf("%s_%s_%s.txt", sanitize(ownerEmail), sanitize(roomName), datetime)

		txt := dm.FormatExportTXT(export, loc)
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.Header().Set("Content-Disposition", contentDispositionAttachment(filename))
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, txt)
	})

	mux.HandleFunc("POST /api/v1/dm/rooms/{roomID}/rotate-key", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) || !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id") {
			return
		}
		principal, ok := dmPrincipalFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		roomID := strings.TrimSpace(r.PathValue("roomID"))
		if roomID == "" {
			writeError(w, http.StatusBadRequest, "room_id is required")
			return
		}
		if err := service.RotateRoomKey(r.Context(), principal, roomID); err != nil {
			writeDMError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func addDMAttachmentDownloadURLs(items []dm.MediaItem, service DMService, publicBaseURL string) {
	publicRoot := strings.TrimRight(publicBaseURL, "/")
	base := publicRoot + "/api/v1"
	if publicRoot == "" {
		base = "/api/v1"
	}
	for i := range items {
		if items[i].MessageType != dm.MessageTypeFile || items[i].MessageID == "" {
			continue
		}
		token, err := service.SignAttachmentDownload(items[i].MessageID, time.Now().UTC().Add(time.Hour))
		if err != nil {
			continue
		}
		items[i].DownloadURL = base + "/dm/messages/" + url.PathEscape(items[i].MessageID) + "/attachment?token=" + url.QueryEscape(token)
	}
}

func addDMMessageAttachmentDownloadURLs(messages []dm.Message, service DMService, publicBaseURL string) {
	for i := range messages {
		addDMMessageAttachmentDownloadURL(&messages[i], service, publicBaseURL)
	}
}

func addDMMessageAttachmentDownloadURL(message *dm.Message, service DMService, publicBaseURL string) {
	if message == nil || message.MessageType != dm.MessageTypeFile || message.ID == "" {
		return
	}
	token, err := service.SignAttachmentDownload(message.ID, time.Now().UTC().Add(time.Hour))
	if err != nil {
		return
	}
	publicRoot := strings.TrimRight(publicBaseURL, "/")
	base := publicRoot + "/api/v1"
	if publicRoot == "" {
		base = "/api/v1"
	}
	message.AttachmentDownloadURL = base + "/dm/messages/" + url.PathEscape(message.ID) + "/attachment?token=" + url.QueryEscape(token)
}

func readDMAttachmentUpload(w http.ResponseWriter, r *http.Request) (dm.AttachmentUpload, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, dm.MaxAttachmentBytes+4096)
	if err := r.ParseMultipartForm(dm.MaxAttachmentBytes + 4096); err != nil {
		writeError(w, http.StatusBadRequest, "invalid attachment upload")
		return dm.AttachmentUpload{}, false
	}
	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return dm.AttachmentUpload{}, false
	}
	defer file.Close()
	body, err := io.ReadAll(io.LimitReader(file, dm.MaxAttachmentBytes+1))
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to read attachment")
		return dm.AttachmentUpload{}, false
	}
	if len(body) == 0 {
		writeError(w, http.StatusBadRequest, "attachment is empty")
		return dm.AttachmentUpload{}, false
	}
	if len(body) > dm.MaxAttachmentBytes {
		writeError(w, http.StatusRequestEntityTooLarge, "attachment is too large")
		return dm.AttachmentUpload{}, false
	}
	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(body)
	}
	return dm.AttachmentUpload{
		Filename:    header.Filename,
		Size:        int64(len(body)),
		ContentType: contentType,
		Body:        body,
	}, true
}

func dmMutationPrincipal(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (dm.Principal, bool) {
	if !rejectUnknownQueryKeys(w, r, "user_id", "company_id", "domain_id") {
		return dm.Principal{}, false
	}
	return dmPrincipalFromRequest(w, r, tokenManager)
}

func dmPrincipalFromRequest(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (dm.Principal, bool) {
	if tokenManager != nil {
		claims, ok := claimsFromRequest(w, r, tokenManager)
		if !ok {
			return dm.Principal{}, false
		}
		if strings.TrimSpace(claims.CompanyID) == "" || strings.TrimSpace(claims.DomainID) == "" {
			writeError(w, http.StatusUnauthorized, "company and domain claims are required")
			return dm.Principal{}, false
		}
		return dm.Principal{UserID: claims.UserID, CompanyID: claims.CompanyID, DomainID: claims.DomainID}, true
	}
	userID, ok := parseBoundedHTTPQuery(w, r, "user_id", true, maxHTTPResourceIDBytes)
	if !ok {
		return dm.Principal{}, false
	}
	companyID, ok := parseBoundedHTTPQuery(w, r, "company_id", true, maxHTTPResourceIDBytes)
	if !ok {
		return dm.Principal{}, false
	}
	domainID, ok := parseBoundedHTTPQuery(w, r, "domain_id", true, maxHTTPResourceIDBytes)
	if !ok {
		return dm.Principal{}, false
	}
	return dm.Principal{UserID: userID, CompanyID: companyID, DomainID: domainID}, true
}

func dmLimitFromRequest(w http.ResponseWriter, r *http.Request, fallback int, max int) (int, bool) {
	raw := strings.TrimSpace(r.URL.Query().Get("limit"))
	if raw == "" {
		return fallback, true
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 || limit > max {
		writeError(w, http.StatusBadRequest, "invalid limit")
		return 0, false
	}
	return limit, true
}

func writeDMError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, dm.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, dm.ErrConflict):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, dm.ErrForbidden):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, dm.ErrNotFound):
		writeError(w, http.StatusNotFound, "dm resource not found")
	default:
		writeInternalServerError(w)
	}
}
