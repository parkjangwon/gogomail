package mailservice

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

type SaveDraftRequest struct {
	UserID          string             `json:"user_id"`
	DraftID         string             `json:"draft_id,omitempty"`
	Intent          ComposeIntent      `json:"intent"`
	SourceMessageID string             `json:"source_message_id,omitempty"`
	From            string             `json:"from,omitempty"`
	To              []outbound.Address `json:"to,omitempty"`
	Cc              []outbound.Address `json:"cc,omitempty"`
	Bcc             []outbound.Address `json:"bcc,omitempty"`
	Subject         string             `json:"subject"`
	TextBody        string             `json:"text_body"`
	AttachmentIDs   []string           `json:"attachment_ids,omitempty"`
}

func ValidateSaveDraftRequest(req SaveDraftRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return err
	}
	if (intent == ComposeIntentReply || intent == ComposeIntentForward) && strings.TrimSpace(req.SourceMessageID) == "" {
		return fmt.Errorf("source_message_id is required for %s", intent)
	}
	if len(req.Subject) > MaxComposeSubjectBytes {
		return fmt.Errorf("subject is too long")
	}
	for _, id := range req.AttachmentIDs {
		if strings.TrimSpace(id) == "" {
			return fmt.Errorf("attachment id must not be blank")
		}
	}
	return nil
}

func ValidateDeleteDraftRequest(userID string, draftID string) error {
	if strings.TrimSpace(userID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(draftID) == "" {
		return fmt.Errorf("draft_id is required")
	}
	return nil
}
