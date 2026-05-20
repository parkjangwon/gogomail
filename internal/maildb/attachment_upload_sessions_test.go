package maildb

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateAttachmentUploadSessionRequest(t *testing.T) {
	t.Parallel()

	valid := CreateAttachmentUploadSessionRequest{
		UserID:       "user-1",
		Filename:     "large.bin",
		DeclaredSize: 42,
		MIMEType:     "application/octet-stream",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	if err := ValidateCreateAttachmentUploadSessionRequest(valid); err != nil {
		t.Fatalf("ValidateCreateAttachmentUploadSessionRequest returned error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*CreateAttachmentUploadSessionRequest)
	}{
		{name: "missing user", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.UserID = " " }},
		{name: "missing filename", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.Filename = " " }},
		{name: "newline filename", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.Filename = "bad\nname" }},
		{name: "oversized filename", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.Filename = strings.Repeat("a", 256) }},
		{name: "negative size", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.DeclaredSize = -1 }},
		{name: "missing mime", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.MIMEType = " " }},
		{name: "newline mime", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.MIMEType = "text/plain\nbad" }},
		{name: "oversized mime", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.MIMEType = strings.Repeat("a", 256) }},
		{name: "missing expiry", mutate: func(req *CreateAttachmentUploadSessionRequest) { req.ExpiresAt = time.Time{} }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := valid
			tt.mutate(&req)
			if err := ValidateCreateAttachmentUploadSessionRequest(req); err == nil {
				t.Fatal("ValidateCreateAttachmentUploadSessionRequest accepted invalid request")
			}
		})
	}
}

func TestValidateCancelAttachmentUploadSessionRequest(t *testing.T) {
	t.Parallel()

	valid := CancelAttachmentUploadSessionRequest{
		UserID:    "user-1",
		SessionID: "session-1",
	}
	if err := ValidateCancelAttachmentUploadSessionRequest(valid); err != nil {
		t.Fatalf("ValidateCancelAttachmentUploadSessionRequest returned error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*CancelAttachmentUploadSessionRequest)
	}{
		{name: "missing user", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.UserID = " " }},
		{name: "missing session", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.SessionID = " " }},
		{name: "newline user", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.UserID = "user\n1" }},
		{name: "newline session", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.SessionID = "session\n1" }},
		{name: "oversized user", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.UserID = strings.Repeat("u", 201) }},
		{name: "oversized session", mutate: func(req *CancelAttachmentUploadSessionRequest) { req.SessionID = strings.Repeat("s", 201) }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := valid
			tt.mutate(&req)
			if err := ValidateCancelAttachmentUploadSessionRequest(req); err == nil {
				t.Fatal("ValidateCancelAttachmentUploadSessionRequest accepted invalid request")
			}
		})
	}
}

func TestValidateAttachmentUploadSessionListRequestRejectsUnsafeFilters(t *testing.T) {
	t.Parallel()

	tests := []AttachmentUploadSessionListRequest{
		{UserID: "user\nbad"},
		{DraftID: strings.Repeat("d", 201)},
		{Status: "ready"},
		{Status: "uploading\nbad"},
	}
	for _, req := range tests {
		req := req
		t.Run(req.UserID+req.DraftID+req.Status, func(t *testing.T) {
			t.Parallel()
			if err := ValidateAttachmentUploadSessionListRequest(req); err == nil {
				t.Fatalf("ValidateAttachmentUploadSessionListRequest accepted %+v", req)
			}
		})
	}
}

func TestValidateGetAttachmentUploadSessionRequest(t *testing.T) {
	t.Parallel()

	valid := GetAttachmentUploadSessionRequest{
		UserID:    "user-1",
		SessionID: "session-1",
	}
	if err := ValidateGetAttachmentUploadSessionRequest(valid); err != nil {
		t.Fatalf("ValidateGetAttachmentUploadSessionRequest returned error: %v", err)
	}

	for _, req := range []GetAttachmentUploadSessionRequest{
		{SessionID: "session-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", SessionID: "session-1"},
		{UserID: "user-1", SessionID: "session\n1"},
	} {
		req := req
		t.Run(req.UserID+"/"+req.SessionID, func(t *testing.T) {
			t.Parallel()

			if err := ValidateGetAttachmentUploadSessionRequest(req); err == nil {
				t.Fatal("ValidateGetAttachmentUploadSessionRequest accepted invalid request")
			}
		})
	}
}

