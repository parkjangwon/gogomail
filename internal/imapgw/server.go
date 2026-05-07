package imapgw

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net"
	stdmail "net/mail"
	"net/textproto"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf16"
	"unicode/utf8"

	messageparse "github.com/gogomail/gogomail/internal/message"
	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

type ServerOptions struct {
	Addr              string
	Backend           Backend
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
	MaxConnections    int
}

type Server struct {
	options  ServerOptions
	mu       sync.Mutex
	listener net.Listener
}

var ErrServerClosed = errors.New("imap server closed")

func NewServer(opts ServerOptions) (*Server, error) {
	addr := strings.TrimSpace(opts.Addr)
	if addr == "" {
		return nil, fmt.Errorf("imap server address is required")
	}
	if strings.ContainsAny(addr, "\r\n") {
		return nil, fmt.Errorf("imap server address cannot contain line breaks")
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return nil, fmt.Errorf("imap server address must be a TCP host:port address: %w", err)
	}
	if opts.Backend == nil {
		return nil, fmt.Errorf("imap backend is required")
	}
	if !opts.AllowInsecureAuth && opts.TLSConfig == nil {
		return nil, fmt.Errorf("imap TLS config is required when insecure auth is disabled")
	}
	if opts.MaxConnections < 0 {
		return nil, fmt.Errorf("imap max connections must not be negative")
	}
	opts.Addr = addr
	return &Server{options: opts}, nil
}

func (s *Server) Options() ServerOptions {
	if s == nil {
		return ServerOptions{}
	}
	return s.options
}

func (s *Server) Serve(listener net.Listener) error {
	if s == nil {
		return fmt.Errorf("imap server is nil")
	}
	if listener == nil {
		return fmt.Errorf("imap listener is required")
	}
	var slots chan struct{}
	if s.options.MaxConnections > 0 {
		slots = make(chan struct{}, s.options.MaxConnections)
	}
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return ErrServerClosed
			}
			return err
		}
		if !acquireIMAPConnectionSlot(slots) {
			rejectIMAPConnectionLimit(conn)
			continue
		}
		go func(conn net.Conn) {
			defer releaseIMAPConnectionSlot(slots)
			_ = s.ServeConn(conn)
		}(conn)
	}
}

func acquireIMAPConnectionSlot(slots chan struct{}) bool {
	if slots == nil {
		return true
	}
	select {
	case slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseIMAPConnectionSlot(slots chan struct{}) {
	if slots == nil {
		return
	}
	<-slots
}

func rejectIMAPConnectionLimit(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "* BYE [ALERT] gogomail IMAP4rev1 server connection limit reached\r\n")
}

func (s *Server) ListenAndServe() error {
	if s == nil {
		return fmt.Errorf("imap server is nil")
	}
	listener, err := s.Listen()
	if err != nil {
		return err
	}
	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()
	defer func() {
		_ = listener.Close()
		s.mu.Lock()
		if s.listener == listener {
			s.listener = nil
		}
		s.mu.Unlock()
	}()
	return s.Serve(listener)
}

func (s *Server) Listen() (net.Listener, error) {
	if s == nil {
		return nil, fmt.Errorf("imap server is nil")
	}
	if s.options.TLSConfig != nil {
		return tls.Listen("tcp", s.options.Addr, s.options.TLSConfig)
	}
	return net.Listen("tcp", s.options.Addr)
}

func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	listener := s.listener
	s.mu.Unlock()
	if listener == nil {
		return nil
	}
	return listener.Close()
}

func (s *Server) ServeConn(conn net.Conn) error {
	if s == nil {
		return fmt.Errorf("imap server is nil")
	}
	if conn == nil {
		return fmt.Errorf("imap connection is required")
	}
	defer conn.Close()
	reader := bufio.NewReaderSize(conn, 8192)
	writer := bufio.NewWriter(conn)
	state := imapConnState{}
	_, state.tlsActive = conn.(*tls.Conn)
	if _, err := writer.WriteString("* OK " + s.capabilityCode(&state) + " gogomail IMAP4rev1 service ready\r\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	defer state.closeSubscription()
	for {
		line, literals, err := s.readCommandLine(reader, writer, &state)
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			var framingErr imapProtocolFramingError
			if errors.As(err, &framingErr) {
				if err := writeIMAPFramingError(writer, framingErr.line, framingErr.message); err != nil {
					return err
				}
				if err := writer.Flush(); err != nil {
					return err
				}
				return nil
			}
			return err
		}
		done, err := s.handleLineWithLiteral(writer, line, literals, &state)
		if err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if state.pendingIdleTag != "" {
			if err := s.serveIdle(reader, writer, &state); err != nil {
				var framingErr imapProtocolFramingError
				if errors.As(err, &framingErr) {
					if err := writeIMAPFramingError(writer, framingErr.line, framingErr.message); err != nil {
						return err
					}
					if err := writer.Flush(); err != nil {
						return err
					}
					return nil
				}
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
		}
		if state.startTLS {
			tlsConn := tls.Server(conn, s.options.TLSConfig)
			if err := tlsConn.Handshake(); err != nil {
				return err
			}
			conn = tlsConn
			reader = bufio.NewReaderSize(conn, 8192)
			writer = bufio.NewWriter(conn)
			state.startTLS = false
			state.tlsActive = true
		}
		if done {
			return nil
		}
	}
}

func (s *Server) readCommandLine(reader *bufio.Reader, writer *bufio.Writer, state *imapConnState) (string, []string, error) {
	line, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
	if err != nil {
		if errors.Is(err, errIMAPCommandLineTooLong) {
			return "", nil, imapProtocolFramingError{message: "command line is too long"}
		}
		return "", nil, err
	}
	if !imapLineHasCRLF(line) {
		if state != nil && state.pendingAuthTag != "" {
			return "", nil, imapProtocolFramingError{line: state.pendingAuthTag + " AUTHENTICATE", message: "command line must end with CRLF"}
		}
		return "", nil, imapProtocolFramingError{line: strings.TrimRight(line, "\n"), message: "command line must end with CRLF"}
	}
	if state != nil && (state.pendingIdleTag != "" || state.pendingAuthTag != "") {
		return line, nil, nil
	}
	var command strings.Builder
	command.WriteString(strings.TrimRight(line, "\r\n"))
	literals := make([]string, 0, 1)
	totalLiteralBytes := 0
	for {
		literalSize, nonSync, ok, err := imapCommandLiteralSize(command.String())
		if err != nil {
			if errors.Is(err, errIMAPCommandLiteralTooLarge) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
			}
			if errors.Is(err, errIMAPCommandLiteralInvalid) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal size is invalid"}
			}
			return command.String(), literals, err
		}
		if !ok {
			return command.String(), literals, nil
		}
		if literalSize > maxIMAPCommandLiteralBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
		}
		if totalLiteralBytes+literalSize > maxIMAPCommandLiteralBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
		}
		if !nonSync {
			if _, err := writer.WriteString("+ Ready for literal data\r\n"); err != nil {
				return "", nil, err
			}
			if err := writer.Flush(); err != nil {
				return "", nil, err
			}
		}
		literal := make([]byte, literalSize)
		if _, err := io.ReadFull(reader, literal); err != nil {
			return "", nil, err
		}
		totalLiteralBytes += literalSize
		literals = append(literals, string(literal))
		suffix, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
		if err != nil {
			if errors.Is(err, errIMAPCommandLineTooLong) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command line is too long"}
			}
			return "", nil, err
		}
		if !imapLineHasCRLF(suffix) {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command line must end with CRLF"}
		}
		if suffix == "\r\n" {
			return command.String(), literals, nil
		}
		if command.Len()+len(suffix) > maxIMAPCommandLineBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command line is too long"}
		}
		command.WriteString(strings.TrimRight(suffix, "\r\n"))
	}
}

var (
	errIMAPCommandLineTooLong     = errors.New("imap command line is too long")
	errIMAPCommandLiteralTooLarge = errors.New("imap command literal is too large")
	errIMAPCommandLiteralInvalid  = errors.New("imap command literal is invalid")
)

type imapProtocolFramingError struct {
	line    string
	message string
}

func (err imapProtocolFramingError) Error() string {
	if err.message == "" {
		return "imap protocol framing error"
	}
	return "imap " + err.message
}

func writeIMAPFramingError(writer *bufio.Writer, line string, message string) error {
	if message == "" {
		message = "protocol framing error"
	}
	if tag := imapTagFromCommandLine(line); tag != "" {
		if _, err := writer.WriteString(tag + " BAD " + message + "\r\n"); err != nil {
			return err
		}
	} else {
		if _, err := writer.WriteString("* BAD " + message + "\r\n"); err != nil {
			return err
		}
	}
	_, err := writer.WriteString("* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n")
	return err
}

func imapTagFromCommandLine(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 || !imapTagValid(fields[0]) {
		return ""
	}
	return fields[0]
}

func readIMAPLine(reader *bufio.Reader, maxBytes int) (string, error) {
	if reader == nil {
		return "", fmt.Errorf("imap reader is required")
	}
	if maxBytes <= 0 {
		return "", fmt.Errorf("imap line limit is invalid")
	}
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxBytes {
			return "", errIMAPCommandLineTooLong
		}
		line = append(line, fragment...)
		if err == nil {
			return string(line), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return "", err
	}
}

func imapLineHasCRLF(line string) bool {
	return strings.HasSuffix(line, "\r\n")
}

type imapConnState struct {
	session               *Session
	selectedMailbox       MailboxID
	selectedMessages      uint32
	selectedHighestModSeq uint64
	selectedNoModSeq      bool
	permanentFlags        map[string]struct{}
	readOnly              bool
	condstoreAware        bool
	savedSearch           []imapSearchSavedMessage
	pendingAuthTag        string
	pendingIdleTag        string
	startTLS              bool
	tlsActive             bool
	events                <-chan MailboxEvent
	cancelEvents          func()
}

func (s *Server) handleLine(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	return s.handleLineWithLiteral(writer, line, nil, state)
}

func (s *Server) handleLineWithLiteral(writer *bufio.Writer, line string, literals []string, state *imapConnState) (bool, error) {
	trimmedLine := strings.TrimRight(line, "\r\n")
	if state.pendingIdleTag != "" {
		return s.handleIdleDone(writer, trimmedLine, state)
	}
	if state.pendingAuthTag != "" {
		return s.handleAuthenticatePlainResponse(writer, trimmedLine, state)
	}
	fields, parseErr := parseIMAPFieldsWithLiteral(trimmedLine, literals)
	if parseErr != nil {
		_, err := writer.WriteString(imapMalformedCommandResponse(trimmedLine))
		return false, err
	}
	if len(fields) < 2 {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	if !imapRawFieldIsAtom(trimmedLine, 0) {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	tag := fields[0]
	if !imapTagValid(tag) {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	if !imapRawFieldIsAtom(trimmedLine, 1) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	if !imapAtomValid(fields[1]) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	command := strings.ToUpper(fields[1])
	if command == "UID" && len(fields) >= 3 && !imapRawFieldIsAtom(trimmedLine, 2) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	if handled, done, err := imapRejectNonAtomAuthenticateArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomSequenceSetArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomStoreControlArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectStringParenthesizedControlListArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomFetchDataItemArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomEnableCapabilityArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if imapCommandShouldDrainSelectedEvents(command) {
		if err := s.drainMailboxEvents(writer, state); err != nil {
			return false, err
		}
	}
	switch command {
	case "CAPABILITY":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD CAPABILITY does not accept arguments\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* CAPABILITY " + strings.Join(s.imapCapabilities(state), " ") + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK CAPABILITY completed\r\n")
		return false, err
	case "ENABLE":
		return s.handleEnable(writer, tag, fields, state)
	case "NOOP":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD NOOP does not accept arguments\r\n")
			return false, err
		}
		if err := s.drainMailboxEvents(writer, state); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK NOOP completed\r\n")
		return false, err
	case "ID":
		if !imapIDArgumentsValidWithLiterals(imapCommandArgumentString(trimmedLine), literals) {
			_, err := writer.WriteString(tag + " BAD ID requires NIL or parameter list\r\n")
			return false, err
		}
		if _, err := writer.WriteString(`* ID ("name" "gogomail")` + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK ID completed\r\n")
		return false, err
	case "STARTTLS":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD STARTTLS does not accept arguments\r\n")
			return false, err
		}
		if state.session != nil {
			_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
			return false, err
		}
		if state.tlsActive || s.options.TLSConfig == nil {
			_, err := writer.WriteString(tag + " BAD STARTTLS is unavailable\r\n")
			return false, err
		}
		state.startTLS = true
		tlsState := *state
		tlsState.startTLS = false
		tlsState.tlsActive = true
		_, err := writer.WriteString(tag + " OK [CAPABILITY " + strings.Join(s.imapCapabilities(&tlsState), " ") + "] Begin TLS negotiation now\r\n")
		return false, err
	case "NAMESPACE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD NAMESPACE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if _, err := writer.WriteString(`* NAMESPACE (("" "/")) NIL NIL` + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK NAMESPACE completed\r\n")
		return false, err
	case "LOGIN":
		if state.session != nil {
			_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
			return false, err
		}
		if len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD LOGIN requires username and password atoms\r\n")
			return false, err
		}
		if !imapLoginCredentialsValid(fields[2], fields[3]) {
			_, err := writer.WriteString(tag + " BAD LOGIN credentials are malformed\r\n")
			return false, err
		}
		if !s.authAllowed(state) {
			_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n")
			return false, err
		}
		authSession, err := s.options.Backend.Authenticate(context.Background(), fields[2], fields[3])
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] LOGIN failed\r\n")
			return false, writeErr
		}
		state.session = &authSession
		_, err = writer.WriteString(tag + " OK " + s.authenticatedCapabilityCode(state) + " LOGIN completed\r\n")
		return false, err
	case "AUTHENTICATE":
		if state.session != nil {
			_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
			return false, err
		}
		if len(fields) != 3 && len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD AUTHENTICATE requires mechanism and optional initial response\r\n")
			return false, err
		}
		if !imapAtomValid(fields[2]) {
			_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is malformed\r\n")
			return false, err
		}
		if !strings.EqualFold(fields[2], "PLAIN") {
			_, err := writer.WriteString(tag + " NO AUTHENTICATE mechanism is unsupported\r\n")
			return false, err
		}
		if len(fields) == 4 {
			if _, _, ok := decodeSASLPlain(fields[3]); !ok {
				_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
				return false, err
			}
		}
		if !s.authAllowed(state) {
			_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n")
			return false, err
		}
		if len(fields) == 4 {
			return s.completeAuthenticatePlain(writer, tag, fields[3], state)
		}
		state.pendingAuthTag = tag
		_, err := writer.WriteString("+ \r\n")
		return false, err
	case "SELECT", "EXAMINE":
		if len(fields) < 3 {
			_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
			return false, err
		}
		condstore, ok := imapSelectCondstore(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
			return false, err
		}
		mailboxName, ok := imapDecodeMailboxName(fields[2])
		if !ok {
			_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is not valid modified UTF-7\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		state.deselectMailbox()
		mailboxState, err := s.options.Backend.SelectMailbox(context.Background(), SelectMailboxRequest{
			UserID:    state.session.UserID,
			MailboxID: MailboxID(mailboxName),
			ReadOnly:  command == "EXAMINE",
		})
		if err != nil {
			if errors.Is(err, ErrMailboxNotFound) {
				_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, command))
				return false, writeErr
			}
			_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
			return false, writeErr
		}
		events, cancel, err := s.options.Backend.Subscribe(context.Background(), state.session.UserID, mailboxState.ID)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
			return false, writeErr
		}
		subscriptionInstalled := false
		defer func() {
			if !subscriptionInstalled && cancel != nil {
				cancel()
			}
		}()
		permanentFlags := imapCanonicalPermanentFlags(mailboxState.PermanentFlags)
		if _, err := writer.WriteString("* FLAGS " + imapFlagList(permanentFlags) + "\r\n"); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", mailboxState.Messages)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d RECENT\r\n", mailboxState.Recent)); err != nil {
			return false, err
		}
		if unseenSequence := s.firstUnseenSequenceNumber(context.Background(), state.session.UserID, mailboxState); unseenSequence > 0 {
			if _, err := writer.WriteString(fmt.Sprintf("* OK [UNSEEN %d] Message %d is first unseen\r\n", unseenSequence, unseenSequence)); err != nil {
				return false, err
			}
		}
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDVALIDITY %d] UIDs valid\r\n", mailboxState.UIDValidity)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDNEXT %d] Predicted next UID\r\n", mailboxState.UIDNext)); err != nil {
			return false, err
		}
		if mailboxState.UIDNotSticky {
			if _, err := writer.WriteString("* OK [UIDNOTSTICKY] UIDs are not sticky\r\n"); err != nil {
				return false, err
			}
		}
		if mailboxState.HighestModSeq > 0 {
			if _, err := writer.WriteString(fmt.Sprintf("* OK [HIGHESTMODSEQ %d] Highest mod-sequence\r\n", mailboxState.HighestModSeq)); err != nil {
				return false, err
			}
		} else if condstore || state.condstoreAware {
			if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
				return false, err
			}
		}
		state.selectedMailbox = mailboxState.ID
		state.selectedMessages = mailboxState.Messages
		state.selectedHighestModSeq = mailboxState.HighestModSeq
		state.selectedNoModSeq = mailboxState.HighestModSeq == 0 && (condstore || state.condstoreAware)
		state.readOnly = command == "EXAMINE"
		if state.readOnly {
			state.permanentFlags = nil
		} else {
			state.permanentFlags = imapPermanentFlagSet(permanentFlags)
		}
		state.savedSearch = nil
		if condstore {
			state.condstoreAware = true
		}
		state.events = events
		state.cancelEvents = cancel
		subscriptionInstalled = true
		if state.readOnly {
			if _, err := writer.WriteString("* OK [PERMANENTFLAGS ()] No permanent flags permitted\r\n"); err != nil {
				return false, err
			}
			_, err = writer.WriteString(tag + " OK [READ-ONLY] EXAMINE completed\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* OK [PERMANENTFLAGS " + imapFlagList(permanentFlags) + "] Permanent flags\r\n"); err != nil {
			return false, err
		}
		_, err = writer.WriteString(tag + " OK [READ-WRITE] SELECT completed\r\n")
		return false, err
	case "LIST":
		return s.handleList(writer, tag, fields, state, false)
	case "LSUB":
		return s.handleList(writer, tag, fields, state, true)
	case "CREATE":
		return s.handleCreate(writer, tag, fields, state)
	case "DELETE":
		return s.handleDeleteMailbox(writer, tag, fields, state)
	case "RENAME":
		return s.handleRenameMailbox(writer, tag, fields, state)
	case "SUBSCRIBE", "UNSUBSCRIBE":
		return s.handleSubscriptionCommand(writer, tag, fields, state, command)
	case "STATUS":
		if len(fields) < 4 {
			_, err := writer.WriteString(tag + " BAD STATUS requires mailbox and status item atoms\r\n")
			return false, err
		}
		if !imapStatusItemListIsParenthesized(fields[3:]) {
			_, err := writer.WriteString(tag + " BAD STATUS requires parenthesized item list\r\n")
			return false, err
		}
		if imapStatusItemListIsEmpty(fields[3:]) {
			_, err := writer.WriteString(tag + " BAD STATUS requires status data items\r\n")
			return false, err
		}
		statusItems, statusErr, ok := imapStatusItems(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD " + statusErr + "\r\n")
			return false, err
		}
		mailboxName, ok := imapDecodeMailboxName(fields[2])
		if !ok {
			_, err := writer.WriteString(tag + " BAD STATUS mailbox name is not valid modified UTF-7\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if imapStatusRequestsItem(statusItems, "HIGHESTMODSEQ") {
			state.condstoreAware = true
		}
		mailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName))
		if err != nil {
			if errors.Is(err, ErrMailboxNotFound) {
				_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "STATUS"))
				return false, writeErr
			}
			_, writeErr := writer.WriteString(tag + " NO STATUS failed\r\n")
			return false, writeErr
		}
		statusName := imapEncodeMailboxName(imapMailboxWireName(imapMailboxDisplayName(mailbox)))
		if _, err := writer.WriteString(fmt.Sprintf("* STATUS %s (%s)\r\n", imapQuotedString(statusName), imapStatusData(mailbox, statusItems))); err != nil {
			return false, err
		}
		_, err = writer.WriteString(tag + " OK STATUS completed\r\n")
		return false, err
	case "UID":
		return s.handleUIDLine(writer, tag, fields, state)
	case "FETCH":
		return s.handleFetch(writer, tag, fields, state)
	case "SEARCH":
		return s.handleSearch(writer, tag, fields, state, false)
	case "ESEARCH":
		_, err := writer.WriteString(tag + " BAD ESEARCH command requires MULTISEARCH capability\r\n")
		return false, err
	case "SORT":
		return s.handleSort(writer, tag, fields, state, false)
	case "THREAD":
		return s.handleThread(writer, tag, fields, state, false)
	case "STORE":
		if len(fields) < 5 {
			return s.handleStore(writer, tag, fields, state)
		}
		return s.handleStore(writer, tag, fields, state)
	case "COPY":
		return s.handleCopy(writer, tag, fields, state)
	case "CHECK":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD CHECK does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " OK CHECK completed\r\n")
		return false, err
	case "IDLE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD IDLE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.pendingIdleTag = tag
		_, err := writer.WriteString("+ idling\r\n")
		return false, err
	case "CLOSE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD CLOSE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		return s.handleClose(writer, tag, state)
	case "UNSELECT":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD UNSELECT does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.deselectMailbox()
		_, err := writer.WriteString(tag + " OK UNSELECT completed\r\n")
		return false, err
	case "EXPUNGE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD EXPUNGE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		if state.readOnly {
			_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
			return false, err
		}
		return s.writeExpungeResponses(writer, tag, state, nil, "EXPUNGE")
	case "MOVE":
		if len(fields) != 4 {
			return s.handleMove(writer, tag, fields, state)
		}
		return s.handleMove(writer, tag, fields, state)
	case "APPEND":
		return s.handleAppend(writer, tag, fields, literals, state)
	case "LOGOUT":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD LOGOUT does not accept arguments\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* BYE gogomail IMAP4rev1 server logging out\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK LOGOUT completed\r\n")
		return true, err
	default:
		_, err := writer.WriteString(tag + " BAD command not implemented\r\n")
		return false, err
	}
}

func imapRejectNonAtomSequenceSetArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	switch command {
	case "SEARCH":
		if imapSearchHasNonAtomSequenceSetArgument(line, fields, 2, 2) {
			_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
			return true, false, err
		}
	case "SORT":
		if criteriaStart := imapSortSearchCriteriaStart(fields, 2); criteriaStart >= 0 {
			if !imapRawFieldIsAtom(line, criteriaStart-1) {
				_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
				return true, false, err
			}
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
				_, err := writer.WriteString(tag + " BAD SORT criteria are unsupported\r\n")
				return true, false, err
			}
		}
	case "THREAD":
		if criteriaStart := imapThreadSearchCriteriaStart(fields, 2); criteriaStart >= 0 {
			if !imapRawFieldIsAtom(line, criteriaStart-1) {
				_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
				return true, false, err
			}
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
				_, err := writer.WriteString(tag + " BAD THREAD criteria are unsupported\r\n")
				return true, false, err
			}
		}
	case "FETCH":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "STORE":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "COPY":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "MOVE":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "UID":
		if len(fields) < 4 || !imapAtomValid(fields[2]) {
			return false, false, nil
		}
		switch strings.ToUpper(fields[2]) {
		case "SEARCH":
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, 3, 3) {
				_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
				return true, false, err
			}
		case "SORT":
			if criteriaStart := imapSortSearchCriteriaStart(fields, 3); criteriaStart >= 0 {
				if !imapRawFieldIsAtom(line, criteriaStart-1) {
					_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
					return true, false, err
				}
				if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
					_, err := writer.WriteString(tag + " BAD SORT criteria are unsupported\r\n")
					return true, false, err
				}
			}
		case "THREAD":
			if criteriaStart := imapThreadSearchCriteriaStart(fields, 3); criteriaStart >= 0 {
				if !imapRawFieldIsAtom(line, criteriaStart-1) {
					_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
					return true, false, err
				}
				if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
					_, err := writer.WriteString(tag + " BAD THREAD criteria are unsupported\r\n")
					return true, false, err
				}
			}
		case "FETCH":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
				return true, false, err
			}
		case "STORE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
				return true, false, err
			}
		case "EXPUNGE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
				return true, false, err
			}
		case "COPY":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
				return true, false, err
			}
		case "MOVE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
				return true, false, err
			}
		}
	}
	return false, false, nil
}

func imapRejectNonAtomAuthenticateArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	if command != "AUTHENTICATE" {
		return false, false, nil
	}
	if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is malformed\r\n")
		return true, false, err
	}
	if len(fields) >= 4 && !imapRawFieldIsAtom(line, 3) {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRejectNonAtomEnableCapabilityArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	if command != "ENABLE" || len(fields) < 3 {
		return false, false, nil
	}
	for i := 2; i < len(fields); i++ {
		if !imapRawFieldIsAtom(line, i) {
			_, err := writer.WriteString(tag + " BAD ENABLE capability is malformed\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapRejectStringParenthesizedControlListArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	switch command {
	case "STORE":
		return imapRejectStringStoreFlagListArgument(writer, tag, line, fields, 3, "STORE")
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "STORE") {
			return imapRejectStringStoreFlagListArgument(writer, tag, line, fields, 4, "UID STORE")
		}
		if len(fields) >= 3 {
			switch strings.ToUpper(fields[2]) {
			case "SEARCH", "SORT", "THREAD":
				subcommand := strings.ToUpper(fields[2])
				if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 3, subcommand); handled {
					return true, done, err
				}
				if subcommand == "SORT" {
					return imapRejectStringSortCriterionListArgument(writer, tag, line, fields, 3)
				}
				if subcommand == "THREAD" {
					return imapRejectStringThreadAlgorithmArgument(writer, tag, line, fields, 3)
				}
			}
		}
	case "APPEND":
		if len(fields) >= 5 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD APPEND options are unsupported\r\n")
			return true, false, err
		}
	case "STATUS":
		if len(fields) >= 4 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD STATUS requires parenthesized item list\r\n")
			return true, false, err
		}
	case "SEARCH":
		return imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "SEARCH")
	case "SORT":
		if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "SORT"); handled {
			return true, done, err
		}
		return imapRejectStringSortCriterionListArgument(writer, tag, line, fields, 2)
	case "THREAD":
		if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "THREAD"); handled {
			return true, done, err
		}
		return imapRejectStringThreadAlgorithmArgument(writer, tag, line, fields, 2)
	case "LIST":
		if len(fields) >= 3 && strings.HasPrefix(fields[2], "(") && imapRawFieldIsStringLike(line, 2) {
			_, err := writer.WriteString(tag + " BAD LIST requires reference and mailbox pattern atoms\r\n")
			return true, false, err
		}
		for i := 4; i+1 < len(fields); i++ {
			if strings.EqualFold(fields[i], "RETURN") {
				if imapRawFieldIsStringLike(line, i) {
					_, err := writer.WriteString(tag + " BAD LIST requires return options atom\r\n")
					return true, false, err
				}
				if strings.HasPrefix(fields[i+1], "(") && imapRawFieldIsStringLike(line, i+1) {
					_, err := writer.WriteString(tag + " BAD LIST requires parenthesized return options\r\n")
					return true, false, err
				}
			}
		}
	case "SELECT", "EXAMINE":
		if len(fields) >= 4 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapRejectStringSortCriterionListArgument(writer *bufio.Writer, tag string, line string, fields []string, argumentStart int) (bool, bool, error) {
	criterionRawIndex := imapSortCriterionRawIndex(fields, argumentStart)
	if criterionRawIndex < 0 || !strings.HasPrefix(fields[criterionRawIndex], "(") || !imapRawFieldIsStringLike(line, criterionRawIndex) {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD SORT requires parenthesized sort criteria\r\n")
	return true, false, err
}

func imapSortCriterionRawIndex(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+2 {
			return -1
		}
		return argumentStart + 2
	}
	return argumentStart
}

