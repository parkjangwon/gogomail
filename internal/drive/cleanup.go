package drive

import (
	"context"
	"fmt"

	"github.com/gogomail/gogomail/internal/storage"
)

type ObjectCleanupResult struct {
	Deleted int `json:"deleted"`
}

type ObjectCleanupError struct {
	StorageBackend string
	StoragePath    string
	Deleted        int
	Err            error
}

func (e ObjectCleanupError) Error() string {
	if e.StorageBackend == "" && e.StoragePath == "" {
		return e.Err.Error()
	}
	return fmt.Sprintf("cleanup drive object %q/%q after %d deletes: %v", e.StorageBackend, e.StoragePath, e.Deleted, e.Err)
}

func (e ObjectCleanupError) Unwrap() error {
	return e.Err
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
			return result, ObjectCleanupError{StorageBackend: object.StorageBackend, StoragePath: object.StoragePath, Deleted: result.Deleted, Err: err}
		}
		storagePath, err := storage.ValidateObjectPath(object.StoragePath)
		if err != nil {
			return result, ObjectCleanupError{StorageBackend: storageBackend, StoragePath: object.StoragePath, Deleted: result.Deleted, Err: err}
		}
		object = DeletedObject{StorageBackend: storageBackend, StoragePath: storagePath}
		if _, ok := seen[object]; ok {
			continue
		}
		seen[object] = struct{}{}

		store := stores[storageBackend]
		if store == nil {
			return result, ObjectCleanupError{StorageBackend: storageBackend, StoragePath: storagePath, Deleted: result.Deleted, Err: fmt.Errorf("storage store %q is required", storageBackend)}
		}
		if err := store.Delete(ctx, storagePath); err != nil {
			return result, ObjectCleanupError{StorageBackend: storageBackend, StoragePath: storagePath, Deleted: result.Deleted, Err: err}
		}
		result.Deleted++
	}
	return result, nil
}
