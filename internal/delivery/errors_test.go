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
