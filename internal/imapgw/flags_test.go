package imapgw

import (
	"reflect"
	"testing"
)

func TestMapMessageFlagsUsesRFC3501SystemFlags(t *testing.T) {
	got := MapMessageFlags(MessageFlags{
		Read:      true,
		Starred:   true,
		Answered:  true,
		Deleted:   true,
		Forwarded: true,
		Keywords:  []string{"$Project", "$Project", `\Seen`, "bad keyword"},
	})
	want := []string{FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDeleted, "$Project"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("MapMessageFlags() = %#v, want %#v", got, want)
	}
}

func TestMapMessageFlagsHandlesDraftFromFlagOrStatus(t *testing.T) {
	for name, flags := range map[string]MessageFlags{
		"flag":   {Draft: true},
		"status": {Status: " draft "},
	} {
		t.Run(name, func(t *testing.T) {
			got := MapMessageFlags(flags)
			want := []string{FlagDraft}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("MapMessageFlags() = %#v, want %#v", got, want)
			}
		})
	}
}

func TestMapMessageFlagsDoesNotMapSoftDeletedStatusToDeleted(t *testing.T) {
	got := MapMessageFlags(MessageFlags{Status: "deleted"})
	if len(got) != 0 {
		t.Fatalf("MapMessageFlags(deleted status) = %#v, want no IMAP flags", got)
	}
	if FlagDeleted != `\Deleted` {
		t.Fatalf("FlagDeleted = %q, want \\\\Deleted", FlagDeleted)
	}
}

func TestApplyIMAPFlagMapsKnownMutableFlags(t *testing.T) {
	flags, ok := ApplyIMAPFlag(MessageFlags{}, ` \seen `, true)
	if !ok || !flags.Read {
		t.Fatalf("ApplyIMAPFlag(Seen) = %#v, %v; want read true", flags, ok)
	}

	flags, ok = ApplyIMAPFlag(flags, `\Flagged`, true)
	if !ok || !flags.Starred {
		t.Fatalf("ApplyIMAPFlag(Flagged) = %#v, %v; want starred true", flags, ok)
	}

	flags, ok = ApplyIMAPFlag(flags, `\Answered`, true)
	if !ok || !flags.Answered {
		t.Fatalf("ApplyIMAPFlag(Answered) = %#v, %v; want answered true", flags, ok)
	}

	flags, ok = ApplyIMAPFlag(flags, `forwarded`, true)
	if !ok || !flags.Forwarded {
		t.Fatalf("ApplyIMAPFlag(Forwarded) = %#v, %v; want forwarded true", flags, ok)
	}

	flags, ok = ApplyIMAPFlag(flags, `\Draft`, true)
	if !ok || !flags.Draft || flags.Status != "draft" {
		t.Fatalf("ApplyIMAPFlag(Draft) = %#v, %v; want draft status", flags, ok)
	}
}

func TestApplyIMAPFlagMapsDeletedSeparatelyFromSoftDelete(t *testing.T) {
	flags := MessageFlags{Read: true}
	got, ok := ApplyIMAPFlag(flags, `\Deleted`, true)
	if !ok || !got.Deleted || got.Status != "" {
		t.Fatalf("ApplyIMAPFlag(Deleted) = %#v, %v; want IMAP-only deleted flag", got, ok)
	}
	if !got.Read {
		t.Fatalf("ApplyIMAPFlag(Deleted) lost existing read flag: %#v", got)
	}
}

func TestMailFlagForIMAPFlagExposesOnlyPersistedMailboxFlags(t *testing.T) {
	tests := map[string]string{
		`\Seen`:      "read",
		`\Flagged`:   "starred",
		`\Answered`:  "answered",
		`Forwarded`:  "forwarded",
		`$Forwarded`: "forwarded",
	}
	for imapFlag, want := range tests {
		got, ok := MailFlagForIMAPFlag(imapFlag)
		if !ok || got != want {
			t.Fatalf("MailFlagForIMAPFlag(%q) = %q, %v; want %q, true", imapFlag, got, ok, want)
		}
	}
	if got, ok := MailFlagForIMAPFlag(`\Draft`); ok || got != "" {
		t.Fatalf("MailFlagForIMAPFlag(Draft) = %q, %v; want no direct persisted flag", got, ok)
	}
}

func TestIMAPKeywordFlagValid(t *testing.T) {
	for _, keyword := range []string{"$Project", "Custom", "$Forwarded"} {
		if !IMAPKeywordFlagValid(keyword) {
			t.Fatalf("IMAPKeywordFlagValid(%q) = false, want true", keyword)
		}
	}
	for _, keyword := range []string{`\Seen`, `\Deleted`, "bad keyword", `"quoted"`} {
		if IMAPKeywordFlagValid(keyword) {
			t.Fatalf("IMAPKeywordFlagValid(%q) = true, want false", keyword)
		}
	}
}
