package drive

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/storage"
)

func TestNewServiceCopiesStoreMap(t *testing.T) {
	t.Parallel()

	original := map[string]storage.Store{"s3": &recordingStore{}}
	service := NewService(nil, original)
	original["s3"] = nil

	if service.stores["s3"] == nil {
		t.Fatal("NewService store map was mutated through caller map")
	}
}

func TestPermanentDeleteNodeRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).PermanentDeleteNode(context.Background(), PermanentDeleteNodeRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("PermanentDeleteNode err = %v, want repository rejection", err)
	}

	service := NewService(nil, map[string]storage.Store{"s3": &recordingStore{}})
	_, err = service.PermanentDeleteNode(context.Background(), PermanentDeleteNodeRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("PermanentDeleteNode err = %v, want repository rejection", err)
	}
}
