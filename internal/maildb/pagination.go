package maildb

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

const MessageListDefaultLimit = 50
const MessageListMaxLimit = 200

type MessageListCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

func NormalizeMessageListLimit(limit int) int {
	if limit <= 0 {
		return MessageListDefaultLimit
	}
	if limit > MessageListMaxLimit {
		return MessageListMaxLimit
	}
	return limit
}

func EncodeMessageListCursor(cursor MessageListCursor) (string, error) {
	if cursor.At.IsZero() {
		return "", fmt.Errorf("cursor timestamp is required")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return "", fmt.Errorf("cursor message id is required")
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal message list cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeMessageListCursor(value string) (MessageListCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return MessageListCursor{}, nil
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return MessageListCursor{}, fmt.Errorf("decode message list cursor: %w", err)
	}
	var cursor MessageListCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return MessageListCursor{}, fmt.Errorf("unmarshal message list cursor: %w", err)
	}
	if cursor.At.IsZero() {
		return MessageListCursor{}, fmt.Errorf("cursor timestamp is required")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return MessageListCursor{}, fmt.Errorf("cursor message id is required")
	}
	return cursor, nil
}
