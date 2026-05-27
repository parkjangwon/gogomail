package maildb

import (
	"errors"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
)

// DNDSchedule represents a Do-Not-Disturb schedule expressed in a local IANA timezone.
//
// A schedule "matches" (DND is active) when the current local time falls inside any of
// the configured TimeRanges on a weekday listed in Weekdays. A TimeRange whose End is
// less than or equal to Start is interpreted as crossing midnight.
type DNDSchedule struct {
	Weekdays   []int       `json:"weekdays"`    // 0-6, Sun=0
	TimeRanges []TimeRange `json:"time_ranges"` // each "HH:MM-HH:MM"
	Timezone   string      `json:"timezone"`    // IANA tz; empty => UTC
}

// TimeRange is a local time-of-day range. Times use 24-hour "HH:MM" format.
// When End <= Start, the range crosses midnight (e.g. {22:00, 08:00}).
type TimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

// FolderNotificationOverride captures per-folder notification settings.
type FolderNotificationOverride struct {
	Enabled     bool        `json:"enabled"`
	DNDInherit  bool        `json:"dnd_inherit"`
	DNDSchedule DNDSchedule `json:"dnd_schedule"`
}

// ThreadNotificationOverride captures per-thread notification settings.
type ThreadNotificationOverride struct {
	Enabled bool `json:"enabled"`
}

// NotificationPreferences represents a user's full notification preference document.
type NotificationPreferences struct {
	UserID            string                                `json:"user_id"`
	GlobalDNDEnabled  bool                                  `json:"global_dnd_enabled"`
	GlobalDNDSchedule DNDSchedule                           `json:"global_dnd_schedule"`
	FolderOverrides   map[string]FolderNotificationOverride `json:"folder_overrides"`
	ThreadOverrides   map[string]ThreadNotificationOverride `json:"thread_overrides"`
	UpdatedAt         time.Time                             `json:"updated_at"`
}

const (
	maxNotificationTimeRanges    = 8
	maxNotificationFolderEntries = 200
	maxNotificationThreadEntries = 500
)

var (
	hhmmRegexp = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	uuidRegexp = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)
)

// ValidateDNDSchedule validates a DNDSchedule. Empty schedules (zero value) are valid.
// On success the returned schedule has its timezone normalized (empty -> "UTC") and
// weekdays sorted, deduped.
func ValidateDNDSchedule(s DNDSchedule) (DNDSchedule, error) {
	if len(s.TimeRanges) > maxNotificationTimeRanges {
		return s, fmt.Errorf("too many time ranges (max %d)", maxNotificationTimeRanges)
	}
	seen := make(map[int]struct{}, len(s.Weekdays))
	weekdays := make([]int, 0, len(s.Weekdays))
	for _, w := range s.Weekdays {
		if w < 0 || w > 6 {
			return s, fmt.Errorf("weekday %d out of range (0-6)", w)
		}
		if _, dup := seen[w]; dup {
			return s, fmt.Errorf("weekday %d duplicated", w)
		}
		seen[w] = struct{}{}
		weekdays = append(weekdays, w)
	}
	sort.Ints(weekdays)

	for i, r := range s.TimeRanges {
		if !hhmmRegexp.MatchString(r.Start) {
			return s, fmt.Errorf("time_ranges[%d].start invalid (want HH:MM)", i)
		}
		if !hhmmRegexp.MatchString(r.End) {
			return s, fmt.Errorf("time_ranges[%d].end invalid (want HH:MM)", i)
		}
	}

	tz := strings.TrimSpace(s.Timezone)
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return s, fmt.Errorf("invalid timezone %q: %w", s.Timezone, err)
	}

	return DNDSchedule{
		Weekdays:   weekdays,
		TimeRanges: append([]TimeRange(nil), s.TimeRanges...),
		Timezone:   tz,
	}, nil
}

// ValidateNotificationPreferences validates and normalizes prefs. The returned
// value is safe to persist; the original is not mutated.
func ValidateNotificationPreferences(prefs NotificationPreferences) (NotificationPreferences, error) {
	if strings.TrimSpace(prefs.UserID) == "" {
		return prefs, fmt.Errorf("user_id is required")
	}
	if !uuidRegexp.MatchString(prefs.UserID) {
		return prefs, fmt.Errorf("user_id must be a uuid")
	}

	global, err := ValidateDNDSchedule(prefs.GlobalDNDSchedule)
	if err != nil {
		return prefs, fmt.Errorf("global_dnd_schedule: %w", err)
	}

	if len(prefs.FolderOverrides) > maxNotificationFolderEntries {
		return prefs, fmt.Errorf("folder_overrides exceeds limit (%d)", maxNotificationFolderEntries)
	}
	if len(prefs.ThreadOverrides) > maxNotificationThreadEntries {
		return prefs, fmt.Errorf("thread_overrides exceeds limit (%d)", maxNotificationThreadEntries)
	}

	normalizedFolders := make(map[string]FolderNotificationOverride, len(prefs.FolderOverrides))
	for folderID, override := range prefs.FolderOverrides {
		if !uuidRegexp.MatchString(folderID) {
			return prefs, fmt.Errorf("folder_overrides key %q is not a uuid", folderID)
		}
		// Always validate the schedule (it may be empty / inherited).
		sched, err := ValidateDNDSchedule(override.DNDSchedule)
		if err != nil {
			return prefs, fmt.Errorf("folder_overrides[%s].dnd_schedule: %w", folderID, err)
		}
		normalizedFolders[folderID] = FolderNotificationOverride{
			Enabled:     override.Enabled,
			DNDInherit:  override.DNDInherit,
			DNDSchedule: sched,
		}
	}
	normalizedThreads := make(map[string]ThreadNotificationOverride, len(prefs.ThreadOverrides))
	for threadID, override := range prefs.ThreadOverrides {
		if !uuidRegexp.MatchString(threadID) {
			return prefs, fmt.Errorf("thread_overrides key %q is not a uuid", threadID)
		}
		normalizedThreads[threadID] = ThreadNotificationOverride{Enabled: override.Enabled}
	}

	return NotificationPreferences{
		UserID:            prefs.UserID,
		GlobalDNDEnabled:  prefs.GlobalDNDEnabled,
		GlobalDNDSchedule: global,
		FolderOverrides:   normalizedFolders,
		ThreadOverrides:   normalizedThreads,
		UpdatedAt:         prefs.UpdatedAt,
	}, nil
}

