package mailservice

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/outbound"
)

const MaxComposeSubjectBytes = 998
const MaxComposeTextBodyBytes = 4 << 20
const MaxComposeHTMLBodyBytes = 4 << 20
const MaxComposeRecipients = 200
const MaxComposeAttachments = 100

type ComposeIntent string

const (
	ComposeIntentNew     ComposeIntent = "new"
	ComposeIntentReply   ComposeIntent = "reply"
	ComposeIntentForward ComposeIntent = "forward"
)

func NormalizeComposeIntent(value string) (ComposeIntent, error) {
	switch ComposeIntent(strings.ToLower(strings.TrimSpace(value))) {
	case "", ComposeIntentNew:
		return ComposeIntentNew, nil
	case ComposeIntentReply:
		return ComposeIntentReply, nil
	case ComposeIntentForward:
		return ComposeIntentForward, nil
	default:
		return "", fmt.Errorf("unsupported compose intent %q", value)
	}
}

func ValidateSendTextRequest(req SendTextRequest) error {
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
	recipientCount := len(req.To) + len(req.Cc) + len(req.Bcc)
	if recipientCount == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	if recipientCount > MaxComposeRecipients {
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

func validateComposeAddresses(field string, addresses []outbound.Address) error {
	for i, address := range addresses {
		if strings.ContainsAny(address.Name, "\r\n") {
			return fmt.Errorf("%s[%d].name must not contain CR or LF", field, i)
		}
		if strings.ContainsAny(address.Email, "\r\n") {
			return fmt.Errorf("%s[%d].email must not contain CR or LF", field, i)
		}
		if strings.TrimSpace(address.Email) == "" {
			return fmt.Errorf("%s[%d].email is required", field, i)
		}
		if recipientGroupTokenKind(address.Email) != "" {
			continue
		}
		if _, err := mail.NormalizeAddress(address.Email); err != nil {
			return fmt.Errorf("%s[%d].email is invalid: %w", field, i, err)
		}
	}
	return nil
}
