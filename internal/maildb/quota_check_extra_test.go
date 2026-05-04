package maildb

import (
	"errors"
	"testing"

	"github.com/gogomail/gogomail/internal/mail"
)

func TestErrMailboxFullIsWrappedCorrectly(t *testing.T) {
	t.Parallel()

	wrapped := errors.New("prefix: " + mail.ErrMailboxFull.Error())
	if errors.Is(wrapped, mail.ErrMailboxFull) {
		t.Fatal("plain wrap should not satisfy errors.Is; use fmt.Errorf %w")
	}

	import_check := mail.ErrMailboxFull
	if import_check == nil {
		t.Fatal("mail.ErrMailboxFull must not be nil")
	}
}
