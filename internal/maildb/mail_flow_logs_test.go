package maildb

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNormalizeMailFlowLogListRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  MailFlowLogListRequest
		want MailFlowLogListRequest
	}{
		{
			name: "zero limit defaults to 50",
			req:  MailFlowLogListRequest{},
			want: MailFlowLogListRequest{Limit: 50},
		},
		{
			name: "negative limit defaults to 50",
			req:  MailFlowLogListRequest{Limit: -1},
			want: MailFlowLogListRequest{Limit: 50},
		},
		{
			name: "limit over 200 caps at 200",
			req:  MailFlowLogListRequest{Limit: 1000},
			want: MailFlowLogListRequest{Limit: 200},
		},
		{
			name: "valid limit unchanged",
			req:  MailFlowLogListRequest{Limit: 100},
			want: MailFlowLogListRequest{Limit: 100},
		},
		{
			name: "trims whitespace",
			req:  MailFlowLogListRequest{Direction: " inbound ", CompanyID: " 123 ", DomainID: " 456 "},
			want: MailFlowLogListRequest{Limit: 50, Direction: "inbound", CompanyID: "123", DomainID: "456"},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeMailFlowLogListRequest(tt.req)
			if got != tt.want {
				t.Errorf("normalizeMailFlowLogListRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeMailFlowLogStatsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  MailFlowLogStatsRequest
		want MailFlowLogStatsRequest
	}{
		{
			name: "empty request",
			req:  MailFlowLogStatsRequest{},
			want: MailFlowLogStatsRequest{},
		},
		{
			name: "trims whitespace",
			req:  MailFlowLogStatsRequest{Direction: " inbound ", CompanyID: " 123 ", DomainID: " 456 ", UserID: " 789 "},
			want: MailFlowLogStatsRequest{Direction: "inbound", CompanyID: "123", DomainID: "456", UserID: "789"},
		},
		{
			name: "preserves time values",
			req:  MailFlowLogStatsRequest{Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
			want: MailFlowLogStatsRequest{Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeMailFlowLogStatsRequest(tt.req)
			if got != tt.want {
				t.Errorf("normalizeMailFlowLogStatsRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestNormalizeMailFlowLogDailyStatsRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		req  MailFlowLogDailyStatsRequest
		want MailFlowLogDailyStatsRequest
	}{
		{
			name: "empty request",
			req:  MailFlowLogDailyStatsRequest{},
			want: MailFlowLogDailyStatsRequest{},
		},
		{
			name: "trims whitespace",
			req:  MailFlowLogDailyStatsRequest{Direction: " outbound ", CompanyID: " abc ", DomainID: " def ", UserID: " ghi "},
			want: MailFlowLogDailyStatsRequest{Direction: "outbound", CompanyID: "abc", DomainID: "def", UserID: "ghi"},
		},
		{
			name: "preserves time values",
			req:  MailFlowLogDailyStatsRequest{Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
			want: MailFlowLogDailyStatsRequest{Since: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Until: time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC)},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := normalizeMailFlowLogDailyStatsRequest(tt.req)
			if got != tt.want {
				t.Errorf("normalizeMailFlowLogDailyStatsRequest() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestMailFlowDirectionConstants(t *testing.T) {
	t.Parallel()

	if MailFlowDirectionInbound != "inbound" {
		t.Errorf("MailFlowDirectionInbound = %q, want %q", MailFlowDirectionInbound, "inbound")
	}
	if MailFlowDirectionOutbound != "outbound" {
		t.Errorf("MailFlowDirectionOutbound = %q, want %q", MailFlowDirectionOutbound, "outbound")
	}
}

func TestMailFlowStatusConstants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status MailFlowStatus
		want   string
	}{
		{MailFlowStatusReceived, "received"},
		{MailFlowStatusDelivered, "delivered"},
		{MailFlowStatusFailed, "failed"},
		{MailFlowStatusBounced, "bounced"},
		{MailFlowStatusFiltered, "filtered"},
		{MailFlowStatusRejected, "rejected"},
		{MailFlowStatusPending, "pending"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.want {
			t.Errorf("MailFlowStatus = %q, want %q", tt.status, tt.want)
		}
	}
}

func TestMailFlowLogQueriesKeepUUIDFiltersSargable(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("mail_flow_logs.go")
	if err != nil {
		t.Fatalf("read mail_flow_logs.go: %v", err)
	}
	for _, forbidden := range []string{
		"company_id::text =",
		"domain_id::text =",
		"user_id::text =",
		"message_id::text =",
	} {
		if strings.Contains(string(source), forbidden) {
			t.Fatalf("mail flow log query still casts indexed UUID column in predicate: %s", forbidden)
		}
	}
}

func TestNullString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  any
	}{
		{"empty string returns nil", "", nil},
		{"whitespace returns value", "   ", "   "},
		{"value returns value", "test", "test"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := nullString(tt.value)
			if got != tt.want {
				t.Errorf("nullString(%q) = %v, want %v", tt.value, got, tt.want)
			}
		})
	}
}
