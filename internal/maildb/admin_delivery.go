package maildb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) ListDeliveryAttempts(ctx context.Context, req DeliveryAttemptListRequest) ([]DeliveryAttemptView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		Status:          req.Status,
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return nil, false, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	queryLimit := req.Limit
	if req.ProbeMore {
		queryLimit = req.Limit + 1
	}

	query, args := buildListDeliveryAttemptsQuery(filters, req.Since, queryLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := scanDeliveryAttempt(rows, &attempt); err != nil {
			return nil, false, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate delivery attempts: %w", err)
	}
	hasMore := req.ProbeMore && len(attempts) > req.Limit
	if hasMore {
		attempts = attempts[:req.Limit]
	}
	return attempts, hasMore, nil
}

func buildListDeliveryAttemptsQuery(filters deliveryAttemptFilters, since time.Time, queryLimit int) (string, []any) {
	query := listDeliveryAttemptsBaseSQL
	whereClause, args := buildDeliveryAttemptWhereClause(filters, since)
	query += whereClause

	args = append(args, queryLimit)
	query += fmt.Sprintf(`
ORDER BY attempted_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func buildDeliveryAttemptWhereClause(filters deliveryAttemptFilters, since time.Time) (string, []any) {
	var conditions []string
	var args []any

	if filters.Status != "" {
		args = append(args, filters.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if !since.IsZero() {
		args = append(args, since.UTC())
		conditions = append(conditions, fmt.Sprintf("attempted_at >= $%d", len(args)))
	}
	if filters.RecipientDomain != "" {
		args = append(args, filters.RecipientDomain)
		conditions = append(conditions, fmt.Sprintf("recipient_domain = $%d", len(args)))
	}
	if filters.MessageID != "" {
		args = append(args, filters.MessageID)
		conditions = append(conditions, fmt.Sprintf("message_id = $%d::uuid", len(args)))
	}
	if filters.Farm != "" {
		args = append(args, filters.Farm)
		conditions = append(conditions, fmt.Sprintf("farm = $%d", len(args)))
	}
	if filters.Sender != "" {
		args = append(args, filters.Sender)
		conditions = append(conditions, fmt.Sprintf("lower(sender) = $%d", len(args)))
	}
	if len(conditions) > 0 {
		return "\nWHERE " + strings.Join(conditions, "\n  AND "), args
	}
	return "", args
}

type deliveryAttemptFilters struct {
	Status          string
	RecipientDomain string
	MessageID       string
	Farm            string
	Sender          string
}

func normalizeDeliveryAttemptFilters(filters deliveryAttemptFilters) (deliveryAttemptFilters, error) {
	filters.Status = strings.ToLower(strings.TrimSpace(filters.Status))
	if filters.Status != "" && !allowedDeliveryAttemptStatus(filters.Status) {
		return deliveryAttemptFilters{}, fmt.Errorf("unsupported delivery attempt status")
	}
	var err error
	if filters.RecipientDomain, err = normalizeDeliveryAttemptTextFilter("recipient_domain", filters.RecipientDomain, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	filters.RecipientDomain = strings.Trim(filters.RecipientDomain, ".")
	if filters.MessageID, err = normalizeDeliveryAttemptTextFilter("message_id", filters.MessageID, false); err != nil {
		return deliveryAttemptFilters{}, err
	}
	if filters.Farm, err = normalizeDeliveryAttemptTextFilter("farm", filters.Farm, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	if filters.Sender, err = normalizeDeliveryAttemptTextFilter("sender", filters.Sender, true); err != nil {
		return deliveryAttemptFilters{}, err
	}
	return filters, nil
}

func normalizeDeliveryAttemptTextFilter(name string, value string, lower bool) (string, error) {
	value = strings.TrimSpace(value)
	if lower {
		value = strings.ToLower(value)
	}
	if err := validatePushNotificationFilter(name, value); err != nil {
		return "", err
	}
	return value, nil
}

func allowedDeliveryAttemptStatus(status string) bool {
	switch status {
	case "delivered", "failed", "bounced", "exhausted":
		return true
	default:
		return false
	}
}

func (r *Repository) GetDeliveryAttemptStats(ctx context.Context, req DeliveryAttemptStatsRequest) (DeliveryAttemptStatsView, error) {
	if r.db == nil {
		return DeliveryAttemptStatsView{}, fmt.Errorf("database handle is required")
	}
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		Status:          req.Status,
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return DeliveryAttemptStatsView{}, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}

	var stats DeliveryAttemptStatsView
	query, args := buildDeliveryAttemptStatsQuery(filters, req.Since)
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.TotalAttempts,
		&stats.UniqueMessages,
		&stats.UniqueRecipients,
		&stats.Delivered,
		&stats.Failed,
		&stats.Bounced,
		&stats.Exhausted,
	); err != nil {
		return DeliveryAttemptStatsView{}, fmt.Errorf("get delivery attempt stats: %w", err)
	}
	return stats, nil
}

func buildDeliveryAttemptStatsQuery(filters deliveryAttemptFilters, since time.Time) (string, []any) {
	whereClause, args := buildDeliveryAttemptWhereClause(filters, since)
	return deliveryAttemptStatsBaseSQL + whereClause, args
}

func (r *Repository) ListExhaustedAttempts(ctx context.Context, req ExhaustedAttemptListRequest) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req.Limit = normalizeLimit(req.Limit)
	filters, err := normalizeDeliveryAttemptFilters(deliveryAttemptFilters{
		RecipientDomain: req.RecipientDomain,
		MessageID:       req.MessageID,
		Farm:            req.Farm,
		Sender:          req.Sender,
	})
	if err != nil {
		return nil, err
	}
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}

	filters.Status = "exhausted"
	query, args := buildListDeliveryAttemptsQuery(filters, req.Since, req.Limit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list exhausted delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := scanDeliveryAttempt(rows, &attempt); err != nil {
			return nil, fmt.Errorf("scan exhausted delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exhausted delivery attempts: %w", err)
	}
	return attempts, nil
}