func imapRejectStringThreadAlgorithmArgument(writer *bufio.Writer, tag string, line string, fields []string, argumentStart int) (bool, bool, error) {
	algorithmRawIndex := imapThreadAlgorithmRawIndex(fields, argumentStart)
	if algorithmRawIndex < 0 || !imapRawFieldIsStringLike(line, algorithmRawIndex) {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
	return true, false, err
}

func imapThreadAlgorithmRawIndex(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+2 {
			return -1
		}
		return argumentStart + 2
	}
	return argumentStart
}

func imapRejectStringSearchReturnControlArgument(writer *bufio.Writer, tag string, line string, fields []string, returnRawIndex int, commandName string) (bool, bool, error) {
	if len(fields) <= returnRawIndex || !strings.EqualFold(fields[returnRawIndex], "RETURN") {
		return false, false, nil
	}
	if imapRawFieldIsStringLike(line, returnRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " requires return options atom\r\n")
		return true, false, err
	}
	if len(fields) > returnRawIndex+1 && strings.HasPrefix(fields[returnRawIndex+1], "(") && imapRawFieldIsStringLike(line, returnRawIndex+1) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " requires parenthesized return options\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRejectStringStoreFlagListArgument(writer *bufio.Writer, tag string, line string, fields []string, storeStart int, commandName string) (bool, bool, error) {
	if len(fields) <= storeStart {
		return false, false, nil
	}
	modeRawIndex := storeStart
	if imapStoreUnchangedSincePresent(fields[storeStart:]) {
		modeRawIndex++
	}
	flagRawIndex := modeRawIndex + 1
	if imapRawFieldIsStringLike(line, flagRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " flags are unsupported\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRawFieldIsStringLike(line string, fieldIndex int) bool {
	kind, ok := imapRawFieldKind(line, fieldIndex)
	return ok && (kind == imapRawFieldQuoted || kind == imapRawFieldLiteral)
}

func imapRejectNonAtomFetchDataItemArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	dataStart := -1
	switch command {
	case "FETCH":
		dataStart = 3
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "FETCH") {
			dataStart = 4
		}
	}
	if dataStart < 0 || len(fields) <= dataStart {
		return false, false, nil
	}
	kind, ok := imapRawFieldKind(line, dataStart)
	if !ok || (kind != imapRawFieldQuoted && kind != imapRawFieldLiteral) {
		return false, false, nil
	}
	if _, hasSyntaxError := imapFetchDataItemsSyntaxError(fields[dataStart:]); hasSyntaxError {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD FETCH data item is unsupported\r\n")
	return true, false, err
}

func imapRejectNonAtomStoreControlArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	commandName := ""
	storeStart := -1
	switch command {
	case "STORE":
		commandName = "STORE"
		storeStart = 3
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "STORE") {
			commandName = "UID STORE"
			storeStart = 4
		}
	}
	if storeStart < 0 || len(fields) <= storeStart {
		return false, false, nil
	}
	modeRawIndex := storeStart
	if imapStoreUnchangedSincePresent(fields[storeStart:]) {
		kind, ok := imapRawFieldKind(line, storeStart)
		if !ok || kind == imapRawFieldQuoted || kind == imapRawFieldLiteral {
			_, err := writer.WriteString(tag + " BAD " + commandName + " UNCHANGEDSINCE modifier is invalid\r\n")
			return true, false, err
		}
		modeRawIndex++
	}
	if !imapRawFieldIsAtom(line, modeRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " mode is unsupported\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapSortSearchCriteriaStart(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+4 {
			return -1
		}
		return argumentStart + 4
	}
	if len(fields) <= argumentStart+2 {
		return -1
	}
	return argumentStart + 2
}

func imapThreadSearchCriteriaStart(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+4 {
			return -1
		}
		return argumentStart + 4
	}
	if len(fields) <= argumentStart+2 {
		return -1
	}
	return argumentStart + 2
}

func imapSearchHasNonAtomSequenceSetArgument(line string, fields []string, criteriaStart int, rawCriteriaStart int) bool {
	for i := criteriaStart; i < len(fields); i++ {
		field := fields[i]
		if !imapSearchFieldRequiresAtomSet(fields, i, criteriaStart) {
			continue
		}
		if imapSearchCriterionLooksLikeSequenceSet(field) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomNumeric(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomDate(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomCharset(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomKeyword(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
	}
	return false
}

func imapSearchFieldRequiresAtomSet(fields []string, index int, criteriaStart int) bool {
	if index > criteriaStart {
		switch strings.ToUpper(fields[index-1]) {
		case "FROM", "TO", "CC", "BCC", "SUBJECT", "BODY", "TEXT":
			return false
		}
	}
	if index > criteriaStart+1 && strings.EqualFold(fields[index-2], "HEADER") {
		return false
	}
	return true
}

func imapSearchFieldRequiresAtomNumeric(fields []string, index int, criteriaStart int) bool {
	if index > criteriaStart {
		switch strings.ToUpper(fields[index-1]) {
		case "LARGER", "SMALLER":
			return true
		case "MODSEQ":
			return imapDecimalToken(fields[index])
		}
	}
	if index > criteriaStart+1 && strings.EqualFold(fields[index-2], "MODSEQ") {
		return true
	}
	if index > criteriaStart+2 && strings.EqualFold(fields[index-3], "MODSEQ") {
		return true
	}
	return false
}

func imapSearchFieldRequiresAtomDate(fields []string, index int, criteriaStart int) bool {
	if index <= criteriaStart {
		return false
	}
	switch strings.ToUpper(fields[index-1]) {
	case "SINCE", "BEFORE", "ON", "SENTSINCE", "SENTBEFORE", "SENTON":
		return true
	default:
		return false
	}
}

func imapSearchFieldRequiresAtomCharset(fields []string, index int, criteriaStart int) bool {
	return index > criteriaStart && strings.EqualFold(fields[index-1], "CHARSET")
}

func imapSearchFieldRequiresAtomKeyword(fields []string, index int, criteriaStart int) bool {
	if index <= criteriaStart {
		return false
	}
	switch strings.ToUpper(fields[index-1]) {
	case "KEYWORD", "UNKEYWORD":
		return true
	default:
		return false
	}
}

func imapDecimalToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func imapMalformedCommandResponse(line string) string {
	if tag := imapTagFromCommandLine(line); tag != "" {
		return tag + " BAD malformed command\r\n"
	}
	return "* BAD malformed command\r\n"
}

func (s *Server) handleList(writer *bufio.Writer, tag string, fields []string, state *imapConnState, subscribed bool) (bool, error) {
	command := "LIST"
	if subscribed {
		command = "LSUB"
	}
	var listFields []string
	if len(fields) > 2 {
		listFields = fields[2:]
	}
	listOptions, listError, ok := imapListCommandOptions(listFields, subscribed)
	if !ok {
		if listError != "" {
			_, err := writer.WriteString(tag + " BAD " + listError + "\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " BAD " + command + " requires reference and mailbox pattern atoms\r\n")
		return false, err
	}
	if len(listOptions.fields) < 2 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires reference and mailbox pattern atoms\r\n")
		return false, err
	}
	patterns, patternOK := imapListPatterns(listOptions.fields)
	if !patternOK {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapStatusRequestsItem(listOptions.statusItems, "HIGHESTMODSEQ") {
		state.condstoreAware = true
	}
	if len(patterns) == 1 && patterns[0] == "" {
		if listOptions.specialUseOnly {
			_, err := writer.WriteString(tag + " OK " + command + " completed\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ""` + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK " + command + " completed\r\n")
		return false, err
	}
	if subscribed || listOptions.subscribedOnly {
		return s.writeSubscribedListResponses(writer, tag, state, patterns, command, listOptions)
	}
	matcher, ok := imapMailboxPatternMatcherAny(patterns)
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is invalid\r\n")
		return false, err
	}
	mailboxes, err := s.options.Backend.ListMailboxes(context.Background(), ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	subscribedNames := map[string]struct{}{}
	if listOptions.subscribedReturn {
		var err error
		subscribedNames, err = s.subscribedMailboxWireNames(context.Background(), state)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
			return false, writeErr
		}
	}
	children := imapMailboxChildren(mailboxes)
	seen := make(map[string]struct{}, len(mailboxes))
	if imapMailboxPatternListContainsRoot(patterns) && !listOptions.specialUseOnly {
		if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ""` + "\r\n"); err != nil {
			return false, err
		}
	}
	for _, mailbox := range mailboxes {
		displayName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		if !matcher(displayName) {
			continue
		}
		key := strings.ToLower(displayName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		wireDisplayName := imapEncodeMailboxName(displayName)
		attributes := imapMailboxListAttributes(mailbox, children[mailbox.ID])
		if _, ok := subscribedNames[strings.ToLower(displayName)]; ok {
			attributes = append(attributes, `\Subscribed`)
		}
		if listOptions.specialUseOnly && len(attributes) == 1 {
			continue
		}
		if _, err := writer.WriteString("* " + command + " " + imapFlagList(attributes) + ` "/" ` + imapQuotedString(wireDisplayName) + "\r\n"); err != nil {
			return false, err
		}
		if len(listOptions.statusItems) > 0 {
			if _, err := writer.WriteString(fmt.Sprintf("* STATUS %s (%s)\r\n", imapQuotedString(wireDisplayName), imapStatusData(mailbox, listOptions.statusItems))); err != nil {
				return false, err
			}
		}
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) writeSubscribedListResponses(writer *bufio.Writer, tag string, state *imapConnState, patterns []string, command string, listOptions imapListOptions) (bool, error) {
	subscriptions, err := s.options.Backend.ListSubscribedMailboxes(context.Background(), ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	mailboxes := make([]Mailbox, 0, len(subscriptions))
	for _, subscription := range subscriptions {
		if subscription.Exists {
			mailboxes = append(mailboxes, subscription.Mailbox)
		}
	}
	children := imapMailboxChildren(mailboxes)
	seen := make(map[string]struct{}, len(subscriptions))
	matcher, ok := imapMailboxPatternMatcherAny(patterns)
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox pattern is invalid\r\n")
		return false, err
	}
	for _, subscription := range subscriptions {
		displayName := imapMailboxWireName(subscription.Name)
		if subscription.Exists {
			displayName = imapMailboxWireName(imapMailboxDisplayName(subscription.Mailbox))
		}
		if !matcher(displayName) {
			parentName := imapLSubParentNameAny(displayName, patterns)
			if parentName == "" {
				continue
			}
			key := strings.ToLower(parentName)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ` + imapQuotedString(imapEncodeMailboxName(parentName)) + "\r\n"); err != nil {
				return false, err
			}
			continue
		}
		key := strings.ToLower(displayName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		attributes := []string{`\Noselect`}
		if subscription.Exists {
			attributes = imapMailboxListAttributes(subscription.Mailbox, children[subscription.Mailbox.ID])
		}
		if listOptions.subscribedReturn {
			attributes = append(attributes, `\Subscribed`)
		}
		if _, err := writer.WriteString("* " + command + " " + imapFlagList(attributes) + ` "/" ` + imapQuotedString(imapEncodeMailboxName(displayName)) + "\r\n"); err != nil {
			return false, err
		}
		if subscription.Exists && len(listOptions.statusItems) > 0 {
			if _, err := writer.WriteString(fmt.Sprintf("* STATUS %s (%s)\r\n", imapQuotedString(imapEncodeMailboxName(displayName)), imapStatusData(subscription.Mailbox, listOptions.statusItems))); err != nil {
				return false, err
			}
		}
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) subscribedMailboxWireNames(ctx context.Context, state *imapConnState) (map[string]struct{}, error) {
	subscriptions, err := s.options.Backend.ListSubscribedMailboxes(ctx, ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		return nil, err
	}
	names := make(map[string]struct{}, len(subscriptions))
	for _, subscription := range subscriptions {
		displayName := imapMailboxWireName(subscription.Name)
		if subscription.Exists {
			displayName = imapMailboxWireName(imapMailboxDisplayName(subscription.Mailbox))
		}
		if displayName == "" {
			continue
		}
		names[strings.ToLower(displayName)] = struct{}{}
	}
	return names, nil
}

func imapLSubParentNameAny(name string, patterns []string) string {
	for _, pattern := range patterns {
		matcher, ok := imapMailboxPatternMatcher(pattern)
		if !ok {
			continue
		}
		if parentName := imapLSubParentName(name, pattern, matcher); parentName != "" {
			return parentName
		}
	}
	return ""
}

func imapLSubParentName(name string, pattern string, matcher func(string) bool) string {
	if !strings.Contains(pattern, "%") || !strings.Contains(name, "/") {
		return ""
	}
	parts := strings.Split(name, "/")
	for i := 1; i < len(parts); i++ {
		parent := strings.Join(parts[:i], "/")
		if matcher(parent) {
			return parent
		}
	}
	return ""
}

func (s *Server) handleCreate(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD CREATE requires mailbox name\r\n")
		return false, err
	}
	mailboxName, ok := imapDecodeMailboxName(fields[2])
	if !ok {
		_, err := writer.WriteString(tag + " BAD CREATE mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(mailboxName) {
		_, err := writer.WriteString(tag + " NO CREATE cannot create INBOX\r\n")
		return false, err
	}
	if _, err := s.options.Backend.CreateMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName)); err != nil {
		_, writeErr := writer.WriteString(tag + " NO CREATE failed\r\n")
		return false, writeErr
	}
	_, err := writer.WriteString(tag + " OK CREATE completed\r\n")
	return false, err
}

func (s *Server) handleDeleteMailbox(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD DELETE requires mailbox name\r\n")
		return false, err
	}
	mailboxName, ok := imapDecodeMailboxName(fields[2])
	if !ok {
		_, err := writer.WriteString(tag + " BAD DELETE mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(mailboxName) {
		_, err := writer.WriteString(tag + " NO DELETE cannot delete INBOX\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "DELETE"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO DELETE failed\r\n")
		return false, writeErr
	}
	if err := s.options.Backend.DeleteMailbox(context.Background(), state.session.UserID, mailbox.ID); err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "DELETE"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO DELETE failed\r\n")
		return false, writeErr
	}
	if state.selectedMailbox == mailbox.ID {
		state.deselectMailbox()
	}
	_, err = writer.WriteString(tag + " OK DELETE completed\r\n")
	return false, err
}

func (s *Server) handleRenameMailbox(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD RENAME requires source and destination mailbox names\r\n")
		return false, err
	}
	sourceName, sourceOK := imapDecodeMailboxName(fields[2])
	destName, destOK := imapDecodeMailboxName(fields[3])
	if !sourceOK || !destOK {
		_, err := writer.WriteString(tag + " BAD RENAME mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(sourceName) {
		_, err := writer.WriteString(tag + " NO RENAME INBOX special semantics are not supported\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(destName) {
		_, err := writer.WriteString(tag + " NO RENAME cannot rename to INBOX\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(sourceName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "RENAME"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO RENAME failed\r\n")
		return false, writeErr
	}
	renamed, err := s.options.Backend.RenameMailbox(context.Background(), state.session.UserID, mailbox.ID, MailboxID(destName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "RENAME"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO RENAME failed\r\n")
		return false, writeErr
	}
	if state.selectedMailbox == mailbox.ID && renamed.ID != "" && renamed.ID != state.selectedMailbox {
		state.closeSubscription()
		state.selectedMailbox = renamed.ID
		state.selectedHighestModSeq = renamed.HighestModSeq
		state.selectedNoModSeq = renamed.HighestModSeq == 0 && state.condstoreAware
		if events, cancel, err := s.options.Backend.Subscribe(context.Background(), state.session.UserID, renamed.ID); err == nil {
			state.events = events
			state.cancelEvents = cancel
		}
	}
	_, err = writer.WriteString(tag + " OK RENAME completed\r\n")
	return false, err
}

func imapMailboxNameIsINBOX(name string) bool {
	return strings.EqualFold(strings.TrimSpace(name), "INBOX")
}

func imapMailboxNotFoundResponse(tag string, command string) string {
	return tag + " NO [NONEXISTENT] " + command + " mailbox does not exist\r\n"
}

func (s *Server) handleSubscriptionCommand(writer *bufio.Writer, tag string, fields []string, state *imapConnState, command string) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom\r\n")
		return false, err
	}
	var err error
	mailboxName, ok := imapDecodeMailboxName(fields[2])
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if command == "SUBSCRIBE" {
		_, err = s.options.Backend.SubscribeMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName))
	} else {
		err = s.options.Backend.UnsubscribeMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName))
	}
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) handleIdleDone(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingIdleTag
	if !strings.EqualFold(line, "DONE") {
		_, err := writer.WriteString(tag + " BAD IDLE terminated by unexpected command\r\n")
		state.pendingIdleTag = ""
		return false, err
	}
	state.pendingIdleTag = ""
	if err := s.drainMailboxEvents(writer, state); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK IDLE completed\r\n")
	return false, err
}

type idleLineResult struct {
	line string
	err  error
}

