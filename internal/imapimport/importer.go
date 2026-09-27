package imapimport

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gogomail/gogomail/internal/imapgw"
)

// StoredMessage is the payload handed to a MessageSink for one message.
type StoredMessage struct {
	// Destination is the mapped gogomail folder name.
	Destination string
	// Flags are the translated gogomail flags.
	Flags imapgw.MessageFlags
	// InternalDate is the source INTERNALDATE (preserved).
	InternalDate time.Time
	// Raw is the full RFC 5322 message.
	Raw []byte
	// IdempotencyKey is the Message-ID (or content hash) for de-duplication.
	IdempotencyKey string
}

// MessageSink persists imported messages into a target gogomail mailbox. The
// concrete implementation (cmd/gogomail) wraps maildb + storage; tests use a
// fake in-memory sink.
type MessageSink interface {
	// EnsureFolder makes sure the destination folder exists, creating it if
	// necessary, and returns nil on success. It must be safe to call
	// concurrently for the same name.
	EnsureFolder(ctx context.Context, name string) error
	// Store persists one message. Implementations may return ErrAlreadyExists
	// if they detect a duplicate server-side; the importer also de-dupes via
	// the ProgressStore.
	Store(ctx context.Context, msg StoredMessage) error
}

// ProgressSnapshot is a mutex-free, serializable view of Progress used at the
// ProgressStore boundary so no lock is ever copied.
type ProgressSnapshot struct {
	Imported  []string `json:"imported"`
	Completed []string `json:"completed_folders"`
}

// ProgressStore persists per-folder import progress so an interrupted run can
// resume without re-importing. Implementations are typically a JSON file.
type ProgressStore interface {
	// Load returns the saved progress snapshot, or an empty snapshot if none
	// exists.
	Load() (ProgressSnapshot, error)
	// Save atomically persists a progress snapshot.
	Save(ProgressSnapshot) error
}

// Progress is the resumable state of an import run.
type Progress struct {
	// Imported maps idempotency key → struct{} for every message already
	// written. Serialized as a sorted slice for stable diffs.
	imported map[string]struct{}
	// CompletedFolders records source folders fully drained.
	completed map[string]struct{}
	mu        sync.Mutex
}

// NewProgress returns an empty Progress.
func NewProgress() *Progress {
	return &Progress{imported: map[string]struct{}{}, completed: map[string]struct{}{}}
}

// Has reports whether a message key was already imported.
func (p *Progress) Has(key string) bool {
	if p == nil || key == "" {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.imported[key]
	return ok
}

// MarkImported records a message key as imported.
func (p *Progress) MarkImported(key string) {
	if p == nil || key == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.imported[key] = struct{}{}
}

// FolderComplete reports whether a folder was fully drained on a prior run.
func (p *Progress) FolderComplete(name string) bool {
	if p == nil {
		return false
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	_, ok := p.completed[name]
	return ok
}

// MarkFolderComplete records that a folder was fully imported.
func (p *Progress) MarkFolderComplete(name string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.completed[name] = struct{}{}
}

// ImportedKeys returns a sorted snapshot of imported keys (for serialization).
func (p *Progress) ImportedKeys() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return sortedKeys(p.imported)
}

// CompletedFolders returns a sorted snapshot of completed folders.
func (p *Progress) CompletedFolders() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return sortedKeys(p.completed)
}

// SetImportedKeys/SetCompletedFolders repopulate progress after loading.
func (p *Progress) SetImportedKeys(keys []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.imported == nil {
		p.imported = make(map[string]struct{}, len(keys))
	}
	for _, k := range keys {
		if k != "" {
			p.imported[k] = struct{}{}
		}
	}
}

func (p *Progress) SetCompletedFolders(names []string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.completed == nil {
		p.completed = make(map[string]struct{}, len(names))
	}
	for _, n := range names {
		if n != "" {
			p.completed[n] = struct{}{}
		}
	}
}

