package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/gogomail/gogomail/internal/httpapi"
)

func TestDrivePublicShareAuditRecorderWritesSanitizedAuditLog(t *testing.T) {
	t.Parallel()

	writer := &fakeAuditWriter{}
	recorder := drivePublicShareAuditRecorder{audit: writer}
	rawToken := strings.Repeat("t", 40)
	err := recorder.RecordDrivePublicShareAccess(context.Background(), httpapi.DrivePublicShareAccessEvent{
		Action:      "download",
		Result:      "success",
		LinkID:      "11111111-1111-1111-1111-111111111111",
		CompanyID:   "22222222-2222-2222-2222-222222222222",
		DomainID:    "33333333-3333-3333-3333-333333333333",
		UserID:      "44444444-4444-4444-4444-444444444444",
		NodeID:      "55555555-5555-5555-5555-555555555555",
		Permission:  "download",
		TokenSuffix: rawToken[len(rawToken)-8:],
		RemoteAddr:  "192.0.2.10",
		UserAgent:   "DriveClient/1.0",
		Range:       "bytes=2-4",
		Status:      206,
	})
	if err != nil {
		t.Fatalf("RecordDrivePublicShareAccess returned error: %v", err)
	}
	if writer.insertCalls != 1 {
		t.Fatalf("insert calls = %d, want 1", writer.insertCalls)
	}
	if writer.log.Category != "drive" || writer.log.Action != "share_link.download" || writer.log.TargetType != "drive_share_link" || writer.log.TargetID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("audit log = %+v, want drive share-link download target", writer.log)
	}
	if writer.log.IPAddress != "192.0.2.10" || writer.log.UserAgent != "DriveClient/1.0" || writer.log.Result != "success" {
		t.Fatalf("audit request metadata = %+v", writer.log)
	}
	var detail map[string]any
	if err := json.Unmarshal(writer.log.Detail, &detail); err != nil {
		t.Fatalf("unmarshal audit detail: %v", err)
	}
	if detail["node_id"] != "55555555-5555-5555-5555-555555555555" || detail["token_suffix"] != rawToken[len(rawToken)-8:] || detail["range"] != "bytes=2-4" {
		t.Fatalf("audit detail = %+v, want node/suffix/range", detail)
	}
	if strings.Contains(string(writer.log.Detail), rawToken) {
		t.Fatalf("audit detail leaked raw token: %s", writer.log.Detail)
	}
}
