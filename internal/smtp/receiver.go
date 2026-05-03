package smtpd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/emersion/go-sasl"
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

type RateLimiter interface {
	Allow(ctx context.Context, key RateLimitKey) (bool, error)
}

type Backpressure interface {
	Accept(ctx context.Context) (bool, error)
}

type Authenticator interface {
	AuthenticatePlain(ctx context.Context, identity string, username string, password string) error
}

type RateLimitKey struct {
	Stage      Stage
	RemoteAddr string
	Recipient  string
}

type DedupKey struct {
	MessageID string
	Recipient string
}

type ReceivedMessage struct {
	EnvelopeFrom   string
	Mailbox        Mailbox
	StoragePath    string
	Parsed         message.ParsedMessage
	Authentication AuthenticationResults
	ReceivedAt     time.Time
	Size           int64
}

type ReceiverOptions struct {
	Store             storage.Store
	Resolver          RecipientResolver
	Recorder          MessageRecorder
	Deduplicator      Deduplicator
	RateLimiter       RateLimiter
	Backpressure      Backpressure
	AuthVerifier      AuthenticationVerifier
	Authenticator     Authenticator
	RequireAuth       bool
	SupportSMTPUTF8   bool
	SupportRequireTLS bool
	SupportDSN        bool
	SupportBinaryMIME bool
	AddReceivedHeader bool
	ReceivedDomain    string
	Hooks             []Hook
	Policy            ReceivePolicy
	IDGenerator       IDGenerator
	Clock             func() time.Time
	MaxMessageBytes   int64
}

type Receiver struct {
	store             storage.Store
	resolver          RecipientResolver
	recorder          MessageRecorder
	deduplicator      Deduplicator
	rateLimiter       RateLimiter
	backpressure      Backpressure
	authVerifier      AuthenticationVerifier
	authenticator     Authenticator
	requireAuth       bool
	supportSMTPUTF8   bool
	supportRequireTLS bool
	supportDSN        bool
	supportBinaryMIME bool
	addReceivedHeader bool
	receivedDomain    string
	hooks             []Hook
	policy            ReceivePolicy
	idGenerator       IDGenerator
	clock             func() time.Time
}

func NewReceiver(opts ReceiverOptions) *Receiver {
	idGenerator := opts.IDGenerator
	if idGenerator == nil {
		idGenerator = randomMessageID
	}
	return &Receiver{
		store:             opts.Store,
		resolver:          opts.Resolver,
		recorder:          recorderOrDefault(opts.Recorder),
		deduplicator:      deduplicatorOrDefault(opts.Deduplicator),
		rateLimiter:       rateLimiterOrDefault(opts.RateLimiter),
		backpressure:      backpressureOrDefault(opts.Backpressure),
		authVerifier:      opts.AuthVerifier,
		authenticator:     opts.Authenticator,
		requireAuth:       opts.RequireAuth,
		supportSMTPUTF8:   opts.SupportSMTPUTF8,
		supportRequireTLS: opts.SupportRequireTLS,
		supportDSN:        opts.SupportDSN,
		supportBinaryMIME: opts.SupportBinaryMIME,
		addReceivedHeader: opts.AddReceivedHeader,
		receivedDomain:    opts.ReceivedDomain,
		hooks:             append([]Hook(nil), opts.Hooks...),
		policy:            normalizePolicy(opts.Policy, opts.MaxMessageBytes),
		idGenerator:       idGenerator,
		clock:             clockOrDefault(opts.Clock),
	}
}

func (r *Receiver) NewSession(conn *gosmtp.Conn) (gosmtp.Session, error) {
	if r.store == nil {
		return nil, fmt.Errorf("smtp receiver store is required")
	}
	if r.resolver == nil {
		return nil, fmt.Errorf("smtp receiver resolver is required")
	}
	return &session{receiver: r, remoteAddr: remoteAddrFromConn(conn)}, nil
}

type session struct {
	receiver      *Receiver
	from          string
	recipients    []Mailbox
	remoteAddr    string
	authenticated bool
}

func (s *session) Mail(from string, opts *gosmtp.MailOptions) error {
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if err := validateMailOptions(opts, extensionSupport{
		SMTPUTF8:   s.receiver.supportSMTPUTF8,
		RequireTLS: s.receiver.supportRequireTLS,
		DSN:        s.receiver.supportDSN,
		BinaryMIME: s.receiver.supportBinaryMIME,
	}); err != nil {
		return err
	}
	normalized, err := mail.NormalizeAddress(from)
	if err != nil {
		return err
	}
	s.from = normalized
	return nil
}

