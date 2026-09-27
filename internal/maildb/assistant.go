package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

// AssistantSettings is the per-user AI assistant configuration. The assistant
// is opt-in: Enabled defaults to false.
type AssistantSettings struct {
	UserID                 string    `json:"user_id"`
	Enabled                bool      `json:"enabled"`
	SummarizationEnabled   bool      `json:"summarization_enabled"`
	CategorizationEnabled  bool      `json:"categorization_enabled"`
	ComposeAssistEnabled   bool      `json:"compose_assist_enabled"`
	Provider               string    `json:"provider"`
	UpdatedAt              time.Time `json:"updated_at,omitempty"`
}

// AssistantCategorizationRule is a persisted user categorization rule.
type AssistantCategorizationRule struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Category  string    `json:"category"`
	Field     string    `json:"field"`
	MatchType string    `json:"match_type"`
	Keyword   string    `json:"keyword"`
	Priority  int       `json:"priority"`
	Weight    float64   `json:"weight"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// UpsertAssistantCategorizationRule carries a create-or-update request.
type UpsertAssistantCategorizationRule struct {
	ID        string  `json:"id"`
	UserID    string  `json:"user_id"`
	Category  string  `json:"category"`
	Field     string  `json:"field"`
	MatchType string  `json:"match_type"`
	Keyword   string  `json:"keyword"`
	Priority  int     `json:"priority"`
	Weight    float64 `json:"weight"`
}

// AssistantMessageLabel is an applied category + extracted feature set for a
// message.
type AssistantMessageLabel struct {
	MessageID      string          `json:"message_id"`
	UserID         string          `json:"user_id"`
	Category       string          `json:"category"`
	Confidence     float64         `json:"confidence"`
	MatchedRuleIDs []string        `json:"matched_rule_ids"`
	Features       json.RawMessage `json:"features"`
	AppliedAt      time.Time       `json:"applied_at"`
}

// GetAssistantSettings returns the user's assistant settings, or defaults
// (disabled, local provider) when none are stored.
func (r *Repository) GetAssistantSettings(ctx context.Context, userID string) (AssistantSettings, error) {
	if r.db == nil {
		return AssistantSettings{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return AssistantSettings{}, fmt.Errorf("user_id is required")
	}
	settings := AssistantSettings{
		UserID:                userID,
		Enabled:               false,
		SummarizationEnabled:  true,
		CategorizationEnabled: true,
		ComposeAssistEnabled:  true,
		Provider:              "local",
	}
	err := r.db.QueryRowContext(ctx, `
SELECT enabled, summarization_enabled, categorization_enabled, compose_assist_enabled, provider, updated_at
FROM assistant_settings
WHERE user_id = $1::uuid`, userID).Scan(
		&settings.Enabled,
		&settings.SummarizationEnabled,
		&settings.CategorizationEnabled,
		&settings.ComposeAssistEnabled,
		&settings.Provider,
		&settings.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return settings, nil
		}
		return AssistantSettings{}, fmt.Errorf("get assistant settings: %w", err)
	}
	return settings, nil
}

// UpsertAssistantSettings stores the user's assistant settings.
func (r *Repository) UpsertAssistantSettings(ctx context.Context, settings AssistantSettings) (AssistantSettings, error) {
	if r.db == nil {
		return AssistantSettings{}, fmt.Errorf("database handle is required")
	}
	settings.UserID = strings.TrimSpace(settings.UserID)
	if settings.UserID == "" {
		return AssistantSettings{}, fmt.Errorf("user_id is required")
	}
	provider := strings.ToLower(strings.TrimSpace(settings.Provider))
	if provider == "" {
		provider = "local"
	}
	err := r.db.QueryRowContext(ctx, `
INSERT INTO assistant_settings (user_id, enabled, summarization_enabled, categorization_enabled, compose_assist_enabled, provider, updated_at)
VALUES ($1::uuid, $2, $3, $4, $5, $6, now())
ON CONFLICT (user_id) DO UPDATE SET
  enabled = EXCLUDED.enabled,
  summarization_enabled = EXCLUDED.summarization_enabled,
  categorization_enabled = EXCLUDED.categorization_enabled,
  compose_assist_enabled = EXCLUDED.compose_assist_enabled,
  provider = EXCLUDED.provider,
  updated_at = now()
