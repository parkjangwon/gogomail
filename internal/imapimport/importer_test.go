package imapimport

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

// fakeSource is an in-memory SourceClient for tests — no network.
type fakeSource struct {
	folders  []SourceFolder
	messages map[string][]SourceMessage // folder name → messages
	closed   bool
}

func (f *fakeSource) ListFolders(_ context.Context) ([]SourceFolder, error) {
	return f.folders, nil
}

func (f *fakeSource) Status(_ context.Context, folder string) (FolderStatus, error) {
	msgs := f.messages[folder]
	var bytes int64
	for _, m := range msgs {
		bytes += int64(len(m.Raw))
	}
	return FolderStatus{Messages: uint32(len(msgs)), Bytes: bytes}, nil
}

func (f *fakeSource) FetchMessages(ctx context.Context, folder string, fn func(SourceMessage) error) error {
	for _, m := range f.messages[folder] {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := fn(m); err != nil {
			return err
		}
	}
	return nil
}

func (f *fakeSource) Close() error { f.closed = true; return nil }

// fakeSink records stored messages.
type fakeSink struct {
	mu       sync.Mutex
	folders  map[string]bool
	stored   []StoredMessage
	storeErr error
}

func newFakeSink() *fakeSink { return &fakeSink{folders: map[string]bool{}} }

func (s *fakeSink) EnsureFolder(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.folders[name] = true
	return nil
}

