package maildb

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/gogomail/gogomail/internal/audit"
)

func (r *Repository) ListPushNotificationAttempts(ctx context.Context, req PushNotificationAttemptListRequest) ([]PushNotificationAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	var err error
	req, err = normalizePushNotificationAttemptListRequest(req)
	if err != nil {
		return nil, err
	}

	query, args := buildListPushNotificationAttemptsQuery(req)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list push notification attempts: %w", err)
	}
	defer rows.Close()

	var attempts []PushNotificationAttemptView
	for rows.Next() {
		var attempt PushNotificationAttemptView
		if err := rows.Scan(
			&attempt.ID,
			&attempt.MessageID,
			&attempt.RFCMessageID,
			&attempt.CompanyID,
			&attempt.DomainID,
			&attempt.UserID,
			&attempt.Recipient,
			&attempt.Subject,
			&attempt.DeviceID,
			&attempt.Platform,
			&attempt.TokenSuffix,
			&attempt.Status,
			&attempt.ErrorMessage,
			&attempt.ProviderMessageID,
			&attempt.ProviderStatus,
			&attempt.AttemptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan push notification attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate push notification attempts: %w", err)
	}
	return attempts, nil
}

func buildListPushNotificationAttemptsQuery(req PushNotificationAttemptListRequest) (string, []any) {
	query := listPushNotificationAttemptsBaseSQL
	whereClause, args := buildPushNotificationAttemptWhereClause(req)
	query += whereClause

	args = append(args, req.Limit)
	query += fmt.Sprintf(`
ORDER BY attempted_at DESC, id DESC
LIMIT $%d`, len(args))
	return query, args
}

