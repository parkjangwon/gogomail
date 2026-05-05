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
	"net"
	stdmail "net/mail"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	messageparse "github.com/gogomail/gogomail/internal/message"
)

type ServerOptions struct {
	Addr              string
	Backend           Backend
	TLSConfig         *tls.Config
	AllowInsecureAuth bool
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
	for {
		conn, err := listener.Accept()
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return ErrServerClosed
			}
			return err
		}
		go func() {
			_ = s.ServeConn(conn)
		}()
	}
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
	if _, err := writer.WriteString("* OK gogomail IMAP4rev1 service ready\r\n"); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	state := imapConnState{}
	_, state.tlsActive = conn.(*tls.Conn)
	defer state.closeSubscription()
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if len(line) > 8192 {
			return fmt.Errorf("imap command line is too long")
		}
		done, err := s.handleLine(writer, line, &state)
		if err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if state.pendingIdleTag != "" {
			if err := s.serveIdle(reader, writer, &state); err != nil {
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

type imapConnState struct {
	session          *Session
	selectedMailbox  MailboxID
	selectedMessages uint32
	readOnly         bool
	pendingAuthTag   string
	pendingIdleTag   string
	startTLS         bool
	tlsActive        bool
	events           <-chan MailboxEvent
	cancelEvents     func()
}

func (s *Server) handleLine(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	if state.pendingIdleTag != "" {
		return s.handleIdleDone(writer, strings.TrimRight(line, "\r\n"), state)
	}
	if state.pendingAuthTag != "" {
		return s.handleAuthenticatePlainResponse(writer, strings.TrimRight(line, "\r\n"), state)
	}
	fields, parseErr := parseIMAPFields(strings.TrimRight(line, "\r\n"))
	if parseErr != nil {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	if len(fields) < 2 {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	tag := fields[0]
	command := strings.ToUpper(fields[1])
	switch command {
	case "CAPABILITY":
		if _, err := writer.WriteString("* CAPABILITY " + strings.Join(s.imapCapabilities(state), " ") + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK CAPABILITY completed\r\n")
		return false, err
	case "NOOP":
		if err := s.drainMailboxEvents(writer, state); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK NOOP completed\r\n")
		return false, err
	case "ID":
		if len(fields) < 3 {
			_, err := writer.WriteString(tag + " BAD ID requires NIL or parameter list\r\n")
			return false, err
		}
		if _, err := writer.WriteString(`* ID ("name" "gogomail")` + "\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK ID completed\r\n")
		return false, err
	case "STARTTLS":
		if state.session != nil {
			_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
			return false, err
		}
		if state.tlsActive || s.options.TLSConfig == nil {
			_, err := writer.WriteString(tag + " BAD STARTTLS is unavailable\r\n")
			return false, err
		}
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD STARTTLS does not accept arguments\r\n")
			return false, err
		}
		state.startTLS = true
		tlsState := *state
		tlsState.startTLS = false
		tlsState.tlsActive = true
		_, err := writer.WriteString(tag + " OK [CAPABILITY " + strings.Join(s.imapCapabilities(&tlsState), " ") + "] Begin TLS negotiation now\r\n")
		return false, err
	case "NAMESPACE":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD NAMESPACE does not accept arguments\r\n")
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
		if !s.authAllowed(state) {
			_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n")
			return false, err
		}
		if len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD LOGIN requires username and password atoms\r\n")
			return false, err
		}
		authSession, err := s.options.Backend.Authenticate(context.Background(), fields[2], fields[3])
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO LOGIN failed\r\n")
			return false, writeErr
		}
		state.session = &authSession
		_, err = writer.WriteString(tag + " OK LOGIN completed\r\n")
		return false, err
	case "AUTHENTICATE":
		if state.session != nil {
			_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
			return false, err
		}
		if !s.authAllowed(state) {
			_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n")
			return false, err
		}
		if (len(fields) != 3 && len(fields) != 4) || strings.ToUpper(fields[2]) != "PLAIN" {
			_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is unsupported\r\n")
			return false, err
		}
		if len(fields) == 4 {
			return s.completeAuthenticatePlain(writer, tag, fields[3], state)
		}
		state.pendingAuthTag = tag
		_, err := writer.WriteString("+ \r\n")
		return false, err
	case "SELECT", "EXAMINE":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if len(fields) != 3 {
			_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom\r\n")
			return false, err
		}
		mailboxState, err := s.options.Backend.SelectMailbox(context.Background(), SelectMailboxRequest{
			UserID:    state.session.UserID,
			MailboxID: MailboxID(fields[2]),
		})
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO SELECT failed\r\n")
			return false, writeErr
		}
		if _, err := writer.WriteString("* FLAGS " + imapFlagList(mailboxState.PermanentFlags) + "\r\n"); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", mailboxState.Messages)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d RECENT\r\n", mailboxState.Recent)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDVALIDITY %d] UIDs valid\r\n", mailboxState.UIDValidity)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDNEXT %d] Predicted next UID\r\n", mailboxState.UIDNext)); err != nil {
			return false, err
		}
		state.closeSubscription()
		state.selectedMailbox = MailboxID(fields[2])
		state.selectedMessages = mailboxState.Messages
		state.readOnly = command == "EXAMINE"
		events, cancel, err := s.options.Backend.Subscribe(context.Background(), state.session.UserID, state.selectedMailbox)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO SELECT failed\r\n")
			return false, writeErr
		}
		state.events = events
		state.cancelEvents = cancel
		if state.readOnly {
			if _, err := writer.WriteString("* OK [PERMANENTFLAGS ()] No permanent flags permitted\r\n"); err != nil {
				return false, err
			}
			_, err = writer.WriteString(tag + " OK [READ-ONLY] EXAMINE completed\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* OK [PERMANENTFLAGS " + imapFlagList(mailboxState.PermanentFlags) + "] Permanent flags\r\n"); err != nil {
			return false, err
		}
		_, err = writer.WriteString(tag + " OK [READ-WRITE] SELECT completed\r\n")
		return false, err
	case "LIST":
		return s.handleList(writer, tag, fields, state, false)
	case "LSUB":
		return s.handleList(writer, tag, fields, state, true)
	case "CREATE", "DELETE", "RENAME":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " NO " + command + " is not supported\r\n")
		return false, err
	case "SUBSCRIBE", "UNSUBSCRIBE":
		return s.handleSubscriptionCommand(writer, tag, fields, state, command)
	case "STATUS":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if len(fields) < 4 {
			_, err := writer.WriteString(tag + " BAD STATUS requires mailbox and status item atoms\r\n")
			return false, err
		}
		statusItems, ok := imapStatusItems(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD STATUS item is unsupported\r\n")
			return false, err
		}
		mailbox, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(fields[2]))
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO STATUS failed\r\n")
			return false, writeErr
		}
		if _, err := writer.WriteString(fmt.Sprintf("* STATUS %s (%s)\r\n", imapQuotedString(imapMailboxDisplayName(mailbox)), imapStatusData(mailbox, statusItems))); err != nil {
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
	case "STORE":
		if state.readOnly {
			_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
			return false, err
		}
		return s.handleStore(writer, tag, fields, state)
	case "COPY":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " NO COPY is not supported\r\n")
		return false, err
	case "CHECK":
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
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD IDLE does not accept arguments\r\n")
			return false, err
		}
		state.pendingIdleTag = tag
		_, err := writer.WriteString("+ idling\r\n")
		return false, err
	case "CLOSE":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.selectedMailbox = ""
		state.selectedMessages = 0
		state.readOnly = false
		state.closeSubscription()
		_, err := writer.WriteString(tag + " OK CLOSE completed\r\n")
		return false, err
	case "UNSELECT":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.selectedMailbox = ""
		state.selectedMessages = 0
		state.readOnly = false
		state.closeSubscription()
		_, err := writer.WriteString(tag + " OK UNSELECT completed\r\n")
		return false, err
	case "EXPUNGE":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " NO EXPUNGE is not supported\r\n")
		return false, err
	case "MOVE":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " NO MOVE is not supported\r\n")
		return false, err
	case "APPEND":
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " NO APPEND is not supported\r\n")
		return false, err
	case "LOGOUT":
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

