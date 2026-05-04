package imapgw

import "testing"

func TestMailboxDisplayNamePrefersName(t *testing.T) {
	got := MailboxDisplayName(Mailbox{Name: " Inbox ", FullPath: "/Archive/2026"})
	if got != "Inbox" {
		t.Fatalf("MailboxDisplayName() = %q, want Inbox", got)
	}
}

func TestMailboxDisplayNameFallsBackToLastPathSegment(t *testing.T) {
	got := MailboxDisplayName(Mailbox{FullPath: "/Projects/gogomail"})
	if got != "gogomail" {
		t.Fatalf("MailboxDisplayName() = %q, want gogomail", got)
	}
}

func TestMailboxPathNormalizesDelimiters(t *testing.T) {
	got := MailboxPath(Mailbox{FullPath: "/Archive/2026/"})
	if got != "Archive/2026" {
		t.Fatalf("MailboxPath() = %q, want Archive/2026", got)
	}
}

func TestMailboxSelectionAndSystemHelpers(t *testing.T) {
	if IsSelectableMailbox(Mailbox{}) {
		t.Fatal("empty mailbox is selectable")
	}
	if !IsSelectableMailbox(Mailbox{Name: "Inbox"}) {
		t.Fatal("named mailbox is not selectable")
	}
	if !IsSystemMailbox(Mailbox{SystemType: " Drafts "}, "drafts") {
		t.Fatal("drafts system mailbox was not recognized")
	}
}
