package imapgw

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
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
	MaxConnections    int
	ReadTimeout       time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
}

// gatewayMetrics is the minimal interface imapgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordConnect(userID string)
	RecordDisconnect()
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
	RecordConnectionLimitExceeded()
}

type Server struct {
	options     ServerOptions
	mu          sync.Mutex
	listener    net.Listener
	metrics     gatewayMetrics
	authTracker *authFailureTracker
	wg          sync.WaitGroup
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
	if opts.ReadTimeout < 0 {
		return nil, fmt.Errorf("imap read timeout must not be negative")
	}
	if opts.WriteTimeout < 0 {
		return nil, fmt.Errorf("imap write timeout must not be negative")
	}
	if opts.IdleTimeout < 0 {
		return nil, fmt.Errorf("imap idle timeout must not be negative")
	}
	opts.Addr = addr
	return &Server{options: opts, authTracker: newAuthFailureTracker()}, nil
}

func (s *Server) Options() ServerOptions {
	if s == nil {
		return ServerOptions{}
	}
	return s.options
}

// SetMetrics sets optional metrics collector for gateway observability
func (s *Server) SetMetrics(metrics gatewayMetrics) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()
}

// recordConnect records a connection with optional metrics
func (s *Server) recordConnect(userID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordConnect(userID)
}

// recordDisconnect records a disconnection with optional metrics
func (s *Server) recordDisconnect() {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordDisconnect()
}

// recordCommand records command processing with optional metrics
func (s *Server) recordCommand(userID string, duration time.Duration) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordCommand(userID, duration)
}

// recordError records command error with optional metrics
func (s *Server) recordError(userID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordError(userID)
}

// recordConnectionLimitExceeded records a rejected connection with optional metrics.
func (s *Server) recordConnectionLimitExceeded() {
	if s == nil {
		return
	}
	s.mu.Lock()
	m := s.metrics
	s.mu.Unlock()
	if m == nil {
		return
	}
	m.RecordConnectionLimitExceeded()
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
			s.recordConnectionLimitExceeded()
			rejectIMAPConnectionLimit(conn)
			continue
		}
		s.wg.Add(1)
		go func(conn net.Conn) {
			defer s.wg.Done()
			defer releaseIMAPConnectionSlot(slots)
			_ = s.ServeConn(conn)
		}(conn)
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
	err := listener.Close()
	s.wg.Wait()
	return err
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
	connCtx, connCancel := context.WithCancel(context.Background())
	defer connCancel()
	state := imapConnState{}
	state.ctx = connCtx
	_, state.tlsActive = conn.(*tls.Conn)
	state.remoteIP = imapRemoteAddrIP(conn.RemoteAddr())

	// Initial connection tracking (unauth'd)
	s.recordConnect("unauthenticated")
	defer s.recordDisconnect()

	if _, err := writer.WriteString("* OK " + s.capabilityCode(&state) + " gogomail IMAP4rev1 service ready\r\n"); err != nil {
		return err
	}
	if err := s.setWriteDeadline(conn); err != nil {
		return err
	}
	if err := writer.Flush(); err != nil {
		return err
	}
	defer state.closeSubscription()
	for {
		if err := s.setReadDeadline(conn, s.options.ReadTimeout); err != nil {
			return err
		}
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
				if err := s.setWriteDeadline(conn); err != nil {
					return err
				}
				if err := writer.Flush(); err != nil {
					return err
				}
				return nil
			}
			return err
		}
		cmdStart := time.Now()
		done, err := s.handleLineWithLiteral(writer, line, literals, &state)
		if err != nil {
			// Record error with current userID
			if state.userID != "" {
				s.recordError(state.userID)
			}
			return err
		}
		// Record command timing
		if state.userID != "" {
			s.recordCommand(state.userID, time.Since(cmdStart))
		}
		if err := s.setWriteDeadline(conn); err != nil {
			return err
		}
		if err := writer.Flush(); err != nil {
			return err
		}
		if state.pendingIdleTag != "" {
			if err := s.serveIdle(conn, reader, writer, &state); err != nil {
				var framingErr imapProtocolFramingError
				if errors.As(err, &framingErr) {
					if err := writeIMAPFramingError(writer, framingErr.line, framingErr.message); err != nil {
						return err
					}
					if err := s.setWriteDeadline(conn); err != nil {
						return err
					}
					if err := writer.Flush(); err != nil {
						return err
					}
					return nil
				}
				return err
			}
			if err := s.setWriteDeadline(conn); err != nil {
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
		}
		if state.startTLS {
			if err := s.setHandshakeDeadline(conn); err != nil {
				return err
			}
			tlsConn := tls.Server(conn, s.options.TLSConfig)
			if err := tlsConn.Handshake(); err != nil {
				return err
			}
			if err := tlsConn.SetDeadline(time.Time{}); err != nil {
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

var (
	errIMAPCommandLineTooLong     = errors.New("imap command line is too long")
	errIMAPCommandLiteralTooLarge = errors.New("imap command literal is too large")
	errIMAPCommandLiteralInvalid  = errors.New("imap command literal is invalid")
)

type imapConnState struct {
	ctx                   context.Context
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
	userID                string // For metrics tracking
	remoteIP              string
}

func (s *Server) handleLine(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	return s.handleLineWithLiteral(writer, line, nil, state)
}

const imapEmptySearchStringToken = "\x00EMPTY-SEARCH-STRING"

const maxIMAPSearchOpenSearchCandidates = 200

// maxIMAPSearchFastPathLimit caps the number of UIDs returned by the fast
// path. If OpenSearch returns this many results we cannot guarantee
// completeness and fall back to the slow path.

// maxIMAPSearchFastPathLimit caps the number of UIDs returned by the fast
// path. If OpenSearch returns this many results we cannot guarantee
// completeness and fall back to the slow path.
const maxIMAPSearchFastPathLimit = 10_000

// imapSearchUIDFastPath handles UID SEARCH when every criterion is fully
// satisfied by the search index (no MODSEQ, no SAVE). It queries the search
// index for matching message IDs, resolves them to IMAP UIDs via a targeted
// Postgres query, and writes the SEARCH/ESEARCH response, all without
// loading the full mailbox into memory.
//
// Returns (true, err) when the response was written (caller must return),
// (false, nil) to signal that the slow path should be used instead.

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

const maxIMAPMIMEPartPathDepth = 32

const (
	imapRawFieldAtom imapRawFieldKindValue = iota
	imapRawFieldQuoted
	imapRawFieldLiteral
	imapRawFieldList
)
