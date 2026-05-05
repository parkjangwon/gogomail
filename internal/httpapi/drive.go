package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/drive"
)

type DriveService interface {
	CreateFolder(ctx context.Context, req drive.CreateFolderRequest) (drive.Node, error)
	CreateFileFromObject(ctx context.Context, req drive.CreateFileFromObjectRequest) (drive.Node, error)
	StoreStagedObject(ctx context.Context, req drive.StoreStagedObjectRequest) (drive.StagedObject, error)
	CreateUploadSession(ctx context.Context, req drive.CreateUploadSessionRequest) (drive.UploadSession, error)
	GetUploadSession(ctx context.Context, req drive.GetUploadSessionRequest) (drive.UploadSession, error)
	ListNodes(ctx context.Context, req drive.ListNodesRequest) ([]drive.Node, error)
	GetNode(ctx context.Context, req drive.GetNodeRequest) (drive.Node, error)
	TrashNode(ctx context.Context, req drive.TrashNodeRequest) (drive.Node, int64, error)
	RestoreNode(ctx context.Context, req drive.RestoreNodeRequest) (drive.Node, int64, error)
	RenameNode(ctx context.Context, req drive.RenameNodeRequest) (drive.Node, error)
	MoveNode(ctx context.Context, req drive.MoveNodeRequest) (drive.Node, error)
	PermanentDeleteNode(ctx context.Context, req drive.PermanentDeleteNodeRequest) (drive.PermanentDeleteServiceResult, error)
}

func RegisterDriveRoutes(mux *http.ServeMux, service DriveService, tokenManager *auth.TokenManager) {
	mux.HandleFunc("GET /api/v1/drive/nodes", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, "user_id", "parent_id", "status", "limit") {
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
		limit, ok := parseQueryLimit(w, r)
		if !ok {
			return
		}
		nodes, err := service.ListNodes(r.Context(), drive.ListNodesRequest{
			UserID:   userID,
			ParentID: parentID,
			Status:   status,
			Limit:    limit,
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
			writeError(w, http.StatusBadRequest, err.Error())
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
