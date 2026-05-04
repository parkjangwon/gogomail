package maildb

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	PushPlatformAPNS    = "apns"
	PushPlatformFCM     = "fcm"
	PushPlatformWebPush = "webpush"

	maxPushDeviceUserIDBytes = 200
	maxPushDeviceTokenBytes  = 4096
	maxPushDeviceLabelBytes  = 200
)

type PushDevice struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id,omitempty"`
	Platform    string    `json:"platform"`
	Token       string    `json:"-"`
	TokenSuffix string    `json:"token_suffix,omitempty"`
	Label       string    `json:"label,omitempty"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type UpsertPushDeviceRequest struct {
	UserID   string `json:"user_id,omitempty"`
	Platform string `json:"platform"`
	Token    string `json:"token"`
	Label    string `json:"label,omitempty"`
}

func ValidateUpsertPushDeviceRequest(req UpsertPushDeviceRequest) error {
	userID := strings.TrimSpace(req.UserID)
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.ContainsAny(userID, "\r\n") || len(userID) > maxPushDeviceUserIDBytes {
		return fmt.Errorf("user_id is invalid")
	}
	if !utf8.ValidString(userID) {
		return fmt.Errorf("user_id must be valid UTF-8")
	}
	if !allowedPushPlatform(req.Platform) {
		return fmt.Errorf("platform must be apns, fcm, or webpush")
	}
	token := strings.TrimSpace(req.Token)
	if token == "" {
		return fmt.Errorf("token is required")
	}
	if len(token) > maxPushDeviceTokenBytes {
		return fmt.Errorf("token is too long")
	}
	if len(req.Label) > maxPushDeviceLabelBytes {
		return fmt.Errorf("label is too long")
	}
	if !utf8.ValidString(token) || !utf8.ValidString(req.Label) {
		return fmt.Errorf("push device fields must be valid UTF-8")
	}
	if strings.ContainsAny(token, "\r\n") || strings.ContainsAny(req.Label, "\r\n") {
		return fmt.Errorf("push device fields must not contain line breaks")
	}
	return nil
}

func allowedPushPlatform(platform string) bool {
	switch strings.ToLower(strings.TrimSpace(platform)) {
	case PushPlatformAPNS, PushPlatformFCM, PushPlatformWebPush:
		return true
	default:
		return false
	}
}

func withPushDeviceTokenSuffix(device PushDevice) PushDevice {
	const suffixRunes = 8
	token := []rune(device.Token)
	if len(token) <= suffixRunes {
		device.TokenSuffix = string(token)
		return device
	}
	device.TokenSuffix = string(token[len(token)-suffixRunes:])
	return device
}

func (r *Repository) UpsertPushDevice(ctx context.Context, req UpsertPushDeviceRequest) (PushDevice, error) {
	if r.db == nil {
		return PushDevice{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateUpsertPushDeviceRequest(req); err != nil {
		return PushDevice{}, err
	}

	const query = `
INSERT INTO push_devices (user_id, platform, token, label, status)
VALUES ($1, $2, $3, $4, 'active')
ON CONFLICT (user_id, platform, token)
DO UPDATE SET
  label = EXCLUDED.label,
  status = 'active',
  updated_at = now()
RETURNING id::text, user_id::text, platform, token, label, status, created_at, updated_at`

	var device PushDevice
	if err := r.db.QueryRowContext(
		ctx,
		query,
		strings.TrimSpace(req.UserID),
		strings.ToLower(strings.TrimSpace(req.Platform)),
		strings.TrimSpace(req.Token),
		strings.TrimSpace(req.Label),
	).Scan(
		&device.ID,
		&device.UserID,
		&device.Platform,
		&device.Token,
		&device.Label,
		&device.Status,
		&device.CreatedAt,
		&device.UpdatedAt,
	); err != nil {
		return PushDevice{}, fmt.Errorf("upsert push device: %w", err)
	}
	return withPushDeviceTokenSuffix(device), nil
}

func (r *Repository) ListPushDevices(ctx context.Context, userID string, limit int) ([]PushDevice, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT id::text, user_id::text, platform, token, label, status, created_at, updated_at
FROM push_devices
WHERE user_id = $1
  AND status = 'active'
ORDER BY updated_at DESC, id DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list push devices: %w", err)
	}
	defer rows.Close()

	devices := make([]PushDevice, 0, limit)
	for rows.Next() {
		var device PushDevice
		if err := rows.Scan(
			&device.ID,
			&device.UserID,
			&device.Platform,
			&device.Token,
			&device.Label,
			&device.Status,
			&device.CreatedAt,
			&device.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan push device: %w", err)
		}
		devices = append(devices, withPushDeviceTokenSuffix(device))
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate push devices: %w", err)
	}
	return devices, nil
}

func (r *Repository) DeletePushDevice(ctx context.Context, userID string, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	id = strings.TrimSpace(id)
	if userID == "" {
		return fmt.Errorf("user_id is required")
	}
	if id == "" {
		return fmt.Errorf("id is required")
	}
	result, err := r.db.ExecContext(ctx, `UPDATE push_devices SET status = 'deleted', updated_at = now() WHERE user_id = $1 AND id = $2`, userID, id)
	if err != nil {
		return fmt.Errorf("delete push device: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("push device %q not found", id)
	}
	return nil
}