RETURNING enabled, summarization_enabled, categorization_enabled, compose_assist_enabled, provider, updated_at`,
		settings.UserID,
		settings.Enabled,
		settings.SummarizationEnabled,
		settings.CategorizationEnabled,
		settings.ComposeAssistEnabled,
		provider,
	).Scan(
		&settings.Enabled,
		&settings.SummarizationEnabled,
		&settings.CategorizationEnabled,
		&settings.ComposeAssistEnabled,
		&settings.Provider,
		&settings.UpdatedAt,
	)
	if err != nil {
		return AssistantSettings{}, fmt.Errorf("upsert assistant settings: %w", err)
	}
	return settings, nil
}

// ListAssistantCategorizationRules returns the user's rules ordered by
// priority (desc) then creation time (asc).
func (r *Repository) ListAssistantCategorizationRules(ctx context.Context, userID string) ([]AssistantCategorizationRule, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	rows, err := r.db.QueryContext(ctx, `
SELECT id::text, user_id::text, category, field, match_type, keyword, priority, weight, created_at, updated_at
FROM assistant_categorization_rules
WHERE user_id = $1::uuid
ORDER BY priority DESC, created_at ASC, id ASC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list assistant rules: %w", err)
	}
	defer rows.Close()
	var out []AssistantCategorizationRule
	for rows.Next() {
		var rule AssistantCategorizationRule
		if err := rows.Scan(
			&rule.ID, &rule.UserID, &rule.Category, &rule.Field, &rule.MatchType,
			&rule.Keyword, &rule.Priority, &rule.Weight, &rule.CreatedAt, &rule.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan assistant rule: %w", err)
		}
		out = append(out, rule)
	}
	return out, rows.Err()
}

// CreateAssistantCategorizationRule inserts a new rule.
func (r *Repository) CreateAssistantCategorizationRule(ctx context.Context, req UpsertAssistantCategorizationRule) (AssistantCategorizationRule, error) {
	if r.db == nil {
		return AssistantCategorizationRule{}, fmt.Errorf("database handle is required")
	}
	req.UserID = strings.TrimSpace(req.UserID)
	if err := validateAssistantRule(req); err != nil {
		return AssistantCategorizationRule{}, err
	}
	var rule AssistantCategorizationRule
	err := r.db.QueryRowContext(ctx, `
INSERT INTO assistant_categorization_rules (user_id, category, field, match_type, keyword, priority, weight)
VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
RETURNING id::text, user_id::text, category, field, match_type, keyword, priority, weight, created_at, updated_at`,
		req.UserID, strings.TrimSpace(req.Category), req.Field, req.MatchType,
		strings.TrimSpace(req.Keyword), req.Priority, normalizedWeight(req.Weight),
	).Scan(
		&rule.ID, &rule.UserID, &rule.Category, &rule.Field, &rule.MatchType,
		&rule.Keyword, &rule.Priority, &rule.Weight, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		return AssistantCategorizationRule{}, fmt.Errorf("create assistant rule: %w", err)
	}
	return rule, nil
}

// UpdateAssistantCategorizationRule updates an existing rule scoped to the user.
func (r *Repository) UpdateAssistantCategorizationRule(ctx context.Context, req UpsertAssistantCategorizationRule) (AssistantCategorizationRule, error) {
	if r.db == nil {
		return AssistantCategorizationRule{}, fmt.Errorf("database handle is required")
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.ID = strings.TrimSpace(req.ID)
	if req.ID == "" {
		return AssistantCategorizationRule{}, fmt.Errorf("rule id is required")
	}
	if err := validateAssistantRule(req); err != nil {
		return AssistantCategorizationRule{}, err
	}
	var rule AssistantCategorizationRule
	err := r.db.QueryRowContext(ctx, `
UPDATE assistant_categorization_rules
SET category = $3, field = $4, match_type = $5, keyword = $6, priority = $7, weight = $8, updated_at = now()
WHERE id = $1::uuid AND user_id = $2::uuid
RETURNING id::text, user_id::text, category, field, match_type, keyword, priority, weight, created_at, updated_at`,
		req.ID, req.UserID, strings.TrimSpace(req.Category), req.Field, req.MatchType,
		strings.TrimSpace(req.Keyword), req.Priority, normalizedWeight(req.Weight),
	).Scan(
		&rule.ID, &rule.UserID, &rule.Category, &rule.Field, &rule.MatchType,
		&rule.Keyword, &rule.Priority, &rule.Weight, &rule.CreatedAt, &rule.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AssistantCategorizationRule{}, fmt.Errorf("rule not found")
		}
		return AssistantCategorizationRule{}, fmt.Errorf("update assistant rule: %w", err)
	}
	return rule, nil
}

