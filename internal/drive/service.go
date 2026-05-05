package drive

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

	"github.com/gogomail/gogomail/internal/storage"
)

type ObjectCleanupFailureRecorder interface {
	RecordObjectCleanupFailure(context.Context, ObjectCleanupFailure) (ObjectCleanupFailure, error)
}

type ObjectCleanupFailureStore interface {
	ObjectCleanupFailureRecorder
	ListObjectCleanupFailures(context.Context, ListObjectCleanupFailuresRequest) ([]ObjectCleanupFailure, error)
	ResolveObjectCleanupFailure(context.Context, ResolveObjectCleanupFailureRequest) (ObjectCleanupFailure, error)
}

type Service struct {
	repo                   *Repository
	stores                 map[string]storage.Store
	cleanupFailureRecorder ObjectCleanupFailureRecorder
	cleanupFailureStore    ObjectCleanupFailureStore
}

type PermanentDeleteServiceResult struct {
	PermanentDelete PermanentDeleteResult `json:"permanent_delete"`
	Cleanup         ObjectCleanupResult   `json:"cleanup"`
}

type RetryObjectCleanupFailuresResult struct {
	Scanned  int `json:"scanned"`
	Deleted  int `json:"deleted"`
	Resolved int `json:"resolved"`
	Failed   int `json:"failed"`
}

func NewService(repo *Repository, stores map[string]storage.Store) *Service {
	copiedStores := make(map[string]storage.Store, len(stores))
	for backend, store := range stores {
		copiedStores[backend] = store
	}
	service := &Service{repo: repo, stores: copiedStores}
	if repo != nil {
		service.cleanupFailureRecorder = repo
		service.cleanupFailureStore = repo
	}
	return service
}

func (s *Service) WithObjectCleanupFailureRecorder(recorder ObjectCleanupFailureRecorder) *Service {
	if s == nil {
		return nil
	}
	s.cleanupFailureRecorder = recorder
	return s
}

func (s *Service) WithObjectCleanupFailureStore(store ObjectCleanupFailureStore) *Service {
	if s == nil {
		return nil
	}
	s.cleanupFailureStore = store
	s.cleanupFailureRecorder = store
	return s
}

func (s *Service) CreateFolder(ctx context.Context, req CreateFolderRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.CreateFolder(ctx, req)
}

func (s *Service) CreateFileFromObject(ctx context.Context, req CreateFileFromObjectRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	storageBackend, err := validateStorageBackend(req.StorageBackend)
	if err != nil {
		return Node{}, err
	}
	store := s.stores[storageBackend]
	if store == nil {
		return Node{}, fmt.Errorf("storage store %q is required", storageBackend)
	}
	return s.repo.CreateFileFromObject(ctx, store, req)
}

func (s *Service) CreateUploadSession(ctx context.Context, req CreateUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	if strings.TrimSpace(req.UploadID) == "" {
		uploadID, err := NewUploadID()
		if err != nil {
			return UploadSession{}, err
		}
		req.UploadID = uploadID
	}
	return s.repo.CreateUploadSession(ctx, req)
}

func (s *Service) GetUploadSession(ctx context.Context, req GetUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.GetUploadSession(ctx, req)
}

func (s *Service) CancelUploadSession(ctx context.Context, req CancelUploadSessionRequest) (UploadSession, error) {
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.CancelUploadSession(ctx, req)
}

func (s *Service) ExpireUploadSessions(ctx context.Context, req ExpireUploadSessionsRequest) ([]UploadSession, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateExpireUploadSessionsRequest(req)
	if err != nil {
		return nil, err
	}
	expired, err := s.repo.ExpireUploadSessions(ctx, req)
	if err != nil {
		return nil, err
	}
	for _, session := range expired {
		storagePath := strings.TrimSpace(session.StoragePath)
		if storagePath == "" {
			continue
		}
		if _, err := storage.ValidateObjectPath(storagePath); err != nil {
			return expired, fmt.Errorf("expired drive upload session storage path is invalid: %w", err)
		}
		store := s.stores[session.StorageBackend]
		if store == nil {
			return expired, fmt.Errorf("storage store %q is required", session.StorageBackend)
		}
		if err := store.Delete(ctx, storagePath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return expired, fmt.Errorf("delete expired drive upload session body: %w", err)
		}
	}
	return expired, nil
}