func (s *Server) serveIdle(reader *bufio.Reader, writer *bufio.Writer, state *imapConnState) error {
	lineCh := make(chan idleLineResult, 1)
	go func() {
		line, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
		lineCh <- idleLineResult{line: line, err: err}
	}()
	for state.pendingIdleTag != "" {
		select {
		case result := <-lineCh:
			if result.err != nil {
				if errors.Is(result.err, io.EOF) {
					return nil
				}
				if errors.Is(result.err, errIMAPCommandLineTooLong) {
					return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line is too long"}
				}
				return result.err
			}
			if len(result.line) > 8192 {
				return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line is too long"}
			}
			if !imapLineHasCRLF(result.line) {
				return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line must end with CRLF"}
			}
			_, err := s.handleIdleDone(writer, strings.TrimRight(result.line, "\r\n"), state)
			return err
		case event, ok := <-state.events:
			if !ok {
				state.events = nil
				state.cancelEvents = nil
				continue
			}
			if event.UserID != state.session.UserID || event.MailboxID != state.selectedMailbox {
				continue
			}
			if err := s.writeMailboxEvent(writer, state, event); err != nil {
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) drainMailboxEvents(writer *bufio.Writer, state *imapConnState) error {
	if state == nil || state.events == nil || state.session == nil || state.selectedMailbox == "" {
		return nil
	}
	for {
		select {
		case event, ok := <-state.events:
			if !ok {
				state.events = nil
				state.cancelEvents = nil
				return nil
			}
			if event.UserID != state.session.UserID || event.MailboxID != state.selectedMailbox {
				continue
			}
			if err := s.writeMailboxEvent(writer, state, event); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (s *Server) firstUnseenSequenceNumber(ctx context.Context, userID UserID, mailbox MailboxState) uint32 {
	if s == nil || s.options.Backend == nil || mailbox.Unseen == 0 || mailbox.Messages == 0 {
		return 0
	}
	messages, err := s.options.Backend.ListMessages(ctx, ListMessagesRequest{
		UserID:    userID,
		MailboxID: mailbox.ID,
		Limit:     int(mailbox.Messages),
	})
	if err != nil {
		return 0
	}
	for i, summary := range messages {
		if summary.Flags.Read {
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		return sequenceNumber
	}
	return 0
}

func (s *Server) writeMailboxEvent(writer *bufio.Writer, state *imapConnState, event MailboxEvent) error {
	switch event.Type {
	case MailboxEventExists:
		if event.Messages > 0 {
			if event.Messages <= state.selectedMessages {
				return nil
			}
			state.selectedMessages = event.Messages
		} else {
			state.selectedMessages++
		}
		_, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", state.selectedMessages))
		return err
	case MailboxEventExpunge:
		sequenceNumber := event.SequenceNumber
		if sequenceNumber == 0 {
			return nil
		}
		if state.selectedMessages > 0 && sequenceNumber > state.selectedMessages {
			sequenceNumber = state.selectedMessages
		}
		if sequenceNumber == 0 {
			return nil
		}
		if state.selectedMessages > 0 {
			state.selectedMessages--
		}
		state.removeExpungedFromSavedSearch([]MessageSummary{{SequenceNumber: sequenceNumber}})
		_, err := writer.WriteString(fmt.Sprintf("* %d EXPUNGE\r\n", sequenceNumber))
		return err
	case MailboxEventFlags:
		message, err := s.options.Backend.FetchMessage(context.Background(), FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       event.UID,
		})
		if err != nil {
			return err
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		state.observeHighestModSeq(message.Summary.ModSeq)
		sequenceNumber, ok := imapSequenceNumber(message.Summary)
		if !ok {
			return fmt.Errorf("imap event sequence number is unavailable")
		}
		attributes := []string{
			fmt.Sprintf("UID %d", message.Summary.UID),
			"FLAGS " + imapFlagList(message.Summary.Flags.IMAPFlags()),
		}
		if state.condstoreAware {
			attributes = append(attributes, fmt.Sprintf("MODSEQ (%d)", message.Summary.ModSeq))
		}
		_, err = writer.WriteString(fmt.Sprintf("* %d FETCH (%s)\r\n", sequenceNumber, strings.Join(attributes, " ")))
		return err
	default:
		return nil
	}
}

func (state *imapConnState) closeSubscription() {
	if state == nil || state.cancelEvents == nil {
		return
	}
	state.cancelEvents()
	state.cancelEvents = nil
	state.events = nil
}

func (state *imapConnState) deselectMailbox() {
	if state == nil {
		return
	}
	state.selectedMailbox = ""
	state.selectedMessages = 0
	state.selectedHighestModSeq = 0
	state.selectedNoModSeq = false
	state.permanentFlags = nil
	state.readOnly = false
	state.savedSearch = nil
	state.closeSubscription()
}

func imapCommandShouldDrainSelectedEvents(command string) bool {
	switch strings.ToUpper(command) {
	case "FETCH", "STORE", "COPY", "MOVE", "SEARCH", "SORT", "THREAD", "CHECK", "CLOSE", "UNSELECT", "EXPUNGE", "UID", "APPEND":
		return true
	default:
		return false
	}
}

func (s *Server) handleAuthenticatePlainResponse(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingAuthTag
	state.pendingAuthTag = ""
	if line == "*" {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE canceled\r\n")
		return false, err
	}
	return s.completeAuthenticatePlain(writer, tag, line, state)
}

func (s *Server) completeAuthenticatePlain(writer *bufio.Writer, tag string, value string, state *imapConnState) (bool, error) {
	username, password, ok := decodeSASLPlain(value)
	if !ok {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
		return false, err
	}
	authSession, err := s.options.Backend.Authenticate(context.Background(), username, password)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] AUTHENTICATE failed\r\n")
		return false, writeErr
	}
	state.session = &authSession
	_, err = writer.WriteString(tag + " OK " + s.authenticatedCapabilityCode(state) + " AUTHENTICATE completed\r\n")
	return false, err
}

func decodeSASLPlain(value string) (string, string, bool) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", "", false
	}
	if len(value) > maxIMAPSASLPlainEncodedBytes {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", "", false
	}
	if len(decoded) > maxIMAPSASLPlainDecodedBytes {
		return "", "", false
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 || parts[1] == "" {
		return "", "", false
	}
	if parts[0] != "" && parts[0] != parts[1] {
		return "", "", false
	}
	if !imapAuthCredentialsValid(parts[1], parts[2]) {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func imapAuthCredentialsValid(username string, password string) bool {
	return imapAuthCredentialsValidWithEmptyPassword(username, password, false)
}

func imapLoginCredentialsValid(username string, password string) bool {
	return imapAuthCredentialsValidWithEmptyPassword(username, password, true)
}

func imapAuthCredentialsValidWithEmptyPassword(username string, password string, allowEmptyPassword bool) bool {
	if strings.ContainsAny(username, "\r\n") || strings.ContainsAny(password, "\r\n") {
		return false
	}
	username = strings.TrimSpace(username)
	if username == "" || (!allowEmptyPassword && password == "") || len(username) > maxIMAPAuthIdentityBytes || len(password) > maxIMAPAuthPasswordBytes {
		return false
	}
	return true
}

func (s *Server) handleUIDLine(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD UID requires subcommand\r\n")
		return false, err
	}
	if !imapAtomValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	subcommand := strings.ToUpper(fields[2])
	if subcommand == "ESEARCH" {
		_, err := writer.WriteString(tag + " BAD ESEARCH command requires MULTISEARCH capability\r\n")
		return false, err
	}
	if !imapUIDSubcommandKnown(subcommand) {
		_, err := writer.WriteString(tag + " BAD UID command not implemented\r\n")
		return false, err
	}
	if handled, done, err := s.validateUIDSubcommandSyntax(writer, tag, fields, subcommand); handled {
		return done, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if err := s.drainMailboxEvents(writer, state); err != nil {
		return false, err
	}
	switch subcommand {
	case "FETCH":
		return s.handleUIDFetch(writer, tag, fields, state)
	case "SEARCH":
		return s.handleSearch(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "SORT":
		return s.handleSort(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "THREAD":
		return s.handleThread(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "STORE":
		if len(fields) < 6 {
			return s.handleUIDStore(writer, tag, fields, state)
		}
		return s.handleUIDStore(writer, tag, fields, state)
	case "EXPUNGE":
		if len(fields) != 4 {
			return s.handleUIDExpunge(writer, tag, fields, state)
		}
		return s.handleUIDExpunge(writer, tag, fields, state)
	case "COPY":
		return s.handleUIDCopy(writer, tag, fields, state)
	case "MOVE":
		if len(fields) != 5 {
			return s.handleUIDMove(writer, tag, fields, state)
		}
		return s.handleUIDMove(writer, tag, fields, state)
	default:
		_, err := writer.WriteString(tag + " BAD UID command not implemented\r\n")
		return false, err
	}
}

func (s *Server) validateUIDSubcommandSyntax(writer *bufio.Writer, tag string, fields []string, subcommand string) (bool, bool, error) {
	switch subcommand {
	case "FETCH":
		if len(fields) < 5 {
			_, err := writer.WriteString(tag + " BAD UID FETCH requires UID set and data items\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
			return true, false, err
		}
		if message, ok := imapFetchDataItemsSyntaxError(fields[4:]); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "SEARCH":
		if len(fields) < 4 {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		_, searchFields, ok := imapSearchReturnOptions(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD SEARCH return options are unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		if !imapSearchFieldsContainCriteria(searchFields) {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "SORT":
		_, sortFields, ok := imapSearchSaveReturnOption(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD SORT return options are unsupported\r\n")
			return true, false, err
		}
		if len(sortFields) < 3 {
			_, err := writer.WriteString(tag + " BAD SORT requires sort criteria, charset, and search criteria\r\n")
			return true, false, err
		}
		_, searchFields, charsetOK, ok := imapSortCommandArguments(sortFields)
		if !ok {
			_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD SORT requires search criteria\r\n")
			return true, false, err
		}
		if charsetOK {
			if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
				_, err := writer.WriteString(tag + " BAD SORT " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
				return true, false, err
			}
		}
	case "THREAD":
		_, threadFields, ok := imapSearchSaveReturnOption(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD THREAD return options are unsupported\r\n")
			return true, false, err
		}
		if len(threadFields) < 3 {
			_, err := writer.WriteString(tag + " BAD THREAD requires algorithm, charset, and search criteria\r\n")
			return true, false, err
		}
		algorithm, searchFields, charsetOK, ok := imapThreadCommandArguments(threadFields)
		if !ok {
			_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
			return true, false, err
		}
		if !imapThreadAlgorithmIsSupported(algorithm) {
			_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD THREAD requires search criteria\r\n")
			return true, false, err
		}
		if charsetOK {
			if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
				_, err := writer.WriteString(tag + " BAD THREAD " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
				return true, false, err
			}
		}
	case "STORE":
		if len(fields) < 6 {
			_, err := writer.WriteString(tag + " BAD UID STORE requires UID, mode, and flags\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
			return true, false, err
		}
		if message, ok := imapStoreArgumentsSyntaxError("UID STORE", fields[4:]); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "EXPUNGE":
		if len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires UID set\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
			return true, false, err
		}
	case "COPY":
		if len(fields) != 5 {
			_, err := writer.WriteString(tag + " BAD UID COPY requires UID set and destination mailbox\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
			return true, false, err
		}
		if _, ok := imapDecodeMailboxName(fields[4]); !ok {
			_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n")
			return true, false, err
		}
	case "MOVE":
		if len(fields) != 5 {
			_, err := writer.WriteString(tag + " BAD UID MOVE requires UID set and destination mailbox\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
			return true, false, err
		}
		if _, ok := imapDecodeMailboxName(fields[4]); !ok {
			_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapUIDSubcommandKnown(subcommand string) bool {
	switch subcommand {
	case "FETCH", "SEARCH", "SORT", "THREAD", "STORE", "EXPUNGE", "COPY", "MOVE":
		return true
	default:
		return false
	}
}

func (s *Server) handleSearch(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	returnOptions, searchFields, ok := imapSearchReturnOptions(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD SEARCH return options are unsupported\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if !imapSearchFieldsContainCriteria(searchFields) {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	criteria, charsetOK := imapSearchCriteria(searchFields)
	if !charsetOK {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] SEARCH charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(criteria) == 0 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	requestsModSeq := imapSearchRequestsModSeq(criteria)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "SEARCH")
		}
		state.condstoreAware = true
	}
	results, highestModSeq, ok, err := s.imapSearchResults(context.Background(), state, criteria, messages, uidMode, requestsModSeq)
	if err != nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
		return false, err
	}
	if returnOptions.save {
		state.savedSearch = imapSavedSearchResults(results, returnOptions.options)
	}
	if returnOptions.extended && len(returnOptions.options) > 0 {
		if _, err := writer.WriteString(imapESearchResponse(tag, results, uidMode, returnOptions.options, highestModSeq, requestsModSeq) + "\r\n"); err != nil {
			return false, err
		}
	} else if !returnOptions.save {
		if _, err := writer.WriteString("* SEARCH" + imapSearchResultSuffix(imapSearchResultValues(results), highestModSeq, requestsModSeq) + "\r\n"); err != nil {
			return false, err
		}
	}
	completion := "SEARCH"
	if uidMode {
		completion = "UID SEARCH"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

func (s *Server) handleSort(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	save, sortFields, ok := imapSearchSaveReturnOption(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT return options are unsupported\r\n")
		return false, err
	}
	if len(sortFields) < 3 {
		_, err := writer.WriteString(tag + " BAD SORT requires sort criteria, charset, and search criteria\r\n")
		return false, err
	}
	sortCriteria, searchFields, charsetOK, ok := imapSortCommandArguments(sortFields)
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
		return false, err
	}
	if charsetOK {
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD SORT " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
			return false, err
		}
	}
	if !charsetOK {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] SORT charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD SORT requires search criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SORT failed\r\n")
		return false, writeErr
	}
	requestsModSeq := imapSearchRequestsModSeq(searchFields)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "SORT")
		}
		state.condstoreAware = true
	}
	results, _, ok, err := s.imapSearchResults(context.Background(), state, searchFields, messages, uidMode, false)
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SORT failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT search criteria are unsupported\r\n")
		return false, err
	}
	imapSortMatches(results, sortCriteria)
	if save {
		state.savedSearch = imapSavedSearchMatches(results)
	}
	if _, err := writer.WriteString("* SORT" + imapSearchResultSuffix(imapSearchResultValues(results), 0, false) + "\r\n"); err != nil {
		return false, err
	}
	completion := "SORT"
	if uidMode {
		completion = "UID SORT"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

func (s *Server) handleThread(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	save, threadFields, ok := imapSearchSaveReturnOption(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD return options are unsupported\r\n")
		return false, err
	}
	if len(threadFields) < 3 {
		_, err := writer.WriteString(tag + " BAD THREAD requires algorithm, charset, and search criteria\r\n")
		return false, err
	}
	algorithm, searchFields, charsetOK, ok := imapThreadCommandArguments(threadFields)
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
		return false, err
	}
	if charsetOK {
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD THREAD " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
			return false, err
		}
	}
	if !imapThreadAlgorithmIsSupported(algorithm) {
		_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
		return false, err
	}
	if !charsetOK {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] THREAD charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD THREAD requires search criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO THREAD failed\r\n")
		return false, writeErr
	}
	requestsModSeq := imapSearchRequestsModSeq(searchFields)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "THREAD")
		}
		state.condstoreAware = true
	}
	results, _, ok, err := s.imapSearchResults(context.Background(), state, searchFields, messages, uidMode, false)
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO THREAD failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD search criteria are unsupported\r\n")
		return false, err
	}
	if save {
		state.savedSearch = imapSavedSearchMatches(results)
	}
	if _, err := writer.WriteString(imapOrderedSubjectThreadResponse(results) + "\r\n"); err != nil {
		return false, err
	}
	completion := "THREAD"
	if uidMode {
		completion = "UID THREAD"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

type imapSearchReturnSpec struct {
	extended bool
	save     bool
	options  []string
}

func imapSearchReturnOptions(fields []string) (imapSearchReturnSpec, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return imapSearchReturnSpec{}, fields, true
	}
	if len(fields) < 2 {
		return imapSearchReturnSpec{}, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	if len(tokens) == 0 {
		tokens = []string{"ALL"}
	}
	seen := make(map[string]struct{}, len(tokens))
	options := make([]string, 0, len(tokens))
	for _, token := range tokens {
		option := strings.ToUpper(token)
		switch option {
		case "MIN", "MAX", "ALL", "COUNT", "SAVE":
		default:
			return imapSearchReturnSpec{}, nil, false
		}
		if _, ok := seen[option]; ok {
			return imapSearchReturnSpec{}, nil, false
		}
		seen[option] = struct{}{}
		if option == "SAVE" {
			continue
		}
		options = append(options, option)
	}
	_, save := seen["SAVE"]
	return imapSearchReturnSpec{extended: true, save: save, options: options}, rest, true
}

func imapSearchSaveReturnOption(fields []string) (bool, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return false, fields, true
	}
	if len(fields) < 2 {
		return false, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return false, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return false, nil, false
	}
	if len(tokens) != 1 || !strings.EqualFold(tokens[0], "SAVE") {
		return false, nil, false
	}
	return true, rest, true
}

func imapSearchReturnOptionTokens(fields []string) ([]string, bool) {
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return nil, false
	}
	return imapParenthesizedAtomListTokens(inner)
}

func imapSearchFieldsContainCriteria(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if strings.EqualFold(fields[0], "CHARSET") {
		return len(fields) >= 3
	}
	return true
}

func imapSearchCriteriaSyntaxError(fields []string) (string, bool) {
	criteria, charsetOK := imapSearchCriteria(fields)
	if !charsetOK {
		return "", false
	}
	if !imapSearchCriteriaSyntaxValid(criteria) {
		return "SEARCH criteria are unsupported", true
	}
	return "", false
}

func imapSearchCriteriaSyntaxValid(criteria []string) bool {
	for i := 0; i < len(criteria); {
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
		if !ok || consumed <= 0 {
			return false
		}
		i += consumed
	}
	return true
}

func imapSearchCriterionSyntaxConsumed(criteria []string) (int, bool) {
	if len(criteria) == 0 {
		return 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "(":
		i := 1
		seenSearchKey := false
		for i < len(criteria) {
			if criteria[i] == ")" {
				break
			}
			consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
			if !ok {
				return 0, false
			}
			seenSearchKey = true
			i += consumed
		}
		if i >= len(criteria) || criteria[i] != ")" || !seenSearchKey {
			return 0, false
		}
		return i + 1, true
	case "ALL", "SEEN", "UNSEEN", "FLAGGED", "UNFLAGGED", "ANSWERED", "UNANSWERED", "DRAFT", "UNDRAFT", "DELETED", "UNDELETED", "RECENT", "OLD", "NEW":
		return 1, true
	case "NOT":
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		return consumed + 1, true
	case "OR":
		leftConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		rightConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1+leftConsumed:])
		if !ok {
			return 0, false
		}
		return 1 + leftConsumed + rightConsumed, true
	case "UID":
		if len(criteria) < 2 || !imapUIDSetSyntaxValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "SINCE", "BEFORE", "ON", "SENTSINCE", "SENTBEFORE", "SENTON":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchDate(criteria[1])
		return 2, ok
	case "LARGER", "SMALLER":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchSize(criteria[1])
		return 2, ok
	case "MODSEQ":
		_, consumed, ok := parseIMAPSearchModSeq(criteria)
		return consumed, ok
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT", "BODY", "TEXT":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := imapSearchStringArgument(criteria[1])
		return 2, ok
	case "HEADER":
		if len(criteria) < 3 {
			return 0, false
		}
		fieldName, ok := imapSearchStringArgument(criteria[1])
		if !ok || !imapSearchHeaderFieldNameValid(fieldName) {
			return 0, false
		}
		if _, ok := imapSearchStringArgument(criteria[2]); !ok {
			return 0, false
		}
		return 3, true
	default:
		if imapSearchCriterionLooksLikeSequenceSet(criteria[0]) {
			return 1, imapSequenceSetSyntaxValid(criteria[0])
		}
		return 0, false
	}
}

func imapSearchCriterionLooksLikeSequenceSet(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if value == "*" || value == "$" || strings.ContainsAny(value, ":,") {
		return true
	}
	first := value[0]
	return first == '+' || first == '-' || ('0' <= first && first <= '9')
}

func imapConsumeParenthesizedFields(fields []string) ([]string, []string, bool) {
	if len(fields) == 0 {
		return nil, nil, false
	}
	depth := 0
	for i, field := range fields {
		if i == 0 && !strings.HasPrefix(strings.TrimSpace(field), "(") {
			return nil, nil, false
		}
		depth += strings.Count(field, "(")
		depth -= strings.Count(field, ")")
		if depth == 0 {
			return fields[:i+1], fields[i+1:], true
		}
		if depth < 0 {
			return nil, nil, false
		}
	}
	return nil, nil, false
}

func imapSearchCriteria(criteria []string) ([]string, bool) {
	if len(criteria) >= 2 && strings.EqualFold(criteria[0], "CHARSET") {
		charset, ok := imapSupportedCharset(criteria[1])
		if !ok {
			return nil, false
		}
		switch charset {
		case "US-ASCII", "UTF-8":
			return imapNormalizeSearchCriteria(criteria[2:]), true
		}
	}
	return imapNormalizeSearchCriteria(criteria), true
}

func imapSupportedCharset(value string) (string, bool) {
	if strings.Contains(value, `"`) {
		return "", false
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}
	charset := strings.ToUpper(value)
	switch charset {
	case "US-ASCII", "UTF-8":
		return charset, true
	default:
		return "", false
	}
}

func imapNormalizeSearchCriteria(criteria []string) []string {
	normalized := make([]string, 0, len(criteria))
	for _, token := range criteria {
		if token == "" {
			normalized = append(normalized, imapEmptySearchStringToken)
			continue
		}
		for strings.HasPrefix(token, "(") {
			normalized = append(normalized, "(")
			token = token[1:]
		}
		trailingGroups := 0
		for strings.HasSuffix(token, ")") {
			trailingGroups++
			token = strings.TrimSuffix(token, ")")
		}
		if token != "" {
			normalized = append(normalized, token)
		}
		for ; trailingGroups > 0; trailingGroups-- {
			normalized = append(normalized, ")")
		}
	}
	return normalized
}

const imapEmptySearchStringToken = "\x00EMPTY-SEARCH-STRING"

type imapSearchMatch struct {
	value          uint32
	uid            UID
	sequenceNumber uint32
	modSeq         uint64
	summary        MessageSummary
}

type imapSearchSavedMessage struct {
	uid            UID
	sequenceNumber uint32
}

func (s *Server) imapSearchResults(ctx context.Context, state *imapConnState, criteria []string, messages []MessageSummary, uidMode bool, requestsModSeq bool) ([]imapSearchMatch, uint64, bool, error) {
	if len(criteria) == 0 {
		return nil, 0, false, nil
	}
	maxUID := imapMaxSummaryUID(messages)
	predicates := make([]imapSearchPredicate, 0, len(criteria))
	for i := 0; i < len(criteria); {
		predicate, consumed, ok := imapParseSearchPredicate(criteria[i:], state.selectedMessages, maxUID, state)
		if !ok {
			return nil, 0, false, nil
		}
		if predicate != nil {
			predicates = append(predicates, predicate)
		}
		i += consumed
	}
	results := make([]imapSearchMatch, 0, len(messages))
	var highestModSeq uint64
	for i, summary := range messages {
		matches, err := s.imapMessageMatchesSearchPredicates(ctx, state, summary, i, predicates)
		if err != nil {
			return nil, 0, true, err
		}
		if !matches {
			continue
		}
		if requestsModSeq && summary.ModSeq > highestModSeq {
			highestModSeq = summary.ModSeq
		}
		var value uint32
		if uidMode {
			value = uint32(summary.UID)
		} else {
			value = imapSearchSequenceNumber(summary, i)
		}
		results = append(results, imapSearchMatch{
			value:          value,
			uid:            summary.UID,
			sequenceNumber: imapSearchSequenceNumber(summary, i),
			modSeq:         summary.ModSeq,
			summary:        summary,
		})
	}
	if len(results) == 0 {
		highestModSeq = 0
	}
	return results, highestModSeq, true, nil
}

func imapMaxSummaryUID(messages []MessageSummary) UID {
	var maxUID UID
	for _, message := range messages {
		if message.UID > maxUID {
			maxUID = message.UID
		}
	}
	return maxUID
}

type imapSortCriterion struct {
	key     string
	reverse bool
}

func imapSortCommandArguments(fields []string) ([]imapSortCriterion, []string, bool, bool) {
	sortFields, rest, ok := imapConsumeParenthesizedFields(fields)
	if !ok || len(rest) < 2 {
		return nil, nil, true, false
	}
	criteria, ok := imapSortCriteria(sortFields)
	if !ok || len(criteria) == 0 {
		return nil, nil, true, false
	}
	if _, ok := imapSupportedCharset(rest[0]); !ok {
		return nil, nil, false, true
	}
	return criteria, imapNormalizeSearchCriteria(rest[1:]), true, true
}

func imapThreadCommandArguments(fields []string) (string, []string, bool, bool) {
	if len(fields) < 3 {
		return "", nil, true, false
	}
	algorithm, ok := imapThreadAlgorithm(fields[0])
	if !ok {
		return "", nil, true, true
	}
	if _, ok := imapSupportedCharset(fields[1]); !ok {
		return algorithm, nil, false, true
	}
	return algorithm, imapNormalizeSearchCriteria(fields[2:]), true, true
}

func imapThreadAlgorithm(value string) (string, bool) {
	if strings.Contains(value, `"`) {
		return "", false
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}
	algorithm := strings.ToUpper(value)
	if algorithm == "" {
		return "", false
	}
	return algorithm, true
}

func imapThreadAlgorithmIsSupported(algorithm string) bool {
	return algorithm == "ORDEREDSUBJECT"
}

func imapSortCriteria(fields []string) ([]imapSortCriterion, bool) {
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return nil, false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return nil, false
	}
	criteria := make([]imapSortCriterion, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		reverse := false
		token := strings.ToUpper(tokens[i])
		if token == "REVERSE" {
			reverse = true
			i++
			if i >= len(tokens) {
				return nil, false
			}
			token = strings.ToUpper(tokens[i])
		}
		switch token {
		case "ARRIVAL", "CC", "DATE", "FROM", "SIZE", "SUBJECT", "TO":
			criteria = append(criteria, imapSortCriterion{key: token, reverse: reverse})
		default:
			return nil, false
		}
	}
	return criteria, true
}

func imapSortMatches(matches []imapSearchMatch, criteria []imapSortCriterion) {
	collator := collate.New(language.Und, collate.IgnoreCase)
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		for _, criterion := range criteria {
			cmp := imapCompareSortCriterion(collator, left.summary, right.summary, criterion.key)
			if cmp == 0 {
				continue
			}
			if criterion.reverse {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.sequenceNumber < right.sequenceNumber
	})
}

type imapThreadGroup struct {
	subject string
	matches []imapSearchMatch
}

func imapOrderedSubjectThreadResponse(matches []imapSearchMatch) string {
	if len(matches) == 0 {
		return "* THREAD"
	}
	criteria := []imapSortCriterion{{key: "SUBJECT"}, {key: "DATE"}}
	imapSortMatches(matches, criteria)
	groups := make([]imapThreadGroup, 0, len(matches))
	groupIndex := make(map[string]int)
	for _, match := range matches {
		subject := imapBaseSubject(match.summary.Envelope.Subject)
		key := strings.ToLower(subject)
		if index, ok := groupIndex[key]; ok {
			groups[index].matches = append(groups[index].matches, match)
			continue
		}
		groupIndex[key] = len(groups)
		groups = append(groups, imapThreadGroup{subject: key, matches: []imapSearchMatch{match}})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		left := groups[i].matches[0]
		right := groups[j].matches[0]
		cmp := imapCompareTime(imapSortSentDate(left.summary), imapSortSentDate(right.summary))
		if cmp != 0 {
			return cmp < 0
		}
		return left.sequenceNumber < right.sequenceNumber
	})
	threads := make([]string, 0, len(groups))
	for _, group := range groups {
		threads = append(threads, imapOrderedSubjectThread(group.matches))
	}
	return "* THREAD " + strings.Join(threads, "")
}

func imapOrderedSubjectThread(matches []imapSearchMatch) string {
	if len(matches) == 0 {
		return "()"
	}
	if len(matches) == 1 {
		return fmt.Sprintf("(%d)", matches[0].value)
	}
	if len(matches) == 2 {
		return fmt.Sprintf("(%d %d)", matches[0].value, matches[1].value)
	}
	children := make([]string, 0, len(matches)-1)
	for _, match := range matches[1:] {
		children = append(children, fmt.Sprintf("(%d)", match.value))
	}
	return fmt.Sprintf("(%d %s)", matches[0].value, strings.Join(children, ""))
}

func imapCompareSortCriterion(collator *collate.Collator, left MessageSummary, right MessageSummary, key string) int {
	switch key {
	case "ARRIVAL":
		return imapCompareTime(left.InternalDate, right.InternalDate)
	case "DATE":
		return imapCompareTime(imapSortSentDate(left), imapSortSentDate(right))
	case "SIZE":
		return imapCompareInt64(left.Size, right.Size)
	case "FROM":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.From), imapSortFirstMailbox(right.Envelope.From))
	case "TO":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.To), imapSortFirstMailbox(right.Envelope.To))
	case "CC":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.Cc), imapSortFirstMailbox(right.Envelope.Cc))
	case "SUBJECT":
		return collator.CompareString(imapBaseSubject(left.Envelope.Subject), imapBaseSubject(right.Envelope.Subject))
	default:
		return 0
	}
}

func imapSortSentDate(summary MessageSummary) time.Time {
	if !summary.Envelope.Date.IsZero() {
		return summary.Envelope.Date
	}
	return summary.InternalDate
}

func imapSortFirstMailbox(addresses []Address) string {
	if len(addresses) == 0 {
		return ""
	}
	return strings.TrimSpace(addresses[0].Mailbox)
}

func imapCompareTime(left time.Time, right time.Time) int {
	if left.Equal(right) {
		return 0
	}
	if left.Before(right) {
		return -1
	}
	return 1
}

func imapCompareInt64(left int64, right int64) int {
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func imapBaseSubject(subject string) string {
	subject = imapDecodeSubjectHeader(subject)
	subject = strings.TrimSpace(strings.Join(strings.Fields(subject), " "))
	for {
		previous := subject
		subject = imapStripSubjectTrailers(subject)
		for {
			stripped := imapStripSubjectLeader(subject)
			if stripped == subject {
				break
			}
			subject = stripped
			subject = imapStripSubjectTrailers(subject)
		}
		if inner, ok := imapStripForwardWrapper(subject); ok {
			subject = strings.TrimSpace(inner)
			continue
		}
		if subject == previous {
			return subject
		}
	}
}

func imapDecodeSubjectHeader(subject string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(subject)
	if err != nil {
		return subject
	}
	return decoded
}

func imapStripSubjectTrailers(subject string) string {
	for {
		trimmed := strings.TrimSpace(subject)
		if strings.HasSuffix(strings.ToLower(trimmed), "(fwd)") {
			subject = strings.TrimSpace(trimmed[:len(trimmed)-5])
			continue
		}
		return trimmed
	}
}

func imapStripSubjectLeader(subject string) string {
	subject = strings.TrimSpace(subject)
	for {
		stripped, ok := imapStripSubjectBlob(subject)
		if !ok {
			break
		}
		subject = strings.TrimSpace(stripped)
	}
	lower := strings.ToLower(subject)
	for _, prefix := range []string{"re", "fw", "fwd"} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := subject[len(prefix):]
		rest = strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(rest, "[") {
			withoutBlob, ok := imapStripSubjectBlob(rest)
			if ok {
				rest = strings.TrimLeft(withoutBlob, " \t")
			}
		}
		if strings.HasPrefix(rest, ":") {
			return strings.TrimSpace(rest[1:])
		}
	}
	return subject
}

func imapStripSubjectBlob(subject string) (string, bool) {
	if !strings.HasPrefix(subject, "[") {
		return subject, false
	}
	if strings.HasPrefix(strings.ToLower(subject), "[fwd:") {
		return subject, false
	}
	end := strings.Index(subject, "]")
	if end <= 0 {
		return subject, false
	}
	return subject[end+1:], true
}

func imapStripForwardWrapper(subject string) (string, bool) {
	trimmed := strings.TrimSpace(subject)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "[fwd:") && strings.HasSuffix(trimmed, "]") {
		return trimmed[5 : len(trimmed)-1], true
	}
	return subject, false
}

type imapSearchPredicate func(context.Context, *Server, *imapConnState, MessageSummary, int) (bool, error)

func imapParseSearchPredicate(criteria []string, maxSequence uint32, maxUID UID, state *imapConnState) (imapSearchPredicate, int, bool) {
	if len(criteria) == 0 {
		return nil, 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "(":
		predicates := make([]imapSearchPredicate, 0, len(criteria))
		i := 1
		seenSearchKey := false
		for i < len(criteria) {
			if criteria[i] == ")" {
				break
			}
			predicate, consumed, ok := imapParseSearchPredicate(criteria[i:], maxSequence, maxUID, state)
			if !ok {
				return nil, 0, false
			}
			if predicate != nil {
				predicates = append(predicates, predicate)
			}
			seenSearchKey = true
			i += consumed
		}
		if i >= len(criteria) || criteria[i] != ")" || !seenSearchKey {
			return nil, 0, false
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			for _, predicate := range predicates {
				matches, err := imapSearchPredicateMatches(ctx, server, state, predicate, summary, index)
				if err != nil {
					return false, err
				}
				if !matches {
					return false, nil
				}
			}
			return true, nil
		}, i + 1, true
	case "ALL":
		return nil, 1, true
	case "NOT":
		predicate, consumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		if predicate == nil {
			return func(context.Context, *Server, *imapConnState, MessageSummary, int) (bool, error) { return false, nil }, consumed + 1, true
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			matches, err := predicate(ctx, server, state, summary, index)
			if err != nil {
				return false, err
			}
			return !matches, nil
		}, consumed + 1, true
	case "OR":
		left, leftConsumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		right, rightConsumed, ok := imapParseSearchPredicate(criteria[1+leftConsumed:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			leftMatches, err := imapSearchPredicateMatches(ctx, server, state, left, summary, index)
			if err != nil {
				return false, err
			}
			if leftMatches {
				return true, nil
			}
			return imapSearchPredicateMatches(ctx, server, state, right, summary, index)
		}, 1 + leftConsumed + rightConsumed, true
	case "SEEN", "UNSEEN", "FLAGGED", "UNFLAGGED", "ANSWERED", "UNANSWERED", "DRAFT", "UNDRAFT", "DELETED", "UNDELETED", "RECENT", "OLD", "NEW":
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesFlagSearch(summary, criterion), nil
		}, 1, true
	case "UID":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		var ranges []imapUIDRange
		if criteria[1] == "$" {
			uids := imapSavedSearchUIDs(state)
			ranges = make([]imapUIDRange, 0, len(uids))
			for _, uid := range uids {
				ranges = append(ranges, imapUIDRange{start: uid, end: uid})
			}
		} else {
			var ok bool
			ranges, ok = parseIMAPUIDSetRanges(criteria[1], maxUID)
			if !ok {
				return nil, 0, false
			}
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapUIDMatchesRanges(summary.UID, ranges), nil
		}, 2, true
	case "SINCE", "BEFORE", "ON":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		day, ok := parseIMAPSearchDate(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesDateSearch(summary, criterion, day), nil
		}, 2, true
	case "SENTSINCE", "SENTBEFORE", "SENTON":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		day, ok := parseIMAPSearchDate(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesSentDateSearch(summary, criterion, day), nil
		}, 2, true
	case "LARGER", "SMALLER":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		size, ok := parseIMAPSearchSize(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesSizeSearch(summary, criterion, size), nil
		}, 2, true
	case "MODSEQ":
		threshold, consumed, ok := parseIMAPSearchModSeq(criteria)
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return summary.ModSeq >= threshold, nil
		}, consumed, true
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return nil, 0, false
		}
		keyword := criteria[1]
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			matches := imapMessageMatchesKeywordSearch(summary, keyword)
			if criterion == "UNKEYWORD" {
				return !matches, nil
			}
			return matches, nil
		}, 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[1])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesTextSearch(summary, criterion, query), nil
		}, 2, true
	case "HEADER":
		if len(criteria) < 3 {
			return nil, 0, false
		}
		fieldName, ok := imapSearchStringArgument(criteria[1])
		if !ok || !imapSearchHeaderFieldNameValid(fieldName) {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[2])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesHeaderSearch(ctx, state, summary, fieldName, query)
		}, 3, true
	case "BODY", "TEXT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[1])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesBodySearch(ctx, state, summary, criterion, query)
		}, 2, true
	default:
		sequenceNumbers, ok := parseIMAPSequenceSetForState(criteria[0], maxSequence, state)
		if ok {
			allowed := make(map[uint32]struct{}, len(sequenceNumbers))
			for _, sequenceNumber := range sequenceNumbers {
				allowed[sequenceNumber] = struct{}{}
			}
			return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, index int) (bool, error) {
				_, ok := allowed[imapSearchSequenceNumber(summary, index)]
				return ok, nil
			}, 1, true
		}
		return nil, 0, false
	}
}

func imapSearchKeywordValid(value string) bool {
	return imapAtomValid(value)
}

func imapMessageMatchesKeywordSearch(summary MessageSummary, keyword string) bool {
	keyword = CanonicalIMAPFlag(keyword)
	for _, flag := range summary.Flags.IMAPFlags() {
		if flag == keyword {
			return true
		}
	}
	return false
}

func imapSearchStringArgument(value string) (string, bool) {
	if value == imapEmptySearchStringToken {
		return "", true
	}
	return value, true
}

func imapSearchRequestsModSeq(criteria []string) bool {
	for _, criterion := range criteria {
		if strings.EqualFold(criterion, "MODSEQ") {
			return true
		}
	}
	return false
}

func imapSearchPredicateMatches(ctx context.Context, server *Server, state *imapConnState, predicate imapSearchPredicate, summary MessageSummary, index int) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	return predicate(ctx, server, state, summary, index)
}

func (s *Server) imapMessageMatchesSearchPredicates(ctx context.Context, state *imapConnState, summary MessageSummary, index int, predicates []imapSearchPredicate) (bool, error) {
	for _, predicate := range predicates {
		matches, err := imapSearchPredicateMatches(ctx, s, state, predicate, summary, index)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil
		}
	}
	return true, nil
}

