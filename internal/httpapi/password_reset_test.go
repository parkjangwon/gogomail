package httpapi

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// --- test double ---

type fakePasswordResetStore struct {
	users  map[string]string // email → userID
	tokens map[string]*PasswordResetTokenRecord

	lastCreatedUserID string
	lastMarkedUsedID  string
	lastResetUserID   string
	respectContext    bool
	lookupCtxErrs     chan error
	createdUserIDs    chan string
}

func (f *fakePasswordResetStore) GetUserIDByEmail(ctx context.Context, email string) (string, string, error) {
	if f.lookupCtxErrs != nil {
		f.lookupCtxErrs <- ctx.Err()
	}
	if f.respectContext {
		if err := ctx.Err(); err != nil {
			return "", "", err
		}
	}
	id, ok := f.users[email]
	if !ok {
		return "", "", context.DeadlineExceeded
	}
	return id, email, nil
}

func (f *fakePasswordResetStore) CreatePasswordResetToken(_ context.Context, userID string, _ []byte, _ time.Time) error {
	f.lastCreatedUserID = userID
	if f.createdUserIDs != nil {
		f.createdUserIDs <- userID
	}
	return nil
}

func (f *fakePasswordResetStore) GetPasswordResetToken(_ context.Context, tokenHash []byte) (*PasswordResetTokenRecord, error) {
	key := hex.EncodeToString(tokenHash)
	rec, ok := f.tokens[key]
	if !ok {
		return nil, context.DeadlineExceeded
	}
	return rec, nil
}

func (f *fakePasswordResetStore) MarkTokenUsed(_ context.Context, tokenID string) error {
	f.lastMarkedUsedID = tokenID
	return nil
}

func (f *fakePasswordResetStore) ResetUserPassword(_ context.Context, userID, _ string) error {
	f.lastResetUserID = userID
	return nil
}

// --- helpers ---

func newTestPasswordResetMux(store PasswordResetStore) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterPasswordResetRoutes(mux, store, nil, "https://mail.example.com")
	return mux
}

func postJSONPasswordReset(mux *http.ServeMux, path string, body any) *httptest.ResponseRecorder {
	b, _ := json.Marshal(body)
	req := httptest.NewRequest("POST", path, bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	return w
}

func goodRawToken() []byte {
	b := make([]byte, 32)
	for i := range b {
		b[i] = byte(i + 1)
	}
	return b
}

func tokenHashHex(raw []byte) string {
	h := sha256.Sum256(raw)
	return hex.EncodeToString(h[:])
}

func makeStoreWithToken(rawToken []byte, expired bool, used bool) *fakePasswordResetStore {
	expiresAt := time.Now().Add(time.Hour)
	if expired {
		expiresAt = time.Now().Add(-time.Hour)
	}
	var usedAt *time.Time
	if used {
		t := time.Now().Add(-time.Minute)
		usedAt = &t
	}

	return &fakePasswordResetStore{
		users: map[string]string{"alice@example.com": "user-1"},
		tokens: map[string]*PasswordResetTokenRecord{
			tokenHashHex(rawToken): {
				ID:        "token-id-1",
				UserID:    "user-1",
				ExpiresAt: expiresAt,
				UsedAt:    usedAt,
			},
		},
	}
}

// --- request tests ---

func TestPasswordResetRequest_AlwaysReturns200(t *testing.T) {
	t.Parallel()

	store := &fakePasswordResetStore{
		users: map[string]string{"alice@example.com": "user-1"},
	}
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/request", map[string]string{"email": "alice@example.com"})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	// unknown email must also return 200 (no enumeration)
	w = postJSONPasswordReset(mux, "/api/v1/auth/password-reset/request", map[string]string{"email": "nobody@example.com"})
	if w.Code != 200 {
		t.Fatalf("expected 200 for unknown email, got %d", w.Code)
	}
}

func TestPasswordResetRequest_MissingEmail(t *testing.T) {
	t.Parallel()
	mux := newTestPasswordResetMux(&fakePasswordResetStore{})
	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/request", map[string]string{})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestPasswordResetRequest_BackgroundIssueSurvivesRequestCancellation(t *testing.T) {
	t.Parallel()

	store := &fakePasswordResetStore{
		users:          map[string]string{"alice@example.com": "user-1"},
		respectContext: true,
		lookupCtxErrs:  make(chan error, 1),
		createdUserIDs: make(chan string, 1),
	}
	mux := newTestPasswordResetMux(store)
	body, _ := json.Marshal(map[string]string{"email": "alice@example.com"})
	req := httptest.NewRequest("POST", "/api/v1/auth/password-reset/request", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx, cancel := context.WithCancel(req.Context())
	cancel()
	req = req.WithContext(ctx)

	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	select {
	case err := <-store.lookupCtxErrs:
		if err != nil {
			t.Fatalf("background issue used canceled request context: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for password reset lookup")
	}
	select {
	case userID := <-store.createdUserIDs:
		if userID != "user-1" {
			t.Fatalf("created token for %q, want user-1", userID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for password reset token creation")
	}
}

// --- confirm tests ---

func TestPasswordResetConfirm_Success(t *testing.T) {
	t.Parallel()

	rawToken := goodRawToken()
	store := makeStoreWithToken(rawToken, false, false)
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        hex.EncodeToString(rawToken),
		"new_password": "newSecurePassword123",
	})
	if w.Code != 200 {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if store.lastResetUserID != "user-1" {
		t.Fatalf("expected reset for user-1, got %q", store.lastResetUserID)
	}
	if store.lastMarkedUsedID != "token-id-1" {
		t.Fatalf("expected mark used for token-id-1, got %q", store.lastMarkedUsedID)
	}
}

func TestPasswordResetConfirm_ExpiredToken(t *testing.T) {
	t.Parallel()

	rawToken := goodRawToken()
	store := makeStoreWithToken(rawToken, true, false)
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        hex.EncodeToString(rawToken),
		"new_password": "newSecurePassword123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for expired token, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "expired") {
		t.Fatalf("expected 'expired' in response, got %s", w.Body.String())
	}
}

func TestPasswordResetConfirm_UsedToken(t *testing.T) {
	t.Parallel()

	rawToken := goodRawToken()
	store := makeStoreWithToken(rawToken, false, true)
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        hex.EncodeToString(rawToken),
		"new_password": "newSecurePassword123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for used token, got %d", w.Code)
	}
}

func TestPasswordResetConfirm_ShortPassword(t *testing.T) {
	t.Parallel()

	rawToken := goodRawToken()
	store := makeStoreWithToken(rawToken, false, false)
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        hex.EncodeToString(rawToken),
		"new_password": "short",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for short password, got %d: %s", w.Code, w.Body.String())
	}
}

func TestPasswordResetConfirm_InvalidTokenFormat(t *testing.T) {
	t.Parallel()

	store := &fakePasswordResetStore{}
	mux := newTestPasswordResetMux(store)

	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        "not-hex!!",
		"new_password": "newSecurePassword123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid token, got %d", w.Code)
	}
}

func TestPasswordResetConfirm_UnknownToken(t *testing.T) {
	t.Parallel()

	store := &fakePasswordResetStore{tokens: map[string]*PasswordResetTokenRecord{}}
	mux := newTestPasswordResetMux(store)

	rawToken := goodRawToken()
	w := postJSONPasswordReset(mux, "/api/v1/auth/password-reset/confirm", map[string]string{
		"token":        hex.EncodeToString(rawToken),
		"new_password": "newSecurePassword123",
	})
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown token, got %d", w.Code)
	}
}
