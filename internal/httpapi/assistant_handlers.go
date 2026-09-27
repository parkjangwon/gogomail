package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gogomail/gogomail/internal/aiassistant"
	"github.com/gogomail/gogomail/internal/maildb"
)

func (h *assistantHandlers) getSettings(w http.ResponseWriter, r *http.Request) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	settings, err := h.service.GetAssistantSettings(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load assistant settings")
		return
	}
	writeJSON(w, http.StatusOK, assistantSettingsBody(settings))
}

type assistantSettingsPUTRequest struct {
	Enabled               bool   `json:"enabled"`
	SummarizationEnabled  bool   `json:"summarization_enabled"`
	CategorizationEnabled bool   `json:"categorization_enabled"`
	ComposeAssistEnabled  bool   `json:"compose_assist_enabled"`
	Provider              string `json:"provider"`
}

func (h *assistantHandlers) putSettings(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	// Domain kill-switch applies even to enabling the assistant.
	policy, err := h.service.GetUserMCPDomainPolicy(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load assistant policy")
		return
	}
	var req assistantSettingsPUTRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid assistant settings")
		return
	}
	if req.Enabled && !policy.Enabled {
		writeError(w, http.StatusForbidden, "assistant is disabled for this domain")
		return
	}
	provider := strings.ToLower(strings.TrimSpace(req.Provider))
	if provider == "" {
		provider = "local"
	}
	settings, err := h.service.UpsertAssistantSettings(r.Context(), maildb.AssistantSettings{
		UserID:                userID,
		Enabled:               req.Enabled,
		SummarizationEnabled:  req.SummarizationEnabled,
		CategorizationEnabled: req.CategorizationEnabled,
		ComposeAssistEnabled:  req.ComposeAssistEnabled,
		Provider:              provider,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid assistant settings")
		return
	}
	h.recordAudit(r.Context(), userID, "assistant.settings.update", userID, provider, "success")
	writeJSON(w, http.StatusOK, assistantSettingsBody(settings))
}

func (h *assistantHandlers) listRules(w http.ResponseWriter, r *http.Request) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	rules, err := h.service.ListAssistantCategorizationRules(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load categorization rules")
		return
	}
	out := make([]map[string]any, 0, len(rules))
	for _, rule := range rules {
		out = append(out, assistantRuleBody(rule))
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": out})
}

type assistantRuleRequest struct {
	Category  string  `json:"category"`
	Field     string  `json:"field"`
	MatchType string  `json:"match_type"`
	Keyword   string  `json:"keyword"`
	Priority  int     `json:"priority"`
	Weight    float64 `json:"weight"`
}

func (req assistantRuleRequest) normalized() maildb.UpsertAssistantCategorizationRule {
	field := strings.ToLower(strings.TrimSpace(req.Field))
	if field == "" {
		field = "any"
	}
	matchType := strings.ToLower(strings.TrimSpace(req.MatchType))
	if matchType == "" {
		matchType = "contains"
	}
	return maildb.UpsertAssistantCategorizationRule{
		Category:  strings.TrimSpace(req.Category),
		Field:     field,
		MatchType: matchType,
		Keyword:   strings.TrimSpace(req.Keyword),
		Priority:  req.Priority,
		Weight:    req.Weight,
	}
}

func (h *assistantHandlers) createRule(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	var req assistantRuleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid categorization rule")
		return
	}
	upsert := req.normalized()
	upsert.UserID = userID
	rule, err := h.service.CreateAssistantCategorizationRule(r.Context(), upsert)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.recordAudit(r.Context(), userID, "assistant.rule.create", rule.ID, "local", "success")
	writeJSON(w, http.StatusCreated, assistantRuleBody(rule))
}

