package mailservice

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/storage"
)

func (s *Service) ListAttachments(ctx context.Context, userID string, messageID string) ([]maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return nil, err
	}
	return s.repository.ListAttachments(ctx, userID, messageID)
}

func (s *Service) CreateAttachmentUpload(ctx context.Context, req CreateAttachmentUploadRequest) (maildb.Attachment, error) {
	req = normalizeCreateAttachmentUploadRequest(req)
	if err := ValidateCreateAttachmentUploadRequest(req); err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.Size); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}
	return repo.CreateAttachmentUpload(ctx, maildb.CreateAttachmentUploadRequest{
		UserID:      req.UserID,
		DraftID:     req.DraftID,
		Filename:    req.Filename,
		Size:        req.Size,
		MIMEType:    req.MIMEType,
		StoragePath: req.StoragePath,
	})
}

func (s *Service) UploadAttachment(ctx context.Context, req UploadAttachmentRequest) (maildb.Attachment, error) {
	req = normalizeUploadAttachmentRequest(req)
	if err := ValidateUploadAttachmentRequest(req); err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.Size); err != nil {
		return maildb.Attachment{}, err
	}
	if s.store == nil {
		return maildb.Attachment{}, fmt.Errorf("mail storage is required")
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}

	path, err := s.buildAttachmentUploadPath(ctx, req.UserID, randomObjectID(), safeObjectFilename(req.Filename))
	if err != nil {
		return maildb.Attachment{}, err
	}
	counter := &countingReader{reader: req.Body}
	limitedBody := &io.LimitedReader{R: counter, N: MaxAttachmentUploadBytes + 1}
	if err := s.store.Put(ctx, path, limitedBody); err != nil {
		return maildb.Attachment{}, fmt.Errorf("store attachment upload: %w", err)
	}
	if limitedBody.N == 0 {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, fmt.Errorf("attachment body exceeds %d bytes", MaxAttachmentUploadBytes)
	}
	if counter.n != req.Size {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, fmt.Errorf("attachment body size %d does not match declared size %d", counter.n, req.Size)
	}

	attachment, err := repo.CreateAttachmentUpload(ctx, maildb.CreateAttachmentUploadRequest{
		UserID:      req.UserID,
		DraftID:     req.DraftID,
		Filename:    req.Filename,
		Size:        req.Size,
		MIMEType:    req.MIMEType,
		StoragePath: path,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return maildb.Attachment{}, err
	}
	return attachment, nil
}

func (s *Service) CancelAttachmentUpload(ctx context.Context, userID string, attachmentID string) (maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	attachmentID = strings.TrimSpace(attachmentID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.Attachment{}, err
	}
	if err := validateServiceResourceID("attachment_id", attachmentID); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload repository is required")
	}
	attachment, err := repo.CancelAttachmentUpload(ctx, userID, attachmentID)
	if err != nil {
		return maildb.Attachment{}, err
	}
	if s.store != nil && strings.TrimSpace(attachment.StoragePath) != "" {
		storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
		if err != nil {
			return attachment, err
		}
		if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return attachment, fmt.Errorf("delete canceled attachment object: %w", err)
		}
	}
	return attachment, nil
}

func (s *Service) CreateAttachmentUploadSession(ctx context.Context, req CreateAttachmentUploadSessionRequest) (maildb.AttachmentUploadSession, error) {
	req = normalizeCreateAttachmentUploadSessionRequest(req)
	if err := ValidateCreateAttachmentUploadSessionRequest(req); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	now := time.Now().UTC()
	if !req.ExpiresAt.After(now) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("expires_at must be in the future")
	}
	if req.ExpiresAt.After(now.Add(MaxAttachmentUploadSessionTTL)) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("expires_at must be within %s", MaxAttachmentUploadSessionTTL)
	}
	if err := s.enforceAttachmentPolicy(ctx, req.UserID, req.DeclaredSize); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	return repo.CreateAttachmentUploadSession(ctx, maildb.CreateAttachmentUploadSessionRequest{
		UserID:       req.UserID,
		DraftID:      req.DraftID,
		Filename:     req.Filename,
		DeclaredSize: req.DeclaredSize,
		MIMEType:     req.MIMEType,
		ExpiresAt:    req.ExpiresAt,
	})
}

