package dm

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/database"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// dmPinsFixture holds seeded identifiers for the pin integration tests.
type dmPinsFixture struct {
	companyID     string
	domainID      string
	memberID      string
	otherMemberID string
	nonMemberID   string
	roomID        string
	otherRoomID   string
	messageID     string
	otherMsgID    string // message that belongs to otherRoomID, not roomID
	systemMsgID   string
}

func (f dmPinsFixture) principal() Principal {
	return Principal{UserID: f.memberID, CompanyID: f.companyID, DomainID: f.domainID}
}

func (f dmPinsFixture) nonMemberPrincipal() Principal {
	return Principal{UserID: f.nonMemberID, CompanyID: f.companyID, DomainID: f.domainID}
}

func TestPostgresPinMessageIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, "first"); err != nil {
		t.Fatalf("PinMessage (1): %v", err)
	}
	// Pinning again must not error and must not create a duplicate row.
	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, "second"); err != nil {
		t.Fatalf("PinMessage (2): %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dm_message_pins WHERE room_id = $1 AND message_id = $2`, fx.roomID, fx.messageID).Scan(&count); err != nil {
		t.Fatalf("count pins: %v", err)
	}
	if count != 1 {
		t.Fatalf("pin count = %d, want 1 (idempotent)", count)
	}
	// The first note is preserved (ON CONFLICT DO NOTHING).
	var note sql.NullString
	if err := db.QueryRowContext(ctx, `SELECT note FROM dm_message_pins WHERE room_id = $1 AND message_id = $2`, fx.roomID, fx.messageID).Scan(&note); err != nil {
		t.Fatalf("select note: %v", err)
	}
	if note.String != "first" {
		t.Fatalf("note = %q, want first", note.String)
	}
}

func TestPostgresPinMessageRejectsNonMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	err := store.PinMessage(ctx, fx.nonMemberPrincipal(), fx.roomID, fx.messageID, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member pin err = %v, want ErrNotFound", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dm_message_pins WHERE room_id = $1`, fx.roomID).Scan(&count); err != nil {
		t.Fatalf("count pins: %v", err)
	}
	if count != 0 {
		t.Fatalf("pin count = %d, want 0", count)
	}
}

func TestPostgresPinMessageRejectsCrossRoomMessage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	// otherMsgID belongs to otherRoomID; pinning it under roomID must fail.
	err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.otherMsgID, "")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-room pin err = %v, want ErrNotFound", err)
	}
}

func TestPostgresPinMessageRejectsSystemMessage(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.systemMsgID, "")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("system message pin err = %v, want ErrInvalid", err)
	}
}

func TestPostgresUnpinMessageNoOpWhenNotPinned(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	// Unpinning a message that was never pinned is a success no-op.
	if err := store.UnpinMessage(ctx, fx.principal(), fx.roomID, fx.messageID); err != nil {
		t.Fatalf("UnpinMessage (no-op): %v", err)
	}
}

func TestPostgresUnpinMessageRejectsNonMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, ""); err != nil {
		t.Fatalf("PinMessage: %v", err)
	}
	err := store.UnpinMessage(ctx, fx.nonMemberPrincipal(), fx.roomID, fx.messageID)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("non-member unpin err = %v, want ErrNotFound", err)
	}
	// The pin must still exist.
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dm_message_pins WHERE room_id = $1 AND message_id = $2`, fx.roomID, fx.messageID).Scan(&count); err != nil {
		t.Fatalf("count pins: %v", err)
	}
	if count != 1 {
		t.Fatalf("pin count = %d, want 1 (unchanged)", count)
	}
}

func TestPostgresListPinnedMessagesNewestFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	// Insert a second pinnable message in the room.
	secondMsgID := insertDMTextMessage(t, ctx, db, fx.roomID, fx.memberID, []byte("second body"))

	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, "note-a"); err != nil {
		t.Fatalf("PinMessage (1): %v", err)
	}
	// Force a later pinned_at for the second pin so ordering is deterministic.
	if _, err := db.ExecContext(ctx, `UPDATE dm_message_pins SET pinned_at = now() - interval '1 hour' WHERE message_id = $1`, fx.messageID); err != nil {
		t.Fatalf("backdate first pin: %v", err)
	}
	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, secondMsgID, "note-b"); err != nil {
		t.Fatalf("PinMessage (2): %v", err)
	}

	pins, err := store.ListPinnedMessages(ctx, fx.principal(), fx.roomID, 50, 0)
	if err != nil {
		t.Fatalf("ListPinnedMessages: %v", err)
	}
	if len(pins) != 2 {
		t.Fatalf("pins = %d, want 2", len(pins))
	}
	// Newest pin first: secondMsgID (pinned now) before messageID (backdated).
	if pins[0].Record.ID != secondMsgID {
		t.Fatalf("pins[0].ID = %q, want %q (newest first)", pins[0].Record.ID, secondMsgID)
	}
	if pins[1].Record.ID != fx.messageID {
		t.Fatalf("pins[1].ID = %q, want %q", pins[1].Record.ID, fx.messageID)
	}
	if pins[0].Note != "note-b" || pins[1].Note != "note-a" {
		t.Fatalf("notes = %q, %q, want note-b, note-a", pins[0].Note, pins[1].Note)
	}
	if pins[0].PinnedByUserID != fx.memberID {
		t.Fatalf("pinned_by = %q, want %q", pins[0].PinnedByUserID, fx.memberID)
	}
	// The encrypted body must be carried through for the service to decrypt.
	if len(pins[0].Record.BodyCiphertext) == 0 {
		t.Fatalf("pinned record missing body ciphertext")
	}
}

func TestPostgresListPinnedMessagesRejectsNonMember(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, ""); err != nil {
		t.Fatalf("PinMessage: %v", err)
	}
	pins, err := store.ListPinnedMessages(ctx, fx.nonMemberPrincipal(), fx.roomID, 50, 0)
	if err != nil {
		t.Fatalf("ListPinnedMessages (non-member): %v", err)
	}
	if len(pins) != 0 {
		t.Fatalf("non-member sees %d pins, want 0 (tenant/membership isolation)", len(pins))
	}
}

func TestPostgresPinCascadesOnMessageDelete(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	db := openMigratedDMPostgresTestDB(t)
	store := NewPostgresStore(db)
	fx := seedDMPinsFixture(t, ctx, db)

	if err := store.PinMessage(ctx, fx.principal(), fx.roomID, fx.messageID, ""); err != nil {
		t.Fatalf("PinMessage: %v", err)
	}
	// Hard-deleting the message must cascade to the pin.
	if _, err := db.ExecContext(ctx, `DELETE FROM dm_messages WHERE id = $1`, fx.messageID); err != nil {
		t.Fatalf("delete message: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM dm_message_pins WHERE message_id = $1`, fx.messageID).Scan(&count); err != nil {
		t.Fatalf("count pins: %v", err)
	}
	if count != 0 {
		t.Fatalf("pin count after message delete = %d, want 0 (ON DELETE CASCADE)", count)
	}
}

// --- fixtures & DB helpers ---

