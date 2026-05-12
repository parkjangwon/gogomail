package carddavgw

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
	"github.com/jackc/pgx/v5/pgconn"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresAddressBookPropertyLanguagesRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedCardDAVPostgresTestDB(t)
	userID := seedCardDAVPostgresUser(t, ctx, db)
	repo := NewRepository(db)

	book, err := repo.CreateAddressBook(ctx, CreateAddressBookRequest{
		UserID:          userID,
		ActorUserID:     userID,
		Name:            "People",
		NameLang:        "ko-KR",
		Description:     "Known contacts",
		DescriptionLang: "en-US",
	})
	if err != nil {
		t.Fatalf("CreateAddressBook returned error: %v", err)
	}
	if book.NameLang != "ko-KR" || book.DescriptionLang != "en-US" {
		t.Fatalf("created address book languages = name %q description %q", book.NameLang, book.DescriptionLang)
	}

	got, err := repo.GetAddressBook(ctx, GetAddressBookRequest{UserID: userID, AddressBookID: book.ID, Status: "active"})
	if err != nil {
		t.Fatalf("GetAddressBook returned error: %v", err)
	}
	if got.NameLang != "ko-KR" || got.DescriptionLang != "en-US" {
		t.Fatalf("stored address book languages = name %q description %q", got.NameLang, got.DescriptionLang)
	}

	updatedName := "Team"
	updatedNameLang := "ja-JP"
	updatedDescription := "Team contacts"
	updatedDescriptionLang := "fr"
	updated, err := repo.UpdateAddressBookProperties(ctx, UpdateAddressBookRequest{
		UserID:          userID,
		ActorUserID:     userID,
		AddressBookID:   book.ID,
		Name:            &updatedName,
		NameLang:        &updatedNameLang,
		Description:     &updatedDescription,
		DescriptionLang: &updatedDescriptionLang,
	})
	if err != nil {
		t.Fatalf("UpdateAddressBookProperties returned error: %v", err)
	}
	if updated.NameLang != "ja-JP" || updated.DescriptionLang != "fr" {
		t.Fatalf("updated address book languages = name %q description %q", updated.NameLang, updated.DescriptionLang)
	}

	var storedNameLang, storedDescriptionLang string
	if err := db.QueryRowContext(ctx, `
SELECT displayname_lang, description_lang
FROM carddav_addressbooks
WHERE id = $1::uuid
`, book.ID).Scan(&storedNameLang, &storedDescriptionLang); err != nil {
		t.Fatalf("query stored address book language columns: %v", err)
	}
	if storedNameLang != "ja-JP" || storedDescriptionLang != "fr" {
		t.Fatalf("raw address book language columns = name %q description %q", storedNameLang, storedDescriptionLang)
	}
}

func TestPostgresAddressBookPropertyLanguageConstraintsRejectInvalidValues(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedCardDAVPostgresTestDB(t)
	userID := seedCardDAVPostgresUser(t, ctx, db)
	repo := NewRepository(db)

	book, err := repo.CreateAddressBook(ctx, CreateAddressBookRequest{
		UserID:      userID,
		ActorUserID: userID,
		Name:        "Constraints",
	})
	if err != nil {
		t.Fatalf("CreateAddressBook returned error: %v", err)
	}

	_, err = db.ExecContext(ctx, `
UPDATE carddav_addressbooks
SET displayname_lang = $2
WHERE id = $1::uuid
`, book.ID, "en US")
	assertCardDAVPostgresCheckViolation(t, err, "carddav_addressbooks_displayname_lang_check")

	_, err = db.ExecContext(ctx, `
UPDATE carddav_addressbooks
SET description_lang = $2
WHERE id = $1::uuid
`, book.ID, strings.Repeat("a", 65))
	assertCardDAVPostgresCheckViolation(t, err, "carddav_addressbooks_description_lang_check")
}

func seedCardDAVPostgresUser(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()

	const (
		companyID = "20000000-0000-4000-8000-000000000001"
		domainID  = "20000000-0000-4000-8000-000000000002"
		userID    = "20000000-0000-4000-8000-000000000003"
	)
	if _, err := db.ExecContext(ctx, `
INSERT INTO companies (id, name, status) VALUES ($1::uuid, 'CardDAV Test Co', 'active');
INSERT INTO domains (id, company_id, name, name_ace, status) VALUES ($2::uuid, $1::uuid, 'carddav.test', 'carddav.test', 'active');
INSERT INTO users (id, domain_id, username, display_name, status) VALUES ($3::uuid, $2::uuid, 'contacts', 'Contacts User', 'active');
`, companyID, domainID, userID); err != nil {
		t.Fatalf("seed CardDAV postgres user: %v", err)
	}
	return userID
}

func openMigratedCardDAVPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL CardDAV integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_carddav_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := cardDAVPostgresURLWithSearchPath(t, baseURL, schema)
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

func cardDAVPostgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
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

func assertCardDAVPostgresCheckViolation(t *testing.T, err error, constraint string) {
	t.Helper()
	if err == nil {
		t.Fatalf("expected PostgreSQL check violation for %s", constraint)
	}
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) {
		t.Fatalf("error = %T %v, want PostgreSQL error", err, err)
	}
	if pgErr.Code != "23514" || pgErr.ConstraintName != constraint {
		t.Fatalf("PostgreSQL error code=%s constraint=%s, want 23514 %s", pgErr.Code, pgErr.ConstraintName, constraint)
	}
}
