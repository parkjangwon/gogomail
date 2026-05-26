package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

func (s *Service) ListThreads(ctx context.Context, userID string, limit int) ([]maildb.ThreadSummary, error) {
	return s.ListThreadsPage(ctx, userID, limit, maildb.ThreadListCursor{}, maildb.ThreadListFilter{})
}

func (s *Service) ListThreadsPage(ctx context.Context, userID string, limit int, cursor maildb.ThreadListCursor, filter maildb.ThreadListFilter) ([]maildb.ThreadSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreadsPage(context.Context, string, int, maildb.ThreadListCursor, maildb.ThreadListFilter) ([]maildb.ThreadSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("thread repository is required")
	}
	userID = strings.TrimSpace(userID)
	filter.FolderID = strings.TrimSpace(filter.FolderID)
	if filter.FolderID != "" {
		if err := validateServiceResourceID("folder_id", filter.FolderID); err != nil {
			return nil, err
		}
	}
	sortMode, ok := maildb.NormalizeListSort(filter.Sort)
	if !ok {
		return nil, fmt.Errorf("sort must be newest or oldest")
	}
	filter.Sort = sortMode
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListThreadsPage(ctx, userID, limit, cursor, filter)
}

func (s *Service) ListThreadMessages(ctx context.Context, userID string, threadID string, limit int) ([]maildb.MessageSummary, error) {
	return s.ListThreadMessagesPage(ctx, userID, threadID, limit, maildb.MessageListCursor{})
}

func (s *Service) ListThreadMessagesPage(ctx context.Context, userID string, threadID string, limit int, cursor maildb.MessageListCursor) ([]maildb.MessageSummary, error) {
	repo, ok := s.repository.(interface {
		ListThreadMessagesPage(context.Context, string, string, int, maildb.MessageListCursor) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("thread repository is required")
	}
	userID = strings.TrimSpace(userID)
	threadID = strings.TrimSpace(threadID)
	if err := validateServiceResourceID("thread_id", threadID); err != nil {
		return nil, err
	}
	limit = maildb.NormalizeMessageListLimit(limit)
	return repo.ListThreadMessagesPage(ctx, userID, threadID, limit, cursor)
}