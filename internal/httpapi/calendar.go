package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/caldavgw"
)

type CalendarRepo interface {
	ListCalendars(ctx context.Context, req caldavgw.ListCalendarsRequest) ([]caldavgw.Calendar, error)
	CreateCalendar(ctx context.Context, req caldavgw.CreateCalendarRequest) (caldavgw.Calendar, error)
	GetCalendar(ctx context.Context, req caldavgw.GetCalendarRequest) (caldavgw.Calendar, error)
	UpdateCalendarProperties(ctx context.Context, req caldavgw.UpdateCalendarRequest) (caldavgw.Calendar, error)
	DeleteCalendar(ctx context.Context, req caldavgw.DeleteCalendarRequest) (caldavgw.Calendar, error)
	ListObjects(ctx context.Context, req caldavgw.ListObjectsRequest) ([]caldavgw.CalendarObject, error)
	GetObject(ctx context.Context, req caldavgw.GetObjectRequest) (caldavgw.CalendarObject, error)
	UpsertObject(ctx context.Context, req caldavgw.UpsertObjectRequest) (caldavgw.CalendarObject, error)
	DeleteObject(ctx context.Context, req caldavgw.DeleteObjectRequest) (caldavgw.CalendarObject, error)
}

type CalendarPrefsRepo interface {
	GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error)
	SetWebmailPreferences(ctx context.Context, userID string, prefs json.RawMessage) error
}

// CalendarSubscription is a subscribed external iCal/ICS feed.
type CalendarSubscription struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	URL   string `json:"url"`
	Color string `json:"color"`
}

type calendarPrefsEnvelope struct {
	Subscriptions []CalendarSubscription `json:"calendar_subscriptions,omitempty"`
}

type CalendarHandler struct {
	repo  CalendarRepo
	prefs CalendarPrefsRepo
}

func NewCalendarHandler(repo CalendarRepo, prefs CalendarPrefsRepo) *CalendarHandler {
	return &CalendarHandler{repo: repo, prefs: prefs}
}

func (h *CalendarHandler) listSubscriptions(ctx context.Context, userID string) ([]CalendarSubscription, error) {
	raw, err := h.prefs.GetWebmailPreferences(ctx, userID)
	if err != nil {
		return nil, err
	}
	var env calendarPrefsEnvelope
	_ = json.Unmarshal(raw, &env)
	if env.Subscriptions == nil {
		env.Subscriptions = []CalendarSubscription{}
	}
	return env.Subscriptions, nil
}

func (h *CalendarHandler) saveSubscriptions(ctx context.Context, userID string, subs []CalendarSubscription) error {
	raw, err := h.prefs.GetWebmailPreferences(ctx, userID)
	if err != nil {
		return err
	}
	// Merge: preserve other prefs keys, update only calendar_subscriptions.
	var merged map[string]json.RawMessage
	if err := json.Unmarshal(raw, &merged); err != nil {
		merged = make(map[string]json.RawMessage)
	}
	subsJSON, err := json.Marshal(subs)
	if err != nil {
		return err
	}
	merged["calendar_subscriptions"] = subsJSON
	updated, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return h.prefs.SetWebmailPreferences(ctx, userID, updated)
}

type CalendarEnvelope struct {
	Calendar caldavgw.Calendar `json:"calendar"`
}

type CalendarListEnvelope struct {
	Calendars []caldavgw.Calendar `json:"calendars"`
}

type CalendarObjectEnvelope struct {
	Object caldavgw.CalendarObject `json:"object"`
}

type CalendarObjectListEnvelope struct {
	Objects []caldavgw.CalendarObject `json:"objects"`
}

