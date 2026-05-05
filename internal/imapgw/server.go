package imapgw

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
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
}

func (s *Server) handleLine(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
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
		_, err := writer.WriteString(tag + " OK NOOP completed\r\n")
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
		if len(fields) != 3 || strings.ToUpper(fields[2]) != "PLAIN" {
			_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is unsupported\r\n")
			return false, err
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
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDVALIDITY %d] UIDs valid\r\n", mailboxState.UIDValidity)); err != nil {
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* OK [UIDNEXT %d] Predicted next UID\r\n", mailboxState.UIDNext)); err != nil {
			return false, err
		}
		state.selectedMailbox = MailboxID(fields[2])
		state.selectedMessages = mailboxState.Messages
		state.readOnly = command == "EXAMINE"
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
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD LIST requires reference and mailbox pattern atoms\r\n")
			return false, err
		}
		mailboxes, err := s.options.Backend.ListMailboxes(context.Background(), ListMailboxesRequest{UserID: state.session.UserID})
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO LIST failed\r\n")
			return false, writeErr
		}
		pattern := imapListPattern(fields[2], fields[3])
		for _, mailbox := range mailboxes {
			displayName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
			if !imapMailboxMatchesPattern(displayName, pattern) {
				continue
			}
			if _, err := writer.WriteString(`* LIST (\HasNoChildren) "/" ` + imapQuotedString(displayName) + "\r\n"); err != nil {
				return false, err
			}
		}
		_, err = writer.WriteString(tag + " OK LIST completed\r\n")
		return false, err
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
		_, err := writer.WriteString(tag + " OK CLOSE completed\r\n")
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

func (s *Server) handleAuthenticatePlainResponse(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingAuthTag
	state.pendingAuthTag = ""
	if strings.TrimSpace(line) == "*" {
		_, err := writer.WriteString(tag + " NO AUTHENTICATE canceled\r\n")
		return false, err
	}
	username, password, ok := decodeSASLPlain(line)
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
	case "STORE":
		if state.readOnly {
			_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
			return false, err
		}
		return s.handleUIDStore(writer, tag, fields, state)
	default:
		_, err := writer.WriteString(tag + " BAD UID command not implemented\r\n")
		return false, err
	}
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
	requestsBody := imapFetchRequestsBody(items)
	requestsEnvelope := imapFetchRequestsEnvelope(items)
	requestsInternalDate := imapFetchRequestsInternalDate(items)
	for _, uid := range uids {
		message, err := s.options.Backend.FetchMessage(context.Background(), FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		})
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO UID FETCH failed\r\n")
			return false, writeErr
		}
		summary := message.Summary
		if summary.UID == 0 {
			summary.UID = uid
		}
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO UID FETCH sequence number is unavailable\r\n")
			return false, err
		}
		if requestsBody {
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
			attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate)
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
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (%s)\r\n", sequenceNumber, strings.Join(imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate), " "))); err != nil {
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

func imapFetchRequestsEnvelope(items []string) bool {
	return imapFetchRequestsToken(items, "ENVELOPE")
}

func imapFetchRequestsInternalDate(items []string) bool {
	return imapFetchRequestsToken(items, "INTERNALDATE")
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

func imapFetchAttributes(summary MessageSummary, includeEnvelope bool, includeInternalDate bool) []string {
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

func (s *Server) imapCapabilities(state *imapConnState) []string {
	capabilities := []string{"IMAP4rev1"}
	if state == nil || state.session == nil {
		capabilities = append(capabilities, "AUTH=PLAIN")
	}
	return capabilities
}

func (s *Server) handleUIDStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 6 {
		_, err := writer.WriteString(tag + " BAD UID STORE requires UID, mode, and flags\r\n")
		return false, err
	}
	uid64, err := strconv.ParseUint(fields[3], 10, 32)
	if err != nil || uid64 == 0 {
		_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID\r\n")
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
	summaries, err := s.options.Backend.StoreFlags(context.Background(), StoreFlagsRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      []UID{UID(uid64)},
		Flags:     flags,
		Mode:      mode,
	})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID STORE failed\r\n")
		return false, writeErr
	}
	if silent {
		_, err := writer.WriteString(tag + " OK UID STORE completed\r\n")
		return false, err
	}
	for _, summary := range summaries {
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO UID STORE sequence number is unavailable\r\n")
			return false, err
		}
		if _, err := writer.WriteString(fmt.Sprintf("* %d FETCH (UID %d FLAGS %s)\r\n", sequenceNumber, summary.UID, imapFlagList(summary.Flags.IMAPFlags()))); err != nil {
			return false, err
		}
	}
	_, err = writer.WriteString(tag + " OK UID STORE completed\r\n")
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
