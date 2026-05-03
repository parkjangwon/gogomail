package app

import "testing"

func TestParseModeTrimsAndLowercasesInput(t *testing.T) {
	mode, err := ParseMode(" OUTBOX-RELAY \n")
	if err != nil {
		t.Fatalf("ParseMode returned error: %v", err)
	}
	if mode != ModeOutboxRelay {
		t.Fatalf("ParseMode = %q, want %q", mode, ModeOutboxRelay)
	}
}
