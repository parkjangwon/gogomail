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

func TestGetUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.GetUploadSession(context.Background(), GetUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("GetUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceGetUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).GetUploadSession(context.Background(), GetUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("GetUploadSession err = %v, want repository rejection", err)
	}
}

func TestCancelUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.CancelUploadSession(context.Background(), CancelUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("CancelUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceCancelUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).CancelUploadSession(context.Background(), CancelUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("CancelUploadSession err = %v, want repository rejection", err)
	}
}