func (s *Server) handleList(writer *bufio.Writer, tag string, fields []string, state *imapConnState, subscribed bool) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	command := "LIST"
	if subscribed {
		command = "LSUB"
	}
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires reference and mailbox pattern atoms\r\n")
		return false, err
	}
	mailboxes, err := s.options.Backend.ListMailboxes(context.Background(), ListMailboxesRequest{UserID: state.session.UserID})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	pattern := imapListPattern(fields[2], fields[3])
	if pattern == "" {
		if _, err := writer.WriteString("* " + command + ` (\Noselect) "/" ""` + "\r\n"); err != nil {
			return false, err
		}
		_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
		return false, err
	}
	for _, mailbox := range mailboxes {
		displayName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		if !imapMailboxMatchesPattern(displayName, pattern) {
			continue
		}
		if _, err := writer.WriteString("* " + command + ` (\HasNoChildren) "/" ` + imapQuotedString(displayName) + "\r\n"); err != nil {
			return false, err
		}
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) handleSubscriptionCommand(writer *bufio.Writer, tag string, fields []string, state *imapConnState, command string) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom\r\n")
		return false, err
	}
	if _, err := s.options.Backend.GetMailbox(context.Background(), state.session.UserID, MailboxID(fields[2])); err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	_, err := writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func (s *Server) handleIdleDone(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingIdleTag
	if strings.ToUpper(strings.TrimSpace(line)) != "DONE" {
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
		line, err := reader.ReadString('\n')
		lineCh <- idleLineResult{line: line, err: err}
	}()
	for state.pendingIdleTag != "" {
		select {
		case result := <-lineCh:
			if result.err != nil {
				if errors.Is(result.err, io.EOF) {
					return nil
				}
				return result.err
			}
			if len(result.line) > 8192 {
				return fmt.Errorf("imap command line is too long")
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

func (s *Server) writeMailboxEvent(writer *bufio.Writer, state *imapConnState, event MailboxEvent) error {
	switch event.Type {
	case MailboxEventExists:
		if event.Messages > 0 {
			state.selectedMessages = event.Messages
		} else {
			state.selectedMessages++
		}
		_, err := writer.WriteString(fmt.Sprintf("* %d EXISTS\r\n", state.selectedMessages))
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
		sequenceNumber, ok := imapSequenceNumber(message.Summary)
		if !ok {
			return fmt.Errorf("imap event sequence number is unavailable")
		}
		_, err = writer.WriteString(fmt.Sprintf("* %d FETCH (UID %d FLAGS %s)\r\n", sequenceNumber, message.Summary.UID, imapFlagList(message.Summary.Flags.IMAPFlags())))
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

func (s *Server) handleAuthenticatePlainResponse(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingAuthTag
	state.pendingAuthTag = ""
	if strings.TrimSpace(line) == "*" {
		_, err := writer.WriteString(tag + " NO AUTHENTICATE canceled\r\n")
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
		_, writeErr := writer.WriteString(tag + " NO AUTHENTICATE failed\r\n")
		return false, writeErr
	}
	state.session = &authSession
	_, err = writer.WriteString(tag + " OK AUTHENTICATE completed\r\n")
	return false, err
}

func decodeSASLPlain(value string) (string, string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", "", false
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 || parts[1] == "" {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func (s *Server) handleUIDLine(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD UID command not implemented\r\n")
		return false, err
	}
	switch strings.ToUpper(fields[2]) {
	case "FETCH":
		return s.handleUIDFetch(writer, tag, fields, state)
	case "SEARCH":
		return s.handleSearch(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "STORE":
		if state.readOnly {
			_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
			return false, err
		}
		return s.handleUIDStore(writer, tag, fields, state)
	case "EXPUNGE":
		_, err := writer.WriteString(tag + " NO UID EXPUNGE is not supported\r\n")
		return false, err
	case "COPY":
		_, err := writer.WriteString(tag + " NO UID COPY is not supported\r\n")
		return false, err
	case "MOVE":
		_, err := writer.WriteString(tag + " NO UID MOVE is not supported\r\n")
		return false, err
	default:
		_, err := writer.WriteString(tag + " BAD UID command not implemented\r\n")
		return false, err
	}
}

func (s *Server) handleSearch(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	criteria, charsetOK := imapSearchCriteria(fields[2:])
	if !charsetOK {
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] SEARCH charset is unsupported\r\n")
		return false, err
	}
	if len(criteria) == 0 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(context.Background(), ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	results, ok, err := s.imapSearchResults(context.Background(), state, criteria, messages, uidMode)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
		return false, err
	}
	if _, err := writer.WriteString("* SEARCH" + imapSearchResultSuffix(results) + "\r\n"); err != nil {
		return false, err
	}
	completion := "SEARCH"
	if uidMode {
		completion = "UID SEARCH"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

func imapSearchCriteria(criteria []string) ([]string, bool) {
	if len(criteria) >= 2 && strings.EqualFold(criteria[0], "CHARSET") {
		charset := strings.ToUpper(strings.Trim(criteria[1], `"`))
		switch charset {
		case "US-ASCII", "UTF-8":
			return criteria[2:], true
		default:
			return nil, false
		}
	}
	return criteria, true
}

func (s *Server) imapSearchResults(ctx context.Context, state *imapConnState, criteria []string, messages []MessageSummary, uidMode bool) ([]uint32, bool, error) {
	if len(criteria) == 0 {
		return nil, false, nil
	}
	predicates := make([]imapSearchPredicate, 0, len(criteria))
	for i := 0; i < len(criteria); {
		predicate, consumed, ok := imapParseSearchPredicate(criteria[i:], state.selectedMessages)
		if !ok {
			return nil, false, nil
		}
		if predicate != nil {
			predicates = append(predicates, predicate)
		}
		i += consumed
	}
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		matches, err := s.imapMessageMatchesSearchPredicates(ctx, state, summary, i, predicates)
		if err != nil {
			return nil, true, err
		}
		if !matches {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		results = append(results, imapSearchSequenceNumber(summary, i))
	}
	return results, true, nil
}

type imapSearchPredicate func(context.Context, *Server, *imapConnState, MessageSummary, int) (bool, error)

func imapParseSearchPredicate(criteria []string, maxSequence uint32) (imapSearchPredicate, int, bool) {
	if len(criteria) == 0 {
		return nil, 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "ALL":
		return nil, 1, true
	case "NOT":
		predicate, consumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence)
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
		left, leftConsumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence)
		if !ok {
			return nil, 0, false
		}
		right, rightConsumed, ok := imapParseSearchPredicate(criteria[1+leftConsumed:], maxSequence)
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
		uids, ok := parseIMAPUIDSet(criteria[1])
		if !ok {
			return nil, 0, false
		}
		allowed := make(map[UID]struct{}, len(uids))
		for _, uid := range uids {
			allowed[uid] = struct{}{}
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			_, ok := allowed[summary.UID]
			return ok, nil
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
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, _ MessageSummary, _ int) (bool, error) {
			return criterion == "UNKEYWORD", nil
		}, 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query := criteria[1]
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesTextSearch(summary, criterion, strings.ToLower(strings.Trim(query, `"`))), nil
		}, 2, true
	case "HEADER":
		if len(criteria) < 3 {
			return nil, 0, false
		}
		fieldName := strings.Trim(criteria[1], `"`)
		query := strings.ToLower(strings.Trim(criteria[2], `"`))
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesHeaderSearch(ctx, state, summary, fieldName, query)
		}, 3, true
	case "BODY", "TEXT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query := strings.ToLower(strings.Trim(criteria[1], `"`))
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesBodySearch(ctx, state, summary, criterion, query)
		}, 2, true
	default:
		sequenceNumbers, ok := parseIMAPSequenceSet(criteria[0], maxSequence)
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
	value = strings.Trim(value, `"`)
	if strings.TrimSpace(value) == "" {
		return false
	}
	if strings.ContainsAny(value, "(){ %*\r\n\t") {
		return false
	}
	return true
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
		return false
	case "UNDELETED":
		return true
	case "RECENT", "NEW":
		return false
	case "OLD":
		return true
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
	day, err := time.Parse("02-Jan-2006", strings.Trim(value, `"`))
	if err != nil {
		return time.Time{}, false
	}
	return day, true
}

func parseIMAPSearchSize(value string) (int64, bool) {
	size, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil || size < 0 {
		return 0, false
	}
	return size, true
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
	query = strings.ToLower(strings.Trim(query, `"`))
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
	if query == "" {
		return false
	}
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

const maxIMAPSearchLiteralBytes = 1 << 20

func (s *Server) imapMessageMatchesBodySearch(ctx context.Context, state *imapConnState, summary MessageSummary, criterion string, query string) (bool, error) {
	if s == nil || state == nil || state.session == nil || query == "" || summary.UID == 0 {
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

func imapSearchResultSuffix(results []uint32) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, strconv.FormatUint(uint64(result), 10))
	}
	return " " + strings.Join(parts, " ")
}

func (s *Server) handleUIDFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	uids, ok := parseIMAPUIDSet(fields[3])
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
		return false, err
	}
	return s.writeFetchResponses(writer, tag, fields[4:], state, uids, "UID FETCH")
}

func (s *Server) handleFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD FETCH requires sequence set and data items\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSet(fields[2], state.selectedMessages)
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

func (s *Server) writeFetchResponses(writer *bufio.Writer, tag string, items []string, state *imapConnState, uids []UID, completionCommand string) (bool, error) {
	items = imapExpandFetchItems(items)
	requestsBody := imapFetchRequestsBody(items)
	partial, requestsPartialBody := imapFetchPartialBody(items)
	requestsHeader := imapFetchRequestsHeader(items)
	requestsText := imapFetchRequestsText(items)
	requestsPartText := imapFetchRequestsPartText(items)
	requestsPartMIME := imapFetchRequestsPartMIME(items)
	headerFields, requestsHeaderFields := imapFetchHeaderFields(items)
	headerFieldsNot, requestsHeaderFieldsNot := imapFetchHeaderFieldsNot(items)
	requestsEnvelope := imapFetchRequestsEnvelope(items)
	requestsInternalDate := imapFetchRequestsInternalDate(items)
	requestsBodyAttribute := imapFetchRequestsBodyAttribute(items)
	requestsBodyStructure := imapFetchRequestsBodyStructure(items)
	for _, uid := range uids {
		fetchReq := FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		}
		message, err := s.options.Backend.FetchMessage(context.Background(), fetchReq)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO UID FETCH failed\r\n")
			return false, writeErr
		}
		summary := message.Summary
		if summary.UID == 0 {
			summary.UID = uid
		}
		requestsLiteral := requestsBody || requestsPartialBody || requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME
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
			_, err := writer.WriteString(tag + " NO UID FETCH sequence number is unavailable\r\n")
			return false, err
		}
		if requestsLiteral {
			if message.Body == nil {
				_, err := writer.WriteString(tag + " NO UID FETCH body is unavailable\r\n")
				return false, err
			}
			body := message.Body
			if summary.Size < 0 {
				_ = body.Close()
				_, err := writer.WriteString(tag + " NO UID FETCH body size is unavailable\r\n")
				return false, err
			}
			if requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME {
				literal, err := readIMAPSectionLiteral(body, requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot)
				if err != nil {
					_ = body.Close()
					return false, err
				}
				if requestsPartMIME {
					literal = []byte("\r\n")
				}
				if requestsHeaderFields {
					literal = filterIMAPHeaderFields(literal, headerFields, false)
				}
				if requestsHeaderFieldsNot {
					literal = filterIMAPHeaderFields(literal, headerFieldsNot, true)
				}
				if err := body.Close(); err != nil {
					return false, err
				}
				attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
				section := "TEXT"
				if requestsPartText {
					section = "1"
				}
				if requestsPartMIME {
					section = "1.MIME"
				}
				if requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot {
					section = "HEADER"
				}
				if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s BODY[%s] {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), section, len(literal))); err != nil {
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
			attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
			if requestsPartialBody {
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
				if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s BODY[]<%d> {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), partial.offset, count)); err != nil {
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
			if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s BODY[] {%d}\r\n", sequenceNumber, strings.Join(attributes, " "), summary.Size)); err != nil {
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
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s)\r\n", sequenceNumber, strings.Join(imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure), " "))); err != nil {
			return false, err
		}
	}
	_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func parseIMAPUIDSet(value string) ([]UID, bool) {
	const maxUIDSetItems = 500

	seen := make(map[UID]struct{})
	uids := make([]UID, 0, 1)
	for _, rawPart := range strings.Split(strings.TrimSpace(value), ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" || strings.Contains(part, "*") {
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
			if len(uids) > maxUIDSetItems {
				return nil, false
			}
			if uid == UID(^uint32(0)) {
				break
			}
		}
	}
	return uids, len(uids) > 0
}

