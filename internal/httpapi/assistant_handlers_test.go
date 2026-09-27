package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gogomail/gogomail/internal/maildb"
)

// fakeAssistantService implements AssistantService for handler tests.
type fakeAssistantService struct {
	settings    maildb.AssistantSettings
	policy      maildb.DomainMCPPolicy
	rules       []maildb.AssistantCategorizationRule
	threadMsgs  []maildb.MessageSummary
	messages    map[string]maildb.MessageDetail
	profile     maildb.UserProfile
	lastUserID  string
	createdRule maildb.UpsertAssistantCategorizationRule
	deletedID   string
}

func (f *fakeAssistantService) GetAssistantSettings(ctx context.Context, userID string) (maildb.AssistantSettings, error) {
	f.lastUserID = userID
	return f.settings, nil
}

func (f *fakeAssistantService) UpsertAssistantSettings(ctx context.Context, settings maildb.AssistantSettings) (maildb.AssistantSettings, error) {
	f.lastUserID = settings.UserID
	f.settings = settings
	f.settings.UpdatedAt = time.Unix(0, 0).UTC()
	return f.settings, nil
}

func (f *fakeAssistantService) GetUserMCPDomainPolicy(ctx context.Context, userID string) (maildb.DomainMCPPolicy, error) {
	return f.policy, nil
}

func (f *fakeAssistantService) ListAssistantCategorizationRules(ctx context.Context, userID string) ([]maildb.AssistantCategorizationRule, error) {
	f.lastUserID = userID
	return f.rules, nil
}

func (f *fakeAssistantService) CreateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error) {
	f.lastUserID = req.UserID
	f.createdRule = req
	return maildb.AssistantCategorizationRule{
		ID: "rule-1", UserID: req.UserID, Category: req.Category, Field: req.Field,
		MatchType: req.MatchType, Keyword: req.Keyword, Priority: req.Priority, Weight: req.Weight,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}, nil
}

func (f *fakeAssistantService) UpdateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error) {
	f.lastUserID = req.UserID
	return maildb.AssistantCategorizationRule{
		ID: req.ID, UserID: req.UserID, Category: req.Category, Field: req.Field,
		MatchType: req.MatchType, Keyword: req.Keyword, Priority: req.Priority, Weight: req.Weight,
		CreatedAt: time.Unix(0, 0).UTC(), UpdatedAt: time.Unix(0, 0).UTC(),
	}, nil
}

func (f *fakeAssistantService) DeleteAssistantCategorizationRule(ctx context.Context, userID, id string) error {
	f.lastUserID = userID
	f.deletedID = id
	return nil
}

func (f *fakeAssistantService) ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	f.lastUserID = userID
	return f.threadMsgs, nil
}

func (f *fakeAssistantService) GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error) {
	if d, ok := f.messages[messageID]; ok {
		return d, nil
	}
	return maildb.MessageDetail{}, nil
}

func (f *fakeAssistantService) GetUserProfile(ctx context.Context, userID string) (maildb.UserProfile, error) {
	return f.profile, nil
}

func newEnabledAssistantService() *fakeAssistantService {
	return &fakeAssistantService{
		settings: maildb.AssistantSettings{
			Enabled: true, SummarizationEnabled: true, CategorizationEnabled: true,
			ComposeAssistEnabled: true, Provider: "local",
		},
		policy:  maildb.DomainMCPPolicy{Enabled: true},
		profile: maildb.UserProfile{UserID: "user-1", DomainID: "domain-1"},
	}
}

func assistantMux(service AssistantService) *http.ServeMux {
	mux := http.NewServeMux()
	RegisterAssistantRoutes(mux, service, nil, AssistantRouteOptions{})
	return mux
}

func TestAssistantGetSettings(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	mux := assistantMux(service)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/me/assistant/settings?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q", service.lastUserID)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["enabled"] != true {
		t.Fatalf("enabled = %v", body["enabled"])
	}
}