func (s *Service) CancelAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if err := validateServiceResourceID("session_id", sessionID); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	session, err := repo.CancelAttachmentUploadSession(ctx, maildb.CancelAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if s.store != nil && strings.TrimSpace(session.StoragePath) != "" {
		storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
		if err != nil {
			return session, err
		}
		if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return session, fmt.Errorf("delete canceled upload session object: %w", err)
		}
	}
	return session, nil
}

func (s *Service) GetAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.AttachmentUploadSession, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := maildb.ValidateGetAttachmentUploadSessionRequest(maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	return repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func (s *Service) StoreAttachmentUploadSessionBody(ctx context.Context, req StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error) {
	req = normalizeStoreAttachmentUploadSessionBodyRequest(req)
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(req); err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if s.store == nil {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("mail storage is required")
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session repository is required")
	}
	session, err := repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
	})
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	if session.Status != "pending" && session.Status != "uploading" && session.Status != "failed" {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q is not writable", req.SessionID)
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session %q is expired", req.SessionID)
	}

	if req.ContentRange != nil {
		return s.storeChunk(ctx, repo, session, req)
	}

	path, err := s.buildAttachmentUploadSessionObjectPath(ctx, req.UserID, req.SessionID, "bodies", randomObjectID())
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	counter := &countingReader{reader: req.Body}
	limitedBody := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if err := s.store.Put(ctx, path, io.TeeReader(limitedBody, hash)); err != nil {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("store attachment upload session body: %w", err)
	}
	if limitedBody.N == 0 {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session body exceeds declared size %d", session.DeclaredSize)
	}
	if counter.n != session.DeclaredSize {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session body size %d does not match declared size %d", counter.n, session.DeclaredSize)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if req.ExpectedChecksumSHA256 != "" && checksum != req.ExpectedChecksumSHA256 {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("attachment upload session checksum %s does not match expected %s", checksum, req.ExpectedChecksumSHA256)
	}
	stored, err := repo.StoreAttachmentUploadSessionBody(ctx, maildb.StoreAttachmentUploadSessionBodyRequest{
		UserID:         req.UserID,
		SessionID:      req.SessionID,
		ReceivedSize:   counter.n,
		StoragePath:    path,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, err
	}
	if previousPath := strings.TrimSpace(session.StoragePath); previousPath != "" && previousPath != path {
		if previousPath, err := validateUploadSessionObjectPath(previousPath); err == nil {
			_ = s.store.Delete(ctx, previousPath)
		}
	}
	return stored, nil
}

func (s *Service) storeChunk(ctx context.Context, repo AttachmentUploadSessionRepository, session maildb.AttachmentUploadSession, req StoreAttachmentUploadSessionBodyRequest) (maildb.AttachmentUploadSession, error) {
	cr := req.ContentRange
	path, err := s.buildAttachmentUploadSessionObjectPath(ctx, req.UserID, req.SessionID, "chunks", fmt.Sprintf("%d-%d", cr.FirstByte, cr.LastByte))
	if err != nil {
		return maildb.AttachmentUploadSession{}, err
	}
	chunkSize := cr.LastByte - cr.FirstByte + 1
	counter := &countingReader{reader: req.Body}
	limitedBody := &io.LimitedReader{R: counter, N: chunkSize + 1}
	if err := s.store.Put(ctx, path, limitedBody); err != nil {
		return maildb.AttachmentUploadSession{}, fmt.Errorf("store chunk: %w", err)
	}
	if limitedBody.N == 0 {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("chunk body exceeds chunk size %d", chunkSize)
	}
	if counter.n != chunkSize {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, fmt.Errorf("chunk body size %d does not match Content-Range size %d", counter.n, chunkSize)
	}
	stored, err := repo.StoreAttachmentUploadSessionChunk(ctx, maildb.StoreAttachmentUploadSessionChunkRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		ContentRange: maildb.ContentRange{
			FirstByte: cr.FirstByte,
			LastByte:  cr.LastByte,
			TotalSize: cr.TotalSize,
		},
		StoragePath: path,
	})
	if err != nil {
		_ = s.store.Delete(ctx, path)
		return maildb.AttachmentUploadSession{}, err
	}
	return stored, nil
}

