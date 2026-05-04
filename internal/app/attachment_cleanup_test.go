package app

import (
	"context"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

type fakeAttachmentCleanupRunner struct {
	lastBefore time.Time
	lastLimit  int
	expired    []maildb.Attachment
	err        error
}

func (f *fakeAttachmentCleanupRunner) ExpireStaleAttachmentUploads(_ context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	f.lastBefore = before
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.expired, nil
}

func TestCleanupStaleAttachmentUploadsOnceUsesCutoffAndBatchSize(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	runner := &fakeAttachmentCleanupRunner{
		expired: []maildb.Attachment{{ID: "att-1"}, {ID: "att-2"}},
	}

	count, err := cleanupStaleAttachmentUploadsOnce(context.Background(), runner, func() time.Time {
		return now
	}, 48*time.Hour, 25, nil)
	if err != nil {
		t.Fatalf("cleanupStaleAttachmentUploadsOnce returned error: %v", err)
	}
	if count != 2 {
		t.Fatalf("count = %d, want 2", count)
	}
	if runner.lastLimit != 25 {
		t.Fatalf("lastLimit = %d, want 25", runner.lastLimit)
	}
	wantBefore := now.UTC().Add(-48 * time.Hour)
	if !runner.lastBefore.Equal(wantBefore) {
		t.Fatalf("lastBefore = %s, want %s", runner.lastBefore, wantBefore)
	}
}
