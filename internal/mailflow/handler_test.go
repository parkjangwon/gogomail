package mailflow

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gogomail/gogomail/internal/eventstream"
)

func TestHandlerHandleEvent(t *testing.T) {
	t.Parallel()

	h := NewHandler(nil)

	tests := []struct {
		name    string
		payload json.RawMessage
		wantErr bool
	}{
		{
			name: "mail.stored valid",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"schema_version": "2026-05-04.mail-stored.v1",
				"message_id": "abc123",
				"rfc_message_id": "<test@example.com>",
				"company_id": "company-1",
				"domain_id": "domain-1",
				"user_id": "user-1",
				"recipient": "user@example.com",
				"subject": "Test Subject",
				"storage_path": "/path/to/message",
				"received_at": "2026-05-08T12:00:00Z",
				"size": 1024
			}`),
			wantErr: true,
		},
		{
			name: "mail.stored missing message_id",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"schema_version": "2026-05-04.mail-stored.v1",
				"company_id": "company-1"
			}`),
			wantErr: true,
		},
		{
			name: "mail.stored invalid schema_version",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"schema_version": "invalid",
				"message_id": "abc123"
			}`),
			wantErr: true,
		},
		{
			name:    "unknown event",
			payload: json.RawMessage(`{"event": "mail.unknown"}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			msg := eventstream.Message{Payload: tt.payload}
			err := h.HandleEvent(context.Background(), msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("HandleEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerParseInboundEvent(t *testing.T) {
	t.Parallel()

	h := &Handler{}

	tests := []struct {
		name    string
		payload json.RawMessage
		wantErr bool
	}{
		{
			name: "valid mail.stored event",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"schema_version": "2026-05-04.mail-stored.v1",
				"message_id": "abc123",
				"rfc_message_id": "<test@example.com>",
				"company_id": "company-1",
				"domain_id": "domain-1",
				"user_id": "user-1",
				"recipient": "user@example.com",
				"subject": "Test Subject",
				"storage_path": "/path/to/message",
				"received_at": "2026-05-08T12:00:00Z",
				"size": 1024
			}`),
			wantErr: false,
		},
		{
			name: "missing message_id",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"schema_version": "2026-05-04.mail-stored.v1"
			}`),
			wantErr: true,
		},
		{
			name: "message_id with newline",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"message_id": "abc\n123"
			}`),
			wantErr: true,
		},
		{
			name: "valid without optional fields",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"message_id": "abc123"
			}`),
			wantErr: false,
		},
		{
			name: "empty recipient",
			payload: json.RawMessage(`{
				"event": "mail.stored",
				"message_id": "abc123",
				"recipient": ""
			}`),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := h.parseInboundEvent(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseInboundEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerParseOutboundEvent(t *testing.T) {
	t.Parallel()

	h := &Handler{}

	tests := []struct {
		name    string
		payload json.RawMessage
		wantErr bool
	}{
		{
			name: "valid mail.delivered event",
			payload: json.RawMessage(`{
				"event": "mail.delivered",
				"message_id": "abc123",
				"rfc_message_id": "<test@example.com>",
				"company_id": "company-1",
				"domain_id": "domain-1",
				"farm": "farm-1",
				"sender": "sender@example.com",
				"recipient": "recipient@example.com",
				"recipient_domain": "example.com",
				"status": "delivered",
				"error_message": "",
				"attempted_at": "2026-05-08T12:00:00Z",
				"storage_path": "/path/to/message",
				"enhanced_status": "2.0.0"
			}`),
			wantErr: false,
		},
		{
			name: "valid mail.bounced event",
			payload: json.RawMessage(`{
				"event": "mail.bounced",
				"message_id": "abc123",
				"status": "bounced",
				"error_message": " mailbox full"
			}`),
			wantErr: false,
		},
		{
			name: "valid mail.delivery_failed event",
			payload: json.RawMessage(`{
				"event": "mail.delivery_failed",
				"message_id": "abc123",
				"status": "permanent_failure"
			}`),
			wantErr: false,
		},
		{
			name: "valid mail.delivery_exhausted event",
			payload: json.RawMessage(`{
				"event": "mail.delivery_exhausted",
				"message_id": "abc123"
			}`),
			wantErr: false,
		},
		{
			name: "missing message_id",
			payload: json.RawMessage(`{
				"event": "mail.delivered"
			}`),
			wantErr: true,
		},
		{
			name: "message_id with carriage return",
			payload: json.RawMessage(`{
				"event": "mail.delivered",
				"message_id": "abc\r123"
			}`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := h.parseOutboundEvent(tt.payload)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseOutboundEvent() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandlerMapDeliveryStatus(t *testing.T) {
	t.Parallel()

	h := &Handler{}

	tests := []struct {
		event  string
		status string
		want   string
	}{
		{"mail.delivered", "", "delivered"},
		{"mail.bounced", "", "bounced"},
		{"mail.delivery_failed", "temporary_failure", "pending"},
		{"mail.delivery_failed", "permanent_failure", "failed"},
		{"mail.delivery_failed", "", "failed"},
		{"mail.delivery_exhausted", "", "failed"},
		{"mail.unknown", "", "failed"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.event+"_"+tt.status, func(t *testing.T) {
			t.Parallel()
			got := h.mapDeliveryStatus(tt.event, tt.status)
			if string(got) != tt.want {
				t.Errorf("mapDeliveryStatus(%q, %q) = %q, want %q", tt.event, tt.status, got, tt.want)
			}
		})
	}
}