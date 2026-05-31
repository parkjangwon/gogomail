package pop3d

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"net"
	"net/textproto"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Mailbox provides access to a user's mailbox.
type Mailbox interface {
	MessageCount() int
	MessageSize(i int) int
	MessageUIDL(i int) string
	MessageContent(i int) string
	MarkDeleted(i int) error
	ResetDeleted()
	Deleted(i int) bool
}

type messageContentWithError interface {
	MessageContentWithError(i int) (string, error)
}

type maildropLockKey interface {
	MaildropLockKey() string
}

// Store authenticates users and returns their mailboxes.
type Store interface {
	Authenticate(user, pass string) (Mailbox, error)
}

// gatewayMetrics is the minimal interface pop3d uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordConnect(userID string)
	RecordDisconnect()
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
	RecordConnectionLimitExceeded()
}

// Server is a POP3 server.
type Server struct {
	Store          Store
	TLSConfig      *tls.Config
	Greeting       string
	IdleTimeout    time.Duration
	MaxConnections int
	mu             sync.Mutex
	listeners      []net.Listener
	maildrops      map[string]struct{}
	authTracker    *pop3AuthFailureTracker
	metrics        gatewayMetrics
	wg             sync.WaitGroup
	ctx            context.Context
	cancel         context.CancelFunc
}

// shutdownCtx returns the server's shutdown context, lazily initializing it on
// first use so zero-value Server instances (e.g. in tests) keep working.
func (s *Server) shutdownCtx() context.Context {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.ctx == nil {
		s.ctx, s.cancel = context.WithCancel(context.Background())
	}
	return s.ctx
}

// Serve accepts connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
	if s == nil {
		return errors.New("pop3 server is nil")
	}
	if ln == nil {
		return errors.New("pop3 listener is required")
	}
	srvCtx := s.shutdownCtx()
	s.mu.Lock()
	s.listeners = append(s.listeners, ln)
	s.mu.Unlock()

	var slots chan struct{}
	if s.MaxConnections > 0 {
		slots = make(chan struct{}, s.MaxConnections)
	}
	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "use of closed") {
				return nil
			}
			return err
		}
		if !acquireConnectionSlot(slots) {
			s.recordConnectionLimitExceeded()
			rejectConnectionLimit(conn)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			defer releaseConnectionSlot(slots)
			s.handleConn(srvCtx, conn)
		}()
	}
}

func acquireConnectionSlot(slots chan struct{}) bool {
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

func releaseConnectionSlot(slots chan struct{}) {
	if slots == nil {
		return
	}
	<-slots
}

func rejectConnectionLimit(conn net.Conn) {
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = conn.Write([]byte("-ERR too many connections\r\n"))
	_ = conn.Close()
}

// SetMetrics sets optional metrics collector for gateway observability.
func (s *Server) SetMetrics(metrics gatewayMetrics) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.metrics = metrics
	s.mu.Unlock()
}

func (s *Server) metricsCollector() gatewayMetrics {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.metrics
}

func (s *Server) recordConnect(userID string) {
	if m := s.metricsCollector(); m != nil {
		m.RecordConnect(userID)
	}
}

func (s *Server) recordDisconnect() {
	if m := s.metricsCollector(); m != nil {
		m.RecordDisconnect()
	}
}

func (s *Server) recordCommand(userID string, duration time.Duration) {
	if m := s.metricsCollector(); m != nil {
		m.RecordCommand(userID, duration)
	}
}

func (s *Server) recordError(userID string) {
	if m := s.metricsCollector(); m != nil {
		m.RecordError(userID)
	}
}

func (s *Server) recordConnectionLimitExceeded() {
	if m := s.metricsCollector(); m != nil {
		m.RecordConnectionLimitExceeded()
	}
}

// Close closes all active listeners.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	for _, ln := range s.listeners {
		ln.Close()
	}
	s.listeners = nil
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