func parseIMAPUIDSetNumber(value string) (UID, bool) {
	uid64, err := strconv.ParseUint(strings.TrimSpace(value), 10, 32)
	if err != nil || uid64 == 0 {
		return 0, false
	}
	return UID(uid64), true
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

func parseIMAPBoundedNumberSet(value string, maxValue uint32, allowStar bool) ([]UID, bool) {
	const maxSetItems = 500

	seen := make(map[UID]struct{})
	values := make([]UID, 0, 1)
	for _, rawPart := range strings.Split(strings.TrimSpace(value), ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
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
			if len(values) > maxSetItems {
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
	value = strings.TrimSpace(value)
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

func imapFetchPartialBody(items []string) (imapPartialBodyRequest, bool) {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if !strings.HasPrefix(token, "BODY[]<") && !strings.HasPrefix(token, "BODY.PEEK[]<") {
				continue
			}
			start := strings.Index(token, "<")
			end := strings.LastIndex(token, ">")
			if start < 0 || end <= start {
				return imapPartialBodyRequest{}, false
			}
			offsetText, countText, ok := strings.Cut(token[start+1:end], ".")
			if !ok {
				return imapPartialBodyRequest{}, false
			}
			offset, err := strconv.ParseUint(offsetText, 10, 63)
			if err != nil {
				return imapPartialBodyRequest{}, false
			}
			count, err := strconv.ParseUint(countText, 10, 31)
			if err != nil {
				return imapPartialBodyRequest{}, false
			}
			return imapPartialBodyRequest{offset: offset, count: count}, true
		}
	}
	return imapPartialBodyRequest{}, false
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
	fieldsText := joined[idx+start+1 : idx+start+1+end]
	fields := make([]string, 0)
	for _, field := range strings.Fields(fieldsText) {
		field = strings.Trim(field, "[]")
		if field != "" {
			fields = append(fields, field)
		}
	}
	return fields, len(fields) > 0
}

func filterIMAPHeaderFields(header []byte, fields []string, exclude bool) []byte {
	if len(header) == 0 || len(fields) == 0 {
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
		_, found := allowed[strings.ToUpper(strings.TrimSpace(name))]
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
				rest, err := io.ReadAll(reader)
				if err != nil {
					return nil, err
				}
				return append(data[end:], rest...), nil
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

func imapFetchRequestsBodyStructure(items []string) bool {
	return imapFetchRequestsToken(items, "BODYSTRUCTURE")
}

func imapFetchRequestsBodyAttribute(items []string) bool {
	return imapFetchRequestsToken(items, "BODY")
}

func imapFetchRequestsToken(items []string, want string) bool {
	for _, item := range items {
		for _, token := range strings.Fields(strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")) {
			if token == want {
				return true
			}
		}
	}
	return false
}

func imapFetchAttributes(summary MessageSummary, includeEnvelope bool, includeInternalDate bool, includeBody bool, includeBodyStructure bool, bodyAttribute string, bodyStructure string) []string {
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
	return "(" + strings.Join([]string{
		imapNString(imapEnvelopeDate(date)),
		imapNString(envelope.Subject),
		imapAddressList(envelope.From),
		imapAddressList(envelope.From),
		imapAddressList(envelope.From),
		imapAddressList(envelope.To),
		imapAddressList(envelope.Cc),
		imapAddressList(envelope.Bcc),
		"NIL",
		imapNString(envelope.MessageID),
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
	parts := make([]string, 0, len(addresses))
	for _, address := range addresses {
		parts = append(parts, "("+strings.Join([]string{
			imapNString(address.Name),
			"NIL",
			imapNString(address.Mailbox),
			imapNString(address.Host),
		}, " ")+")")
	}
	return "(" + strings.Join(parts, " ") + ")"
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
		childBodies = append(childBodies, imapQuotedString(imapMIMESubtype(part.MediaSubtype)))
		if extended {
			childBodies = append(childBodies, imapMIMEBodyParameterList(part.Params), "NIL", "NIL", "NIL")
		}
		return "(" + strings.Join(childBodies, " ") + ")"
	}
	return imapMIMESinglePartBody(part, fallbackSize, extended)
}

func imapMIMESinglePartBody(part messageparse.MIMEPart, fallbackSize int64, extended bool) string {
	mediaType := imapMIMEToken(part.MediaType, "TEXT")
	mediaSubtype := imapMIMEToken(part.MediaSubtype, "PLAIN")
	size := part.Size
	if size == 0 && fallbackSize > 0 {
		size = fallbackSize
	}
	fields := []string{
		imapQuotedString(mediaType),
		imapQuotedString(mediaSubtype),
		imapMIMEBodyParameterList(part.Params),
		imapNString(part.ContentID),
		imapNString(part.Description),
		imapQuotedString(imapMIMEToken(part.Encoding, "7BIT")),
		fmt.Sprintf("%d", maxInt64(size, 0)),
	}
	if mediaType == "TEXT" {
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

func imapMIMEBodyDisposition(part messageparse.MIMEPart) string {
	if strings.TrimSpace(part.Disposition) == "" {
		return "NIL"
	}
	return "(" + imapQuotedString(imapMIMEToken(part.Disposition, "ATTACHMENT")) + " " + imapMIMEBodyParameterList(part.DispositionParams) + ")"
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
		imapNString(metadata.id),
		imapNString(metadata.description),
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
		metadata.encoding = strings.ToUpper(encoding)
	}
	metadata.id = strings.TrimSpace(message.Header.Get("Content-ID"))
	metadata.description = strings.TrimSpace(message.Header.Get("Content-Description"))
	return metadata
}

func imapMediaTypeParts(value string) (string, string, bool) {
	typ, subtype, ok := strings.Cut(strings.TrimSpace(value), "/")
	typ = strings.ToUpper(strings.TrimSpace(typ))
	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	if !ok || typ == "" || subtype == "" || strings.ContainsAny(typ+subtype, " \t\r\n") {
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
		if key == "" || value == "" || strings.ContainsAny(key, " \t\r\n") {
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
	values := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		values = append(values, imapQuotedString(strings.ToUpper(key)), imapQuotedString(params[key]))
	}
	return "(" + strings.Join(values, " ") + ")"
}

func imapMIMEToken(value string, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return fallback
	}
	return value
}

func imapMIMESubtype(value string) string {
	return imapMIMEToken(value, "MIXED")
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func (s *Server) imapCapabilities(state *imapConnState) []string {
	capabilities := []string{"IMAP4rev1", "IDLE", "ID", "NAMESPACE", "UNSELECT"}
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
	uids, ok := parseIMAPUIDSet(fields[3])
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
		return false, err
	}
	mode, silent, ok := imapStoreMode(fields[4])
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE mode is unsupported\r\n")
		return false, err
	}
	flags, ok := imapStoreFlags(strings.Join(fields[5:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE flags are unsupported\r\n")
		return false, err
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, "UID STORE")
}

func (s *Server) handleStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if len(fields) < 5 {
		_, err := writer.WriteString(tag + " BAD STORE requires sequence set, mode, and flags\r\n")
		return false, err
	}
	sequenceNumbers, ok := parseIMAPSequenceSet(fields[2], state.selectedMessages)
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(context.Background(), state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO STORE failed\r\n")
		return false, writeErr
	}
	mode, silent, ok := imapStoreMode(fields[3])
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE mode is unsupported\r\n")
		return false, err
	}
	flags, ok := imapStoreFlags(strings.Join(fields[4:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE flags are unsupported\r\n")
		return false, err
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, "STORE")
}

func (s *Server) writeStoreResponses(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, flags MessageFlags, mode StoreFlagsMode, silent bool, completionCommand string) (bool, error) {
	summaries, err := s.options.Backend.StoreFlags(context.Background(), StoreFlagsRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      uids,
		Flags:     flags,
		Mode:      mode,
	})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	if silent {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	for _, summary := range summaries {
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (UID %d FLAGS %s)\r\n", sequenceNumber, summary.UID, imapFlagList(summary.Flags.IMAPFlags()))); err != nil {
			return false, err
		}
	}
	_, err = writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func imapSequenceNumber(summary MessageSummary) (uint32, bool) {
	if summary.SequenceNumber == 0 {
		return 0, false
	}
	return summary.SequenceNumber, true
}

func imapStoreMode(value string) (StoreFlagsMode, bool, bool) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
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
	var flags MessageFlags
	ok := false
	for _, raw := range strings.Fields(strings.Trim(value, "()")) {
		switch CanonicalIMAPFlag(raw) {
		case FlagSeen:
			flags.Read = true
			ok = true
		case FlagFlagged:
			flags.Starred = true
			ok = true
		case FlagAnswered:
			flags.Answered = true
			ok = true
		case FlagDraft:
			flags.Draft = true
			ok = true
		default:
			return MessageFlags{}, false
		}
	}
	return flags, ok
}

func imapMailboxDisplayName(mailbox Mailbox) string {
	if strings.TrimSpace(mailbox.FullPath) != "" {
		return strings.TrimSpace(mailbox.FullPath)
	}
	if strings.TrimSpace(mailbox.Name) != "" {
		return strings.TrimSpace(mailbox.Name)
	}
	return strings.TrimSpace(string(mailbox.ID))
}

func imapMailboxWireName(value string) string {
	value = strings.ToValidUTF8(value, "")
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	return strings.Join(strings.Fields(value), " ")
}

func imapListPattern(reference string, pattern string) string {
	reference = strings.Trim(reference, `"`)
	pattern = strings.Trim(pattern, `"`)
	if reference == "" || pattern == "" || strings.HasPrefix(pattern, "/") {
		return pattern
	}
	return strings.TrimRight(reference, "/") + "/" + pattern
}

func imapMailboxMatchesPattern(name string, pattern string) bool {
	if pattern == "" {
		return name == ""
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
	matched, err := regexp.MatchString(b.String(), name)
	return err == nil && matched
}

func imapStatusItems(items []string) ([]string, bool) {
	out := make([]string, 0, len(items))
	for _, raw := range items {
		for _, token := range strings.Fields(strings.Trim(raw, "()")) {
			item := strings.ToUpper(strings.TrimSpace(token))
			switch item {
			case "MESSAGES", "RECENT", "UIDNEXT", "UIDVALIDITY", "UNSEEN":
				out = append(out, item)
			default:
				return nil, false
			}
		}
	}
	return out, len(out) > 0
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
	value = strings.ToValidUTF8(value, "")
	value = strings.ReplaceAll(value, `\`, `\\`)
	value = strings.ReplaceAll(value, `"`, `\"`)
	value = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	return `"` + strings.Join(strings.Fields(value), " ") + `"`
}

func parseIMAPFields(line string) ([]string, error) {
	fields := make([]string, 0, 4)
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
					b.WriteByte(line[i])
					i++
				case '"':
					i++
					fields = append(fields, b.String())
					closed = true
				default:
					if line[i] < 0x20 || line[i] == 0x7f {
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
		start := i
		for i < len(line) && line[i] != ' ' && line[i] != '\t' {
			i++
		}
		field := line[start:i]
		if imapLooksLikeLiteral(field) {
			return nil, fmt.Errorf("imap literals are not supported")
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func imapLooksLikeLiteral(field string) bool {
	if len(field) < 3 || field[0] != '{' || field[len(field)-1] != '}' {
		return false
	}
	for i := 1; i < len(field)-1; i++ {
		if field[i] < '0' || field[i] > '9' {
			return false
		}
	}
	return true
}
