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
		Forwarded: true,
	})
	want := []string{FlagSeen, FlagFlagged, FlagAnswered}
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
	if PlannedFlagDeleted != `\Deleted` {
		t.Fatalf("PlannedFlagDeleted = %q, want \\\\Deleted", PlannedFlagDeleted)
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

	flags, ok = ApplyIMAPFlag(flags, `\Draft`, true)
	if !ok || !flags.Draft || flags.Status != "draft" {
		t.Fatalf("ApplyIMAPFlag(Draft) = %#v, %v; want draft status", flags, ok)
	}
}

func TestApplyIMAPFlagRejectsDeletedForCurrentModel(t *testing.T) {
	flags := MessageFlags{Read: true}
	got, ok := ApplyIMAPFlag(flags, `\Deleted`, true)
	if ok {
		t.Fatal("ApplyIMAPFlag(Deleted) succeeded; current soft-delete is not IMAP Deleted")
	}
	if !reflect.DeepEqual(got, flags) {
		t.Fatalf("ApplyIMAPFlag(Deleted) mutated flags to %#v", got)
	}
}

func TestMailFlagForIMAPFlagExposesOnlyPersistedMailboxFlags(t *testing.T) {
	tests := map[string]string{
		`\Seen`:     "read",
		`\Flagged`:  "starred",
		`\Answered`: "answered",
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
