package mailservice

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/aiassistant"
	"github.com/gogomail/gogomail/internal/maildb"
)

// assistantRepository is the subset of *maildb.Repository used by the AI
// assistant. The concrete repository satisfies it; tests may provide a fake.
type assistantRepository interface {
	GetAssistantSettings(ctx context.Context, userID string) (maildb.AssistantSettings, error)
	UpsertAssistantSettings(ctx context.Context, settings maildb.AssistantSettings) (maildb.AssistantSettings, error)
	ListAssistantCategorizationRules(ctx context.Context, userID string) ([]maildb.AssistantCategorizationRule, error)
	CreateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error)
	UpdateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error)
	DeleteAssistantCategorizationRule(ctx context.Context, userID, id string) error
	UpsertAssistantMessageLabel(ctx context.Context, label maildb.AssistantMessageLabel) error
	GetAssistantMessageLabel(ctx context.Context, userID, messageID string) (maildb.AssistantMessageLabel, bool, error)
}

func (s *Service) assistantRepo() (assistantRepository, error) {
	repo, ok := s.repository.(assistantRepository)
	if !ok {
		return nil, fmt.Errorf("assistant repository is required")
	}
	return repo, nil
}

// GetAssistantSettings returns the user's assistant settings.
func (s *Service) GetAssistantSettings(ctx context.Context, userID string) (maildb.AssistantSettings, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantSettings{}, err
	}
	return repo.GetAssistantSettings(ctx, userID)
}

// UpsertAssistantSettings stores the user's assistant settings.
func (s *Service) UpsertAssistantSettings(ctx context.Context, settings maildb.AssistantSettings) (maildb.AssistantSettings, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantSettings{}, err
	}
	return repo.UpsertAssistantSettings(ctx, settings)
}

// ListAssistantCategorizationRules returns the user's categorization rules.
func (s *Service) ListAssistantCategorizationRules(ctx context.Context, userID string) ([]maildb.AssistantCategorizationRule, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return nil, err
	}
	return repo.ListAssistantCategorizationRules(ctx, userID)
}

// CreateAssistantCategorizationRule creates a new rule.
func (s *Service) CreateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantCategorizationRule{}, err
	}
	return repo.CreateAssistantCategorizationRule(ctx, req)
}

// UpdateAssistantCategorizationRule updates an existing rule.
func (s *Service) UpdateAssistantCategorizationRule(ctx context.Context, req maildb.UpsertAssistantCategorizationRule) (maildb.AssistantCategorizationRule, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantCategorizationRule{}, err
	}
	return repo.UpdateAssistantCategorizationRule(ctx, req)
}

// DeleteAssistantCategorizationRule deletes a rule.
func (s *Service) DeleteAssistantCategorizationRule(ctx context.Context, userID, id string) error {
	repo, err := s.assistantRepo()
	if err != nil {
		return err
	}
	return repo.DeleteAssistantCategorizationRule(ctx, userID, id)
}

// GetAssistantMessageLabel returns the applied category/features label for a
// message, scoped to the user.
func (s *Service) GetAssistantMessageLabel(ctx context.Context, userID, messageID string) (maildb.AssistantMessageLabel, bool, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantMessageLabel{}, false, err
	}
	return repo.GetAssistantMessageLabel(ctx, userID, messageID)
}

// ApplyCategorization runs the user's categorization rules against a stored
// message and persists the resulting label + extracted features. It is the
// designated apply point for auto-categorization; the delivery pipeline (or a
// post-delivery worker) calls it after a message is stored. It is a no-op when
// the assistant or categorization is disabled for the user.
//
// Features are always persisted (even when no rule matches) so a future trained
// classifier can consume the same signal set without re-plumbing delivery.
// This runs off the hot SMTP receive path by design.
func (s *Service) ApplyCategorization(ctx context.Context, userID, messageID string, hasAttachment, listHeaders bool) (maildb.AssistantMessageLabel, error) {
	repo, err := s.assistantRepo()
	if err != nil {
		return maildb.AssistantMessageLabel{}, err
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" || messageID == "" {
		return maildb.AssistantMessageLabel{}, fmt.Errorf("user_id and message_id are required")
	}

	settings, err := repo.GetAssistantSettings(ctx, userID)
	if err != nil {
		return maildb.AssistantMessageLabel{}, err
	}
	if !settings.Enabled || !settings.CategorizationEnabled {
		return maildb.AssistantMessageLabel{}, nil
	}

	detail, err := s.repository.GetMessage(ctx, userID, messageID)
	if err != nil {
		return maildb.AssistantMessageLabel{}, err
	}
	rules, err := repo.ListAssistantCategorizationRules(ctx, userID)
	if err != nil {
		return maildb.AssistantMessageLabel{}, err
	}

	engineRules := make([]aiassistant.CategorizationRule, 0, len(rules))
	for _, rule := range rules {
		engineRules = append(engineRules, aiassistant.CategorizationRule{
			ID:       rule.ID,
			Category: rule.Category,
			Field:    aiassistant.RuleField(rule.Field),
			Match:    aiassistant.RuleMatchType(rule.MatchType),
			Keyword:  rule.Keyword,
			Priority: rule.Priority,
			Weight:   rule.Weight,
		})
	}

	msg := aiassistant.Message{
		ID:         detail.ID,
		FromName:   detail.FromName,
		FromAddr:   detail.FromAddr,
		ToAddrs:    parseAssistantAddressList(detail.ToAddrs),
		Subject:    detail.Subject,
		Body:       detail.TextBody,
		ReceivedAt: detail.ReceivedAt,
	}

	result := aiassistant.NewCategorizer().Categorize(msg, engineRules, hasAttachment || detail.HasAttachment, listHeaders)
	features, err := json.Marshal(result.Features)
	if err != nil {
		return maildb.AssistantMessageLabel{}, fmt.Errorf("marshal message features: %w", err)
	}
	ruleIDs := result.MatchedRuleIDs
	if ruleIDs == nil {
		ruleIDs = []string{}
	}
	label := maildb.AssistantMessageLabel{
		MessageID:      messageID,
		UserID:         userID,
		Category:       result.Category,
		Confidence:     result.Confidence,
		MatchedRuleIDs: ruleIDs,
		Features:       features,
	}
	if err := repo.UpsertAssistantMessageLabel(ctx, label); err != nil {
		return maildb.AssistantMessageLabel{}, err
	}
	return label, nil
}

func parseAssistantAddressList(raw json.RawMessage) []string {
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