// Shutdown closes listeners, signals active connections to drain, and waits for
// in-flight handleConn goroutines to finish. It returns ctx.Err() if the given
// context expires before all goroutines exit.
func (s *Server) Shutdown(ctx context.Context) error {
	if s == nil {
		return nil
	}
	_ = s.Close()
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

const (
	stateAuthorization = iota
	stateTransaction
)

type session struct {
	server    *Server
	conn      net.Conn
	state     int
	mailbox   Mailbox
	user      string
	reader    *bufio.Reader
	writer    *bufio.Writer
	textConn  *textproto.Conn
	tlsActive bool
	release   func()
}

func (s *Server) handleConn(ctx context.Context, conn net.Conn) {
	s.recordConnect("unauthenticated")
	defer s.recordDisconnect()

	idle := s.IdleTimeout
	if idle == 0 {
		idle = 30 * time.Minute
	}
	_ = conn.SetDeadline(time.Now().Add(idle))

	sess := &session{
		server:    s,
		conn:      conn,
		state:     stateAuthorization,
		reader:    bufio.NewReader(conn),
		writer:    bufio.NewWriter(conn),
		tlsActive: isTLSConn(conn),
	}
	defer func() {
		sess.releaseMaildropLock()
		_ = sess.conn.Close()
	}()
	sess.textConn = textproto.NewConn(conn)

	// On server shutdown, force the connection to unblock by tightening its
	// deadline. The defer below stops the watcher goroutine when handleConn
	// returns normally.
	watchDone := make(chan struct{})
	if ctx != nil {
		go func() {
			select {
			case <-ctx.Done():
				_ = sess.conn.SetDeadline(time.Now().Add(5 * time.Second))
			case <-watchDone:
			}
		}()
	}
	defer close(watchDone)

	greeting := s.Greeting
	if greeting == "" {
		greeting = "POP3 server ready"
	}
	sess.writeLine("+OK " + greeting)

	for {
		line, err := readPOP3Line(sess.reader)
		if err != nil {
			if errors.Is(err, errPOP3LineTooLong) {
				sess.writeERR("line too long")
			}
			return
		}
		if ctx != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
		_ = conn.SetDeadline(time.Now().Add(idle))
		cmdStart := time.Now()
		sess.handleCommand(line)
		s.recordCommand(sess.metricsUserID(), time.Since(cmdStart))
	}
}

const maxPOP3LineOctets = 512

var errPOP3LineTooLong = errors.New("pop3 line too long")

func readPOP3Line(reader *bufio.Reader) (string, error) {
	if reader == nil {
		return "", errors.New("pop3 reader is required")
	}
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxPOP3LineOctets {
			return "", errPOP3LineTooLong
		}
		line = append(line, fragment...)
		if err == nil {
			return strings.TrimRight(string(line), "\r\n"), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return "", err
	}
}

func isTLSConn(conn net.Conn) bool {
	_, ok := conn.(*tls.Conn)
	return ok
}

func (sess *session) writeLine(line string) {
	_, _ = sess.writer.WriteString(line)
	_, _ = sess.writer.WriteString("\r\n")
	_ = sess.writer.Flush()
}

func (sess *session) writeOK(msg string) {
	sess.writeStatusLine("+OK", msg)
}

func (sess *session) writeERR(msg string) {
	if sess != nil && sess.server != nil {
		sess.server.recordError(sess.metricsUserID())
	}
	sess.writeStatusLine("-ERR", msg)
}

func (sess *session) writeStatusLine(prefix, msg string) {
	_, _ = sess.writer.WriteString(prefix)
	if msg != "" {
		_ = sess.writer.WriteByte(' ')
		_, _ = sess.writer.WriteString(msg)
	}
	_, _ = sess.writer.WriteString("\r\n")
	_ = sess.writer.Flush()
}

func (sess *session) handleCommand(line string) {
	cmd, arg1, arg2, argc := parsePOP3Command(line)
	if cmd == "" {
		sess.writeERR("syntax error")
		return
	}

	switch sess.state {
	case stateAuthorization:
		sess.handleAuth(cmd, arg1, arg2, argc)
	case stateTransaction:
		sess.handleTransaction(cmd, arg1, arg2, argc)
	}
}

func (sess *session) handleAuth(cmd, arg1, arg2 string, argc int) {
	switch cmd {
	case "USER":
		if argc != 1 {
			sess.writeERR("syntax error")
			return
		}
		sess.user = arg1
		sess.writeOK("")
	case "PASS":
		if argc != 1 {
			sess.writeERR("syntax error")
			return
		}
		user := sess.extractUser()
		pass := arg1
		if user == "" {
			sess.writeERR("authentication failed")
			return
		}
		sess.authenticate(user, pass)
	case "QUIT":
		sess.writeOK("bye")
		sess.conn.Close()
	case "CAPA":
		sess.writeCapabilities()
	case "STLS":
		if !sess.canUseSTLS() {
			sess.writeERR("STLS not available")
			return
		}
		sess.writeOK("Begin TLS negotiation")
		tlsConn := tls.Server(sess.conn, sess.server.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			sess.writeERR("TLS handshake failed")
			sess.conn.Close()
			return
		}
		sess.conn = tlsConn
		sess.reader = bufio.NewReader(tlsConn)
		sess.writer = bufio.NewWriter(tlsConn)
		sess.textConn = textproto.NewConn(tlsConn)
		sess.tlsActive = true
		sess.user = ""
	case "NOOP":
		sess.writeOK("")
	case "AUTH":
		if argc == 0 {
			sess.writeERR("syntax error")
			return
		}
		mechanism := pop3UpperASCII(arg1)
		switch mechanism {
		case "PLAIN":
			if argc > 2 {
				sess.writeERR("syntax error")
				return
			}
			var encoded string
			if argc >= 2 {
				encoded = arg2
			} else {
				sess.writeLine("+ ")
				line, err := readPOP3Line(sess.reader)
				if err != nil {
					if errors.Is(err, errPOP3LineTooLong) {
						sess.writeERR("line too long")
					}
					sess.conn.Close()
					return
				}
				encoded = line
				if encoded == "*" {
					sess.writeERR("authentication cancelled")
					return
				}
			}
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				sess.writeERR("invalid base64")
				return
			}
			user, pass, ok := pop3AuthPlainCredentials(decoded)
			if !ok {
				sess.writeERR("invalid credentials format")
				return
			}
			sess.authenticate(user, pass)
		case "LOGIN":
			if argc != 1 {
				sess.writeERR("syntax error")
				return
			}
			sess.writeLine("+ " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			userLine, err := readPOP3Line(sess.reader)
			if err != nil {
				if errors.Is(err, errPOP3LineTooLong) {
					sess.writeERR("line too long")
				}
				sess.conn.Close()
				return
			}
			userEncoded := userLine
			if userEncoded == "*" {
				sess.writeERR("authentication cancelled")
				return
			}
			userDecoded, err := base64.StdEncoding.DecodeString(userEncoded)
			if err != nil {
				sess.writeERR("invalid base64")
				return
			}
			sess.writeLine("+ " + base64.StdEncoding.EncodeToString([]byte("Password:")))
			passLine, err := readPOP3Line(sess.reader)
			if err != nil {
				if errors.Is(err, errPOP3LineTooLong) {
					sess.writeERR("line too long")
				}
				sess.conn.Close()
				return
			}
			passEncoded := passLine
			if passEncoded == "*" {
				sess.writeERR("authentication cancelled")
				return
			}
			passDecoded, err := base64.StdEncoding.DecodeString(passEncoded)
			if err != nil {
				sess.writeERR("invalid base64")
				return
			}
			sess.authenticate(string(userDecoded), string(passDecoded))
		default:
			sess.writeERR("unsupported authentication mechanism")
		}
	default:
		sess.writeERR("unknown command")
	}
}

func (sess *session) authenticate(user, pass string) {
	if sess.server == nil || sess.server.Store == nil {
		sess.writeERR("authentication failed")
		return
	}
	ip := pop3RemoteAddrIP(sess.conn.RemoteAddr())
	tracker := sess.server.getAuthTracker()
	if tracker.isLocked(ip) {
		sess.writeERR("authentication failed")
		return
	}
	mb, err := sess.server.Store.Authenticate(user, pass)
	if err != nil {
		tracker.record(ip)
		sess.writeERR("authentication failed")
		return
	}
	release, ok := sess.server.acquireMaildropLock(mailboxLockKey(mb, user))
	if !ok {
		sess.writeERR("maildrop already locked")
		return
	}
	sess.mailbox = mb
	sess.release = release
	sess.user = user
	sess.state = stateTransaction
	sess.writeOK("mailbox ready")
}

func (sess *session) extractUser() string {
	return sess.user
}

func (sess *session) metricsUserID() string {
	if sess == nil {
		return "unknown"
	}
	user := strings.TrimSpace(sess.user)
	if user == "" {
		return "unauthenticated"
	}
	return user
}

func (sess *session) releaseMaildropLock() {
	if sess.release == nil {
		return
	}
	release := sess.release
	sess.release = nil
	release()
}

func (sess *session) handleTransaction(cmd, arg1, arg2 string, argc int) {
	switch cmd {
	case "STAT":
		count, size := sess.msgCountAndSize()
		sess.writeOKIntPair(count, size)
	case "LIST":
		if argc == 0 {
			sess.writeOK("")
			for i := 0; i < sess.mailbox.MessageCount(); i++ {
				if !sess.mailbox.Deleted(i) {
					sess.writeNumberPairLine(i+1, sess.mailbox.MessageSize(i))
				}
			}
			sess.writeLine(".")
		} else {
			idx, err := strconv.Atoi(arg1)
			if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
				sess.writeERR("no such message")
				return
			}
			sess.writeOKIntPair(idx, sess.mailbox.MessageSize(idx-1))
		}
	case "RETR":
		if argc != 1 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(arg1)
		if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
			sess.writeERR("no such message")
			return
		}
		content, err := sess.messageContent(idx - 1)
		if err != nil {
			sess.writeERR("message content unavailable")
			return
		}
		sess.writeOKIntSuffix(sess.mailbox.MessageSize(idx-1), " octets")
		sess.writeDotStuffedMultiline(content)
		sess.writeLine(".")
	case "DELE":
		if argc != 1 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(arg1)
		if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
			sess.writeERR("no such message")
			return
		}
		if err := sess.mailbox.MarkDeleted(idx - 1); err != nil {
			sess.writeERR("delete failed")
			return
		}
		sess.writeOK("message deleted")
	case "NOOP":
		sess.writeOK("")
	case "RSET":
		sess.mailbox.ResetDeleted()
		sess.writeOK("")
	case "UIDL":
		if argc == 0 {
			sess.writeOK("")
			for i := 0; i < sess.mailbox.MessageCount(); i++ {
				if !sess.mailbox.Deleted(i) {
					sess.writeNumberStringLine(i+1, sess.mailbox.MessageUIDL(i))
				}
			}
			sess.writeLine(".")
		} else {
			idx, err := strconv.Atoi(arg1)
			if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
				sess.writeERR("no such message")
				return
			}
			sess.writeOKIntString(idx, sess.mailbox.MessageUIDL(idx-1))
		}
	case "TOP":
		if argc != 2 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(arg1)
		if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
			sess.writeERR("no such message")
			return
		}
		lines, err := strconv.Atoi(arg2)
		if err != nil || lines < 0 {
			sess.writeERR("syntax error")
			return
		}
		content, err := sess.messageContent(idx - 1)
		if err != nil {
			sess.writeERR("message content unavailable")
			return
		}
		sess.writeOK("")
		sess.writeTopDotStuffedMultiline(content, lines)
		sess.writeLine(".")
	case "QUIT":
		if committer, ok := sess.mailbox.(interface{ CommitDeletes() error }); ok && sess.hasDeletedMessages() {
			if err := committer.CommitDeletes(); err != nil {
				sess.mailbox.ResetDeleted()
				sess.writeERR("commit failed")
				return
			}
		}
		sess.writeOK("bye")
		sess.releaseMaildropLock()
		sess.conn.Close()
	case "CAPA":
		sess.writeCapabilities()
	case "STLS":
		sess.writeERR("STLS not available in transaction state")
	default:
		sess.writeERR("unknown command")
	}
}

