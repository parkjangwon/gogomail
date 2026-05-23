package maildb_test

import (
	"testing"

	"github.com/gogomail/gogomail/internal/maildb"
)

func TestWebPushSubscriptionRequest_Validate(t *testing.T) {
	tests := []struct {
		name    string
		req     maildb.UpsertWebPushSubscriptionRequest
		wantErr bool
	}{
		{
			name: "valid",
			req: maildb.UpsertWebPushSubscriptionRequest{
				UserID:   "11111111-1111-1111-1111-111111111111",
				Endpoint: "https://updates.push.services.mozilla.com/wpush/v2/abc123",
				P256DH:   "BNcRdreALRFXTkOOUHK1EtK2wtCONKTMl7aEkYaSs8k",
				Auth:     "tBHItJI5svbpez7KI4CCXg",
			},
		},
		{
			name:    "missing user_id",
			req:     maildb.UpsertWebPushSubscriptionRequest{Endpoint: "https://example.com", P256DH: "a", Auth: "b"},
			wantErr: true,
		},
		{
			name:    "missing endpoint",
			req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", P256DH: "a", Auth: "b"},
			wantErr: true,
		},
		{
			name:    "endpoint not https",
			req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "http://example.com", P256DH: "a", Auth: "b"},
			wantErr: true,
		},
		{
			name:    "missing p256dh",
			req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "https://example.com", Auth: "b"},
			wantErr: true,
		},
		{
			name:    "missing auth",
			req:     maildb.UpsertWebPushSubscriptionRequest{UserID: "11111111-1111-1111-1111-111111111111", Endpoint: "https://example.com", P256DH: "a"},
			wantErr: true,
		},
		{
			name: "whitespace only user_id",
			req: maildb.UpsertWebPushSubscriptionRequest{
				UserID:   "   ",
				Endpoint: "https://example.com",
				P256DH:   "a",
				Auth:     "b",
			},
			wantErr: true,
		},
		{
			name: "user_id with embedded line break",
			req: maildb.UpsertWebPushSubscriptionRequest{
				UserID:   "11111111\n-1111-1111-1111-111111111111",
				Endpoint: "https://example.com",
				P256DH:   "a",
				Auth:     "b",
			},
			wantErr: true,
		},
		{
			name: "endpoint exceeds max length",
			req: maildb.UpsertWebPushSubscriptionRequest{
				UserID:   "11111111-1111-1111-1111-111111111111",
				Endpoint: "https://" + string(make([]byte, 2048)),
				P256DH:   "a",
				Auth:     "b",
			},
			wantErr: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.req.Validate()
			if (err != nil) != tc.wantErr {
				t.Fatalf("Validate() error = %v, wantErr = %v", err, tc.wantErr)
			}
		})
	}
}
