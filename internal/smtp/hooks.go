package smtpd

import (
	"context"
	"time"

	"github.com/gogomail/gogomail/internal/message"
)

type Stage string

const (
	StageRcpt         Stage = "rcpt"
	StageSpooled      Stage = "spooled"
	StageParsed       Stage = "parsed"
	StageDedupChecked Stage = "dedup_checked"
	StageStored       Stage = "stored"
	StageRecorded     Stage = "recorded"
)

type Event struct {
	Stage        Stage
	EnvelopeFrom string
	Mailbox      Mailbox
	StoragePath  string
	Parsed       message.ParsedMessage
	ReceivedAt   time.Time
	Size         int64
	Duplicate    bool
}

type Hook func(ctx context.Context, event Event) error
