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
