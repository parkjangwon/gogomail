package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	"github.com/gogomail/gogomail/internal/outbound"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresMigrationsApplyWithReleaseSchema(t *testing.T) {
	t.Parallel()

	db := openMigratedPostgresTestDB(t)
	var count int
	if err := db.QueryRowContext(context.Background(), `SELECT count(*) FROM goose_db_version`).Scan(&count); err != nil {
		t.Fatalf("query goose version table: %v", err)
	}
	if count == 0 {
		t.Fatal("goose_db_version is empty after migrations")
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

type postgresSeed struct {
	companyID string
	domainID  string
	userID    string
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
)
SELECT domain.company_id::text, domain.id::text, app_user.id::text
FROM domain, app_user`).Scan(&seed.companyID, &seed.domainID, &seed.userID); err != nil {
		t.Fatalf("seed postgres mail user: %v", err)
	}
	return seed
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
