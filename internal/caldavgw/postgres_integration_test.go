package caldavgw

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
	_ "github.com/jackc/pgx/v5/stdlib"
)

func TestPostgresCalendarPropertyLanguagesRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedCalDAVPostgresTestDB(t)
	userID := seedCalDAVPostgresUser(t, ctx, db)
	repo := NewRepository(db)

	nameLang := "ko-KR"
	descriptionLang := "en-US"
	calendar, err := repo.CreateCalendar(ctx, CreateCalendarRequest{
		UserID:          userID,
		ActorUserID:     userID,
		Name:            "Work",
		NameLang:        &nameLang,
		Description:     "Shared schedule",
		DescriptionLang: &descriptionLang,
	})
	if err != nil {
		t.Fatalf("CreateCalendar returned error: %v", err)
	}
	if calendar.NameLang != "ko-KR" || calendar.DescriptionLang != "en-US" {
		t.Fatalf("created calendar languages = name %q description %q", calendar.NameLang, calendar.DescriptionLang)
	}

	got, err := repo.GetCalendar(ctx, GetCalendarRequest{UserID: userID, CalendarID: calendar.ID, Status: "active"})
	if err != nil {
		t.Fatalf("GetCalendar returned error: %v", err)
	}
	if got.NameLang != "ko-KR" || got.DescriptionLang != "en-US" {
		t.Fatalf("stored calendar languages = name %q description %q", got.NameLang, got.DescriptionLang)
	}

	updatedNameLang := "ja-JP"
	updatedDescriptionLang := "fr"
	updatedName := "Focus"
	updatedDescription := "Focused schedule"
	updated, err := repo.UpdateCalendarProperties(ctx, UpdateCalendarRequest{
		UserID:          userID,
		ActorUserID:     userID,
		CalendarID:      calendar.ID,
		Name:            &updatedName,
		NameLang:        &updatedNameLang,
		Description:     &updatedDescription,
		DescriptionLang: &updatedDescriptionLang,
	})
	if err != nil {
		t.Fatalf("UpdateCalendarProperties returned error: %v", err)
	}
	if updated.NameLang != "ja-JP" || updated.DescriptionLang != "fr" {
		t.Fatalf("updated calendar languages = name %q description %q", updated.NameLang, updated.DescriptionLang)
	}

	var storedNameLang, storedDescriptionLang string
	if err := db.QueryRowContext(ctx, `
SELECT displayname_lang, description_lang
FROM caldav_calendars
WHERE id = $1::uuid
`, calendar.ID).Scan(&storedNameLang, &storedDescriptionLang); err != nil {
		t.Fatalf("query stored calendar language columns: %v", err)
	}
	if storedNameLang != "ja-JP" || storedDescriptionLang != "fr" {
		t.Fatalf("raw calendar language columns = name %q description %q", storedNameLang, storedDescriptionLang)
	}
}

func seedCalDAVPostgresUser(t *testing.T, ctx context.Context, db *sql.DB) string {
	t.Helper()

	const (
		companyID = "10000000-0000-4000-8000-000000000001"
		domainID  = "10000000-0000-4000-8000-000000000002"
		userID    = "10000000-0000-4000-8000-000000000003"
	)
	if _, err := db.ExecContext(ctx, `
INSERT INTO companies (id, name, status) VALUES ($1::uuid, 'CalDAV Test Co', 'active');
INSERT INTO domains (id, company_id, name, name_ace, status) VALUES ($2::uuid, $1::uuid, 'caldav.test', 'caldav.test', 'active');
INSERT INTO users (id, domain_id, username, display_name, status) VALUES ($3::uuid, $2::uuid, 'calendar', 'Calendar User', 'active');
`, companyID, domainID, userID); err != nil {
		t.Fatalf("seed CalDAV postgres user: %v", err)
	}
	return userID
}

func openMigratedCalDAVPostgresTestDB(t *testing.T) *sql.DB {
	t.Helper()

	baseURL := strings.TrimSpace(os.Getenv("GOGOMAIL_TEST_DATABASE_URL"))
	if baseURL == "" {
		t.Skip("set GOGOMAIL_TEST_DATABASE_URL to run PostgreSQL CalDAV integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	t.Cleanup(cancel)

	adminDB, err := sql.Open("pgx", baseURL)
	if err != nil {
		t.Fatalf("open postgres admin connection: %v", err)
	}
	t.Cleanup(func() { _ = adminDB.Close() })

	schema := fmt.Sprintf("gogomail_caldav_test_%d", time.Now().UnixNano())
	if _, err := adminDB.ExecContext(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatalf("create test schema: %v", err)
	}
	t.Cleanup(func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = adminDB.ExecContext(cleanupCtx, `DROP SCHEMA IF EXISTS `+schema+` CASCADE`)
	})

	dbURL := calDAVPostgresURLWithSearchPath(t, baseURL, schema)
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

func calDAVPostgresURLWithSearchPath(t *testing.T, rawURL string, schema string) string {
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