func imapSearchSequenceNumber(summary MessageSummary, index int) uint32 {
	if summary.SequenceNumber != 0 {
		return summary.SequenceNumber
	}
	return uint32(index + 1)
}

func imapSearchFlagResults(messages []MessageSummary, uidMode bool, criterion string) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesFlagSearch(summary, criterion) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesFlagSearch(summary MessageSummary, criterion string) bool {
	switch criterion {
	case "SEEN":
		return summary.Flags.Read
	case "UNSEEN":
		return !summary.Flags.Read
	case "FLAGGED":
		return summary.Flags.Starred
	case "UNFLAGGED":
		return !summary.Flags.Starred
	case "ANSWERED":
		return summary.Flags.Answered
	case "UNANSWERED":
		return !summary.Flags.Answered
	case "DRAFT":
		return summary.Flags.Draft || strings.EqualFold(strings.TrimSpace(summary.Flags.Status), "draft")
	case "UNDRAFT":
		return !summary.Flags.Draft && !strings.EqualFold(strings.TrimSpace(summary.Flags.Status), "draft")
	case "DELETED":
		return summary.Flags.Deleted
	case "UNDELETED":
		return !summary.Flags.Deleted
	case "RECENT":
		return summary.Recent
	case "NEW":
		return summary.Recent && !summary.Flags.Read
	case "OLD":
		return !summary.Recent
	default:
		return false
	}
}

func imapSearchAllResults(messages []MessageSummary, uidMode bool) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapSearchDateResults(messages []MessageSummary, uidMode bool, criterion string, day time.Time) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesDateSearch(summary, criterion, day) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesDateSearch(summary MessageSummary, criterion string, day time.Time) bool {
	date := summary.InternalDate
	if date.IsZero() {
		return false
	}
	messageDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	switch criterion {
	case "SINCE":
		return !messageDay.Before(day)
	case "BEFORE":
		return messageDay.Before(day)
	case "ON":
		return messageDay.Equal(day)
	default:
		return false
	}
}

func imapSearchSentDateResults(messages []MessageSummary, uidMode bool, criterion string, day time.Time) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesSentDateSearch(summary, criterion, day) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesSentDateSearch(summary MessageSummary, criterion string, day time.Time) bool {
	date := summary.Envelope.Date
	if date.IsZero() {
		return false
	}
	messageDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	switch criterion {
	case "SENTSINCE":
		return !messageDay.Before(day)
	case "SENTBEFORE":
		return messageDay.Before(day)
	case "SENTON":
		return messageDay.Equal(day)
	default:
		return false
	}
}

func parseIMAPSearchDate(value string) (time.Time, bool) {
	if strings.Contains(value, `"`) {
		return time.Time{}, false
	}
	if strings.TrimSpace(value) != value {
		return time.Time{}, false
	}
	var ok bool
	value, ok = imapCanonicalDateMonth(value)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{"02-Jan-2006", "2-Jan-2006"} {
		day, err := time.Parse(layout, value)
		if err == nil {
			return day, true
		}
	}
	return time.Time{}, false
}

func parseIMAPSearchSize(value string) (int64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomRFC3501(value) {
		return 0, false
	}
	size, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, false
	}
	return int64(size), true
}

func parseIMAPSearchModSeq(criteria []string) (uint64, int, bool) {
	if len(criteria) < 2 || !strings.EqualFold(criteria[0], "MODSEQ") {
		return 0, 0, false
	}
	if threshold, ok := parseIMAPModSeqValue(criteria[1]); ok {
		return threshold, 2, true
	}
	if len(criteria) < 4 || !imapSearchModSeqEntryTypeValid(criteria[2]) {
		return 0, 0, false
	}
	threshold, ok := parseIMAPModSeqValue(criteria[3])
	if !ok {
		return 0, 0, false
	}
	return threshold, 4, true
}

func parseIMAPModSeqValue(value string) (uint64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomDigitsOnly(value) {
		return 0, false
	}
	modseq, err := strconv.ParseUint(value, 10, 64)
	if err != nil || modseq == 0 {
		return 0, false
	}
	return modseq, true
}

func parseIMAPModSeqValzer(value string) (uint64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomDigitsOnly(value) {
		return 0, false
	}
	modseq, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return modseq, true
}

func imapSearchModSeqEntryTypeValid(value string) bool {
	if strings.Contains(value, `"`) {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	switch strings.ToUpper(value) {
	case "SHARED", "PRIV", "ALL":
		return true
	default:
		return false
	}
}

func imapNumberAtomDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func imapNZNumberAtomDigitsOnly(value string) bool {
	if !imapNumberAtomDigitsOnly(value) {
		return false
	}
	return value[0] != '0'
}

func imapNumberAtomRFC3501(value string) bool {
	if !imapNumberAtomDigitsOnly(value) {
		return false
	}
	return value == "0" || value[0] != '0'
}

func imapSearchSizeResults(messages []MessageSummary, uidMode bool, criterion string, size int64) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesSizeSearch(summary, criterion, size) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesSizeSearch(summary MessageSummary, criterion string, size int64) bool {
	switch criterion {
	case "LARGER":
		return summary.Size > size
	case "SMALLER":
		return summary.Size < size
	default:
		return false
	}
}

func imapSearchTextResults(messages []MessageSummary, uidMode bool, criterion string, query string) []uint32 {
	query = strings.ToLower(query)
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesTextSearch(summary, criterion, query) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesTextSearch(summary MessageSummary, criterion string, query string) bool {
	switch criterion {
	case "SUBJECT":
		return strings.Contains(strings.ToLower(summary.Envelope.Subject), query)
	case "FROM":
		return imapAddressListContains(summary.Envelope.From, query)
	case "TO":
		return imapAddressListContains(summary.Envelope.To, query)
	case "CC":
		return imapAddressListContains(summary.Envelope.Cc, query)
	case "BCC":
		return imapAddressListContains(summary.Envelope.Bcc, query)
	default:
		return false
	}
}

const (
	maxIMAPCommandLineBytes      = 8192
	maxIMAPAuthIdentityBytes     = 1024
	maxIMAPAuthPasswordBytes     = 4096
	maxIMAPSASLPlainDecodedBytes = maxIMAPAuthIdentityBytes*2 + maxIMAPAuthPasswordBytes + 2
	maxIMAPSASLPlainEncodedBytes = ((maxIMAPSASLPlainDecodedBytes + 2) / 3) * 4
	maxIMAPSearchLiteralBytes    = 1 << 20
	maxIMAPCommandLiteralBytes   = 10 << 20
	maxIMAPBodyMetadataTextBytes = 1024
	maxIMAPEnvelopeAddressCount  = 100
)

func (s *Server) imapMessageMatchesBodySearch(ctx context.Context, state *imapConnState, summary MessageSummary, criterion string, query string) (bool, error) {
	if s == nil || state == nil || state.session == nil || summary.UID == 0 {
		return false, nil
	}
	message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UID:       summary.UID,
	})
	if err != nil {
		return false, err
	}
	if message.Body == nil {
		return false, nil
	}
	defer message.Body.Close()
	literal, err := readIMAPSearchLiteral(message.Body, strings.EqualFold(criterion, "BODY"))
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(literal)), query), nil
}

func (s *Server) imapMessageMatchesHeaderSearch(ctx context.Context, state *imapConnState, summary MessageSummary, fieldName string, query string) (bool, error) {
	if s == nil || state == nil || state.session == nil || strings.TrimSpace(fieldName) == "" || summary.UID == 0 {
		return false, nil
	}
	message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UID:       summary.UID,
	})
	if err != nil {
		return false, err
	}
	if message.Body == nil {
		return false, nil
	}
	defer message.Body.Close()
	header, err := readIMAPSearchHeader(message.Body)
	if err != nil {
		return false, err
	}
	fieldLiteral := filterIMAPHeaderFields(header, []string{fieldName}, false)
	if strings.TrimSpace(string(fieldLiteral)) == "" {
		return false, nil
	}
	return strings.Contains(strings.ToLower(string(fieldLiteral)), query), nil
}

func readIMAPSearchHeader(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		data = data[:maxIMAPSearchLiteralBytes]
	}
	if end := imapHeaderEnd(data); end >= 0 {
		return data[:end], nil
	}
	return data, nil
}

func readIMAPSearchLiteral(reader io.Reader, bodyOnly bool) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		data = data[:maxIMAPSearchLiteralBytes]
	}
	if !bodyOnly {
		return data, nil
	}
	if end := imapHeaderEnd(data); end >= 0 {
		return data[end:], nil
	}
	return nil, nil
}

func imapAddressListContains(addresses []Address, query string) bool {
	for _, address := range addresses {
		if strings.Contains(strings.ToLower(address.Name), query) ||
			strings.Contains(strings.ToLower(address.Mailbox), query) ||
			strings.Contains(strings.ToLower(address.Host), query) {
			return true
		}
	}
	return false
}

func imapSearchResultSuffix(results []uint32, highestModSeq uint64, includeModSeq bool) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, strconv.FormatUint(uint64(result), 10))
	}
	suffix := " " + strings.Join(parts, " ")
	if includeModSeq {
		suffix += fmt.Sprintf(" (MODSEQ %d)", highestModSeq)
	}
	return suffix
}

func imapSearchResultValues(results []imapSearchMatch) []uint32 {
	values := make([]uint32, 0, len(results))
	for _, result := range results {
		values = append(values, result.value)
	}
	return values
}

func imapESearchResponse(tag string, results []imapSearchMatch, uidMode bool, options []string, highestModSeq uint64, includeModSeq bool) string {
	parts := []string{"* ESEARCH", "(TAG " + imapQuotedString(tag) + ")"}
	if uidMode {
		parts = append(parts, "UID")
	}
	values := imapSearchResultValues(results)
	for _, option := range options {
		switch option {
		case "MIN":
			if len(values) > 0 {
				parts = append(parts, "MIN", strconv.FormatUint(uint64(imapMinSearchResult(values)), 10))
			}
		case "MAX":
			if len(values) > 0 {
				parts = append(parts, "MAX", strconv.FormatUint(uint64(imapMaxSearchResult(values)), 10))
			}
		case "ALL":
			if len(values) > 0 {
				parts = append(parts, "ALL", imapSearchSequenceSet(values))
			}
		case "COUNT":
			parts = append(parts, "COUNT", strconv.FormatUint(uint64(len(values)), 10))
		}
	}
	if includeModSeq {
		if modSeq := imapESearchModSeq(results, options, highestModSeq); modSeq > 0 {
			parts = append(parts, "MODSEQ", strconv.FormatUint(modSeq, 10))
		}
	}
	return strings.Join(parts, " ")
}

func imapMinSearchResult(values []uint32) uint32 {
	minValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}

func imapMaxSearchResult(values []uint32) uint32 {
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func imapSearchSequenceSet(values []uint32) string {
	if len(values) == 0 {
		return ""
	}
	compact := append([]uint32(nil), values...)
	sort.Slice(compact, func(i, j int) bool {
		return compact[i] < compact[j]
	})
	parts := make([]string, 0, len(compact))
	start := compact[0]
	prev := compact[0]
	for _, value := range compact[1:] {
		if value == prev {
			continue
		}
		if value == prev+1 {
			prev = value
			continue
		}
		parts = append(parts, imapSearchSequenceRange(start, prev))
		start = value
		prev = value
	}
	parts = append(parts, imapSearchSequenceRange(start, prev))
	return strings.Join(parts, ",")
}

func imapSearchSequenceRange(start uint32, end uint32) string {
	if start == end {
		return strconv.FormatUint(uint64(start), 10)
	}
	return strconv.FormatUint(uint64(start), 10) + ":" + strconv.FormatUint(uint64(end), 10)
}

func imapESearchModSeq(results []imapSearchMatch, options []string, highestModSeq uint64) uint64 {
	if len(results) == 0 {
		return 0
	}
	if len(options) == 1 && options[0] == "MIN" {
		return imapSearchResultModSeq(results, imapMinSearchResult(imapSearchResultValues(results)))
	}
	if len(options) == 1 && options[0] == "MAX" {
		return imapSearchResultModSeq(results, imapMaxSearchResult(imapSearchResultValues(results)))
	}
	if len(options) == 2 && imapSearchReturnHas(options, "MIN") && imapSearchReturnHas(options, "MAX") {
		minModSeq := imapSearchResultModSeq(results, imapMinSearchResult(imapSearchResultValues(results)))
		maxModSeq := imapSearchResultModSeq(results, imapMaxSearchResult(imapSearchResultValues(results)))
		if maxModSeq > minModSeq {
			return maxModSeq
		}
		return minModSeq
	}
	return highestModSeq
}

func imapSearchReturnHas(options []string, want string) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}

func imapSearchResultModSeq(results []imapSearchMatch, value uint32) uint64 {
	for _, result := range results {
		if result.value == value {
			return result.modSeq
		}
	}
	return 0
}

func imapSavedSearchResults(results []imapSearchMatch, options []string) []imapSearchSavedMessage {
	if len(results) == 0 {
		return nil
	}
	values := imapSearchResultValues(results)
	saveAll := len(options) == 0 || imapSearchReturnHas(options, "ALL") || imapSearchReturnHas(options, "COUNT")
	if saveAll {
		return imapSavedSearchMatches(results)
	}
	saved := make([]imapSearchSavedMessage, 0, 2)
	if imapSearchReturnHas(options, "MIN") {
		if result, ok := imapSearchResultForValue(results, imapMinSearchResult(values)); ok {
			saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
		}
	}
	if imapSearchReturnHas(options, "MAX") {
		if result, ok := imapSearchResultForValue(results, imapMaxSearchResult(values)); ok && !imapSavedSearchContains(saved, result) {
			saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
		}
	}
	return saved
}

func imapSavedSearchMatches(results []imapSearchMatch) []imapSearchSavedMessage {
	saved := make([]imapSearchSavedMessage, 0, len(results))
	for _, result := range results {
		saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
	}
	return saved
}

func imapSearchResultForValue(results []imapSearchMatch, value uint32) (imapSearchMatch, bool) {
	for _, result := range results {
		if result.value == value {
			return result, true
		}
	}
	return imapSearchMatch{}, false
}

func imapSavedSearchContains(saved []imapSearchSavedMessage, result imapSearchMatch) bool {
	for _, existing := range saved {
		if existing.uid == result.uid && existing.sequenceNumber == result.sequenceNumber {
			return true
		}
	}
	return false
}

func (s *Server) handleUIDFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 5 {
		_, err := writer.WriteString(tag + " BAD UID FETCH requires UID set and data items\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(context.Background(), state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID FETCH failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
		return false, err
	}
	return s.writeFetchResponses(writer, tag, fields[4:], state, uids, "UID FETCH")
}

func (s *Server) handleUIDCopy(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 5 {
		_, err := writer.WriteString(tag + " BAD UID COPY requires UID set and destination mailbox\r\n")
		return false, err
	}
	destMailbox, destOK := imapDecodeMailboxName(fields[4])
	if !destOK {
		_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(context.Background(), state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID COPY failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
		return false, err
	}
	return s.writeCopyResponse(writer, tag, state, uids, MailboxID(destMailbox), "UID COPY")
}

func (s *Server) handleUIDMove(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 5 {
		_, err := writer.WriteString(tag + " BAD UID MOVE requires UID set and destination mailbox\r\n")
		return false, err
	}
	destMailbox, destOK := imapDecodeMailboxName(fields[4])
	if !destOK {
		_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(context.Background(), state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID MOVE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	return s.writeMoveResponse(writer, tag, state, uids, MailboxID(destMailbox), "UID MOVE")
}

func (s *Server) handleUIDExpunge(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires UID set\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(context.Background(), state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID EXPUNGE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	return s.writeExpungeResponses(writer, tag, state, uids, "UID EXPUNGE")
}

func (s *Server) handleFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD FETCH requires sequence set and data items\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
		return false, err
	}
	if message, ok := imapFetchDataItemsSyntaxError(fields[3:]); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSetForState(fields[2], state.selectedMessages, state)
	if !ok {
		_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(context.Background(), state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO FETCH failed\r\n")
		return false, writeErr
	}
	return s.writeFetchResponses(writer, tag, fields[3:], state, uids, "FETCH")
}

func (s *Server) handleCopy(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD COPY requires sequence set and destination mailbox\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
		return false, err
	}
	destMailbox, destOK := imapDecodeMailboxName(fields[3])
	if !destOK {
		_, err := writer.WriteString(tag + " BAD COPY destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSetForState(fields[2], state.selectedMessages, state)
	if !ok {
		_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(context.Background(), state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO COPY failed\r\n")
		return false, writeErr
	}
	return s.writeCopyResponse(writer, tag, state, uids, MailboxID(destMailbox), "COPY")
}

func (s *Server) handleMove(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD MOVE requires sequence set and destination mailbox\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
		return false, err
	}
	destMailbox, destOK := imapDecodeMailboxName(fields[3])
	if !destOK {
		_, err := writer.WriteString(tag + " BAD MOVE destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSetForState(fields[2], state.selectedMessages, state)
	if !ok {
		_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(context.Background(), state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO MOVE failed\r\n")
		return false, writeErr
	}
	return s.writeMoveResponse(writer, tag, state, uids, MailboxID(destMailbox), "MOVE")
}

func (s *Server) handleAppend(writer *bufio.Writer, tag string, fields []string, literals []string, state *imapConnState) (bool, error) {
	if len(literals) == 0 {
		_, err := writer.WriteString(tag + " BAD APPEND requires mailbox and literal\r\n")
		return false, err
	}
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD APPEND requires mailbox and literal\r\n")
		return false, err
	}
	flags, internalDate, ok := imapAppendOptions(fields[3 : len(fields)-1])
	if !ok {
		_, err := writer.WriteString(tag + " BAD APPEND options are unsupported\r\n")
		return false, err
	}
	mailboxName, ok := imapDecodeMailboxName(fields[2])
	if !ok {
		_, err := writer.WriteString(tag + " BAD APPEND mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(mailboxName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] APPEND mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO APPEND failed\r\n")
		return false, writeErr
	}
	if state.readOnly && mailbox.ID == state.selectedMailbox {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	body := literals[len(literals)-1]
	result, err := s.options.Backend.AppendMessage(context.Background(), AppendMessageRequest{
		UserID:       state.session.UserID,
		MailboxID:    mailbox.ID,
		Flags:        flags,
		InternalDate: internalDate,
		Size:         int64(len(body)),
		Body:         strings.NewReader(body),
	})
	if err != nil {
		if errors.Is(err, ErrUnsupportedAppend) {
			_, writeErr := writer.WriteString(tag + " NO APPEND is not supported\r\n")
			return false, writeErr
		}
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] APPEND mailbox does not exist\r\n")
			return false, writeErr
		}
		if errors.Is(err, ErrOverQuota) {
			_, writeErr := writer.WriteString(tag + " NO [OVERQUOTA] APPEND would exceed quota\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO APPEND failed\r\n")
		return false, writeErr
	}
	summary := result.Summary
	if summary.MailboxID == state.selectedMailbox {
		state.selectedMessages = imapAppendExistsCount(state.selectedMessages, summary)
		state.observeHighestModSeq(summary.ModSeq)
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", state.selectedMessages)); err != nil {
			return false, err
		}
	}
	responseCode := ""
	if !result.UIDNotSticky && result.UIDValidity != 0 && summary.UID != 0 {
		responseCode = fmt.Sprintf(" [APPENDUID %d %d]", result.UIDValidity, summary.UID)
	}
	_, err = writer.WriteString(tag + " OK" + responseCode + " APPEND completed\r\n")
	return false, err
}

func imapAppendExistsCount(current uint32, summary MessageSummary) uint32 {
	if summary.SequenceNumber > current {
		return summary.SequenceNumber
	}
	return current + 1
}

func imapAppendOptions(fields []string) (MessageFlags, time.Time, bool) {
	var flags MessageFlags
	var internalDate time.Time
	if len(fields) == 0 {
		return flags, internalDate, true
	}
	i := 0
	if strings.HasPrefix(fields[i], "(") {
		var flagParts []string
		for i < len(fields) {
			flagParts = append(flagParts, fields[i])
			done := strings.HasSuffix(fields[i], ")")
			i++
			if done {
				break
			}
		}
		if len(flagParts) == 0 || !strings.HasSuffix(flagParts[len(flagParts)-1], ")") {
			return MessageFlags{}, time.Time{}, false
		}
		parsed, ok := imapStoreFlags(strings.Join(flagParts, " "))
		if !ok {
			return MessageFlags{}, time.Time{}, false
		}
		flags = parsed
	}
	if i < len(fields) {
		parsed, ok := parseIMAPAppendDate(fields[i])
		if !ok {
			return MessageFlags{}, time.Time{}, false
		}
		internalDate = parsed
		i++
	}
	if i != len(fields) {
		return MessageFlags{}, time.Time{}, false
	}
	return flags, internalDate, true
}

func parseIMAPAppendDate(value string) (time.Time, bool) {
	if len(value) < len("01-Jan-2006 00:00:00 +0000") || value[2] != '-' {
		return time.Time{}, false
	}
	if value[0] == ' ' {
		if value[1] < '1' || value[1] > '9' {
			return time.Time{}, false
		}
	} else if value[0] < '0' || value[0] > '9' || value[1] < '0' || value[1] > '9' {
		return time.Time{}, false
	}
	var ok bool
	value, ok = imapCanonicalDateMonth(value)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"_2-Jan-2006 15:04:05 -0700",
		"02-Jan-2006 15:04:05 -0700",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func imapCanonicalDateMonth(value string) (string, bool) {
	firstDash := strings.IndexByte(value, '-')
	if firstDash < 1 || len(value) < firstDash+5 {
		return "", false
	}
	monthStart := firstDash + 1
	monthEnd := monthStart + 3
	if value[monthEnd] != '-' {
		return "", false
	}
	month, ok := imapCanonicalMonth(value[monthStart:monthEnd])
	if !ok {
		return "", false
	}
	if value[monthStart:monthEnd] == month {
		return value, true
	}
	return value[:monthStart] + month + value[monthEnd:], true
}

func imapCanonicalMonth(value string) (string, bool) {
	if len(value) != 3 {
		return "", false
	}
	switch [3]byte{imapASCIILower(value[0]), imapASCIILower(value[1]), imapASCIILower(value[2])} {
	case [3]byte{'j', 'a', 'n'}:
		return "Jan", true
	case [3]byte{'f', 'e', 'b'}:
		return "Feb", true
	case [3]byte{'m', 'a', 'r'}:
		return "Mar", true
	case [3]byte{'a', 'p', 'r'}:
		return "Apr", true
	case [3]byte{'m', 'a', 'y'}:
		return "May", true
	case [3]byte{'j', 'u', 'n'}:
		return "Jun", true
	case [3]byte{'j', 'u', 'l'}:
		return "Jul", true
	case [3]byte{'a', 'u', 'g'}:
		return "Aug", true
	case [3]byte{'s', 'e', 'p'}:
		return "Sep", true
	case [3]byte{'o', 'c', 't'}:
		return "Oct", true
	case [3]byte{'n', 'o', 'v'}:
		return "Nov", true
	case [3]byte{'d', 'e', 'c'}:
		return "Dec", true
	default:
		return "", false
	}
}

func imapASCIILower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func (s *Server) handleClose(writer *bufio.Writer, tag string, state *imapConnState) (bool, error) {
	if !state.readOnly {
		if _, err := s.options.Backend.Expunge(context.Background(), ExpungeRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
		}); err != nil {
			_, writeErr := writer.WriteString(tag + " NO CLOSE failed\r\n")
			return false, writeErr
		}
	}
	state.deselectMailbox()
	_, err := writer.WriteString(tag + " OK CLOSE completed\r\n")
	return false, err
}

func (s *Server) writeCopyResponse(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, destMailboxID MailboxID, completionCommand string) (bool, error) {
	if len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	destMailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, destMailboxID)
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	results, err := s.options.Backend.CopyMessages(context.Background(), CopyMessagesRequest{
		UserID:          state.session.UserID,
		SourceMailboxID: state.selectedMailbox,
		DestMailboxID:   destMailbox.ID,
		UIDs:            uids,
	})
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	summaries := imapCopyDestinationSummaries(results)
	if destMailbox.ID == state.selectedMailbox && len(summaries) > 0 {
		state.selectedMessages = imapSummariesExistsCount(state.selectedMessages, summaries)
		state.observeHighestModSeq(imapHighestSummaryModSeq(summaries))
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", state.selectedMessages)); err != nil {
			return false, err
		}
	}
	if copyUID := imapCopyUIDResponse(destMailbox, results); copyUID != "" {
		_, err = writer.WriteString(tag + " OK [" + copyUID + "] " + completionCommand + " completed\r\n")
		return false, err
	}
	_, err = writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) writeMoveResponse(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, destMailboxID MailboxID, completionCommand string) (bool, error) {
	if len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	destMailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, destMailboxID)
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	summaries, err := s.options.Backend.MoveMessages(context.Background(), MoveMessagesRequest{
		UserID:          state.session.UserID,
		SourceMailboxID: state.selectedMailbox,
		DestMailboxID:   destMailbox.ID,
		UIDs:            uids,
	})
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	copyUID := imapMoveCopyUIDResponse(destMailbox, summaries)
	if destMailbox.ID == state.selectedMailbox && len(summaries) > 0 {
		state.selectedMessages = imapSummariesExistsCount(state.selectedMessages, imapMoveDestinationSummaries(summaries))
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", state.selectedMessages)); err != nil {
			return false, err
		}
	}
	if highestModSeq := imapMoveHighestModSeq(summaries); highestModSeq > 0 {
		state.observeHighestModSeq(highestModSeq)
		if _, err := writer.WriteString(fmt.Sprintf("* OK [HIGHESTMODSEQ %d] %s source mod-sequence\r\n", highestModSeq, completionCommand)); err != nil {
			return false, err
		}
	}
	if copyUID != "" {
		if _, err := writer.WriteString("* OK [" + copyUID + "] " + completionCommand + " copied UIDs\r\n"); err != nil {
			return false, err
		}
	}
	return s.writeMovedExpungeResponses(writer, tag, state, imapMoveSourceSummaries(summaries), completionCommand, "")
}

func (s *Server) writeExpungeResponses(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, completionCommand string) (bool, error) {
	if uids != nil && len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	summaries, err := s.options.Backend.Expunge(context.Background(), ExpungeRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      uids,
	})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	return s.writeMovedExpungeResponses(writer, tag, state, summaries, completionCommand, "")
}

func (s *Server) writeMovedExpungeResponses(writer *bufio.Writer, tag string, state *imapConnState, summaries []MessageSummary, completionCommand string, responseCode string) (bool, error) {
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SequenceNumber < summaries[j].SequenceNumber
	})
	for i, summary := range summaries {
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return false, err
		}
		adjusted := sequenceNumber - uint32(i)
		if adjusted == 0 {
			adjusted = 1
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXPUNGE\r\n", adjusted)); err != nil {
			return false, err
		}
	}
	state.removeExpungedFromSavedSearch(summaries)
	if uint32(len(summaries)) >= state.selectedMessages {
		state.selectedMessages = 0
	} else {
		state.selectedMessages -= uint32(len(summaries))
	}
	if responseCode != "" {
		_, err := writer.WriteString(tag + " OK [" + responseCode + "] " + completionCommand + " completed\r\n")
		return false, err
	}
	_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (state *imapConnState) removeExpungedFromSavedSearch(summaries []MessageSummary) {
	if state == nil || len(state.savedSearch) == 0 || len(summaries) == 0 {
		return
	}
	expunged := make([]uint32, 0, len(summaries))
	for _, summary := range summaries {
		if summary.SequenceNumber > 0 {
			expunged = append(expunged, summary.SequenceNumber)
		}
	}
	sort.Slice(expunged, func(i, j int) bool {
		return expunged[i] < expunged[j]
	})
	for i, sequenceNumber := range expunged {
		adjusted := sequenceNumber - uint32(i)
		if adjusted == 0 {
			adjusted = 1
		}
		next := state.savedSearch[:0]
		for _, saved := range state.savedSearch {
			switch {
			case saved.sequenceNumber == adjusted:
				continue
			case saved.sequenceNumber > adjusted:
				saved.sequenceNumber--
			}
			next = append(next, saved)
		}
		state.savedSearch = next
	}
	if len(state.savedSearch) == 0 {
		state.savedSearch = nil
	}
}

