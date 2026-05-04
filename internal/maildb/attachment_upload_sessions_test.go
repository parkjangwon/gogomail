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
