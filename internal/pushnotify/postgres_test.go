package pushnotify

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeCandidateRecordBoundsDiagnostics(t *testing.T) {
	t.Parallel()

	record := normalizeCandidateRecord(CandidateRecord{
		MessageID:    " msg-1 ",
		UserID:       " user-1 ",
		DeviceID:     " device-1 ",
		Platform:     " FCM ",
		Subject:      strings.Repeat("s", 600),
		ErrorMessage: strings.Repeat("e", 2100),
	})

	if record.MessageID != "msg-1" || record.UserID != "user-1" || record.DeviceID != "device-1" {
		t.Fatalf("identity fields not normalized: %+v", record)
	}
	if record.Platform != "fcm" {
		t.Fatalf("Platform = %q, want fcm", record.Platform)
	}
	if len(record.Subject) != 500 {
		t.Fatalf("Subject length = %d, want 500", len(record.Subject))
	}
	if len(record.ErrorMessage) != 2000 {
		t.Fatalf("ErrorMessage length = %d, want 2000", len(record.ErrorMessage))
	}
}

func TestNormalizeCandidateRecordBoundsDiagnosticsAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	record := normalizeCandidateRecord(CandidateRecord{
		MessageID:    "msg-1",
		UserID:       "user-1",
		DeviceID:     "device-1",
		Subject:      strings.Repeat("가", 251),
		ErrorMessage: strings.Repeat("🚀", 501),
	})

	if len(record.Subject) > 500 || !utf8.ValidString(record.Subject) {
		t.Fatalf("Subject length/utf8 = %d/%v", len(record.Subject), utf8.ValidString(record.Subject))
	}
	if len(record.ErrorMessage) > 2000 || !utf8.ValidString(record.ErrorMessage) {
		t.Fatalf("ErrorMessage length/utf8 = %d/%v", len(record.ErrorMessage), utf8.ValidString(record.ErrorMessage))
	}
}

func TestNormalizeAttemptOutcome(t *testing.T) {
	t.Parallel()

	outcome, err := normalizeAttemptOutcome(AttemptOutcome{
		AttemptID:         " attempt-1 ",
		Status:            " INVALID_TOKEN ",
		ErrorMessage:      strings.Repeat("e", 2100),
		ProviderMessageID: strings.Repeat("m", 600),
		ProviderStatus:    strings.Repeat("s", 600),
	})
	if err != nil {
		t.Fatalf("normalizeAttemptOutcome returned error: %v", err)
	}
	if outcome.AttemptID != "attempt-1" || outcome.Status != "invalid_token" {
		t.Fatalf("outcome = %+v", outcome)
	}
	if len(outcome.ErrorMessage) != 2000 {
		t.Fatalf("ErrorMessage length = %d, want 2000", len(outcome.ErrorMessage))
	}
	if len(outcome.ProviderMessageID) != 500 {
		t.Fatalf("ProviderMessageID length = %d, want 500", len(outcome.ProviderMessageID))
	}
	if len(outcome.ProviderStatus) != 500 {
		t.Fatalf("ProviderStatus length = %d, want 500", len(outcome.ProviderStatus))
	}
}

func TestNormalizeAttemptOutcomeBoundsDiagnosticsAtUTF8Boundary(t *testing.T) {
	t.Parallel()

	outcome, err := normalizeAttemptOutcome(AttemptOutcome{
		AttemptID:         "attempt-1",
		Status:            "failed",
		ErrorMessage:      strings.Repeat("한", 668),
		ProviderMessageID: strings.Repeat("메", 168),
		ProviderStatus:    strings.Repeat("상", 168),
	})
	if err != nil {
		t.Fatalf("normalizeAttemptOutcome returned error: %v", err)
	}
	if len(outcome.ErrorMessage) > 2000 || !utf8.ValidString(outcome.ErrorMessage) {
		t.Fatalf("ErrorMessage length/utf8 = %d/%v", len(outcome.ErrorMessage), utf8.ValidString(outcome.ErrorMessage))
	}
	if len(outcome.ProviderMessageID) > 500 || !utf8.ValidString(outcome.ProviderMessageID) {
		t.Fatalf("ProviderMessageID length/utf8 = %d/%v", len(outcome.ProviderMessageID), utf8.ValidString(outcome.ProviderMessageID))
	}
	if len(outcome.ProviderStatus) > 500 || !utf8.ValidString(outcome.ProviderStatus) {
		t.Fatalf("ProviderStatus length/utf8 = %d/%v", len(outcome.ProviderStatus), utf8.ValidString(outcome.ProviderStatus))
	}
}

func TestNormalizeAttemptOutcomeRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	_, err := normalizeAttemptOutcome(AttemptOutcome{AttemptID: "attempt-1", Status: "candidate"})
	if err == nil {
		t.Fatal("normalizeAttemptOutcome accepted candidate status")
	}
}