func TestValidateStoreAttachmentUploadSessionBodyRequest(t *testing.T) {
	t.Parallel()

	valid := StoreAttachmentUploadSessionBodyRequest{
		UserID:         "user-1",
		SessionID:      "session-1",
		ReceivedSize:   42,
		StoragePath:    "upload-sessions/user-1/session-1/body",
		ChecksumSHA256: strings.Repeat("a", 64),
	}
	if err := ValidateStoreAttachmentUploadSessionBodyRequest(valid); err != nil {
		t.Fatalf("ValidateStoreAttachmentUploadSessionBodyRequest returned error: %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*StoreAttachmentUploadSessionBodyRequest)
	}{
		{name: "missing user", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.UserID = " " }},
		{name: "missing session", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.SessionID = " " }},
		{name: "negative size", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.ReceivedSize = -1 }},
		{name: "missing path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = " " }},
		{name: "newline path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = "bad\npath" }},
		{name: "backslash path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = `upload-sessions\user-1\body` }},
		{name: "absolute path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = "/upload-sessions/user-1/body" }},
		{name: "parent segment path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = "upload-sessions/user-1/../body" }},
		{name: "double slash path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = "upload-sessions//user-1/body" }},
		{name: "wrong prefix path", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.StoragePath = "uploads/user-1/body" }},
		{name: "missing checksum", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.ChecksumSHA256 = " " }},
		{name: "short checksum", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.ChecksumSHA256 = "abc" }},
		{name: "uppercase checksum", mutate: func(req *StoreAttachmentUploadSessionBodyRequest) { req.ChecksumSHA256 = strings.Repeat("A", 64) }},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			req := valid
			tt.mutate(&req)
			if err := ValidateStoreAttachmentUploadSessionBodyRequest(req); err == nil {
				t.Fatal("ValidateStoreAttachmentUploadSessionBodyRequest accepted invalid request")
			}
		})
	}
}

func TestValidateFinalizeAttachmentUploadSessionRequest(t *testing.T) {
	t.Parallel()

	valid := FinalizeAttachmentUploadSessionRequest{UserID: "user-1", SessionID: "session-1"}
	if err := ValidateFinalizeAttachmentUploadSessionRequest(valid); err != nil {
		t.Fatalf("ValidateFinalizeAttachmentUploadSessionRequest returned error: %v", err)
	}
	if err := ValidateFinalizeAttachmentUploadSessionRequest(FinalizeAttachmentUploadSessionRequest{UserID: "user-1"}); err == nil {
		t.Fatal("ValidateFinalizeAttachmentUploadSessionRequest accepted missing session")
	}
	if err := ValidateFinalizeAttachmentUploadSessionRequest(FinalizeAttachmentUploadSessionRequest{UserID: "user\n1", SessionID: "session-1"}); err == nil {
		t.Fatal("ValidateFinalizeAttachmentUploadSessionRequest accepted newline user")
	}
}

func TestFinalizeAttachmentUploadSessionSQLProjectsTargetColumns(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"SELECT\n    user_id,\n    draft_id,\n    upload_id,\n    storage_path,\n    filename,\n    declared_size,\n    mime_type",
		"FOR UPDATE",
		"RETURNING COALESCE(target.draft_id::text, '') AS draft_id",
	} {
		if !strings.Contains(finalizeAttachmentUploadSessionSQL, want) {
			t.Fatalf("finalizeAttachmentUploadSessionSQL missing %q:\n%s", want, finalizeAttachmentUploadSessionSQL)
		}
	}
	if strings.Contains(finalizeAttachmentUploadSessionSQL, "SELECT *") {
		t.Fatalf("finalizeAttachmentUploadSessionSQL still projects every session column:\n%s", finalizeAttachmentUploadSessionSQL)
	}
}

func TestValidateExpireAttachmentUploadSessionsRequest(t *testing.T) {
	t.Parallel()

	valid := ExpireAttachmentUploadSessionsRequest{
		Before: time.Now(),
		Limit:  10,
	}
	if err := ValidateExpireAttachmentUploadSessionsRequest(valid); err != nil {
		t.Fatalf("ValidateExpireAttachmentUploadSessionsRequest returned error: %v", err)
	}
	if err := ValidateExpireAttachmentUploadSessionsRequest(ExpireAttachmentUploadSessionsRequest{Limit: 10}); err == nil {
		t.Fatal("ValidateExpireAttachmentUploadSessionsRequest accepted zero before")
	}
	if err := ValidateExpireAttachmentUploadSessionsRequest(ExpireAttachmentUploadSessionsRequest{Before: time.Now(), Limit: -1}); err == nil {
		t.Fatal("ValidateExpireAttachmentUploadSessionsRequest accepted negative limit")
	}
}

func TestExpireAttachmentUploadSessionsSQLUsesBatchUpdates(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($1::uuid[])",
		"UPDATE attachment_upload_sessions s",
		"FROM input",
	} {
		if !strings.Contains(expireAttachmentUploadSessionsSQL, want) {
			t.Fatalf("expireAttachmentUploadSessionsSQL missing %q:\n%s", want, expireAttachmentUploadSessionsSQL)
		}
	}
	for _, want := range []string{
		"unnest($1::uuid[], $2::bigint[])",
		"user_usage AS",
		"domain_usage AS",
		"company_usage AS",
		"UPDATE users u",
		"UPDATE domains d",
		"UPDATE companies c",
	} {
		if !strings.Contains(decrementUserQuotasSQL, want) {
			t.Fatalf("decrementUserQuotasSQL missing %q:\n%s", want, decrementUserQuotasSQL)
		}
	}
}
