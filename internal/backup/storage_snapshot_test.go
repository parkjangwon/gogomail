package backup

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func seedStore(t *testing.T) (storage.Store, map[string]string) {
	t.Helper()
	store := storage.NewLocalStore(t.TempDir())
	ctx := context.Background()
	objects := map[string]string{
		"tenant-a/user@a.example/Inbox/msg-001.eml": "first message body",
		"tenant-a/user@a.example/Inbox/msg-002.eml": "second message body",
		"tenant-b/user@b.example/Sent/msg-100.eml":  "third message body",
	}
	for key, body := range objects {
		if err := store.Put(ctx, key, strings.NewReader(body)); err != nil {
			t.Fatalf("seed %q: %v", key, err)
		}
	}
	return store, objects
}

func TestBuildStorageSnapshotListsAllObjectsSorted(t *testing.T) {
	t.Parallel()

	store, objects := seedStore(t)
	snap, err := BuildStorageSnapshot(context.Background(), store, SnapshotOptions{Backend: "local"})
	if err != nil {
		t.Fatalf("BuildStorageSnapshot: %v", err)
	}
	if snap.ObjectCount != len(objects) {
		t.Fatalf("ObjectCount = %d, want %d", snap.ObjectCount, len(objects))
	}
	if snap.Hashed {
		t.Fatal("Hashed should be false when Hash is not requested")
	}
	// Verify ascending key order.
	for i := 1; i < len(snap.Objects); i++ {
		if snap.Objects[i-1].Key > snap.Objects[i].Key {
			t.Fatalf("objects not sorted: %q > %q", snap.Objects[i-1].Key, snap.Objects[i].Key)
		}
	}
	var total int64
	for _, o := range snap.Objects {
		total += o.Size
		if o.SHA256 != "" {
			t.Fatalf("unexpected hash for %q when Hash=false", o.Key)
		}
	}
	if total != snap.TotalBytes {
		t.Fatalf("TotalBytes = %d, want %d", snap.TotalBytes, total)
	}
}

func TestBuildStorageSnapshotComputesHashesWhenRequested(t *testing.T) {
	t.Parallel()

	store, objects := seedStore(t)
	snap, err := BuildStorageSnapshot(context.Background(), store, SnapshotOptions{Backend: "local", Hash: true})
	if err != nil {
		t.Fatalf("BuildStorageSnapshot: %v", err)
	}
	if !snap.Hashed {
		t.Fatal("Hashed should be true when Hash requested")
	}
	for _, o := range snap.Objects {
		body, ok := objects[o.Key]
		if !ok {
			t.Fatalf("unexpected key %q", o.Key)
		}
		sum := sha256.Sum256([]byte(body))
		if o.SHA256 != hex.EncodeToString(sum[:]) {
			t.Fatalf("hash mismatch for %q: got %s", o.Key, o.SHA256)
		}
	}
}

func TestBuildStorageSnapshotHonorsPrefix(t *testing.T) {
	t.Parallel()

	store, _ := seedStore(t)
	snap, err := BuildStorageSnapshot(context.Background(), store, SnapshotOptions{Backend: "local", Prefix: "tenant-b/"})
	if err != nil {
		t.Fatalf("BuildStorageSnapshot: %v", err)
	}
	if snap.ObjectCount != 1 {
		t.Fatalf("ObjectCount = %d, want 1 for tenant-b prefix", snap.ObjectCount)
	}
	if !strings.HasPrefix(snap.Objects[0].Key, "tenant-b/") {
		t.Fatalf("object %q not under tenant-b/", snap.Objects[0].Key)
	}
}

func TestBuildStorageSnapshotEnforcesMaxObjects(t *testing.T) {
	t.Parallel()

	store, _ := seedStore(t)
	_, err := BuildStorageSnapshot(context.Background(), store, SnapshotOptions{Backend: "local", MaxObjects: 1})
	if err == nil || !strings.Contains(err.Error(), "MaxObjects") {
		t.Fatalf("error = %v, want MaxObjects exceeded", err)
	}
}

func TestBuildStorageSnapshotRejectsNilStore(t *testing.T) {
	t.Parallel()

	if _, err := BuildStorageSnapshot(context.Background(), nil, SnapshotOptions{}); err == nil {
		t.Fatal("expected error for nil store")
	}
}