func RegisterCalendarRoutes(mux *http.ServeMux, handler *CalendarHandler, tokenManager *auth.TokenManager) {
	allows := []string{}
	if tokenManager == nil {
		allows = []string{"user_id"}
	}

	mux.HandleFunc("GET /api/v1/calendars", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendars, err := handler.repo.ListCalendars(r.Context(), caldavgw.ListCalendarsRequest{UserID: userID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "calendar list failed")
			return
		}
		writeJSON(w, http.StatusOK, CalendarListEnvelope{Calendars: calendars})
	})

	mux.HandleFunc("POST /api/v1/calendars", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req caldavgw.CreateCalendarRequest
		if err := decodeJSONBody(r, &req); err != nil {
			slog.WarnContext(r.Context(), "decode calendar create request failed", "error", err)
			writeError(w, http.StatusBadRequest, "invalid calendar request")
			return
		}
		req.UserID = userID
		calendar, err := handler.repo.CreateCalendar(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "create calendar failed", "error", err, "user_id", userID)
			writeError(w, http.StatusBadRequest, "invalid calendar request")
			return
		}
		w.Header().Set("Location", fmt.Sprintf("/api/v1/calendars/%s", calendar.ID))
		writeJSON(w, http.StatusCreated, CalendarEnvelope{Calendar: calendar})
	})

	mux.HandleFunc("GET /api/v1/calendars/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		if calendarID == "" {
			writeError(w, http.StatusBadRequest, "calendar id is required")
			return
		}
		calendar, err := handler.repo.GetCalendar(r.Context(), caldavgw.GetCalendarRequest{UserID: userID, CalendarID: calendarID})
		if err != nil {
			writeError(w, http.StatusNotFound, "calendar not found")
			return
		}
		writeJSON(w, http.StatusOK, CalendarEnvelope{Calendar: calendar})
	})

	mux.HandleFunc("PATCH /api/v1/calendars/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		if calendarID == "" {
			writeError(w, http.StatusBadRequest, "calendar id is required")
			return
		}
		var req caldavgw.UpdateCalendarRequest
		if err := decodeJSONBody(r, &req); err != nil {
			slog.WarnContext(r.Context(), "decode calendar update request failed", "error", err)
			writeError(w, http.StatusBadRequest, "invalid calendar request")
			return
		}
		req.UserID = userID
		req.CalendarID = calendarID
		calendar, err := handler.repo.UpdateCalendarProperties(r.Context(), req)
		if err != nil {
			slog.ErrorContext(r.Context(), "update calendar failed", "error", err, "user_id", userID, "calendar_id", calendarID)
			writeError(w, http.StatusBadRequest, "invalid calendar request")
			return
		}
		writeJSON(w, http.StatusOK, CalendarEnvelope{Calendar: calendar})
	})

	mux.HandleFunc("DELETE /api/v1/calendars/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		if calendarID == "" {
			writeError(w, http.StatusBadRequest, "calendar id is required")
			return
		}
		_, err := handler.repo.DeleteCalendar(r.Context(), caldavgw.DeleteCalendarRequest{UserID: userID, CalendarID: calendarID})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("delete calendar: %w", err).Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/calendars/{id}/objects", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		if calendarID == "" {
			writeError(w, http.StatusBadRequest, "calendar id is required")
			return
		}
		objects, err := handler.repo.ListObjects(r.Context(), caldavgw.ListObjectsRequest{UserID: userID, CalendarID: calendarID})
		if err != nil {
			writeError(w, http.StatusInternalServerError, "calendar object list failed")
			return
		}
		writeJSON(w, http.StatusOK, CalendarObjectListEnvelope{Objects: objects})
	})

	mux.HandleFunc("GET /api/v1/calendars/{id}/objects/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		objectName := r.PathValue("name")
		if calendarID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "calendar id and object name are required")
			return
		}
		object, err := handler.repo.GetObject(r.Context(), caldavgw.GetObjectRequest{UserID: userID, CalendarID: calendarID, ObjectName: objectName})
		if err != nil {
			writeError(w, http.StatusNotFound, "calendar object not found")
			return
		}
		if matchETag(r, object.ETag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, object.ETag))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusOK)
		w.Write(object.ICS)
	})

	mux.HandleFunc("PUT /api/v1/calendars/{id}/objects/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		objectName := r.PathValue("name")
		if calendarID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "calendar id and object name are required")
			return
		}
		contentType := r.Header.Get("Content-Type")
		if contentType != "text/calendar" && contentType != "application/ics" {
			writeError(w, http.StatusUnsupportedMediaType, "Content-Type must be text/calendar or application/ics")
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 1<<20))
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("read request body: %w", err).Error())
			return
		}
		etag := r.Header.Get("If-Match")
		object, err := handler.repo.UpsertObject(r.Context(), caldavgw.UpsertObjectRequest{
			UserID:       userID,
			CalendarID:   calendarID,
			ObjectName:   objectName,
			ICS:          body,
			ObservedETag: etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("upsert calendar object: %w", err).Error())
			return
		}
		w.Header().Set("ETag", fmt.Sprintf(`"%s"`, object.ETag))
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, CalendarObjectEnvelope{Object: object})
	})

	mux.HandleFunc("DELETE /api/v1/calendars/{id}/objects/{name}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		calendarID := r.PathValue("id")
		objectName := r.PathValue("name")
		if calendarID == "" || objectName == "" {
			writeError(w, http.StatusBadRequest, "calendar id and object name are required")
			return
		}
		etag := r.Header.Get("If-Match")
		_, err := handler.repo.DeleteObject(r.Context(), caldavgw.DeleteObjectRequest{
			UserID:       userID,
			CalendarID:   calendarID,
			ObjectName:   objectName,
			ObservedETag: etag,
		})
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("delete calendar object: %w", err).Error())
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

