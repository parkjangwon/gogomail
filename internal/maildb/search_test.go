package maildb

import "testing"

func TestMessageSearchQueryNormalizesLimit(t *testing.T) {
	t.Parallel()

	if got := (MessageSearchQuery{}).normalizedLimit(); got != MessageListDefaultLimit {
		t.Fatalf("default limit = %d", got)
	}
	if got := (MessageSearchQuery{Limit: 201}).normalizedLimit(); got != MessageListMaxLimit {
		t.Fatalf("max limit = %d", got)
	}
}