func (s *Service) FinalizeAttachmentUploadSession(ctx context.Context, userID string, sessionID string) (maildb.Attachment, error) {
	userID = strings.TrimSpace(userID)
	sessionID = strings.TrimSpace(sessionID)
	if err := maildb.ValidateFinalizeAttachmentUploadSessionRequest(maildb.FinalizeAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	}); err != nil {
		return maildb.Attachment{}, err
	}
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return maildb.Attachment{}, fmt.Errorf("attachment upload session repository is required")
	}
	if s.store == nil {
		return maildb.Attachment{}, fmt.Errorf("mail storage is required")
	}
	session, err := repo.GetAttachmentUploadSession(ctx, maildb.GetAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
	if err != nil {
		return maildb.Attachment{}, err
	}
	if err := s.verifyUploadSessionBody(ctx, session); err != nil {
		return maildb.Attachment{}, err
	}
	return repo.FinalizeAttachmentUploadSession(ctx, maildb.FinalizeAttachmentUploadSessionRequest{
		UserID:    userID,
		SessionID: sessionID,
	})
}

func (s *Service) verifyUploadSessionBody(ctx context.Context, session maildb.AttachmentUploadSession) error {
	if session.Status != "uploading" {
		return fmt.Errorf("attachment upload session %q is not ready for finalization", session.ID)
	}
	if session.ReceivedSize != session.DeclaredSize || session.DeclaredSize < 0 {
		return fmt.Errorf("attachment upload session %q has incomplete body", session.ID)
	}
	if strings.TrimSpace(session.StoragePath) == "" {
		return fmt.Errorf("attachment upload session %q storage path is required", session.ID)
	}
	storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
	if err != nil {
		return err
	}
	if !isLowerSHA256Hex(strings.TrimSpace(session.ChecksumSHA256)) {
		return fmt.Errorf("attachment upload session %q checksum is required", session.ID)
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return fmt.Errorf("attachment upload session %q is expired", session.ID)
	}
	body, err := s.store.Get(ctx, storagePath)
	if err != nil {
		return fmt.Errorf("open attachment upload session body: %w", err)
	}
	defer body.Close()

	counter := &countingReader{reader: body}
	limitedBody := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if _, err := io.Copy(hash, limitedBody); err != nil {
		return fmt.Errorf("read attachment upload session body: %w", err)
	}
	if limitedBody.N == 0 || counter.n != session.DeclaredSize {
		return fmt.Errorf("attachment upload session body size %d does not match declared size %d", counter.n, session.DeclaredSize)
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if checksum != session.ChecksumSHA256 {
		return fmt.Errorf("attachment upload session checksum %s does not match stored %s", checksum, session.ChecksumSHA256)
	}
	return nil
}

func (s *Service) ExpireAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error) {
	repo, ok := s.repository.(AttachmentUploadSessionRepository)
	if !ok {
		return nil, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return nil, err
	}
	expired, err := repo.ExpireAttachmentUploadSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	if s.store != nil {
		for _, session := range expired {
			if strings.TrimSpace(session.StoragePath) == "" {
				continue
			}
			storagePath, err := validateUploadSessionObjectPath(session.StoragePath)
			if err != nil {
				return expired, err
			}
			if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				return expired, fmt.Errorf("delete expired upload session object: %w", err)
			}
		}
	}
	return expired, nil
}

func (s *Service) CountStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadSessionCount, error) {
	repo, ok := s.repository.(AttachmentUploadSessionCleanupRepository)
	if !ok {
		return maildb.StaleAttachmentUploadSessionCount{}, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return maildb.StaleAttachmentUploadSessionCount{}, err
	}
	return repo.CountStaleAttachmentUploadSessions(ctx, req)
}

func (s *Service) ListStaleAttachmentUploadSessions(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadSessionCandidate, error) {
	repo, ok := s.repository.(AttachmentUploadSessionCleanupRepository)
	if !ok {
		return nil, fmt.Errorf("attachment upload session repository is required")
	}
	req := maildb.ExpireAttachmentUploadSessionsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireAttachmentUploadSessionsRequest(req); err != nil {
		return nil, err
	}
	return repo.ListStaleAttachmentUploadSessions(ctx, req)
}

func normalizeCreateAttachmentUploadRequest(req CreateAttachmentUploadRequest) CreateAttachmentUploadRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	req.StoragePath = strings.TrimSpace(req.StoragePath)
	return req
}

func normalizeCreateAttachmentUploadSessionRequest(req CreateAttachmentUploadSessionRequest) CreateAttachmentUploadSessionRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	return req
}

