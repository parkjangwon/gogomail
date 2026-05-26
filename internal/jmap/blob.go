package jmap

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/google/uuid"
)

type blobInfo struct {
	AccountID string `json:"accountId"`
	BlobID    string `json:"blobId"`
	Type      string `json:"type"`
	Size      int64  `json:"size"`
}

// ServeUpload handles POST /jmap/upload/{accountId}/
func (h *Handler) ServeUpload(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"type": "unauthorized"})
		return
	}

	accountID := r.PathValue("accountId")
	if accountID != userID {
		http.Error(w, `{"type":"forbidden"}`, http.StatusForbidden)
		return
	}

	if h.deps.Store == nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"type": "serverFail"})
		return
	}

	blobID := uuid.New().String()
	storagePath := "jmap-blobs/" + accountID + "/" + blobID
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	if err := h.deps.Store.Put(r.Context(), storagePath, r.Body); err != nil {
		writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"type": "serverFail"})
		return
	}

	var size int64
	if info, err := h.deps.Store.Stat(r.Context(), storagePath); err == nil {
		size = info.Size
	}

	// Record in jmap_blobs. Log on error — the blob is stored but won't be
	// retrievable via the normal download path until the record exists.
	if h.deps.Repo != nil {
		if _, err := h.deps.Repo.DB().ExecContext(r.Context(),
			`INSERT INTO jmap_blobs (id, account_id, storage_path, content_type, size)
             VALUES ($1::uuid, $2::uuid, $3, $4, $5)`,
			blobID, accountID, storagePath, contentType, size,
		); err != nil {
			slog.Error("jmap: failed to record blob in jmap_blobs", "blobId", blobID, "err", err)
			writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"type": "serverFail"})
			return
		}
	}

	writeJSONResponse(w, http.StatusCreated, blobInfo{
		AccountID: accountID,
		BlobID:    blobID,
		Type:      contentType,
		Size:      size,
	})
}

// ServeDownload handles GET /jmap/download/{accountId}/{blobId}/{name}
func (h *Handler) ServeDownload(w http.ResponseWriter, r *http.Request) {
	userID, ok := h.userIDFromBearer(r)
	if !ok {
		writeJSONResponse(w, http.StatusUnauthorized, map[string]string{"type": "unauthorized"})
		return
	}

	accountID := r.PathValue("accountId")
	blobID := r.PathValue("blobId")
	name := r.PathValue("name")

	if accountID != userID {
		http.Error(w, `{"type":"forbidden"}`, http.StatusForbidden)
		return
	}

	var storagePath, contentType string
	if h.deps.Repo != nil {
		err := h.deps.Repo.DB().QueryRowContext(r.Context(),
			`SELECT storage_path, content_type FROM jmap_blobs
             WHERE id = $1::uuid AND account_id = $2::uuid AND expires_at > now()`,
			blobID, accountID,
		).Scan(&storagePath, &contentType)
		if err != nil {
			if !errors.Is(err, sql.ErrNoRows) {
				// Real DB error — don't fall back silently
				slog.Error("jmap: blob lookup failed", "blobId", blobID, "err", err)
				writeJSONResponse(w, http.StatusInternalServerError, map[string]string{"type": "serverFail"})
				return
			}
			// ErrNoRows — treat blobId as a direct storage path (e.g. message body)
			storagePath = blobID
			contentType = "application/octet-stream"
		}
	} else {
		// No DB (test mode) — treat blobId as a direct storage path
		storagePath = blobID
		contentType = "application/octet-stream"
	}

	if h.deps.Store == nil {
		http.Error(w, `{"type":"notFound"}`, http.StatusNotFound)
		return
	}

	reader, err := h.deps.Store.Get(r.Context(), storagePath)
	if err != nil {
		http.Error(w, `{"type":"notFound"}`, http.StatusNotFound)
		return
	}
	defer reader.Close()

	// Set filename for download per RFC 8620 §6.2
	if name != "" {
		w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
	}
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "private, max-age=3600")
	if _, err := io.Copy(w, reader); err != nil {
		// Headers already sent; log but cannot change status code
		slog.Warn("jmap: blob download copy error", "blobId", blobID, "err", err)
	}
}
