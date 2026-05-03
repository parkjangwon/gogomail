package mailservice

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

const MaxComposeSubjectBytes = 998
const MaxComposeTextBodyBytes = 4 << 20

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
	if len(req.Subject) > MaxComposeSubjectBytes {
		return fmt.Errorf("subject is too long")
	}
	if len(req.TextBody) > MaxComposeTextBodyBytes {
		return fmt.Errorf("text_body is too long")
	}
	if len(req.To)+len(req.Cc)+len(req.Bcc) == 0 {
		return fmt.Errorf("at least one recipient is required")
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
	return nil
}

func validateComposeAddresses(field string, addresses []outbound.Address) error {
	for i, address := range addresses {
		if strings.TrimSpace(address.Email) == "" {
			return fmt.Errorf("%s[%d].email is required", field, i)
		}
	}
	return nil
}