func TestAssistantPutSettingsBlockedByDomainKillSwitch(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	service.policy = maildb.DomainMCPPolicy{Enabled: false}
	mux := assistantMux(service)

	body := `{"enabled":true,"summarization_enabled":true,"categorization_enabled":true,"compose_assist_enabled":true,"provider":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/assistant/settings?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s want 403", rec.Code, rec.Body.String())
	}
}

func TestAssistantPutSettingsAllowsDisableWhenDomainOff(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	service.policy = maildb.DomainMCPPolicy{Enabled: false}
	mux := assistantMux(service)

	body := `{"enabled":false,"summarization_enabled":true,"categorization_enabled":true,"compose_assist_enabled":true,"provider":"local"}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/me/assistant/settings?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s want 200", rec.Code, rec.Body.String())
	}
}

func TestAssistantComposeAssistRequiresOptIn(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	service.settings.Enabled = false // user has not opted in
	mux := assistantMux(service)

	body := `{"subject":"Report","body":"See attached.","recipients":["a@b.com"],"attachment_count":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compose/assist?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body = %s want 403", rec.Code, rec.Body.String())
	}
}

func TestAssistantComposeAssistMissingAttachment(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	mux := assistantMux(service)

	body := `{"subject":"Report","body":"Please find the report attached.","recipients":["a@b.com"],"attachment_count":0}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/compose/assist?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Checks []struct {
			Code string `json:"code"`
		} `json:"checks"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, c := range result.Checks {
		if c.Code == "missing_attachment" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing_attachment check, got %+v", result.Checks)
	}
}

func TestAssistantThreadSummary(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	service.threadMsgs = []maildb.MessageSummary{
		{ID: "m1", Subject: "Q1 Planning", FromName: "Alice", FromAddr: "alice@example.com", Preview: "Please send the budget by Friday."},
	}
	service.messages = map[string]maildb.MessageDetail{
		"m1": {ID: "m1", Subject: "Q1 Planning", FromAddr: "alice@example.com", TextBody: "Please send the budget draft by Friday. The roadmap must cover marketing.", ToAddrs: json.RawMessage(`[{"address":"bob@example.com"}]`)},
	}
	mux := assistantMux(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/threads/thread-1/summary?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if service.lastUserID != "user-1" {
		t.Fatalf("lastUserID = %q (scoping not enforced)", service.lastUserID)
	}
	var summary struct {
		Provider         string `json:"provider"`
		GeneratedOffline bool   `json:"generated_offline"`
		MessageCount     int    `json:"message_count"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if summary.Provider != "local" || !summary.GeneratedOffline {
		t.Fatalf("expected local offline provider, got %+v", summary)
	}
	if summary.MessageCount != 1 {
		t.Fatalf("message_count = %d", summary.MessageCount)
	}
}

func TestAssistantThreadSummaryBlockedByDomain(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	service.policy = maildb.DomainMCPPolicy{Enabled: false}
	mux := assistantMux(service)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/threads/thread-1/summary?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d want 403", rec.Code)
	}
}

func TestAssistantCreateRule(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	mux := assistantMux(service)

	body := `{"category":"Finance","field":"subject","match_type":"contains","keyword":"invoice","priority":5,"weight":2}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/me/assistant/categorization-rules?user_id=user-1", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if service.createdRule.UserID != "user-1" {
		t.Fatalf("rule userID = %q (scoping not enforced)", service.createdRule.UserID)
	}
	if service.createdRule.Category != "Finance" || service.createdRule.Keyword != "invoice" {
		t.Fatalf("createdRule = %+v", service.createdRule)
	}
}

func TestAssistantDeleteRule(t *testing.T) {
	t.Parallel()
	service := newEnabledAssistantService()
	mux := assistantMux(service)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/me/assistant/categorization-rules/rule-9?user_id=user-1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	if service.deletedID != "rule-9" || service.lastUserID != "user-1" {
		t.Fatalf("deletedID = %q lastUserID = %q", service.deletedID, service.lastUserID)
	}
}
