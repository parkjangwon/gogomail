package attachmentscan

import (
	"context"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/message"
	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

func TestHookIgnoresNonParsedStageAndMessagesWithoutAttachments(t *testing.T) {
	t.Parallel()

	scanner := &fakeScanner{}
	hook := Hook(HookOptions{Scanner: scanner})
	if err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageStored}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if err := hook(context.Background(), smtpd.Event{Stage: smtpd.StageParsed}); err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if scanner.calls != 0 {
		t.Fatalf("scanner calls = %d, want 0", scanner.calls)
	}
}

func TestHookScansParsedAttachmentMetadata(t *testing.T) {
	t.Parallel()

	scanner := &fakeScanner{result: Result{Verdict: VerdictAccept}}
	hook := Hook(HookOptions{Scanner: scanner})
	err := hook(context.Background(), smtpd.Event{
		Stage:        smtpd.StageParsed,
		RemoteAddr:   "192.0.2.10:25",
		EnvelopeFrom: "sender@example.com",
		Recipients:   []string{"rcpt@example.com"},
		Mailbox:      smtpd.Mailbox{CompanyID: "company-1", DomainID: "domain-1", UserID: "user-1"},
		Parsed: message.ParsedMessage{
			MessageID:     "<msg@example.com>",
			Subject:       "hello",
			HasAttachment: true,
			Attachments:   []message.Attachment{{Filename: "report.pdf"}},
		},
		Size: 123,
	})
	if err != nil {
		t.Fatalf("hook returned error: %v", err)
	}
	if scanner.calls != 1 {
		t.Fatalf("scanner calls = %d, want 1", scanner.calls)
	}
	if scanner.last.DomainID != "domain-1" || scanner.last.Attachments[0].Filename != "report.pdf" {
		t.Fatalf("request = %+v", scanner.last)
	}
}

func TestHookRejectsScannerVerdict(t *testing.T) {
	t.Parallel()

	scanner := &fakeScanner{result: Result{Verdict: VerdictReject, Reason: "blocked\nbad"}}
	hook := Hook(HookOptions{Scanner: scanner})
	err := hook(context.Background(), smtpd.Event{
		Stage: smtpd.StageParsed,
		Parsed: message.ParsedMessage{
			HasAttachment: true,
			Attachments:   []message.Attachment{{Filename: "blocked.exe"}},
		},
	})
	if err == nil {
		t.Fatal("hook accepted rejected attachment")
	}
	if strings.ContainsAny(err.Error(), "\r\n") {
		t.Fatalf("error contains newline: %q", err.Error())
	}
}

func TestHookBoundsScannerReason(t *testing.T) {
	t.Parallel()

	scanner := &fakeScanner{result: Result{Verdict: VerdictTempfail, Reason: strings.Repeat("\u20ac", maxScannerReasonBytes)}}
	hook := Hook(HookOptions{Scanner: scanner})
	err := hook(context.Background(), smtpd.Event{
		Stage: smtpd.StageParsed,
		Parsed: message.ParsedMessage{
			HasAttachment: true,
			Attachments:   []message.Attachment{{Filename: "large.bin"}},
		},
	})
	if err == nil {
		t.Fatal("hook accepted temporary attachment scanner failure")
	}
	reason := strings.TrimPrefix(err.Error(), "attachment scanner temporarily failed message: ")
	if len(reason) > maxScannerReasonBytes {
		t.Fatalf("reason length = %d, want <= %d", len(reason), maxScannerReasonBytes)
	}
	if strings.ContainsRune(reason, '\uFFFD') {
		t.Fatalf("reason is not UTF-8 safely truncated: %q", reason)
	}
}

type fakeScanner struct {
	calls  int
	last   Request
	result Result
}

func (s *fakeScanner) ScanAttachments(_ context.Context, req Request) (Result, error) {
	s.calls++
	s.last = req
	return s.result, nil
}
