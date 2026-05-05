package drive

import (
	"strings"
	"testing"
	"time"
)

func TestValidateCreateUploadSessionRequest(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req, err := ValidateCreateUploadSessionRequest(CreateUploadSessionRequest{
		UserID:         " user-1 ",
		ParentID:       " parent-1 ",
		UploadID:       " upload-1 ",
		Name:           " Report.PDF ",
		DeclaredSize:   123,
		MIMEType:       "",
		StorageBackend: " s3 ",
	}, now)
	if err != nil {
		t.Fatalf("ValidateCreateUploadSessionRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.ParentID != "parent-1" || req.UploadID != "upload-1" || req.Name != "Report.PDF" {
		t.Fatalf("request = %+v, want trimmed identity/name fields", req)
	}
	if req.MIMEType != "application/octet-stream" || req.StorageBackend != "s3" {
		t.Fatalf("request = %+v, want default MIME and trimmed backend", req)
	}
	if !req.ExpiresAt.Equal(now.Add(DefaultUploadSessionTTL)) {
		t.Fatalf("ExpiresAt = %s, want default TTL", req.ExpiresAt)
	}
}

func TestValidateCreateUploadSessionRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	tests := []CreateUploadSessionRequest{
		{UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3"},
		{UserID: "user-1", Name: "Report.pdf", StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", StorageBackend: "s3"},
		{UserID: "user\n1", UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3"},
		{UserID: "user-1", ParentID: "parent\n1", UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload/1", Name: "Report.pdf", StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Reports/2026", StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", DeclaredSize: -1, StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", DeclaredSize: MaxUploadSessionBytes + 1, StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", MIMEType: "text/plain\nbad", StorageBackend: "s3"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3\nbad"},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3", ExpiresAt: now},
		{UserID: "user-1", UploadID: "upload-1", Name: "Report.pdf", StorageBackend: "s3", ExpiresAt: now.Add(MaxUploadSessionTTL + time.Second)},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.UploadID+"-"+tc.Name, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateCreateUploadSessionRequest(tc, now); err == nil {
				t.Fatalf("ValidateCreateUploadSessionRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateGetUploadSessionRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateGetUploadSessionRequest(GetUploadSessionRequest{UserID: " user-1 ", SessionID: " session-1 "})
	if err != nil {
		t.Fatalf("ValidateGetUploadSessionRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.SessionID != "session-1" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
}

func TestValidateGetUploadSessionRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []GetUploadSessionRequest{
		{SessionID: "session-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", SessionID: "session-1"},
		{UserID: "user-1", SessionID: "session\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.SessionID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateGetUploadSessionRequest(tc); err == nil {
				t.Fatalf("ValidateGetUploadSessionRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateCancelUploadSessionRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateCancelUploadSessionRequest(CancelUploadSessionRequest{UserID: " user-1 ", SessionID: " session-1 "})
	if err != nil {
		t.Fatalf("ValidateCancelUploadSessionRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.SessionID != "session-1" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
}

func TestValidateCancelUploadSessionRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []CancelUploadSessionRequest{
		{SessionID: "session-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", SessionID: "session-1"},
		{UserID: "user-1", SessionID: "session\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.SessionID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateCancelUploadSessionRequest(tc); err == nil {
				t.Fatalf("ValidateCancelUploadSessionRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateStoreUploadSessionBodyRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateStoreUploadSessionBodyRequest(StoreUploadSessionBodyRequest{
		UserID:                 " user-1 ",
		SessionID:              " session-1 ",
		ExpectedChecksumSHA256: strings.Repeat("A", 64),
		Body:                   strings.NewReader("body"),
	})
	if err != nil {
		t.Fatalf("ValidateStoreUploadSessionBodyRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.SessionID != "session-1" || req.ExpectedChecksumSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("request = %+v, want trimmed identity and normalized checksum", req)
	}
}

func TestValidateStoreUploadSessionBodyRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []StoreUploadSessionBodyRequest{
		{SessionID: "session-1", Body: strings.NewReader("body")},
		{UserID: "user-1", Body: strings.NewReader("body")},
		{UserID: "user\n1", SessionID: "session-1", Body: strings.NewReader("body")},
		{UserID: "user-1", SessionID: "session\n1", Body: strings.NewReader("body")},
		{UserID: "user-1", SessionID: "session-1", ExpectedChecksumSHA256: "not-sha", Body: strings.NewReader("body")},
		{UserID: "user-1", SessionID: "session-1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.SessionID+"-"+tc.ExpectedChecksumSHA256, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateStoreUploadSessionBodyRequest(tc); err == nil {
				t.Fatalf("ValidateStoreUploadSessionBodyRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateFinalizeUploadSessionRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateFinalizeUploadSessionRequest(FinalizeUploadSessionRequest{UserID: " user-1 ", SessionID: " session-1 "})
	if err != nil {
		t.Fatalf("ValidateFinalizeUploadSessionRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.SessionID != "session-1" {
		t.Fatalf("request = %+v, want trimmed fields", req)
	}
}

func TestValidateFinalizeUploadSessionRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []FinalizeUploadSessionRequest{
		{SessionID: "session-1"},
		{UserID: "user-1"},
		{UserID: "user\n1", SessionID: "session-1"},
		{UserID: "user-1", SessionID: "session\n1"},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.SessionID, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateFinalizeUploadSessionRequest(tc); err == nil {
				t.Fatalf("ValidateFinalizeUploadSessionRequest(%+v) error = nil, want rejection", tc)
			}
		})
	}
}

func TestValidateExpireUploadSessionsRequest(t *testing.T) {
	t.Parallel()

	before := time.Date(2026, 5, 6, 12, 0, 0, 0, time.UTC)
	req, err := ValidateExpireUploadSessionsRequest(ExpireUploadSessionsRequest{Before: before, Limit: 0})
	if err != nil {
		t.Fatalf("ValidateExpireUploadSessionsRequest returned error: %v", err)
	}
	if !req.Before.Equal(before) || req.Limit != UploadSessionCleanupDefaultLimit {
		t.Fatalf("request = %+v, want UTC before and default limit", req)
	}
	capped, err := ValidateExpireUploadSessionsRequest(ExpireUploadSessionsRequest{Before: before, Limit: UploadSessionCleanupMaxLimit + 1})
	if err != nil {
		t.Fatalf("ValidateExpireUploadSessionsRequest capped returned error: %v", err)
	}
	if capped.Limit != UploadSessionCleanupMaxLimit {
		t.Fatalf("limit = %d, want max cleanup limit", capped.Limit)
	}
}

func TestValidateExpireUploadSessionsRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	if _, err := ValidateExpireUploadSessionsRequest(ExpireUploadSessionsRequest{Limit: 10}); err == nil {
		t.Fatal("ValidateExpireUploadSessionsRequest accepted zero before")
	}
	if _, err := ValidateExpireUploadSessionsRequest(ExpireUploadSessionsRequest{Before: time.Now(), Limit: -1}); err == nil {
		t.Fatal("ValidateExpireUploadSessionsRequest accepted negative limit")
	}
}

func TestValidateRecordUploadSessionBodyRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateRecordUploadSessionBodyRequest(RecordUploadSessionBodyRequest{
		UserID:         " user-1 ",
		SessionID:      " session-1 ",
		ReceivedSize:   4,
		StoragePath:    " drive/users/user-1/upload-sessions/session-1/bodies/body-1 ",
		ChecksumSHA256: strings.Repeat("A", 64),
	})
	if err != nil {
		t.Fatalf("ValidateRecordUploadSessionBodyRequest returned error: %v", err)
	}
	if req.StoragePath != "drive/users/user-1/upload-sessions/session-1/bodies/body-1" || req.ChecksumSHA256 != strings.Repeat("a", 64) {
		t.Fatalf("request = %+v, want trimmed path and normalized checksum", req)
	}
}

func TestValidateUploadSessionStatus(t *testing.T) {
	t.Parallel()

	status, err := ValidateUploadSessionStatus(" Uploading ")
	if err != nil {
		t.Fatalf("ValidateUploadSessionStatus returned error: %v", err)
	}
	if status != UploadSessionStatusUploading {
		t.Fatalf("status = %q, want uploading", status)
	}
	if _, err := ValidateUploadSessionStatus(strings.Repeat("x", 1)); err == nil {
		t.Fatal("ValidateUploadSessionStatus accepted unsupported status")
	}
}