func (s *fakeSink) Store(_ context.Context, msg StoredMessage) error {
	if s.storeErr != nil {
		return s.storeErr
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stored = append(s.stored, msg)
	return nil
}

func rawWithID(id, body string) []byte {
	return []byte(fmt.Sprintf("Subject: t\r\nMessage-ID: <%s>\r\n\r\n%s", id, body))
}

func sampleSource() *fakeSource {
	return &fakeSource{
		folders: []SourceFolder{
			{Name: "INBOX", Delimiter: "/"},
			{Name: "Work/Projects", Delimiter: "/"},
			{Name: "[Gmail]", Delimiter: "/", Attributes: []string{`\Noselect`}},
			{Name: "[Gmail]/All Mail", Delimiter: "/", Attributes: []string{`\All`}},
		},
		messages: map[string][]SourceMessage{
			"INBOX": {
				{UID: 1, Flags: []string{`\Seen`}, InternalDate: time.Unix(1000, 0), Raw: rawWithID("a@x", "one")},
				{UID: 2, Flags: []string{`\Flagged`}, InternalDate: time.Unix(2000, 0), Raw: rawWithID("b@x", "two")},
			},
			"Work/Projects": {
				{UID: 1, InternalDate: time.Unix(3000, 0), Raw: rawWithID("c@x", "three")},
			},
			"[Gmail]/All Mail": {
				{UID: 9, Raw: rawWithID("a@x", "dup")}, // duplicate of INBOX a@x
			},
		},
	}
}

func TestImporterDryRun(t *testing.T) {
	src := sampleSource()
	imp := New(src, nil, nil, Options{
		Mapping: MappingConfig{Strategy: MappingPrefix, Separator: "/"},
		DryRun:  true,
	})
	report, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// [Gmail] is \Noselect (skipped), [Gmail]/All Mail is a well-known skip.
	if report.FoldersSkipped != 2 {
		t.Errorf("FoldersSkipped = %d want 2", report.FoldersSkipped)
	}
	// INBOX (2) + Work/Projects (1) = 3 messages counted, none stored.
	if report.MessagesSeen != 3 {
		t.Errorf("MessagesSeen = %d want 3", report.MessagesSeen)
	}
	if report.MessagesStored != 0 {
		t.Errorf("dry-run stored %d messages, want 0", report.MessagesStored)
	}
}

func TestImporterStoresWithMappingAndFlags(t *testing.T) {
	src := sampleSource()
	sink := newFakeSink()
	imp := New(src, sink, nil, Options{
		Mapping: MappingConfig{Strategy: MappingPrefix, Separator: "/", Prefix: "Imported"},
	})
	report, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.MessagesStored != 3 {
		t.Errorf("MessagesStored = %d want 3", report.MessagesStored)
	}
	if !sink.folders["INBOX"] || !sink.folders["Imported/Work/Projects"] {
		t.Errorf("folders not created as expected: %v", sink.folders)
	}
	// Verify flag translation + internal date preservation for INBOX msg 1.
	var found bool
	for _, m := range sink.stored {
		if m.IdempotencyKey == "a@x" {
			found = true
			if !m.Flags.Read {
				t.Error("\\Seen not translated to Read")
			}
			if !m.InternalDate.Equal(time.Unix(1000, 0)) {
				t.Errorf("internal date not preserved: %v", m.InternalDate)
			}
			if m.Destination != "INBOX" {
				t.Errorf("destination = %q want INBOX", m.Destination)
			}
		}
	}
	if !found {
		t.Error("INBOX message a@x not stored")
	}
}

func TestImporterFlatLabelsAddsLabelKeyword(t *testing.T) {
	src := &fakeSource{
		folders:  []SourceFolder{{Name: "Work/Projects", Delimiter: "/"}},
		messages: map[string][]SourceMessage{"Work/Projects": {{UID: 1, Raw: rawWithID("c@x", "x")}}},
	}
	sink := newFakeSink()
	imp := New(src, sink, nil, Options{Mapping: MappingConfig{Strategy: MappingFlatLabels, Separator: "/"}})
	if _, err := imp.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if len(sink.stored) != 1 {
		t.Fatalf("stored %d want 1", len(sink.stored))
	}
	if !sink.folders["Projects"] {
		t.Errorf("flat-labels should create leaf folder Projects: %v", sink.folders)
	}
	var hasLabel bool
	for _, k := range sink.stored[0].Flags.Keywords {
		if k == "Label_Work_Projects" {
			hasLabel = true
		}
	}
	if !hasLabel {
		t.Errorf("label keyword missing: %v", sink.stored[0].Flags.Keywords)
	}
}

func TestImporterIdempotencySkipsDuplicatesWithinRun(t *testing.T) {
	// Two folders both containing a message with the same Message-ID.
	src := &fakeSource{
		folders: []SourceFolder{{Name: "A", Delimiter: "/"}, {Name: "B", Delimiter: "/"}},
		messages: map[string][]SourceMessage{
			"A": {{UID: 1, Raw: rawWithID("same@x", "one")}},
			"B": {{UID: 1, Raw: rawWithID("same@x", "one")}},
		},
	}
	sink := newFakeSink()
	imp := New(src, sink, nil, Options{Mapping: MappingConfig{Strategy: MappingMerge, MergeTarget: "INBOX"}})
	report, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if report.MessagesStored != 1 {
		t.Errorf("MessagesStored = %d want 1 (duplicate should be skipped)", report.MessagesStored)
	}
	if report.MessagesSkipped != 1 {
		t.Errorf("MessagesSkipped = %d want 1", report.MessagesSkipped)
	}
}

func TestImporterResumeSkipsAlreadyImported(t *testing.T) {
	store := &memProgressStore{}
	// Pre-seed progress as if a@x and b@x were already imported.
	_ = store.Save(ProgressSnapshot{Imported: []string{"a@x", "b@x"}})

	src := sampleSource()
	sink := newFakeSink()
	imp := New(src, sink, store, Options{Mapping: MappingConfig{Strategy: MappingPrefix, Separator: "/"}})
	report, err := imp.Run(context.Background())
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// Only Work/Projects c@x should be newly stored; a@x/b@x already done, and
	// the [Gmail]/All Mail dup of a@x is skipped.
	if report.MessagesStored != 1 {
		t.Errorf("MessagesStored = %d want 1", report.MessagesStored)
	}
	// Progress should now include c@x.
	snap, _ := store.Load()
	var hasC bool
	for _, k := range snap.Imported {
		if k == "c@x" {
			hasC = true
		}
	}
	if !hasC {
		t.Errorf("progress missing c@x: %v", snap.Imported)
	}
}

func TestImporterRateLimit(t *testing.T) {
	src := &fakeSource{
		folders: []SourceFolder{{Name: "INBOX", Delimiter: "/"}},
		messages: map[string][]SourceMessage{
			"INBOX": {
				{UID: 1, Raw: rawWithID("a@x", "1")},
				{UID: 2, Raw: rawWithID("b@x", "2")},
				{UID: 3, Raw: rawWithID("c@x", "3")},
			},
		},
	}
	sink := newFakeSink()
	// 50 msg/sec → ~20ms spacing; 3 messages should take at least ~40ms.
	imp := New(src, sink, nil, Options{
		Mapping:   MappingConfig{Strategy: MappingMerge},
		RateLimit: 50,
	})
	start := time.Now()
	if _, err := imp.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if elapsed := time.Since(start); elapsed < 30*time.Millisecond {
		t.Errorf("rate limit not applied: elapsed %v", elapsed)
	}
	if len(sink.stored) != 3 {
		t.Errorf("stored %d want 3", len(sink.stored))
	}
}

// memProgressStore is an in-memory ProgressStore for tests.
type memProgressStore struct {
	mu   sync.Mutex
	snap ProgressSnapshot
}

func (m *memProgressStore) Load() (ProgressSnapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.snap, nil
}

func (m *memProgressStore) Save(s ProgressSnapshot) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snap = s
	return nil
}
