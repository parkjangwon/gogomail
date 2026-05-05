package maildb

import "testing"

func TestCanonicalIMAPSubscriptionNamePreservesMailboxIdentity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
		want  string
	}{
		{name: "case insensitive", value: " INBOX ", want: "inbox"},
		{name: "leading delimiter", value: "/Archive", want: "/archive"},
		{name: "trailing delimiter", value: "Archive/", want: "archive/"},
		{name: "internal spacing", value: "Project  2026", want: "project  2026"},
		{name: "quoted direct value", value: `"Archive"`, want: `"archive"`},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := canonicalIMAPSubscriptionName(tt.value); got != tt.want {
				t.Fatalf("canonicalIMAPSubscriptionName(%q) = %q, want %q", tt.value, got, tt.want)
			}
		})
	}
}
