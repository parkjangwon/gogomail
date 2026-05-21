package maildb

import (
	"os"
	"strings"
	"testing"
)

func TestQuotaWarningEmitterThresholdLookupUsesTypedScopeID(t *testing.T) {
	t.Parallel()

	source, err := os.ReadFile("quota_warning_emitter.go")
	if err != nil {
		t.Fatalf("read quota_warning_emitter.go: %v", err)
	}
	text := string(source)
	if !strings.Contains(text, "scope_id = $%d::uuid") {
		t.Fatalf("quota warning threshold lookup does not use typed scope id predicate")
	}
	if strings.Contains(text, "scope_id::text =") {
		t.Fatalf("quota warning threshold lookup casts indexed scope id column")
	}
}
