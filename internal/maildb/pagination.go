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
const MessageListCursorMaxBytes = 1024

const (
	ListSortNewest = "newest"
	ListSortOldest = "oldest"
)

type MessageListCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

type MessageListFilter struct {
	Read          *bool
	Starred       *bool
	HasAttachment *bool
	Sort          string
}

type MessageListPage struct {
	Messages   []MessageSummary `json:"messages"`
	Limit      int              `json:"limit"`
	HasMore    bool             `json:"has_more"`
	NextCursor string           `json:"next_cursor,omitempty"`
}

type ThreadListCursor struct {
	At time.Time `json:"at"`
	ID string    `json:"id"`
}

type ThreadListFilter struct {
	FolderID      string
	Read          *bool
	Starred       *bool
	HasAttachment *bool
	Sort          string
}

type ThreadListPage struct {
	Threads    []ThreadSummary `json:"threads"`
	Limit      int             `json:"limit"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

type DraftListPage struct {
	Drafts     []MessageDetail `json:"drafts"`
	Limit      int             `json:"limit"`
	HasMore    bool            `json:"has_more"`
	NextCursor string          `json:"next_cursor,omitempty"`
}

func NewMessageListPage(messages []MessageSummary, requestedLimit int) (MessageListPage, error) {
	limit := NormalizeMessageListLimit(requestedLimit)
	hasMore := len(messages) > limit
	if hasMore {
		messages = messages[:limit]
	}
	page := MessageListPage{
		Messages: messages,
		Limit:    limit,
		HasMore:  hasMore,
	}
	if len(messages) == 0 {
		return page, nil
	}
	last := messages[len(messages)-1]
	if last.ID == "" || last.ReceivedAt.IsZero() {
		return page, nil
	}
	next, err := EncodeMessageListCursor(MessageListCursor{At: last.ReceivedAt, ID: last.ID})
	if err != nil {
		return MessageListPage{}, err
	}
	page.NextCursor = next
	return page, nil
}

func NewDraftListPage(drafts []MessageDetail, requestedLimit int) (DraftListPage, error) {
	limit := NormalizeMessageListLimit(requestedLimit)
	hasMore := len(drafts) > limit
	if hasMore {
		drafts = drafts[:limit]
	}
	page := DraftListPage{
		Drafts:  drafts,
		Limit:   limit,
		HasMore: hasMore,
	}
	if len(drafts) == 0 {
		return page, nil
	}
	last := drafts[len(drafts)-1]
	if last.ID == "" || last.ReceivedAt.IsZero() {
		return page, nil
	}
	next, err := EncodeMessageListCursor(MessageListCursor{At: last.ReceivedAt, ID: last.ID})
	if err != nil {
		return DraftListPage{}, err
	}
	page.NextCursor = next
	return page, nil
}

func NewThreadListPage(threads []ThreadSummary, requestedLimit int) (ThreadListPage, error) {
	limit := NormalizeMessageListLimit(requestedLimit)
	hasMore := len(threads) > limit
	if hasMore {
		threads = threads[:limit]
	}
	page := ThreadListPage{
		Threads: threads,
		Limit:   limit,
		HasMore: hasMore,
	}
	if len(threads) == 0 {
		return page, nil
	}
	last := threads[len(threads)-1]
	if last.ID == "" || last.LatestAt.IsZero() {
		return page, nil
	}
	next, err := EncodeThreadListCursor(ThreadListCursor{At: last.LatestAt, ID: last.ID})
	if err != nil {
		return ThreadListPage{}, err
	}
	page.NextCursor = next
	return page, nil
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

func NormalizeListSort(sort string) (string, bool) {
	sort = strings.ToLower(strings.TrimSpace(sort))
	if sort == "" {
		return ListSortNewest, true
	}
	switch sort {
	case ListSortNewest, ListSortOldest:
		return sort, true
	default:
		return "", false
	}
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
	if len(value) > MessageListCursorMaxBytes {
		return MessageListCursor{}, fmt.Errorf("message list cursor is too long")
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
	if !isUUIDLike(cursor.ID) {
		return MessageListCursor{}, fmt.Errorf("cursor message id must be a UUID")
	}
	return cursor, nil
}

func EncodeThreadListCursor(cursor ThreadListCursor) (string, error) {
	if cursor.At.IsZero() {
		return "", fmt.Errorf("cursor timestamp is required")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return "", fmt.Errorf("cursor thread id is required")
	}
	raw, err := json.Marshal(cursor)
	if err != nil {
		return "", fmt.Errorf("marshal thread list cursor: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}

func DecodeThreadListCursor(value string) (ThreadListCursor, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return ThreadListCursor{}, nil
	}
	if len(value) > MessageListCursorMaxBytes {
		return ThreadListCursor{}, fmt.Errorf("thread list cursor is too long")
	}
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return ThreadListCursor{}, fmt.Errorf("decode thread list cursor: %w", err)
	}
	var cursor ThreadListCursor
	if err := json.Unmarshal(raw, &cursor); err != nil {
		return ThreadListCursor{}, fmt.Errorf("unmarshal thread list cursor: %w", err)
	}
	if cursor.At.IsZero() {
		return ThreadListCursor{}, fmt.Errorf("cursor timestamp is required")
	}
	if strings.TrimSpace(cursor.ID) == "" {
		return ThreadListCursor{}, fmt.Errorf("cursor thread id is required")
	}
	if !isUUIDLike(cursor.ID) {
		return ThreadListCursor{}, fmt.Errorf("cursor thread id must be a UUID")
	}
	return cursor, nil
}

func isUUIDLike(value string) bool {
	value = strings.ToLower(strings.TrimSpace(value))
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if (ch < '0' || ch > '9') && (ch < 'a' || ch > 'f') {
				return false
			}
		}
	}
	return true
}
