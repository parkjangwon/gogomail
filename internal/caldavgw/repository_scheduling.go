package caldavgw

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (r *Repository) DeliverSchedulingMessage(ctx context.Context, req DeliverSchedulingMessageRequest) (SchedulingMessage, error) {
	if r == nil || r.db == nil {
		return SchedulingMessage{}, fmt.Errorf("database handle is required")
	}
	req, etag, err := ValidateDeliverSchedulingMessageRequest(req)
	if err != nil {
		return SchedulingMessage{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("begin scheduling deliver: %w", err)
	}
	defer tx.Rollback()
	inboxID, err := r.getSchedulingInboxID(ctx, tx, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SchedulingMessage{}, fmt.Errorf("scheduling inbox not found")
		}
		return SchedulingMessage{}, fmt.Errorf("lookup scheduling inbox: %w", err)
	}
	objectName := fmt.Sprintf("%s.ics", req.UID)
	var existingObject string
	err = tx.QueryRowContext(ctx, `
SELECT object_name
FROM caldav_calendar_objects
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND uid = $3
  AND status = 'active'`, req.UserID, inboxID, req.UID).Scan(&existingObject)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return SchedulingMessage{}, fmt.Errorf("check existing scheduling object: %w", err)
	}
	if existingObject != "" {
		objectName = existingObject
	}
	var msg SchedulingMessage
	if req.Method == ScheduleMethodCancel || req.Method == ScheduleMethodDeclineCounter {
		if existingObject != "" {
			_, err = tx.ExecContext(ctx, `
UPDATE caldav_calendar_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND calendar_id = $2::uuid
  AND object_name = $3
  AND status = 'active'`, req.UserID, inboxID, objectName)
			if err != nil {
				return SchedulingMessage{}, fmt.Errorf("cancel scheduling object: %w", err)
			}
		}
		msg = SchedulingMessage{
			UserID:       req.UserID,
			Recipient:    req.Recipient,
			Method:       req.Method,
			UID:          req.UID,
			ICSPayload:   req.ICSPayload,
			ETag:         etag,
			ProcessedAt:  time.Now().UTC(),
			ResponseCode: "2.0;success",
		}
	} else {
		_, err = tx.ExecContext(ctx, `
INSERT INTO caldav_calendar_objects (
  user_id, calendar_id, object_name, uid, component_type, etag, size, ics
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
ON CONFLICT (calendar_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  component_type = EXCLUDED.component_type,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  ics = EXCLUDED.ics,
  updated_at = now()`, req.UserID, inboxID, objectName, req.UID, strings.ToUpper(string(req.Method)), etag, len(req.ICSPayload), string(req.ICSPayload))
		if err != nil {
			return SchedulingMessage{}, fmt.Errorf("store scheduling object: %w", err)
		}
		component, _ := parseICSScheduleMethod(req.ICSPayload)
		if component == "" {
			component = "VEVENT"
		}
		msg = SchedulingMessage{
			UserID:       req.UserID,
			Recipient:    req.Recipient,
			Method:       req.Method,
			UID:          req.UID,
			ICSPayload:   req.ICSPayload,
			ETag:         etag,
			ProcessedAt:  time.Now().UTC(),
			ResponseCode: "2.0;success",
		}
		_ = component
	}
	if err := tx.Commit(); err != nil {
		return SchedulingMessage{}, fmt.Errorf("commit scheduling deliver: %w", err)
	}
	return msg, nil
}

func (r *Repository) SendSchedulingMessage(ctx context.Context, req SendSchedulingMessageRequest) (SchedulingMessage, error) {
	if r == nil || r.db == nil {
		return SchedulingMessage{}, fmt.Errorf("database handle is required")
	}
	req, etag, err := ValidateSendSchedulingMessageRequest(req)
	if err != nil {
		return SchedulingMessage{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("begin scheduling send: %w", err)
	}
	defer tx.Rollback()
	outboxID, err := r.getSchedulingOutboxID(ctx, tx, req.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SchedulingMessage{}, fmt.Errorf("scheduling outbox not found")
		}
		return SchedulingMessage{}, fmt.Errorf("lookup scheduling outbox: %w", err)
	}
	objectName := fmt.Sprintf("%s.ics", req.UID)
	_, err = tx.ExecContext(ctx, `
INSERT INTO caldav_calendar_objects (
  user_id, calendar_id, object_name, uid, component_type, etag, size, ics
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8)
ON CONFLICT (calendar_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  component_type = EXCLUDED.component_type,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  ics = EXCLUDED.ics,
  updated_at = now()`, req.UserID, outboxID, objectName, req.UID, strings.ToUpper(string(req.Method)), etag, len(req.ICSPayload), string(req.ICSPayload))
	if err != nil {
		return SchedulingMessage{}, fmt.Errorf("store outbox object: %w", err)
	}
	if err := r.insertSchedulingOutboxEvent(ctx, tx, req.UserID, req.UID, string(req.Method), req.ICSPayload); err != nil {
		return SchedulingMessage{}, err
	}
	msg := SchedulingMessage{
		UserID:      req.UserID,
		Method:      req.Method,
		UID:         req.UID,
		ICSPayload:  req.ICSPayload,
		ETag:        etag,
		ProcessedAt: time.Now().UTC(),
	}
	if err := tx.Commit(); err != nil {
		return SchedulingMessage{}, fmt.Errorf("commit scheduling send: %w", err)
	}
	return msg, nil
}

