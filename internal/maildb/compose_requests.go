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
	AttachmentIDs   []string
	TrackOpens      bool
	ScheduledAt     time.Time
}

type CreateAttachmentUploadRequest struct {
	UserID      string
	DraftID     string
	Filename    string
	Size        int64
	MIMEType    string
	StoragePath string
}
