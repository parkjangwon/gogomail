package maildb

import (
	"context"
	"testing"
)

func TestValidateDNDScheduleAcceptsEmpty(t *testing.T) {
	t.Parallel()
	out, err := ValidateDNDSchedule(DNDSchedule{})
	if err != nil {
		t.Fatalf("empty schedule should be valid, got %v", err)
	}
	if out.Timezone != "UTC" {
		t.Fatalf("empty timezone should normalize to UTC, got %q", out.Timezone)
	}
}

func TestValidateDNDScheduleRejectsBadWeekday(t *testing.T) {
	t.Parallel()
	_, err := ValidateDNDSchedule(DNDSchedule{Weekdays: []int{0, 7}})
	if err == nil {
		t.Fatal("weekday 7 should be rejected")
	}
}

func TestValidateDNDScheduleRejectsDuplicateWeekday(t *testing.T) {
	t.Parallel()
	_, err := ValidateDNDSchedule(DNDSchedule{Weekdays: []int{0, 0}})
	if err == nil {
		t.Fatal("duplicate weekday should be rejected")
	}
}

func TestValidateDNDScheduleRejectsBadTimeFormat(t *testing.T) {
	t.Parallel()
	cases := []TimeRange{
		{Start: "24:00", End: "08:00"},
		{Start: "7:00", End: "08:00"},
		{Start: "07:60", End: "08:00"},
		{Start: "abc", End: "08:00"},
		{Start: "07:00", End: ""},
	}
	for i, c := range cases {
		if _, err := ValidateDNDSchedule(DNDSchedule{TimeRanges: []TimeRange{c}}); err == nil {
			t.Errorf("case %d: expected error for %+v", i, c)
		}
	}
}

func TestValidateDNDScheduleRejectsTooManyRanges(t *testing.T) {
	t.Parallel()
	ranges := make([]TimeRange, maxNotificationTimeRanges+1)
	for i := range ranges {
		ranges[i] = TimeRange{Start: "01:00", End: "02:00"}
	}
	if _, err := ValidateDNDSchedule(DNDSchedule{TimeRanges: ranges}); err == nil {
		t.Fatal("too many ranges should be rejected")
	}
}

func TestValidateDNDScheduleRejectsBadTimezone(t *testing.T) {
	t.Parallel()
	_, err := ValidateDNDSchedule(DNDSchedule{Timezone: "Mars/Olympus"})
	if err == nil {
		t.Fatal("invalid timezone should be rejected")
	}
}

