package milter

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

const (
	cmdOptneg     = 'O'
	cmdConnect    = 'C'
	cmdHelo       = 'H'
	cmdMail       = 'M'
	cmdRcpt       = 'R'
	cmdData       = 'T'
	cmdEOB        = 'E'
	cmdAbort      = 'A'
	cmdMacro      = 'D'
	cmdBody       = 'B'
	cmdEOH        = 'N'
	cmdHeader     = 'L'
	cmdQuit       = 'Q'
	cmdQuitNewCon = 'K'
	cmdSkip       = 'S'

	respContinue = 'c'
	respReject   = 'r'
	respTempfail = 't'
	respAccept   = 'a'
	respDiscard  = 'd'
	respSkip     = 's'

	// maxPacketSize is the maximum allowed milter packet payload size (65535 bytes).
	// This prevents unbounded memory allocation on receive and enforces the common
	// milter server limit on send.
	maxPacketSize = 65535
)

// Family values for the CONNECT command.
const (
	FamilyUnknown byte = 'U'
	FamilyLocal   byte = 'L'
	FamilyIPv4    byte = '4'
	FamilyIPv6    byte = '6'
)

// milterVersion is the protocol version advertised during OPTNEG.
const milterVersion = 6

// milterActions is the bitmask of message-modification actions the MTA supports.
const milterActions = 0x7F

type Action int

const (
	ActionContinue Action = iota
	ActionReject
	ActionTempfail
	ActionAccept
	ActionDiscard
	ActionUnknown
)

func ActionFromCode(code byte) Action {
	switch code {
	case respContinue:
		return ActionContinue
	case respReject:
		return ActionReject
	case respTempfail:
		return ActionTempfail
	case respAccept:
		return ActionAccept
	case respDiscard:
		return ActionDiscard
	default:
		return ActionUnknown
	}
}

// Packet is used for buffer-based encode/decode (testing and inspection).
type Packet struct {
	Command byte
	Data    []byte
}

func (p *Packet) Encode() ([]byte, error) {
	size := len(p.Data) + 1
	if size > 0x7fffffff {
		return nil, fmt.Errorf("packet too large")
	}

	data := make([]byte, 5+len(p.Data))
	binary.BigEndian.PutUint32(data[0:4], uint32(size))
	data[4] = p.Command
	copy(data[5:], p.Data)
	return data, nil
}

func DecodePacket(data []byte) (*Packet, error) {
	if len(data) < 5 {
		return nil, fmt.Errorf("packet too short")
	}

	size := binary.BigEndian.Uint32(data[0:4])
	if int(size) != len(data)-4 {
		return nil, fmt.Errorf("packet size mismatch")
	}

	return &Packet{
		Command: data[4],
		Data:    data[5:],
	}, nil
}

// sendPacket writes a single milter packet to w.
// Returns an error if the payload exceeds maxPacketSize.
func sendPacket(w io.Writer, cmd byte, data []byte) error {
	if len(data) > maxPacketSize {
		return fmt.Errorf("milter: packet payload %d bytes exceeds maximum %d", len(data), maxPacketSize)
	}
	size := uint32(1 + len(data))
	var hdr [4]byte
	binary.BigEndian.PutUint32(hdr[:], size)
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write([]byte{cmd}); err != nil {
		return err
	}
	if len(data) > 0 {
		_, err := w.Write(data)
		return err
	}
	return nil
}

// recvPacket reads a single milter packet from r.
// Returns an error if the declared payload size exceeds maxPacketSize.
func recvPacket(r io.Reader) (cmd byte, data []byte, err error) {
	var hdr [4]byte
	if _, err := io.ReadFull(r, hdr[:]); err != nil {
		return 0, nil, fmt.Errorf("read packet header: %w", err)
	}
	size := binary.BigEndian.Uint32(hdr[:])
	if size == 0 {
		return 0, nil, fmt.Errorf("empty packet")
	}
	// size includes the command byte, so payload is size-1.
	if size-1 > maxPacketSize {
		return 0, nil, fmt.Errorf("milter: incoming packet payload %d bytes exceeds maximum %d", size-1, maxPacketSize)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return 0, nil, fmt.Errorf("read packet body: %w", err)
	}
	return buf[0], buf[1:], nil
}