func seedDMPinsFixture(t *testing.T, ctx context.Context, db *sql.DB) dmPinsFixture {
	t.Helper()
	const (
		companyID     = "20000000-0000-4000-8000-000000000001"
		domainID      = "20000000-0000-4000-8000-000000000002"
		memberID      = "20000000-0000-4000-8000-000000000003"
		otherMemberID = "20000000-0000-4000-8000-000000000004"
		nonMemberID   = "20000000-0000-4000-8000-000000000005"
	)
	// NOTE: lib/pq (via database/sql prepared statements) rejects multiple
	// commands in one Exec, so each INSERT is a separate call (repo convention).
	if _, err := db.ExecContext(ctx, `
INSERT INTO companies (id, name, status) VALUES ($1::uuid, 'DM Pins Test Co', 'active')`,
		companyID); err != nil {
		t.Fatalf("seed dm pins company: %v", err)
	}
	if _, err := db.ExecContext(ctx, `
INSERT INTO domains (id, company_id, name, name_ace, status) VALUES ($1::uuid, $2::uuid, 'dmpins.test', 'dmpins.test', 'active')`,
		domainID, companyID); err != nil {
		t.Fatalf("seed dm pins domain: %v", err)
	}
	for _, u := range [][2]string{{memberID, "member"}, {otherMemberID, "other"}, {nonMemberID, "outsider"}} {
		if _, err := db.ExecContext(ctx, `
INSERT INTO users (id, domain_id, username, display_name, status) VALUES ($1::uuid, $2::uuid, $3, $3, 'active')`,
			u[0], domainID, u[1]); err != nil {
			t.Fatalf("seed dm pins user %s: %v", u[1], err)
		}
	}

	fx := dmPinsFixture{
		companyID:     companyID,
		domainID:      domainID,
		memberID:      memberID,
		otherMemberID: otherMemberID,
		nonMemberID:   nonMemberID,
	}

	fx.roomID = insertDMGroupRoom(t, ctx, db, companyID, domainID, memberID, "Room One")
	fx.otherRoomID = insertDMGroupRoom(t, ctx, db, companyID, domainID, memberID, "Room Two")
	insertDMParticipant(t, ctx, db, fx.roomID, memberID)
	insertDMParticipant(t, ctx, db, fx.roomID, otherMemberID)
	insertDMParticipant(t, ctx, db, fx.otherRoomID, memberID)

	fx.messageID = insertDMTextMessage(t, ctx, db, fx.roomID, memberID, []byte("hello ciphertext"))
	fx.otherMsgID = insertDMTextMessage(t, ctx, db, fx.otherRoomID, memberID, []byte("other room ciphertext"))
	fx.systemMsgID = insertDMSystemMessage(t, ctx, db, fx.roomID, []byte("system ciphertext"))
	return fx
}

func insertDMGroupRoom(t *testing.T, ctx context.Context, db *sql.DB, companyID, domainID, ownerID, name string) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx, `
INSERT INTO dm_rooms (company_id, domain_id, room_type, visibility, name, owner_id, created_by)
VALUES ($1::uuid, $2::uuid, 'group', 'private', $3, $4::uuid, $4::uuid)
RETURNING id::text`, companyID, domainID, name, ownerID).Scan(&id); err != nil {
		t.Fatalf("insert dm room: %v", err)
	}
	return id
}

func insertDMParticipant(t *testing.T, ctx context.Context, db *sql.DB, roomID, userID string) {
	t.Helper()
	if _, err := db.ExecContext(ctx, `INSERT INTO dm_participants (room_id, user_id) VALUES ($1::uuid, $2::uuid)`, roomID, userID); err != nil {
		t.Fatalf("insert dm participant: %v", err)
	}
}

func insertDMTextMessage(t *testing.T, ctx context.Context, db *sql.DB, roomID, senderID string, body []byte) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx, `
INSERT INTO dm_messages (room_id, sender_id, message_type, body)
VALUES ($1::uuid, $2::uuid, 'text', $3)
RETURNING id::text`, roomID, senderID, body).Scan(&id); err != nil {
		t.Fatalf("insert dm message: %v", err)
	}
	return id
}

func insertDMSystemMessage(t *testing.T, ctx context.Context, db *sql.DB, roomID string, body []byte) string {
	t.Helper()
	var id string
	if err := db.QueryRowContext(ctx, `
INSERT INTO dm_messages (room_id, message_type, body)
VALUES ($1::uuid, 'system', $2)
RETURNING id::text`, roomID, body).Scan(&id); err != nil {
		t.Fatalf("insert dm system message: %v", err)
	}
	return id
}

func openMigratedDMPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL DM pins integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_dm_pins_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := dmPostgresURLWithSearchPath(t, baseURL, schema)
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

func dmPostgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
	t.Helper()

	parsed, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("parse GOGOMAIL_TEST_DATABASE_URL: %v", err)
	}
	query := parsed.Query()
	options := strings.TrimSpace(query.Get("options"))
	if options != "" {
		options += " "
	}
	options += "-c search_path=" + schema + ",public"
	query.Set("options", options)
	parsed.RawQuery = query.Encode()
	return parsed.String()
}
