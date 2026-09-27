package imapimport

import (
	"context"
	"time"
)

// SourceFolder describes a mailbox discovered on the source server via LIST.
type SourceFolder struct {
	// Name is the full mailbox name as returned by the source (e.g.
	// "[Gmail]/Sent Mail" or "Work/Projects").
	Name string
	// Delimiter is the hierarchy separator reported by the source for this
	// mailbox (LIST response), e.g. "/" or ".".
	Delimiter string
	// Attributes are the mailbox \-attributes (e.g. "\Noselect", "\All",
	// "\Sent"). Used to skip non-selectable containers.
	Attributes []string
	// Exists is the reported message count (from SELECT/STATUS); may be zero
	// when unknown until the folder is selected.
	Exists uint32
}

// Selectable reports whether the folder can be SELECTed and fetched from.
// \Noselect mailboxes are pure hierarchy nodes (RFC 3501 §7.2.2).
func (f SourceFolder) Selectable() bool {
	for _, a := range f.Attributes {
		if a == `\Noselect` || a == `\NonExistent` {
			return false
		}
	}
	return true
}

// SourceMessage is a single fetched message from the source server.
type SourceMessage struct {
	// UID is the source-assigned UID within the currently selected folder.
	UID uint32
	// Flags are the raw IMAP flags on the message ("\Seen", "\Flagged", ...).
	Flags []string
	// InternalDate is the server's INTERNALDATE for the message.
	InternalDate time.Time
	// Raw is the full RFC 5322 message (BODY[]/RFC822).
	Raw []byte
}

// FolderStatus is a lightweight count/size summary used by dry-run.
type FolderStatus struct {
	Messages uint32
	// Bytes is the sum of RFC822.SIZE across the folder when the source
	// supports it; zero when unknown.
	Bytes int64
}

// SourceClient abstracts a source IMAP server. The importer depends only on this
// interface, so tests can drive the import loop with an in-memory fake and no
// real network I/O (required for `go test -short`).
type SourceClient interface {
	// ListFolders enumerates all selectable and non-selectable mailboxes.
	ListFolders(ctx context.Context) ([]SourceFolder, error)
	// Status returns message count/size for a folder without fetching bodies
	// (used by dry-run). Implementations may SELECT/EXAMINE or STATUS.
	Status(ctx context.Context, folder string) (FolderStatus, error)
	// FetchMessages selects the folder read-only and invokes fn for every
	// message in ascending UID order. Implementations should stream rather than
	// buffer the whole folder. Returning a non-nil error from fn aborts the
	// fetch and is propagated.
	FetchMessages(ctx context.Context, folder string, fn func(SourceMessage) error) error
	// Close releases the connection.
	Close() error
}
