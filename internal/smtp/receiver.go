package smtpd

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/mail"
	"github.com/gogomail/gogomail/internal/message"
	"github.com/gogomail/gogomail/internal/storage"
)

type Mailbox struct {
	CompanyID string
	DomainID  string
	UserID    string
	Address   string
}

type RecipientResolver interface {
	ResolveRecipient(ctx context.Context, address string) (Mailbox, error)
}

type StaticResolver map[string]Mailbox

func (r StaticResolver) ResolveRecipient(_ context.Context, address string) (Mailbox, error) {
	normalized, err := mail.NormalizeAddress(address)
	if err != nil {
		return Mailbox{}, err
	}
	mailbox, ok := r[normalized]
	if !ok {
		return Mailbox{}, fmt.Errorf("recipient %q not found", normalized)
	}
	return mailbox, nil
}

type IDGenerator func() string

type MessageRecorder interface {
	Record(ctx context.Context, msg ReceivedMessage) error
}

type Deduplicator interface {
	CheckAndSet(ctx context.Context, key DedupKey) (bool, error)
}

type DedupKey struct {
	MessageID string
	Recipient string
}

type ReceivedMessage struct {
	EnvelopeFrom string
	Mailbox      Mailbox
	StoragePath  string
	Parsed       message.ParsedMessage
	ReceivedAt   time.Time
	Size         int64
}

type ReceiverOptions struct {
	Store           storage.Store
	Resolver        RecipientResolver
	Recorder        MessageRecorder
	Deduplicator    Deduplicator
	Hooks           []Hook
	Policy          ReceivePolicy
	IDGenerator     IDGenerator
	Clock           func() time.Time
	MaxMessageBytes int64
}

type Receiver struct {
	store        storage.Store
	resolver     RecipientResolver
	recorder     MessageRecorder
	deduplicator Deduplicator
	hooks        []Hook
	policy       ReceivePolicy
	idGenerator  IDGenerator
	clock        func() time.Time
}

func NewReceiver(opts ReceiverOptions) *Receiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	return &Receiver{
		store:        opts.Store,
		resolver:     opts.Resolver,
		recorder:     recorderOrDefault(opts.Recorder),
		deduplicator: deduplicatorOrDefault(opts.Deduplicator),
		hooks:        append([]Hook(nil), opts.Hooks...),
		policy:       normalizePolicy(opts.Policy, opts.MaxMessageBytes),
		idGenerator:  idGenerator,
		clock:        clockOrDefault(opts.Clock),
	}
}

func (r *Receiver) NewSession(_ *gosmtp.Conn) (gosmtp.Session, error) {
	if r.store == nil {
		return nil, fmt.Errorf("smtp receiver store is required")
	}
	if r.resolver == nil {
		return nil, fmt.Errorf("smtp receiver resolver is required")
	}
	return &session{receiver: r}, nil
}

type session struct {
	receiver   *Receiver
	from       string
	recipients []Mailbox
}

func (s *session) Mail(from string, _ *gosmtp.MailOptions) error {
	normalized, err := mail.NormalizeAddress(from)
	if err != nil {
		return err
	}
	s.from = normalized
	return nil
}

