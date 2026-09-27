package httpapi

import (
	"context"
	"net/http"
	"time"

	"github.com/gogomail/gogomail/internal/aiassistant"
	"github.com/gogomail/gogomail/internal/auth"
	"github.com/gogomail/gogomail/internal/maildb"
)

// AssistantService is the persistence + policy abstraction used by the AI
// assistant HTTP handlers. The default implementation delegates to
// *maildb.Repository and the mail service.
type AssistantService interface {
	// Settings / opt-in.
	GetAssistantSettings(ctx context.Context, userID string) (maildb.AssistantSettings, error)
	UpsertAssistantSettings(ctx context.Context, settings maildb.AssistantSettings) (maildb.AssistantSettings, error)
	// Domain kill-switch: reuses the MCP domain policy (Enabled == assistant on).
	GetUserMCPDomainPolicy(ctx context.Context, userID string) (maildb.DomainMCPPolicy, error)
	// Categorization rules CRUD.
	ListAssistantCategorizationRules(ctx context.Context, userID string) ([]maildb.AssistantCategorizationRule, error)
	CreateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error)
	UpdateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error)
	DeleteAssistantCategorizationRule(ctx context.Context, userID, id string) error
	// Thread access for summarization.
	ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error)
	GetMessage(ctx context.Context, userID string, messageID string) (maildb.MessageDetail, error)
	// User identity for audit scoping.
	GetUserProfile(ctx context.Context, userID string) (maildb.UserProfile, error)
}

// AssistantAuditRecorder records assistant usage to the audit log. The app
// layer supplies a concrete implementation backed by the audit repository.
type AssistantAuditRecorder interface {
	RecordAssistantUsage(ctx context.Context, event AssistantUsageEvent) error
}

// AssistantUsageEvent describes a single assistant invocation for auditing. It
// intentionally carries no mail content — only metadata.
type AssistantUsageEvent struct {
	UserID   string
	DomainID string
	Action   string // "assistant.summary" | "assistant.compose_assist" | "assistant.rule.*" | "assistant.settings.update"
	TargetID string
	Provider string
	Result   string
}

// AssistantRouteOptions configures the assistant routes.
type AssistantRouteOptions struct {
	// Audit is optional; when nil, usage is not recorded.
	Audit AssistantAuditRecorder
	// Summarizer is the provider used for thread summaries. When nil, the local
	// offline extractive summarizer is used.
	Summarizer aiassistant.Summarizer
}

const maxAssistantThreadMessages = 50

// RegisterAssistantRoutes wires the AI email assistant endpoints onto mux:
//
//	GET    /api/v1/me/assistant/settings
//	PUT    /api/v1/me/assistant/settings
//	GET    /api/v1/me/assistant/categorization-rules
//	POST   /api/v1/me/assistant/categorization-rules
//	PUT    /api/v1/me/assistant/categorization-rules/{id}
//	DELETE /api/v1/me/assistant/categorization-rules/{id}
//	POST   /api/v1/threads/{id}/summary
//	POST   /api/v1/compose/assist
//
// All endpoints are user-scoped via the shared JWT/API-key helpers. The
// assistant is opt-in (default off) and gated behind the per-domain kill switch
// (the MCP domain policy Enabled flag).
func RegisterAssistantRoutes(mux *http.ServeMux, service AssistantService, tokenManager *auth.TokenManager, opts AssistantRouteOptions) {
	if service == nil {
		return
	}
	summarizer := opts.Summarizer
	if summarizer == nil {
		summarizer = aiassistant.NewLocalSummarizer()
	}
	limiter := NewAdminIPRateLimiter(30, time.Minute)

	h := &assistantHandlers{
		service:    service,
		token:      tokenManager,
		summarizer: summarizer,
		audit:      opts.Audit,
		limiter:    limiter,
	}

	mux.HandleFunc("GET /api/v1/me/assistant/settings", h.getSettings)
	mux.HandleFunc("PUT /api/v1/me/assistant/settings", h.putSettings)
	mux.HandleFunc("GET /api/v1/me/assistant/categorization-rules", h.listRules)
	mux.HandleFunc("POST /api/v1/me/assistant/categorization-rules", h.createRule)
	mux.HandleFunc("PUT /api/v1/me/assistant/categorization-rules/{id}", h.updateRule)
	mux.HandleFunc("DELETE /api/v1/me/assistant/categorization-rules/{id}", h.deleteRule)
	mux.HandleFunc("POST /api/v1/threads/{id}/summary", h.threadSummary)
	mux.HandleFunc("POST /api/v1/compose/assist", h.composeAssist)
}

type assistantHandlers struct {
	service    AssistantService
	token      *auth.TokenManager
	summarizer aiassistant.Summarizer
	audit      AssistantAuditRecorder
	limiter    *AdminIPRateLimiter
}

// ensureAssistantEnabled verifies the per-domain kill switch and per-user
// opt-in. It writes the appropriate error and returns ok=false when blocked.
func (h *assistantHandlers) ensureAssistantEnabled(w http.ResponseWriter, r *http.Request, userID string, feature string) (maildb.AssistantSettings, bool) {
	policy, err := h.service.GetUserMCPDomainPolicy(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load assistant policy")
		return maildb.AssistantSettings{}, false
	}
	if !policy.Enabled {
		writeError(w, http.StatusForbidden, "assistant is disabled for this domain")
		return maildb.AssistantSettings{}, false
	}
	settings, err := h.service.GetAssistantSettings(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load assistant settings")
		return maildb.AssistantSettings{}, false
	}
	if !settings.Enabled {
		writeError(w, http.StatusForbidden, "assistant is not enabled; opt in via settings")
		return maildb.AssistantSettings{}, false
	}
	switch feature {
	case "summarization":
		if !settings.SummarizationEnabled {
			writeError(w, http.StatusForbidden, "thread summarization is disabled in settings")
			return maildb.AssistantSettings{}, false
		}
	case "compose":
		if !settings.ComposeAssistEnabled {
			writeError(w, http.StatusForbidden, "compose assist is disabled in settings")
			return maildb.AssistantSettings{}, false
		}
	}
	return settings, true
}

func (h *assistantHandlers) recordAudit(ctx context.Context, userID, action, targetID, provider, result string) {
	if h.audit == nil {
		return
	}
	domainID := ""
	if profile, err := h.service.GetUserProfile(ctx, userID); err == nil {
		domainID = profile.DomainID
	}
	_ = h.audit.RecordAssistantUsage(ctx, AssistantUsageEvent{
		UserID:   userID,
		DomainID: domainID,
		Action:   action,
		TargetID: targetID,
		Provider: provider,
		Result:   result,
	})
}

func assistantSettingsBody(s maildb.AssistantSettings) map[string]any {
	body := map[string]any{
		"enabled":                s.Enabled,
		"summarization_enabled":  s.SummarizationEnabled,
		"categorization_enabled": s.CategorizationEnabled,
		"compose_assist_enabled": s.ComposeAssistEnabled,
		"provider":               s.Provider,
	}
	if !s.UpdatedAt.IsZero() {
		body["updated_at"] = s.UpdatedAt.UTC().Format(time.RFC3339)
	}
	return body
}

func assistantRuleBody(rule maildb.AssistantCategorizationRule) map[string]any {
	return map[string]any{
		"id":         rule.ID,
		"category":   rule.Category,
		"field":      rule.Field,
		"match_type": rule.MatchType,
		"keyword":    rule.Keyword,
		"priority":   rule.Priority,
		"weight":     rule.Weight,
		"created_at": rule.CreatedAt.UTC().Format(time.RFC3339),
		"updated_at": rule.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