func buildPushNotificationAttemptWhereClause(req PushNotificationAttemptListRequest) (string, []any) {
	var conditions []string
	var args []any

	if req.MessageID != "" {
		args = append(args, req.MessageID)
		conditions = append(conditions, fmt.Sprintf("message_id = $%d::uuid", len(args)))
	}
	if req.Status != "" {
		args = append(args, req.Status)
		conditions = append(conditions, fmt.Sprintf("status = $%d", len(args)))
	}
	if req.UserID != "" {
		args = append(args, req.UserID)
		conditions = append(conditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		conditions = append(conditions, fmt.Sprintf("attempted_at >= $%d", len(args)))
	}
	if req.Platform != "" {
		args = append(args, req.Platform)
		conditions = append(conditions, fmt.Sprintf("platform = $%d", len(args)))
	}
	if req.DeviceID != "" {
		args = append(args, req.DeviceID)
		conditions = append(conditions, fmt.Sprintf("device_id = $%d::uuid", len(args)))
	}
	if req.ProviderStatus != "" {
		args = append(args, req.ProviderStatus)
		conditions = append(conditions, fmt.Sprintf("provider_status = $%d", len(args)))
	}
	if req.ProviderMessageID != "" {
		args = append(args, req.ProviderMessageID)
		conditions = append(conditions, fmt.Sprintf("provider_message_id = $%d", len(args)))
	}
	if len(conditions) > 0 {
		return "\nWHERE " + strings.Join(conditions, "\n  AND "), args
	}
	return "", args
}

func (r *Repository) GetPushNotificationAttempt(ctx context.Context, id string) (PushNotificationAttemptView, error) {
	if r.db == nil {
		return PushNotificationAttemptView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if err := validatePushNotificationFilter("attempt_id", id); err != nil {
		return PushNotificationAttemptView{}, err
	}
	if id == "" {
		return PushNotificationAttemptView{}, fmt.Errorf("attempt_id is required")
	}

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  user_id::text,
  recipient,
  subject,
  COALESCE(device_id::text, ''),
  platform,
  token_suffix,
  status,
  error_message,
  provider_message_id,
  provider_status,
  attempted_at
FROM push_notification_attempts
WHERE id = $1::uuid`

	var attempt PushNotificationAttemptView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.RFCMessageID,
		&attempt.CompanyID,
		&attempt.DomainID,
		&attempt.UserID,
		&attempt.Recipient,
		&attempt.Subject,
		&attempt.DeviceID,
		&attempt.Platform,
		&attempt.TokenSuffix,
		&attempt.Status,
		&attempt.ErrorMessage,
		&attempt.ProviderMessageID,
		&attempt.ProviderStatus,
		&attempt.AttemptedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return PushNotificationAttemptView{}, fmt.Errorf("push notification attempt %q not found", id)
		}
		return PushNotificationAttemptView{}, fmt.Errorf("get push notification attempt: %w", err)
	}
	return attempt, nil
}

func normalizePushNotificationAttemptListRequest(req PushNotificationAttemptListRequest) (PushNotificationAttemptListRequest, error) {
	req.Limit = normalizeLimit(req.Limit)
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	req.ProviderStatus = strings.TrimSpace(req.ProviderStatus)
	req.ProviderMessageID = strings.TrimSpace(req.ProviderMessageID)
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	for field, value := range map[string]string{
		"message_id":          req.MessageID,
		"status":              req.Status,
		"user_id":             req.UserID,
		"platform":            req.Platform,
		"device_id":           req.DeviceID,
		"provider_status":     req.ProviderStatus,
		"provider_message_id": req.ProviderMessageID,
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return PushNotificationAttemptListRequest{}, err
		}
	}
	if req.Status != "" && !allowedPushNotificationAttemptStatus(req.Status) {
		return PushNotificationAttemptListRequest{}, fmt.Errorf("unsupported push notification attempt status")
	}
	if req.Platform != "" && !allowedPushPlatform(req.Platform) {
		return PushNotificationAttemptListRequest{}, fmt.Errorf("unsupported push notification platform")
	}
	return req, nil
}

func allowedPushNotificationAttemptStatus(status string) bool {
	switch status {
	case "candidate", "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}

func (r *Repository) UpdatePushNotificationOutcome(ctx context.Context, req UpdatePushNotificationOutcomeRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	normalized, err := normalizeUpdatePushNotificationOutcomeRequest(req)
	if err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin push notification outcome transaction: %w", err)
	}
	defer tx.Rollback()

	attempt, err := readPushNotificationAttemptForUpdate(ctx, tx, normalized.AttemptID)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE push_notification_attempts
SET status = $2,
    error_message = $3,
    provider_message_id = $4,
    provider_status = $5,
    attempted_at = now()
WHERE id = $1::uuid`,
		normalized.AttemptID,
		normalized.Status,
		normalized.ErrorMessage,
		normalized.ProviderMessageID,
		normalized.ProviderStatus,
	); err != nil {
		return fmt.Errorf("update push notification outcome: %w", err)
	}

	invalidTokenDeviceDeleted := false
	if normalized.Status == "invalid_token" && strings.TrimSpace(attempt.DeviceID) != "" {
		result, err := tx.ExecContext(
			ctx,
			`UPDATE push_devices SET status = 'deleted', updated_at = now() WHERE id = $1::uuid AND user_id = $2::uuid`,
			attempt.DeviceID,
			attempt.UserID,
		)
		if err != nil {
			return fmt.Errorf("delete invalid push device: %w", err)
		}
		if affected, err := result.RowsAffected(); err == nil && affected > 0 {
			invalidTokenDeviceDeleted = true
		}
	}

	detail, err := pushNotificationOutcomeAuditDetail(attempt, normalized, invalidTokenDeviceDeleted)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  attempt.CompanyID,
		DomainID:   attempt.DomainID,
		UserID:     attempt.UserID,
		Category:   "admin",
		Action:     "push_notification.outcome_update",
		TargetType: "push_notification_attempt",
		TargetID:   attempt.ID,
		Result:     normalized.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record push notification outcome audit: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit push notification outcome: %w", err)
	}
	return nil
}