// Client is a milter protocol client. It sends mail events to a milter server
// and receives action verdicts.
type Client struct {
	conn    net.Conn
	timeout time.Duration
}

// NewClient wraps conn as a milter client. Use net.Pipe() in tests.
func NewClient(conn net.Conn, timeout time.Duration) *Client {
	return &Client{conn: conn, timeout: timeout}
}

// Dial connects to a milter server at address and returns a Client.
func Dial(ctx context.Context, network, address string, timeout time.Duration) (*Client, error) {
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, network, address)
	if err != nil {
		return nil, fmt.Errorf("milter dial: %w", err)
	}
	return NewClient(conn, timeout), nil
}

func (c *Client) deadline() time.Time {
	if c.timeout <= 0 {
		return time.Time{}
	}
	return time.Now().Add(c.timeout)
}

// effectiveDeadline returns the earlier of the timeout-derived deadline and
// any deadline carried by ctx. A zero time means no deadline.
func (c *Client) effectiveDeadline(ctx context.Context) time.Time {
	td := c.deadline()
	if d, ok := ctx.Deadline(); ok {
		if td.IsZero() || d.Before(td) {
			return d
		}
	}
	return td
}

// send checks ctx before writing, then sends a packet.
func (c *Client) send(ctx context.Context, cmd byte, data []byte) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("milter: context done before send: %w", ctx.Err())
	default:
	}
	_ = c.conn.SetDeadline(c.effectiveDeadline(ctx))
	return sendPacket(c.conn, cmd, data)
}

// recv checks ctx before reading, then receives a response.
func (c *Client) recv(ctx context.Context) (Action, error) {
	select {
	case <-ctx.Done():
		return ActionTempfail, fmt.Errorf("milter: context done before recv: %w", ctx.Err())
	default:
	}
	_ = c.conn.SetDeadline(c.effectiveDeadline(ctx))
	cmd, _, err := recvPacket(c.conn)
	if err != nil {
		return ActionUnknown, err
	}
	return ActionFromCode(cmd), nil
}

// containsCRLF reports whether s contains a carriage return or newline character.
func containsCRLF(s string) bool {
	return strings.ContainsAny(s, "\r\n")
}

// Negotiate performs the OPTNEG handshake. Must be called before any other method.
func (c *Client) Negotiate(ctx context.Context) error {
	data := make([]byte, 12)
	binary.BigEndian.PutUint32(data[0:4], milterVersion)
	binary.BigEndian.PutUint32(data[4:8], milterActions)
	binary.BigEndian.PutUint32(data[8:12], 0)

	if err := c.send(ctx, cmdOptneg, data); err != nil {
		return fmt.Errorf("milter negotiate send: %w", err)
	}

	select {
	case <-ctx.Done():
		return fmt.Errorf("milter negotiate recv: %w", ctx.Err())
	default:
	}
	_ = c.conn.SetDeadline(c.effectiveDeadline(ctx))
	cmd, _, err := recvPacket(c.conn)
	if err != nil {
		return fmt.Errorf("milter negotiate recv: %w", err)
	}
	if cmd != cmdOptneg {
		return fmt.Errorf("milter negotiate: unexpected response %q", cmd)
	}
	return nil
}

// Connect sends the CONNECT event (client IP and hostname).
func (c *Client) Connect(ctx context.Context, hostname string, family byte, port uint16, addr string) (Action, error) {
	data := make([]byte, 0, len(hostname)+1+1+2+len(addr)+1)
	data = append(data, []byte(hostname)...)
	data = append(data, 0)
	data = append(data, family)
	var portBuf [2]byte
	binary.BigEndian.PutUint16(portBuf[:], port)
	data = append(data, portBuf[:]...)
	data = append(data, []byte(addr)...)
	data = append(data, 0)
	if err := c.send(ctx, cmdConnect, data); err != nil {
		return ActionUnknown, fmt.Errorf("milter connect send: %w", err)
	}
	return c.recv(ctx)
}

