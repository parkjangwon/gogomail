package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/searchindex"
)

func (s *Service) SearchMessages(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	query = normalizeMessageSearchQuery(query)
	if err := validateMessageSearchQuery(query); err != nil {
		return nil, err
	}
	if s.searchIDSource != nil && canUseSearchIDSource(query) {
		return s.searchMessagesByExternalIDs(ctx, query)
	}
	repo, ok := s.repository.(interface {
		SearchMessages(context.Context, maildb.MessageSearchQuery) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("search repository is required")
	}
	return repo.SearchMessages(ctx, query)
}

func (s *Service) SearchMessageIDs(ctx context.Context, query maildb.MessageSearchQuery) ([]string, error) {
	query = normalizeMessageSearchQuery(query)
	if err := validateMessageSearchQuery(query); err != nil {
		return nil, err
	}
	if s.searchIDSource != nil && canUseSearchIDSourceForIDs(query) {
		hits, err := s.searchIDSource.SearchMessageIDs(ctx, openSearchSearchQueryFromMessageSearchQuery(query, 200))
		if err != nil {
			return nil, err
		}
		return uniqueSearchHitIDs(hits), nil
	}
	repo, ok := s.repository.(interface {
		SearchMessages(context.Context, maildb.MessageSearchQuery) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("search repository is required")
	}
	messages, err := repo.SearchMessages(ctx, query)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(messages))
	for _, message := range messages {
		id := strings.TrimSpace(message.ID)
		if id == "" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}

func (s *Service) SearchDrafts(ctx context.Context, query maildb.DraftSearchQuery) ([]maildb.MessageDetail, error) {
	query = normalizeDraftSearchQuery(query)
	if err := validateDraftSearchQuery(query); err != nil {
		return nil, err
	}
	repo, ok := s.repository.(interface {
		SearchDrafts(context.Context, maildb.DraftSearchQuery) ([]maildb.MessageDetail, error)
	})
	if !ok {
		return nil, fmt.Errorf("draft search repository is required")
	}
	return repo.SearchDrafts(ctx, query)
}

func (s *Service) searchMessagesByExternalIDs(ctx context.Context, query maildb.MessageSearchQuery) ([]maildb.MessageSummary, error) {
	hydrator, ok := s.repository.(interface {
		ListMessagesByIDs(context.Context, string, []string) ([]maildb.MessageSummary, error)
	})
	if !ok {
		return nil, fmt.Errorf("search hydration repository is required")
	}
	hits, err := s.searchIDSource.SearchMessageIDs(ctx, openSearchSearchQueryFromMessageSearchQuery(query, query.Limit))
	if err != nil {
		return nil, err
	}
	messageIDs, ranks, highlights := searchMessageIDsFromHits(hits)
	messages, err := hydrator.ListMessagesByIDs(ctx, query.UserID, messageIDs)
	if err != nil {
		return nil, err
	}
	if query.IncludeRank {
		for i := range messages {
			if rank, ok := ranks[messages[i].ID]; ok {
				messages[i].SearchRank = &rank
			}
		}
	}
	if query.IncludeHighlights {
		for i := range messages {
			if highlight, ok := highlights[messages[i].ID]; ok {
				messages[i].SearchHighlights = &maildb.MessageSearchHighlights{
					Subject: append([]string(nil), highlight.Subject...),
					From:    append([]string(nil), highlight.From...),
					Body:    append([]string(nil), highlight.Body...),
				}
			}
		}
	}
	return messages, nil
}

func canUseSearchIDSource(query maildb.MessageSearchQuery) bool {
	return strings.TrimSpace(query.Query) != "" &&
		normalizedSearchSort(query.Sort) == maildb.MessageSearchSortRelevance
}

func canUseSearchIDSourceForIDs(query maildb.MessageSearchQuery) bool {
	return strings.TrimSpace(query.Query) != "" ||
		strings.TrimSpace(query.FolderID) != "" ||
		strings.TrimSpace(query.From) != "" ||
		strings.TrimSpace(query.To) != "" ||
		strings.TrimSpace(query.Cc) != "" ||
		strings.TrimSpace(query.Bcc) != "" ||
		strings.TrimSpace(query.Subject) != "" ||
		query.HasAttachment != nil ||
		strings.TrimSpace(query.Since) != "" ||
		strings.TrimSpace(query.Until) != ""
}

func openSearchSearchQueryFromMessageSearchQuery(query maildb.MessageSearchQuery, limit int) searchindex.OpenSearchSearchQuery {
	return searchindex.OpenSearchSearchQuery{
		UserID:            query.UserID,
		FolderID:          query.FolderID,
		Query:             query.Query,
		From:              query.From,
		To:                query.To,
		Cc:                query.Cc,
		Bcc:               query.Bcc,
		Subject:           query.Subject,
		HasAttachment:     query.HasAttachment,
		Since:             query.Since,
		Until:             query.Until,
		IncludeHighlights: query.IncludeHighlights,
		Limit:             limit,
	}
}

func searchMessageIDsFromHits(hits []searchindex.OpenSearchHit) ([]string, map[string]float64, map[string]searchindex.OpenSearchHighlights) {
	messageIDs := make([]string, 0, len(hits))
	ranks := make(map[string]float64, len(hits))
	highlights := make(map[string]searchindex.OpenSearchHighlights, len(hits))
	for _, hit := range hits {
		id := strings.TrimSpace(hit.MessageID)
		if id == "" {
			continue
		}
		if _, ok := ranks[id]; ok {
			continue
		}
		messageIDs = append(messageIDs, id)
		ranks[id] = hit.Score
		highlights[id] = hit.Highlights
	}
	return messageIDs, ranks, highlights
}

func uniqueSearchHitIDs(hits []searchindex.OpenSearchHit) []string {
	messageIDs, _, _ := searchMessageIDsFromHits(hits)
	return messageIDs
}

func normalizedSearchSort(sort string) string {
	switch strings.ToLower(strings.TrimSpace(sort)) {
	case "", maildb.MessageSearchSortDate:
		return maildb.MessageSearchSortDate
	case maildb.MessageSearchSortRelevance:
		return maildb.MessageSearchSortRelevance
	default:
		return ""
	}
}

func normalizeMessageSearchQuery(query maildb.MessageSearchQuery) maildb.MessageSearchQuery {
	query.UserID = strings.TrimSpace(query.UserID)
	query.Query = strings.TrimSpace(query.Query)
	query.FolderID = strings.TrimSpace(query.FolderID)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Cc = strings.TrimSpace(query.Cc)
	query.Bcc = strings.TrimSpace(query.Bcc)
	query.Subject = strings.TrimSpace(query.Subject)
	if sort := normalizedSearchSort(query.Sort); sort != "" {
		query.Sort = sort
	} else {
		query.Sort = strings.TrimSpace(query.Sort)
	}
	return query
}

const maxSearchFilterBytes = 1024

func validateMessageSearchQuery(query maildb.MessageSearchQuery) error {
	if query.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if query.FolderID != "" {
		if err := validateServiceResourceID("folder_id", query.FolderID); err != nil {
			return err
		}
	}
	for field, value := range map[string]string{
		"q":       query.Query,
		"from":    query.From,
		"to":      query.To,
		"cc":      query.Cc,
		"bcc":     query.Bcc,
		"subject": query.Subject,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain CR or LF", field)
		}
		if len(value) > maxSearchFilterBytes {
			return fmt.Errorf("%s is too long", field)
		}
	}
	return nil
}

func normalizeDraftSearchQuery(query maildb.DraftSearchQuery) maildb.DraftSearchQuery {
	query.UserID = strings.TrimSpace(query.UserID)
	query.Query = strings.TrimSpace(query.Query)
	query.From = strings.TrimSpace(query.From)
	query.To = strings.TrimSpace(query.To)
	query.Cc = strings.TrimSpace(query.Cc)
	query.Bcc = strings.TrimSpace(query.Bcc)
	query.Subject = strings.TrimSpace(query.Subject)
	query.Limit = maildb.NormalizeMessageListLimit(query.Limit)
	return query
}

func validateDraftSearchQuery(query maildb.DraftSearchQuery) error {
	if query.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	for field, value := range map[string]string{
		"q":       query.Query,
		"from":    query.From,
		"to":      query.To,
		"cc":      query.Cc,
		"bcc":     query.Bcc,
		"subject": query.Subject,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain CR or LF", field)
		}
		if len(value) > maxSearchFilterBytes {
			return fmt.Errorf("%s is too long", field)
		}
	}
	return nil
}