func readPushNotificationAttemptForUpdate(ctx context.Context, tx *sql.Tx, id string) (PushNotificationAttemptView, error) {
	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  COALESCE(company_id::text, ''),
  COALESCE(domain_id::text, ''),
  user_id::text,
  recipient,
  subject,
  COALESCE(device_id::text, ''),
  platform,
  token_suffix,
  status,
  error_message,
  provider_message_id,
  provider_status,
  attempted_at
FROM push_notification_attempts
WHERE id = $1::uuid
FOR UPDATE`

	var attempt PushNotificationAttemptView
	if err := tx.QueryRowContext(ctx, query, id).Scan(
		&attempt.ID,
		&attempt.MessageID,
		&attempt.RFCMessageID,
		&attempt.CompanyID,
		&attempt.DomainID,
		&attempt.UserID,
		&attempt.Recipient,
		&attempt.Subject,
		&attempt.DeviceID,
		&attempt.Platform,
		&attempt.TokenSuffix,
		&attempt.Status,
		&attempt.ErrorMessage,
		&attempt.ProviderMessageID,
		&attempt.ProviderStatus,
		&attempt.AttemptedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return PushNotificationAttemptView{}, fmt.Errorf("push notification attempt %q not found", id)
		}
		return PushNotificationAttemptView{}, fmt.Errorf("read push notification attempt for outcome audit: %w", err)
	}
	return attempt, nil
}

func pushNotificationOutcomeAuditDetail(attempt PushNotificationAttemptView, update UpdatePushNotificationOutcomeRequest, invalidTokenDeviceDeleted bool) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"attempt_id":                   attempt.ID,
		"message_id":                   attempt.MessageID,
		"rfc_message_id":               attempt.RFCMessageID,
		"platform":                     attempt.Platform,
		"device_id":                    attempt.DeviceID,
		"previous_status":              attempt.Status,
		"status":                       update.Status,
		"previous_error_message":       attempt.ErrorMessage,
		"error_message":                update.ErrorMessage,
		"previous_provider_message_id": attempt.ProviderMessageID,
		"provider_message_id":          update.ProviderMessageID,
		"previous_provider_status":     attempt.ProviderStatus,
		"provider_status":              update.ProviderStatus,
		"invalid_token_device_deleted": invalidTokenDeviceDeleted,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal push notification outcome audit detail: %w", err)
	}
	return detail, nil
}

func normalizeUpdatePushNotificationOutcomeRequest(req UpdatePushNotificationOutcomeRequest) (UpdatePushNotificationOutcomeRequest, error) {
	req.AttemptID = strings.TrimSpace(req.AttemptID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	req.ErrorMessage = cleanAdminBoundedText(req.ErrorMessage, 2000)
	req.ProviderMessageID = cleanAdminBoundedText(req.ProviderMessageID, 500)
	req.ProviderStatus = cleanAdminBoundedText(req.ProviderStatus, 500)
	if err := validatePushNotificationFilter("attempt_id", req.AttemptID); err != nil {
		return UpdatePushNotificationOutcomeRequest{}, err
	}
	if req.AttemptID == "" {
		return UpdatePushNotificationOutcomeRequest{}, fmt.Errorf("attempt_id is required")
	}
	if !allowedPushNotificationOutcomeStatus(req.Status) {
		return UpdatePushNotificationOutcomeRequest{}, fmt.Errorf("unsupported push notification outcome status")
	}
	return req, nil
}

func allowedPushNotificationOutcomeStatus(status string) bool {
	switch status {
	case "queued", "delivered", "failed", "invalid_token":
		return true
	default:
		return false
	}
}

func (r *Repository) GetPushNotificationStats(ctx context.Context, req PushNotificationStatsRequest) (PushNotificationStatsView, error) {
	if r.db == nil {
		return PushNotificationStatsView{}, fmt.Errorf("database handle is required")
	}
	var err error
	req, err = normalizePushNotificationStatsRequest(req)
	if err != nil {
		return PushNotificationStatsView{}, err
	}

	var stats PushNotificationStatsView
	query, args := buildPushNotificationStatsQuery(req)
	if err := r.db.QueryRowContext(ctx, query, args...).Scan(
		&stats.ActiveDevices,
		&stats.TotalAttempts,
		&stats.Candidate,
		&stats.Queued,
		&stats.Delivered,
		&stats.Failed,
		&stats.InvalidToken,
	); err != nil {
		return PushNotificationStatsView{}, fmt.Errorf("get push notification stats: %w", err)
	}
	return stats, nil
}

func buildPushNotificationStatsQuery(req PushNotificationStatsRequest) (string, []any) {
	var args []any
	activeConditions := []string{"status = 'active'"}
	var attemptConditions []string

	if req.UserID != "" {
		args = append(args, req.UserID)
		activeConditions = append(activeConditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
		attemptConditions = append(attemptConditions, fmt.Sprintf("user_id = $%d::uuid", len(args)))
	}
	if req.MessageID != "" {
		args = append(args, req.MessageID)
		attemptConditions = append(attemptConditions, fmt.Sprintf("message_id = $%d::uuid", len(args)))
	}
	if req.Platform != "" {
		args = append(args, req.Platform)
		activeConditions = append(activeConditions, fmt.Sprintf("platform = $%d", len(args)))
		attemptConditions = append(attemptConditions, fmt.Sprintf("platform = $%d", len(args)))
	}
	if req.DeviceID != "" {
		args = append(args, req.DeviceID)
		activeConditions = append(activeConditions, fmt.Sprintf("id = $%d::uuid", len(args)))
		attemptConditions = append(attemptConditions, fmt.Sprintf("device_id = $%d::uuid", len(args)))
	}
	if !req.Since.IsZero() {
		args = append(args, req.Since.UTC())
		attemptConditions = append(attemptConditions, fmt.Sprintf("attempted_at >= $%d", len(args)))
	}

	query := fmt.Sprintf(`
SELECT
  COALESCE((
    SELECT COUNT(*)
    FROM push_devices
    WHERE %s
  ), 0),
  COALESCE(COUNT(*), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'candidate'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'queued'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'delivered'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'failed'), 0),
  COALESCE(COUNT(*) FILTER (WHERE status = 'invalid_token'), 0)