func (s *session) Rcpt(to string, opts *gosmtp.RcptOptions) error {
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if s.from == "" {
		return fmt.Errorf("mail command is required before rcpt")
	}
	if err := validateRcptOptions(opts, extensionSupport{DSN: s.receiver.supportDSN}); err != nil {
		return err
	}
	if len(s.recipients) >= s.receiver.policy.MaxRecipientsPerMessage {
		return fmt.Errorf("too many recipients; max %d", s.receiver.policy.MaxRecipientsPerMessage)
	}

	allowed, err := s.receiver.rateLimiter.Allow(context.Background(), RateLimitKey{
		Stage:      StageRcpt,
		RemoteAddr: s.remoteAddr,
		Recipient:  to,
	})
	if err != nil {
		return fmt.Errorf("check rcpt rate limit: %w", err)
	}
	if !allowed {
		return fmt.Errorf("rate limit exceeded for recipient %q", to)
	}

	mailbox, err := s.receiver.resolver.ResolveRecipient(context.Background(), to)
	if err != nil {
		return err
	}
	s.recipients = append(s.recipients, mailbox)
	return nil
}

func (s *session) Data(r io.Reader) error {
	if s.receiver.requireAuth && !s.authenticated {
		return gosmtp.ErrAuthRequired
	}
	if s.from == "" {
		return fmt.Errorf("mail command is required before data")
	}
	if len(s.recipients) == 0 {
		return fmt.Errorf("at least one recipient is required before data")
	}
	defer s.Reset()

	accepted, err := s.receiver.backpressure.Accept(context.Background())
	if err != nil {
		return fmt.Errorf("check backpressure: %w", err)
	}
	if !accepted {
		return fmt.Errorf("service temporarily unavailable")
	}
	if err := s.emit(context.Background(), Event{
		Stage:        StageBackpressureChecked,
		EnvelopeFrom: s.from,
	}); err != nil {
		return err
	}

	spooled, size, err := spoolMessage(r, s.receiver.policy.MaxMessageBytes)
	if err != nil {
		return err
	}
	messageID := s.receiver.idGenerator()
	receivedAt := s.receiver.clock()
	if s.receiver.addReceivedHeader {
		prefixed, prefixedSize, err := prependHeaderToSpool(spooled, BuildReceivedHeader(s.remoteAddr, s.receiver.receivedDomain, messageID, receivedAt))
		cleanupSpool(spooled)
		if err != nil {
			return err
		}
		spooled = prefixed
		size = prefixedSize
	}
	defer cleanupSpool(spooled)
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
	parsed, err := message.ParseEMLWithOptions(spooled, message.ParseOptions{SkipTextBody: true})
	if err != nil {
		return fmt.Errorf("parse smtp message: %w", err)
	}
	if parsed.MessageID == "" {
		parsed.MessageID = message.FallbackMessageID(s.from, mailboxAddresses(s.recipients), parsed.Date, parsed.Subject)
		prefixed, prefixedSize, err := insertHeaderAfterTraceHeaders(spooled, "Message-ID: "+parsed.MessageID+"\r\n")
		cleanupSpool(spooled)
		if err != nil {
			return err
		}
		spooled = prefixed
		size = prefixedSize
	}
	if err := s.emit(context.Background(), Event{
		Stage:        StageParsed,
		EnvelopeFrom: s.from,
		Parsed:       parsed,
		Size:         size,
	}); err != nil {
		return err
	}

	authResults, err := s.verifyAuthentication(context.Background(), parsed, size)
	if err != nil {
		return err
	}
	if s.receiver.authVerifier != nil {
		if err := s.emit(context.Background(), Event{
			Stage:          StageAuthenticationChecked,
			EnvelopeFrom:   s.from,
			Recipients:     mailboxAddresses(s.recipients),
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return err
		}
	}

	for _, recipient := range s.recipients {
		shouldProcess, err := s.receiver.deduplicator.CheckAndSet(context.Background(), DedupKey{
			MessageID: parsed.MessageID,
			Recipient: recipient.Address,
		})
		if err != nil {
			return fmt.Errorf("check duplicate message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(context.Background(), Event{
			Stage:          StageDedupChecked,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
			Duplicate:      !shouldProcess,
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
			Stage:          StageStored,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			StoragePath:    path,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return err
		}
		if err := s.receiver.recorder.Record(context.Background(), ReceivedMessage{
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			StoragePath:    path,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return fmt.Errorf("record message for %s: %w", recipient.Address, err)
		}
		if err := s.emit(context.Background(), Event{
			Stage:          StageRecorded,
			EnvelopeFrom:   s.from,
			Mailbox:        recipient,
			StoragePath:    path,
			Parsed:         parsed,
			Authentication: authResults,
			ReceivedAt:     receivedAt,
			Size:           size,
		}); err != nil {
			return err
		}
	}
	return nil
}

func (s *session) verifyAuthentication(ctx context.Context, parsed message.ParsedMessage, size int64) (AuthenticationResults, error) {
	if s.receiver.authVerifier == nil {
		return AuthenticationResults{}, nil
	}
	results, err := s.receiver.authVerifier.VerifyAuthentication(ctx, AuthenticationRequest{
		RemoteAddr:   s.remoteAddr,
		EnvelopeFrom: s.from,
		Recipients:   mailboxAddresses(s.recipients),
		Parsed:       parsed,
		Size:         size,
	})
	if err != nil {
		return AuthenticationResults{}, fmt.Errorf("verify smtp authentication results: %w", err)
	}
	return results, nil
}

func mailboxAddresses(mailboxes []Mailbox) []string {
	addresses := make([]string, 0, len(mailboxes))
	for _, mailbox := range mailboxes {
		addresses = append(addresses, mailbox.Address)
	}
	return addresses
}

func prependHeaderToSpool(spooled *os.File, header string) (*os.File, int64, error) {
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind spooled message for header prepend: %w", err)
	}
	prefixed, err := os.CreateTemp("", "gogomail-spool-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create prefixed spool: %w", err)
	}
	written, err := io.WriteString(prefixed, header)
	if err != nil {
		cleanupSpool(prefixed)
		return nil, 0, fmt.Errorf("write received header: %w", err)
	}
	copied, err := io.Copy(prefixed, spooled)
	if err != nil {
		cleanupSpool(prefixed)
		return nil, 0, fmt.Errorf("copy spooled message after received header: %w", err)
	}
	return prefixed, int64(written) + copied, nil
}

func insertHeaderAfterTraceHeaders(spooled *os.File, header string) (*os.File, int64, error) {
	if _, err := spooled.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind spooled message for header insert: %w", err)
	}
	updated, err := os.CreateTemp("", "gogomail-spool-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create updated spool: %w", err)
	}

	var written int64
	writeString := func(value string) error {
		n, err := io.WriteString(updated, value)
		written += int64(n)
		return err
	}

	reader := bufio.NewReader(spooled)
	inserted := false
	inTrace := false
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			isContinuation := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
			isReceived := strings.HasPrefix(strings.ToLower(line), "received:")
			if !inserted && !isReceived && !(inTrace && isContinuation) {
				if err := writeString(header); err != nil {
					cleanupSpool(updated)
					return nil, 0, fmt.Errorf("write inserted header: %w", err)
				}
				inserted = true
			}
			if err := writeString(line); err != nil {
				cleanupSpool(updated)
				return nil, 0, fmt.Errorf("copy spooled header line: %w", err)
			}
			inTrace = isReceived || (inTrace && isContinuation)
		}
		if err == nil {
			continue
		}
		if err == io.EOF {
			break
		}
		cleanupSpool(updated)
		return nil, 0, fmt.Errorf("read spooled message for header insert: %w", err)
	}
	if !inserted {
		if err := writeString(header); err != nil {
			cleanupSpool(updated)
			return nil, 0, fmt.Errorf("write inserted header: %w", err)
		}
	}
	return updated, written, nil
}

