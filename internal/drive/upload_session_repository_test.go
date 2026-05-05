package drive

import (
	"context"
	"strings"
	"testing"
)

func TestCreateUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CreateUploadSession(context.Background(), CreateUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CreateUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceCreateUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).CreateUploadSession(context.Background(), CreateUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("CreateUploadSession err = %v, want repository rejection", err)
	}
}