func (sess *session) hasDeletedMessages() bool {
	for i := 0; i < sess.mailbox.MessageCount(); i++ {
		if sess.mailbox.Deleted(i) {
			return true
		}
	}
	return false
}

func mailboxLockKey(mailbox Mailbox, fallback string) string {
	if keyed, ok := mailbox.(maildropLockKey); ok {
		if key := strings.TrimSpace(keyed.MaildropLockKey()); key != "" {
			return key
		}
	}
	return fallback
}

func normalizeMaildropLockKey(key string) string {
	return strings.ToLower(strings.TrimSpace(key))
}

func (s *Server) acquireMaildropLock(key string) (func(), bool) {
	key = normalizeMaildropLockKey(key)
	if key == "" {
		return func() {}, true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.maildrops == nil {
		s.maildrops = make(map[string]struct{})
	}
	if _, exists := s.maildrops[key]; exists {
		return nil, false
	}
	s.maildrops[key] = struct{}{}
	return func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		delete(s.maildrops, key)
	}, true
}

func (sess *session) writeCapabilities() {
	sess.writeOK("Capability list follows")
	sess.writer.WriteString("IMPLEMENTATION gogomail\r\n")
	sess.writer.WriteString("LOGIN-DELAY 0\r\n")
	sess.writer.WriteString("UIDL\r\n")
	sess.writer.WriteString("TOP\r\n")
	if sess.state == stateAuthorization {
		sess.writer.WriteString("USER\r\n")
		sess.writer.WriteString("SASL PLAIN LOGIN\r\n")
		if sess.canUseSTLS() {
			sess.writer.WriteString("STLS\r\n")
		}
	}
	sess.writer.WriteString(".\r\n")
	sess.writer.Flush()
}

