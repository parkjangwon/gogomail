package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

// GetWebmailPreferences returns the user's stored webmail preferences.
// Returns an empty JSON object when no preferences have been saved.
func (r *Repository) GetWebmailPreferences(ctx context.Context, userID string) (json.RawMessage, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	var raw []byte
	err := r.db.QueryRowContext(ctx,
		`SELECT COALESCE(settings->'webmail', '{}'::jsonb) FROM users WHERE id = $1::uuid`,
		userID,
	).Scan(&raw)
	if err != nil {
		if err == sql.ErrNoRows {
			return json.RawMessage("{}"), nil
		}
		return nil, fmt.Errorf("get webmail preferences: %w", err)
	}
	return json.RawMessage(raw), nil
}

// SetWebmailPreferences replaces the user's webmail preferences with prefs.
// prefs must be a valid JSON object.
func (r *Repository) SetWebmailPreferences(ctx context.Context, userID string, prefs json.RawMessage) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if !json.Valid(prefs) {
		return fmt.Errorf("preferences must be valid JSON")
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET settings = jsonb_set(settings, '{webmail}', $2::jsonb, true), updated_at = now() WHERE id = $1::uuid`,
		userID, []byte(prefs),
	)
	if err != nil {
		return fmt.Errorf("set webmail preferences: %w", err)
	}
	return nil
}