// Options configures an import run.
type Options struct {
	Mapping MappingConfig
	// Concurrency is the number of folders imported in parallel. Defaults to 1.
	Concurrency int
	// RateLimit bounds messages fetched+stored per second across all workers
	// (0 = unlimited). Protects source servers with strict throttling
	// (Gmail/M365).
	RateLimit float64
	// DryRun reports counts/sizes without storing anything.
	DryRun bool
	// Logger receives human-readable progress lines; may be nil.
	Logger func(format string, args ...any)
	// SaveEvery persists progress after this many stored messages (0 = every
	// message is expensive; default 50).
	SaveEvery int
}

func (o Options) concurrency() int {
	if o.Concurrency <= 0 {
		return 1
	}
	return o.Concurrency
}

func (o Options) saveEvery() int {
	if o.SaveEvery <= 0 {
		return 50
	}
	return o.SaveEvery
}

func (o Options) log(format string, args ...any) {
	if o.Logger != nil {
		o.Logger(format, args...)
	}
}

// Report summarizes an import run.
type Report struct {
	FoldersTotal    int
	FoldersSkipped  int
	FoldersImported int
	MessagesSeen    int
	MessagesStored  int
	MessagesSkipped int // duplicates skipped via idempotency
	BytesSeen       int64
	// PerFolder maps destination folder → messages stored (or, in dry-run,
	// messages that would be stored).
	PerFolder map[string]int
	// Errors collects non-fatal per-message/per-folder errors so the run can
	// continue and surface a summary.
	Errors []string
}

func newReport() *Report {
	return &Report{PerFolder: map[string]int{}}
}

// Importer runs an import from a SourceClient into a MessageSink.
type Importer struct {
	src      SourceClient
	sink     MessageSink
	progress *Progress
	store    ProgressStore
	opts     Options

	mu sync.Mutex // guards report mutation
}

// New constructs an Importer. sink and store may be nil for dry-run; in that
// case no messages are persisted and progress is not saved.
func New(src SourceClient, sink MessageSink, store ProgressStore, opts Options) *Importer {
	progress := NewProgress()
	if store != nil {
		if loaded, err := store.Load(); err == nil {
			progress.SetImportedKeys(loaded.Imported)
			progress.SetCompletedFolders(loaded.Completed)
		}
	}
	return &Importer{src: src, sink: sink, progress: progress, store: store, opts: opts}
}

// Run executes the import and returns a Report. It is context-cancelable;
// cancellation stops scheduling new work and returns the partial report.
func (im *Importer) Run(ctx context.Context) (*Report, error) {
	report := newReport()

	folders, err := im.src.ListFolders(ctx)
	if err != nil {
		return report, fmt.Errorf("list source folders: %w", err)
	}
	report.FoldersTotal = len(folders)

	// Build the work list, applying mapping and skip rules.
	type job struct {
		source string
		mapped FolderMapping
	}
	var jobs []job
	for _, f := range folders {
		if !f.Selectable() {
			report.FoldersSkipped++
			continue
		}
		mapped := MapFolder(im.opts.Mapping, f.Name)
		if mapped.Skip {
			im.opts.log("skip source folder %q (virtual/duplicate container)", f.Name)
			report.FoldersSkipped++
			continue
		}
		jobs = append(jobs, job{source: f.Name, mapped: mapped})
	}
	// Deterministic order for stable resume/logging.
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].source < jobs[j].source })

	limiter := newRateLimiter(im.opts.RateLimit)

	// A single connection cannot safely be shared across concurrent SELECTs, so
	// the default real client runs concurrency=1. Fake clients (tests) and
	// per-folder clients can raise it. We still parallelize the sink work.
	sem := make(chan struct{}, im.opts.concurrency())
	var wg sync.WaitGroup
	var stored int

	for _, jb := range jobs {
		if ctx.Err() != nil {
			break
		}
		jb := jb
		if im.progress.FolderComplete(jb.source) && !im.opts.DryRun {
			im.opts.log("resume: folder %q already complete, skipping", jb.source)
			im.mu.Lock()
			report.FoldersImported++
			im.mu.Unlock()
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-sem }()

			folderStored, folderErr := im.importFolder(ctx, jb.source, jb.mapped, limiter, report)
			im.mu.Lock()
			report.FoldersImported++
			stored += folderStored
			if folderErr != nil {
				report.Errors = append(report.Errors, fmt.Sprintf("folder %q: %v", jb.source, folderErr))
			}
			im.mu.Unlock()

			if folderErr == nil && !im.opts.DryRun {
				im.progress.MarkFolderComplete(jb.source)
				im.saveProgress()
			}
		}()
	}
	wg.Wait()

	if !im.opts.DryRun {
		im.saveProgress()
	}
	return report, ctx.Err()
}