func RegisterCalendarSubscriptionRoutes(mux *http.ServeMux, handler *CalendarHandler, tokenManager *auth.TokenManager) {
	allows := []string{}
	if tokenManager == nil {
		allows = []string{"user_id"}
	}

	mux.HandleFunc("GET /api/v1/calendar-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		subs, err := handler.listSubscriptions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "subscription list failed")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"subscriptions": subs})
	})

	mux.HandleFunc("POST /api/v1/calendar-subscriptions", func(w http.ResponseWriter, r *http.Request) {
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		var req struct {
			Name  string `json:"name"`
			URL   string `json:"url"`
			Color string `json:"color"`
		}
		if err := decodeJSONBody(r, &req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		req.URL = strings.TrimSpace(req.URL)
		if req.URL == "" {
			writeError(w, http.StatusBadRequest, "url is required")
			return
		}
		u, err := url.Parse(req.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") {
			writeError(w, http.StatusBadRequest, "url must be an http or https URL")
			return
		}
		if req.Name == "" {
			req.Name = u.Host
		}
		if req.Color == "" {
			req.Color = "#4285f4"
		}
		subs, err := handler.listSubscriptions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load subscriptions failed")
			return
		}
		sub := CalendarSubscription{
			ID:    uuid.NewString(),
			Name:  req.Name,
			URL:   req.URL,
			Color: req.Color,
		}
		subs = append(subs, sub)
		if err := handler.saveSubscriptions(r.Context(), userID, subs); err != nil {
			writeError(w, http.StatusInternalServerError, "save subscription failed")
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{"subscription": sub})
	})

	mux.HandleFunc("DELETE /api/v1/calendar-subscriptions/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		subID := r.PathValue("id")
		subs, err := handler.listSubscriptions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load subscriptions failed")
			return
		}
		filtered := subs[:0]
		for _, s := range subs {
			if s.ID != subID {
				filtered = append(filtered, s)
			}
		}
		if err := handler.saveSubscriptions(r.Context(), userID, filtered); err != nil {
			writeError(w, http.StatusInternalServerError, "save subscription failed")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})

	mux.HandleFunc("GET /api/v1/calendar-subscriptions/{id}/events", func(w http.ResponseWriter, r *http.Request) {
		if !rejectBodylessRequestPayload(w, r) {
			return
		}
		if !rejectUnknownQueryKeys(w, r, allows...) {
			return
		}
		userID, ok := userIDFromRequest(w, r, tokenManager)
		if !ok {
			return
		}
		subID := r.PathValue("id")
		subs, err := handler.listSubscriptions(r.Context(), userID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "load subscriptions failed")
			return
		}
		var target *CalendarSubscription
		for i := range subs {
			if subs[i].ID == subID {
				target = &subs[i]
				break
			}
		}
		if target == nil {
			writeError(w, http.StatusNotFound, "subscription not found")
			return
		}
		client := &http.Client{Timeout: 15 * time.Second}
		resp, err := client.Get(target.URL)
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to fetch subscription URL")
			return
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20)) // 5 MB limit
		if err != nil {
			writeError(w, http.StatusBadGateway, "failed to read subscription response")
			return
		}
		w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
		w.Header().Set("Cache-Control", "max-age=3600")
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})
}

func matchETag(r *http.Request, etag string) bool {
	ifNoneMatch := r.Header.Get("If-None-Match")
	if ifNoneMatch == "" {
		return false
	}
	if ifNoneMatch == "*" {
		return true
	}
	return ifNoneMatch == fmt.Sprintf(`"%s"`, etag) || ifNoneMatch == etag
}
