package maildb

import "testing"

func TestNormalizeSearchMessageIDsTrimsDeduplicatesAndPreservesOrder(t *testing.T) {
	got, err := normalizeSearchMessageIDs([]string{" msg-1 ", "msg-2", "msg-1", "", "msg-3"})
	if err != nil {
		t.Fatalf("normalizeSearchMessageIDs returned error: %v", err)
	}
	want := []string{"msg-1", "msg-2", "msg-3"}
	if len(got) != len(want) {
		t.Fatalf("ids = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ids = %#v, want %#v", got, want)
		}
	}
}

func TestNormalizeSearchMessageIDsCapsBatchSize(t *testing.T) {
	ids := make([]string, 201)
	for i := range ids {
		ids[i] = "msg"
	}
	if _, err := normalizeSearchMessageIDs(ids); err == nil {
		t.Fatal("normalizeSearchMessageIDs accepted too many ids")
	}
}