func normalizeStoreAttachmentUploadSessionBodyRequest(req StoreAttachmentUploadSessionBodyRequest) StoreAttachmentUploadSessionBodyRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.SessionID = strings.TrimSpace(req.SessionID)
	req.ExpectedChecksumSHA256 = strings.TrimSpace(req.ExpectedChecksumSHA256)
	return req
}

func validateUploadSessionObjectPath(storagePath string) (string, error) {
	storagePath = strings.TrimSpace(storagePath)
	if err := validateAttachmentStoragePath(storagePath); err != nil {
		return "", err
	}
	if storagePath == "" {
		return "", fmt.Errorf("storage_path is required")
	}
	if !strings.HasPrefix(storagePath, "upload-sessions/") && !(strings.HasPrefix(storagePath, "uploads/") && strings.Contains(storagePath, "/upload-sessions/")) {
		return "", fmt.Errorf("storage_path must use upload-sessions prefix")
	}
	return storagePath, nil
}

func (s *Service) userStorageScope(ctx context.Context, userID string) (maildb.UserStorageScope, bool, error) {
	userID = strings.TrimSpace(userID)
	if err := validateServiceResourceID("user_id", userID); err != nil {
		return maildb.UserStorageScope{}, false, err
	}
	repo, ok := s.repository.(UserStorageScopeRepository)
	if !ok {
		return maildb.UserStorageScope{UserID: userID}, false, nil
	}
	scope, err := repo.UserStorageScope(ctx, userID)
	if err != nil {
		return maildb.UserStorageScope{}, false, err
	}
	return scope, true, nil
}

func (s *Service) buildAttachmentUploadPath(ctx context.Context, userID string, objectID string, filename string) (string, error) {
	scope, scoped, err := s.userStorageScope(ctx, userID)
	if err != nil {
		return "", err
	}
	if !scoped {
		return strings.Join([]string{
			"uploads",
			safeObjectPathSegment(scope.UserID),
			safeObjectPathSegment(objectID),
			filename,
		}, "/"), nil
	}
	return strings.Join([]string{
		"uploads",
		safeObjectPathSegment(scope.CompanyID),
		safeObjectPathSegment(scope.DomainID),
		"users",
		safeObjectPathSegment(scope.UserID),
		"attachments",
		safeObjectPathSegment(objectID),
		filename,
	}, "/"), nil
}

func (s *Service) buildAttachmentUploadSessionObjectPath(ctx context.Context, userID string, sessionID string, kind string, objectID string) (string, error) {
	scope, scoped, err := s.userStorageScope(ctx, userID)
	if err != nil {
		return "", err
	}
	if !scoped {
		return strings.Join([]string{
			"upload-sessions",
			safeObjectPathSegment(scope.UserID),
			safeObjectPathSegment(sessionID),
			safeObjectPathSegment(kind),
			safeObjectPathSegment(objectID),
		}, "/"), nil
	}
	return strings.Join([]string{
		"uploads",
		safeObjectPathSegment(scope.CompanyID),
		safeObjectPathSegment(scope.DomainID),
		"users",
		safeObjectPathSegment(scope.UserID),
		"upload-sessions",
		safeObjectPathSegment(sessionID),
		safeObjectPathSegment(kind),
		safeObjectPathSegment(objectID),
	}, "/"), nil
}

func normalizeUploadAttachmentRequest(req UploadAttachmentRequest) UploadAttachmentRequest {
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.Filename = strings.TrimSpace(req.Filename)
	req.MIMEType = strings.TrimSpace(req.MIMEType)
	return req
}

type countingReader struct {
	reader io.Reader
	n      int64
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	r.n += int64(n)
	return n, err
}

