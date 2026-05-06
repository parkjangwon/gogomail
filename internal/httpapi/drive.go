package httpapi

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/drive"
	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/storage"
)

type DriveService interface {
	CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	CreateFileFromObject(ctx context.Context, req drive.CreateFileFromObjectRequest) (drive.Node, error)
	StoreStagedObject(ctx context.Context, req drive.StoreStagedObjectRequest) (drive.StagedObject, error)
	CreateUploadSession(ctx context.Context, req drive.CreateUploadSessionRequest) (drive.UploadSession, error)
	GetUploadSession(ctx context.Context, req drive.GetUploadSessionRequest) (drive.UploadSession, error)
	ListUploadSessions(ctx context.Context, req drive.ListUploadSessionsRequest) ([]drive.UploadSession, error)
	CancelUploadSession(ctx context.Context, req drive.CancelUploadSessionRequest) (drive.UploadSession, error)
	StoreUploadSessionBody(ctx context.Context, req drive.StoreUploadSessionBodyRequest) (drive.UploadSession, error)
	FinalizeUploadSession(ctx context.Context, req drive.FinalizeUploadSessionRequest) (drive.Node, error)
	ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	OpenFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileDownload, error)
	OpenFileRange(ctx context.Context, req drive.OpenFileRangeRequest) (drive.FileDownload, error)
	StatFile(ctx context.Context, req drive.OpenFileRequest) (drive.FileMetadata, error)
	GetUsageSummary(ctx context.Context, req drive.GetUsageSummaryRequest) (drive.UsageSummary, error)
	CreateShareLink(ctx context.Context, req drive.CreateShareLinkRequest) (drive.ShareLink, error)
	ListShareLinks(ctx context.Context, req drive.ListShareLinksRequest) ([]drive.ShareLink, error)
	RevokeShareLink(ctx context.Context, req drive.RevokeShareLinkRequest) (drive.ShareLink, error)
	ResolveShareLink(ctx context.Context, req drive.ResolveShareLinkRequest) (drive.ResolvedShareLink, error)
	OpenSharedFile(ctx context.Context, req drive.ResolveShareLinkRequest) (drive.FileDownload, error)
	OpenSharedFileRange(ctx context.Context, req drive.ResolveShareLinkRequest, rangeReq storage.RangeRequest) (drive.FileDownload, error)
	StatSharedFile(ctx context.Context, req drive.ResolveShareLinkRequest) (drive.FileMetadata, error)
	TrashNode(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error)
	RestoreNode(ctx context.Context, req drive.RestoreNodeRequest) (drive.Node, int64, error)
	RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	CopyNode(ctx context.Context, req drive.CopyNodeRequest) (drive.Node, error)
	PermanentDeleteNode(ctx context.Context, req drive.PermanentDeleteNodeRequest) (drive.PermanentDeleteServiceResult, error)
}