// GetNotificationPreferences returns the user's prefs, or a zero-value record
// (with default empty schedule and no folder overrides) when no row exists.
func (r *Repository) GetNotificationPreferences(ctx context.Context, userID string) (*NotificationPreferences, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	if !uuidRegexp.MatchString(userID) {
		return nil, fmt.Errorf("user_id must be a uuid")
	}

	const query = `
SELECT
  global_dnd_enabled,
  COALESCE(global_dnd_schedule, '{}'::jsonb),
  COALESCE(folder_overrides, '{}'::jsonb),
  COALESCE(thread_overrides, '{}'::jsonb),
  updated_at
FROM notification_preferences
WHERE user_id = $1::uuid`

	var (
		globalEnabled  bool
		globalSchedRaw []byte
		folderOverRaw  []byte
		threadOverRaw  []byte
		updatedAt      time.Time
	)
	err := r.db.QueryRowContext(ctx, query, userID).Scan(&globalEnabled, &globalSchedRaw, &folderOverRaw, &threadOverRaw, &updatedAt)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return &NotificationPreferences{
				UserID:          userID,
				FolderOverrides: map[string]FolderNotificationOverride{},
				ThreadOverrides: map[string]ThreadNotificationOverride{},
			}, nil
		}
		return nil, fmt.Errorf("get notification preferences: %w", err)
	}

	out := &NotificationPreferences{
		UserID:           userID,
		GlobalDNDEnabled: globalEnabled,
		FolderOverrides:  map[string]FolderNotificationOverride{},
		ThreadOverrides:  map[string]ThreadNotificationOverride{},
		UpdatedAt:        updatedAt,
	}
	if len(globalSchedRaw) > 0 {
		if err := json.Unmarshal(globalSchedRaw, &out.GlobalDNDSchedule); err != nil {
			return nil, fmt.Errorf("decode global_dnd_schedule: %w", err)
		}
	}
	if len(folderOverRaw) > 0 {
		if err := json.Unmarshal(folderOverRaw, &out.FolderOverrides); err != nil {
			return nil, fmt.Errorf("decode folder_overrides: %w", err)
		}
		if out.FolderOverrides == nil {
			out.FolderOverrides = map[string]FolderNotificationOverride{}
		}
	}
	if len(threadOverRaw) > 0 {
		if err := json.Unmarshal(threadOverRaw, &out.ThreadOverrides); err != nil {
			return nil, fmt.Errorf("decode thread_overrides: %w", err)
		}
		if out.ThreadOverrides == nil {
			out.ThreadOverrides = map[string]ThreadNotificationOverride{}
		}
	}
	return out, nil
}

// UpsertNotificationPreferences validates prefs and persists them, replacing any
// existing row for the user. updated_at is set to now() by the database.
func (r *Repository) UpsertNotificationPreferences(ctx context.Context, prefs NotificationPreferences) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	normalized, err := ValidateNotificationPreferences(prefs)
	if err != nil {
		return err
	}

	globalRaw, err := json.Marshal(normalized.GlobalDNDSchedule)
	if err != nil {
		return fmt.Errorf("marshal global_dnd_schedule: %w", err)
	}
	folderRaw, err := json.Marshal(normalized.FolderOverrides)
	if err != nil {
		return fmt.Errorf("marshal folder_overrides: %w", err)
	}
	threadRaw, err := json.Marshal(normalized.ThreadOverrides)
	if err != nil {
		return fmt.Errorf("marshal thread_overrides: %w", err)
	}

	const query = `
INSERT INTO notification_preferences (user_id, global_dnd_enabled, global_dnd_schedule, folder_overrides, thread_overrides, updated_at)
VALUES ($1::uuid, $2, $3::jsonb, $4::jsonb, $5::jsonb, now())
ON CONFLICT (user_id) DO UPDATE
SET global_dnd_enabled = EXCLUDED.global_dnd_enabled,
    global_dnd_schedule = EXCLUDED.global_dnd_schedule,
    folder_overrides = EXCLUDED.folder_overrides,
    thread_overrides = EXCLUDED.thread_overrides,
    updated_at = now()`

	if _, err := r.db.ExecContext(ctx, query, normalized.UserID, normalized.GlobalDNDEnabled, string(globalRaw), string(folderRaw), string(threadRaw)); err != nil {
		return fmt.Errorf("upsert notification preferences: %w", err)
	}
	return nil
}
