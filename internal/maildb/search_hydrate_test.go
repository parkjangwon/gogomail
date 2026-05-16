package maildb

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

func TestListMessagesByIDsSQLUsesUuidUnnest(t *testing.T) {
	t.Parallel()

	for _, want := range []string{
		"unnest($2::uuid[]) WITH ORDINALITY",
		"ORDER BY requested.ordinality",
		"JOIN messages m ON m.id = requested.id",
	} {
		if !strings.Contains(listMessagesByIDsSQL, want) {
			t.Fatalf("listMessagesByIDsSQL does not include %q:\n%s", want, listMessagesByIDsSQL)
		}
	}
	if strings.Contains(listMessagesByIDsSQL, "jsonb_array_elements_text") {
		t.Fatalf("listMessagesByIDsSQL still uses JSON array expansion:\n%s", listMessagesByIDsSQL)
	}
}

func BenchmarkNormalizeSearchMessageIDs100(b *testing.B) {
	benchNormalizeSearchMessageIDs(b, 100)
}

func BenchmarkNormalizeSearchMessageIDs200(b *testing.B) {
	benchNormalizeSearchMessageIDs(b, 200)
}

func BenchmarkSearchMessageIDsArrayValue1K(b *testing.B) {
	benchSearchMessageIDsArrayValue(b, 1_000)
}

func BenchmarkSearchMessageIDsArrayValue10K(b *testing.B) {
	benchSearchMessageIDsArrayValue(b, 10_000)
}

func benchNormalizeSearchMessageIDs(b *testing.B, count int) {
	b.Helper()
	messageIDs := benchmarkSearchMessageIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		normalized, err := normalizeSearchMessageIDs(messageIDs)
		if err != nil {
			b.Fatalf("normalizeSearchMessageIDs returned error: %v", err)
		}
		if len(normalized) != count {
			b.Fatalf("normalized len = %d, want %d", len(normalized), count)
		}
	}
}

func benchSearchMessageIDsArrayValue(b *testing.B, count int) {
	b.Helper()
	messageIDs := benchmarkSearchMessageIDs(count)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		value, err := pq.Array(messageIDs).Value()
		if err != nil {
			b.Fatalf("pq.Array.Value returned error: %v", err)
		}
		if value == nil {
			b.Fatal("pq.Array.Value returned nil")
		}
	}
}

func benchmarkSearchMessageIDs(count int) []string {
	ids := make([]string, 0, count)
	for len(ids) < count {
		ids = append(ids, uuid.NewString())
	}
	return ids
}