func RegisterDriveRoutes(mux *http.ServeMux, service DriveService, tokenManager *auth.TokenManager) {
	mux.HandleFunc("GET /api/v1/drive/nodes", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "parent_id", "status", "node_type", "q", "sort", "all_parents", "limit") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		parentID, ok := parseBoundedHTTPQuery(w, r, "parent_id", false, maxHTTPResourceIDBytes)
		if !ok {
			return
		}
		status, ok := parseBoundedHTTPQuery(w, r, "status", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		nodeType, ok := parseBoundedHTTPQuery(w, r, "node_type", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		searchQuery, ok := parseBoundedHTTPQuery(w, r, "q", false, drive.MaxNodeNameBytes)
		if !ok {
			return
		}
		sortMode, ok := parseBoundedHTTPQuery(w, r, "sort", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		allParents, ok := parseBoolQueryDefaultFalse(w, r, "all_parents")
		if !ok {
			return
		}
		if allParents && parentID != "" {
			writeError(w, http.StatusBadRequest, "parent_id cannot be combined with all_parents")
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		nodes, err := service.ListNodes(r.Context(), drive.ListNodesRequest{
			UserID:     userID,
			ParentID:   parentID,
			Status:     status,
			NodeType:   nodeType,
			Query:      searchQuery,
			Sort:       sortMode,
			AllParents: allParents,
			Limit:      limit,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_nodes": nodes})
	})

	mux.HandleFunc("GET /api/v1/drive/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "status") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		status, ok := parseBoundedHTTPQuery(w, r, "status", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		node, err := service.GetNode(r.Context(), drive.GetNodeRequest{UserID: userID, NodeID: nodeID, Status: status})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("GET /api/v1/drive/nodes/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		rangeHeader, ok := singleHTTPHeaderValue(w, r, "Range", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		if rangeHeader != "" {
			metadata, err := service.StatFile(r.Context(), drive.OpenFileRequest{UserID: userID, NodeID: nodeID})
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			byteRange, err := parseSingleHTTPByteRange(rangeHeader, metadata.Object.Size)
			if err != nil {
				writeDriveRangeError(w, metadata.Object.Size, err.Error())
				return
			}
			download, err := service.OpenFileRange(r.Context(), drive.OpenFileRangeRequest{
				UserID: userID,
				NodeID: nodeID,
				Offset: byteRange.Offset,
				Length: byteRange.Length,
			})
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			defer download.Body.Close()
			writeDriveFilePartialDownloadHeaders(w, driveNodeWithStatSize(download.Node, metadata.Object.Size), byteRange)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.Copy(w, download.Body)
			return
		}
		download, err := service.OpenFile(r.Context(), drive.OpenFileRequest{UserID: userID, NodeID: nodeID})
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		defer download.Body.Close()
		writeDriveFileDownloadHeaders(w, download.Node)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, download.Body)
	})

	mux.HandleFunc("HEAD /api/v1/drive/nodes/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		metadata, err := service.StatFile(r.Context(), drive.OpenFileRequest{UserID: userID, NodeID: nodeID})
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeDriveFileDownloadHeaders(w, driveNodeWithStatSize(metadata.Node, metadata.Object.Size))
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("GET /api/v1/drive/usage", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		summary, err := service.GetUsageSummary(r.Context(), drive.GetUsageSummaryRequest{UserID: userID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_usage_summary": summary})
	})

	mux.HandleFunc("POST /api/v1/drive/folders", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			ParentID string `json:"parent_id"`
			Name     string `json:"name"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		node, err := service.CreateFolder(r.Context(), drive.CreateFolderRequest{
			UserID:   userID,
			ParentID: req.ParentID,
			Name:     req.Name,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("POST /api/v1/drive/upload-sessions", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			ParentID       string `json:"parent_id"`
			Name           string `json:"name"`
			DeclaredSize   int64  `json:"declared_size"`
			MIMEType       string `json:"mime_type"`
			StorageBackend string `json:"storage_backend"`
			ExpiresAt      string `json:"expires_at"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		var expiresAt time.Time
		if strings.TrimSpace(req.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
			if err != nil {
				writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
				return
			}
			expiresAt = parsed
		}
		session, err := service.CreateUploadSession(r.Context(), drive.CreateUploadSessionRequest{
			UserID:         userID,
			ParentID:       req.ParentID,
			Name:           req.Name,
			DeclaredSize:   req.DeclaredSize,
			MIMEType:       req.MIMEType,
			StorageBackend: req.StorageBackend,
			ExpiresAt:      expiresAt,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_upload_session": session})
	})

	mux.HandleFunc("GET /api/v1/drive/upload-sessions", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "status", "limit") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		status, ok := parseBoundedHTTPQuery(w, r, "status", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		sessions, err := service.ListUploadSessions(r.Context(), drive.ListUploadSessionsRequest{
			UserID: userID,
			Status: status,
			Limit:  limit,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_upload_sessions": sessions})
	})

	mux.HandleFunc("GET /api/v1/drive/upload-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		session, err := service.GetUploadSession(r.Context(), drive.GetUploadSessionRequest{UserID: userID, SessionID: sessionID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_upload_session": session})
	})

	mux.HandleFunc("DELETE /api/v1/drive/upload-sessions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		session, err := service.CancelUploadSession(r.Context(), drive.CancelUploadSessionRequest{UserID: userID, SessionID: sessionID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_upload_session": session})
	})

	mux.HandleFunc("PUT /api/v1/drive/upload-sessions/{id}/body", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		contentRange, ok := singleHTTPHeaderValue(w, r, "Content-Range", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		if contentRange != "" {
			writeError(w, http.StatusBadRequest, "content-range is not supported for drive upload session body storage")
			return
		}
		checksum, ok := singleHTTPHeaderValue(w, r, "X-Content-SHA256", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		body := http.MaxBytesReader(w, r.Body, drive.MaxUploadSessionBytes+1)
		session, err := service.StoreUploadSessionBody(r.Context(), drive.StoreUploadSessionBodyRequest{
			UserID:                 userID,
			SessionID:              sessionID,
			ExpectedChecksumSHA256: checksum,
			Body:                   body,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_upload_session": session})
	})

	mux.HandleFunc("POST /api/v1/drive/upload-sessions/{id}/finalize", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		sessionID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		node, err := service.FinalizeUploadSession(r.Context(), drive.FinalizeUploadSessionRequest{UserID: userID, SessionID: sessionID})
		if err != nil {
			writeDriveServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("POST /api/v1/drive/files/finalize", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			ParentID       string `json:"parent_id"`
			Name           string `json:"name"`
			StorageBackend string `json:"storage_backend"`
			StoragePath    string `json:"storage_path"`
			MIMEType       string `json:"mime_type"`
			ChecksumSHA256 string `json:"checksum_sha256"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		node, err := service.CreateFileFromObject(r.Context(), drive.CreateFileFromObjectRequest{
			UserID:         userID,
			ParentID:       req.ParentID,
			Name:           req.Name,
			StorageBackend: req.StorageBackend,
			StoragePath:    req.StoragePath,
			MIMEType:       req.MIMEType,
			ChecksumSHA256: req.ChecksumSHA256,
		})
		if err != nil {
			writeDriveServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("PUT /api/v1/drive/files/staged/{upload_id}/body", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id", "storage_backend") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		uploadID, ok := parseBoundedHTTPPathValue(w, r, "upload_id")
		if !ok {
			return
		}
		storageBackend, ok := parseBoundedHTTPQuery(w, r, "storage_backend", true, maxHTTPControlBytes)
		if !ok {
			return
		}
		body := http.MaxBytesReader(w, r.Body, drive.MaxDriveStagedObjectBytes+1)
		staged, err := service.StoreStagedObject(r.Context(), drive.StoreStagedObjectRequest{
			UserID:         userID,
			UploadID:       uploadID,
			StorageBackend: storageBackend,
			Body:           body,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_staged_object": staged})
	})

	mux.HandleFunc("POST /api/v1/drive/nodes/{id}/trash", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		node, updated, err := service.TrashNode(r.Context(), drive.TrashNodeRequest{UserID: userID, NodeID: nodeID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node, "updated": updated})
	})

	mux.HandleFunc("POST /api/v1/drive/nodes/{id}/restore", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		node, updated, err := service.RestoreNode(r.Context(), drive.RestoreNodeRequest{UserID: userID, NodeID: nodeID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node, "updated": updated})
	})

	mux.HandleFunc("PATCH /api/v1/drive/nodes/{id}/name", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
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
		node, err := service.RenameNode(r.Context(), drive.RenameNodeRequest{UserID: userID, NodeID: nodeID, Name: req.Name})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("PATCH /api/v1/drive/nodes/{id}/parent", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			ParentID string `json:"parent_id"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		node, err := service.MoveNode(r.Context(), drive.MoveNodeRequest{UserID: userID, NodeID: nodeID, ParentID: req.ParentID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("POST /api/v1/drive/nodes/{id}/copy", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			ParentID string `json:"parent_id"`
			Name     string `json:"name"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		node, err := service.CopyNode(r.Context(), drive.CopyNodeRequest{UserID: userID, NodeID: nodeID, ParentID: req.ParentID, Name: req.Name})
		if err != nil {
			writeDriveServiceError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_node": node})
	})

	mux.HandleFunc("POST /api/v1/drive/nodes/{id}/share-links", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			Permission string `json:"permission"`
			ExpiresAt  string `json:"expires_at"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid JSON body")
			return
		}
		var expiresAt time.Time
		if strings.TrimSpace(req.ExpiresAt) != "" {
			parsed, err := time.Parse(time.RFC3339, strings.TrimSpace(req.ExpiresAt))
			if err != nil {
				writeError(w, http.StatusBadRequest, "expires_at must be RFC3339")
				return
			}
			expiresAt = parsed
		}
		link, err := service.CreateShareLink(r.Context(), drive.CreateShareLinkRequest{
			UserID:     userID,
			NodeID:     nodeID,
			Permission: req.Permission,
			ExpiresAt:  expiresAt,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"drive_share_link": link})
	})

	mux.HandleFunc("GET /api/v1/drive/share-links", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "node_id", "status", "limit") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		nodeID, ok := parseBoundedHTTPQuery(w, r, "node_id", false, maxHTTPResourceIDBytes)
		if !ok {
			return
		}
		status, ok := parseBoundedHTTPQuery(w, r, "status", false, maxHTTPControlBytes)
		if !ok {
			return
		}
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		links, err := service.ListShareLinks(r.Context(), drive.ListShareLinksRequest{
			UserID: userID,
			NodeID: nodeID,
			Status: status,
			Limit:  limit,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_share_links": links})
	})

	mux.HandleFunc("GET /api/v1/drive/share-links/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		token, ok := parseDriveShareTokenPathValue(w, r)
		if !ok {
			return
		}
		resolved, err := service.ResolveShareLink(r.Context(), drive.ResolveShareLinkRequest{Token: token})
		if err != nil {
			writeDriveShareLinkError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_shared_file": driveSharedFileMetadata(resolved)})
	})

	mux.HandleFunc("GET /api/v1/drive/share-links/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		token, ok := parseDriveShareTokenPathValue(w, r)
		if !ok {
			return
		}
		rangeHeader, ok := singleHTTPHeaderValue(w, r, "Range", maxHTTPAuthHeaderBytes)
		if !ok {
			return
		}
		if rangeHeader != "" {
			metadata, err := service.StatSharedFile(r.Context(), drive.ResolveShareLinkRequest{Token: token})
			if err != nil {
				writeDriveShareLinkError(w, err)
				return
			}
			byteRange, err := parseSingleHTTPByteRange(rangeHeader, metadata.Object.Size)
			if err != nil {
				writeDriveRangeError(w, metadata.Object.Size, err.Error())
				return
			}
			download, err := service.OpenSharedFileRange(r.Context(), drive.ResolveShareLinkRequest{Token: token}, storage.RangeRequest{
				Offset: byteRange.Offset,
				Length: byteRange.Length,
			})
			if err != nil {
				writeDriveShareLinkError(w, err)
				return
			}
			defer download.Body.Close()
			writeDriveFilePartialDownloadHeaders(w, driveNodeWithStatSize(download.Node, metadata.Object.Size), byteRange)
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.Copy(w, download.Body)
			return
		}
		download, err := service.OpenSharedFile(r.Context(), drive.ResolveShareLinkRequest{Token: token})
		if err != nil {
			writeDriveShareLinkError(w, err)
			return
		}
		defer download.Body.Close()
		writeDriveFileDownloadHeaders(w, download.Node)
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, download.Body)
	})

	mux.HandleFunc("HEAD /api/v1/drive/share-links/{id}/download", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		token, ok := parseDriveShareTokenPathValue(w, r)
		if !ok {
			return
		}
		metadata, err := service.StatSharedFile(r.Context(), drive.ResolveShareLinkRequest{Token: token})
		if err != nil {
			writeDriveShareLinkError(w, err)
			return
		}
		writeDriveFileDownloadHeaders(w, driveNodeWithStatSize(metadata.Node, metadata.Object.Size))
		w.WriteHeader(http.StatusOK)
	})

	mux.HandleFunc("DELETE /api/v1/drive/share-links/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		linkID, ok := parseBoundedHTTPPathValue(w, r, "id")
		if !ok {
			return
		}
		link, err := service.RevokeShareLink(r.Context(), drive.RevokeShareLinkRequest{UserID: userID, LinkID: linkID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_share_link": link})
	})

	mux.HandleFunc("DELETE /api/v1/drive/nodes/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id") {
			return
		}
		userID, nodeID, ok := driveNodeRequestIdentity(w, r, tokenManager)
		if !ok {
			return
		}
		result, err := service.PermanentDeleteNode(r.Context(), drive.PermanentDeleteNodeRequest{UserID: userID, NodeID: nodeID})
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"drive_delete": result})
	})
}

type driveSharedFileMetadataResponse struct {
	NodeID         string    `json:"node_id"`
	Name           string    `json:"name"`
	MIMEType       string    `json:"mime_type,omitempty"`
	Size           int64     `json:"size"`
	ChecksumSHA256 string    `json:"checksum_sha256,omitempty"`
	Permission     string    `json:"permission"`
	ExpiresAt      time.Time `json:"expires_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func driveSharedFileMetadata(resolved drive.ResolvedShareLink) driveSharedFileMetadataResponse {
	return driveSharedFileMetadataResponse{
		NodeID:         resolved.Node.ID,
		Name:           resolved.Node.Name,
		MIMEType:       resolved.Node.MIMEType,
		Size:           resolved.Node.Size,
		ChecksumSHA256: safeSHA256Header(resolved.Node.ChecksumSHA256),
		Permission:     resolved.ShareLink.Permission,
		ExpiresAt:      resolved.ShareLink.ExpiresAt,
		UpdatedAt:      resolved.Node.UpdatedAt,
	}
}

func parseDriveShareTokenPathValue(w http.ResponseWriter, r *http.Request) (string, bool) {
	value := strings.TrimSpace(r.PathValue("id"))
	if value == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return "", false
	}
	if strings.ContainsAny(value, "\r\n") {
		writeError(w, http.StatusBadRequest, "id must not contain CR or LF")
		return "", false
	}
	if len(value) > drive.MaxShareLinkTokenBytes {
		writeError(w, http.StatusBadRequest, "id is too long")
		return "", false
	}
	return value, true
}

func writeDriveShareLinkError(w http.ResponseWriter, err error) {
	if errors.Is(err, drive.ErrShareLinkPermissionDenied) {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	if strings.Contains(err.Error(), "not found") {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeDriveServiceError(w http.ResponseWriter, err error) {
	if errors.Is(err, mail.ErrMailboxFull) {
		writeError(w, http.StatusInsufficientStorage, err.Error())
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

func writeDriveFileDownloadHeaders(w http.ResponseWriter, node drive.Node) {
	w.Header().Set("Content-Type", attachmentContentType(node.MIMEType))
	w.Header().Set("Content-Disposition", contentDispositionAttachment(node.Name))
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Accept-Ranges", "bytes")
	if sha256Hex := safeSHA256Header(node.ChecksumSHA256); sha256Hex != "" {
		w.Header().Set("X-Gogomail-Drive-SHA256", sha256Hex)
	}
	if node.Size >= 0 {
		w.Header().Set("Content-Length", strconv.FormatInt(node.Size, 10))
	}
}

type httpByteRange struct {
	Offset int64
	Length int64
	Total  int64
}

func writeDriveFilePartialDownloadHeaders(w http.ResponseWriter, node drive.Node, byteRange httpByteRange) {
	partial := driveNodeWithStatSize(node, byteRange.Length)
	writeDriveFileDownloadHeaders(w, partial)
	w.Header().Set("Content-Range", "bytes "+strconv.FormatInt(byteRange.Offset, 10)+"-"+strconv.FormatInt(byteRange.Offset+byteRange.Length-1, 10)+"/"+strconv.FormatInt(byteRange.Total, 10))
}

func writeDriveRangeError(w http.ResponseWriter, total int64, message string) {
	w.Header().Set("Content-Range", "bytes */"+strconv.FormatInt(total, 10))
	writeError(w, http.StatusRequestedRangeNotSatisfiable, message)
}

func parseSingleHTTPByteRange(value string, total int64) (httpByteRange, error) {
	value = strings.TrimSpace(value)
	if total < 0 {
		return httpByteRange{}, errInvalidDriveRange()
	}
	if !strings.HasPrefix(strings.ToLower(value), "bytes=") {
		return httpByteRange{}, errInvalidDriveRange()
	}
	spec := strings.TrimSpace(value[len("bytes="):])
	if spec == "" || strings.Contains(spec, ",") {
		return httpByteRange{}, errInvalidDriveRange()
	}
	parts := strings.Split(spec, "-")
	if len(parts) != 2 {
		return httpByteRange{}, errInvalidDriveRange()
	}
	startValue := strings.TrimSpace(parts[0])
	endValue := strings.TrimSpace(parts[1])
	if startValue == "" {
		suffix, ok := parseHTTPRangeNumber(endValue)
		if !ok || suffix <= 0 || total == 0 {
			return httpByteRange{}, errInvalidDriveRange()
		}
		if suffix >= total {
			return httpByteRange{Offset: 0, Length: total, Total: total}, nil
		}
		return httpByteRange{Offset: total - suffix, Length: suffix, Total: total}, nil
	}
	start, ok := parseHTTPRangeNumber(startValue)
	if !ok {
		return httpByteRange{}, errInvalidDriveRange()
	}
	if start >= total {
		return httpByteRange{}, errInvalidDriveRange()
	}
	end := total - 1
	if endValue != "" {
		parsedEnd, ok := parseHTTPRangeNumber(endValue)
		if !ok || parsedEnd < start {
			return httpByteRange{}, errInvalidDriveRange()
		}
		if parsedEnd < end {
			end = parsedEnd
		}
	}
	return httpByteRange{Offset: start, Length: end - start + 1, Total: total}, nil
}

func parseHTTPRangeNumber(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	return parsed, err == nil
}

func errInvalidDriveRange() error {
	return errDriveRangeInvalid{}
}

type errDriveRangeInvalid struct{}

func (errDriveRangeInvalid) Error() string {
	return "range must be a single satisfiable bytes range"
}

func driveNodeWithStatSize(node drive.Node, size int64) drive.Node {
	node.Size = size
	return node
}

func driveNodeRequestIdentity(w http.ResponseWriter, r *http.Request, tokenManager *auth.TokenManager) (string, string, bool) {
	userID, ok := userIDFromRequest(w, r, tokenManager)
	if !ok {
		return "", "", false
	}
	nodeID, ok := parseBoundedHTTPPathValue(w, r, "id")
	if !ok {
		return "", "", false
	}
	return userID, nodeID, true
}