FROM push_notification_attempts`, strings.Join(activeConditions, "\n      AND "))
	if len(attemptConditions) > 0 {
		query += "\nWHERE " + strings.Join(attemptConditions, "\n  AND ")
	}
	return query, args
}

func normalizePushNotificationStatsRequest(req PushNotificationStatsRequest) (PushNotificationStatsRequest, error) {
	req.MessageID = strings.TrimSpace(req.MessageID)
	req.UserID = strings.TrimSpace(req.UserID)
	req.Platform = strings.ToLower(strings.TrimSpace(req.Platform))
	req.DeviceID = strings.TrimSpace(req.DeviceID)
	if !req.Since.IsZero() {
		req.Since = req.Since.UTC()
	}
	for field, value := range map[string]string{
		"message_id": req.MessageID,
		"user_id":    req.UserID,
		"platform":   req.Platform,
		"device_id":  req.DeviceID,
	} {
		if err := validatePushNotificationFilter(field, value); err != nil {
			return PushNotificationStatsRequest{}, err
		}
	}
	if req.Platform != "" && !allowedPushPlatform(req.Platform) {
		return PushNotificationStatsRequest{}, fmt.Errorf("unsupported push notification platform")
	}
	return req, nil
}

func validatePushNotificationFilter(field string, value string) error {
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s must not contain line breaks", field)
	}
	if len(value) > maxPushNotificationFilterBytes {
		return fmt.Errorf("%s is too long", field)
	}
	if !utf8.ValidString(value) {
		return fmt.Errorf("%s must be valid UTF-8", field)
	}
	return nil
}

func cleanAdminBoundedText(value string, maxBytes int) string {
	value = strings.ToValidUTF8(strings.TrimSpace(value), "")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	cut := 0
	for i := range value {
		if i > maxBytes {
			return value[:cut]
		}
		cut = i
	}
	return value[:cut]
}
