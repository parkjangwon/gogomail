package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/imapgw"
	messageparse "github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/outbound"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresMigrationsApplyWithReleaseSchema(t *testing.T) {
	t.Parallel()

	db := openMigratedPostgresTestDB(t)
	migrationDir := filepath.Join("..", "..", "migrations")
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM goose_db_version`).Scan(&count); err != nil {
		t.Fatalf("query goose version table: %v", err)
	}
	if count == 0 {
		t.Fatal("goose_db_version is empty after migrations")
	}
	current, expected, err := database.MigrationVersionReady(context.Background(), db, migrationDir)
	if err != nil {
		t.Fatalf("MigrationVersionReady returned error: %v", err)
	}
	if current != expected {
		t.Fatalf("migration version = %d/%d, want current at expected", current, expected)
	}
}

func TestPostgresDraftToSendMovesAttachmentsAndQueuesOutbox(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	draft, err := repo.SaveDraft(ctx, SaveDraftRequest{
		UserID:   seed.userID,
		Intent:   "new",
		From:     "alice@example.com",
		To:       []outbound.Address{{Email: "bob@example.net", Name: "Bob"}},
		Subject:  "release postgres draft",
		TextBody: "hello from postgres",
	})
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}

	attachment, err := repo.CreateAttachmentUpload(ctx, CreateAttachmentUploadRequest{
		UserID:      seed.userID,
		DraftID:     draft.ID,
		Filename:    "release.txt",
		Size:        12,
		MIMEType:    "text/plain",
		StoragePath: "uploads/release.txt",
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUpload returned error: %v", err)
	}
	draftForSend, err := repo.GetDraftForSend(ctx, seed.userID, draft.ID)
	if err != nil {
		t.Fatalf("GetDraftForSend returned error: %v", err)
	}
	if len(draftForSend.AttachmentIDs) != 1 || draftForSend.AttachmentIDs[0] != attachment.ID {
		t.Fatalf("draft attachment IDs = %+v, want [%s]", draftForSend.AttachmentIDs, attachment.ID)
	}

	sentID, err := repo.RecordOutgoing(ctx, OutgoingMessage{
		CompanyID:       seed.companyID,
		DomainID:        seed.domainID,
		UserID:          seed.userID,
		ComposeIntent:   draftForSend.Intent,
		SourceMessageID: draftForSend.SourceMessageID,
		RFCMessageID:    "<release-postgres@example.com>",
		Subject:         draft.Subject,
		From:            outbound.Address{Email: "alice@example.com", Name: "Alice"},
		To:              draftForSend.To,
		SentAt:          time.Now().UTC(),
		Size:            128,
		StoragePath:     "sent/release-postgres.eml",
		Farm:            outbound.FarmGeneral,
		DSN: smtpd.DSNOptions{
			Return:     "HDRS",
			EnvelopeID: "release-env",
			Recipients: []smtpd.DSNRecipientOptions{{
				Address: "bob@example.net",
				Notify:  []string{"FAILURE", "DELAY"},
			}},
		},
	})
	if err != nil {
		t.Fatalf("RecordOutgoing returned error: %v", err)
	}

	if err := repo.MarkDraftSent(ctx, seed.userID, draft.ID, sentID); err != nil {
		t.Fatalf("MarkDraftSent returned error: %v", err)
	}

	var draftStatus, movedMessageID, attachmentStatus string
	if err := db.QueryRowContext(ctx, `
SELECT m.status, a.message_id::text, a.status
FROM messages m
JOIN attachments a ON a.id = $2
WHERE m.id = $1`, draft.ID, attachment.ID).Scan(&draftStatus, &movedMessageID, &attachmentStatus); err != nil {
		t.Fatalf("query draft/attachment state: %v", err)
	}
	if draftStatus != "deleted" {
		t.Fatalf("draft status = %q, want deleted", draftStatus)
	}
	if movedMessageID != sentID || attachmentStatus != "stored" {
		t.Fatalf("attachment message/status = %q/%q, want %q/stored", movedMessageID, attachmentStatus, sentID)
	}

	var hasAttachment bool
	var queuedTopic, queuedEvent, queuedEnvelopeID string
	if err := db.QueryRowContext(ctx, `
SELECT m.has_attachment, o.topic, o.payload->>'event', o.payload->'dsn'->>'envelope_id'
FROM messages m
JOIN outbox o ON o.partition_key = m.id::text
WHERE m.id = $1`, sentID).Scan(&hasAttachment, &queuedTopic, &queuedEvent, &queuedEnvelopeID); err != nil {
		t.Fatalf("query sent/outbox state: %v", err)
	}
	if !hasAttachment {
		t.Fatal("sent message has_attachment = false, want true after draft attachment handoff")
	}
	if queuedTopic != "mail.outbound.general" || queuedEvent != "mail.queued" || queuedEnvelopeID != "release-env" {
		t.Fatalf("outbox topic/event/envid = %q/%q/%q", queuedTopic, queuedEvent, queuedEnvelopeID)
	}
}

func TestPostgresCanceledDraftAttachmentCannotBeRebound(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	draft, err := repo.SaveDraft(ctx, SaveDraftRequest{
		UserID:   seed.userID,
		Intent:   "new",
		From:     "alice@example.com",
		Subject:  "canceled attachment",
		TextBody: "hello",
	})
	if err != nil {
		t.Fatalf("SaveDraft returned error: %v", err)
	}
	attachment, err := repo.CreateAttachmentUpload(ctx, CreateAttachmentUploadRequest{
		UserID:      seed.userID,
		DraftID:     draft.ID,
		Filename:    "cancel.txt",
		Size:        12,
		MIMEType:    "text/plain",
		StoragePath: "uploads/cancel.txt",
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUpload returned error: %v", err)
	}
	if _, err := repo.SaveDraft(ctx, SaveDraftRequest{
		UserID:        seed.userID,
		DraftID:       draft.ID,
		Intent:        "new",
		From:          "alice@example.com",
		Subject:       "canceled attachment",
		TextBody:      "hello with attachment",
		AttachmentIDs: []string{attachment.ID},
	}); err != nil {
		t.Fatalf("SaveDraft with attachment returned error: %v", err)
	}
	if _, err := repo.CancelAttachmentUpload(ctx, seed.userID, attachment.ID); err != nil {
		t.Fatalf("CancelAttachmentUpload returned error: %v", err)
	}
	var hasAttachment bool
	var canceledDraftID sql.NullString
	if err := db.QueryRowContext(ctx, `
SELECT m.has_attachment, a.draft_id::text
FROM messages m
JOIN attachments a ON a.id = $2
WHERE m.id = $1`, draft.ID, attachment.ID).Scan(&hasAttachment, &canceledDraftID); err != nil {
		t.Fatalf("query draft has_attachment: %v", err)
	}
	if hasAttachment {
		t.Fatal("draft has_attachment = true after canceling its only upload")
	}
	if canceledDraftID.Valid {
		t.Fatalf("canceled attachment draft_id = %q, want NULL", canceledDraftID.String)
	}

	if _, err := repo.SaveDraft(ctx, SaveDraftRequest{
		UserID:        seed.userID,
		DraftID:       draft.ID,
		Intent:        "new",
		From:          "alice@example.com",
		Subject:       "canceled attachment",
		TextBody:      "hello again",
		AttachmentIDs: []string{attachment.ID},
	}); err == nil {
		t.Fatal("SaveDraft rebound a canceled attachment")
	}
	draftForSend, err := repo.GetDraftForSend(ctx, seed.userID, draft.ID)
	if err != nil {
		t.Fatalf("GetDraftForSend returned error: %v", err)
	}
	if len(draftForSend.AttachmentIDs) != 0 {
		t.Fatalf("draft attachment IDs = %+v, want none", draftForSend.AttachmentIDs)
	}
}

func TestPostgresCreateAttachmentUploadSessionReservesQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var before int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&before); err != nil {
		t.Fatalf("query quota before: %v", err)
	}
	session, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "large.bin",
		DeclaredSize: 512,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	if session.ID == "" || session.UploadID == "" || session.Status != "pending" || session.DeclaredSize != 512 {
		t.Fatalf("session = %+v", session)
	}
	var after int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&after); err != nil {
		t.Fatalf("query quota after: %v", err)
	}
	if after != before+512 {
		t.Fatalf("quota after = %d, want %d", after, before+512)
	}
}

func TestPostgresCancelAttachmentUploadSessionReleasesQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	session, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "large.bin",
		DeclaredSize: 512,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	var reserved int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&reserved); err != nil {
		t.Fatalf("query reserved quota: %v", err)
	}
	canceled, err := repo.CancelAttachmentUploadSession(ctx, CancelAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	})
	if err != nil {
		t.Fatalf("CancelAttachmentUploadSession returned error: %v", err)
	}
	if canceled.Status != "canceled" || canceled.CanceledAt.IsZero() {
		t.Fatalf("canceled session = %+v", canceled)
	}
	var released int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&released); err != nil {
		t.Fatalf("query released quota: %v", err)
	}
	if released != reserved-512 {
		t.Fatalf("released quota = %d, want %d", released, reserved-512)
	}
	if _, err := repo.CancelAttachmentUploadSession(ctx, CancelAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	}); err == nil {
		t.Fatal("CancelAttachmentUploadSession accepted already canceled session")
	}
	var afterSecondCancel int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&afterSecondCancel); err != nil {
		t.Fatalf("query quota after second cancel: %v", err)
	}
	if afterSecondCancel != released {
		t.Fatalf("quota changed after second cancel = %d, want %d", afterSecondCancel, released)
	}
}

func TestPostgresExpireAttachmentUploadSessionsReleasesQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	expiredCandidate, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "old.bin",
		DeclaredSize: 128,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(-time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession old returned error: %v", err)
	}
	freshCandidate, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "fresh.bin",
		DeclaredSize: 256,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession fresh returned error: %v", err)
	}
	var reserved int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&reserved); err != nil {
		t.Fatalf("query reserved quota: %v", err)
	}
	counts, err := repo.CountStaleAttachmentUploadSessions(ctx, ExpireAttachmentUploadSessionsRequest{
		Before: time.Now(),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("CountStaleAttachmentUploadSessions returned error: %v", err)
	}
	if counts.TotalCount != 1 || counts.LimitedCount != 1 {
		t.Fatalf("stale session counts = %+v", counts)
	}
	candidates, err := repo.ListStaleAttachmentUploadSessions(ctx, ExpireAttachmentUploadSessionsRequest{
		Before: time.Now(),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ListStaleAttachmentUploadSessions returned error: %v", err)
	}
	if len(candidates) != 1 || candidates[0].ID != expiredCandidate.ID || candidates[0].Status != "pending" {
		t.Fatalf("stale session candidates = %+v", candidates)
	}

	expired, err := repo.ExpireAttachmentUploadSessions(ctx, ExpireAttachmentUploadSessionsRequest{
		Before: time.Now(),
		Limit:  10,
	})
	if err != nil {
		t.Fatalf("ExpireAttachmentUploadSessions returned error: %v", err)
	}
	if len(expired) != 1 || expired[0].ID != expiredCandidate.ID || expired[0].Status != "expired" {
		t.Fatalf("expired sessions = %+v", expired)
	}
	var released int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&released); err != nil {
		t.Fatalf("query released quota: %v", err)
	}
	if released != reserved-128 {
		t.Fatalf("released quota = %d, want %d", released, reserved-128)
	}
	var freshStatus string
	if err := db.QueryRowContext(ctx, `SELECT status FROM attachment_upload_sessions WHERE id = $1`, freshCandidate.ID).Scan(&freshStatus); err != nil {
		t.Fatalf("query fresh status: %v", err)
	}
	if freshStatus != "pending" {
		t.Fatalf("fresh status = %q, want pending", freshStatus)
	}
}

func TestPostgresFinalizeAttachmentUploadSessionCreatesAttachmentWithoutDoubleQuota(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var before int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&before); err != nil {
		t.Fatalf("query quota before: %v", err)
	}
	session, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "large.bin",
		DeclaredSize: 512,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	if _, err := repo.StoreAttachmentUploadSessionBody(ctx, StoreAttachmentUploadSessionBodyRequest{
		UserID:         seed.userID,
		SessionID:      session.ID,
		ReceivedSize:   512,
		StoragePath:    "upload-sessions/" + seed.userID + "/" + session.ID + "/body",
		ChecksumSHA256: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("StoreAttachmentUploadSessionBody returned error: %v", err)
	}
	attachment, err := repo.FinalizeAttachmentUploadSession(ctx, FinalizeAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	})
	if err != nil {
		t.Fatalf("FinalizeAttachmentUploadSession returned error: %v", err)
	}
	if attachment.ID == "" || attachment.UploadID != session.UploadID || attachment.Size != 512 || attachment.Status != "uploading" {
		t.Fatalf("attachment = %+v session = %+v", attachment, session)
	}
	var after int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&after); err != nil {
		t.Fatalf("query quota after: %v", err)
	}
	if after != before+512 {
		t.Fatalf("quota after finalize = %d, want %d", after, before+512)
	}
	var sessionStatus string
	var finalizedAt sql.NullTime
	if err := db.QueryRowContext(ctx, `SELECT status, finalized_at FROM attachment_upload_sessions WHERE id = $1`, session.ID).Scan(&sessionStatus, &finalizedAt); err != nil {
		t.Fatalf("query finalized session: %v", err)
	}
	if sessionStatus != "finalized" || !finalizedAt.Valid {
		t.Fatalf("session status/finalized_at = %q/%v", sessionStatus, finalizedAt.Valid)
	}
}

func TestPostgresFinalizeAttachmentUploadSessionRejectsDuplicateFinalize(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	session, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "large.bin",
		DeclaredSize: 512,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	if _, err := repo.StoreAttachmentUploadSessionBody(ctx, StoreAttachmentUploadSessionBodyRequest{
		UserID:         seed.userID,
		SessionID:      session.ID,
		ReceivedSize:   512,
		StoragePath:    "upload-sessions/" + seed.userID + "/" + session.ID + "/body",
		ChecksumSHA256: strings.Repeat("a", 64),
	}); err != nil {
		t.Fatalf("StoreAttachmentUploadSessionBody returned error: %v", err)
	}
	if _, err := repo.FinalizeAttachmentUploadSession(ctx, FinalizeAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	}); err != nil {
		t.Fatalf("FinalizeAttachmentUploadSession returned error: %v", err)
	}
	var quotaAfterFirst int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfterFirst); err != nil {
		t.Fatalf("query quota after first finalize: %v", err)
	}
	if _, err := repo.FinalizeAttachmentUploadSession(ctx, FinalizeAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	}); err == nil {
		t.Fatal("FinalizeAttachmentUploadSession accepted duplicate finalize")
	}
	var quotaAfterSecond int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfterSecond); err != nil {
		t.Fatalf("query quota after duplicate finalize: %v", err)
	}
	if quotaAfterSecond != quotaAfterFirst {
		t.Fatalf("quota after duplicate finalize = %d, want %d", quotaAfterSecond, quotaAfterFirst)
	}
	var attachmentCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE upload_id = $1`, session.UploadID).Scan(&attachmentCount); err != nil {
		t.Fatalf("query attachment count: %v", err)
	}
	if attachmentCount != 1 {
		t.Fatalf("attachment count = %d, want 1", attachmentCount)
	}
}