// Helo sends the HELO/EHLO event.
func (c *Client) Helo(ctx context.Context, helo string) (Action, error) {
	data := append([]byte(helo), 0)
	if err := c.send(ctx, cmdHelo, data); err != nil {
		return ActionUnknown, fmt.Errorf("milter helo send: %w", err)
	}
	return c.recv(ctx)
}

// MailFrom sends the MAIL FROM event.
func (c *Client) MailFrom(ctx context.Context, from string) (Action, error) {
	data := append([]byte(from), 0)
	if err := c.send(ctx, cmdMail, data); err != nil {
		return ActionUnknown, fmt.Errorf("milter mail send: %w", err)
	}
	return c.recv(ctx)
}

// RcptTo sends the RCPT TO event.
func (c *Client) RcptTo(ctx context.Context, to string) (Action, error) {
	data := append([]byte(to), 0)
	if err := c.send(ctx, cmdRcpt, data); err != nil {
		return ActionUnknown, fmt.Errorf("milter rcpt send: %w", err)
	}
	return c.recv(ctx)
}

// Header sends a single message header.
// Returns an error if name or value contains CRLF characters (injection protection).
func (c *Client) Header(ctx context.Context, name, value string) (Action, error) {
	if containsCRLF(name) {
		return ActionUnknown, fmt.Errorf("milter: header name contains illegal CRLF characters")
	}
	if containsCRLF(value) {
		return ActionUnknown, fmt.Errorf("milter: header value contains illegal CRLF characters")
	}
	data := make([]byte, 0, len(name)+1+len(value)+1)
	data = append(data, []byte(name)...)
	data = append(data, 0)
	data = append(data, []byte(value)...)
	data = append(data, 0)
	if err := c.send(ctx, cmdHeader, data); err != nil {
		return ActionUnknown, fmt.Errorf("milter header send: %w", err)
	}
	return c.recv(ctx)
}

// EndOfHeaders signals the end of message headers.
func (c *Client) EndOfHeaders(ctx context.Context) (Action, error) {
	if err := c.send(ctx, cmdEOH, nil); err != nil {
		return ActionUnknown, fmt.Errorf("milter eoh send: %w", err)
	}
	return c.recv(ctx)
}

// BodyChunk sends a chunk of the message body.
func (c *Client) BodyChunk(ctx context.Context, chunk []byte) (Action, error) {
	if err := c.send(ctx, cmdBody, chunk); err != nil {
		return ActionUnknown, fmt.Errorf("milter body send: %w", err)
	}
	return c.recv(ctx)
}

// EndOfMessage signals end of message and returns the final verdict.
func (c *Client) EndOfMessage(ctx context.Context) (Action, error) {
	if err := c.send(ctx, cmdEOB, nil); err != nil {
		return ActionUnknown, fmt.Errorf("milter eom send: %w", err)
	}
	return c.recv(ctx)
}

// Abort aborts the current message (no server response expected).
func (c *Client) Abort(ctx context.Context) error {
	return c.send(ctx, cmdAbort, nil)
}

// Quit sends QUIT and closes the connection.
func (c *Client) Quit(ctx context.Context) error {
	_ = c.send(ctx, cmdQuit, nil)
	return c.conn.Close()
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.conn.Close()
}

// Handler is a server-side milter event interface (for embedding in milter servers).
type Handler interface {
	OnConnect(hostname string, family byte, port uint16, address string) Action
	OnHelo(helo string) Action
	OnMail(from string) Action
	OnRcpt(to string) Action
	OnData() Action
	OnEOB() Action
}

type Milter struct {
	handler Handler
}

func NewMilter(handler Handler) *Milter {
	return &Milter{handler: handler}
}
