package maildb

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

// WebPushSubscription represents a row in the web_push_subscriptions table.
type WebPushSubscription struct {
	ID        string
	UserID    string
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent string
	Status    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// UpsertWebPushSubscriptionRequest holds the fields required to create or
// update a Web Push subscription.
type UpsertWebPushSubscriptionRequest struct {
	UserID    string
	Endpoint  string
	P256DH    string
	Auth      string
	UserAgent string
}

// Validate trims whitespace and returns an error if any required field is
// missing or invalid.
func (r *UpsertWebPushSubscriptionRequest) Validate() error {
	r.UserID = strings.TrimSpace(r.UserID)
	r.Endpoint = strings.TrimSpace(r.Endpoint)
	r.P256DH = strings.TrimSpace(r.P256DH)
	r.Auth = strings.TrimSpace(r.Auth)
	r.UserAgent = strings.TrimSpace(r.UserAgent)

	if r.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.ContainsAny(r.UserID, "\r\n") {
		return fmt.Errorf("user_id must not contain line breaks")
	}
	if r.Endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	if !strings.HasPrefix(r.Endpoint, "https://") {
		return fmt.Errorf("endpoint must be an HTTPS URL")
	}
	if len(r.Endpoint) > 2048 {
		return fmt.Errorf("endpoint must not exceed 2048 characters")
	}
	if strings.ContainsAny(r.Endpoint, "\r\n") {
		return fmt.Errorf("endpoint must not contain line breaks")
	}
	if r.P256DH == "" {
		return fmt.Errorf("p256dh is required")
	}
	if r.Auth == "" {
		return fmt.Errorf("auth is required")
	}
	return nil
}

// UpsertWebPushSubscription inserts a new subscription or, on endpoint
// conflict, updates the keys and user-agent of the existing active row.
func (r *Repository) UpsertWebPushSubscription(ctx context.Context, req UpsertWebPushSubscriptionRequest) (WebPushSubscription, error) {
	if err := req.Validate(); err != nil {
		return WebPushSubscription{}, err
	}
	var sub WebPushSubscription
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO web_push_subscriptions (user_id, endpoint, p256dh, auth, user_agent, status)
		VALUES ($1, $2, $3, $4, $5, 'active')
		ON CONFLICT (endpoint) WHERE status = 'active'
		DO UPDATE SET
			p256dh     = EXCLUDED.p256dh,
			auth       = EXCLUDED.auth,
			user_agent = EXCLUDED.user_agent,
			updated_at = now()
		RETURNING id, user_id, endpoint, p256dh, auth, COALESCE(user_agent, ''), status, created_at, updated_at
	`, req.UserID, req.Endpoint, req.P256DH, req.Auth, req.UserAgent).Scan(
		&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth,
		&sub.UserAgent, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt,
	)
	if err != nil {
		return WebPushSubscription{}, fmt.Errorf("upsert web push subscription: %w", err)
	}
	return sub, nil
}

// ListActiveWebPushSubscriptions returns all active subscriptions for the
// given user, ordered newest first.
func (r *Repository) ListActiveWebPushSubscriptions(ctx context.Context, userID string) ([]WebPushSubscription, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, user_id, endpoint, p256dh, auth, COALESCE(user_agent, ''), status, created_at, updated_at
		FROM web_push_subscriptions
		WHERE user_id = $1 AND status = 'active'
		ORDER BY created_at DESC
	`, userID)
	if err != nil {
		return nil, fmt.Errorf("list web push subscriptions: %w", err)
	}
	defer rows.Close()

	var subs []WebPushSubscription
	for rows.Next() {
		var sub WebPushSubscription
		if err := rows.Scan(
			&sub.ID, &sub.UserID, &sub.Endpoint, &sub.P256DH, &sub.Auth,
			&sub.UserAgent, &sub.Status, &sub.CreatedAt, &sub.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan web push subscription: %w", err)
		}
		subs = append(subs, sub)
	}
	return subs, rows.Err()
}

// DeleteWebPushSubscription soft-deletes the subscription identified by
// userID + id (only if currently active).
func (r *Repository) DeleteWebPushSubscription(ctx context.Context, userID, id string) error {
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" || id == "" {
		return fmt.Errorf("user_id and id are required")
	}
	result, err := r.db.ExecContext(ctx, `
		UPDATE web_push_subscriptions
		SET status = 'deleted', updated_at = now()
		WHERE user_id = $1 AND id = $2 AND status = 'active'
	`, userID, id)
	if err != nil {
		return fmt.Errorf("delete web push subscription: %w", err)
	}
	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete web push subscription rows affected: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("web push subscription %q not found", id)
	}
	return nil
}

// SoftDeleteWebPushSubscriptionByEndpoint marks the active subscription for
// the given endpoint as deleted. Used when a push service returns 410 Gone.
func (r *Repository) SoftDeleteWebPushSubscriptionByEndpoint(ctx context.Context, endpoint string) error {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		return fmt.Errorf("endpoint is required")
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE web_push_subscriptions
		SET status = 'deleted', updated_at = now()
		WHERE endpoint = $1 AND status = 'active'
	`, endpoint)
	if err != nil {
		return fmt.Errorf("soft-delete web push subscription by endpoint: %w", err)
	}
	return nil
}

// webPushSubscriptionExists returns true when err is NOT sql.ErrNoRows.
func webPushSubscriptionExists(err error) bool {
	return !errors.Is(err, sql.ErrNoRows)
}
