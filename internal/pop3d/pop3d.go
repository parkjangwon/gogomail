package pop3d

import (
	"bufio"
	"crypto/tls"
	"encoding/base64"
	"fmt"
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
		server: s,
		conn:   conn,
		state:  stateAuthorization,
		reader: bufio.NewReader(conn),
		writer: bufio.NewWriter(conn),
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

func (sess *session) writeLine(line string) {
	sess.writer.WriteString(line + "\r\n")
	sess.writer.Flush()
}

func (sess *session) writeOK(msg string) {
	if msg != "" {
		sess.writeLine("+OK " + msg)
	} else {
		sess.writeLine("+OK")
	}
}

func (sess *session) writeERR(msg string) {
	if msg != "" {
		sess.writeLine("-ERR " + msg)
	} else {
		sess.writeLine("-ERR")
	}
}

func (sess *session) handleCommand(line string) {
	parts := strings.Fields(line)
	if len(parts) == 0 {
		sess.writeERR("syntax error")
		return
	}
	cmd := strings.ToUpper(parts[0])
	args := parts[1:]

	switch sess.state {
	case stateAuthorization:
		sess.handleAuth(cmd, args, line)
	case stateTransaction:
		sess.handleTransaction(cmd, args)
	}
}

func (sess *session) handleAuth(cmd string, args []string, raw string) {
	switch cmd {
	case "USER":
		if len(args) != 1 {
			sess.writeERR("syntax error")
			return
		}
		sess.user = args[0]
		sess.writeOK("")
	case "PASS":
		if len(args) != 1 {
			sess.writeERR("syntax error")
			return
		}
		user := sess.extractUser(raw)
		pass := args[0]
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
			sess.writeERR("TLS handshake failed: " + err.Error())
			sess.conn.Close()
			return
		}
		sess.conn = tlsConn
		sess.reader = bufio.NewReader(tlsConn)
		sess.writer = bufio.NewWriter(tlsConn)
		sess.textConn = textproto.NewConn(tlsConn)
		sess.tlsActive = true
	case "NOOP":
		sess.writeOK("")
	case "AUTH":
		if len(args) == 0 {
			sess.writeERR("syntax error")
			return
		}
		mechanism := strings.ToUpper(args[0])
		switch mechanism {
		case "PLAIN":
			var encoded string
			if len(args) >= 2 {
				encoded = args[1]
			} else {
				sess.writeLine("+ ")
				line, err := sess.reader.ReadString('\n')
				if err != nil {
					sess.conn.Close()
					return
				}
				encoded = strings.TrimRight(line, "\r\n")
			}
			decoded, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				sess.writeERR("invalid base64")
				return
			}
			parts := strings.SplitN(string(decoded), "\x00", 3)
			if len(parts) != 3 {
				sess.writeERR("invalid credentials format")
				return
			}
			user, pass := parts[1], parts[2]
			sess.authenticate(user, pass)
		case "LOGIN":
			sess.writeLine("+ " + base64.StdEncoding.EncodeToString([]byte("Username:")))
			userLine, err := sess.reader.ReadString('\n')
			if err != nil {
				sess.conn.Close()
				return
			}
			userDecoded, err := base64.StdEncoding.DecodeString(strings.TrimRight(userLine, "\r\n"))
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
			passDecoded, err := base64.StdEncoding.DecodeString(strings.TrimRight(passLine, "\r\n"))
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

func (sess *session) extractUser(raw string) string {
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

func (sess *session) handleTransaction(cmd string, args []string) {
	switch cmd {
	case "STAT":
		count, size := sess.msgCountAndSize()
		sess.writeOK(fmt.Sprintf("%d %d", count, size))
	case "LIST":
		if len(args) == 0 {
			sess.writeOK("")
			for i := 0; i < sess.mailbox.MessageCount(); i++ {
				if !sess.mailbox.Deleted(i) {
					sess.writer.WriteString(fmt.Sprintf("%d %d\r\n", i+1, sess.mailbox.MessageSize(i)))
				}
			}
			sess.writer.WriteString(".\r\n")
			sess.writer.Flush()
		} else {
			idx, err := strconv.Atoi(args[0])
			if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
				sess.writeERR("no such message")
				return
			}
			sess.writeOK(fmt.Sprintf("%d %d", idx, sess.mailbox.MessageSize(idx-1)))
		}
	case "RETR":
		if len(args) != 1 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(args[0])
		if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
			sess.writeERR("no such message")
			return
		}
		content, err := sess.messageContent(idx - 1)
		if err != nil {
			sess.writeERR("message content unavailable")
			return
		}
		sess.writeOK(fmt.Sprintf("%d octets", len(content)))
		sess.writeDotStuffedMultiline(content)
		sess.writer.WriteString(".\r\n")
		sess.writer.Flush()
	case "DELE":
		if len(args) != 1 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(args[0])
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
		if len(args) == 0 {
			sess.writeOK("")
			for i := 0; i < sess.mailbox.MessageCount(); i++ {
				if !sess.mailbox.Deleted(i) {
					sess.writer.WriteString(fmt.Sprintf("%d %s\r\n", i+1, sess.mailbox.MessageUIDL(i)))
				}
			}
			sess.writer.WriteString(".\r\n")
			sess.writer.Flush()
		} else {
			idx, err := strconv.Atoi(args[0])
			if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
				sess.writeERR("no such message")
				return
			}
			sess.writeOK(fmt.Sprintf("%d %s", idx, sess.mailbox.MessageUIDL(idx-1)))
		}
	case "TOP":
		if len(args) != 2 {
			sess.writeERR("syntax error")
			return
		}
		idx, err := strconv.Atoi(args[0])
		if err != nil || idx < 1 || idx > sess.mailbox.MessageCount() || sess.mailbox.Deleted(idx-1) {
			sess.writeERR("no such message")
			return
		}
		lines, _ := strconv.Atoi(args[1])
		content, err := sess.messageContent(idx - 1)
		if err != nil {
			sess.writeERR("message content unavailable")
			return
		}
		sess.writeOK("")
		sess.writeDotStuffedMultiline(topContent(content, lines))
		sess.writer.WriteString(".\r\n")
		sess.writer.Flush()
	case "QUIT":
		if committer, ok := sess.mailbox.(interface{ CommitDeletes() error }); ok {
			if err := committer.CommitDeletes(); err != nil {
				sess.mailbox.ResetDeleted()
				sess.writeERR("commit failed: " + err.Error())
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
	for _, line := range pop3MultilineLines(content) {
		if strings.HasPrefix(line, ".") {
			sess.writer.WriteByte('.')
		}
		sess.writer.WriteString(line)
		sess.writer.WriteString("\r\n")
	}
}

func topContent(content string, n int) string {
	content = normalizePOP3LineEndings(content)
	parts := strings.SplitN(content, "\n\n", 2)
	var b strings.Builder
	if len(parts) >= 1 {
		b.WriteString(parts[0])
		b.WriteString("\n\n")
	}
	if len(parts) == 2 && n > 0 {
		for i, line := range pop3MultilineLines(parts[1]) {
			if i >= n {
				break
			}
			b.WriteString(line)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func pop3MultilineLines(content string) []string {
	content = normalizePOP3LineEndings(content)
	if content == "" {
		return nil
	}
	lines := make([]string, 0, strings.Count(content, "\n")+1)
	for len(content) > 0 {
		idx := strings.IndexByte(content, '\n')
		if idx < 0 {
			lines = append(lines, content)
			break
		}
		lines = append(lines, content[:idx])
		content = content[idx+1:]
	}
	return lines
}

func normalizePOP3LineEndings(content string) string {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "\n")
	return content
}
