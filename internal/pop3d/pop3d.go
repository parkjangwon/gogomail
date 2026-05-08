package pop3d

import (
	"bufio"
	"crypto/tls"
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

// Store authenticates users and returns their mailboxes.
type Store interface {
	Authenticate(user, pass string) (Mailbox, error)
}

// Server is a POP3 server.
type Server struct {
	Store       Store
	TLSConfig   *tls.Config
	Greeting    string
	IdleTimeout time.Duration
	mu          sync.Mutex
	listeners   []net.Listener
}

// Serve accepts connections on the listener.
func (s *Server) Serve(ln net.Listener) error {
	s.mu.Lock()
	s.listeners = append(s.listeners, ln)
	s.mu.Unlock()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if strings.Contains(err.Error(), "closed") || strings.Contains(err.Error(), "use of closed") {
				return nil
			}
			return err
		}
		go s.handleConn(conn)
	}
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
	server   *Server
	conn     net.Conn
	state    int
	mailbox  Mailbox
	user     string
	reader   *bufio.Reader
	writer   *bufio.Writer
	textConn *textproto.Conn
}

func (s *Server) handleConn(conn net.Conn) {
	defer conn.Close()

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
		mb, err := sess.server.Store.Authenticate(user, pass)
		if err != nil {
			sess.writeERR("authentication failed")
			return
		}
		sess.mailbox = mb
		sess.state = stateTransaction
		sess.writeOK("mailbox ready")
	case "QUIT":
		sess.writeOK("bye")
		sess.conn.Close()
	case "CAPA":
		sess.writeOK("Capability list follows")
		sess.writer.WriteString("UIDL\r\n")
		sess.writer.WriteString("TOP\r\n")
		sess.writer.WriteString("USER\r\n")
		if sess.server.TLSConfig != nil {
			sess.writer.WriteString("STLS\r\n")
		}
		sess.writer.WriteString(".\r\n")
		sess.writer.Flush()
	case "STLS":
		if sess.server.TLSConfig == nil {
			sess.writeERR("STLS not available")
			return
		}
		sess.writeOK("Begin TLS negotiation")
		tlsConn := tls.Server(sess.conn, sess.server.TLSConfig)
		if err := tlsConn.Handshake(); err != nil {
			sess.conn.Close()
			return
		}
		sess.conn = tlsConn
		sess.reader = bufio.NewReader(tlsConn)
		sess.writer = bufio.NewWriter(tlsConn)
		sess.textConn = textproto.NewConn(tlsConn)
	case "NOOP":
		sess.writeOK("")
	case "AUTH":
		sess.writeERR("AUTH not implemented")
	default:
		sess.writeERR("unknown command")
	}
}

func (sess *session) extractUser(raw string) string {
	return sess.user
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
		content := sess.mailbox.MessageContent(idx - 1)
		sess.writeOK(fmt.Sprintf("%d octets", len(content)))
		sess.writer.WriteString(content)
		if !strings.HasSuffix(content, "\n") {
			sess.writer.WriteString("\r\n")
		}
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
		content := sess.mailbox.MessageContent(idx - 1)
		sess.writeOK("")
		sess.sendTopContent(content, lines)
		sess.writer.WriteString(".\r\n")
		sess.writer.Flush()
	case "QUIT":
		sess.writeOK("bye")
		sess.conn.Close()
	case "CAPA":
		sess.writeOK("Capability list follows")
		sess.writer.WriteString("UIDL\r\n")
		sess.writer.WriteString("TOP\r\n")
		sess.writer.WriteString("USER\r\n")
		if sess.server.TLSConfig != nil {
			sess.writer.WriteString("STLS\r\n")
		}
		sess.writer.WriteString(".\r\n")
		sess.writer.Flush()
	default:
		sess.writeERR("unknown command")
	}
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

func (sess *session) sendTopContent(content string, n int) {
	parts := strings.SplitN(content, "\r\n\r\n", 2)
	if len(parts) == 1 {
		parts = strings.SplitN(content, "\n\n", 2)
	}
	if len(parts) >= 1 {
		sess.writer.WriteString(parts[0])
		sess.writer.WriteString("\r\n\r\n")
	}
	if len(parts) == 2 && n > 0 {
		bodyLines := strings.Split(parts[1], "\n")
		for i, line := range bodyLines {
			if i >= n {
				break
			}
			sess.writer.WriteString(line)
			sess.writer.WriteString("\r\n")
		}
	}
}