// DeleteAssistantCategorizationRule removes a rule scoped to the user.
func (r *Repository) DeleteAssistantCategorizationRule(ctx context.Context, userID, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return fmt.Errorf("user_id and id are required")
	}
	res, err := r.db.ExecContext(ctx, `
DELETE FROM assistant_categorization_rules
WHERE id = $1::uuid AND user_id = $2::uuid`, id, userID)
	if err != nil {
		return fmt.Errorf("delete assistant rule: %w", err)
	}
	affected, err := res.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("rule not found")
	}
	return nil
}

// UpsertAssistantMessageLabel stores the applied category and features for a
// message. Callers must ensure the message belongs to the user.
func (r *Repository) UpsertAssistantMessageLabel(ctx context.Context, label AssistantMessageLabel) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	label.MessageID = strings.TrimSpace(label.MessageID)
	label.UserID = strings.TrimSpace(label.UserID)
	if label.MessageID == "" || label.UserID == "" {
		return fmt.Errorf("message_id and user_id are required")
	}
	features := label.Features
	if len(features) == 0 || !json.Valid(features) {
		features = json.RawMessage(`{}`)
	}
	ruleIDs := label.MatchedRuleIDs
	if ruleIDs == nil {
		ruleIDs = []string{}
	}
	_, err := r.db.ExecContext(ctx, `
INSERT INTO assistant_message_labels (message_id, user_id, category, confidence, matched_rule_ids, features, applied_at)
VALUES ($1::uuid, $2::uuid, $3, $4, $5::uuid[], $6::jsonb, now())
ON CONFLICT (message_id) DO UPDATE SET
  category = EXCLUDED.category,
  confidence = EXCLUDED.confidence,
  matched_rule_ids = EXCLUDED.matched_rule_ids,
  features = EXCLUDED.features,
  applied_at = now()`,
		label.MessageID, label.UserID, strings.TrimSpace(label.Category),
		label.Confidence, pq.Array(ruleIDs), []byte(features),
	)
	if err != nil {
		return fmt.Errorf("upsert assistant message label: %w", err)
	}
	return nil
}

// GetAssistantMessageLabel returns the applied label for a message, scoped to
// the user. Returns ok=false when no label exists.
func (r *Repository) GetAssistantMessageLabel(ctx context.Context, userID, messageID string) (AssistantMessageLabel, bool, error) {
	if r.db == nil {
		return AssistantMessageLabel{}, false, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" || messageID == "" {
		return AssistantMessageLabel{}, false, fmt.Errorf("user_id and message_id are required")
	}
	var label AssistantMessageLabel
	var features []byte
	err := r.db.QueryRowContext(ctx, `
SELECT message_id::text, user_id::text, category, confidence, matched_rule_ids, features, applied_at
FROM assistant_message_labels
WHERE message_id = $1::uuid AND user_id = $2::uuid`, messageID, userID).Scan(
		&label.MessageID, &label.UserID, &label.Category, &label.Confidence,
		pq.Array(&label.MatchedRuleIDs), &features, &label.AppliedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AssistantMessageLabel{}, false, nil
		}
		return AssistantMessageLabel{}, false, fmt.Errorf("get assistant message label: %w", err)
	}
	label.Features = json.RawMessage(features)
	return label, true, nil
}

func normalizedWeight(w float64) float64 {
	if w <= 0 {
		return 1
	}
	return w
}

func validateAssistantRule(req UpsertAssistantCategorizationRule) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.Category) == "" {
		return fmt.Errorf("category is required")
	}
	if strings.TrimSpace(req.Keyword) == "" {
		return fmt.Errorf("keyword is required")
	}
	switch req.Field {
	case "from", "subject", "body", "to", "any":
	default:
		return fmt.Errorf("field must be one of from, subject, body, to, any")
	}
	switch req.MatchType {
	case "contains", "prefix", "suffix", "exact":
	default:
		return fmt.Errorf("match_type must be one of contains, prefix, suffix, exact")
	}
	return nil
}