func TestPostgresFinalizeAttachmentUploadSessionRejectsUnstoredBody(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	session, err := repo.CreateAttachmentUploadSession(ctx, CreateAttachmentUploadSessionRequest{
		UserID:       seed.userID,
		Filename:     "large.bin",
		DeclaredSize: 512,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("CreateAttachmentUploadSession returned error: %v", err)
	}
	var quotaAfterCreate int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfterCreate); err != nil {
		t.Fatalf("query quota after create: %v", err)
	}
	if _, err := repo.FinalizeAttachmentUploadSession(ctx, FinalizeAttachmentUploadSessionRequest{
		UserID:    seed.userID,
		SessionID: session.ID,
	}); err == nil {
		t.Fatal("FinalizeAttachmentUploadSession accepted unstored body")
	}
	var quotaAfterFinalize int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfterFinalize); err != nil {
		t.Fatalf("query quota after rejected finalize: %v", err)
	}
	if quotaAfterFinalize != quotaAfterCreate {
		t.Fatalf("quota after rejected finalize = %d, want %d", quotaAfterFinalize, quotaAfterCreate)
	}
	var attachmentCount int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM attachments WHERE upload_id = $1`, session.UploadID).Scan(&attachmentCount); err != nil {
		t.Fatalf("query attachment count: %v", err)
	}
	if attachmentCount != 0 {
		t.Fatalf("attachment count = %d, want 0", attachmentCount)
	}
}

func TestPostgresRunAPIUsageLedgerRetentionRequiresReadinessAndDeletesBoundedRows(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	repo := NewRepository(db)

	now := time.Now().UTC().Truncate(time.Second)
	windowStart := now.Add(-4 * time.Hour)
	cutoff := now.Add(-time.Hour)
	candidateTimes := []time.Time{
		now.Add(-3 * time.Hour),
		now.Add(-2 * time.Hour),
		now.Add(-90 * time.Minute),
	}
	for i, eventAt := range candidateTimes {
		insertPostgresAPIUsageLedgerEvent(t, db, fmt.Sprintf("retention-old-%d", i+1), eventAt, now.Add(-30*time.Minute), "tenant-1", "principal-1")
	}
	insertPostgresAPIUsageLedgerEvent(t, db, "retention-fresh", now.Add(-30*time.Minute), now.Add(-20*time.Minute), "tenant-1", "principal-1")

	blocked, err := repo.RunAPIUsageLedgerRetention(ctx, APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     "tenant-1",
		PrincipalID:  "principal-1",
		Limit:        2,
		DryRun:       false,
		ConfirmReady: true,
	})
	if err != nil {
		t.Fatalf("RunAPIUsageLedgerRetention blocked returned error: %v", err)
	}
	if blocked.Ready || blocked.DeletedCount != 0 || blocked.CandidateCount != 3 || blocked.LimitedCount != 2 {
		t.Fatalf("blocked retention run = %+v", blocked)
	}
	if blocked.ID == "" || blocked.CreatedAt.IsZero() {
		t.Fatalf("blocked retention run audit identity = %+v", blocked)
	}

	insertPostgresAPIUsageExportEvidence(t, db, now, windowStart, cutoff, 3)
	dryRun, err := repo.RunAPIUsageLedgerRetention(ctx, APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     "tenant-1",
		PrincipalID:  "principal-1",
		Limit:        2,
		DryRun:       true,
		ConfirmReady: false,
	})
	if err != nil {
		t.Fatalf("RunAPIUsageLedgerRetention dry-run returned error: %v", err)
	}
	if !dryRun.Ready || dryRun.DeletedCount != 0 || dryRun.CandidateCount != 3 || dryRun.LimitedCount != 2 {
		t.Fatalf("dry retention run = %+v", dryRun)
	}
	if dryRun.ID == "" || dryRun.CreatedAt.IsZero() {
		t.Fatalf("dry retention run audit identity = %+v", dryRun)
	}

	run, err := repo.RunAPIUsageLedgerRetention(ctx, APIUsageLedgerRetentionRunRequest{
		Cutoff:       cutoff,
		TenantID:     "tenant-1",
		PrincipalID:  "principal-1",
		Limit:        2,
		DryRun:       false,
		ConfirmReady: true,
	})
	if err != nil {
		t.Fatalf("RunAPIUsageLedgerRetention returned error: %v", err)
	}
	if !run.Ready || run.DeletedCount != 2 || run.CandidateCount != 3 || run.LimitedCount != 2 {
		t.Fatalf("retention run = %+v", run)
	}
	if run.ID == "" || run.CreatedAt.IsZero() {
		t.Fatalf("retention run audit identity = %+v", run)
	}

	var oldRemaining, freshRemaining int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM api_usage_ledger WHERE event_timestamp < $1`, cutoff).Scan(&oldRemaining); err != nil {
		t.Fatalf("query old remaining: %v", err)
	}
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM api_usage_ledger WHERE event_id = 'retention-fresh'`).Scan(&freshRemaining); err != nil {
		t.Fatalf("query fresh remaining: %v", err)
	}
	if oldRemaining != 1 || freshRemaining != 1 {
		t.Fatalf("remaining old/fresh = %d/%d, want 1/1", oldRemaining, freshRemaining)
	}

	var auditRows int
	if err := db.QueryRowContext(ctx, `