func (r *Repository) getSchedulingInboxID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND name = 'inbox'
  AND status = 'active'
LIMIT 1`, userID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) getSchedulingOutboxID(ctx context.Context, tx *sql.Tx, userID string) (string, error) {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM caldav_calendars
WHERE user_id = $1::uuid
  AND name = 'outbox'
  AND status = 'active'
LIMIT 1`, userID).Scan(&id)
	if err != nil {
		return "", err
	}
	return id, nil
}

func (r *Repository) insertSchedulingOutboxEvent(ctx context.Context, tx *sql.Tx, userID string, uid string, method string, payload []byte) error {
	partitionKey := strings.TrimSpace(userID)
	if partitionKey == "" {
		partitionKey = "unknown"
	}
	schedPayload, err := json.Marshal(map[string]any{
		"event":          "scheduling.outbox",
		"schema_version": "2026-05-08.scheduling.v1",
		"dav_kind":       "caldav-scheduling",
		"user_id":        userID,
		"uid":            uid,
		"method":         method,
		"payload":        string(payload),
		"created_at":     time.Now().UTC(),
	})
	if err != nil {
		return fmt.Errorf("marshal scheduling outbox event: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO outbox (topic, partition_key, payload, status)
VALUES ($1, $2, $3::jsonb, 'pending')`, "scheduling.outbox", partitionKey, string(schedPayload))
	if err != nil {
		return fmt.Errorf("insert scheduling outbox event: %w", err)
	}
	return nil
}

func ValidateDeliverSchedulingMessageRequest(req DeliverSchedulingMessageRequest) (DeliverSchedulingMessageRequest, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	recipient := strings.TrimSpace(req.Recipient)
	if recipient == "" {
		recipient = userID
	}
	uid, err := ValidateCalendarObjectUID(req.UID)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	if len(req.ICSPayload) == 0 {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload is required")
	}
	if len(req.ICSPayload) > MaxCalendarObjectBytes {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload exceeds maximum size")
	}
	parsed, err := ParseICalendarObjectForScheduling(req.ICSPayload)
	if err != nil {
		return DeliverSchedulingMessageRequest{}, "", err
	}
	if parsed.UID != uid {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("ICS UID does not match request UID")
	}
	method := req.Method
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethodForDelivery(method) {
		return DeliverSchedulingMessageRequest{}, "", fmt.Errorf("invalid schedule method for delivery: %s", method)
	}
	etag, _ := StrongETag(req.ICSPayload)
	return DeliverSchedulingMessageRequest{
		UserID:     userID,
		Recipient:  recipient,
		Method:     method,
		UID:        uid,
		ICSPayload: req.ICSPayload,
	}, etag, nil
}

func ValidateSendSchedulingMessageRequest(req SendSchedulingMessageRequest) (SendSchedulingMessageRequest, string, error) {
	userID, err := validateCalDAVID("user_id", req.UserID, true)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	uid, err := ValidateCalendarObjectUID(req.UID)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	if len(req.ICSPayload) == 0 {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload is required")
	}
	if len(req.ICSPayload) > MaxCalendarObjectBytes {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS payload exceeds maximum size")
	}
	parsed, err := ParseICalendarObjectForScheduling(req.ICSPayload)
	if err != nil {
		return SendSchedulingMessageRequest{}, "", err
	}
	if parsed.UID != uid {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("ICS UID does not match request UID")
	}
	method := req.Method
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethodForSend(method) {
		return SendSchedulingMessageRequest{}, "", fmt.Errorf("invalid schedule method for sending: %s", method)
	}
	etag, _ := StrongETag(req.ICSPayload)
	return SendSchedulingMessageRequest{
		UserID:     userID,
		Method:     method,
		UID:        uid,
		ICSPayload: req.ICSPayload,
	}, etag, nil
}

func isValidScheduleMethodForDelivery(method ScheduleMethod) bool {
	switch method {
	case ScheduleMethodRequest, ScheduleMethodReply, ScheduleMethodCancel,
		ScheduleMethodAdd, ScheduleMethodCounter, ScheduleMethodDeclineCounter,
		ScheduleMethodRefresh, ScheduleMethodPublish:
		return true
	default:
		return false
	}
}

func isValidScheduleMethodForSend(method ScheduleMethod) bool {
	switch method {
	case ScheduleMethodRequest, ScheduleMethodReply, ScheduleMethodCancel,
		ScheduleMethodAdd, ScheduleMethodCounter, ScheduleMethodDeclineCounter,
		ScheduleMethodPublish:
		return true
	default:
		return false
	}
}

func parseICSScheduleMethod(payload []byte) (string, error) {
	return ExtractICSMethod(payload)
}
