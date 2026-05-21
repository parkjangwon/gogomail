package pop3d

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"encoding/base64"
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
}

// Serve accepts connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
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
			rejectConnectionLimit(conn)
			continue
		}
		go func() {
			defer releaseConnectionSlot(slots)
			s.handleConn(conn)
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

// Close closes all active listeners.
func (s *Server) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ln := range s.listeners {
		ln.Close()
	}
	s.listeners = nil
	return nil
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

func (s *Server) handleConn(conn net.Conn) {
	idle := s.IdleTimeout
	if idle == 0 {
		idle = 30 * time.Minute
	}
	conn.SetDeadline(time.Now().Add(idle))

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

	greeting := s.Greeting
	if greeting == "" {
		greeting = "POP3 server ready"
	}
	sess.writeLine("+OK " + greeting)

	for {
		line, err := sess.reader.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimRight(line, "\r\n")
		conn.SetDeadline(time.Now().Add(idle))
		sess.handleCommand(line)
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
				line, err := sess.reader.ReadString('\n')
				if err != nil {
					sess.conn.Close()
					return
				}
				encoded = strings.TrimRight(line, "\r\n")
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
			userLine, err := sess.reader.ReadString('\n')
			if err != nil {
				sess.conn.Close()
				return
			}
			userEncoded := strings.TrimRight(userLine, "\r\n")
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
			passLine, err := sess.reader.ReadString('\n')
			if err != nil {
				sess.conn.Close()
				return
			}
			passEncoded := strings.TrimRight(passLine, "\r\n")
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
	mb, err := sess.server.Store.Authenticate(user, pass)
	if err != nil {
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
	sess.state = stateTransaction
	sess.writeOK("mailbox ready")
}

func (sess *session) extractUser() string {
	return sess.user
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
