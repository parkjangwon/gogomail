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

func TestStoreUploadSessionBodyRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.StoreUploadSessionBody(context.Background(), RecordUploadSessionBodyRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("StoreUploadSessionBody err = %v, want database handle rejection first", err)
	}
}

func TestServiceStoreUploadSessionBodyRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).StoreUploadSessionBody(context.Background(), StoreUploadSessionBodyRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("StoreUploadSessionBody err = %v, want repository rejection", err)
	}
}

func TestFinalizeUploadSessionRequiresDatabase(t *testing.T) {
	t.Parallel()

	repo := NewRepository(nil)
	_, err := repo.FinalizeUploadSession(context.Background(), fakeStore{}, FinalizeUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "database handle is required") {
		t.Fatalf("FinalizeUploadSession err = %v, want database handle rejection first", err)
	}
}

func TestServiceFinalizeUploadSessionRequiresRepository(t *testing.T) {
	t.Parallel()

	_, err := (*Service)(nil).FinalizeUploadSession(context.Background(), FinalizeUploadSessionRequest{})
	if err == nil || !strings.Contains(err.Error(), "drive repository is required") {
		t.Fatalf("FinalizeUploadSession err = %v, want repository rejection", err)
	}
}
