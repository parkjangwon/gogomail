package pushnotify

import (
	"strings"
	"testing"
	"unicode/utf8"
)

func TestNormalizeCandidateRecordBoundsDiagnostics(t *testing.T) {
	t.Parallel()

	record, err := normalizeCandidateRecord(CandidateRecord{
		MessageID:    " msg-1 ",
		UserID:       " user-1 ",
		DeviceID:     " device-1 ",
		Platform:     " FCM ",
		Subject:      strings.Repeat("s", 600),
		ErrorMessage: strings.Repeat("e", 2100),
	})
	if err != nil {
		t.Fatalf("normalizeCandidateRecord returned error: %v", err)
	}

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

	record, err := normalizeCandidateRecord(CandidateRecord{
		MessageID:    "msg-1",
		UserID:       "user-1",
		DeviceID:     "device-1",
		Subject:      strings.Repeat("가", 251),
		ErrorMessage: strings.Repeat("🚀", 501),
	})
	if err != nil {
		t.Fatalf("normalizeCandidateRecord returned error: %v", err)
	}

	if len(record.Subject) > 500 || !utf8.ValidString(record.Subject) {
		t.Fatalf("Subject length/utf8 = %d/%v", len(record.Subject), utf8.ValidString(record.Subject))
	}
	if len(record.ErrorMessage) > 2000 || !utf8.ValidString(record.ErrorMessage) {
		t.Fatalf("ErrorMessage length/utf8 = %d/%v", len(record.ErrorMessage), utf8.ValidString(record.ErrorMessage))
	}
}

func TestNormalizeCandidateRecordRejectsUnsafeIDs(t *testing.T) {
	t.Parallel()

	tests := []CandidateRecord{
		{MessageID: "", UserID: "user-1", DeviceID: "device-1"},
		{MessageID: "message-1\nbad", UserID: "user-1", DeviceID: "device-1"},
		{MessageID: strings.Repeat("m", maxPushAttemptIDBytes+1), UserID: "user-1", DeviceID: "device-1"},
		{MessageID: "message-1", UserID: string([]byte{0xff}), DeviceID: "device-1"},
		{MessageID: "message-1", UserID: "user-1", DeviceID: "device-1", CompanyID: "company-1\nbad"},
		{MessageID: "message-1", UserID: "user-1", DeviceID: "device-1", Platform: "pager"},
	}
	for _, record := range tests {
		record := record
		t.Run(record.MessageID+record.UserID+record.DeviceID+record.Platform, func(t *testing.T) {
			t.Parallel()
			if _, err := normalizeCandidateRecord(record); err == nil {
				t.Fatalf("normalizeCandidateRecord accepted %+v", record)
			}
		})
	}
}
