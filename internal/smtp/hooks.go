package smtpd

import (
	"context"
	"time"

	"github.com/gogomail/gogomail/internal/message"
)

type Stage string

const (
	StageAuthenticated       Stage = "authenticated"
	StageMailFrom            Stage = "mail_from"
	StageRcpt                Stage = "rcpt"
	StageBackpressureChecked Stage = "backpressure_checked"
	StageSpooled             Stage = "spooled"
	StageParsed              Stage = "parsed"
	StageDedupChecked        Stage = "dedup_checked"
	StageStored              Stage = "stored"
	StageRecorded            Stage = "recorded"
)

type Event struct {
	Stage          Stage
	EnvelopeFrom   string
	Mailbox        Mailbox
	SubmissionUser SubmissionUser
	Recipients     []string
	StoragePath    string
	Parsed         message.ParsedMessage
	ReceivedAt     time.Time
	SubmittedAt    time.Time
	Size           int64
	Duplicate      bool
}

type Hook func(ctx context.Context, event Event) error
