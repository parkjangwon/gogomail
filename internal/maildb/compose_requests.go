package maildb

import (
	"time"

	"github.com/gogomail/gogomail/internal/outbound"
)

type SaveDraftRequest struct {
	UserID          string
	DraftID         string
	Intent          string
	SourceMessageID string
	From            string
	To              []outbound.Address
	Cc              []outbound.Address
	Bcc             []outbound.Address
	Subject         string
	TextBody        string
	HTMLBody        string
	AttachmentIDs   []string
	TrackOpens      bool
	ScheduledAt     time.Time
	// IfUpdatedAt enables optimistic locking: if non-zero, the update only
	// proceeds when draft_updated_at equals this value. A mismatch returns
	// ErrDraftConflict (HTTP 409).
	IfUpdatedAt time.Time
}

type CreateAttachmentUploadRequest struct {
	UserID      string
	DraftID     string
	Filename    string
	Size        int64
	MIMEType    string
	StoragePath string
}