func (state *imapConnState) observeHighestModSeq(modSeq uint64) {
	if state == nil || modSeq == 0 || modSeq <= state.selectedHighestModSeq {
		return
	}
	state.selectedHighestModSeq = modSeq
}

func (state *imapConnState) selectedSupportsPersistentModSeq() bool {
	return state != nil && state.selectedMailbox != "" && !state.selectedNoModSeq
}

func (s *Server) rejectSelectedNoModSeq(writer *bufio.Writer, tag string, state *imapConnState, command string) (bool, error) {
	if state != nil && !state.condstoreAware {
		state.condstoreAware = true
		if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
			return false, err
		}
	}
	_, err := writer.WriteString(tag + " BAD " + command + " requires persistent mod-sequences\r\n")
	return false, err
}

func (s *Server) uidsForSequenceNumbers(ctx context.Context, state *imapConnState, sequenceNumbers []uint32) ([]UID, error) {
	messages, err := s.options.Backend.ListMessages(ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		return nil, err
	}
	bySequence := make(map[uint32]UID, len(messages))
	for i, summary := range messages {
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		if summary.UID != 0 {
			bySequence[sequenceNumber] = summary.UID
		}
	}
	uids := make([]UID, 0, len(sequenceNumbers))
	for _, sequenceNumber := range sequenceNumbers {
		uid, ok := bySequence[sequenceNumber]
		if !ok {
			return nil, fmt.Errorf("sequence number %d not found", sequenceNumber)
		}
		uids = append(uids, uid)
	}
	return uids, nil
}

func imapCopyUIDResponse(destMailbox Mailbox, results []CopyMessageResult) string {
	if destMailbox.UIDNotSticky || destMailbox.UIDValidity == 0 || len(results) == 0 {
		return ""
	}
	sourceUIDs := make([]UID, 0, len(results))
	destUIDs := make([]UID, 0, len(results))
	for _, result := range results {
		if result.SourceUID == 0 || result.Destination.UID == 0 {
			return ""
		}
		sourceUIDs = append(sourceUIDs, result.SourceUID)
		destUIDs = append(destUIDs, result.Destination.UID)
	}
	return fmt.Sprintf("COPYUID %d %s %s", destMailbox.UIDValidity, imapUIDSetResponse(sourceUIDs), imapUIDSetResponse(destUIDs))
}

func imapMoveCopyUIDResponse(destMailbox Mailbox, results []MoveMessageResult) string {
	if destMailbox.UIDNotSticky || destMailbox.UIDValidity == 0 || len(results) == 0 {
		return ""
	}
	sourceUIDs := make([]UID, 0, len(results))
	destUIDs := make([]UID, 0, len(results))
	for _, result := range results {
		if result.Source.UID == 0 || result.Destination.UID == 0 {
			return ""
		}
		sourceUIDs = append(sourceUIDs, result.Source.UID)
		destUIDs = append(destUIDs, result.Destination.UID)
	}
	return fmt.Sprintf("COPYUID %d %s %s", destMailbox.UIDValidity, imapUIDSetResponse(sourceUIDs), imapUIDSetResponse(destUIDs))
}

func imapMoveSourceSummaries(results []MoveMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Source)
	}
	return summaries
}

func imapMoveDestinationSummaries(results []MoveMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Destination)
	}
	return summaries
}

func imapCopyDestinationSummaries(results []CopyMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Destination)
	}
	return summaries
}

func imapHighestSummaryModSeq(summaries []MessageSummary) uint64 {
	var highest uint64
	for _, summary := range summaries {
		if summary.ModSeq > highest {
			highest = summary.ModSeq
		}
	}
	return highest
}

func imapSummariesExistsCount(current uint32, summaries []MessageSummary) uint32 {
	maxSequence := current
	for _, summary := range summaries {
		if summary.SequenceNumber > maxSequence {
			maxSequence = summary.SequenceNumber
		}
	}
	if maxSequence > current {
		return maxSequence
	}
	return current + uint32(len(summaries))
}

func imapMoveHighestModSeq(results []MoveMessageResult) uint64 {
	var highest uint64
	for _, result := range results {
		if result.SourceHighestModSeq > highest {
			highest = result.SourceHighestModSeq
		}
	}
	return highest
}

func imapUIDSetResponse(uids []UID) string {
	parts := make([]string, 0, len(uids))
	for i := 0; i < len(uids); {
		start := uids[i]
		end := start
		j := i + 1
		for j < len(uids) && uids[j] == end+1 {
			end = uids[j]
			j++
		}
		if end > start {
			parts = append(parts, fmt.Sprintf("%d:%d", start, end))
		} else {
			parts = append(parts, strconv.FormatUint(uint64(start), 10))
		}
		i = j
	}
	return strings.Join(parts, ",")
}

func (s *Server) writeFetchResponses(writer *bufio.Writer, tag string, items []string, state *imapConnState, uids []UID, completionCommand string) (bool, error) {
	changedSince, requestsChangedSince, _ := imapFetchChangedSince(items)
	if message, ok := imapFetchDataItemsSyntaxError(items); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	items = imapExpandFetchItems(items)
	requestsBody := imapFetchRequestsBody(items)
	partial, requestsPartialBody := imapFetchPartialBody(items)
	partialSection, requestsPartialSection := imapFetchPartialSection(items)
	partRequest, requestsMIMEPart := imapFetchMIMEPartRequest(items)
	requestsHeader := imapFetchRequestsHeader(items)
	requestsText := imapFetchRequestsText(items)
	requestsPartText := imapFetchRequestsPartText(items)
	requestsPartMIME := imapFetchRequestsPartMIME(items)
	headerFields, requestsHeaderFields := imapFetchHeaderFields(items)
	headerFieldsNot, requestsHeaderFieldsNot := imapFetchHeaderFieldsNot(items)
	partialHeaderFields, requestsPartialHeaderFields := imapFetchPartialHeaderFields(items)
	partialHeaderFieldsNot, requestsPartialHeaderFieldsNot := imapFetchPartialHeaderFieldsNot(items)
	requestsEnvelope := imapFetchRequestsEnvelope(items)
	requestsInternalDate := imapFetchRequestsInternalDate(items)
	requestsModSeq := requestsChangedSince || imapFetchRequestsModSeq(items)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, completionCommand)
		}
		state.condstoreAware = true
	}
	requestsBodyAttribute := imapFetchRequestsBodyAttribute(items)
	requestsBodyStructure := imapFetchRequestsBodyStructure(items)
	setsSeen := imapFetchSetsSeen(items)
	for _, uid := range uids {
		fetchReq := FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		}
		message, err := s.options.Backend.FetchMessage(context.Background(), fetchReq)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
			return false, writeErr
		}
		summary := message.Summary
		if summary.UID == 0 {
			summary.UID = uid
		}
		if requestsChangedSince && summary.ModSeq <= changedSince {
			if message.Body != nil {
				if err := message.Body.Close(); err != nil {
					return false, err
				}
			}
			continue
		}
		requestsLiteral := requestsBody || requestsPartialBody || requestsPartialSection || requestsMIMEPart || requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME
		bodyAttribute := ""
		bodyStructure := ""
		if requestsBodyAttribute || requestsBodyStructure {
			structureMessage := message
			if requestsLiteral {
				var err error
				structureMessage, err = s.options.Backend.FetchMessage(context.Background(), fetchReq)
				if err != nil {
					structureMessage = Message{}
				}
			}
			if structureMessage.Body != nil {
				structure, err := messageparse.ParseMIMEStructure(structureMessage.Body, messageparse.MIMEStructureOptions{})
				if closeErr := structureMessage.Body.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err == nil {
					bodyAttribute = imapBodyFromMIMEStructure(summary, structure)
					bodyStructure = imapBodyStructureFromMIMEStructure(summary, structure)
				}
			}
			if bodyAttribute == "" {
				bodyAttribute = imapBody(summary)
			}
			if bodyStructure == "" {
				bodyStructure = imapBodyStructure(summary)
			}
			if !requestsLiteral {
				message.Body = nil
			}
		} else if !requestsLiteral && message.Body != nil {
			if err := message.Body.Close(); err != nil {
				return false, err
			}
			message.Body = nil
		}
		if !requestsLiteral {
			if message.Body != nil {
				if err := message.Body.Close(); err != nil {
					return false, err
				}
				message.Body = nil
			}
		}
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return false, err
		}
		if requestsLiteral {
			if message.Body == nil {
				_, err := writer.WriteString(tag + " NO " + completionCommand + " body is unavailable\r\n")
				return false, err
			}
			body := message.Body
			if summary.Size < 0 {
				_ = body.Close()
				_, err := writer.WriteString(tag + " NO " + completionCommand + " body size is unavailable\r\n")
				return false, err
			}
			if setsSeen {
				var err error
				summary, err = s.markFetchSeen(context.Background(), state, summary)
				if err != nil {
					_ = body.Close()
					_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
					return false, writeErr
				}
			}
			if requestsMIMEPart {
				literal, found, err := readIMAPMIMEPartLiteral(body, partRequest)
				if closeErr := body.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err != nil {
					return false, err
				}
				if !found {
					_, err := writer.WriteString(tag + " NO " + completionCommand + " body section is unavailable\r\n")
					return false, err
				}
				attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
				if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s BODY[%s]%s {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), partRequest.sectionName(), partRequest.partialSuffix(), len(literal))); err != nil {
					return false, err
				}
				if _, err := writer.Write(literal); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			if requestsPartialSection || requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME {
				wantHeader := requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || partialSection.headerLike()
				literal, err := readIMAPSectionLiteral(body, wantHeader)
				if err != nil {
					_ = body.Close()
					return false, err
				}
				if requestsPartMIME || partialSection.section == "1.MIME" {
					literal = []byte("\r\n")
				}
				if requestsHeaderFields {
					literal = filterIMAPHeaderFields(literal, headerFields, false)
				}
				if requestsHeaderFieldsNot {
					literal = filterIMAPHeaderFields(literal, headerFieldsNot, true)
				}
				if requestsPartialHeaderFields {
					literal = imapPartialLiteral(literal, partialHeaderFields)
				}
				if requestsPartialHeaderFieldsNot {
					literal = imapPartialLiteral(literal, partialHeaderFieldsNot)
				}
				if err := body.Close(); err != nil {
					return false, err
				}
				attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
				section := "TEXT"
				if requestsPartText {
					section = "1"
				}
				if requestsPartMIME {
					section = "1.MIME"
				}
				if requestsPartialSection {
					section = partialSection.section
					literal = imapPartialLiteral(literal, partialSection.partial)
				}
				if requestsHeader {
					section = "HEADER"
				}
				if requestsHeaderFields {
					section = imapHeaderFieldsSectionName("HEADER.FIELDS", headerFields)
				}
				if requestsHeaderFieldsNot {
					section = imapHeaderFieldsSectionName("HEADER.FIELDS.NOT", headerFieldsNot)
				}
				partialSuffix := ""
				if requestsPartialSection {
					partialSuffix = fmt.Sprintf("<%d>", partialSection.partial.offset)
				}
				if requestsPartialHeaderFields {
					partialSuffix = fmt.Sprintf("<%d>", partialHeaderFields.offset)
				}
				if requestsPartialHeaderFieldsNot {
					partialSuffix = fmt.Sprintf("<%d>", partialHeaderFieldsNot.offset)
				}
				itemName := imapSectionLiteralResponseName(items, section)
				if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s %s%s {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), itemName, partialSuffix, len(literal))); err != nil {
					return false, err
				}
				if _, err := writer.Write(literal); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
			if requestsPartialBody {
				itemName := imapPartialBodyLiteralResponseName(items)
				count := partial.count
				if partial.offset >= uint64(summary.Size) {
					count = 0
				} else if remaining := uint64(summary.Size) - partial.offset; count > remaining {
					count = remaining
				}
				if _, err := io.CopyN(io.Discard, body, int64(partial.offset)); err != nil && !errors.Is(err, io.EOF) {
					_ = body.Close()
					return false, err
				}
				if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s %s<%d> {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), itemName, partial.offset, count)); err != nil {
					_ = body.Close()
					return false, err
				}
				if count > 0 {
					if _, err := io.CopyN(writer, body, int64(count)); err != nil {
						_ = body.Close()
						return false, err
					}
				}
				if err := body.Close(); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s %s {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), imapFullBodyLiteralResponseName(items), summary.Size)); err != nil {
				_ = body.Close()
				return false, err
			}
			if _, err := io.CopyN(writer, body, summary.Size); err != nil {
				_ = body.Close()
				return false, err
			}
			if err := body.Close(); err != nil {
				return false, err
			}
			if _, err := writer.WriteString(")\r\n"); err != nil {
				return false, err
			}
			continue
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s)\r\n", sequenceNumber, strings.Join(imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure), " "))); err != nil {
			return false, err
		}
	}
	_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) markFetchSeen(ctx context.Context, state *imapConnState, summary MessageSummary) (MessageSummary, error) {
	if state == nil || state.readOnly || summary.Flags.Read || summary.UID == 0 {
		return summary, nil
	}
	if s == nil || s.options.Backend == nil {
		return summary, fmt.Errorf("imap backend is required")
	}
	updated, err := s.options.Backend.StoreFlags(ctx, StoreFlagsRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      []UID{summary.UID},
		Flags:     MessageFlags{Read: true},
		Mode:      StoreFlagsAdd,
	})
	if err != nil {
		return summary, err
	}
	summary.Flags.Read = true
	for _, item := range updated {
		if item.UID != summary.UID {
			continue
		}
		if item.ModSeq > summary.ModSeq {
			summary.ModSeq = item.ModSeq
		}
		if item.SequenceNumber != 0 {
			summary.SequenceNumber = item.SequenceNumber
		}
		break
	}
	return summary, nil
}

const maxIMAPExpandedSetItems = 10000

func parseIMAPUIDSet(value string) ([]UID, bool) {
	if strings.TrimSpace(value) != value {
		return nil, false
	}
	seen := make(map[UID]struct{})
	uids := make([]UID, 0, 1)
	for _, rawPart := range strings.Split(value, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" || part != rawPart || strings.Contains(part, "*") {
			return nil, false
		}
		startText, endText, hasRange := strings.Cut(part, ":")
		start, ok := parseIMAPUIDSetNumber(startText)
		if !ok {
			return nil, false
		}
		end := start
		if hasRange {
			end, ok = parseIMAPUIDSetNumber(endText)
			if !ok {
				return nil, false
			}
		}
		if start > end {
			start, end = end, start
		}
		for uid := start; uid <= end; uid++ {
			if _, ok := seen[uid]; ok {
				continue
			}
			seen[uid] = struct{}{}
			uids = append(uids, uid)
			if len(uids) > maxIMAPExpandedSetItems {
				return nil, false
			}
			if uid == UID(^uint32(0)) {
				break
			}
		}
	}
	return uids, len(uids) > 0
}

func parseIMAPUIDSetForState(value string, state *imapConnState) ([]UID, bool) {
	if value != "$" {
		return parseIMAPUIDSet(value)
	}
	uids := imapSavedSearchUIDs(state)
	return uids, true
}

type imapUIDRange struct {
	start UID
	end   UID
}

func (s *Server) uidsForUIDSet(ctx context.Context, state *imapConnState, value string) ([]UID, bool, error) {
	if value == "$" {
		uids := imapSavedSearchUIDs(state)
		return uids, true, nil
	}
	if !strings.ContainsAny(value, ":*,") || s == nil || s.options.Backend == nil || state == nil || state.session == nil || state.selectedMailbox == "" {
		uids, ok := parseIMAPUIDSet(value)
		return uids, ok, nil
	}
	messages, err := s.options.Backend.ListMessages(ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		return nil, false, err
	}
	var maxUID UID
	for _, message := range messages {
		if message.UID > maxUID {
			maxUID = message.UID
		}
	}
	ranges, ok := parseIMAPUIDSetRanges(value, maxUID)
	if !ok {
		return nil, false, nil
	}
	uids, ok := imapUIDsMatchingRanges(messages, ranges)
	return uids, ok, nil
}

func parseIMAPUIDSetRanges(value string, maxUID UID) ([]imapUIDRange, bool) {
	if strings.TrimSpace(value) != value {
		return nil, false
	}
	ranges := make([]imapUIDRange, 0, 1)
	for _, rawPart := range strings.Split(value, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" || part != rawPart {
			return nil, false
		}
		startText, endText, hasRange := strings.Cut(part, ":")
		start, ok := parseIMAPUIDSetRangeNumber(startText, maxUID)
		if !ok {
			return nil, false
		}
		end := start
		if hasRange {
			end, ok = parseIMAPUIDSetRangeNumber(endText, maxUID)
			if !ok {
				return nil, false
			}
		}
		if start > end {
			start, end = end, start
		}
		ranges = append(ranges, imapUIDRange{start: start, end: end})
	}
	return ranges, len(ranges) > 0
}

func parseIMAPUIDSetRangeNumber(value string, maxUID UID) (UID, bool) {
	if value == "*" {
		return maxUID, true
	}
	return parseIMAPUIDSetNumber(value)
}

func imapUIDsMatchingRanges(messages []MessageSummary, ranges []imapUIDRange) ([]UID, bool) {
	seen := make(map[UID]struct{})
	uids := make([]UID, 0, len(messages))
	for _, message := range messages {
		if message.UID == 0 {
			continue
		}
		if imapUIDMatchesRanges(message.UID, ranges) {
			if _, ok := seen[message.UID]; ok {
				continue
			}
			seen[message.UID] = struct{}{}
			uids = append(uids, message.UID)
			if len(uids) > maxIMAPExpandedSetItems {
				return nil, false
			}
		}
	}
	return uids, true
}

func imapUIDMatchesRanges(uid UID, ranges []imapUIDRange) bool {
	if uid == 0 {
		return false
	}
	for _, uidRange := range ranges {
		if uid >= uidRange.start && uid <= uidRange.end {
			return true
		}
	}
	return false
}

func imapSavedSearchUIDs(state *imapConnState) []UID {
	if state == nil || len(state.savedSearch) == 0 {
		return nil
	}
	seen := make(map[UID]struct{}, len(state.savedSearch))
	uids := make([]UID, 0, len(state.savedSearch))
	for _, saved := range state.savedSearch {
		if saved.uid == 0 {
			continue
		}
		if _, ok := seen[saved.uid]; ok {
			continue
		}
		seen[saved.uid] = struct{}{}
		uids = append(uids, saved.uid)
	}
	return uids
}

func parseIMAPUIDSetNumber(value string) (UID, bool) {
	if !imapNZNumberAtomDigitsOnly(value) {
		return 0, false
	}
	uid64, err := strconv.ParseUint(value, 10, 32)
	if err != nil || uid64 == 0 {
		return 0, false
	}
	return UID(uid64), true
}

func imapUIDSetSyntaxValid(value string) bool {
	return imapSetSyntaxValid(value, true, true)
}

func imapSequenceSetSyntaxValid(value string) bool {
	return imapSetSyntaxValid(value, true, true)
}

func imapSetSyntaxValid(value string, allowStar bool, allowDollar bool) bool {
	if strings.TrimSpace(value) != value {
		return false
	}
	if value == "" {
		return false
	}
	if value == "$" {
		return allowDollar
	}
	for _, rawPart := range strings.Split(value, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" || part != rawPart {
			return false
		}
		startText, endText, hasRange := strings.Cut(part, ":")
		if !imapSetSyntaxNumberValid(startText, allowStar) {
			return false
		}
		if hasRange && !imapSetSyntaxNumberValid(endText, allowStar) {
			return false
		}
	}
	return true
}

func imapSetSyntaxNumberValid(value string, allowStar bool) bool {
	if value == "*" {
		return allowStar
	}
	_, ok := parseIMAPUIDSetNumber(value)
	return ok
}

func parseIMAPSequenceSet(value string, maxSequence uint32) ([]uint32, bool) {
	if maxSequence == 0 {
		return nil, false
	}
	uids, ok := parseIMAPBoundedNumberSet(value, maxSequence, true)
	if !ok {
		return nil, false
	}
	out := make([]uint32, len(uids))
	for i, uid := range uids {
		out[i] = uint32(uid)
	}
	return out, true
}

func parseIMAPSequenceSetForState(value string, maxSequence uint32, state *imapConnState) ([]uint32, bool) {
	if value != "$" {
		return parseIMAPSequenceSet(value, maxSequence)
	}
	sequenceNumbers := imapSavedSearchSequenceNumbers(state, maxSequence)
	return sequenceNumbers, true
}

func imapSavedSearchSequenceNumbers(state *imapConnState, maxSequence uint32) []uint32 {
	if state == nil || len(state.savedSearch) == 0 {
		return nil
	}
	seen := make(map[uint32]struct{}, len(state.savedSearch))
	sequenceNumbers := make([]uint32, 0, len(state.savedSearch))
	for _, saved := range state.savedSearch {
		if saved.sequenceNumber == 0 || saved.sequenceNumber > maxSequence {
			continue
		}
		if _, ok := seen[saved.sequenceNumber]; ok {
			continue
		}
		seen[saved.sequenceNumber] = struct{}{}
		sequenceNumbers = append(sequenceNumbers, saved.sequenceNumber)
	}
	return sequenceNumbers
}

func parseIMAPBoundedNumberSet(value string, maxValue uint32, allowStar bool) ([]UID, bool) {
	if strings.TrimSpace(value) != value {
		return nil, false
	}
	seen := make(map[UID]struct{})
	values := make([]UID, 0, 1)
	for _, rawPart := range strings.Split(value, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" || part != rawPart {
			return nil, false
		}
		startText, endText, hasRange := strings.Cut(part, ":")
		start, ok := parseIMAPSetNumber(startText, maxValue, allowStar)
		if !ok {
			return nil, false
		}
		end := start
		if hasRange {
			end, ok = parseIMAPSetNumber(endText, maxValue, allowStar)
			if !ok {
				return nil, false
			}
		}
		if start > end {
			start, end = end, start
		}
		for value := start; value <= end; value++ {
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			values = append(values, value)
			if len(values) > maxIMAPExpandedSetItems {
				return nil, false
			}
			if value == UID(maxValue) {
				break
			}
		}
	}
	return values, len(values) > 0
}

func parseIMAPSetNumber(value string, maxValue uint32, allowStar bool) (UID, bool) {
	if value == "*" {
		if allowStar && maxValue > 0 {
			return UID(maxValue), true
		}
		return 0, false
	}
	parsed, ok := parseIMAPUIDSetNumber(value)
	if !ok || parsed > UID(maxValue) {
		return 0, false
	}
	return parsed, true
}

func imapFetchRequestsBody(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[]" || token == "BODY.PEEK[]" || token == "RFC822" {
			return true
		}
	}
	return false
}

func imapFetchSetsSeen(items []string) bool {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			switch {
			case token == "RFC822" || strings.HasPrefix(token, "RFC822<") || token == "RFC822.TEXT" || strings.HasPrefix(token, "RFC822.TEXT<"):
				return true
			case token == "RFC822.HEADER" || strings.HasPrefix(token, "RFC822.HEADER<"):
				continue
			case strings.HasPrefix(token, "BODY.PEEK["):
				continue
			case strings.HasPrefix(token, "BODY["):
				return true
			}
		}
	}
	return false
}

func imapFullBodyLiteralResponseName(items []string) string {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if token == "RFC822" {
				return "RFC822"
			}
		}
	}
	return "BODY[]"
}

func imapPartialBodyLiteralResponseName(items []string) string {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if strings.HasPrefix(token, "RFC822<") {
				return "RFC822"
			}
		}
	}
	return "BODY[]"
}

func imapSectionLiteralResponseName(items []string, section string) string {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if section == "HEADER" && (token == "RFC822.HEADER" || strings.HasPrefix(token, "RFC822.HEADER<")) {
				return "RFC822.HEADER"
			}
			if section == "TEXT" && (token == "RFC822.TEXT" || strings.HasPrefix(token, "RFC822.TEXT<")) {
				return "RFC822.TEXT"
			}
		}
	}
	return "BODY[" + section + "]"
}

func imapHeaderFieldsSectionName(marker string, fields []string) string {
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ToUpper(strings.TrimSpace(field))
		if field != "" {
			normalized = append(normalized, field)
		}
	}
	return marker + " (" + strings.Join(normalized, " ") + ")"
}

func imapExpandFetchItems(items []string) []string {
	expanded := make([]string, 0, len(items)+4)
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		switch token {
		case "FAST":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE")
		case "ALL":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE")
		case "FULL":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE", "BODY")
		default:
			expanded = append(expanded, item)
		}
	}
	return expanded
}

type imapPartialBodyRequest struct {
	offset uint64
	count  uint64
}

type imapPartialSectionRequest struct {
	section string
	partial imapPartialBodyRequest
}

type imapMIMEPartRequest struct {
	path                []int
	mime                bool
	messageSection      string
	messageHeaderFields []string
	messageHeaderNot    bool
	partial             imapPartialBodyRequest
}

const maxIMAPMIMEPartPathDepth = 32

func (r imapMIMEPartRequest) sectionName() string {
	parts := make([]string, 0, len(r.path)+1)
	for _, value := range r.path {
		parts = append(parts, strconv.Itoa(value))
	}
	if r.mime {
		parts = append(parts, "MIME")
	}
	if r.messageSection != "" {
		parts = append(parts, r.messageSection)
		if strings.HasPrefix(r.messageSection, "HEADER.FIELDS") {
			parts[len(parts)-1] += " (" + strings.Join(r.messageHeaderFields, " ") + ")"
		}
	}
	return strings.Join(parts, ".")
}

func (r imapMIMEPartRequest) partialSuffix() string {
	if r.partial.count == 0 {
		return ""
	}
	return fmt.Sprintf("<%d>", r.partial.offset)
}

func (r imapPartialSectionRequest) headerLike() bool {
	return r.section == "HEADER" || r.section == "1.MIME"
}

func imapFetchPartialBody(items []string) (imapPartialBodyRequest, bool) {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if !strings.HasPrefix(token, "BODY[]<") && !strings.HasPrefix(token, "BODY.PEEK[]<") && !strings.HasPrefix(token, "RFC822<") {
				continue
			}
			return imapParsePartialBodyToken(token)
		}
	}
	return imapPartialBodyRequest{}, false
}

func imapFetchPartialSection(items []string) (imapPartialSectionRequest, bool) {
	sections := []struct {
		prefixes []string
		section  string
	}{
		{[]string{"BODY[HEADER]<", "BODY.PEEK[HEADER]<", "RFC822.HEADER<"}, "HEADER"},
		{[]string{"BODY[TEXT]<", "BODY.PEEK[TEXT]<", "RFC822.TEXT<"}, "TEXT"},
		{[]string{"BODY[1]<", "BODY.PEEK[1]<"}, "1"},
		{[]string{"BODY[1.MIME]<", "BODY.PEEK[1.MIME]<"}, "1.MIME"},
	}
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			for _, candidate := range sections {
				for _, prefix := range candidate.prefixes {
					if !strings.HasPrefix(token, prefix) {
						continue
					}
					partial, ok := imapParsePartialBodyToken(token)
					if !ok {
						return imapPartialSectionRequest{}, false
					}
					return imapPartialSectionRequest{section: candidate.section, partial: partial}, true
				}
			}
		}
	}
	return imapPartialSectionRequest{}, false
}

func imapFetchMIMEPartRequest(items []string) (imapMIMEPartRequest, bool) {
	if req, ok := imapParseMIMEPartHeaderFieldsRequest(items); ok {
		return req, true
	}
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			req, ok := imapParseMIMEPartRequestToken(token)
			if ok {
				return req, true
			}
		}
	}
	return imapMIMEPartRequest{}, false
}

