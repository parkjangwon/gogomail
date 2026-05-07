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

func TestValidateListUploadSessionsRequest(t *testing.T) {
	t.Parallel()

	req, err := ValidateListUploadSessionsRequest(ListUploadSessionsRequest{
		UserID: " user-1 ",
		Status: " Uploading ",
		Limit:  0,
	})
	if err != nil {
		t.Fatalf("ValidateListUploadSessionsRequest returned error: %v", err)
	}
	if req.UserID != "user-1" || req.Status != UploadSessionStatusUploading || req.Limit != UploadSessionCleanupDefaultLimit {
		t.Fatalf("request = %+v, want normalized list request", req)
	}
}

func TestValidateListUploadSessionsRequestRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []ListUploadSessionsRequest{
		{Status: UploadSessionStatusPending},
		{UserID: "user\n1"},
		{UserID: "user-1", Status: "ready"},
		{UserID: "user-1", Limit: -1},
	}
	for _, tc := range tests {
		tc := tc
		t.Run(tc.UserID+"-"+tc.Status, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateListUploadSessionsRequest(tc); err == nil {
				t.Fatalf("ValidateListUploadSessionsRequest(%+v) error = nil, want rejection", tc)
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

func TestValidateRecordUploadSessionBodyRequestRejectsWrongUserPath(t *testing.T) {
	t.Parallel()

	_, err := ValidateRecordUploadSessionBodyRequest(RecordUploadSessionBodyRequest{
		UserID:         "user-1",
		SessionID:      "session-1",
		StoragePath:    "drive/users/user-2/upload-sessions/session-1/bodies/body-1",
		ChecksumSHA256: strings.Repeat("a", 64),
	})
	if err == nil {
		t.Fatal("ValidateRecordUploadSessionBodyRequest accepted another user's storage path")
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

func TestParseContentRange(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		value       string
		want        ContentRange
		wantErr     bool
		errContains string
	}{
		{
			name:    "asterisk form",
			value:   "bytes */100",
			want:    ContentRange{Total: 100, IsAsteriskForm: true},
			wantErr: false,
		},
		{
			name:    "full range",
			value:   "bytes 0-99/100",
			want:    ContentRange{Start: 0, End: 99, Total: 100},
			wantErr: false,
		},
		{
			name:    "full range with spaces",
			value:   "bytes  0 - 99 / 100 ",
			want:    ContentRange{Start: 0, End: 99, Total: 100},
			wantErr: false,
		},
		{
			name:        "empty value",
			value:       "",
			wantErr:     true,
			errContains: "empty",
		},
		{
			name:        "missing bytes prefix",
			value:       "*/100",
			wantErr:     true,
			errContains: "bytes",
		},
		{
			name:        "missing slash",
			value:       "bytes 0-99",
			wantErr:     true,
			errContains: "<range>/<total>",
		},
		{
			name:        "start greater than end",
			value:       "bytes 10-5/100",
			wantErr:     true,
			errContains: "start must not exceed end",
		},
		{
			name:        "end equals total",
			value:       "bytes 0-100/100",
			wantErr:     true,
			errContains: "end must be less than total",
		},
		{
			name:        "end greater than total",
			value:       "bytes 0-101/100",
			wantErr:     true,
			errContains: "end must be less than total",
		},
		{
			name:        "negative start",
			value:       "bytes -1-99/100",
			wantErr:     true,
			errContains: "invalid start",
		},
		{
			name:        "negative end",
			value:       "bytes 0--1/100",
			wantErr:     true,
			errContains: "invalid end",
		},
		{
			name:        "negative total",
			value:       "bytes 0-99/-100",
			wantErr:     true,
			errContains: "invalid total size",
		},
		{
			name:        "non-numeric start",
			value:       "bytes ab-99/100",
			wantErr:     true,
			errContains: "invalid start",
		},
		{
			name:        "non-numeric end",
			value:       "bytes 0-xy/100",
			wantErr:     true,
			errContains: "invalid end",
		},
		{
			name:        "non-numeric total",
			value:       "bytes 0-99/abc",
			wantErr:     true,
			errContains: "invalid total size",
		},
		{
			name:        "missing asterisk value",
			value:       "bytes */",
			wantErr:     true,
			errContains: "invalid total size",
		},
		{
			name:        "missing range part",
			value:       "bytes /100",
			wantErr:     true,
			errContains: "<start>-<end>",
		},
		{
			name:        "too many dashes",
			value:       "bytes 0-99-100/200",
			wantErr:     true,
			errContains: "invalid end",
		},
		{
			name:    "zero total asterisk form",
			value:   "bytes */0",
			want:    ContentRange{Total: 0, IsAsteriskForm: true},
			wantErr: false,
		},
		{
			name:    "zero range",
			value:   "bytes 0-0/1",
			want:    ContentRange{Start: 0, End: 0, Total: 1},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseContentRange(tc.value)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseContentRange(%q) error = nil, want error containing %q", tc.value, tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("ParseContentRange(%q) error = %q, want error containing %q", tc.value, err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseContentRange(%q) returned error: %v", tc.value, err)
			}
			if got != tc.want {
				t.Fatalf("ParseContentRange(%q) = %+v, want %+v", tc.value, got, tc.want)
			}
		})
	}
}

func TestValidateContentRangeComplete(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		contentRange ContentRange
		declaredSize int64
		wantErr     bool
		errContains string
	}{
		{
			name:        "valid asterisk form matching size",
			contentRange: ContentRange{Total: 100, IsAsteriskForm: true},
			declaredSize: 100,
			wantErr:     false,
		},
		{
			name:        "valid full range matching size",
			contentRange: ContentRange{Start: 0, End: 99, Total: 100},
			declaredSize: 100,
			wantErr:     false,
		},
		{
			name:        "asterisk total mismatch",
			contentRange: ContentRange{Total: 200, IsAsteriskForm: true},
			declaredSize: 100,
			wantErr:     true,
			errContains: "total 200 does not match declared size 100",
		},
		{
			name:        "full range start not zero",
			contentRange: ContentRange{Start: 10, End: 99, Total: 100},
			declaredSize: 100,
			wantErr:     true,
			errContains: "start must be 0",
		},
		{
			name:        "full range end mismatch",
			contentRange: ContentRange{Start: 0, End: 98, Total: 100},
			declaredSize: 100,
			wantErr:     true,
			errContains: "end must be 99",
		},
		{
			name:        "full range total mismatch",
			contentRange: ContentRange{Start: 0, End: 99, Total: 200},
			declaredSize: 100,
			wantErr:     true,
			errContains: "total 200 does not match declared size 100",
		},
		{
			name:        "zero declared size",
			contentRange: ContentRange{Total: 100, IsAsteriskForm: true},
			declaredSize: 0,
			wantErr:     true,
			errContains: "declared size is zero",
		},
		{
			name:        "zero declared size non-asterisk",
			contentRange: ContentRange{Start: 0, End: 0, Total: 1},
			declaredSize: 0,
			wantErr:     true,
			errContains: "declared size is zero",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			err := ValidateContentRangeComplete(tc.contentRange, tc.declaredSize)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateContentRangeComplete(%+v, %d) error = nil, want error containing %q", tc.contentRange, tc.declaredSize, tc.errContains)
				}
				if !strings.Contains(err.Error(), tc.errContains) {
					t.Fatalf("ValidateContentRangeComplete(%+v, %d) error = %q, want error containing %q", tc.contentRange, tc.declaredSize, err.Error(), tc.errContains)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateContentRangeComplete(%+v, %d) returned error: %v", tc.contentRange, tc.declaredSize, err)
			}
		})
	}
}