func (s *session) Rcpt(to string, _ *gosmtp.RcptOptions) error {
	if len(s.recipients) >= s.receiver.policy.MaxRecipientsPerMessage {
		return fmt.Errorf("too many recipients; max %d", s.receiver.policy.MaxRecipientsPerMessage)
	}

	mailbox, err := s.receiver.resolver.ResolveRecipient(context.Background(), to)
	if err != nil {
		return err
	}
	s.recipients = append(s.recipients, mailbox)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if s.from == "" {
		return fmt.Errorf("mail command is required before data")
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("at least one recipient is required before data")
	}

	spooled, size, err := spoolMessage(r, s.receiver.policy.MaxMessageBytes)
	if err != nil {
		return err
	}
	defer func() {
		_ = spooled.Close()
		_ = os.Remove(spooled.Name())
	}()
	if err := s.emit(context.Background(), Event{
		Stage:        StageSpooled,
		EnvelopeFrom: s.from,
		Size:         size,
	}); err != nil {
		return err
	}

	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("rewind spooled message for parse: %w", err)
	}
	parsed, err := message.ParseEML(spooled)
	if err != nil {
		return fmt.Errorf("parse smtp message: %w", err)
	}
	if err := s.emit(context.Background(), Event{
		Stage:        StageParsed,
		EnvelopeFrom: s.from,
		Parsed:       parsed,
		Size:         size,
	}); err != nil {
		return err
	}

	messageID := s.receiver.idGenerator()
	receivedAt := s.receiver.clock()
	for _, recipient := range s.recipients {
		shouldProcess, err := s.receiver.deduplicator.CheckAndSet(context.Background(), DedupKey{
			MessageID: parsed.MessageID,
			Recipient: recipient.Address,
		})
		if err != nil {
			return fmt.Errorf("check duplicate message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(context.Background(), Event{
			Stage:        StageDedupChecked,
			EnvelopeFrom: s.from,
			Mailbox:      recipient,
			Parsed:       parsed,
			ReceivedAt:   receivedAt,
			Size:         size,
			Duplicate:    !shouldProcess,
		}); err != nil {
			return err
		}
		if !shouldProcess {
			continue
		}

		path := BuildStoragePath(recipient, messageID, receivedAt)
		if _, err := spooled.Seek(0, io.SeekStart); err != nil {
			return fmt.Errorf("rewind spooled message for store: %w", err)
		}
		if err := s.receiver.store.Put(context.Background(), path, spooled); err != nil {
			return fmt.Errorf("store message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(context.Background(), Event{
			Stage:        StageStored,
			EnvelopeFrom: s.from,
			Mailbox:      recipient,
			StoragePath:  path,
			Parsed:       parsed,
			ReceivedAt:   receivedAt,
			Size:         size,
		}); err != nil {
			return err
		}
		if err := s.receiver.recorder.Record(context.Background(), ReceivedMessage{
			EnvelopeFrom: s.from,
			Mailbox:      recipient,
			StoragePath:  path,
			Parsed:       parsed,
			ReceivedAt:   receivedAt,
			Size:         size,
		}); err != nil {
			return fmt.Errorf("record message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(context.Background(), Event{
			Stage:        StageRecorded,
			EnvelopeFrom: s.from,
			Mailbox:      recipient,
			StoragePath:  path,
			Parsed:       parsed,
			ReceivedAt:   receivedAt,
			Size:         size,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) emit(ctx context.Context, event Event) error {
	for _, hook := range s.receiver.hooks {
		if err := hook(ctx, event); err != nil {
			return fmt.Errorf("smtp hook %s: %w", event.Stage, err)
		}
	}
	return nil
}

func (s *session) Reset() {
	s.from = ""
	s.recipients = nil
}

func (s *session) Logout() error {
	s.Reset()
	return nil
}

func BuildStoragePath(mailbox Mailbox, messageID string, receivedAt time.Time) string {
	return strings.Join([]string{
		"mailstore",
		mailbox.CompanyID,
		mailbox.DomainID,
		mailbox.UserID,
		"maildir",
		receivedAt.Format("2006"),
		receivedAt.Format("01"),
		messageID + ".eml",
	}, "/")
}

func clockOrDefault(clock func() time.Time) func() time.Time {
	if clock != nil {
		return clock
	}
	return time.Now
}

type noopRecorder struct{}

func (noopRecorder) Record(context.Context, ReceivedMessage) error {
	return nil
}

func recorderOrDefault(recorder MessageRecorder) MessageRecorder {
	if recorder != nil {
		return recorder
	}
	return noopRecorder{}
}

type noopDeduplicator struct{}

func (noopDeduplicator) CheckAndSet(context.Context, DedupKey) (bool, error) {
	return true, nil
}

func deduplicatorOrDefault(deduplicator Deduplicator) Deduplicator {
	if deduplicator != nil {
		return deduplicator
	}
	return noopDeduplicator{}
}

func spoolMessage(r io.Reader, maxBytes int64) (*os.File, int64, error) {
	file, err := os.CreateTemp("", "gogomail-smtp-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create smtp spool file: %w", err)
	}

	limited := io.LimitReader(r, maxBytes+1)
	size, copyErr := io.Copy(file, limited)
	if copyErr != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, 0, fmt.Errorf("spool smtp message: %w", copyErr)
	}
	if size > maxBytes {
		_ = file.Close()
		_ = os.Remove(file.Name())
		return nil, size, fmt.Errorf("smtp message exceeds max size %d bytes", maxBytes)
	}
	return file, size, nil
}

func randomMessageID() string {
	var random [8]byte
	if _, err := rand.Read(random[:]); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%d-%s", time.Now().UnixMilli(), hex.EncodeToString(random[:]))
}