func imapParseMIMEPartHeaderFieldsRequest(items []string) (imapMIMEPartRequest, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
		idx := strings.Index(joined, "."+marker)
		if idx < 0 {
			continue
		}
		openIdx := strings.LastIndex(joined[:idx], "BODY[")
		if peekIdx := strings.LastIndex(joined[:idx], "BODY.PEEK["); peekIdx > openIdx {
			openIdx = peekIdx
		}
		if openIdx < 0 {
			return imapMIMEPartRequest{}, false
		}
		pathText := joined[openIdx:idx]
		pathText = strings.TrimPrefix(pathText, "BODY.PEEK[")
		pathText = strings.TrimPrefix(pathText, "BODY[")
		path, ok := parseIMAPMIMEPartPath(pathText)
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		fieldsStart := strings.Index(joined[idx+len(marker)+1:], "(")
		if fieldsStart < 0 {
			return imapMIMEPartRequest{}, false
		}
		fieldsStart += idx + len(marker) + 1
		fieldsEnd := strings.Index(joined[fieldsStart+1:], ")")
		if fieldsEnd < 0 {
			return imapMIMEPartRequest{}, false
		}
		fieldsEnd += fieldsStart + 1
		fields, ok := imapHeaderFieldListNames(joined[fieldsStart+1 : fieldsEnd])
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		req := imapMIMEPartRequest{
			path:                path,
			messageSection:      marker,
			messageHeaderFields: fields,
			messageHeaderNot:    marker == "HEADER.FIELDS.NOT",
		}
		suffix := strings.TrimSpace(joined[fieldsEnd+1:])
		suffix = strings.TrimPrefix(suffix, "]")
		if strings.HasPrefix(suffix, "<") {
			partial, ok := imapParsePartialBodyToken(suffix)
			if !ok {
				return imapMIMEPartRequest{}, false
			}
			req.partial = partial
		}
		return req, true
	}
	return imapMIMEPartRequest{}, false
}

func imapParseMIMEPartRequestToken(token string) (imapMIMEPartRequest, bool) {
	if strings.HasPrefix(token, "BODY.PEEK[") {
		token = "BODY[" + strings.TrimPrefix(token, "BODY.PEEK[")
	}
	if !strings.HasPrefix(token, "BODY[") {
		return imapMIMEPartRequest{}, false
	}
	closeIdx := strings.Index(token, "]")
	if closeIdx < 0 {
		return imapMIMEPartRequest{}, false
	}
	section := token[len("BODY["):closeIdx]
	if section == "" || section == "HEADER" || section == "TEXT" || strings.HasPrefix(section, "HEADER.") {
		return imapMIMEPartRequest{}, false
	}
	parts := strings.Split(section, ".")
	mimeSection := false
	if parts[len(parts)-1] == "MIME" {
		mimeSection = true
		parts = parts[:len(parts)-1]
	}
	messageSection := ""
	if !mimeSection && (parts[len(parts)-1] == "HEADER" || parts[len(parts)-1] == "TEXT") {
		messageSection = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return imapMIMEPartRequest{}, false
	}
	if len(parts) > maxIMAPMIMEPartPathDepth {
		return imapMIMEPartRequest{}, false
	}
	path, ok := parseIMAPMIMEPartPath(strings.Join(parts, "."))
	if !ok {
		return imapMIMEPartRequest{}, false
	}
	req := imapMIMEPartRequest{path: path, mime: mimeSection, messageSection: messageSection}
	if suffix := token[closeIdx+1:]; suffix != "" {
		if !strings.HasPrefix(suffix, "<") {
			return imapMIMEPartRequest{}, false
		}
		partial, ok := imapParsePartialBodyToken(token)
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		req.partial = partial
	}
	return req, true
}

func parseIMAPMIMEPartPath(value string) ([]int, bool) {
	if strings.TrimSpace(value) != value {
		return nil, false
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > maxIMAPMIMEPartPathDepth {
		return nil, false
	}
	path := make([]int, 0, len(parts))
	for _, part := range parts {
		if !imapNZNumberAtomDigitsOnly(part) {
			return nil, false
		}
		number, err := strconv.ParseUint(part, 10, 32)
		if err != nil || number == 0 {
			return nil, false
		}
		if strconv.IntSize == 32 && number > uint64(int(^uint(0)>>1)) {
			return nil, false
		}
		path = append(path, int(number))
	}
	return path, true
}

func imapParsePartialBodyToken(token string) (imapPartialBodyRequest, bool) {
	start := strings.Index(token, "<")
	end := strings.LastIndex(token, ">")
	if start < 0 || end <= start || end != len(token)-1 {
		return imapPartialBodyRequest{}, false
	}
	offsetText, countText, ok := strings.Cut(token[start+1:end], ".")
	if !ok {
		return imapPartialBodyRequest{}, false
	}
	if !imapNumberAtomRFC3501(offsetText) || !imapNZNumberAtomDigitsOnly(countText) {
		return imapPartialBodyRequest{}, false
	}
	offset, err := strconv.ParseUint(offsetText, 10, 32)
	if err != nil {
		return imapPartialBodyRequest{}, false
	}
	count, err := strconv.ParseUint(countText, 10, 32)
	if err != nil || count == 0 {
		return imapPartialBodyRequest{}, false
	}
	return imapPartialBodyRequest{offset: offset, count: count}, true
}

func imapPartialLiteral(literal []byte, partial imapPartialBodyRequest) []byte {
	if partial.offset >= uint64(len(literal)) {
		return nil
	}
	end := partial.offset + partial.count
	if end > uint64(len(literal)) {
		end = uint64(len(literal))
	}
	return literal[partial.offset:end]
}

func readIMAPMIMEPartLiteral(reader io.Reader, req imapMIMEPartRequest) ([]byte, bool, error) {
	if reader == nil {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		if len(req.path) == 1 && req.path[0] == 1 && !req.mime {
			if req.partial.count > 0 {
				data = imapPartialLiteral(data, req.partial)
			}
			return data, true, nil
		}
		return nil, false, nil
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		if mediaType == "message/rfc822" && len(req.path) > 1 && req.path[0] == 1 {
			literal, found, err := readIMAPMIMEPartLiteralFromMessage(message.Body, req.path[1:], req)
			if err != nil || !found {
				return nil, found, err
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		if req.messageSection != "" && len(req.path) == 1 && req.path[0] == 1 && mediaType == "message/rfc822" {
			literal, found, err := readIMAPMIMEPartLiteralFromMessage(message.Body, nil, req)
			if err != nil || !found {
				return nil, false, err
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		if len(req.path) == 1 && req.path[0] == 1 && req.mime {
			return []byte("\r\n"), true, nil
		}
		if len(req.path) == 1 && req.path[0] == 1 && !req.mime {
			literal, err := io.ReadAll(io.LimitReader(message.Body, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		return nil, false, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, false, nil
	}
	literal, found, err := readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(message.Body, boundary), req.path, req)
	if err != nil || !found {
		return nil, found, err
	}
	if req.partial.count > 0 {
		literal = imapPartialLiteral(literal, req.partial)
	}
	return literal, true, nil
}

func readIMAPMIMEPartLiteralFromMultipart(reader *multipart.Reader, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	if len(path) == 0 {
		return nil, false, nil
	}
	for i := 1; ; i++ {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if i != path[0] {
			_ = part.Close()
			continue
		}
		defer part.Close()
		if len(path) == 1 {
			if req.messageSection != "" {
				mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
				if err != nil || strings.ToLower(mediaType) != "message/rfc822" {
					return nil, false, nil
				}
				literal, found, err := readIMAPMIMEPartLiteralFromMessage(part, nil, req)
				if err != nil || !found {
					return nil, false, err
				}
				return literal, true, nil
			}
			if req.mime {
				return imapMIMEHeaderLiteral(part.Header), true, nil
			}
			literal, err := io.ReadAll(io.LimitReader(part, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			return literal, true, nil
		}
		mediaType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		mediaType = strings.ToLower(mediaType)
		if err == nil && mediaType == "message/rfc822" {
			return readIMAPMIMEPartLiteralFromMessage(part, path[1:], req)
		}
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			return nil, false, nil
		}
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			return nil, false, nil
		}
		return readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(part, boundary), path[1:], req)
	}
}

func readIMAPMIMEPartLiteralFromMessage(reader io.Reader, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		return nil, false, fmt.Errorf("imap message/rfc822 literal exceeds limit")
	}
	if req.messageSection != "" {
		if len(path) != 0 {
			return nil, false, nil
		}
		return readIMAPRawMessageSectionLiteral(data, req), true, nil
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return readIMAPMalformedMessageLiteral(data, path, req)
	}
	if len(path) == 0 {
		return nil, false, nil
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		if len(path) == 1 && path[0] == 1 && req.mime {
			return []byte("\r\n"), true, nil
		}
		if len(path) == 1 && path[0] == 1 && !req.mime {
			literal, err := io.ReadAll(io.LimitReader(message.Body, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			return literal, true, nil
		}
		return nil, false, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, false, nil
	}
	return readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(message.Body, boundary), path, imapMIMEPartRequest{mime: req.mime})
}

func readIMAPRawMessageSectionLiteral(data []byte, req imapMIMEPartRequest) []byte {
	end := imapHeaderEnd(data)
	if end < 0 {
		if req.messageSection == "TEXT" {
			return data
		}
		return []byte("\r\n")
	}
	if req.messageSection == "TEXT" {
		return data[end:]
	}
	header := data[:end]
	if strings.HasPrefix(req.messageSection, "HEADER.FIELDS") {
		header = filterIMAPHeaderFields(header, req.messageHeaderFields, req.messageHeaderNot)
	}
	return header
}

func readIMAPMalformedMessageLiteral(data []byte, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	if req.messageSection != "" {
		if len(path) != 0 {
			return nil, false, nil
		}
		if req.messageSection == "TEXT" {
			return data, true, nil
		}
		return []byte("\r\n"), true, nil
	}
	if len(path) == 1 && path[0] == 1 {
		if req.mime {
			return []byte("\r\n"), true, nil
		}
		return data, true, nil
	}
	return nil, false, nil
}

func readIMAPMessageSectionLiteral(reader io.Reader, req imapMIMEPartRequest) ([]byte, error) {
	literal, err := readIMAPSectionLiteral(reader, req.messageSection != "TEXT")
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(req.messageSection, "HEADER.FIELDS") {
		literal = filterIMAPHeaderFields(literal, req.messageHeaderFields, req.messageHeaderNot)
	}
	return literal, nil
}

func imapMIMEHeaderLiteral(header textproto.MIMEHeader) []byte {
	if len(header) == 0 {
		return []byte("\r\n")
	}
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		for _, value := range header[key] {
			out.WriteString(key)
			out.WriteString(": ")
			out.WriteString(value)
			out.WriteString("\r\n")
		}
	}
	out.WriteString("\r\n")
	return []byte(out.String())
}

func imapFetchRequestsHeader(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[HEADER]" || token == "BODY.PEEK[HEADER]" || token == "RFC822.HEADER" {
			return true
		}
	}
	return false
}

func imapFetchHeaderFields(items []string) ([]string, bool) {
	return imapFetchHeaderFieldList(items, "HEADER.FIELDS")
}

func imapFetchHeaderFieldsNot(items []string) ([]string, bool) {
	return imapFetchHeaderFieldList(items, "HEADER.FIELDS.NOT")
}

func imapFetchPartialHeaderFields(items []string) (imapPartialBodyRequest, bool) {
	return imapFetchPartialHeaderFieldList(items, "HEADER.FIELDS")
}

func imapFetchPartialHeaderFieldsNot(items []string) (imapPartialBodyRequest, bool) {
	return imapFetchPartialHeaderFieldList(items, "HEADER.FIELDS.NOT")
}

func imapFetchPartialHeaderFieldList(items []string, marker string) (imapPartialBodyRequest, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	idx := strings.Index(joined, marker)
	if idx < 0 {
		return imapPartialBodyRequest{}, false
	}
	if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
		return imapPartialBodyRequest{}, false
	}
	start := strings.Index(joined[idx:], "(")
	if start < 0 {
		return imapPartialBodyRequest{}, false
	}
	end := strings.Index(joined[idx+start+1:], ")")
	if end < 0 {
		return imapPartialBodyRequest{}, false
	}
	suffix := strings.TrimSpace(joined[idx+start+1+end+1:])
	suffix = strings.TrimPrefix(suffix, "]")
	if !strings.HasPrefix(suffix, "<") {
		return imapPartialBodyRequest{}, false
	}
	return imapParsePartialBodyToken(suffix)
}

func imapFetchHeaderFieldList(items []string, marker string) ([]string, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	idx := strings.Index(joined, marker)
	if idx < 0 {
		return nil, false
	}
	if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
		return nil, false
	}
	start := strings.Index(joined[idx:], "(")
	if start < 0 {
		return nil, false
	}
	end := strings.Index(joined[idx+start+1:], ")")
	if end < 0 {
		return nil, false
	}
	return imapHeaderFieldListNames(joined[idx+start+1 : idx+start+1+end])
}

func imapFetchHeaderFieldListsValid(items []string) bool {
	joined := strings.ToUpper(strings.Join(items, " "))
	for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
		offset := 0
		for {
			idx := strings.Index(joined[offset:], marker)
			if idx < 0 {
				break
			}
			idx += offset
			if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
				offset = idx + len(marker)
				continue
			}
			start := strings.Index(joined[idx:], "(")
			if start < 0 {
				return false
			}
			end := strings.Index(joined[idx+start+1:], ")")
			if end < 0 {
				return false
			}
			if _, ok := imapHeaderFieldListNames(joined[idx+start+1 : idx+start+1+end]); !ok {
				return false
			}
			offset = idx + start + 1 + end + 1
		}
	}
	return true
}

func imapHeaderFieldListNames(fieldsText string) ([]string, bool) {
	if fieldsText == "" {
		return nil, true
	}
	if strings.TrimSpace(fieldsText) != fieldsText {
		return nil, false
	}
	fields := strings.Split(fieldsText, " ")
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if !imapHeaderFieldNameValid(field) {
			return nil, false
		}
		names = append(names, field)
	}
	return names, true
}

func imapHeaderFieldNameValid(field string) bool {
	if field == "" {
		return false
	}
	for i := 0; i < len(field); i++ {
		c := field[i]
		switch c {
		case '(', ')', '{', '%', '*', '"', '\\', ']', ':':
			return false
		default:
			if c <= 0x20 || c >= 0x7f {
				return false
			}
		}
	}
	return true
}

func imapSearchHeaderFieldNameValid(field string) bool {
	return imapHeaderFieldNameValid(strings.ToUpper(field))
}

func filterIMAPHeaderFields(header []byte, fields []string, exclude bool) []byte {
	if len(header) == 0 {
		return []byte("\r\n")
	}
	if len(fields) == 0 {
		if exclude {
			return header
		}
		return []byte("\r\n")
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[strings.ToUpper(field)] = struct{}{}
	}
	lines := strings.SplitAfter(string(header), "\n")
	var out strings.Builder
	include := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if include {
				out.WriteString(line)
			}
			continue
		}
		name, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			include = false
			continue
		}
		_, found := allowed[strings.ToUpper(name)]
		include = found
		if exclude {
			include = !found
		}
		if include {
			out.WriteString(line)
		}
	}
	out.WriteString("\r\n")
	return []byte(out.String())
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func imapFetchRequestsText(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[TEXT]" || token == "BODY.PEEK[TEXT]" || token == "RFC822.TEXT" {
			return true
		}
	}
	return false
}

func imapFetchRequestsPartText(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[1]" || token == "BODY.PEEK[1]" {
			return true
		}
	}
	return false
}

func imapFetchRequestsPartMIME(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[1.MIME]" || token == "BODY.PEEK[1.MIME]" {
			return true
		}
	}
	return false
}

func readIMAPSectionLiteral(reader io.Reader, wantHeader bool) ([]byte, error) {
	const maxHeaderBytes = 1 << 20

	var data []byte
	buffer := make([]byte, 4096)
	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			data = append(data, buffer[:n]...)
			if len(data) > maxHeaderBytes {
				return nil, fmt.Errorf("imap header literal exceeds limit")
			}
			if end := imapHeaderEnd(data); end >= 0 {
				if wantHeader {
					return data[:end], nil
				}
				return readRemainingIMAPSectionText(data[end:], reader)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if wantHeader {
					return data, nil
				}
				return nil, nil
			}
			return nil, err
		}
	}
}

func readRemainingIMAPSectionText(prefix []byte, reader io.Reader) ([]byte, error) {
	if len(prefix) > maxIMAPSearchLiteralBytes {
		return nil, fmt.Errorf("imap text literal exceeds limit")
	}
	remainingLimit := maxIMAPSearchLiteralBytes - len(prefix)
	rest, err := io.ReadAll(io.LimitReader(reader, int64(remainingLimit)+1))
	if err != nil {
		return nil, err
	}
	if len(rest) > remainingLimit {
		return nil, fmt.Errorf("imap text literal exceeds limit")
	}
	return append(prefix, rest...), nil
}

func imapHeaderEnd(value []byte) int {
	if idx := bytes.Index(value, []byte("\r\n\r\n")); idx >= 0 {
		return idx + 4
	}
	if idx := bytes.Index(value, []byte("\n\n")); idx >= 0 {
		return idx + 2
	}
	return -1
}

func imapFetchRequestsEnvelope(items []string) bool {
	return imapFetchRequestsToken(items, "ENVELOPE")
}

func imapFetchRequestsInternalDate(items []string) bool {
	return imapFetchRequestsToken(items, "INTERNALDATE")
}

func imapFetchRequestsModSeq(items []string) bool {
	return imapFetchRequestsToken(items, "MODSEQ")
}

func imapFetchChangedSince(items []string) (uint64, bool, bool) {
	var threshold uint64
	found := false
	for i := 0; i < len(items); i++ {
		token := strings.ToUpper(strings.TrimSpace(items[i]))
		if !strings.Contains(token, "CHANGEDSINCE") {
			continue
		}
		if found || token != "(CHANGEDSINCE" || i+1 >= len(items) {
			return 0, false, false
		}
		valueToken := items[i+1]
		if !strings.HasSuffix(valueToken, ")") || strings.HasSuffix(valueToken, "))") {
			return 0, false, false
		}
		value := strings.TrimSuffix(valueToken, ")")
		modseq, ok := parseIMAPModSeqValue(value)
		if !ok {
			return 0, false, false
		}
		threshold = modseq
		found = true
		i++
	}
	return threshold, found, true
}

func imapFetchDataItemParenthesesValid(items []string) bool {
	for _, item := range items {
		token := strings.TrimSpace(item)
		if strings.HasPrefix(token, "((") || strings.HasSuffix(token, "))") {
			return false
		}
	}
	return true
}

func imapFetchDataItemsSyntaxError(items []string) (string, bool) {
	if _, _, ok := imapFetchChangedSince(items); !ok {
		return "FETCH CHANGEDSINCE modifier is invalid", true
	}
	if !imapFetchDataItemOuterWhitespaceValid(items) {
		return "FETCH data item list is invalid", true
	}
	if !imapFetchDataItemParenthesesValid(items) {
		return "FETCH data item list is invalid", true
	}
	if !imapFetchMacroUsageValid(items) {
		return "FETCH macro is invalid", true
	}
	if !imapFetchHeaderFieldListsValid(imapExpandFetchItems(items)) {
		return "FETCH header field list is invalid", true
	}
	if !imapFetchDataItemsSupported(imapExpandFetchItems(items)) {
		return "FETCH data item is unsupported", true
	}
	return "", false
}

func imapFetchDataItemOuterWhitespaceValid(items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) != item {
			return false
		}
	}
	return true
}

func imapFetchNormalizedTokens(items []string) []string {
	tokens := make([]string, 0, len(items))
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if token != "" {
				tokens = append(tokens, token)
			}
		}
	}
	return tokens
}

func imapFetchDataItemsSupported(items []string) bool {
	for i := 0; i < len(items); i++ {
		token := imapFetchToken(items[i])
		if token == "" {
			continue
		}
		if token == "CHANGEDSINCE" {
			i++
			continue
		}
		if imapFetchHeaderFieldSectionStart(token) {
			end, ok := imapFetchHeaderFieldSectionEnd(items, i)
			if !ok {
				return false
			}
			i = end
			continue
		}
		if imapFetchDataItemTokenSupported(token) {
			continue
		}
		return false
	}
	return true
}

func imapFetchToken(item string) string {
	return strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
}

func imapFetchHeaderFieldSectionStart(token string) bool {
	for _, prefix := range []string{"BODY.PEEK[", "BODY["} {
		section, ok := strings.CutPrefix(token, prefix)
		if !ok {
			continue
		}
		for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
			if strings.HasPrefix(section, marker) {
				return true
			}
			markerIndex := strings.Index(section, "."+marker)
			if markerIndex <= 0 {
				continue
			}
			if _, ok := parseIMAPMIMEPartPath(section[:markerIndex]); ok {
				return true
			}
		}
	}
	return false
}

func imapFetchHeaderFieldSectionEnd(items []string, start int) (int, bool) {
	for i := start; i < len(items); i++ {
		token := strings.ToUpper(strings.TrimSpace(items[i]))
		closeIdx := strings.Index(token, ")]")
		if closeIdx < 0 {
			continue
		}
		suffix := strings.Trim(token[closeIdx+2:], ")")
		if suffix == "" {
			return i, true
		}
		if strings.HasPrefix(suffix, "<") {
			_, ok := imapParsePartialBodyToken(suffix)
			return i, ok
		}
		return i, false
	}
	return 0, false
}

func imapFetchDataItemTokenSupported(token string) bool {
	switch token {
	case "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE", "BODY", "BODYSTRUCTURE", "UID", "MODSEQ":
		return true
	case "RFC822", "RFC822.HEADER", "RFC822.TEXT":
		return true
	case "BODY[]", "BODY.PEEK[]", "BODY[HEADER]", "BODY.PEEK[HEADER]", "BODY[TEXT]", "BODY.PEEK[TEXT]":
		return true
	}
	switch {
	case strings.HasPrefix(token, "BODY[]<") || strings.HasPrefix(token, "BODY.PEEK[]<") || strings.HasPrefix(token, "RFC822<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "BODY[HEADER]<") || strings.HasPrefix(token, "BODY.PEEK[HEADER]<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "BODY[TEXT]<") || strings.HasPrefix(token, "BODY.PEEK[TEXT]<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "RFC822.HEADER<") || strings.HasPrefix(token, "RFC822.TEXT<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	}
	_, ok := imapParseMIMEPartRequestToken(token)
	return ok
}

func imapFetchMacroUsageValid(items []string) bool {
	tokens := imapFetchNormalizedTokens(items)
	for _, token := range tokens {
		switch token {
		case "FAST", "ALL", "FULL":
			return len(tokens) == 1 && strings.EqualFold(strings.TrimSpace(strings.Join(items, " ")), token)
		}
	}
	return true
}

func imapFetchRequestsBodyStructure(items []string) bool {
	return imapFetchRequestsToken(items, "BODYSTRUCTURE")
}

func imapFetchRequestsBodyAttribute(items []string) bool {
	return imapFetchRequestsToken(items, "BODY")
}

func imapFetchRequestsToken(items []string, want string) bool {
	for _, token := range imapFetchNormalizedTokens(items) {
		if token == want {
			return true
		}
	}
	return false
}

func imapFetchAttributes(summary MessageSummary, includeEnvelope bool, includeInternalDate bool, includeModSeq bool, includeBody bool, includeBodyStructure bool, bodyAttribute string, bodyStructure string) []string {
	attributes := []string{
		fmt.Sprintf("UID %d", summary.UID),
		"FLAGS " + imapFlagList(summary.Flags.IMAPFlags()),
		fmt.Sprintf("RFC822.SIZE %d", summary.Size),
	}
	if includeInternalDate {
		attributes = append(attributes, "INTERNALDATE "+imapQuotedString(imapInternalDate(summary.InternalDate)))
	}
	if includeEnvelope {
		attributes = append(attributes, "ENVELOPE "+imapEnvelope(summary))
	}
	if includeModSeq {
		attributes = append(attributes, fmt.Sprintf("MODSEQ (%d)", summary.ModSeq))
	}
	if includeBody {
		if bodyAttribute == "" {
			bodyAttribute = imapBody(summary)
		}
		attributes = append(attributes, "BODY "+bodyAttribute)
	}
	if includeBodyStructure {
		if bodyStructure == "" {
			bodyStructure = imapBodyStructure(summary)
		}
		attributes = append(attributes, "BODYSTRUCTURE "+bodyStructure)
	}
	return attributes
}

func imapInternalDate(value time.Time) string {
	if value.IsZero() {
		value = time.Unix(0, 0).UTC()
	}
	return value.Format("02-Jan-2006 15:04:05 -0700")
}

func imapEnvelope(summary MessageSummary) string {
	envelope := summary.Envelope
	date := envelope.Date
	if date.IsZero() {
		date = summary.InternalDate
	}
	sender := envelope.Sender
	if len(sender) == 0 {
		sender = envelope.From
	}
	replyTo := envelope.ReplyTo
	if len(replyTo) == 0 {
		replyTo = envelope.From
	}
	return "(" + strings.Join([]string{
		imapEnvelopeNString(imapEnvelopeDate(date)),
		imapEnvelopeNString(envelope.Subject),
		imapAddressList(envelope.From),
		imapAddressList(sender),
		imapAddressList(replyTo),
		imapAddressList(envelope.To),
		imapAddressList(envelope.Cc),
		imapAddressList(envelope.Bcc),
		imapEnvelopeNString(envelope.InReplyTo),
		imapEnvelopeNString(envelope.MessageID),
	}, " ") + ")"
}

func imapEnvelopeDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC1123Z)
}