func (im *Importer) importFolder(ctx context.Context, source string, mapped FolderMapping, limiter *rateLimiter, report *Report) (int, error) {
	if im.opts.DryRun {
		status, err := im.src.Status(ctx, source)
		if err != nil {
			return 0, err
		}
		im.mu.Lock()
		report.MessagesSeen += int(status.Messages)
		report.BytesSeen += status.Bytes
		report.PerFolder[mapped.Destination] += int(status.Messages)
		im.mu.Unlock()
		im.opts.log("dry-run: %q → %q: %d messages", source, mapped.Destination, status.Messages)
		return 0, nil
	}

	if im.sink == nil {
		return 0, fmt.Errorf("message sink is required for a non-dry-run import")
	}
	if err := im.sink.EnsureFolder(ctx, mapped.Destination); err != nil {
		return 0, fmt.Errorf("ensure folder %q: %w", mapped.Destination, err)
	}

	var folderStored int
	fetchErr := im.src.FetchMessages(ctx, source, func(sm SourceMessage) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := limiter.wait(ctx); err != nil {
			return err
		}

		im.mu.Lock()
		report.MessagesSeen++
		report.BytesSeen += int64(len(sm.Raw))
		im.mu.Unlock()

		key := DeriveIdempotencyKey(messageIDFromRaw(sm.Raw), sm.Raw)
		if im.progress.Has(key) {
			im.mu.Lock()
			report.MessagesSkipped++
			im.mu.Unlock()
			return nil
		}

		var extra []string
		if mapped.Label != "" {
			extra = append(extra, sanitizeLabel(mapped.Label))
		}
		flags := TranslateFlags(sm.Flags, extra...)

		internalDate := sm.InternalDate
		if internalDate.IsZero() {
			internalDate = time.Now().UTC()
		}
		if err := im.sink.Store(ctx, StoredMessage{
			Destination:    mapped.Destination,
			Flags:          flags,
			InternalDate:   internalDate,
			Raw:            sm.Raw,
			IdempotencyKey: key,
		}); err != nil {
			im.mu.Lock()
			report.Errors = append(report.Errors, fmt.Sprintf("store message uid=%d in %q: %v", sm.UID, source, err))
			im.mu.Unlock()
			return nil // non-fatal: continue the folder
		}

		im.progress.MarkImported(key)
		im.mu.Lock()
		report.MessagesStored++
		report.PerFolder[mapped.Destination]++
		im.mu.Unlock()
		folderStored++

		if folderStored%im.opts.saveEvery() == 0 {
			im.saveProgress()
		}
		return nil
	})
	if fetchErr != nil {
		return folderStored, fetchErr
	}
	im.opts.log("imported %q → %q: %d messages", source, mapped.Destination, folderStored)
	return folderStored, nil
}

func (im *Importer) saveProgress() {
	if im.store == nil {
		return
	}
	snapshot := ProgressSnapshot{
		Imported:  im.progress.ImportedKeys(),
		Completed: im.progress.CompletedFolders(),
	}
	if err := im.store.Save(snapshot); err != nil {
		im.opts.log("warning: save progress failed: %v", err)
	}
}

// sanitizeLabel makes a folder-path label a valid IMAP keyword atom by replacing
// spaces and separators with underscores.
func sanitizeLabel(label string) string {
	replacer := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", "(", "", ")", "", "{", "", "}", "", "\"", "")
	return "Label_" + replacer.Replace(strings.TrimSpace(label))
}
