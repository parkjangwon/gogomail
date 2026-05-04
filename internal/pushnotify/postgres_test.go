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