func (sess *session) messageContent(i int) (string, error) {
	if mailbox, ok := sess.mailbox.(messageContentWithError); ok {
		return mailbox.MessageContentWithError(i)
	}
	return sess.mailbox.MessageContent(i), nil
}

func (sess *session) canUseSTLS() bool {
	return sess.state == stateAuthorization && sess.server.TLSConfig != nil && !sess.tlsActive
}

func (sess *session) msgCountAndSize() (int, int) {
	count := 0
	size := 0
	for i := 0; i < sess.mailbox.MessageCount(); i++ {
		if !sess.mailbox.Deleted(i) {
			count++
			size += sess.mailbox.MessageSize(i)
		}
	}
	return count, size
}

func (sess *session) writeDotStuffedMultiline(content string) {
	sess.writePOP3Multiline(content, false, 0)
}

func (sess *session) writeTopDotStuffedMultiline(content string, n int) {
	sess.writePOP3Multiline(content, true, n)
}

func (sess *session) writePOP3Multiline(content string, topMode bool, topLimit int) {
	sawSeparator := false
	bodyLines := 0
	for len(content) > 0 {
		line, rest := pop3NextLine(content)
		content = rest
		if topMode && !sawSeparator {
			if line == "" {
				sawSeparator = true
				sess.writeNormalizedPOP3Line(line)
			} else {
				sess.writeNormalizedPOP3Line(line)
			}
			continue
		}
		if topMode && topLimit >= 0 && sawSeparator {
			if bodyLines >= topLimit {
				continue
			}
			bodyLines++
		}
		sess.writeNormalizedPOP3Line(line)
	}
	if topMode && !sawSeparator {
		sess.writeNormalizedPOP3Line("")
	}
}

