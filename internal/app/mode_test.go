package app

import "testing"

func TestParseModeAcceptsKnownBackendModes(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		raw  string
		want Mode
	}{
		{name: "all in one", raw: "all-in-one", want: ModeAllInOne},
		{name: "edge mta", raw: "edge-mta", want: ModeEdgeMTA},
		{name: "inbound mta", raw: "inbound-mta", want: ModeInboundMTA},
		{name: "outbound mta", raw: "outbound-mta", want: ModeOutboundMTA},
		{name: "delivery worker", raw: "delivery-worker", want: ModeDeliveryWorker},
		{name: "search index worker", raw: "search-index-worker", want: ModeSearchIndexWorker},
		{name: "api metering worker", raw: "api-metering-worker", want: ModeAPIMeteringWorker},
		{name: "push notification worker", raw: "push-notification-worker", want: ModePushWorker},
		{name: "batch worker", raw: "batch-worker", want: ModeBatchWorker},
		{name: "outbox relay", raw: "outbox-relay", want: ModeOutboxRelay},
		{name: "event worker", raw: "event-worker", want: ModeEventWorker},
		{name: "auth server", raw: "auth-server", want: ModeAuthServer},
		{name: "mail api", raw: "mail-api", want: ModeMailAPI},
		{name: "admin api", raw: "admin-api", want: ModeAdminAPI},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := ParseMode(tt.raw)
			if err != nil {
				t.Fatalf("ParseMode(%q) returned error: %v", tt.raw, err)
			}
			if got != tt.want {
				t.Fatalf("ParseMode(%q) = %q, want %q", tt.raw, got, tt.want)
			}
		})
	}
}

func TestParseModeRejectsUnknownMode(t *testing.T) {
	t.Parallel()

	if _, err := ParseMode("webmail"); err == nil {
		t.Fatal("ParseMode accepted unknown backend mode")
	}
}
