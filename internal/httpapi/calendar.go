package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

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

type CalendarHandler struct {
	repo CalendarRepo
}

func NewCalendarHandler(repo CalendarRepo) *CalendarHandler {
	return &CalendarHandler{repo: repo}
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
		if r.ContentLength > maxJSONBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		var req caldavgw.CreateCalendarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode calendar create request: %w", err).Error())
			return
		}
		req.UserID = userID
		calendar, err := handler.repo.CreateCalendar(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("create calendar: %w", err).Error())
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
		if r.ContentLength > maxJSONBodyBytes {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return
		}
		var req caldavgw.UpdateCalendarRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("decode calendar update request: %w", err).Error())
			return
		}
		req.UserID = userID
		req.CalendarID = calendarID
		calendar, err := handler.repo.UpdateCalendarProperties(r.Context(), req)
		if err != nil {
			writeError(w, http.StatusBadRequest, fmt.Errorf("update calendar: %w", err).Error())
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
