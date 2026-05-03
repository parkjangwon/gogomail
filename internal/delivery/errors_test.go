package delivery

import (
	"errors"
	"net/textproto"
	"testing"
)

func TestWrapSMTPErrorClassifiesPermanentFailure(t *testing.T) {
	t.Parallel()

	err := WrapSMTPError("rcpt", &textproto.Error{Code: 550, Msg: "mailbox unavailable"})
	if !IsPermanentFailure(err) {
		t.Fatalf("IsPermanentFailure(%v) = false, want true", err)
	}
	if IsTemporaryFailure(err) {
		t.Fatalf("IsTemporaryFailure(%v) = true, want false", err)
	}
}

func TestWrapSMTPErrorClassifiesTemporaryFailure(t *testing.T) {
	t.Parallel()

	err := WrapSMTPError("mail", &textproto.Error{Code: 451, Msg: "try again"})
	if !IsTemporaryFailure(err) {
		t.Fatalf("IsTemporaryFailure(%v) = false, want true", err)
	}
	if IsPermanentFailure(err) {
		t.Fatalf("IsPermanentFailure(%v) = true, want false", err)
	}
}

func TestWrapSMTPErrorKeepsNetworkErrorsRetryable(t *testing.T) {
	t.Parallel()

	err := WrapSMTPError("dial", errors.New("network down"))
	if IsPermanentFailure(err) {
		t.Fatalf("IsPermanentFailure(%v) = true, want false", err)
	}
}

func TestEnhancedStatusForAttemptParsesSMTPReply(t *testing.T) {
	t.Parallel()

	err := &SMTPStatusError{Op: "rcpt", Code: 550, Message: "5.1.1 user unknown"}
	if got := enhancedStatusForAttempt(AttemptBounced, err); got != "5.1.1" {
		t.Fatalf("enhancedStatusForAttempt = %q, want 5.1.1", got)
	}

	err = &SMTPStatusError{Op: "rcpt", Code: 451, Message: "try later (4.7.1)"}
	if got := enhancedStatusForAttempt(AttemptFailed, err); got != "4.7.1" {
		t.Fatalf("enhancedStatusForAttempt = %q, want 4.7.1", got)
	}
}

func TestEnhancedStatusForAttemptIgnoresMismatchedClass(t *testing.T) {
	t.Parallel()

	err := &SMTPStatusError{Op: "rcpt", Code: 550, Message: "4.7.1 try later"}
	if got := enhancedStatusForAttempt(AttemptBounced, err); got != "5.0.0" {
		t.Fatalf("enhancedStatusForAttempt = %q, want bounced default", got)
	}
}

func TestEnhancedStatusForAttemptDefaultsByStatus(t *testing.T) {
	t.Parallel()

	if got := enhancedStatusForAttempt(AttemptDelivered, nil); got != "2.0.0" {
		t.Fatalf("delivered enhanced status = %q", got)
	}
	if got := enhancedStatusForAttempt(AttemptFailed, nil); got != "4.0.0" {
		t.Fatalf("failed enhanced status = %q", got)
	}
	if got := enhancedStatusForAttempt(AttemptBounced, nil); got != "5.0.0" {
		t.Fatalf("bounced enhanced status = %q", got)
	}
}
