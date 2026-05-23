package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

type fakeNotificationPrefService struct {
	mu          sync.Mutex
	prefsByUser map[string]*maildb.NotificationPreferences
	upsertErr   error
	getErr      error
	getCalls    int
	upsertCalls int
}

func newFakeNotificationPrefService() *fakeNotificationPrefService {
	return &fakeNotificationPrefService{prefsByUser: map[string]*maildb.NotificationPreferences{}}
}

func (f *fakeNotificationPrefService) GetNotificationPreferences(_ context.Context, userID string) (*maildb.NotificationPreferences, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.getCalls++
	if f.getErr != nil {
		return nil, f.getErr
	}
	if p, ok := f.prefsByUser[userID]; ok {
		copy := *p
		return &copy, nil
	}
	return &maildb.NotificationPreferences{
		UserID:          userID,
		FolderOverrides: map[string]maildb.FolderNotificationOverride{},
	}, nil
}

func (f *fakeNotificationPrefService) UpsertNotificationPreferences(_ context.Context, prefs maildb.NotificationPreferences) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.upsertCalls++
	if f.upsertErr != nil {
		return f.upsertErr
	}
	if _, err := maildb.ValidateNotificationPreferences(prefs); err != nil {
		return err
	}
	saved := prefs
	saved.UpdatedAt = time.Now().UTC()
	f.prefsByUser[prefs.UserID] = &saved
	return nil
}

const notifTestUserID = "11111111-2222-3333-4444-555555555555"
const notifTestSecret = "test-secret-httpapi-notif-at-least-32b"

func notifTestMux(t *testing.T) (*http.ServeMux, *auth.TokenManager, *fakeNotificationPrefService) {
	t.Helper()
	manager, err := auth.NewTokenManager(notifTestSecret)
	if err != nil {
		t.Fatalf("NewTokenManager: %v", err)
	}
	service := newFakeNotificationPrefService()
	mux := http.NewServeMux()
	RegisterNotificationPreferenceRoutes(mux, service, manager)
	return mux, manager, service
}

func notifTestToken(t *testing.T, manager *auth.TokenManager, userID string) string {
	t.Helper()
	tok, err := manager.Sign(auth.Claims{UserID: userID}, time.Minute)
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return tok
}

func TestNotificationPreferencesGetReturnsDefault(t *testing.T) {
	t.Parallel()
	mux, manager, _ := notifTestMux(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/notification-preferences", nil)
	req.Header.Set("Authorization", "Bearer "+notifTestToken(t, manager, notifTestUserID))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["global_dnd_enabled"] != false {
		t.Fatalf("global_dnd_enabled = %v, want false", body["global_dnd_enabled"])
	}
	if _, ok := body["folder_overrides"]; !ok {
		t.Fatal("missing folder_overrides")
	}
}

func TestNotificationPreferencesPutThenGetRoundTrip(t *testing.T) {
	t.Parallel()
	mux, manager, _ := notifTestMux(t)
	token := notifTestToken(t, manager, notifTestUserID)

	const folderID = "aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee"
	putBody := `{
		"global_dnd_enabled": true,
		"global_dnd_schedule": {
			"weekdays": [0,6],
			"time_ranges": [{"start": "22:00", "end": "08:00"}],
			"timezone": "Asia/Seoul"
		},
		"folder_overrides": {
			"` + folderID + `": {"enabled": false, "dnd_inherit": true, "dnd_schedule": {}}
		}
	}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notification-preferences", strings.NewReader(putBody))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT status = %d, body = %s", rec.Code, rec.Body.String())
	}

	getReq := httptest.NewRequest(http.MethodGet, "/api/v1/me/notification-preferences", nil)
	getReq.Header.Set("Authorization", "Bearer "+token)
	getRec := httptest.NewRecorder()
	mux.ServeHTTP(getRec, getReq)
	if getRec.Code != http.StatusOK {
		t.Fatalf("GET status = %d, body = %s", getRec.Code, getRec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(getRec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["global_dnd_enabled"] != true {
		t.Fatalf("global_dnd_enabled = %v, want true", body["global_dnd_enabled"])
	}
	folders, ok := body["folder_overrides"].(map[string]any)
	if !ok {
		t.Fatalf("folder_overrides type = %T", body["folder_overrides"])
	}
	if _, ok := folders[folderID]; !ok {
		t.Fatalf("missing folder override for %s", folderID)
	}
}

func TestNotificationPreferencesPutRejectsMalformed(t *testing.T) {
	t.Parallel()
	mux, manager, _ := notifTestMux(t)
	token := notifTestToken(t, manager, notifTestUserID)

	cases := []struct {
		name string
		body string
	}{
		{"bad weekday", `{"global_dnd_schedule":{"weekdays":[9]}}`},
		{"bad time", `{"global_dnd_schedule":{"time_ranges":[{"start":"99:99","end":"00:00"}]}}`},
		{"bad tz", `{"global_dnd_schedule":{"timezone":"Mars/Olympus"}}`},
		{"bad folder id", `{"folder_overrides":{"not-a-uuid":{"enabled":true,"dnd_inherit":true,"dnd_schedule":{}}}}`},
		{"unknown field", `{"global_dnd_enabled":true,"unknown":1}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notification-preferences", strings.NewReader(c.body))
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, req)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "invalid notification preferences") {
				t.Fatalf("body should carry static error, got %s", rec.Body.String())
			}
		})
	}
}

func TestNotificationPreferencesRequireAuth(t *testing.T) {
	t.Parallel()
	mux, _, _ := notifTestMux(t)

	cases := []struct {
		method string
		body   string
	}{
		{http.MethodGet, ""},
		{http.MethodPut, `{}`},
	}
	for _, c := range cases {
		var req *http.Request
		if c.body == "" {
			req = httptest.NewRequest(c.method, "/api/v1/me/notification-preferences", nil)
		} else {
			req = httptest.NewRequest(c.method, "/api/v1/me/notification-preferences", strings.NewReader(c.body))
			req.Header.Set("Content-Type", "application/json")
		}
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("%s status = %d, want 401, body = %s", c.method, rec.Code, rec.Body.String())
		}
	}
}

func TestNotificationPreferencesPutRateLimited(t *testing.T) {
	t.Parallel()
	mux, manager, _ := notifTestMux(t)
	token := notifTestToken(t, manager, notifTestUserID)

	body := `{"global_dnd_enabled":false}`
	var lastCode int
	// 30 should succeed, the 31st should be 429.
	for i := 0; i < 31; i++ {
		req := httptest.NewRequest(http.MethodPut, "/api/v1/me/notification-preferences", strings.NewReader(body))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		lastCode = rec.Code
		if i < 30 && rec.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, body = %s", i, rec.Code, rec.Body.String())
		}
	}
	if lastCode != http.StatusTooManyRequests {
		t.Fatalf("final status = %d, want 429", lastCode)
	}
}
