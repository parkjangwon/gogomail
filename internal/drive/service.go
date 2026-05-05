package drive

import (
	"context"
	"errors"
	"fmt"

	"github.com/gogomail/gogomail/internal/storage"
)

type ObjectCleanupFailureRecorder interface {
	RecordObjectCleanupFailure(context.Context, ObjectCleanupFailure) (ObjectCleanupFailure, error)
}

type Service struct {
	repo                   *Repository
	stores                 map[string]storage.Store
	cleanupFailureRecorder ObjectCleanupFailureRecorder
}

type PermanentDeleteServiceResult struct {
	PermanentDelete PermanentDeleteResult
	Cleanup         ObjectCleanupResult
}

func NewService(repo *Repository, stores map[string]storage.Store) *Service {
	copiedStores := make(map[string]storage.Store, len(stores))
	for backend, store := range stores {
		copiedStores[backend] = store
	}
	return &Service{repo: repo, stores: copiedStores}
}

func (s *Service) WithObjectCleanupFailureRecorder(recorder ObjectCleanupFailureRecorder) *Service {
	if s == nil {
		return nil
	}
	s.cleanupFailureRecorder = recorder
	return s
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
