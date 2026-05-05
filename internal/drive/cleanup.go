package drive

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/storage"
)

type ObjectCleanupResult struct {
	Deleted int
}

func CleanupDeletedObjects(ctx context.Context, stores map[string]storage.Store, objects []DeletedObject) (ObjectCleanupResult, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if len(objects) == 0 {
		return ObjectCleanupResult{}, nil
	}
	if len(stores) == 0 {
		return ObjectCleanupResult{}, fmt.Errorf("storage stores are required")
	}

	seen := make(map[DeletedObject]struct{}, len(objects))
	result := ObjectCleanupResult{}
	for _, object := range objects {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		storageBackend, err := validateStorageBackend(object.StorageBackend)
		if err != nil {
			return result, fmt.Errorf("cleanup storage backend %q: %w", object.StorageBackend, err)
		}
		storagePath, err := storage.ValidateObjectPath(object.StoragePath)
		if err != nil {
			return result, fmt.Errorf("cleanup storage path %q: %w", object.StoragePath, err)
		}
		object = DeletedObject{StorageBackend: storageBackend, StoragePath: storagePath}
		if _, ok := seen[object]; ok {
			continue
		}
		seen[object] = struct{}{}

		store := stores[storageBackend]
		if store == nil {
			return result, fmt.Errorf("storage store %q is required", storageBackend)
		}
		if err := store.Delete(ctx, storagePath); err != nil {
			return result, fmt.Errorf("cleanup drive object %q/%q: %w", storageBackend, storagePath, err)
		}
		result.Deleted++
	}
	return result, nil
}