func TestValidateDNDScheduleNormalizesWeekdaysAndTimezone(t *testing.T) {
	t.Parallel()
	out, err := ValidateDNDSchedule(DNDSchedule{
		Weekdays:   []int{5, 1, 3},
		TimeRanges: []TimeRange{{Start: "22:00", End: "08:00"}},
		Timezone:   "Asia/Seoul",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := out.Weekdays; len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 5 {
		t.Fatalf("weekdays not sorted: %v", got)
	}
}

func TestValidateNotificationPreferencesRequiresUserID(t *testing.T) {
	t.Parallel()
	if _, err := ValidateNotificationPreferences(NotificationPreferences{}); err == nil {
		t.Fatal("missing user_id should be rejected")
	}
	if _, err := ValidateNotificationPreferences(NotificationPreferences{UserID: "not-a-uuid"}); err == nil {
		t.Fatal("non-uuid user_id should be rejected")
	}
}

func TestValidateNotificationPreferencesRejectsBadFolderID(t *testing.T) {
	t.Parallel()
	prefs := NotificationPreferences{
		UserID: "00000000-0000-0000-0000-000000000001",
		FolderOverrides: map[string]FolderNotificationOverride{
			"not-a-uuid": {Enabled: true, DNDInherit: true},
		},
	}
	if _, err := ValidateNotificationPreferences(prefs); err == nil {
		t.Fatal("non-uuid folder id should be rejected")
	}
}

func TestValidateNotificationPreferencesAcceptsThreadOverrides(t *testing.T) {
	t.Parallel()
	threadID := "00000000-0000-0000-0000-000000000002"
	out, err := ValidateNotificationPreferences(NotificationPreferences{
		UserID: "00000000-0000-0000-0000-000000000001",
		ThreadOverrides: map[string]ThreadNotificationOverride{
			threadID: {Enabled: false},
		},
	})
	if err != nil {
		t.Fatalf("thread override should be valid: %v", err)
	}
	override, ok := out.ThreadOverrides[threadID]
	if !ok {
		t.Fatalf("missing normalized thread override for %s", threadID)
	}
	if override.Enabled {
		t.Fatalf("thread override enabled = true, want false")
	}
}

func TestValidateNotificationPreferencesRejectsBadThreadID(t *testing.T) {
	t.Parallel()
	prefs := NotificationPreferences{
		UserID: "00000000-0000-0000-0000-000000000001",
		ThreadOverrides: map[string]ThreadNotificationOverride{
			"not-a-uuid": {Enabled: false},
		},
	}
	if _, err := ValidateNotificationPreferences(prefs); err == nil {
		t.Fatal("non-uuid thread id should be rejected")
	}
}

func TestValidateNotificationPreferencesRejectsTooManyFolders(t *testing.T) {
	t.Parallel()
	prefs := NotificationPreferences{
		UserID:          "00000000-0000-0000-0000-000000000001",
		FolderOverrides: make(map[string]FolderNotificationOverride, maxNotificationFolderEntries+1),
	}
	// Generate well-formed UUIDs by varying the last bytes.
	for i := 0; i <= maxNotificationFolderEntries; i++ {
		id := genTestFolderID(i)
		prefs.FolderOverrides[id] = FolderNotificationOverride{Enabled: true, DNDInherit: true}
	}
	if _, err := ValidateNotificationPreferences(prefs); err == nil {
		t.Fatal("over-cap folder map should be rejected")
	}
}

func genTestFolderID(i int) string {
	const hex = "0123456789abcdef"
	// Encode i (max 16 bits) into the last 4 hex digits of the UUID.
	last4 := []byte{hex[(i>>12)&0xF], hex[(i>>8)&0xF], hex[(i>>4)&0xF], hex[i&0xF]}
	return "00000000-0000-0000-0000-00000000" + string(last4) + "0000"
}

// --- Integration tests (require GOGOMAIL_TEST_DATABASE_URL) ---

func TestPostgresNotificationPreferencesGetMissingReturnsDefault(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	prefs, err := repo.GetNotificationPreferences(ctx, seed.userID)
	if err != nil {
		t.Fatalf("GetNotificationPreferences: %v", err)
	}
	if prefs == nil {
		t.Fatal("expected non-nil prefs")
	}
	if prefs.GlobalDNDEnabled {
		t.Fatal("default GlobalDNDEnabled should be false")
	}
	if len(prefs.FolderOverrides) != 0 {
		t.Fatalf("default FolderOverrides should be empty, got %v", prefs.FolderOverrides)
	}
}

func TestPostgresNotificationPreferencesUpsertRoundTrip(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	prefs := NotificationPreferences{
		UserID:           seed.userID,
		GlobalDNDEnabled: true,
		GlobalDNDSchedule: DNDSchedule{
			Weekdays:   []int{0, 6},
			TimeRanges: []TimeRange{{Start: "22:00", End: "08:00"}},
			Timezone:   "Asia/Seoul",
		},
		FolderOverrides: map[string]FolderNotificationOverride{
			seed.inboxID: {Enabled: false, DNDInherit: true},
		},
	}
	if err := repo.UpsertNotificationPreferences(ctx, prefs); err != nil {
		t.Fatalf("UpsertNotificationPreferences: %v", err)
	}
	got, err := repo.GetNotificationPreferences(ctx, seed.userID)
	if err != nil {
		t.Fatalf("GetNotificationPreferences: %v", err)
	}
	if !got.GlobalDNDEnabled {
		t.Fatal("expected GlobalDNDEnabled=true")
	}
	if got.GlobalDNDSchedule.Timezone != "Asia/Seoul" {
		t.Fatalf("timezone = %q", got.GlobalDNDSchedule.Timezone)
	}
	if len(got.GlobalDNDSchedule.TimeRanges) != 1 || got.GlobalDNDSchedule.TimeRanges[0].Start != "22:00" {
		t.Fatalf("time ranges = %+v", got.GlobalDNDSchedule.TimeRanges)
	}
	override, ok := got.FolderOverrides[seed.inboxID]
	if !ok {
		t.Fatalf("missing folder override for %s", seed.inboxID)
	}
	if override.Enabled || !override.DNDInherit {
		t.Fatalf("override mismatch: %+v", override)
	}
	if got.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt should be set after upsert")
	}
}

func TestPostgresNotificationPreferencesUpsertRejectsBadInput(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	bad := NotificationPreferences{
		UserID:            seed.userID,
		GlobalDNDSchedule: DNDSchedule{Weekdays: []int{9}},
	}
	if err := repo.UpsertNotificationPreferences(ctx, bad); err == nil {
		t.Fatal("expected error for invalid weekday")
	}
}

func TestPostgresNotificationPreferencesCascadeDelete(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	db := openMigratedPostgresTestDB(t)
	seed := seedPostgresMailUser(t, db)
	repo := NewRepository(db)

	if err := repo.UpsertNotificationPreferences(ctx, NotificationPreferences{
		UserID:           seed.userID,
		GlobalDNDEnabled: true,
	}); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := db.ExecContext(ctx, `DELETE FROM users WHERE id = $1::uuid`, seed.userID); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	var count int
	if err := db.QueryRowContext(ctx, `SELECT count(*) FROM notification_preferences WHERE user_id = $1::uuid`, seed.userID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows after user delete, got %d", count)
	}
}
