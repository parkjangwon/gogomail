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
	Pending        []DeletedObject
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

	for _, object := range objects {
		storageBackend, err := validateStorageBackend(object.StorageBackend)
		if err != nil {
			return ObjectCleanupResult{}, ObjectCleanupError{StorageBackend: object.StorageBackend, StoragePath: object.StoragePath, Err: err}
		}
		storagePath, err := storage.ValidateObjectPath(object.StoragePath)
		if err != nil {
			return ObjectCleanupResult{}, ObjectCleanupError{StorageBackend: storageBackend, StoragePath: object.StoragePath, Err: err}
		}
		if stores[storageBackend] == nil {
			return ObjectCleanupResult{}, ObjectCleanupError{StorageBackend: storageBackend, StoragePath: storagePath, Err: fmt.Errorf("storage store %q is required", storageBackend)}
		}
	}

	seen := make(map[DeletedObject]struct{}, len(objects))
	pending := make([]DeletedObject, 0, len(objects))
	for _, object := range objects {
		storageBackend, _ := validateStorageBackend(object.StorageBackend)
		storagePath, _ := storage.ValidateObjectPath(object.StoragePath)
		object = DeletedObject{StorageBackend: storageBackend, StoragePath: storagePath}
		if _, ok := seen[object]; ok {
			continue
		}
		seen[object] = struct{}{}
		pending = append(pending, object)
	}

	result := ObjectCleanupResult{}
	for i, object := range pending {
		if err := ctx.Err(); err != nil {
			return result, ObjectCleanupError{StorageBackend: object.StorageBackend, StoragePath: object.StoragePath, Deleted: result.Deleted, Pending: pending[i:], Err: err}
		}
		if err := stores[object.StorageBackend].Delete(ctx, object.StoragePath); err != nil {
			return result, ObjectCleanupError{StorageBackend: object.StorageBackend, StoragePath: object.StoragePath, Deleted: result.Deleted, Pending: pending[i:], Err: err}
		}
		result.Deleted++
	}
	return result, nil
}
