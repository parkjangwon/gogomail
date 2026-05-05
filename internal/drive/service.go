package drive

import (
	"context"
	"errors"
	"fmt"
	"strings"

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
