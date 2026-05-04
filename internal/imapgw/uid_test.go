package imapgw

import "testing"

func TestUIDStateRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	if err := ValidateUIDState(UIDState{MailboxID: "inbox", UIDValidity: 1, UIDNext: 1, HighestModSeq: 1}); err != nil {
		t.Fatalf("ValidateUIDState returned error: %v", err)
	}
	for name, state := range map[string]UIDState{
		"mailbox":     {UIDValidity: 1, UIDNext: 1, HighestModSeq: 1},
		"validity":    {MailboxID: "inbox", UIDNext: 1, HighestModSeq: 1},
		"next":        {MailboxID: "inbox", UIDValidity: 1, HighestModSeq: 1},
		"highest seq": {MailboxID: "inbox", UIDValidity: 1, UIDNext: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateUIDState(state); err == nil {
				t.Fatalf("ValidateUIDState accepted %+v", state)
			}
		})
	}
}

func TestMessageUIDRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	if err := ValidateMessageUID(MessageUID{MessageID: "msg-1", MailboxID: "inbox", UID: 1, ModSeq: 1}); err != nil {
		t.Fatalf("ValidateMessageUID returned error: %v", err)
	}
	for name, msg := range map[string]MessageUID{
		"message": {MailboxID: "inbox", UID: 1, ModSeq: 1},
		"mailbox": {MessageID: "msg-1", UID: 1, ModSeq: 1},
		"uid":     {MessageID: "msg-1", MailboxID: "inbox", ModSeq: 1},
		"modseq":  {MessageID: "msg-1", MailboxID: "inbox", UID: 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateMessageUID(msg); err == nil {
				t.Fatalf("ValidateMessageUID accepted %+v", msg)
			}
		})
	}
}
