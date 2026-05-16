package mailservice

import (
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type SaveDraftRequest struct {
	UserID          string             `json:"user_id"`
	UserEmail       string             `json:"user_email,omitempty"`
	DraftID         string             `json:"draft_id,omitempty"`
	Intent          ComposeIntent      `json:"intent"`
	SourceMessageID string             `json:"source_message_id,omitempty"`
	From            string             `json:"from,omitempty"`
	To              []outbound.Address `json:"to,omitempty"`
	Cc              []outbound.Address `json:"cc,omitempty"`
	Bcc             []outbound.Address `json:"bcc,omitempty"`
	Subject         string             `json:"subject"`
	TextBody        string             `json:"text_body"`
	HTMLBody        string             `json:"html_body,omitempty"`
	AttachmentIDs   []string           `json:"attachment_ids,omitempty"`
	TrackOpens      bool               `json:"track_opens,omitempty"`
	ScheduledAt     time.Time          `json:"scheduled_at,omitempty"`
}

func ValidateSaveDraftRequest(req SaveDraftRequest) error {
	if strings.TrimSpace(req.UserID) == "" {
		return fmt.Errorf("user_id is required")
	}
	if strings.TrimSpace(req.DraftID) != "" {
		if err := validateServiceResourceID("draft_id", req.DraftID); err != nil {
			return err
		}
	}
	intent, err := NormalizeComposeIntent(string(req.Intent))
	if err != nil {
		return err
	}
	if (intent == ComposeIntentReply || intent == ComposeIntentForward) && strings.TrimSpace(req.SourceMessageID) == "" {
		return fmt.Errorf("source_message_id is required for %s", intent)
	}
	if strings.TrimSpace(req.SourceMessageID) != "" {
		if err := validateServiceResourceID("source_message_id", req.SourceMessageID); err != nil {
			return err
		}
	}
	if strings.ContainsAny(req.From, "\r\n") {
		return fmt.Errorf("from must not contain CR or LF")
	}
	if strings.ContainsAny(req.Subject, "\r\n") {
		return fmt.Errorf("subject must not contain CR or LF")
	}
	if len(req.Subject) > MaxComposeSubjectBytes {
		return fmt.Errorf("subject is too long")
	}
	if len(req.TextBody) > MaxComposeTextBodyBytes {
		return fmt.Errorf("text_body is too long")
	}
	if len(req.HTMLBody) > MaxComposeHTMLBodyBytes {
		return fmt.Errorf("html_body is too long")
	}
	if len(req.To)+len(req.Cc)+len(req.Bcc) > MaxComposeRecipients {
		return fmt.Errorf("too many recipients")
	}
	if err := validateComposeAddresses("to", req.To); err != nil {
		return err
	}
	if err := validateComposeAddresses("cc", req.Cc); err != nil {
		return err
	}
	if err := validateComposeAddresses("bcc", req.Bcc); err != nil {
		return err
	}
	if len(req.AttachmentIDs) > MaxComposeAttachments {
		return fmt.Errorf("too many attachments")
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
	return validateServiceResourceID("draft_id", draftID)
}
