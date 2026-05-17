package smtpd

import (
	"context"
	"time"

	"github.com/gogomail/gogomail/internal/message"
)

type Stage string

const (
	StageAuthenticated         Stage = "authenticated"
	StageMailFrom              Stage = "mail_from"
	StageRcpt                  Stage = "rcpt"
	StageBackpressureChecked   Stage = "backpressure_checked"
	StageSpooled               Stage = "spooled"
	StageParsed                Stage = "parsed"
	StageAuthenticationChecked Stage = "authentication_checked"
	StageDedupChecked          Stage = "dedup_checked"
	StageStored                Stage = "stored"
	StageRecorded              Stage = "recorded"
)

type Event struct {
	Stage          Stage
	RemoteAddr     string
	EnvelopeFrom   string
	Mailbox        Mailbox
	SubmissionUser SubmissionUser
	Recipients     []string
	DSN            DSNOptions
	SpoolPath      string
	StoragePath    string
	Parsed         message.ParsedMessage
	Authentication AuthenticationResults
	ReceivedAt     time.Time
	SubmittedAt    time.Time
	Size           int64
	Duplicate      bool
}

type Hook func(ctx context.Context, event Event) error