func (h *assistantHandlers) updateRule(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	ruleID, ok := parseBoundedHTTPPathValue(w, r, "id")
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	var req assistantRuleRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid categorization rule")
		return
	}
	upsert := req.normalized()
	upsert.UserID = userID
	upsert.ID = ruleID
	rule, err := h.service.UpdateAssistantCategorizationRule(r.Context(), upsert)
	if err != nil {
		if err.Error() == "rule not found" {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.recordAudit(r.Context(), userID, "assistant.rule.update", rule.ID, "local", "success")
	writeJSON(w, http.StatusOK, assistantRuleBody(rule))
}

func (h *assistantHandlers) deleteRule(w http.ResponseWriter, r *http.Request) {
	if !rejectBodylessRequestPayload(w, r) {
		return
	}
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	ruleID, ok := parseBoundedHTTPPathValue(w, r, "id")
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if err := h.service.DeleteAssistantCategorizationRule(r.Context(), userID, ruleID); err != nil {
		if err.Error() == "rule not found" {
			writeError(w, http.StatusNotFound, "rule not found")
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h.recordAudit(r.Context(), userID, "assistant.rule.delete", ruleID, "local", "success")
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "id": ruleID})
}

type composeAssistRequest struct {
	Subject         string   `json:"subject"`
	Body            string   `json:"body"`
	Recipients      []string `json:"recipients"`
	AttachmentCount int      `json:"attachment_count"`
}

func (h *assistantHandlers) composeAssist(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	userID, ok := userIDFromRequest(w, r, h.token)
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if _, ok := h.ensureAssistantEnabled(w, r, userID, "compose"); !ok {
		return
	}
	var req composeAssistRequest
	if err := decodeJSONBody(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid compose draft")
		return
	}
	assistant := aiassistant.NewComposeAssistant()
	result := assistant.Assist(aiassistant.ComposeDraft{
		Subject:         req.Subject,
		Body:            req.Body,
		Recipients:      req.Recipients,
		AttachmentCount: req.AttachmentCount,
	})
	h.recordAudit(r.Context(), userID, "assistant.compose_assist", "", "local", "success")
	writeJSON(w, http.StatusOK, result)
}

func (h *assistantHandlers) threadSummary(w http.ResponseWriter, r *http.Request) {
	if !rejectUnknownQueryKeys(w, r, "user_id", "user_email") {
		return
	}
	// This endpoint accepts an optional empty body; tolerate no content-type.
	if r.ContentLength > 0 {
		if err := requireJSONContentType(r); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		_ = r.Body.Close()
	}
	userID, ok := userIDFromRequest(w, r, h.token, nil)
	if !ok {
		return
	}
	threadID, ok := parseBoundedHTTPPathValue(w, r, "id")
	if !ok {
		return
	}
	if !h.limiter.allow(userID) {
		w.Header().Set("Retry-After", "60")
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return
	}
	if _, ok := h.ensureAssistantEnabled(w, r, userID, "summarization"); !ok {
		return
	}

	summaries, err := h.service.ListThreadMessagesPage(r.Context(), userID, threadID, maxAssistantThreadMessages, maildb.MessageListCursor{})
	if err != nil {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	}
	if len(summaries) == 0 {
		writeError(w, http.StatusNotFound, "thread not found")
		return
	}

	thread := aiassistant.Thread{Messages: make([]aiassistant.Message, 0, len(summaries))}
	for _, s := range summaries {
		// Fetch full body per message (user-scoped) so the summary reflects
		// full content, not just the preview. Errors on individual messages
		// fall back to the preview.
		body := s.Preview
		var toAddrs []string
		if detail, err := h.service.GetMessage(r.Context(), userID, s.ID); err == nil {
			if strings.TrimSpace(detail.TextBody) != "" {
				body = detail.TextBody
			}
			toAddrs = parseAddressList(detail.ToAddrs)
		}
		if thread.Subject == "" {
			thread.Subject = s.Subject
		}
		thread.Messages = append(thread.Messages, aiassistant.Message{
			ID:         s.ID,
			FromName:   s.FromName,
			FromAddr:   s.FromAddr,
			ToAddrs:    toAddrs,
			Subject:    s.Subject,
			Body:       body,
			ReceivedAt: s.ReceivedAt,
		})
	}

	summary, err := h.summarizer.Summarize(r.Context(), thread)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to summarize thread")
		return
	}
	h.recordAudit(r.Context(), userID, "assistant.summary", threadID, h.summarizer.Name(), "success")
	writeJSON(w, http.StatusOK, summary)
}

// parseAddressList extracts email addresses from the stored to_addrs JSON
// ([{name,address}]).
func parseAddressList(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return nil
	}
	var items []struct {
		Address string `json:"address"`
	}
	if err := json.Unmarshal(raw, &items); err != nil {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, it := range items {
		if a := strings.TrimSpace(it.Address); a != "" {
			out = append(out, a)
		}
	}
	return out
}