func imapAddressList(addresses []Address) string {
	if len(addresses) == 0 {
		return "NIL"
	}
	parts := make([]string, 0, min(len(addresses), maxIMAPEnvelopeAddressCount))
	for _, address := range addresses {
		if !imapEnvelopeAddressRenderable(address) {
			continue
		}
		parts = append(parts, "("+strings.Join([]string{
			imapEnvelopeNString(address.Name),
			"NIL",
			imapEnvelopeNString(address.Mailbox),
			imapEnvelopeNString(address.Host),
		}, " ")+")")
		if len(parts) == maxIMAPEnvelopeAddressCount {
			break
		}
	}
	if len(parts) == 0 {
		return "NIL"
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func imapEnvelopeAddressRenderable(address Address) bool {
	return strings.TrimSpace(address.Mailbox) != "" && strings.TrimSpace(address.Host) != ""
}

func imapEnvelopeNString(value string) string {
	value = imapBodyMetadataText(value)
	if value == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapNString(value string) string {
	if strings.TrimSpace(value) == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapBodyStructure(summary MessageSummary) string {
	return imapBodyStructureFromHeader(summary, nil)
}

func imapBodyStructureFromMIMEStructure(summary MessageSummary, structure messageparse.MIMEStructure) string {
	if structure.Root.MediaType == "" {
		return imapBodyStructure(summary)
	}
	return imapMIMEPartBody(structure.Root, maxInt64(summary.Size, 0), true)
}

func imapBodyStructureFromHeader(summary MessageSummary, header []byte) string {
	return imapBodyFromHeaderExtended(summary, header, true)
}

func imapBody(summary MessageSummary) string {
	return imapBodyFromHeader(summary, nil)
}

func imapBodyFromMIMEStructure(summary MessageSummary, structure messageparse.MIMEStructure) string {
	if structure.Root.MediaType == "" {
		return imapBody(summary)
	}
	return imapMIMEPartBody(structure.Root, maxInt64(summary.Size, 0), false)
}

func imapBodyFromHeader(summary MessageSummary, header []byte) string {
	return imapBodyFromHeaderExtended(summary, header, false)
}

func imapMIMEPartBody(part messageparse.MIMEPart, fallbackSize int64, extended bool) string {
	if part.MediaType == "MULTIPART" {
		childBodies := make([]string, 0, len(part.Parts)+5)
		for _, child := range part.Parts {
			childBodies = append(childBodies, imapMIMEPartBody(child, child.Size, extended))
		}
		if len(childBodies) == 0 {
			return imapBodyFromHeaderExtended(MessageSummary{Size: fallbackSize}, nil, extended)
		}
		_, mediaSubtype := imapMIMETypePair("MULTIPART", part.MediaSubtype, "MULTIPART", "MIXED")
		childBodies = append(childBodies, imapQuotedString(mediaSubtype))
		if extended {
			childBodies = append(childBodies, imapMIMEBodyParameterList(part.Params), "NIL", "NIL", "NIL")
		}
		return "(" + strings.Join(childBodies, " ") + ")"
	}
	return imapMIMESinglePartBody(part, fallbackSize, extended)
}

func imapMIMESinglePartBody(part messageparse.MIMEPart, fallbackSize int64, extended bool) string {
	mediaType, mediaSubtype := imapMIMETypePair(part.MediaType, part.MediaSubtype, "TEXT", "PLAIN")
	size := part.Size
	if size == 0 && fallbackSize > 0 {
		size = fallbackSize
	}
	fields := []string{
		imapQuotedString(mediaType),
		imapQuotedString(mediaSubtype),
		imapMIMEBodyParameterList(part.Params),
		imapBodyMetadataNString(part.ContentID),
		imapBodyMetadataNString(part.Description),
		imapQuotedString(imapMIMEToken(part.Encoding, "7BIT")),
		fmt.Sprintf("%d", maxInt64(size, 0)),
	}
	if mediaType == "MESSAGE" && mediaSubtype == "RFC822" {
		fields = append(fields, imapMIMEEnvelope(part.Envelope), imapMIMEMessageBody(part, extended), fmt.Sprintf("%d", maxInt64(part.Lines, 0)))
	} else if mediaType == "TEXT" {
		lines := part.Lines
		if lines == 0 && size > 0 {
			lines = 1
		}
		fields = append(fields, fmt.Sprintf("%d", lines))
	}
	if extended {
		fields = append(fields, "NIL", imapMIMEBodyDisposition(part), "NIL", "NIL")
	}
	return "(" + strings.Join(fields, " ") + ")"
}

func imapMIMEEnvelope(envelope messageparse.MIMEEnvelope) string {
	return imapEnvelope(MessageSummary{
		InternalDate: envelope.Date,
		Envelope: Envelope{
			Date:      envelope.Date,
			Subject:   envelope.Subject,
			From:      imapMIMEEnvelopeAddresses(envelope.From),
			Sender:    imapMIMEEnvelopeAddresses(envelope.Sender),
			ReplyTo:   imapMIMEEnvelopeAddresses(envelope.ReplyTo),
			To:        imapMIMEEnvelopeAddresses(envelope.To),
			Cc:        imapMIMEEnvelopeAddresses(envelope.Cc),
			Bcc:       imapMIMEEnvelopeAddresses(envelope.Bcc),
			InReplyTo: envelope.InReplyTo,
			MessageID: envelope.MessageID,
		},
	})
}

func imapMIMEEnvelopeAddresses(addresses []messageparse.Address) []Address {
	if len(addresses) == 0 {
		return nil
	}
	out := make([]Address, 0, len(addresses))
	for _, address := range addresses {
		mailbox, host, ok := strings.Cut(address.Address, "@")
		if !ok {
			continue
		}
		out = append(out, Address{Name: address.Name, Mailbox: mailbox, Host: host})
	}
	return out
}

func imapMIMEMessageBody(part messageparse.MIMEPart, extended bool) string {
	if len(part.Parts) > 0 {
		child := part.Parts[0]
		return imapMIMEPartBody(child, child.Size, extended)
	}
	return imapBodyFromHeaderExtended(MessageSummary{Size: part.Size}, nil, extended)
}

func imapMIMEBodyDisposition(part messageparse.MIMEPart) string {
	disposition := imapMIMEToken(part.Disposition, "")
	if disposition == "" {
		return "NIL"
	}
	return "(" + imapQuotedString(disposition) + " " + imapMIMEBodyParameterList(part.DispositionParams) + ")"
}

func imapBodyMetadataNString(value string) string {
	value = imapBodyMetadataText(value)
	if value == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapBodyMetadataText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxIMAPBodyMetadataTextBytes {
		value = value[:maxIMAPBodyMetadataTextBytes]
		for !utf8.ValidString(value) && len(value) > 0 {
			value = value[:len(value)-1]
		}
	}
	return value
}

func imapBodyFromHeaderExtended(summary MessageSummary, header []byte, extended bool) string {
	metadata := imapBodyMetadataFromHeader(header)
	lines := int64(0)
	if summary.Size > 0 {
		lines = 1
	}
	size := maxInt64(summary.Size, 0)
	fields := []string{
		imapQuotedString(metadata.mediaType),
		imapQuotedString(metadata.mediaSubtype),
		imapBodyParameterList(metadata.params),
		imapBodyMetadataNString(metadata.id),
		imapBodyMetadataNString(metadata.description),
		imapQuotedString(metadata.encoding),
		fmt.Sprintf("%d", size),
	}
	if metadata.mediaType == "TEXT" {
		fields = append(fields, fmt.Sprintf("%d", lines))
	}
	if extended {
		fields = append(fields, "NIL", "NIL", "NIL", "NIL")
	}
	return "(" + strings.Join(fields, " ") + ")"
}

type imapBodyMetadata struct {
	mediaType    string
	mediaSubtype string
	params       map[string]string
	id           string
	description  string
	encoding     string
}

func imapBodyMetadataFromHeader(header []byte) imapBodyMetadata {
	metadata := imapBodyMetadata{
		mediaType:    "TEXT",
		mediaSubtype: "PLAIN",
		params:       map[string]string{"CHARSET": "UTF-8"},
		encoding:     "7BIT",
	}
	if len(header) == 0 {
		return metadata
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(header))
	if err != nil {
		return metadata
	}
	contentType := strings.TrimSpace(message.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err == nil {
			if typ, subtype, ok := imapMediaTypeParts(mediaType); ok {
				if typ == "MULTIPART" {
					return metadata
				}
				metadata.mediaType = typ
				metadata.mediaSubtype = subtype
				metadata.params = imapBodyParams(params)
			}
		}
	}
	if encoding := strings.TrimSpace(message.Header.Get("Content-Transfer-Encoding")); encoding != "" {
		metadata.encoding = imapMIMEToken(encoding, "7BIT")
	}
	metadata.id = strings.TrimSpace(message.Header.Get("Content-ID"))
	metadata.description = strings.TrimSpace(message.Header.Get("Content-Description"))
	return metadata
}

func imapMediaTypeParts(value string) (string, string, bool) {
	typ, subtype, ok := strings.Cut(strings.TrimSpace(value), "/")
	typ = strings.ToUpper(strings.TrimSpace(typ))
	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	if !ok || !imapMIMETokenValid(typ) || !imapMIMETokenValid(subtype) {
		return "", "", false
	}
	return typ, subtype, true
}

func imapBodyParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for key, value := range params {
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !imapMIMETokenValid(key) || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func imapBodyParameterList(params map[string]string) string {
	return imapMIMEBodyParameterList(params)
}

func imapMIMEBodyParameterList(params map[string]string) string {
	if len(params) == 0 {
		return "NIL"
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	canonical := make(map[string]string, len(keys))
	for _, rawKey := range keys {
		key := strings.ToUpper(strings.TrimSpace(rawKey))
		value := imapBodyMetadataText(params[rawKey])
		if !imapMIMETokenValid(key) || value == "" {
			continue
		}
		if _, exists := canonical[key]; exists {
			continue
		}
		canonical[key] = value
	}
	if len(canonical) == 0 {
		return "NIL"
	}
	keys = keys[:0]
	for key := range canonical {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		values = append(values, imapQuotedString(key), imapQuotedString(canonical[key]))
	}
	return "(" + strings.Join(values, " ") + ")"
}

func imapMIMEToken(value string, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !imapMIMETokenValid(value) {
		return fallback
	}
	return value
}

func imapMIMETokenValid(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c <= 0x20 || c >= 0x7f || strings.ContainsRune("()<>@,;:\\\"/[]?=", rune(c)) {
			return false
		}
	}
	return true
}

func imapMIMETypePair(mediaType string, mediaSubtype string, fallbackType string, fallbackSubtype string) (string, string) {
	mediaType = strings.ToUpper(strings.TrimSpace(mediaType))
	mediaSubtype = strings.ToUpper(strings.TrimSpace(mediaSubtype))
	if !imapMIMETokenValid(mediaType) || !imapMIMETokenValid(mediaSubtype) {
		return fallbackType, fallbackSubtype
	}
	return mediaType, mediaSubtype
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Server) imapCapabilities(state *imapConnState) []string {
	capabilities := []string{"IMAP4rev1", "LITERAL+", "IDLE", "ID", "NAMESPACE", "CHILDREN", "UNSELECT", "UIDPLUS", "MOVE", "CONDSTORE", "ENABLE", "SPECIAL-USE", "LIST-EXTENDED", "LIST-STATUS", "ESEARCH", "SEARCHRES", "STATUS=SIZE", "SORT", "THREAD=ORDEREDSUBJECT"}
	if state != nil && state.session == nil && !state.tlsActive && s != nil && s.options.TLSConfig != nil {
		capabilities = append(capabilities, "STARTTLS")
	}
	if state == nil || state.session == nil {
		if s.authAllowed(state) {
			capabilities = append(capabilities, "SASL-IR", "AUTH=PLAIN")
		} else {
			capabilities = append(capabilities, "LOGINDISABLED")
		}
	}
	return capabilities
}

func (s *Server) authenticatedCapabilityCode(state *imapConnState) string {
	return s.capabilityCode(state)
}

func (s *Server) capabilityCode(state *imapConnState) string {
	return "[CAPABILITY " + strings.Join(s.imapCapabilities(state), " ") + "]"
}

func (s *Server) handleEnable(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD ENABLE requires at least one capability\r\n")
		return false, err
	}
	for _, field := range fields[2:] {
		if !imapAtomValid(field) {
			_, err := writer.WriteString(tag + " BAD ENABLE capability is malformed\r\n")
			return false, err
		}
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	wasCondstoreAware := state.condstoreAware
	enableCondstore := false
	enabled := make([]string, 0, len(fields)-2)
	for _, field := range fields[2:] {
		if strings.EqualFold(field, "CONDSTORE") {
			enableCondstore = true
			state.condstoreAware = true
			if !imapStringSliceContainsFold(enabled, "CONDSTORE") {
				enabled = append(enabled, "CONDSTORE")
			}
		}
	}
	if len(enabled) == 0 {
		if _, err := writer.WriteString("* ENABLED\r\n"); err != nil {
			return false, err
		}
	} else if _, err := writer.WriteString("* ENABLED " + strings.Join(enabled, " ") + "\r\n"); err != nil {
		return false, err
	}
	if enableCondstore && !wasCondstoreAware && state.selectedMailbox != "" {
		if state.selectedHighestModSeq > 0 {
			state.selectedNoModSeq = false
			if _, err := writer.WriteString(fmt.Sprintf("* OK [HIGHESTMODSEQ %d] Highest mod-sequence\r\n", state.selectedHighestModSeq)); err != nil {
				return false, err
			}
		} else {
			state.selectedNoModSeq = true
			if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
				return false, err
			}
		}
	}
	_, err := writer.WriteString(tag + " OK ENABLE completed\r\n")
	return false, err
}

func imapStringSliceContainsFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func (s *Server) authAllowed(state *imapConnState) bool {
	if s == nil {
		return false
	}
	if s.options.AllowInsecureAuth {
		return true
	}
	return state != nil && state.tlsActive
}

func (s *Server) handleUIDStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 6 {
		_, err := writer.WriteString(tag + " BAD UID STORE requires UID, mode, and flags\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(context.Background(), state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID STORE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
		return false, err
	}
	unchangedSince, unchangedSinceSet, storeFields, ok := imapStoreUnchangedSince(fields[4:])
	if !ok || len(storeFields) < 2 {
		_, err := writer.WriteString(tag + " BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n")
		return false, err
	}
	if imapStoreUnchangedSincePresent(fields[4:]) && !state.selectedSupportsPersistentModSeq() {
		return s.rejectSelectedNoModSeq(writer, tag, state, "UID STORE")
	}
	mode, silent, ok := imapStoreMode(storeFields[0])
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE mode is unsupported\r\n")
		return false, err
	}
	flags, requestedFlags, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE flags are unsupported\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	if !imapPermanentFlagsAllow(state.permanentFlags, requestedFlags, mode) {
		_, err := writer.WriteString(tag + " NO UID STORE flags are not permitted\r\n")
		return false, err
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, unchangedSince, unchangedSinceSet, "UID STORE")
}

func (s *Server) handleStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 5 {
		_, err := writer.WriteString(tag + " BAD STORE requires sequence set, mode, and flags\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
		return false, err
	}
	if message, ok := imapStoreArgumentsSyntaxError("STORE", fields[3:]); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSetForState(fields[2], state.selectedMessages, state)
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
		return false, err
	}
	unchangedSince, unchangedSinceSet, storeFields, ok := imapStoreUnchangedSince(fields[3:])
	if !ok || len(storeFields) < 2 {
		_, err := writer.WriteString(tag + " BAD STORE UNCHANGEDSINCE modifier is invalid\r\n")
		return false, err
	}
	if imapStoreUnchangedSincePresent(fields[3:]) && !state.selectedSupportsPersistentModSeq() {
		return s.rejectSelectedNoModSeq(writer, tag, state, "STORE")
	}
	mode, silent, ok := imapStoreMode(storeFields[0])
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE mode is unsupported\r\n")
		return false, err
	}
	flags, requestedFlags, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE flags are unsupported\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	if !imapPermanentFlagsAllow(state.permanentFlags, requestedFlags, mode) {
		_, err := writer.WriteString(tag + " NO STORE flags are not permitted\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(context.Background(), state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO STORE failed\r\n")
		return false, writeErr
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, unchangedSince, unchangedSinceSet, "STORE")
}

func imapStoreArgumentsSyntaxError(command string, fields []string) (string, bool) {
	_, _, storeFields, ok := imapStoreUnchangedSince(fields)
	if !ok || len(storeFields) < 2 {
		return command + " UNCHANGEDSINCE modifier is invalid", true
	}
	if _, _, ok := imapStoreMode(storeFields[0]); !ok {
		return command + " mode is unsupported", true
	}
	if _, _, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " ")); !ok {
		return command + " flags are unsupported", true
	}
	return "", false
}

func (s *Server) writeStoreResponses(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, flags MessageFlags, mode StoreFlagsMode, silent bool, unchangedSince uint64, unchangedSinceSet bool, completionCommand string) (bool, error) {
	if unchangedSinceSet {
		state.condstoreAware = true
	}
	if len(uids) == 0 || ((mode == StoreFlagsAdd || mode == StoreFlagsRemove) && imapMessageFlagsEmpty(flags)) {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	summaries, err := s.options.Backend.StoreFlags(context.Background(), StoreFlagsRequest{
		UserID:            state.session.UserID,
		MailboxID:         state.selectedMailbox,
		UIDs:              uids,
		Flags:             flags,
		Mode:              mode,
		UnchangedSince:    unchangedSince,
		UnchangedSinceSet: unchangedSinceSet,
	})
	if err != nil {
		var modified *StoreModifiedError
		if errors.As(err, &modified) {
			successfulSummaries := imapStoreSuccessfulSummaries(summaries, modified)
			state.observeHighestModSeq(imapHighestSummaryModSeq(successfulSummaries))
			if err := s.writeStoreFetchResponses(writer, tag, successfulSummaries, state.condstoreAware, completionCommand); err != nil {
				return false, err
			}
			modifiedSet, err := s.storeModifiedSetResponse(context.Background(), state, modified.UIDs, completionCommand == "UID STORE")
			if err != nil {
				_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
				return false, writeErr
			}
			_, writeErr := writer.WriteString(fmt.Sprintf("%s OK [MODIFIED %s] %s conditional store completed\r\n", tag, modifiedSet, completionCommand))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	state.observeHighestModSeq(imapHighestSummaryModSeq(summaries))
	if silent && !unchangedSinceSet {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	if err := s.writeStoreFetchResponses(writer, tag, summaries, state.condstoreAware, completionCommand); err != nil {
		return false, err
	}
	_, err = writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) writeStoreFetchResponses(writer *bufio.Writer, tag string, summaries []MessageSummary, includeModSeq bool, completionCommand string) error {
	for _, summary := range summaries {
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return err
		}
		attributes := []string{
			fmt.Sprintf("UID %d", summary.UID),
			"FLAGS " + imapFlagList(summary.Flags.IMAPFlags()),
		}
		if includeModSeq {
			attributes = append(attributes, fmt.Sprintf("MODSEQ (%d)", summary.ModSeq))
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s)\r\n", sequenceNumber, strings.Join(attributes, " "))); err != nil {
			return err
		}
	}
	return nil
}

func imapStoreSuccessfulSummaries(summaries []MessageSummary, modified *StoreModifiedError) []MessageSummary {
	if modified == nil {
		return summaries
	}
	source := modified.Summaries
	if len(source) == 0 {
		source = summaries
	}
	if len(source) == 0 || len(modified.UIDs) == 0 {
		return source
	}
	modifiedUIDs := make(map[UID]struct{}, len(modified.UIDs))
	for _, uid := range modified.UIDs {
		modifiedUIDs[uid] = struct{}{}
	}
	successful := make([]MessageSummary, 0, len(source))
	for _, summary := range source {
		if _, stale := modifiedUIDs[summary.UID]; stale {
			continue
		}
		successful = append(successful, summary)
	}
	return successful
}

func (s *Server) storeModifiedSetResponse(ctx context.Context, state *imapConnState, uids []UID, uidMode bool) (string, error) {
	if uidMode {
		return imapUIDSetResponse(uids), nil
	}
	sequenceNumbers := make([]UID, 0, len(uids))
	for _, uid := range uids {
		message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		})
		if err != nil {
			return "", err
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		sequenceNumber, ok := imapSequenceNumber(message.Summary)
		if !ok {
			return "", fmt.Errorf("imap modified sequence number is unavailable")
		}
		sequenceNumbers = append(sequenceNumbers, UID(sequenceNumber))
	}
	return imapUIDSetResponse(sequenceNumbers), nil
}

func imapStoreUnchangedSince(fields []string) (uint64, bool, []string, bool) {
	if len(fields) == 0 {
		return 0, false, fields, true
	}
	first := strings.ToUpper(fields[0])
	if !strings.Contains(first, "UNCHANGEDSINCE") && !strings.HasPrefix(first, "(") {
		return 0, false, fields, true
	}
	if first != "(UNCHANGEDSINCE" || len(fields) < 2 {
		return 0, false, nil, false
	}
	valueToken := fields[1]
	if !strings.HasSuffix(valueToken, ")") || strings.HasSuffix(valueToken, "))") {
		return 0, false, nil, false
	}
	value := strings.TrimSuffix(valueToken, ")")
	threshold, ok := parseIMAPModSeqValzer(value)
	if !ok {
		return 0, false, nil, false
	}
	return threshold, true, fields[2:], true
}

func imapStoreUnchangedSincePresent(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	return strings.EqualFold(fields[0], "(UNCHANGEDSINCE")
}

func imapSequenceNumber(summary MessageSummary) (uint32, bool) {
	if summary.SequenceNumber == 0 {
		return 0, false
	}
	return summary.SequenceNumber, true
}

func imapStoreMode(value string) (StoreFlagsMode, bool, bool) {
	if strings.TrimSpace(value) != value {
		return "", false, false
	}
	switch strings.ToUpper(value) {
	case "FLAGS":
		return StoreFlagsReplace, false, true
	case "FLAGS.SILENT":
		return StoreFlagsReplace, true, true
	case "+FLAGS":
		return StoreFlagsAdd, false, true
	case "+FLAGS.SILENT":
		return StoreFlagsAdd, true, true
	case "-FLAGS":
		return StoreFlagsRemove, false, true
	case "-FLAGS.SILENT":
		return StoreFlagsRemove, true, true
	default:
		return "", false, false
	}
}

func imapStoreFlags(value string) (MessageFlags, bool) {
	flags, _, ok := imapStoreFlagsWithNames(value)
	return flags, ok
}

func imapStoreFlagsWithNames(value string) (MessageFlags, []string, bool) {
	var flags MessageFlags
	if strings.TrimSpace(value) != value {
		return MessageFlags{}, nil, false
	}
	if value != "()" && (!strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")")) {
		return MessageFlags{}, nil, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
	tokens, ok := imapFlagListTokens(inner)
	if !ok {
		return MessageFlags{}, nil, false
	}
	if len(tokens) == 0 {
		return flags, nil, value == "()"
	}
	names := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, raw := range tokens {
		name := CanonicalIMAPFlag(raw)
		switch name {
		case FlagSeen:
			flags.Read = true
		case FlagFlagged:
			flags.Starred = true
		case FlagAnswered:
			flags.Answered = true
		case FlagForwarded:
			flags.Forwarded = true
		case FlagDraft:
			flags.Draft = true
		case FlagDeleted:
			flags.Deleted = true
		default:
			if !IMAPKeywordFlagValid(name) {
				return MessageFlags{}, nil, false
			}
			flags.Keywords = append(flags.Keywords, name)
		}
		if _, ok := seen[name]; ok {
			return MessageFlags{}, nil, false
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return flags, names, true
}

func imapFlagListTokens(inner string) ([]string, bool) {
	if inner == "" {
		return nil, true
	}
	if strings.TrimSpace(inner) != inner {
		return nil, false
	}
	tokens := strings.Split(inner, " ")
	for _, token := range tokens {
		if token == "" || strings.TrimSpace(token) != token {
			return nil, false
		}
	}
	return tokens, true
}

func imapPermanentFlagSet(flags []string) map[string]struct{} {
	permitted := make(map[string]struct{}, len(flags))
	for _, name := range imapCanonicalPermanentFlags(flags) {
		permitted[name] = struct{}{}
	}
	return permitted
}

func imapCanonicalPermanentFlags(flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(flags))
	custom := make([]string, 0)
	for _, raw := range flags {
		switch name := CanonicalIMAPFlag(raw); name {
		case FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDraft, FlagDeleted:
			present[name] = struct{}{}
		default:
			if !IMAPKeywordFlagValid(name) {
				continue
			}
			if _, ok := present[name]; ok {
				continue
			}
			present[name] = struct{}{}
			custom = append(custom, name)
		}
	}
	if len(present) == 0 {
		return nil
	}
	ordered := []string{FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDraft, FlagDeleted}
	canonical := make([]string, 0, len(present))
	for _, name := range ordered {
		if _, ok := present[name]; ok {
			canonical = append(canonical, name)
		}
	}
	canonical = append(canonical, custom...)
	return canonical
}

func imapPermanentFlagsAllow(permitted map[string]struct{}, requested []string, mode StoreFlagsMode) bool {
	if len(requested) == 0 {
		return mode != StoreFlagsReplace || len(permitted) > 0
	}
	for _, flag := range requested {
		if _, ok := permitted[flag]; !ok {
			return false
		}
	}
	return true
}

func imapMessageFlagsEmpty(flags MessageFlags) bool {
	return !flags.Read &&
		!flags.Starred &&
		!flags.Answered &&
		!flags.Forwarded &&
		!flags.Draft &&
		!flags.Deleted &&
		len(flags.Keywords) == 0 &&
		strings.TrimSpace(flags.Status) == ""
}

func imapMailboxDisplayName(mailbox Mailbox) string {
	if strings.TrimSpace(mailbox.FullPath) != "" {
		if value := strings.Trim(strings.TrimSpace(mailbox.FullPath), "/"); value != "" {
			return value
		}
	}
	if strings.TrimSpace(mailbox.Name) != "" {
		return strings.TrimSpace(mailbox.Name)
	}
	return strings.TrimSpace(string(mailbox.ID))
}

func imapMailboxListAttributes(mailbox Mailbox, hasChildren bool) []string {
	attributes := []string{`\HasNoChildren`}
	if hasChildren {
		attributes[0] = `\HasChildren`
	}
	switch strings.ToLower(strings.TrimSpace(mailbox.SystemType)) {
	case "all":
		attributes = append(attributes, `\All`)
	case "archive":
		attributes = append(attributes, `\Archive`)
	case "drafts":
		attributes = append(attributes, `\Drafts`)
	case "flagged":
		attributes = append(attributes, `\Flagged`)
	case "junk", "spam":
		attributes = append(attributes, `\Junk`)
	case "sent":
		attributes = append(attributes, `\Sent`)
	case "trash":
		attributes = append(attributes, `\Trash`)
	}
	return attributes
}

func imapMailboxChildren(mailboxes []Mailbox) map[MailboxID]bool {
	children := make(map[MailboxID]bool)
	byWireName := make(map[string]MailboxID, len(mailboxes))
	for _, mailbox := range mailboxes {
		if mailbox.ID == "" {
			continue
		}
		wireName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		if wireName != "" {
			byWireName[strings.ToLower(wireName)] = mailbox.ID
		}
	}
	for _, mailbox := range mailboxes {
		if mailbox.ParentID != "" {
			children[mailbox.ParentID] = true
			continue
		}
		wireName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		parentName, ok := imapMailboxParentWireName(wireName)
		if !ok {
			continue
		}
		if parentID, ok := byWireName[parentName]; ok {
			children[parentID] = true
		}
	}
	return children
}

func imapMailboxParentWireName(wireName string) (string, bool) {
	wireName = strings.ToLower(strings.Trim(wireName, "/"))
	index := strings.LastIndex(wireName, "/")
	if index <= 0 {
		return "", false
	}
	return wireName[:index], true
}

type imapListOptions struct {
	fields           []string
	specialUseOnly   bool
	subscribedOnly   bool
	subscribedReturn bool
	statusItems      []string
}

func imapListCommandOptions(fields []string, subscribed bool) (imapListOptions, string, bool) {
	if subscribed {
		if len(fields) > 0 && strings.HasPrefix(strings.TrimSpace(fields[0]), "(") {
			return imapListOptions{}, "LSUB does not support LIST extension options", false
		}
		if len(fields) > 2 && strings.EqualFold(fields[2], "RETURN") {
			return imapListOptions{}, "LSUB does not support LIST extension options", false
		}
		return imapListOptions{fields: fields}, "", true
	}
	options := imapListOptions{}
	if len(fields) > 0 && strings.HasPrefix(strings.TrimSpace(fields[0]), "(") {
		if !strings.HasPrefix(fields[0], "(") {
			return imapListOptions{}, "", false
		}
		optionFields, rest, ok := imapConsumeParenthesizedFields(fields)
		if !ok {
			return imapListOptions{}, "", false
		}
		tokens, ok := imapSearchReturnOptionTokens(optionFields)
		if !ok || len(tokens) == 0 {
			return imapListOptions{}, "", false
		}
		for _, token := range tokens {
			switch strings.ToUpper(token) {
			case "SPECIAL-USE":
				options.specialUseOnly = true
			case "SUBSCRIBED":
				options.subscribedOnly = true
			default:
				return imapListOptions{}, "", false
			}
		}
		fields = rest
	}
	if len(fields) < 2 {
		return imapListOptions{}, "", false
	}
	options.fields = fields[:2]
	rest := fields[2:]
	if strings.HasPrefix(strings.TrimSpace(fields[1]), "(") {
		if !strings.HasPrefix(fields[1], "(") {
			return imapListOptions{}, "", false
		}
		patternFields, patternRest, ok := imapConsumeParenthesizedFields(fields[1:])
		if !ok {
			return imapListOptions{}, "", false
		}
		options.fields = append([]string{fields[0]}, patternFields...)
		rest = patternRest
	}
	if len(rest) == 0 {
		return options, "", true
	}
	if len(rest) < 2 || !strings.EqualFold(rest[0], "RETURN") {
		return imapListOptions{}, "", false
	}
	if !imapListReturnOptionsParenthesized(rest[1:]) {
		return imapListOptions{}, "LIST requires parenthesized return options", false
	}
	if !imapListStatusReturnItemsParenthesized(rest[1:]) {
		return imapListOptions{}, "LIST requires parenthesized status item list", false
	}
	tokens := imapFetchNormalizedTokens(rest[1:])
	if len(tokens) == 0 {
		return imapListOptions{}, "", false
	}
	statusReturnSeen := false
	for i := 0; i < len(tokens); {
		switch strings.ToUpper(tokens[i]) {
		case "CHILDREN":
			i++
		case "SPECIAL-USE":
			i++
		case "SUBSCRIBED":
			options.subscribedReturn = true
			i++
		case "STATUS":
			if statusReturnSeen {
				return imapListOptions{}, "LIST status return option is duplicated", false
			}
			statusReturnSeen = true
			i++
			start := i
			for i < len(tokens) && !strings.EqualFold(tokens[i], "CHILDREN") && !strings.EqualFold(tokens[i], "SPECIAL-USE") && !strings.EqualFold(tokens[i], "SUBSCRIBED") {
				i++
			}
			if start == i {
				return imapListOptions{}, "LIST requires status data items", false
			}
			statusItems, statusErr, ok := imapStatusItemsFromTokens(tokens[start:i])
			if !ok {
				return imapListOptions{}, strings.Replace(statusErr, "STATUS item", "LIST status item", 1), false
			}
			options.statusItems = statusItems
		default:
			return imapListOptions{}, "", false
		}
	}
	return options, "", true
}

func imapListReturnOptionsParenthesized(fields []string) bool {
	value := strings.Join(fields, " ")
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return false
	}
	if strings.HasPrefix(value, "((") {
		return false
	}
	depth := 0
	for _, r := range value {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func imapListStatusReturnItemsParenthesized(fields []string) bool {
	joined := strings.TrimSpace(strings.Join(fields, " "))
	upper := strings.ToUpper(joined)
	offset := 0
	for {
		index := strings.Index(upper[offset:], "STATUS")
		if index < 0 {
			return true
		}
		index += offset
		end := index + len("STATUS")
		if !imapTokenBoundary(upper, index, end) {
			offset = end
			continue
		}
		rest := strings.TrimLeft(joined[end:], " \t")
		return strings.HasPrefix(rest, "(")
	}
}

func imapTokenBoundary(value string, start int, end int) bool {
	if start > 0 {
		prev := value[start-1]
		if ('A' <= prev && prev <= 'Z') || ('0' <= prev && prev <= '9') || prev == '-' {
			return false
		}
	}
	if end < len(value) {
		next := value[end]
		if ('A' <= next && next <= 'Z') || ('0' <= next && next <= '9') || next == '-' {
			return false
		}
	}
	return true
}

func imapMailboxWireName(value string) string {
	value = strings.ToValidUTF8(value, "")
	var b strings.Builder
	lastSanitizedSpace := false
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			if !lastSanitizedSpace {
				b.WriteRune(' ')
				lastSanitizedSpace = true
			}
			continue
		}
		b.WriteRune(r)
		lastSanitizedSpace = false
	}
	return strings.TrimSpace(b.String())
}

func imapEncodeMailboxName(value string) string {
	value = strings.ToValidUTF8(value, "")
	var b strings.Builder
	var shifted []uint16
	flushShifted := func() {
		if len(shifted) == 0 {
			return
		}
		raw := make([]byte, 0, len(shifted)*2)
		for _, unit := range shifted {
			raw = append(raw, byte(unit>>8), byte(unit))
		}
		encoded := base64.RawStdEncoding.EncodeToString(raw)
		encoded = strings.ReplaceAll(encoded, "/", ",")
		b.WriteByte('&')
		b.WriteString(encoded)
		b.WriteByte('-')
		shifted = shifted[:0]
	}
	for _, r := range value {
		if r >= 0x20 && r <= 0x7e && r != '&' {
			flushShifted()
			b.WriteRune(r)
			continue
		}
		if r == '&' {
			flushShifted()
			b.WriteString("&-")
			continue
		}
		shifted = append(shifted, utf16.Encode([]rune{r})...)
	}
	flushShifted()
	return b.String()
}

func imapDecodeMailboxName(value string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(value); {
		if value[i] == '&' {
			end := strings.IndexByte(value[i+1:], '-')
			if end < 0 {
				return "", false
			}
			end += i + 1
			encoded := value[i+1 : end]
			if encoded == "" {
				b.WriteByte('&')
				i = end + 1
				continue
			}
			decoded, ok := imapDecodeMailboxBase64(encoded)
			if !ok {
				return "", false
			}
			b.WriteString(decoded)
			i = end + 1
			continue
		}
		if value[i] >= 0x80 || value[i] < 0x20 || value[i] == 0x7f {
			return "", false
		}
		b.WriteByte(value[i])
		i++
	}
	decoded := b.String()
	if strings.Contains(value, "&") && imapEncodeMailboxName(decoded) != value {
		return "", false
	}
	return decoded, true
}

func imapDecodeMailboxBase64(value string) (string, bool) {
	if strings.ContainsAny(value, "&-") || len(value)%4 == 1 {
		return "", false
	}
	raw, err := base64.RawStdEncoding.DecodeString(strings.ReplaceAll(value, ",", "/"))
	if err != nil || len(raw) == 0 || len(raw)%2 != 0 {
		return "", false
	}
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i < len(raw); i += 2 {
		units = append(units, uint16(raw[i])<<8|uint16(raw[i+1]))
	}
	var runes []rune
	for i := 0; i < len(units); i++ {
		unit := units[i]
		switch {
		case 0xd800 <= unit && unit <= 0xdbff:
			if i+1 >= len(units) || units[i+1] < 0xdc00 || units[i+1] > 0xdfff {
				return "", false
			}
			runes = append(runes, utf16.DecodeRune(rune(unit), rune(units[i+1])))
			i++
		case 0xdc00 <= unit && unit <= 0xdfff:
			return "", false
		default:
			runes = append(runes, rune(unit))
		}
	}
	for _, r := range runes {
		if r >= 0x20 && r <= 0x7e {
			return "", false
		}
		if r < 0x20 || r == 0x7f {
			return "", false
		}
	}
	return string(runes), true
}

func imapListPattern(reference string, pattern string) (string, bool) {
	var ok bool
	reference, ok = imapDecodeMailboxName(reference)
	if !ok {
		return "", false
	}
	pattern, ok = imapDecodeMailboxName(pattern)
	if !ok {
		return "", false
	}
	if strings.HasPrefix(pattern, "/") {
		return strings.TrimPrefix(pattern, "/"), true
	}
	reference = strings.TrimPrefix(reference, "/")
	if reference == "" || pattern == "" {
		return pattern, true
	}
	return strings.TrimRight(reference, "/") + "/" + pattern, true
}

func imapListPatterns(fields []string) ([]string, bool) {
	if len(fields) < 2 {
		return nil, false
	}
	reference := fields[0]
	if len(fields) == 2 && !strings.HasPrefix(strings.TrimSpace(fields[1]), "(") {
		pattern, ok := imapListPattern(reference, fields[1])
		if !ok {
			return nil, false
		}
		return []string{pattern}, true
	}
	if len(fields) < 2 || !strings.HasPrefix(fields[1], "(") {
		return nil, false
	}
	raw := strings.Join(fields[1:], " ")
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") || strings.HasPrefix(raw, "((") {
		return nil, false
	}
	patternFields := imapParenthesizedMailboxPatternFields(fields[1:])
	patterns := make([]string, 0, len(patternFields))
	seen := make(map[string]struct{}, len(patternFields))
	for _, patternField := range patternFields {
		if patternField == "" {
			return nil, false
		}
		pattern, ok := imapListPattern(reference, patternField)
		if !ok {
			return nil, false
		}
		key := strings.ToLower(pattern)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		patterns = append(patterns, pattern)
	}
	return patterns, len(patterns) > 0
}

func imapParenthesizedMailboxPatternFields(fields []string) []string {
	raw := strings.TrimSpace(strings.Join(fields, " "))
	if !strings.HasPrefix(raw, "(") || !strings.HasSuffix(raw, ")") || strings.HasPrefix(raw, "((") {
		return nil
	}
	inner := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(raw, "("), ")"))
	if inner == "" {
		return nil
	}
	patterns, err := parseIMAPFields(inner)
	if err != nil || len(patterns) == 0 {
		return nil
	}
	for _, pattern := range patterns {
		if pattern == "" || strings.ContainsAny(pattern, "()") {
			return nil
		}
	}
	return patterns
}

