package mailservice

import (
	"context"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/maildb"
)

func (s *Service) SaveDraft(ctx context.Context, req SaveDraftRequest) (maildb.MessageDetail, error) {
	req = normalizeSaveDraftRequest(req)
	if err := ValidateSaveDraftRequest(req); err != nil {
		return maildb.MessageDetail{}, err
	}
	repo, ok := s.repository.(DraftRepository)
	if !ok {
		return maildb.MessageDetail{}, fmt.Errorf("draft repository is required")
	}
	return repo.SaveDraft(ctx, maildb.SaveDraftRequest{
		UserID:          req.UserID,
		DraftID:         req.DraftID,
		Intent:          string(req.Intent),
		SourceMessageID: req.SourceMessageID,
		From:            req.From,
		To:              req.To,
		Cc:              req.Cc,
		Bcc:             req.Bcc,
		Subject:         req.Subject,
		TextBody:        req.TextBody,
		HTMLBody:        req.HTMLBody,
		AttachmentIDs:   req.AttachmentIDs,
		TrackOpens:      req.TrackOpens,
		ScheduledAt:     req.ScheduledAt,
		IfUpdatedAt:     req.IfUpdatedAt,
	})
}

func normalizeSaveDraftRequest(req SaveDraftRequest) SaveDraftRequest {
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err == nil {
		req.Intent = intent
	}
	req.UserID = strings.TrimSpace(req.UserID)
	req.DraftID = strings.TrimSpace(req.DraftID)
	req.SourceMessageID = strings.TrimSpace(req.SourceMessageID)
	req.From = strings.TrimSpace(req.From)
	req.To = normalizeComposeAddresses(req.To)
	req.Cc = normalizeComposeAddresses(req.Cc)
	req.Bcc = normalizeComposeAddresses(req.Bcc)
	req.AttachmentIDs = normalizeStringList(req.AttachmentIDs)
	return req
}

func (s *Service) DeleteDraft(ctx context.Context, userID string, draftID string) error {
	userID = strings.TrimSpace(userID)
	draftID = strings.TrimSpace(draftID)
	if err := ValidateDeleteDraftRequest(userID, draftID); err != nil {
		return fmt.Errorf("delete draft: %w", err)
	}
	repo, ok := s.repository.(DraftRepository)
	if !ok {
		return fmt.Errorf("draft repository is required")
	}
	return repo.DeleteDraft(ctx, userID, draftID)
}