func (sess *session) writeNormalizedPOP3Line(line string) {
	if strings.HasPrefix(line, ".") {
		_ = sess.writer.WriteByte('.')
	}
	_, _ = sess.writer.WriteString(line)
	_, _ = sess.writer.WriteString("\r\n")
}

func (sess *session) writeNumberPairLine(index, size int) {
	var buf [64]byte
	out := buf[:0]
	out = strconv.AppendInt(out, int64(index), 10)
	out = append(out, ' ')
	out = strconv.AppendInt(out, int64(size), 10)
	out = append(out, '\r', '\n')
	_, _ = sess.writer.Write(out)
}

func (sess *session) writeNumberStringLine(index int, uidl string) {
	var buf [64]byte
	out := buf[:0]
	out = strconv.AppendInt(out, int64(index), 10)
	out = append(out, ' ')
	out = append(out, uidl...)
	out = append(out, '\r', '\n')
	_, _ = sess.writer.Write(out)
}

func (sess *session) writeOKIntPair(left, right int) {
	var buf [64]byte
	out := append(buf[:0], "+OK "...)
	out = strconv.AppendInt(out, int64(left), 10)
	out = append(out, ' ')
	out = strconv.AppendInt(out, int64(right), 10)
	out = append(out, '\r', '\n')
	_, _ = sess.writer.Write(out)
	_ = sess.writer.Flush()
}

