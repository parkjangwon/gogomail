package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"
)

type MessageDeliveryStatusView struct {
	MessageID      string                `json:"message_id"`
	RFCMessageID   string                `json:"rfc_message_id"`
	DeliveryStatus string                `json:"delivery_status"`
	BounceStatus   string                `json:"bounce_status"`
	Attempts       []DeliveryAttemptView `json:"attempts"`
	UpdatedAt      time.Time             `json:"updated_at"`
}

func (r *Repository) MessageDeliveryStatus(ctx context.Context, userID string, messageID string) (MessageDeliveryStatusView, error) {
	if r.db == nil {
		return MessageDeliveryStatusView{}, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	messageID = strings.TrimSpace(messageID)
	if userID == "" {
		return MessageDeliveryStatusView{}, fmt.Errorf("user_id is required")
	}
	if messageID == "" {
		return MessageDeliveryStatusView{}, fmt.Errorf("message id is required")
	}

	const messageQuery = `
SELECT id::text, COALESCE(rfc_message_id, '')
FROM messages
WHERE id = $1
  AND user_id = $2
  AND status = 'active'
LIMIT 1`

	var view MessageDeliveryStatusView
	if err := r.db.QueryRowContext(ctx, messageQuery, messageID, userID).Scan(&view.MessageID, &view.RFCMessageID); err != nil {
		if err == sql.ErrNoRows {
			return MessageDeliveryStatusView{}, fmt.Errorf("message %q not found", messageID)
		}
		return MessageDeliveryStatusView{}, fmt.Errorf("lookup message delivery status owner: %w", err)
	}

	const attemptsQuery = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  sender,
  recipient,
  recipient_domain,
  status,
  enhanced_status,
  error_message,
  dsn_return,
  dsn_envelope_id,
  dsn_notify,
  original_recipient,
  attempted_at
FROM delivery_attempts
WHERE message_id = $1
ORDER BY attempted_at DESC, id DESC
LIMIT 200`

	rows, err := r.db.QueryContext(ctx, attemptsQuery, messageID)
	if err != nil {
		return MessageDeliveryStatusView{}, fmt.Errorf("list message delivery attempts: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := scanDeliveryAttempt(rows, &attempt); err != nil {
			return MessageDeliveryStatusView{}, fmt.Errorf("scan message delivery attempt: %w", err)
		}
		view.Attempts = append(view.Attempts, attempt)
		if attempt.AttemptedAt.After(view.UpdatedAt) {
			view.UpdatedAt = attempt.AttemptedAt
		}
	}
	if err := rows.Err(); err != nil {
		return MessageDeliveryStatusView{}, fmt.Errorf("iterate message delivery attempts: %w", err)
	}
	view.DeliveryStatus, view.BounceStatus = summarizeDeliveryAttempts(view.Attempts)
	return view, nil
}

func summarizeDeliveryAttempts(attempts []DeliveryAttemptView) (string, string) {
	if len(attempts) == 0 {
		return "pending", "none"
	}
	delivered := false
	failed := false
	bounced := false
	retry := false
	for _, attempt := range attempts {
		status := strings.ToLower(strings.TrimSpace(attempt.Status))
		if status == "failed" && enhancedStatusClass(attempt.EnhancedStatus) == '4' {
			retry = true
			continue
		}
		switch status {
		case "delivered":
			delivered = true
		case "bounced", "hard_bounce":
			bounced = true
		case "retry", "deferred", "temporary_failure":
			retry = true
		case "failed", "permanent_failure", "exhausted":
			failed = true
		}
	}
	switch {
	case bounced:
		return "bounced", "hard"
	case failed && !delivered:
		return "failed", "none"
	case retry && !delivered:
		return "retrying", "none"
	case delivered && (failed || retry):
		return "partial", "none"
	case delivered:
		return "delivered", "none"
	default:
		return "pending", "none"
	}
}

func enhancedStatusClass(status string) byte {
	status = strings.TrimSpace(status)
	parts := strings.Split(status, ".")
	if len(parts) != 3 || len(parts[0]) != 1 {
		return 0
	}
	for _, part := range parts[1:] {
		if part == "" || len(part) > 3 {
			return 0
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return 0
			}
		}
	}
	switch parts[0][0] {
	case '2', '4', '5':
		return parts[0][0]
	default:
		return 0
	}
}