func safeObjectFilename(filename string) string {
	filename = strings.ReplaceAll(strings.TrimSpace(filename), "/", "_")
	filename = strings.ReplaceAll(filename, `\`, "_")
	if filename == "" {
		return "attachment"
	}
	return filename
}

func safeObjectPathSegment(value string) string {
	value = strings.TrimSpace(value)
	var b strings.Builder
	b.Grow(len(value))
	lastUnderscore := false
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "_.")
	if out == "" {
		return "unknown"
	}
	return out
}

type AttachmentDownload struct {
	Attachment maildb.Attachment
	Body       io.ReadCloser
}

type AttachmentMetadata struct {
	Attachment maildb.Attachment
	Object     storage.ObjectInfo
}

func (s *Service) OpenAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (AttachmentDownload, error) {
	if s.store == nil {
		return AttachmentDownload{}, fmt.Errorf("mail storage is required")
	}
	attachment, storagePath, err := s.attachmentObject(ctx, userID, messageID, attachmentID)
	if err != nil {
		return AttachmentDownload{}, err
	}
	body, err := s.store.Get(ctx, storagePath)
	if err != nil {
		return AttachmentDownload{}, fmt.Errorf("open attachment body: %w", err)
	}
	return AttachmentDownload{Attachment: attachment, Body: body}, nil
}

func (s *Service) StatAttachment(ctx context.Context, userID string, messageID string, attachmentID string) (AttachmentMetadata, error) {
	if s.store == nil {
		return AttachmentMetadata{}, fmt.Errorf("mail storage is required")
	}
	attachment, storagePath, err := s.attachmentObject(ctx, userID, messageID, attachmentID)
	if err != nil {
		return AttachmentMetadata{}, err
	}
	info, err := s.store.Stat(ctx, storagePath)
	if err != nil {
		return AttachmentMetadata{}, fmt.Errorf("stat attachment body: %w", err)
	}
	return AttachmentMetadata{Attachment: attachment, Object: info}, nil
}

func (s *Service) attachmentObject(ctx context.Context, userID string, messageID string, attachmentID string) (maildb.Attachment, string, error) {
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	attachmentID = strings.TrimSpace(attachmentID)
	if err := validateServiceResourceID("message_id", messageID); err != nil {
		return maildb.Attachment{}, "", err
	}
	if err := validateServiceResourceID("attachment_id", attachmentID); err != nil {
		return maildb.Attachment{}, "", err
	}
	attachment, err := s.repository.GetAttachment(ctx, userID, messageID, attachmentID)
	if err != nil {
		return maildb.Attachment{}, "", err
	}
	storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
	if err != nil {
		return maildb.Attachment{}, "", err
	}
	return attachment, storagePath, nil
}

func (s *Service) ExpireStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	repo, ok := s.repository.(AttachmentCleanupRepository)
	if !ok {
		return nil, fmt.Errorf("attachment cleanup repository is required")
	}
	req := maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return nil, err
	}
	expired, err := repo.ExpireStaleAttachmentUploads(ctx, req)
	if err != nil {
		return nil, err
	}
	if s.store == nil {
		return expired, nil
	}
	var deleteErrors []error
	for _, attachment := range expired {
		if strings.TrimSpace(attachment.StoragePath) != "" {
			storagePath, err := requireStoredObjectPath("attachment body", attachment.StoragePath)
			if err != nil {
				deleteErrors = append(deleteErrors, fmt.Errorf("%s: %w", attachment.ID, err))
				continue
			}
			if err := s.store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
				deleteErrors = append(deleteErrors, fmt.Errorf("%s: %w", attachment.ID, err))
			}
		}
	}
	if len(deleteErrors) > 0 {
		return expired, fmt.Errorf("delete expired attachment objects: %w", errors.Join(deleteErrors...))
	}
	return expired, nil
}

func (s *Service) CountStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) (maildb.StaleAttachmentUploadCount, error) {
	repo, ok := s.repository.(AttachmentCleanupRepository)
	if !ok {
		return maildb.StaleAttachmentUploadCount{}, fmt.Errorf("attachment cleanup repository is required")
	}
	req := maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return maildb.StaleAttachmentUploadCount{}, err
	}
	return repo.CountStaleAttachmentUploads(ctx, req)
}

func (s *Service) ListStaleAttachmentUploads(ctx context.Context, before time.Time, limit int) ([]maildb.StaleAttachmentUploadCandidate, error) {
	repo, ok := s.repository.(AttachmentCleanupRepository)
	if !ok {
		return nil, fmt.Errorf("attachment cleanup repository is required")
	}
	req := maildb.ExpireStaleAttachmentUploadsRequest{
		Before: before,
		Limit:  limit,
	}
	if err := maildb.ValidateExpireStaleAttachmentUploadsRequest(req); err != nil {
		return nil, err
	}
	return repo.ListStaleAttachmentUploads(ctx, req)
}