func (sess *session) writeOKIntString(left int, right string) {
	var buf [64]byte
	out := append(buf[:0], "+OK "...)
	out = strconv.AppendInt(out, int64(left), 10)
	out = append(out, ' ')
	out = append(out, right...)
	out = append(out, '\r', '\n')
	_, _ = sess.writer.Write(out)
	_ = sess.writer.Flush()
}

func (sess *session) writeOKIntSuffix(num int, suffix string) {
	var buf [64]byte
	out := append(buf[:0], "+OK "...)
	out = strconv.AppendInt(out, int64(num), 10)
	out = append(out, suffix...)
	out = append(out, '\r', '\n')
	_, _ = sess.writer.Write(out)
	_ = sess.writer.Flush()
}

func pop3NextLine(content string) (line, rest string) {
	for i := 0; i < len(content); i++ {
		switch content[i] {
		case '\r':
			if i+1 < len(content) && content[i+1] == '\n' {
				return content[:i], content[i+2:]
			}
			return content[:i], content[i+1:]
		case '\n':
			return content[:i], content[i+1:]
		}
	}
	return content, ""
}

func pop3AuthPlainCredentials(decoded []byte) (user, pass string, ok bool) {
	first := bytes.IndexByte(decoded, 0)
	if first < 0 {
		return "", "", false
	}
	rest := decoded[first+1:]
	second := bytes.IndexByte(rest, 0)
	if second < 0 {
		return "", "", false
	}
	return string(rest[:second]), string(rest[second+1:]), true
}

func parsePOP3Command(line string) (cmd, arg1, arg2 string, argc int) {
	start := 0
	for start < len(line) && isPOP3Space(line[start]) {
		start++
	}
	end := len(line)
	for end > start && isPOP3Space(line[end-1]) {
		end--
	}
	if start >= end {
		return "", "", "", 0
	}

	i := start
	for i < end && !isPOP3Space(line[i]) {
		i++
	}
	cmd = pop3UpperASCII(line[start:i])
	for i < end {
		for i < end && isPOP3Space(line[i]) {
			i++
		}
		if i >= end {
			break
		}
		j := i
		for j < end && !isPOP3Space(line[j]) {
			j++
		}
		switch argc {
		case 0:
			arg1 = line[i:j]
		case 1:
			arg2 = line[i:j]
		}
		argc++
		i = j
	}
	return cmd, arg1, arg2, argc
}

func isPOP3Space(b byte) bool {
	switch b {
	case ' ', '\t', '\v', '\f':
		return true
	default:
		return false
	}
}

func pop3UpperASCII(s string) string {
	for i := 0; i < len(s); i++ {
		if 'a' <= s[i] && s[i] <= 'z' {
			return strings.ToUpper(s)
		}
	}
	return s
}

func (s *Server) getAuthTracker() *pop3AuthFailureTracker {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.authTracker == nil {
		s.authTracker = newPOP3AuthFailureTracker()
	}
	return s.authTracker
}

type pop3AuthFailureTracker struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	window   time.Duration
	maxFails int
}

func newPOP3AuthFailureTracker() *pop3AuthFailureTracker {
	return &pop3AuthFailureTracker{
		failures: make(map[string][]time.Time),
		window:   10 * time.Minute,
		maxFails: 10,
	}
}

func (t *pop3AuthFailureTracker) record(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-t.window)
	prev := t.failures[ip]
	fresh := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	fresh = append(fresh, now)
	if len(prev) > 0 && len(fresh) == 1 {
		t.failures[ip] = []time.Time{now}
	} else {
		t.failures[ip] = fresh
	}
	return len(fresh) > t.maxFails
}

func (t *pop3AuthFailureTracker) isLocked(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-t.window)
	var count int
	for _, ts := range t.failures[ip] {
		if ts.After(cutoff) {
			count++
		}
	}
	if count == 0 {
		delete(t.failures, ip)
	}
	return count >= t.maxFails
}

func pop3RemoteAddrIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
