package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/caldavgw"
)

type fakeCalendarRepo struct {
	calendars []caldavgw.Calendar
	objects   []caldavgw.CalendarObject
	err      error
}

func (f *fakeCalendarRepo) ListCalendars(ctx context.Context, req caldavgw.ListCalendarsRequest) ([]caldavgw.Calendar, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.calendars, nil
}

func (f *fakeCalendarRepo) CreateCalendar(ctx context.Context, req caldavgw.CreateCalendarRequest) (caldavgw.Calendar, error) {
	if f.err != nil {
		return caldavgw.Calendar{}, f.err
	}
	if req.Name == "" {
		return caldavgw.Calendar{}, fmt.Errorf("name is required")
	}
	return caldavgw.Calendar{
		ID:   "cal-1",
		Name: req.Name,
	}, nil
}

func (f *fakeCalendarRepo) GetCalendar(ctx context.Context, req caldavgw.GetCalendarRequest) (caldavgw.Calendar, error) {
	if f.err != nil {
		return caldavgw.Calendar{}, f.err
	}
	for _, cal := range f.calendars {
		if cal.ID == req.CalendarID {
			return cal, nil
		}
	}
	return caldavgw.Calendar{}, fmt.Errorf("calendar not found")
}

func (f *fakeCalendarRepo) UpdateCalendarProperties(ctx context.Context, req caldavgw.UpdateCalendarRequest) (caldavgw.Calendar, error) {
	if f.err != nil {
		return caldavgw.Calendar{}, f.err
	}
	return caldavgw.Calendar{ID: req.CalendarID, Name: "Updated"}, nil
}

func (f *fakeCalendarRepo) DeleteCalendar(ctx context.Context, req caldavgw.DeleteCalendarRequest) (caldavgw.Calendar, error) {
	if f.err != nil {
		return caldavgw.Calendar{}, f.err
	}
	return caldavgw.Calendar{ID: req.CalendarID}, nil
}

func (f *fakeCalendarRepo) ListObjects(ctx context.Context, req caldavgw.ListObjectsRequest) ([]caldavgw.CalendarObject, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.objects, nil
}

func (f *fakeCalendarRepo) GetObject(ctx context.Context, req caldavgw.GetObjectRequest) (caldavgw.CalendarObject, error) {
	if f.err != nil {
		return caldavgw.CalendarObject{}, f.err
	}
	if len(f.objects) > 0 {
		return f.objects[0], nil
	}
	return caldavgw.CalendarObject{ID: "obj-1", ObjectName: req.ObjectName}, nil
}

func (f *fakeCalendarRepo) UpsertObject(ctx context.Context, req caldavgw.UpsertObjectRequest) (caldavgw.CalendarObject, error) {
	if f.err != nil {
		return caldavgw.CalendarObject{}, f.err
	}
	return caldavgw.CalendarObject{ID: "obj-1", ObjectName: req.ObjectName, ETag: "etag-1"}, nil
}

func (f *fakeCalendarRepo) DeleteObject(ctx context.Context, req caldavgw.DeleteObjectRequest) (caldavgw.CalendarObject, error) {
	if f.err != nil {
		return caldavgw.CalendarObject{}, f.err
	}
	return caldavgw.CalendarObject{ID: "obj-1", ObjectName: req.ObjectName}, nil
}

type calendarHandlerForTest struct {
	repo *fakeCalendarRepo
}

func TestCalendarListCalendars(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendars?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/calendars: got status %d, want 200", rec.Code)
	}
}

