package imapgw

import (
	"fmt"
	"strings"
)

type UIDState struct {
	MailboxID     MailboxID
	UIDValidity   uint32
	UIDNext       UID
	HighestModSeq uint64
}

type MessageUID struct {
	MessageID      MessageID
	MailboxID      MailboxID
	UID            UID
	SequenceNumber uint32
	ModSeq         uint64
}

func ValidateUIDState(state UIDState) error {
	if strings.TrimSpace(string(state.MailboxID)) == "" {
		return fmt.Errorf("mailbox_id is required")
	}
	if state.UIDValidity == 0 {
		return fmt.Errorf("uidvalidity is required")
	}
	if state.UIDNext == 0 {
		return fmt.Errorf("uidnext is required")
	}
	if state.HighestModSeq == 0 {
		return fmt.Errorf("highest_modseq is required")
	}
	return nil
}

func ValidateMessageUID(message MessageUID) error {
	if strings.TrimSpace(string(message.MessageID)) == "" {
		return fmt.Errorf("message_id is required")
	}
	if strings.TrimSpace(string(message.MailboxID)) == "" {
		return fmt.Errorf("mailbox_id is required")
	}
	if message.UID == 0 {
		return fmt.Errorf("uid is required")
	}
	if message.ModSeq == 0 {
		return fmt.Errorf("modseq is required")
	}
	return nil
}
