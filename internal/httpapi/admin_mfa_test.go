package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

type fakeMFAStore struct {
	setupUserID string
	setupSecret string
	setupCodes  []string
}

func (f *fakeMFAStore) GetUserMFAStatus(context.Context, string) (maildb.UserMFAStatus, error) {
	return maildb.UserMFAStatus{}, nil
}

func (f *fakeMFAStore) GetMFASecret(context.Context, string) (string, []string, error) {
	return "", nil, maildb.ErrMFANotEnrolled
}

func (f *fakeMFAStore) GetPendingMFASecret(context.Context, string) (string, error) {
	return f.setupSecret, nil
}

func (f *fakeMFAStore) VerifyAndRecordTOTP(context.Context, string, string, string, time.Time) error {
	return nil
}

func (f *fakeMFAStore) VerifyAndConsumeRecoveryCode(context.Context, string, string) error {
	return nil
}

func (f *fakeMFAStore) SetupMFASecret(_ context.Context, userID, secret string, recoveryCodes []string) error {
	f.setupUserID = userID
	f.setupSecret = secret
	f.setupCodes = recoveryCodes
	return nil
}

func (f *fakeMFAStore) ActivateMFA(context.Context, string) error { return nil }

func (f *fakeMFAStore) DisableMFA(context.Context, string) error { return nil }

func TestAdminMFASetupDefaultsProvisioningLabelToUserEmail(t *testing.T) {
	t.Parallel()

	manager, err := auth.NewTokenManager("admin-auth-secret-at-least-32bytes")
	if err != nil {
		t.Fatalf("NewTokenManager returned error: %v", err)
	}
	service := &fakeAdminService{
		domains:         []maildb.DomainView{{ID: "domain-1", CompanyID: "company-1", Name: "example.com"}},
		users:           []maildb.UserView{{ID: "user-1", DomainID: "domain-1", Username: "admin", Role: "company_admin", Status: "active"}},
		sessionVersions: map[string]int64{"user-1": 1},
	}
	manager.SetRevocationChecker(service)
	mfaStore := &fakeMFAStore{}
	mux := http.NewServeMux()
	RegisterAdminRoutes(mux, service, "", WithTokenManager(manager), WithAdminMFAStore(mfaStore))

	token, err := manager.Sign(auth.Claims{UserID: "user-1", DomainID: "domain-1", CompanyID: "company-1", Role: "company_admin", SessionVersion: 1}, time.Minute)
	if err != nil {
		t.Fatalf("Sign returned error: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/admin/v1/auth/mfa/setup", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var body struct {
		QRURI string `json:"qr_uri"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if !strings.Contains(body.QRURI, "GoGoMail%20Admin:admin@example.com") {
		t.Fatalf("qr_uri = %q, want email label", body.QRURI)
	}
	if strings.Contains(body.QRURI, "user-1") {
		t.Fatalf("qr_uri = %q, should not use user id label", body.QRURI)
	}
	if mfaStore.setupUserID != "user-1" || mfaStore.setupSecret == "" || len(mfaStore.setupCodes) != 8 {
		t.Fatalf("mfa setup store = userID:%q secret:%q codes:%d", mfaStore.setupUserID, mfaStore.setupSecret, len(mfaStore.setupCodes))
	}
}
