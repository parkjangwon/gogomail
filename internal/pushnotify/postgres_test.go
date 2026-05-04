package pushnotify

import (
	"strings"
	"testing"
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

func TestNormalizeAttemptOutcome(t *testing.T) {
	t.Parallel()

	outcome, err := normalizeAttemptOutcome(AttemptOutcome{
		AttemptID:    " attempt-1 ",
		Status:       " INVALID_TOKEN ",
		ErrorMessage: strings.Repeat("e", 2100),
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
}

func TestNormalizeAttemptOutcomeRejectsInvalidStatus(t *testing.T) {
	t.Parallel()

	_, err := normalizeAttemptOutcome(AttemptOutcome{AttemptID: "attempt-1", Status: "candidate"})
	if err == nil {
		t.Fatal("normalizeAttemptOutcome accepted candidate status")
	}
}
