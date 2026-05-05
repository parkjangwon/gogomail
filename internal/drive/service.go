package drive

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/storage"
)

type Service struct {
	repo   *Repository
	stores map[string]storage.Store
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
		return result, fmt.Errorf("cleanup permanently deleted drive objects: %w", err)
	}
	return result, nil
}