SELECT count(*)
FROM api_usage_ledger_retention_runs
WHERE tenant_id = 'tenant-1' AND principal_id = 'principal-1'`).Scan(&auditRows); err != nil {
		t.Fatalf("query retention audit row count: %v", err)
	}
	if auditRows != 3 {
		t.Fatalf("retention audit row count = %d, want 3", auditRows)
	}
	var deletedCount int64
	var ready bool
	var dry bool
	var readinessCandidateCount int64
	if err := db.QueryRowContext(ctx, `
SELECT deleted_count, ready, dry_run, (readiness->>'candidate_event_count')::bigint
FROM api_usage_ledger_retention_runs
WHERE id = $1`, run.ID).Scan(&deletedCount, &ready, &dry, &readinessCandidateCount); err != nil {
		t.Fatalf("query destructive retention audit row: %v", err)
	}
	if deletedCount != 2 || !ready || dry || readinessCandidateCount != 3 {
		t.Fatalf("destructive retention audit row = deleted:%d ready:%v dry:%v candidates:%d", deletedCount, ready, dry, readinessCandidateCount)
	}

	runs, err := repo.ListAPIUsageLedgerRetentionRuns(ctx, APIUsageLedgerRetentionRunListRequest{
		TenantID:    "tenant-1",
		PrincipalID: "principal-1",
		CreatedFrom: now.Add(-time.Hour),
		CreatedTo:   now.Add(time.Hour),
		Limit:       10,
	})
	if err != nil {
		t.Fatalf("ListAPIUsageLedgerRetentionRuns returned error: %v", err)
	}
	if len(runs) != 3 || runs[0].ID == "" || runs[0].Readiness.CandidateEventCount == 0 {
		t.Fatalf("retention audit runs = %+v", runs)
	}
	gotRun, err := repo.GetAPIUsageLedgerRetentionRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetAPIUsageLedgerRetentionRun returned error: %v", err)
	}
	if gotRun.ID != run.ID || gotRun.DeletedCount != 2 || gotRun.Readiness.CandidateEventCount != 3 {
		t.Fatalf("retention audit detail = %+v", gotRun)
	}
}

func TestPostgresQuotaCorrectionRecordsAudit(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	if _, err := db.ExecContext(ctx, `UPDATE users SET quota_used = 123 WHERE id = $1`, seed.userID); err != nil {
		t.Fatalf("seed quota drift: %v", err)
	}
	dryRun, err := repo.CorrectQuotaReconciliation(ctx, CorrectQuotaReconciliationRequest{
		Scope:  "user",
		ID:     seed.userID,
		DryRun: true,
	})
	if err != nil {
		t.Fatalf("CorrectQuotaReconciliation dry-run returned error: %v", err)
	}
	if !dryRun.DryRun || len(dryRun.Corrected) != 1 {
		t.Fatalf("dry quota correction = %+v", dryRun)
	}
	applied, err := repo.CorrectQuotaReconciliation(ctx, CorrectQuotaReconciliationRequest{
		Scope: "user",
		ID:    seed.userID,
	})
	if err != nil {
		t.Fatalf("CorrectQuotaReconciliation returned error: %v", err)
	}
	if applied.DryRun || len(applied.Corrected) != 0 {
		t.Fatalf("applied quota correction = %+v", applied)
	}

	var auditRows int
	var beforeCount int
	var afterCount int
	var hashedRows int
	if err := db.QueryRowContext(ctx, `
