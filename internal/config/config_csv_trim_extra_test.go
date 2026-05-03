package config

import "testing"

func TestSplitCSVTrimsAndDropsBlankValues(t *testing.T) {
	got := splitCSV(" a@example.com, ,b@example.com ,, c@example.com ")
	want := []string{"a@example.com", "b@example.com", "c@example.com"}
	if len(got) != len(want) {
		t.Fatalf("splitCSV length = %d, want %d (%v)", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("splitCSV[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
