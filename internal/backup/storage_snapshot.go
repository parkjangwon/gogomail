package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/gogomail/gogomail/internal/storage"
)

// StorageObject records a single object in the storage backend at backup time:
// its key, size, and (optionally) a content hash. The hash lets restore or a
// later audit verify that object bytes were not silently altered.
type StorageObject struct {
	Key    string `json:"key"`
	Size   int64  `json:"size"`
	SHA256 string `json:"sha256,omitempty"`
	// ETag is the backend-reported entity tag when available (S3). For local
	// backends it is empty.
	ETag string `json:"etag,omitempty"`
}

// StorageSnapshot is a manifest of the object-store contents at backup time.
// It is a listing, not a copy: object bytes for S3 backends are expected to be
// protected by bucket versioning / replication / lifecycle policy (see
// docs/BACKUP_RESTORE.md). For local/NFS backends the operator archives the
// MailstoreRoot directory alongside the bundle.
type StorageSnapshot struct {
	Backend     string          `json:"backend"`
	Prefix      string          `json:"prefix,omitempty"`
	Bucket      string          `json:"bucket,omitempty"`
	ObjectCount int             `json:"object_count"`
	TotalBytes  int64           `json:"total_bytes"`
	Hashed      bool            `json:"hashed"`
	Objects     []StorageObject `json:"objects"`
}

// SnapshotOptions controls how a storage snapshot is built.
type SnapshotOptions struct {
	// Prefix restricts the listing to keys under this prefix. Empty lists all.
	Prefix string
	// Hash, when true, streams each object to compute a SHA-256 content hash.
	// This is exact but O(total bytes); leave false for large S3 buckets where
	// the ETag and versioning suffice.
	Hash bool
	// MaxObjects caps the number of objects captured (0 = unlimited). Guards
	// against unbounded manifests on very large stores.
	MaxObjects int
	// PageSize is the List page size; 0 uses the store default.
	PageSize int
	// Backend / Bucket are recorded verbatim into the snapshot header.
	Backend string
	Bucket  string
}

// BuildStorageSnapshot walks the object store and produces a [StorageSnapshot].
// It uses the paginated List API so it never loads the whole keyspace into a
// single request, and it optionally hashes object content when opts.Hash is set.
func BuildStorageSnapshot(ctx context.Context, store storage.Store, opts SnapshotOptions) (StorageSnapshot, error) {
	if store == nil {
		return StorageSnapshot{}, fmt.Errorf("storage store is required")
	}
	snap := StorageSnapshot{
		Backend: normalizeBackend(opts.Backend),
		Prefix:  opts.Prefix,
		Bucket:  opts.Bucket,
		Hashed:  opts.Hash,
	}

	cursor := ""
	for {
		if err := ctx.Err(); err != nil {
			return StorageSnapshot{}, err
		}
		page, err := store.List(ctx, storage.ListOptions{
			Prefix: opts.Prefix,
			Limit:  opts.PageSize,
			Cursor: cursor,
		})
		if err != nil {
			return StorageSnapshot{}, fmt.Errorf("list storage objects: %w", err)
		}
		for _, obj := range page.Objects {
			entry := StorageObject{
				Key:  obj.Path,
				Size: obj.Size,
				ETag: strings.Trim(obj.ETag, "\""),
			}
			if opts.Hash {
				sum, err := hashObject(ctx, store, obj.Path)
				if err != nil {
					return StorageSnapshot{}, fmt.Errorf("hash storage object %q: %w", obj.Path, err)
				}
				entry.SHA256 = sum
			}
			snap.Objects = append(snap.Objects, entry)
			snap.TotalBytes += obj.Size
			if opts.MaxObjects > 0 && len(snap.Objects) >= opts.MaxObjects {
				return StorageSnapshot{}, fmt.Errorf("storage object count exceeded MaxObjects=%d; increase the limit or narrow the prefix", opts.MaxObjects)
			}
		}
		if !page.HasMore || page.NextCursor == "" {
			break
		}
		cursor = page.NextCursor
	}

	// Deterministic ordering makes the manifest reproducible and diff-friendly.
	sort.Slice(snap.Objects, func(i, j int) bool {
		return snap.Objects[i].Key < snap.Objects[j].Key
	})
	snap.ObjectCount = len(snap.Objects)
	return snap, nil
}

func hashObject(ctx context.Context, store storage.Store, key string) (string, error) {
	rc, err := store.Get(ctx, key)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	h := sha256.New()
	if _, err := io.Copy(h, rc); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func normalizeBackend(value string) string {
	backend := strings.ToLower(strings.TrimSpace(value))
	if backend == "" {
		return "local"
	}
	return backend
}
