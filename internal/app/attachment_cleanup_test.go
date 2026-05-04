package app

import (
	"context"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

type fakeAttachmentCleanupRunner struct {
	lastBefore        time.Time
	lastLimit         int
	lastSessionBefore time.Time
	lastSessionLimit  int
	expired           []maildb.Attachment
	expiredSessions   []maildb.AttachmentUploadSession
	err               error
	sessionErr        error
}

func (f *fakeAttachmentCleanupRunner) ExpireStaleAttachmentUploads(_ context.Context, before time.Time, limit int) ([]maildb.Attachment, error) {
	f.lastBefore = before
	f.lastLimit = limit
	if f.err != nil {
		return nil, f.err
	}
	return f.expired, nil
}

func (f *fakeAttachmentCleanupRunner) ExpireAttachmentUploadSessions(_ context.Context, before time.Time, limit int) ([]maildb.AttachmentUploadSession, error) {
	f.lastSessionBefore = before
	f.lastSessionLimit = limit
	if f.sessionErr != nil {
		return nil, f.sessionErr
	}
	return f.expiredSessions, nil
}

func TestCleanupStaleAttachmentUploadsOnceUsesCutoffAndBatchSize(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.FixedZone("KST", 9*60*60))
	runner := &fakeAttachmentCleanupRunner{
		expired:         []maildb.Attachment{{ID: "att-1"}, {ID: "att-2"}},
		expiredSessions: []maildb.AttachmentUploadSession{{ID: "session-1"}},
	}

	result, err := cleanupStaleAttachmentUploadsOnce(context.Background(), runner, func() time.Time {
		return now
	}, 48*time.Hour, 25, nil)
	if err != nil {
		t.Fatalf("cleanupStaleAttachmentUploadsOnce returned error: %v", err)
	}
	if result.ExpiredUploads != 2 {
		t.Fatalf("ExpiredUploads = %d, want 2", result.ExpiredUploads)
	}
	if result.ExpiredSessions != 1 {
		t.Fatalf("ExpiredSessions = %d, want 1", result.ExpiredSessions)
	}
	if runner.lastLimit != 25 {
		t.Fatalf("lastLimit = %d, want 25", runner.lastLimit)
	}
	if runner.lastSessionLimit != 25 {
		t.Fatalf("lastSessionLimit = %d, want 25", runner.lastSessionLimit)
	}
	wantBefore := now.UTC().Add(-48 * time.Hour)
	if !runner.lastBefore.Equal(wantBefore) {
		t.Fatalf("lastBefore = %s, want %s", runner.lastBefore, wantBefore)
	}
	if !runner.lastSessionBefore.Equal(wantBefore) {
		t.Fatalf("lastSessionBefore = %s, want %s", runner.lastSessionBefore, wantBefore)
	}
}