func (s *Service) StoreUploadSessionBody(ctx context.Context, req StoreUploadSessionBodyRequest) (UploadSession, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if s == nil || s.repo == nil {
		return UploadSession{}, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateStoreUploadSessionBodyRequest(req)
	if err != nil {
		return UploadSession{}, err
	}
	session, err := s.repo.GetUploadSession(ctx, GetUploadSessionRequest{UserID: req.UserID, SessionID: req.SessionID})
	if err != nil {
		return UploadSession{}, err
	}
	if session.Status != UploadSessionStatusPending && session.Status != UploadSessionStatusUploading && session.Status != UploadSessionStatusFailed {
		return UploadSession{}, fmt.Errorf("drive upload session is not writable")
	}
	if !session.ExpiresAt.After(time.Now().UTC()) {
		return UploadSession{}, fmt.Errorf("drive upload session is expired")
	}
	store := s.stores[session.StorageBackend]
	if store == nil {
		return UploadSession{}, fmt.Errorf("storage store %q is required", session.StorageBackend)
	}
	objectID, err := NewUploadID()
	if err != nil {
		return UploadSession{}, err
	}
	storagePath, err := BuildUploadSessionBodyPath(session.UserID, session.ID, objectID)
	if err != nil {
		return UploadSession{}, err
	}

	counter := &countingReader{reader: req.Body}
	limited := &io.LimitedReader{R: counter, N: session.DeclaredSize + 1}
	hash := sha256.New()
	if err := store.Put(ctx, storagePath, io.TeeReader(limited, hash)); err != nil {
		return UploadSession{}, fmt.Errorf("store drive upload session body: %w", err)
	}
	if counter.bytesRead > session.DeclaredSize {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, fmt.Errorf("drive upload session body exceeds declared_size")
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	if req.ExpectedChecksumSHA256 != "" && checksum != req.ExpectedChecksumSHA256 {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, fmt.Errorf("drive upload session checksum mismatch")
	}
	updated, err := s.repo.StoreUploadSessionBody(ctx, RecordUploadSessionBodyRequest{
		UserID:         req.UserID,
		SessionID:      req.SessionID,
		ReceivedSize:   counter.bytesRead,
		StoragePath:    storagePath,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		_ = store.Delete(ctx, storagePath)
		return UploadSession{}, err
	}
	if session.StoragePath != "" && session.StoragePath != storagePath {
		_ = store.Delete(ctx, session.StoragePath)
	}
	return updated, nil
}

func (s *Service) FinalizeUploadSession(ctx context.Context, req FinalizeUploadSessionRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	req, err := ValidateFinalizeUploadSessionRequest(req)
	if err != nil {
		return Node{}, err
	}
	session, err := s.repo.GetUploadSession(ctx, GetUploadSessionRequest{UserID: req.UserID, SessionID: req.SessionID})
	if err != nil {
		return Node{}, err
	}
	storageBackend, err := validateStorageBackend(session.StorageBackend)
	if err != nil {
		return Node{}, err
	}
	store := s.stores[storageBackend]
	if store == nil {
		return Node{}, fmt.Errorf("storage store %q is required", storageBackend)
	}
	return s.repo.FinalizeUploadSession(ctx, store, req)
}

func (s *Service) ListNodes(ctx context.Context, req ListNodesRequest) ([]Node, error) {
	if s == nil || s.repo == nil {
		return nil, fmt.Errorf("drive repository is required")
	}
	return s.repo.ListNodes(ctx, req)
}

func (s *Service) GetNode(ctx context.Context, req GetNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.GetNode(ctx, req)
}

func (s *Service) TrashNode(ctx context.Context, req TrashNodeRequest) (Node, int64, error) {
	if s == nil || s.repo == nil {
		return Node{}, 0, fmt.Errorf("drive repository is required")
	}
	return s.repo.TrashNode(ctx, req)
}

func (s *Service) RestoreNode(ctx context.Context, req RestoreNodeRequest) (Node, int64, error) {
	if s == nil || s.repo == nil {
		return Node{}, 0, fmt.Errorf("drive repository is required")
	}
	return s.repo.RestoreNode(ctx, req)
}

func (s *Service) RenameNode(ctx context.Context, req RenameNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.RenameNode(ctx, req)
}

func (s *Service) MoveNode(ctx context.Context, req MoveNodeRequest) (Node, error) {
	if s == nil || s.repo == nil {
		return Node{}, fmt.Errorf("drive repository is required")
	}
	return s.repo.MoveNode(ctx, req)
}

func (s *Service) PermanentDeleteNode(ctx context.Context, req PermanentDeleteNodeRequest) (PermanentDeleteServiceResult, error) {
	if s == nil || s.repo == nil {
		return PermanentDeleteServiceResult{}, fmt.Errorf("drive repository is required")
	}
	deleted, err := s.repo.PermanentDeleteNode(ctx, req)
	if err != nil {
		return PermanentDeleteServiceResult{}, err
	}
	cleanup, err := CleanupDeletedObjects(ctx, s.stores, deleted.Objects)
	result := PermanentDeleteServiceResult{
		PermanentDelete: deleted,
		Cleanup:         cleanup,
	}
	if err != nil {
		recordErr := s.recordObjectCleanupFailure(ctx, deleted, err)
		if recordErr != nil {
			return result, fmt.Errorf("record drive object cleanup failure after cleanup error %v: %w", err, recordErr)
		}
		return result, fmt.Errorf("cleanup permanently deleted drive objects: %w", err)
	}
	return result, nil
}

func (s *Service) RetryObjectCleanupFailures(ctx context.Context, req ListObjectCleanupFailuresRequest) (RetryObjectCleanupFailuresResult, error) {
	if s == nil || s.cleanupFailureStore == nil {
		return RetryObjectCleanupFailuresResult{}, fmt.Errorf("drive cleanup failure store is required")
	}
	req.Status = ObjectCleanupFailureStatusPending
	failures, err := s.cleanupFailureStore.ListObjectCleanupFailures(ctx, req)
	if err != nil {
		return RetryObjectCleanupFailuresResult{}, err
	}
	result := RetryObjectCleanupFailuresResult{Scanned: len(failures)}
	for _, failure := range failures {
		cleanup, err := CleanupDeletedObjects(ctx, s.stores, []DeletedObject{{
			StorageBackend: failure.StorageBackend,
			StoragePath:    failure.StoragePath,
		}})
		result.Deleted += cleanup.Deleted
		if err != nil {
			result.Failed++
			if recordErr := s.recordObjectCleanupFailure(ctx, PermanentDeleteResult{Root: Node{ID: failure.NodeID, UserID: failure.UserID}}, err); recordErr != nil {
				return result, fmt.Errorf("record drive object cleanup retry failure after cleanup error %v: %w", err, recordErr)
			}
			continue
		}
		if _, err := s.cleanupFailureStore.ResolveObjectCleanupFailure(ctx, ResolveObjectCleanupFailureRequest{ID: failure.ID}); err != nil {
			return result, err
		}
		result.Resolved++
	}
	if result.Failed > 0 {
		return result, fmt.Errorf("retry drive object cleanup: %d failures remain", result.Failed)
	}
	return result, nil
}

func (s *Service) recordObjectCleanupFailure(ctx context.Context, deleted PermanentDeleteResult, cleanupErr error) error {
	if s == nil || s.cleanupFailureRecorder == nil {
		return nil
	}
	var objectErr ObjectCleanupError
	if !errors.As(cleanupErr, &objectErr) {
		return nil
	}
	_, err := s.cleanupFailureRecorder.RecordObjectCleanupFailure(ctx, ObjectCleanupFailure{
		UserID:         deleted.Root.UserID,
		NodeID:         deleted.Root.ID,
		StorageBackend: objectErr.StorageBackend,
		StoragePath:    objectErr.StoragePath,
		LastError:      objectErr.Err.Error(),
	})
	return err
}
