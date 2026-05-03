package mailservice

import (
	"fmt"
	"strings"
)

const MaxComposeSubjectBytes = 998

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
	if len(req.To)+len(req.Cc)+len(req.Bcc) == 0 {
		return fmt.Errorf("at least one recipient is required")
	}
	return nil
}