func imapSelectCondstore(fields []string) (bool, bool) {
	if len(fields) == 0 {
		return false, true
	}
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return false, false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return false, false
	}
	if len(tokens) != 1 || tokens[0] != "CONDSTORE" {
		return false, false
	}
	return true, true
}

func imapParenthesizedAtomListShapeValid(fields []string) bool {
	value := strings.TrimSpace(strings.Join(fields, " "))
	if !strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")") {
		return false
	}
	if strings.HasPrefix(value, "((") || strings.HasSuffix(value, "))") {
		return false
	}
	depth := 0
	for _, r := range value {
		switch r {
		case '(':
			depth++
			if depth > 1 {
				return false
			}
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

func imapMailboxMatchesPattern(name string, pattern string) bool {
	matcher, ok := imapMailboxPatternMatcher(pattern)
	return ok && matcher(name)
}

func imapMailboxPatternMatcherAny(patterns []string) (func(string) bool, bool) {
	matchers := make([]func(string) bool, 0, len(patterns))
	for _, pattern := range patterns {
		if pattern == "" {
			continue
		}
		matcher, ok := imapMailboxPatternMatcher(pattern)
		if !ok {
			return nil, false
		}
		matchers = append(matchers, matcher)
	}
	return func(name string) bool {
		for _, matcher := range matchers {
			if matcher(name) {
				return true
			}
		}
		return false
	}, true
}

func imapMailboxPatternListContainsRoot(patterns []string) bool {
	for _, pattern := range patterns {
		if pattern == "" {
			return true
		}
	}
	return false
}

func imapMailboxPatternMatcher(pattern string) (func(string) bool, bool) {
	if pattern == "" {
		return func(name string) bool { return name == "" }, true
	}
	var b strings.Builder
	b.WriteString("^")
	for _, r := range pattern {
		switch r {
		case '*':
			b.WriteString(".*")
		case '%':
			b.WriteString(`[^/]*`)
		default:
			b.WriteString(regexp.QuoteMeta(string(r)))
		}
	}
	b.WriteString("$")
	compiled, err := regexp.Compile(b.String())
	if err != nil {
		return nil, false
	}
	return compiled.MatchString, true
}

func imapStatusItems(items []string) ([]string, string, bool) {
	inner, ok := imapStatusItemListInner(items)
	if !ok {
		return nil, "STATUS item is unsupported", false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return nil, "STATUS item is unsupported", false
	}
	return imapStatusItemsFromTokens(tokens)
}

func imapStatusItemsFromTokens(tokens []string) ([]string, string, bool) {
	out := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		item := strings.ToUpper(token)
		switch item {
		case "MESSAGES", "RECENT", "UIDNEXT", "UIDVALIDITY", "UNSEEN", "HIGHESTMODSEQ", "SIZE":
			if _, ok := seen[item]; ok {
				return nil, "STATUS item is duplicated", false
			}
			seen[item] = struct{}{}
			out = append(out, item)
		default:
			return nil, "STATUS item is unsupported", false
		}
	}
	if len(out) == 0 {
		return nil, "STATUS requires status data items", false
	}
	return out, "", true
}

func imapStatusItemListInner(items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	joined := strings.Join(items, " ")
	if strings.TrimSpace(joined) != joined ||
		!strings.HasPrefix(joined, "(") ||
		!strings.HasSuffix(joined, ")") ||
		strings.Count(joined, "(") != 1 ||
		strings.Count(joined, ")") != 1 {
		return "", false
	}
	return joined[1 : len(joined)-1], true
}

func imapParenthesizedAtomListTokens(inner string) ([]string, bool) {
	if inner == "" {
		return nil, true
	}
	if strings.TrimSpace(inner) != inner {
		return nil, false
	}
	tokens := strings.Split(inner, " ")
	for _, token := range tokens {
		if token == "" || strings.TrimSpace(token) != token {
			return nil, false
		}
	}
	return tokens, true
}

func imapStatusItemListIsParenthesized(items []string) bool {
	if len(items) == 0 {
		return false
	}
	_, ok := imapStatusItemListInner(items)
	return ok
}

func imapStatusItemListIsEmpty(items []string) bool {
	joined := strings.TrimSpace(strings.Join(items, " "))
	if !strings.HasPrefix(joined, "(") || !strings.HasSuffix(joined, ")") {
		return false
	}
	return strings.TrimSpace(joined[1:len(joined)-1]) == ""
}

func imapStatusRequestsItem(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

func imapStatusData(mailbox Mailbox, items []string) string {
	parts := make([]string, 0, len(items)*2)
	for _, item := range items {
		switch item {
		case "MESSAGES":
			parts = append(parts, "MESSAGES", strconv.FormatUint(uint64(mailbox.Messages), 10))
		case "RECENT":
			parts = append(parts, "RECENT", strconv.FormatUint(uint64(mailbox.Recent), 10))
		case "UIDNEXT":
			parts = append(parts, "UIDNEXT", strconv.FormatUint(uint64(mailbox.UIDNext), 10))
		case "UIDVALIDITY":
			parts = append(parts, "UIDVALIDITY", strconv.FormatUint(uint64(mailbox.UIDValidity), 10))
		case "UNSEEN":
			parts = append(parts, "UNSEEN", strconv.FormatUint(uint64(mailbox.Unseen), 10))
		case "HIGHESTMODSEQ":
			parts = append(parts, "HIGHESTMODSEQ", strconv.FormatUint(mailbox.HighestModSeq, 10))
		case "SIZE":
			parts = append(parts, "SIZE", strconv.FormatInt(mailbox.Size, 10))
		}
	}
	return strings.Join(parts, " ")
}

func imapFlagList(flags []string) string {
	if len(flags) == 0 {
		return "()"
	}
	return "(" + strings.Join(flags, " ") + ")"
}

func imapQuotedString(value string) string {
	value = strings.ToValidUTF8(value, "?")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		if r >= 0x80 {
			return '?'
		}
		return r
	}, value)
	return `"` + value + `"`
}

func parseIMAPFields(line string) ([]string, error) {
	return parseIMAPFieldsWithLiteral(line, nil)
}

func parseIMAPFieldsWithLiteral(line string, literals []string) ([]string, error) {
	fields := make([]string, 0, 4)
	literalIndex := 0
	for i := 0; i < len(line); {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			break
		}
		if line[i] == '"' {
			i++
			var b strings.Builder
			closed := false
			for i < len(line) {
				switch line[i] {
				case '\\':
					i++
					if i >= len(line) {
						return nil, fmt.Errorf("unterminated quoted string")
					}
					if line[i] != '\\' && line[i] != '"' {
						return nil, fmt.Errorf("invalid quoted escape")
					}
					b.WriteByte(line[i])
					i++
				case '"':
					i++
					if i < len(line) && line[i] != ' ' && line[i] != '\t' && line[i] != ')' {
						return nil, fmt.Errorf("quoted string must be delimited")
					}
					fields = append(fields, b.String())
					closed = true
				default:
					if line[i] < 0x20 || line[i] >= 0x7f {
						return nil, fmt.Errorf("invalid quoted control character")
					}
					b.WriteByte(line[i])
					i++
				}
				if closed {
					break
				}
			}
			if !closed {
				return nil, fmt.Errorf("unterminated quoted string")
			}
			continue
		}
		if line[i] == '(' && imapParenthesizedFieldNeedsGrouping(line, i) {
			field, next, err := parseIMAPParenthesizedField(line, i, literals, &literalIndex)
			if err != nil {
				return nil, err
			}
			fields = append(fields, field)
			i = next
			continue
		}
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			if line[i] == '"' && line[start] != '(' {
				return nil, fmt.Errorf("invalid embedded atom quote character")
			}
			if line[i] < 0x20 || line[i] >= 0x7f {
				return nil, fmt.Errorf("invalid atom control character")
			}
			i++
		}
		field := line[start:i]
		if imapLooksLikeLiteral(field) {
			if literalIndex >= len(literals) {
				return nil, fmt.Errorf("imap literal is not available")
			}
			fields = append(fields, literals[literalIndex])
			literalIndex++
			continue
		}
		if strings.HasSuffix(field, ")") && imapLooksLikeLiteral(strings.TrimSuffix(field, ")")) {
			if literalIndex >= len(literals) {
				return nil, fmt.Errorf("imap literal is not available")
			}
			fields = append(fields, literals[literalIndex]+")")
			literalIndex++
			continue
		}
		if imapLooksLikeLiteralPrefix(field) {
			return nil, fmt.Errorf("imap literal syntax is unsupported")
		}
		fields = append(fields, field)
	}
	if literalIndex != len(literals) {
		return nil, fmt.Errorf("unused imap literal")
	}
	return fields, nil
}

func imapRawFieldIsAtom(line string, fieldIndex int) bool {
	kind, ok := imapRawFieldKind(line, fieldIndex)
	return ok && kind == imapRawFieldAtom
}

type imapRawFieldKindValue int

const (
	imapRawFieldAtom imapRawFieldKindValue = iota
	imapRawFieldQuoted
	imapRawFieldLiteral
	imapRawFieldList
)

func imapRawFieldKind(line string, fieldIndex int) (imapRawFieldKindValue, bool) {
	current := 0
	for i := 0; i < len(line); {
		for i < len(line) && (line[i] == ' ' || line[i] == '\t') {
			i++
		}
		if i >= len(line) {
			return 0, false
		}
		kind := imapRawFieldAtom
		switch line[i] {
		case '"':
			kind = imapRawFieldQuoted
			i++
			for i < len(line) {
				if line[i] == '\\' {
					i += 2
					continue
				}
				if line[i] == '"' {
					i++
					break
				}
				i++
			}
		case '(':
			kind = imapRawFieldList
			depth := 0
			for i < len(line) {
				switch line[i] {
				case '"':
					i++
					for i < len(line) {
						if line[i] == '\\' {
							i += 2
							continue
						}
						if line[i] == '"' {
							i++
							break
						}
						i++
					}
					continue
				case '(':
					depth++
				case ')':
					depth--
					if depth == 0 {
						i++
						goto fieldDone
					}
				}
				i++
			}
		default:
			start := i
			for i < len(line) && line[i] != ' ' && line[i] != '\t' {
				i++
			}
			if imapLooksLikeLiteral(line[start:i]) {
				kind = imapRawFieldLiteral
			}
		}
	fieldDone:
		if current == fieldIndex {
			return kind, true
		}
		current++
	}
	return 0, false
}

func imapParenthesizedFieldNeedsGrouping(line string, start int) bool {
	depth := 0
	quoted := false
	escaped := false
	quotedHasWhitespace := false
	hasLiteralMarker := false
	for i := start; i < len(line); i++ {
		c := line[i]
		if quoted {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				quoted = false
			case ' ', '\t':
				quotedHasWhitespace = true
			}
			continue
		}
		switch c {
		case '"':
			quoted = true
		case '{':
			end := strings.IndexByte(line[i:], '}')
			if end >= 0 && imapParenthesizedLiteralMarkerDelimited(line, start, i) && imapLooksLikeLiteral(line[i:i+end+1]) {
				hasLiteralMarker = true
				i += end
			}
		case '(':
			depth++
		case ')':
			depth--
			if depth <= 0 {
				return quotedHasWhitespace || hasLiteralMarker
			}
		}
	}
	return false
}

func parseIMAPParenthesizedField(line string, start int, literals []string, literalIndex *int) (string, int, error) {
	depth := 0
	quoted := false
	escaped := false
	var field strings.Builder
	for i := start; i < len(line); i++ {
		c := line[i]
		if c < 0x20 || c >= 0x7f {
			return "", 0, fmt.Errorf("invalid parenthesized control character")
		}
		if quoted {
			if escaped {
				if c != '\\' && c != '"' {
					return "", 0, fmt.Errorf("invalid quoted escape")
				}
				field.WriteByte(c)
				escaped = false
				continue
			}
			switch c {
			case '\\':
				field.WriteByte(c)
				escaped = true
			case '"':
				field.WriteByte(c)
				quoted = false
			default:
				field.WriteByte(c)
			}
			continue
		}
		switch c {
		case '"':
			field.WriteByte(c)
			quoted = true
		case '{':
			end := strings.IndexByte(line[i:], '}')
			if end >= 0 {
				marker := line[i : i+end+1]
				if imapParenthesizedLiteralMarkerDelimited(line, start, i) && imapLooksLikeLiteral(marker) {
					if literalIndex == nil || *literalIndex >= len(literals) {
						return "", 0, fmt.Errorf("imap literal is not available")
					}
					literal := literals[*literalIndex]
					if !imapParenthesizedLiteralValueValid(literal) {
						return "", 0, fmt.Errorf("invalid parenthesized literal value")
					}
					field.WriteString(imapQuotedString(literal))
					*literalIndex = *literalIndex + 1
					i += end
					continue
				}
			}
			field.WriteByte(c)
		case '(':
			field.WriteByte(c)
			depth++
		case ')':
			field.WriteByte(c)
			depth--
			if depth < 0 {
				return "", 0, fmt.Errorf("unbalanced parenthesized field")
			}
			if depth == 0 {
				next := i + 1
				if next < len(line) && line[next] != ' ' && line[next] != '\t' {
					return "", 0, fmt.Errorf("parenthesized field must be delimited")
				}
				return field.String(), next, nil
			}
		default:
			field.WriteByte(c)
		}
	}
	if quoted || escaped {
		return "", 0, fmt.Errorf("unterminated quoted string")
	}
	return "", 0, fmt.Errorf("unterminated parenthesized field")
}

func imapParenthesizedLiteralValueValid(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] >= 0x7f {
			return false
		}
	}
	return true
}

func imapParenthesizedLiteralMarkerDelimited(line string, start int, marker int) bool {
	if marker <= start || marker > len(line) {
		return false
	}
	prev := line[marker-1]
	return prev == '(' || prev == ' ' || prev == '\t'
}

func imapLooksLikeLiteral(field string) bool {
	if len(field) < 3 || field[0] != '{' || field[len(field)-1] != '}' {
		return false
	}
	end := len(field) - 1
	if end > 1 && field[end-1] == '+' {
		end--
	}
	if end == 1 {
		return false
	}
	for i := 1; i < end; i++ {
		if field[i] < '0' || field[i] > '9' {
			return false
		}
	}
	return true
}

func imapLooksLikeLiteralPrefix(field string) bool {
	return len(field) >= 2 && field[0] == '{'
}

func imapCommandArgumentString(line string) string {
	line = strings.TrimSpace(line)
	first := strings.IndexAny(line, " \t")
	if first < 0 {
		return ""
	}
	rest := strings.TrimLeft(line[first:], " \t")
	second := strings.IndexAny(rest, " \t")
	if second < 0 {
		return ""
	}
	return strings.TrimSpace(rest[second:])
}

func imapTagValid(tag string) bool {
	return imapAtomValid(tag) && !strings.Contains(tag, "+")
}

func imapAtomValid(tag string) bool {
	if tag == "" {
		return false
	}
	for i := 0; i < len(tag); i++ {
		switch tag[i] {
		case '(', ')', '{', ' ', '\t', '%', '*', '"', '\\', ']':
			return false
		default:
			if tag[i] < 0x20 || tag[i] >= 0x7f {
				return false
			}
		}
	}
	return true
}

func imapIDArgumentsValid(argument string) bool {
	return imapIDArgumentsValidWithLiterals(argument, nil)
}

func imapIDArgumentsValidWithLiterals(argument string, literals []string) bool {
	argument = strings.TrimSpace(argument)
	if argument == "" {
		return len(literals) == 0
	}
	if strings.EqualFold(argument, "NIL") {
		return len(literals) == 0
	}
	if len(argument) < 2 || argument[0] != '(' || argument[len(argument)-1] != ')' {
		return false
	}
	tokens, ok := imapIDListTokens(argument[1:len(argument)-1], literals)
	if !ok || len(tokens)%2 != 0 || len(tokens)/2 > 30 {
		return false
	}
	seenFields := make(map[string]struct{}, len(tokens)/2)
	for i := 0; i < len(tokens); i += 2 {
		field := tokens[i]
		value := tokens[i+1]
		if strings.EqualFold(field, "NIL") || len(field) == 0 || len(field) > 30 || len(value) > 1024 {
			return false
		}
		key := strings.ToLower(field)
		if _, ok := seenFields[key]; ok {
			return false
		}
		seenFields[key] = struct{}{}
	}
	return true
}

func imapIDListTokens(value string, literals []string) ([]string, bool) {
	tokens := make([]string, 0, 8)
	literalIndex := 0
	for i := 0; i < len(value); {
		for i < len(value) && (value[i] == ' ' || value[i] == '\t') {
			i++
		}
		if i >= len(value) {
			break
		}
		if value[i] == '"' {
			token, next, ok := imapParseQuotedToken(value, i)
			if !ok {
				return nil, false
			}
			if next < len(value) && value[next] != ' ' && value[next] != '\t' {
				return nil, false
			}
			tokens = append(tokens, token)
			i = next
			continue
		}
		start := i
		for i < len(value) && value[i] != ' ' && value[i] != '\t' {
			if value[i] == '(' || value[i] == ')' || value[i] < 0x20 || value[i] >= 0x7f {
				return nil, false
			}
			i++
		}
		token := value[start:i]
		if imapLooksLikeLiteral(token) {
			if literalIndex >= len(literals) {
				return nil, false
			}
			tokens = append(tokens, literals[literalIndex])
			literalIndex++
			continue
		}
		if !imapAtomValid(token) {
			return nil, false
		}
		tokens = append(tokens, token)
	}
	if literalIndex != len(literals) {
		return nil, false
	}
	return tokens, true
}

func imapParseQuotedToken(value string, start int) (string, int, bool) {
	i := start + 1
	var b strings.Builder
	for i < len(value) {
		switch value[i] {
		case '\\':
			i++
			if i >= len(value) {
				return "", 0, false
			}
			if value[i] != '\\' && value[i] != '"' {
				return "", 0, false
			}
			b.WriteByte(value[i])
			i++
		case '"':
			return b.String(), i + 1, true
		default:
			if value[i] < 0x20 || value[i] >= 0x7f {
				return "", 0, false
			}
			b.WriteByte(value[i])
			i++
		}
	}
	return "", 0, false
}

func imapCommandLiteralSize(line string) (int, bool, bool, error) {
	trimmed := strings.TrimRight(line, "\r\n")
	if !strings.HasSuffix(trimmed, "}") {
		return 0, false, false, nil
	}
	start := strings.LastIndex(trimmed, "{")
	if start < 0 {
		return 0, false, false, nil
	}
	if start > 0 && trimmed[start-1] != ' ' && trimmed[start-1] != '\t' && trimmed[start-1] != '(' {
		return 0, false, false, nil
	}
	value := trimmed[start+1 : len(trimmed)-1]
	if value == "" {
		return 0, false, false, fmt.Errorf("imap literal size is required")
	}
	nonSync := strings.HasSuffix(value, "+")
	if nonSync {
		value = strings.TrimSuffix(value, "+")
		if value == "" {
			return 0, false, true, fmt.Errorf("imap literal size is required")
		}
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, nonSync, true, errIMAPCommandLiteralInvalid
	}
	var size int64
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, nonSync, true, errIMAPCommandLiteralInvalid
		}
		size = size*10 + int64(value[i]-'0')
		if size > maxIMAPCommandLiteralBytes {
			return 0, nonSync, true, errIMAPCommandLiteralTooLarge
		}
	}
	return int(size), nonSync, true, nil
}