SELECT count(*)::int,
  max((detail->>'before_drift_count')::int),
  min((detail->>'after_drift_count')::int),
  count(*) FILTER (WHERE hash <> '')::int
FROM audit_logs
WHERE action = 'quota.reconciliation_correction'
  AND target_type = 'user'
  AND target_id = $1`, seed.userID).Scan(&auditRows, &beforeCount, &afterCount, &hashedRows); err != nil {
		t.Fatalf("query quota correction audit: %v", err)
	}
	if auditRows != 2 || beforeCount != 1 || afterCount != 0 || hashedRows != 2 {
		t.Fatalf("quota correction audit rows/counts/hash = %d/%d/%d/%d, want 2/1/0/2", auditRows, beforeCount, afterCount, hashedRows)
	}
}

func TestPostgresAuditLogReads(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	var keptID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO audit_logs (
  company_id, domain_id, user_id, category, action, target_type, target_id,
  result, detail, prev_hash, hash, created_at
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, 'admin', 'quota.reconciliation_correction',
  'user', $3::uuid, 'applied', '{"before_drift_count":1}'::jsonb, 'prev-a', 'hash-a', $4
) RETURNING id::text`, seed.companyID, seed.domainID, seed.userID, now).Scan(&keptID); err != nil {
		t.Fatalf("insert kept audit log: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO audit_logs (
  company_id, domain_id, category, action, target_type, result, detail, prev_hash, hash, created_at
) VALUES (
  $1::uuid, $2::uuid, 'smtp', 'message.stored', 'message', 'ok', '{"stored":true}'::jsonb, 'prev-b', 'hash-b', $3
)`, seed.companyID, seed.domainID, now.Add(-time.Hour)); err != nil {
		t.Fatalf("insert filtered audit log: %v", err)
	}

	logs, err := repo.ListAuditLogs(ctx, AuditLogListRequest{
		Limit:      10,
		Category:   "admin",
		Action:     "quota.reconciliation_correction",
		Result:     "applied",
		TargetType: "user",
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		UserID:     seed.userID,
		Since:      now.Add(-time.Minute),
	})
	if err != nil {
		t.Fatalf("ListAuditLogs returned error: %v", err)
	}
	if len(logs) != 1 || logs[0].ID != keptID || !strings.Contains(string(logs[0].Detail), "before_drift_count") {
		t.Fatalf("audit logs = %+v", logs)
	}

	got, err := repo.GetAuditLog(ctx, keptID)
	if err != nil {
		t.Fatalf("GetAuditLog returned error: %v", err)
	}
	if got.ID != keptID || got.CompanyID != seed.companyID || got.Hash != "hash-a" || got.PrevHash != "prev-a" {
		t.Fatalf("audit log detail = %+v", got)
	}
}

func TestPostgresAuditLogIntegrityCheck(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)
	writer := audit.NewPostgresRepository(db)
	now := time.Date(2026, 5, 5, 12, 0, 0, 0, time.UTC)

	if err := writer.Insert(ctx, audit.Log{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		UserID:     seed.userID,
		Category:   "mail",
		Action:     "mail.received",
		TargetType: "message",
		Result:     "success",
		Detail:     json.RawMessage(`{"message_id":"msg-1"}`),
		CreatedAt:  now,
	}); err != nil {
		t.Fatalf("insert first audit log: %v", err)
	}
	if err := writer.Insert(ctx, audit.Log{
		CompanyID:  seed.companyID,
		DomainID:   seed.domainID,
		UserID:     seed.userID,
		Category:   "delivery",
		Action:     "mail.delivered",
		TargetType: "message",
		Result:     "success",
		Detail:     json.RawMessage(`{"message_id":"msg-2"}`),
		CreatedAt:  now.Add(time.Minute),
	}); err != nil {
		t.Fatalf("insert second audit log: %v", err)
	}

	valid, err := repo.CheckAuditLogIntegrity(ctx, AuditLogIntegrityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("CheckAuditLogIntegrity returned error: %v", err)
	}
	if !valid.Valid || valid.CheckedCount != 2 || len(valid.Breaks) != 0 {
		t.Fatalf("valid integrity = %+v", valid)
	}

	if _, err := db.ExecContext(ctx, `UPDATE audit_logs SET hash = 'tampered' WHERE action = 'mail.received'`); err != nil {
		t.Fatalf("tamper audit log: %v", err)
	}
	broken, err := repo.CheckAuditLogIntegrity(ctx, AuditLogIntegrityRequest{Limit: 10})
	if err != nil {
		t.Fatalf("CheckAuditLogIntegrity after tamper returned error: %v", err)
	}
	if broken.Valid || len(broken.Breaks) == 0 {
		t.Fatalf("broken integrity = %+v", broken)
	}
}

func TestPostgresIMAPUIDBackfillAndMoveInvalidation(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var firstID, secondID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id, rfc_message_id, subject,
  from_addr, received_at, size, storage_path
) VALUES
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<first@example.com>', 'first',
   'sender@example.net', '2026-05-04T00:00:00Z'::timestamptz, 100, 'mail/first.eml'),
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<second@example.com>', 'second',
   'sender@example.net', '2026-05-04T00:01:00Z'::timestamptz, 100, 'mail/second.eml')
RETURNING id::text`, seed.companyID, seed.domainID, seed.userID, seed.inboxID).Scan(&firstID); err != nil {
		t.Fatalf("insert first message: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
SELECT id::text
FROM messages
WHERE user_id = $1::uuid
  AND folder_id = $2::uuid
  AND subject = 'second'`, seed.userID, seed.inboxID).Scan(&secondID); err != nil {
		t.Fatalf("select second message: %v", err)
	}

	assigned, err := repo.BackfillIMAPMailboxUIDs(ctx, seed.userID, seed.inboxID, 10)
	if err != nil {
		t.Fatalf("BackfillIMAPMailboxUIDs returned error: %v", err)
	}
	if len(assigned) != 2 || assigned[0].UID != 1 || assigned[1].UID != 2 {
		t.Fatalf("assigned UIDs = %#v, want stable 1,2", assigned)
	}
	existing, err := repo.ExistingIMAPMessageUIDs(ctx, seed.userID, []string{firstID, secondID})
	if err != nil {
		t.Fatalf("ExistingIMAPMessageUIDs returned error: %v", err)
	}
	if len(existing) != 2 || existing[0].UID != 1 || existing[0].SequenceNumber != 1 || existing[1].UID != 2 || existing[1].SequenceNumber != 2 {
		t.Fatalf("existing IMAP UIDs = %#v, want sequence-bearing 1,2", existing)
	}

	if err := repo.MoveMessage(ctx, seed.userID, firstID, seed.sentID); err != nil {
		t.Fatalf("MoveMessage returned error: %v", err)
	}
	if _, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, assigned[0].UID); err == nil {
		t.Fatal("GetIMAPMessage found moved message in old mailbox")
	}
	if _, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, firstID); !errors.Is(err, ErrIMAPMessageNotActive) {
		t.Fatalf("EnsureIMAPMessageUID for stale old mailbox error = %v, want ErrIMAPMessageNotActive", err)
	}
	movedUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.sentID, firstID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID after move returned error: %v", err)
	}
	if string(movedUID.MailboxID) != seed.sentID {
		t.Fatalf("moved UID mailbox = %q, want sent mailbox %q", movedUID.MailboxID, seed.sentID)
	}
	if movedUID.UID != 1 {
		t.Fatalf("moved UID = %d, want fresh UID 1 in sent mailbox", movedUID.UID)
	}

	remaining, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, assigned[1].UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage for remaining inbox UID returned error: %v", err)
	}
	if string(remaining.Summary.ID) != secondID {
		t.Fatalf("remaining inbox message = %q, want %q", remaining.Summary.ID, secondID)
	}
	if remaining.Summary.SequenceNumber != 1 {
		t.Fatalf("remaining inbox sequence number = %d, want 1 after first message moved", remaining.Summary.SequenceNumber)
	}
}

func TestPostgresIMAPMailboxSubscriptionsPersistNames(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	subscription, err := repo.SubscribeIMAPMailbox(ctx, seed.userID, "INBOX")
	if err != nil {
		t.Fatalf("SubscribeIMAPMailbox returned error: %v", err)
	}
	if !subscription.Exists || subscription.Name != "Inbox" {
		t.Fatalf("subscription = %#v, want existing Inbox", subscription)
	}

	listed, err := repo.ListSubscribedIMAPMailboxes(ctx, seed.userID)
	if err != nil {
		t.Fatalf("ListSubscribedIMAPMailboxes returned error: %v", err)
	}
	if len(listed) != 1 || !listed[0].Exists || listed[0].Mailbox.ID != imapgw.MailboxID(seed.inboxID) {
		t.Fatalf("listed subscriptions = %#v, want existing inbox", listed)
	}

	if _, err := db.ExecContext(ctx, `DELETE FROM folders WHERE id = $1::uuid`, seed.inboxID); err != nil {
		t.Fatalf("delete subscribed mailbox: %v", err)
	}
	listed, err = repo.ListSubscribedIMAPMailboxes(ctx, seed.userID)
	if err != nil {
		t.Fatalf("ListSubscribedIMAPMailboxes after delete returned error: %v", err)
	}
	if len(listed) != 1 || listed[0].Exists || listed[0].Name != "Inbox" {
		t.Fatalf("deleted mailbox subscription = %#v, want retained noselect name", listed)
	}

	if err := repo.UnsubscribeIMAPMailbox(ctx, seed.userID, "INBOX"); err != nil {
		t.Fatalf("UnsubscribeIMAPMailbox returned error: %v", err)
	}
	listed, err = repo.ListSubscribedIMAPMailboxes(ctx, seed.userID)
	if err != nil {
		t.Fatalf("ListSubscribedIMAPMailboxes after unsubscribe returned error: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("subscriptions after unsubscribe = %#v, want empty", listed)
	}
}

func TestPostgresIMAPMoveMessagesMovesActiveUIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var firstID, secondID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id, rfc_message_id, subject,
  from_addr, received_at, size, storage_path
) VALUES
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<move-first@example.com>',
   'move first', 'sender@example.net', '2026-05-04T00:00:00Z'::timestamptz,
   100, 'mail/move-first.eml'),
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<move-second@example.com>',
   'move second', 'sender@example.net', '2026-05-04T00:01:00Z'::timestamptz,
   100, 'mail/move-second.eml')
RETURNING id::text`, seed.companyID, seed.domainID, seed.userID, seed.inboxID).Scan(&firstID); err != nil {
		t.Fatalf("insert first move message: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
SELECT id::text
FROM messages
WHERE user_id = $1::uuid
  AND folder_id = $2::uuid
  AND subject = 'move second'`, seed.userID, seed.inboxID).Scan(&secondID); err != nil {
		t.Fatalf("select second move message: %v", err)
	}
	firstUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, firstID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID first returned error: %v", err)
	}
	secondUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, secondID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID second returned error: %v", err)
	}

	moved, err := repo.MoveIMAPMessages(ctx, seed.userID, seed.inboxID, seed.sentID, []imapgw.UID{secondUID.UID})
	if err != nil {
		t.Fatalf("MoveIMAPMessages returned error: %v", err)
	}
	if len(moved) != 1 || moved[0].Source.ID != imapgw.MessageID(secondID) || moved[0].Source.UID != secondUID.UID || moved[0].Source.SequenceNumber != 2 {
		t.Fatalf("moved results = %#v, want second source UID with sequence 2", moved)
	}
	if moved[0].Destination.ID != imapgw.MessageID(secondID) || moved[0].Destination.MailboxID != imapgw.MailboxID(seed.sentID) || moved[0].Destination.UID != 1 || moved[0].Destination.SequenceNumber != 1 {
		t.Fatalf("destination move result = %#v, want sent UID 1 with sequence 1", moved[0].Destination)
	}
	if moved[0].SourceHighestModSeq != secondUID.ModSeq+1 {
		t.Fatalf("source highest modseq = %d, want %d", moved[0].SourceHighestModSeq, secondUID.ModSeq+1)
	}
	if _, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, secondUID.UID); err == nil {
		t.Fatal("GetIMAPMessage found moved message in source mailbox")
	}
	remaining, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, firstUID.UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage remaining source returned error: %v", err)
	}
	if remaining.Summary.SequenceNumber != 1 {
		t.Fatalf("remaining source sequence = %d, want 1", remaining.Summary.SequenceNumber)
	}
	freshUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.sentID, secondID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID destination returned error: %v", err)
	}
	if freshUID.UID != 1 || freshUID.MailboxID != imapgw.MailboxID(seed.sentID) {
		t.Fatalf("destination UID = %#v, want moved sent UID 1", freshUID)
	}
	sourceState, err := repo.EnsureIMAPMailboxState(ctx, seed.userID, seed.inboxID)
	if err != nil {
		t.Fatalf("EnsureIMAPMailboxState source returned error: %v", err)
	}
	if sourceState.HighestModSeq != moved[0].SourceHighestModSeq {
		t.Fatalf("source mailbox highest modseq = %d, want %d", sourceState.HighestModSeq, moved[0].SourceHighestModSeq)
	}
	var folderID string
	if err := db.QueryRowContext(ctx, `SELECT folder_id::text FROM messages WHERE id = $1::uuid`, secondID).Scan(&folderID); err != nil {
		t.Fatalf("read moved folder id: %v", err)
	}
	if folderID != seed.sentID {
		t.Fatalf("moved folder = %q, want %q", folderID, seed.sentID)
	}
}

func TestPostgresIMAPCopyMessagesAssignsFreshUIDsAndCopiesAttachments(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var messageID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id, rfc_message_id, subject,
  from_addr, received_at, size, storage_path
) VALUES (
  $1::uuid, $2::uuid, $3::uuid, $4::uuid, '<copy-source@example.com>',
  'copy source', 'sender@example.net', '2026-05-04T00:00:00Z'::timestamptz,
  100, 'mail/copy-source.eml'
) RETURNING id::text`, seed.companyID, seed.domainID, seed.userID, seed.inboxID).Scan(&messageID); err != nil {
		t.Fatalf("insert source message: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO attachments (
  message_id, user_id, upload_id, storage_path, filename, size, mime_type, status
) VALUES (
  $1::uuid, $2::uuid, 'copy-source-upload', 'attachments/source-report.pdf',
  'source-report.pdf', 12, 'application/pdf', 'stored'
)`, messageID, seed.userID); err != nil {
		t.Fatalf("insert source attachment: %v", err)
	}

	sourceUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, messageID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID returned error: %v", err)
	}
	var quotaBefore int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaBefore); err != nil {
		t.Fatalf("read quota before copy: %v", err)
	}

	copied, err := repo.CopyIMAPMessages(ctx, seed.userID, seed.inboxID, seed.sentID, []imapgw.UID{sourceUID.UID})
	if err != nil {
		t.Fatalf("CopyIMAPMessages returned error: %v", err)
	}
	if len(copied) != 1 {
		t.Fatalf("copied summaries = %#v, want 1", copied)
	}
	if copied[0].ID == imapgw.MessageID(messageID) {
		t.Fatalf("copied message reused source id %q", messageID)
	}
	if copied[0].MailboxID != imapgw.MailboxID(seed.sentID) || copied[0].UID != 1 || copied[0].SequenceNumber != 1 {
		t.Fatalf("copied summary mailbox/uid/seq = %q/%d/%d, want sent/1/1", copied[0].MailboxID, copied[0].UID, copied[0].SequenceNumber)
	}

	sourceStillThere, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, sourceUID.UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage source after copy returned error: %v", err)
	}
	if string(sourceStillThere.Summary.ID) != messageID {
		t.Fatalf("source message after copy = %q, want %q", sourceStillThere.Summary.ID, messageID)
	}
	copiedMessage, err := repo.GetIMAPMessage(ctx, seed.userID, seed.sentID, copied[0].UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage copied destination returned error: %v", err)
	}
	if copiedMessage.Summary.Envelope.Subject != "copy source" {
		t.Fatalf("copied subject = %q, want source metadata", copiedMessage.Summary.Envelope.Subject)
	}

	var copiedAttachmentCount int
	var copiedAttachmentSize int64
	var copiedStoragePath string
	if err := db.QueryRowContext(ctx, `
SELECT COUNT(*), COALESCE(SUM(size), 0), COALESCE(MAX(storage_path), '')
FROM attachments
WHERE message_id = $1::uuid`, string(copied[0].ID)).Scan(&copiedAttachmentCount, &copiedAttachmentSize, &copiedStoragePath); err != nil {
		t.Fatalf("query copied attachment: %v", err)
	}
	if copiedAttachmentCount != 1 || copiedAttachmentSize != 12 || copiedStoragePath != "attachments/source-report.pdf" {
		t.Fatalf("copied attachment count/size/path = %d/%d/%q, want 1/12/source path", copiedAttachmentCount, copiedAttachmentSize, copiedStoragePath)
	}
	var quotaAfter int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfter); err != nil {
		t.Fatalf("read quota after copy: %v", err)
	}
	if quotaAfter-quotaBefore != 112 {
		t.Fatalf("quota delta = %d, want copied message plus attachment bytes 112", quotaAfter-quotaBefore)
	}
}

func TestPostgresIMAPAppendStoresMessageAndUID(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	target, err := repo.ResolveIMAPAppendTarget(ctx, seed.userID, "INBOX")
	if err != nil {
		t.Fatalf("ResolveIMAPAppendTarget returned error: %v", err)
	}
	if target.MailboxID != seed.inboxID || target.UserID != seed.userID || target.DomainID != seed.domainID || target.CompanyID != seed.companyID {
		t.Fatalf("append target = %#v, want seeded inbox/user/domain/company", target)
	}
	if target.UIDValidity == 0 {
		t.Fatal("append target UIDValidity = 0")
	}

	raw := strings.Join([]string{
		"Message-ID: <imap-append@example.com>",
		"Date: Tue, 5 May 2026 12:34:56 +0900",
		"From: Sender <sender@example.net>",
		"To: Alice <alice@example.com>",
		"Subject: IMAP append integration",
		"",
		"hello from append",
	}, "\r\n")
	parsed, err := messageparse.ParseEML(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseEML returned error: %v", err)
	}
	internalDate := time.Date(2026, 5, 5, 12, 34, 56, 0, time.FixedZone("KST", 9*60*60))

	var quotaBefore int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaBefore); err != nil {
		t.Fatalf("read quota before append: %v", err)
	}
	result, err := repo.AppendStoredIMAPMessage(ctx, AppendStoredIMAPMessageRequest{
		Target:       target,
		StoragePath:  "mailstore/company/domain/user/imap-append/2026/05/append.eml",
		Parsed:       parsed,
		Flags:        imapgw.MessageFlags{Read: true, Starred: true},
		InternalDate: internalDate,
		Size:         int64(len(raw)),
	})
	if err != nil {
		t.Fatalf("AppendStoredIMAPMessage returned error: %v", err)
	}
	if result.UIDValidity != target.UIDValidity || result.Summary.UID != 1 || result.Summary.MailboxID != imapgw.MailboxID(seed.inboxID) || result.Summary.SequenceNumber != 1 {
		t.Fatalf("append result uidvalidity/mailbox/uid/seq = %d/%q/%d/%d, want %d/%q/1/1", result.UIDValidity, result.Summary.MailboxID, result.Summary.UID, result.Summary.SequenceNumber, target.UIDValidity, seed.inboxID)
	}
	if result.Summary.Envelope.Subject != "IMAP append integration" || !result.Summary.Flags.Read || !result.Summary.Flags.Starred {
		t.Fatalf("append summary = %#v, want subject and initial flags", result.Summary)
	}

	stored, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, result.Summary.UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage appended message returned error: %v", err)
	}
	if stored.StoragePath != "mailstore/company/domain/user/imap-append/2026/05/append.eml" {
		t.Fatalf("appended storage path = %q", stored.StoragePath)
	}

	var quotaAfter int64
	if err := db.QueryRowContext(ctx, `SELECT quota_used FROM users WHERE id = $1`, seed.userID).Scan(&quotaAfter); err != nil {
		t.Fatalf("read quota after append: %v", err)
	}
	if quotaAfter-quotaBefore != int64(len(raw)) {
		t.Fatalf("quota delta = %d, want %d", quotaAfter-quotaBefore, len(raw))
	}

	var outboxTopic, outboxEvent, outboxStoragePath string
	if err := db.QueryRowContext(ctx, `
SELECT topic, payload->>'event', payload->>'storage_path'
FROM outbox
WHERE partition_key = $1`, string(result.Summary.ID)).Scan(&outboxTopic, &outboxEvent, &outboxStoragePath); err != nil {
		t.Fatalf("query append outbox: %v", err)
	}
	if outboxTopic != "mail.event" || outboxEvent != "mail.stored" || outboxStoragePath != stored.StoragePath {
		t.Fatalf("append outbox = %q/%q/%q, want mail.event/mail.stored/%q", outboxTopic, outboxEvent, outboxStoragePath, stored.StoragePath)
	}
}

func TestPostgresIMAPExpungeDeletesOnlyMarkedUIDs(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	var firstID, secondID string
	if err := db.QueryRowContext(ctx, `
INSERT INTO messages (
  tenant_id, domain_id, user_id, folder_id, rfc_message_id, subject,
  from_addr, received_at, size, storage_path, flags
) VALUES
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<expunge-first@example.com>',
   'expunge first', 'sender@example.net', '2026-05-04T00:00:00Z'::timestamptz,
   100, 'mail/expunge-first.eml', '{"imap_deleted":true}'::jsonb),
  ($1::uuid, $2::uuid, $3::uuid, $4::uuid, '<expunge-second@example.com>',
   'expunge second', 'sender@example.net', '2026-05-04T00:01:00Z'::timestamptz,
   100, 'mail/expunge-second.eml', '{"imap_deleted":true}'::jsonb)
RETURNING id::text`, seed.companyID, seed.domainID, seed.userID, seed.inboxID).Scan(&firstID); err != nil {
		t.Fatalf("insert first expunge message: %v", err)
	}
	if err := db.QueryRowContext(ctx, `
SELECT id::text
FROM messages
WHERE user_id = $1::uuid
  AND folder_id = $2::uuid
  AND subject = 'expunge second'`, seed.userID, seed.inboxID).Scan(&secondID); err != nil {
		t.Fatalf("select second expunge message: %v", err)
	}
	firstUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, firstID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID first returned error: %v", err)
	}
	secondUID, err := repo.EnsureIMAPMessageUID(ctx, seed.userID, seed.inboxID, secondID)
	if err != nil {
		t.Fatalf("EnsureIMAPMessageUID second returned error: %v", err)
	}

	expunged, err := repo.ExpungeIMAPMessages(ctx, seed.userID, seed.inboxID, []imapgw.UID{secondUID.UID})
	if err != nil {
		t.Fatalf("ExpungeIMAPMessages returned error: %v", err)
	}
	if len(expunged) != 1 || expunged[0].UID != secondUID.UID || expunged[0].SequenceNumber != 2 {
		t.Fatalf("expunged summaries = %#v, want second UID with pre-expunge sequence 2", expunged)
	}
	if _, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, secondUID.UID); err == nil {
		t.Fatal("GetIMAPMessage found expunged message")
	}
	remaining, err := repo.GetIMAPMessage(ctx, seed.userID, seed.inboxID, firstUID.UID)
	if err != nil {
		t.Fatalf("GetIMAPMessage remaining returned error: %v", err)
	}
	if string(remaining.Summary.ID) != firstID || !remaining.Summary.Flags.Deleted {
		t.Fatalf("remaining summary = %#v, want first message still marked deleted", remaining.Summary)
	}
	var status string
	if err := db.QueryRowContext(ctx, `SELECT status FROM messages WHERE id = $1::uuid`, secondID).Scan(&status); err != nil {
		t.Fatalf("query expunged status: %v", err)
	}
	if status != "deleted" {
		t.Fatalf("expunged status = %q, want deleted", status)
	}
}

type postgresSeed struct {
	companyID string
	domainID  string
	userID    string
	inboxID   string
	sentID    string
}

func seedPostgresMailUser(t *testing.T, db *sql.DB) postgresSeed {
	t.Helper()

	ctx := context.Background()
	var seed postgresSeed
	if err := db.QueryRowContext(ctx, `
WITH company AS (
  INSERT INTO companies (name) VALUES ('Release Co') RETURNING id
), domain AS (
  INSERT INTO domains (company_id, name, name_ace)
  SELECT id, 'example.com', 'example.com' FROM company RETURNING id, company_id
), app_user AS (
  INSERT INTO users (domain_id, username, display_name)
  SELECT id, 'alice', 'Alice' FROM domain RETURNING id, domain_id
), address AS (
  INSERT INTO user_addresses (user_id, domain_id, local_part, local_part_ace, domain_ace, address, address_ace, is_primary)
  SELECT app_user.id, app_user.domain_id, 'alice', 'alice', 'example.com', 'alice@example.com', 'alice@example.com', true
  FROM app_user
), folders AS (
  INSERT INTO folders (user_id, name, full_path, type, system_type)
  SELECT id, 'Inbox', '/Inbox', 'system', 'inbox' FROM app_user
  UNION ALL
  SELECT id, 'Drafts', '/Drafts', 'system', 'drafts' FROM app_user
  UNION ALL
  SELECT id, 'Sent', '/Sent', 'system', 'sent' FROM app_user
  RETURNING id, system_type
)
SELECT
  domain.company_id::text,
  domain.id::text,
  app_user.id::text,
  (SELECT id::text FROM folders WHERE system_type = 'inbox'),
  (SELECT id::text FROM folders WHERE system_type = 'sent')
FROM domain, app_user`).Scan(&seed.companyID, &seed.domainID, &seed.userID, &seed.inboxID, &seed.sentID); err != nil {
		t.Fatalf("seed postgres mail user: %v", err)
	}
	return seed
}

func insertPostgresAPIUsageLedgerEvent(t *testing.T, db *sql.DB, eventID string, eventAt time.Time, recordedAt time.Time, tenantID string, principalID string) {
	t.Helper()

	if _, err := db.ExecContext(context.Background(), `
INSERT INTO api_usage_ledger (
  event_id,
  schema_version,
  event_timestamp,
  recorded_at,
  method,
  route,
  status,
  tenant_id,
  principal_id,
  auth_source,
  request_count,
  request_bytes,
  response_bytes,
  latency_ms,
  payload
) VALUES ($1, '2026-05-04.api-usage.v2', $2, $3, 'GET', '/api/v1/messages', 200, $4, $5, 'bearer', 1, 10, 20, 5, '{}'::jsonb)`, eventID, eventAt.UTC(), recordedAt.UTC(), tenantID, principalID); err != nil {
		t.Fatalf("insert api usage ledger event %s: %v", eventID, err)
	}
}

func insertPostgresAPIUsageExportEvidence(t *testing.T, db *sql.DB, completedAt time.Time, windowStart time.Time, windowEnd time.Time, eventCount int64) {
	t.Helper()

	const digest = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO api_usage_export_batches (
  id, completed_at, status, export_format, tenant_id, principal_id, window_start, window_end,
  event_count, request_count, request_bytes, response_bytes, latency_ms_total, latency_ms_max,
  first_event_at, last_event_at, manifest
) VALUES (
  'retention-batch-1', $1, 'completed', 'ndjson', 'tenant-1', 'principal-1', $2, $3,
  $4, $4, 30, 60, 15, 5, $2, $3, '{}'::jsonb
)`, completedAt.UTC(), windowStart.UTC(), windowEnd.UTC(), eventCount); err != nil {
		t.Fatalf("insert api usage export batch: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO api_usage_export_artifacts (
  id, batch_id, object_key, content_type, byte_count, sha256_hex, event_count, metadata
) VALUES (
  'retention-artifact-1', 'retention-batch-1', 'exports/retention-batch-1.ndjson',
  'application/x-ndjson', 100, $1, $2, '{}'::jsonb
)`, digest, eventCount); err != nil {
		t.Fatalf("insert api usage export artifact: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO api_usage_export_manifest_digests (
  id, batch_id, schema_version, digest_algorithm, digest_hex, manifest
) VALUES (
  'retention-digest-1', 'retention-batch-1', '2026-05-04.api-usage-export-manifest.v1',
  'sha256', $1, '{}'::jsonb
)`, digest); err != nil {
		t.Fatalf("insert api usage export digest: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), `
INSERT INTO api_usage_export_manifest_signatures (
  id, digest_id, batch_id, signer_backend, key_id, signature_algorithm, signed_digest_hex, signature_hex, metadata
) VALUES (
  'retention-signature-1', 'retention-digest-1', 'retention-batch-1', 'local-hmac', 'key-1',
  'hmac-sha256', $1, 'bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb', '{}'::jsonb
)`, digest); err != nil {
		t.Fatalf("insert api usage export signature: %v", err)
	}
}

func openMigratedPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL migration/repository integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := postgresURLWithSearchPath(t, baseURL, schema)
	db, err := database.Open(ctx, dbURL)
	if err != nil {
		t.Fatalf("open postgres test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	migrationDir, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migration directory: %v", err)
	}
	if err := database.MigrateUp(ctx, db, migrationDir); err != nil {
		t.Fatalf("migrate postgres test database: %v", err)
	}
	return db
}

func postgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	searchPathOption := "-c search_path=" + schema + ",public"
	if options != "" {
		options += " "
	}
	options += searchPathOption
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
