package maildb

import (
	"strings"
	"testing"
	"time"
)

func TestListPushNotificationAttemptsQueryUsesSargableFilters(t *testing.T) {
	t.Parallel()

	query, args := buildListPushNotificationAttemptsQuery(PushNotificationAttemptListRequest{
		Limit:             101,
		MessageID:         "123e4567-e89b-12d3-a456-426614174000",
		Status:            "failed",
		UserID:            "123e4567-e89b-12d3-a456-426614174001",
		Platform:          "webpush",
		DeviceID:          "123e4567-e89b-12d3-a456-426614174002",
		ProviderStatus:    "410",
		ProviderMessageID: "provider-message-1",
		Since:             time.Date(2026, 5, 21, 1, 2, 3, 0, time.FixedZone("KST", 9*60*60)),
	})

	for _, want := range []string{
		"WHERE message_id = $1::uuid",
		"AND status = $2",
		"AND user_id = $3::uuid",
		"AND attempted_at >= $4",
		"AND platform = $5",
		"AND device_id = $6::uuid",
		"AND provider_status = $7",
		"AND provider_message_id = $8",
		"ORDER BY attempted_at DESC, id DESC",
		"LIMIT $9",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("push notification attempt list query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"::timestamptz IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("push notification attempt list query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 9 {
		t.Fatalf("args len = %d, want 9", len(args))
	}
	if got := args[3].(time.Time).Location(); got != time.UTC {
		t.Fatalf("since arg location = %v, want UTC", got)
	}
}

func TestPushNotificationStatsQueryUsesSargableFilters(t *testing.T) {
	t.Parallel()

	query, args := buildPushNotificationStatsQuery(PushNotificationStatsRequest{
		MessageID: "123e4567-e89b-12d3-a456-426614174010",
		UserID:    "123e4567-e89b-12d3-a456-426614174011",
		Platform:  "webpush",
		DeviceID:  "123e4567-e89b-12d3-a456-426614174012",
		Since:     time.Date(2026, 5, 21, 4, 5, 6, 0, time.FixedZone("KST", 9*60*60)),
	})

	for _, want := range []string{
		"FROM push_devices",
		"WHERE status = 'active'",
		"AND user_id = $1::uuid",
		"AND platform = $3",
		"AND id = $4::uuid",
		"FROM push_notification_attempts",
		"WHERE user_id = $1::uuid",
		"AND message_id = $2::uuid",
		"AND platform = $3",
		"AND device_id = $4::uuid",
		"AND attempted_at >= $5",
	} {
		if !strings.Contains(query, want) {
			t.Fatalf("push notification stats query missing %q:\n%s", want, query)
		}
	}
	for _, forbidden := range []string{
		"NULLIF",
		" OR ",
		"::timestamptz IS NULL",
	} {
		if strings.Contains(query, forbidden) {
			t.Fatalf("push notification stats query contains non-sargable filter %q:\n%s", forbidden, query)
		}
	}
	if len(args) != 5 {
		t.Fatalf("args len = %d, want 5", len(args))
	}
	if got := args[4].(time.Time).Location(); got != time.UTC {
		t.Fatalf("since arg location = %v, want UTC", got)
	}
}
