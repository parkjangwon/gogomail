package maildb

import (
	"context"
	"testing"
	"time"
)

func TestPruneExpiredAttachmentShareLinksRejectsNilDB(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredAttachmentShareLinks(context.Background(), time.Now())
	if err == nil {
		t.Fatal("PruneExpiredAttachmentShareLinks with nil db should return error")
	}
}

func TestPruneExpiredAttachmentShareLinksRejectsZeroCutoff(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredAttachmentShareLinks(context.Background(), time.Time{})
	if err == nil {
		t.Fatal("PruneExpiredAttachmentShareLinks with zero cutoff should return error")
	}
}

func TestPruneExpiredDriveShareLinksRejectsNilDB(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredDriveShareLinks(context.Background(), time.Now())
	if err == nil {
		t.Fatal("PruneExpiredDriveShareLinks with nil db should return error")
	}
}

func TestPruneExpiredDriveShareLinksRejectsZeroCutoff(t *testing.T) {
	t.Parallel()

	r := &Repository{db: nil}
	_, err := r.PruneExpiredDriveShareLinks(context.Background(), time.Time{})
	if err == nil {
		t.Fatal("PruneExpiredDriveShareLinks with zero cutoff should return error")
	}
}