func TestCalendarCreateRequestValidation(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	body := `{"name":""}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/calendars?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("POST /api/v1/calendars with empty name: got status %d, want 400", rec.Code)
	}
}

func TestCalendarGetNotFound(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendars/nonexistent?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("GET /api/v1/calendars/nonexistent: got status %d, want 404", rec.Code)
	}
}

func TestCalendarUpdateRequestValidation(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	body := `{"name":"Updated"}`
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/calendars/cal-1?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH /api/v1/calendars/cal-1: got status %d, want 200", rec.Code)
	}
}

func TestCalendarDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/calendars/cal-1?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/v1/calendars/cal-1: got status %d, want 204", rec.Code)
	}
}

func TestCalendarObjectListSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendars/cal-1/objects?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/calendars/cal-1/objects: got status %d, want 200", rec.Code)
	}
}

func TestCalendarObjectGetSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendars/cal-1/objects/event.ics?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("GET /api/v1/calendars/cal-1/objects/event.ics: got status %d, want 200", rec.Code)
	}
}

func TestCalendarObjectPutSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	body := `BEGIN:VCALENDAR\r\nVERSION:2.0\r\nEND:VCALENDAR`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/calendars/cal-1/objects/event.ics?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/calendar")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("PUT /api/v1/calendars/cal-1/objects/event.ics: got status %d, want 200", rec.Code)
	}
}

func TestCalendarObjectPutInvalidContentType(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	body := `not ical`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/calendars/cal-1/objects/event.ics?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("PUT with invalid content type: got status %d, want 415", rec.Code)
	}
}

func TestCalendarObjectDeleteSuccess(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{repo: &fakeCalendarRepo{}}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/calendars/cal-1/objects/event.ics?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("DELETE /api/v1/calendars/cal-1/objects/event.ics: got status %d, want 204", rec.Code)
	}
}

func TestCalendarEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := CalendarEnvelope{
		Calendar: caldavgw.Calendar{ID: "cal-1", Name: "Test"},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal CalendarEnvelope: %v", err)
	}
	var out CalendarEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal CalendarEnvelope: %v", err)
	}
	if out.Calendar.ID != "cal-1" || out.Calendar.Name != "Test" {
		t.Fatalf("CalendarEnvelope round-trip: got %+v, want {ID:cal-1 Name:Test}", out.Calendar)
	}
}

func TestCalendarListEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := CalendarListEnvelope{
		Calendars: []caldavgw.Calendar{
			{ID: "cal-1", Name: "Calendar 1"},
			{ID: "cal-2", Name: "Calendar 2"},
		},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal CalendarListEnvelope: %v", err)
	}
	var out CalendarListEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal CalendarListEnvelope: %v", err)
	}
	if len(out.Calendars) != 2 {
		t.Fatalf("CalendarListEnvelope calendars: got %d, want 2", len(out.Calendars))
	}
}

func TestCalendarObjectEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := CalendarObjectEnvelope{
		Object: caldavgw.CalendarObject{ID: "obj-1", ObjectName: "event.ics", ETag: "etag-1"},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal CalendarObjectEnvelope: %v", err)
	}
	var out CalendarObjectEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal CalendarObjectEnvelope: %v", err)
	}
	if out.Object.ETag != "etag-1" {
		t.Fatalf("CalendarObjectEnvelope ETag: got %s, want etag-1", out.Object.ETag)
	}
}

func TestMatchETag(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		ifNoneMatch string
		etag      string
		want      bool
	}{
		{"empty", "", "etag-1", false},
		{"wildcard", "*", "etag-1", true},
		{"exact match", `"etag-1"`, "etag-1", true},
		{"no quotes", "etag-1", "etag-1", true},
		{"mismatch", `"etag-2"`, "etag-1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.Header.Set("If-None-Match", tt.ifNoneMatch)
			got := matchETag(req, tt.etag)
			if got != tt.want {
				t.Errorf("matchETag(%q, %q) = %v, want %v", tt.ifNoneMatch, tt.etag, got, tt.want)
			}
		})
	}
}

func TestCalendarObjectListEnvelopeJSON(t *testing.T) {
	t.Parallel()

	env := CalendarObjectListEnvelope{
		Objects: []caldavgw.CalendarObject{
			{ID: "obj-1", ObjectName: "event1.ics"},
			{ID: "obj-2", ObjectName: "event2.ics"},
		},
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("Marshal CalendarObjectListEnvelope: %v", err)
	}
	var out CalendarObjectListEnvelope
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("Unmarshal CalendarObjectListEnvelope: %v", err)
	}
	if len(out.Objects) != 2 {
		t.Fatalf("CalendarObjectListEnvelope objects: got %d, want 2", len(out.Objects))
	}
}

type failingCalendarRepo struct{}

func (f *failingCalendarRepo) ListCalendars(ctx context.Context, req caldavgw.ListCalendarsRequest) ([]caldavgw.Calendar, error) {
	return nil, io.EOF
}

func TestCalendarListMissingUserID(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	handler := &CalendarHandler{}
	RegisterCalendarRoutes(mux, handler, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/calendars", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("GET /api/v1/calendars without user_id: got status %d, want 400", rec.Code)
	}
}
