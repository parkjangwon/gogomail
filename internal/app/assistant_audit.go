package app

import (
	"context"
	"encoding/json"

	"github.com/gogomail/gogomail/internal/audit"
	"github.com/gogomail/gogomail/internal/httpapi"
)

// assistantAuditRecorder records AI assistant usage to the audit log. It
// records metadata only — never mail content — which supports the privacy
// guarantee documented in docs/AI_ASSISTANT.md.
type assistantAuditRecorder struct {
	audit auditWriter
}

func (r assistantAuditRecorder) RecordAssistantUsage(ctx context.Context, event httpapi.AssistantUsageEvent) error {
	if r.audit == nil {
		return nil
	}
	detail, _ := json.Marshal(struct {
		Provider string `json:"provider,omitempty"`
	}{Provider: event.Provider})

	result := event.Result
	if result == "" {
		result = "success"
	}
	return r.audit.Insert(ctx, audit.Log{
		DomainID:   event.DomainID,
		UserID:     event.UserID,
		ActorID:    event.UserID,
		Category:   "assistant",
		Action:     event.Action,
		TargetType: "assistant",
		TargetID:   event.TargetID,
		Result:     result,
		Detail:     detail,
	})
}
