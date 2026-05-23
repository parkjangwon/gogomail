package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

// NotificationPreferenceService is the persistence abstraction used by the
// notification-preference HTTP handlers. The default implementation
// (NewMaildbNotificationPreferenceAdapter) delegates to *maildb.Repository.
type NotificationPreferenceService interface {
	GetNotificationPreferences(ctx context.Context, userID string) (*maildb.NotificationPreferences, error)
	UpsertNotificationPreferences(ctx context.Context, prefs maildb.NotificationPreferences) error
}

// MaildbNotificationPreferenceAdapter adapts *maildb.Repository to NotificationPreferenceService.
type MaildbNotificationPreferenceAdapter struct {
	r *maildb.Repository
}

// NewMaildbNotificationPreferenceAdapter wraps a repository for use as a NotificationPreferenceService.
func NewMaildbNotificationPreferenceAdapter(r *maildb.Repository) *MaildbNotificationPreferenceAdapter {
	return &MaildbNotificationPreferenceAdapter{r: r}
}

func (a *MaildbNotificationPreferenceAdapter) GetNotificationPreferences(ctx context.Context, userID string) (*maildb.NotificationPreferences, error) {
	return a.r.GetNotificationPreferences(ctx, userID)
}

func (a *MaildbNotificationPreferenceAdapter) UpsertNotificationPreferences(ctx context.Context, prefs maildb.NotificationPreferences) error {
	return a.r.UpsertNotificationPreferences(ctx, prefs)
}

// notificationPreferencePUTRequest mirrors the wire schema for PUT bodies. It is
// kept separate from maildb.NotificationPreferences so DisallowUnknownFields
// can reject unexpected keys at the API boundary.
type notificationPreferencePUTRequest struct {
	GlobalDNDEnabled  bool                                      `json:"global_dnd_enabled"`
	GlobalDNDSchedule notificationPrefDNDScheduleDTO            `json:"global_dnd_schedule"`
	FolderOverrides   map[string]notificationPrefFolderOverride `json:"folder_overrides"`
}

type notificationPrefDNDScheduleDTO struct {
	Weekdays   []int                       `json:"weekdays"`
	TimeRanges []notificationPrefTimeRange `json:"time_ranges"`
	Timezone   string                      `json:"timezone"`
}

type notificationPrefTimeRange struct {
	Start string `json:"start"`
	End   string `json:"end"`
}

type notificationPrefFolderOverride struct {
	Enabled     bool                           `json:"enabled"`
	DNDInherit  bool                           `json:"dnd_inherit"`
	DNDSchedule notificationPrefDNDScheduleDTO `json:"dnd_schedule"`
}

func (s notificationPrefDNDScheduleDTO) toMaildb() maildb.DNDSchedule {
	ranges := make([]maildb.TimeRange, 0, len(s.TimeRanges))
	for _, r := range s.TimeRanges {
		ranges = append(ranges, maildb.TimeRange{Start: r.Start, End: r.End})
	}
	return maildb.DNDSchedule{
		Weekdays:   append([]int(nil), s.Weekdays...),
		TimeRanges: ranges,
		Timezone:   s.Timezone,
	}
}

func dndScheduleFromMaildb(s maildb.DNDSchedule) notificationPrefDNDScheduleDTO {
	ranges := make([]notificationPrefTimeRange, 0, len(s.TimeRanges))
	for _, r := range s.TimeRanges {
		ranges = append(ranges, notificationPrefTimeRange{Start: r.Start, End: r.End})
	}
	return notificationPrefDNDScheduleDTO{
		Weekdays:   append([]int(nil), s.Weekdays...),
		TimeRanges: ranges,
		Timezone:   s.Timezone,
	}
}

func notificationPrefsResponseBody(prefs *maildb.NotificationPreferences) map[string]any {
	folders := make(map[string]notificationPrefFolderOverride, len(prefs.FolderOverrides))
	for id, o := range prefs.FolderOverrides {
		folders[id] = notificationPrefFolderOverride{
			Enabled:     o.Enabled,
			DNDInherit:  o.DNDInherit,
			DNDSchedule: dndScheduleFromMaildb(o.DNDSchedule),
		}
	}
	body := map[string]any{
		"global_dnd_enabled":  prefs.GlobalDNDEnabled,
		"global_dnd_schedule": dndScheduleFromMaildb(prefs.GlobalDNDSchedule),
		"folder_overrides":    folders,
	}
	if !prefs.UpdatedAt.IsZero() {
		body["updated_at"] = prefs.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return body
}

// RegisterNotificationPreferenceRoutes wires up the per-user notification
// preference endpoints onto mux.
//
//	GET /api/v1/me/notification-preferences
//	PUT /api/v1/me/notification-preferences
//
// Authentication uses tokenManager (JWT) and, when present, the API-key context
// just like the rest of the mail API. PUT requests are rate-limited per user
// (30 req/min) to bound database write load from a single account.
func RegisterNotificationPreferenceRoutes(mux *http.ServeMux, service NotificationPreferenceService, tokenManager *auth.TokenManager) {
	if service == nil {
		return
	}
	putLimiter := NewAdminIPRateLimiter(30, time.Minute)

	mux.HandleFunc("GET /api/v1/me/notification-preferences", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		prefs, err := service.GetNotificationPreferences(r.Context(), userID)
		if err != nil {
			slog.Warn("get notification preferences failed", "err", err, "user_id", userID)
			writeError(w, http.StatusInternalServerError, "failed to load notification preferences")
			return
		}
		writeJSON(w, http.StatusOK, notificationPrefsResponseBody(prefs))
	})

	mux.HandleFunc("PUT /api/v1/me/notification-preferences", func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		if !rejectUnknownQueryKeys(w, r) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		if !putLimiter.allow(userID) {
			w.Header().Set("Retry-After", "60")
			writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
			return
		}

		var req notificationPreferencePUTRequest
		if err := decodeJSONBody(r, &req); err != nil {
			slog.Info("notification preferences decode failed", "err", err, "user_id", userID)
			writeError(w, http.StatusBadRequest, "invalid notification preferences")
			return
		}

		folders := make(map[string]maildb.FolderNotificationOverride, len(req.FolderOverrides))
		for id, o := range req.FolderOverrides {
			folders[id] = maildb.FolderNotificationOverride{
				Enabled:     o.Enabled,
				DNDInherit:  o.DNDInherit,
				DNDSchedule: o.DNDSchedule.toMaildb(),
			}
		}
		prefs := maildb.NotificationPreferences{
			UserID:            userID,
			GlobalDNDEnabled:  req.GlobalDNDEnabled,
			GlobalDNDSchedule: req.GlobalDNDSchedule.toMaildb(),
			FolderOverrides:   folders,
		}
		if err := service.UpsertNotificationPreferences(r.Context(), prefs); err != nil {
			slog.Info("notification preferences upsert failed", "err", err, "user_id", userID)
			// Validation errors and storage errors both map to a static client message
			// to avoid leaking internal details. The detailed reason is logged above.
			writeError(w, http.StatusBadRequest, "invalid notification preferences")
			return
		}
		updated, err := service.GetNotificationPreferences(r.Context(), userID)
		if err != nil {
			slog.Warn("reload notification preferences after upsert failed", "err", err, "user_id", userID)
			writeError(w, http.StatusInternalServerError, "failed to load notification preferences")
			return
		}
		writeJSON(w, http.StatusOK, notificationPrefsResponseBody(updated))
	})
}
