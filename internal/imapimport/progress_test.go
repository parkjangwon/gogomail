package imapimport

import (
	"path/filepath"
	"testing"
)

func TestFileProgressStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	store := NewFileProgressStore(path)

	// Load on a missing file returns empty, no error.
	snap, err := store.Load()
	if err != nil {
		t.Fatalf("Load missing: %v", err)
	}
	if len(snap.Imported) != 0 || len(snap.Completed) != 0 {
		t.Errorf("expected empty snapshot, got %+v", snap)
	}

	want := ProgressSnapshot{
		Imported:  []string{"a@x", "b@x"},
		Completed: []string{"INBOX"},
	}
	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}
	got, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(got.Imported) != 2 || got.Imported[0] != "a@x" {
		t.Errorf("Imported = %v want [a@x b@x]", got.Imported)
	}
	if len(got.Completed) != 1 || got.Completed[0] != "INBOX" {
		t.Errorf("Completed = %v want [INBOX]", got.Completed)
	}

	// Overwrite is atomic and replaces prior state.
	if err := store.Save(ProgressSnapshot{Imported: []string{"c@x"}}); err != nil {
		t.Fatalf("Save overwrite: %v", err)
	}
	got2, _ := store.Load()
	if len(got2.Imported) != 1 || got2.Imported[0] != "c@x" {
		t.Errorf("overwrite Imported = %v want [c@x]", got2.Imported)
	}
}