func cleanupSpool(spooled *os.File) {
	if spooled == nil {
		return
	}
	_ = spooled.Close()
	_ = os.Remove(spooled.Name())
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
	s.authenticated = false
	return nil
}

func (s *session) AuthMechanisms() []string {
	if s.receiver.authenticator == nil {
		return nil
	}
	return []string{sasl.Plain}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	if s.receiver.authenticator == nil || !strings.EqualFold(mech, sasl.Plain) {
		return nil, gosmtp.ErrAuthUnsupported
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		if err := s.receiver.authenticator.AuthenticatePlain(context.Background(), identity, username, password); err != nil {
			return gosmtp.ErrAuthFailed
		}
		s.authenticated = true
		return nil
	}), nil
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

type noopRateLimiter struct{}

func (noopRateLimiter) Allow(context.Context, RateLimitKey) (bool, error) {
	return true, nil
}

func rateLimiterOrDefault(rateLimiter RateLimiter) RateLimiter {
	if rateLimiter != nil {
		return rateLimiter
	}
	return noopRateLimiter{}
}

func remoteAddrFromConn(conn *gosmtp.Conn) string {
	if conn == nil || conn.Conn() == nil {
		return ""
	}
	addr := conn.Conn().RemoteAddr()
	if addr == nil {
		return ""
	}
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		return tcpAddr.IP.String()
	}
	return addr.String()
}

type noopBackpressure struct{}

func (noopBackpressure) Accept(context.Context) (bool, error) {
	return true, nil
}

func backpressureOrDefault(backpressure Backpressure) Backpressure {
	if backpressure != nil {
		return backpressure
	}
	return noopBackpressure{}
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
