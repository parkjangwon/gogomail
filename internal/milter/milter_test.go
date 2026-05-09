package milter

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// stubServer runs a goroutine acting as a milter server.
// It responds to OPTNEG automatically; for all other commands it sends finalAction.
// QUIT causes the goroutine to exit. ABORT receives no response (per spec).
func stubServer(t *testing.T, conn net.Conn, finalAction byte) {
	t.Helper()
	go func() {
		defer conn.Close()
		for {
			cmd, _, err := recvPacket(conn)
			if err != nil {
				return
			}
			switch cmd {
			case cmdOptneg:
				data := make([]byte, 12)
				binary.BigEndian.PutUint32(data[0:4], 6)
				binary.BigEndian.PutUint32(data[4:8], 0x7F)
				binary.BigEndian.PutUint32(data[8:12], 0)
				if err := sendPacket(conn, cmdOptneg, data); err != nil {
					return
				}
			case cmdQuit, cmdQuitNewCon:
				return
			case cmdAbort:
				// no response
			default:
				if err := sendPacket(conn, finalAction, nil); err != nil {
					return
				}
			}
		}
	}()
}

func TestCommandConstants(t *testing.T) {
	if cmdConnect != 'C' {
		t.Fatalf("cmdConnect = %d, want %d", cmdConnect, 'C')
	}
	if cmdHelo != 'H' {
		t.Fatalf("cmdHelo = %d, want %d", cmdHelo, 'H')
	}
	if cmdMail != 'M' {
		t.Fatalf("cmdMail = %d, want %d", cmdMail, 'M')
	}
	if cmdRcpt != 'R' {
		t.Fatalf("cmdRcpt = %d, want %d", cmdRcpt, 'R')
	}
	if cmdData != 'T' {
		t.Fatalf("cmdData = %d, want %d", cmdData, 'T')
	}
	if cmdEOB != 'E' {
		t.Fatalf("cmdEOB = %d, want %d", cmdEOB, 'E')
	}
}

func TestResponseConstants(t *testing.T) {
	if respContinue != 'c' {
		t.Fatalf("respContinue = %d, want %d", respContinue, 'c')
	}
	if respReject != 'r' {
		t.Fatalf("respReject = %d, want %d", respReject, 'r')
	}
	if respTempfail != 't' {
		t.Fatalf("respTempfail = %d, want %d", respTempfail, 't')
	}
}

func TestEncodePacket(t *testing.T) {
	pkt := &Packet{
		Command: cmdConnect,
		Data:    []byte("192.168.1.1\x00test.example.com"),
	}

	data, err := pkt.Encode()
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("Encode returned empty data")
	}

	if len(data) < 5 {
		t.Fatalf("data too short: %d", len(data))
	}
}

func TestDecodePacket(t *testing.T) {
	original := &Packet{
		Command: cmdHelo,
		Data:    []byte("mail.example.com"),
	}

	encoded, err := original.Encode()
	if err != nil {
		t.Fatalf("Encode error: %v", err)
	}

	decoded, err := DecodePacket(encoded)
	if err != nil {
		t.Fatalf("DecodePacket error: %v", err)
	}
	if decoded.Command != cmdHelo {
		t.Fatalf("Command = %d, want %d", decoded.Command, cmdHelo)
	}
	if string(decoded.Data) != "mail.example.com" {
		t.Fatalf("Data = %s, want mail.example.com", string(decoded.Data))
	}
}

func TestDecodePacketTooShort(t *testing.T) {
	_, err := DecodePacket([]byte{0, 0, 0})
	if err == nil {
		t.Fatal("DecodePacket should error for short data")
	}
}

func TestActionFromCode(t *testing.T) {
	tests := []struct {
		code     byte
		expected Action
	}{
		{respContinue, ActionContinue},
		{respReject, ActionReject},
		{respTempfail, ActionTempfail},
		{'X', ActionUnknown},
	}

	for _, tt := range tests {
		got := ActionFromCode(tt.code)
		if got != tt.expected {
			t.Errorf("ActionFromCode(%d) = %d, want %d", tt.code, got, tt.expected)
		}
	}
}

// --- Client tests ---

func newTestClient(t *testing.T) (*Client, net.Conn) {
	t.Helper()
	clientConn, serverConn := net.Pipe()
	c := NewClient(clientConn, 5*time.Second)
	return c, serverConn
}

func TestClientNegotiate(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respContinue)

	if err := c.Negotiate(context.Background()); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}
}

func TestClientAcceptAtEOM(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respAccept)

	ctx := context.Background()
	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}

	action, err := c.Connect(ctx, "mail.example.com", FamilyIPv4, 25, "1.2.3.4")
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	if action != ActionAccept {
		t.Fatalf("Connect action = %v, want Accept", action)
	}
}

func TestClientRejectAtMailFrom(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respReject)

	ctx := context.Background()
	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}

	action, err := c.MailFrom(ctx, "<sender@example.com>")
	if err != nil {
		t.Fatalf("MailFrom: %v", err)
	}
	if action != ActionReject {
		t.Fatalf("MailFrom action = %v, want Reject", action)
	}
}

func TestClientTempfailAtRcptTo(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respTempfail)

	ctx := context.Background()
	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}

	action, err := c.RcptTo(ctx, "<rcpt@example.com>")
	if err != nil {
		t.Fatalf("RcptTo: %v", err)
	}
	if action != ActionTempfail {
		t.Fatalf("RcptTo action = %v, want Tempfail", action)
	}
}

func TestClientFullMessageFlow(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	c := NewClient(clientConn, 5*time.Second)
	defer c.Close()

	// Server: CONTINUE for everything, ACCEPT at EOM
	go func() {
		defer serverConn.Close()
		commands := []byte{cmdOptneg, cmdConnect, cmdHelo, cmdMail, cmdRcpt, cmdHeader, cmdEOH, cmdBody, cmdEOB}
		for _, expected := range commands {
			cmd, _, err := recvPacket(serverConn)
			if err != nil {
				return
			}
			if expected == cmdOptneg {
				data := make([]byte, 12)
				binary.BigEndian.PutUint32(data[0:4], 6)
				binary.BigEndian.PutUint32(data[4:8], 0x7F)
				binary.BigEndian.PutUint32(data[8:12], 0)
				sendPacket(serverConn, cmdOptneg, data)
			} else if cmd == cmdEOB {
				sendPacket(serverConn, respAccept, nil)
			} else {
				sendPacket(serverConn, respContinue, nil)
			}
		}
	}()

	ctx := context.Background()
	steps := []struct {
		name string
		fn   func() (Action, error)
	}{
		{"Connect", func() (Action, error) { return c.Connect(ctx, "mx.example.com", FamilyIPv4, 25, "10.0.0.1") }},
		{"Helo", func() (Action, error) { return c.Helo(ctx, "mx.example.com") }},
		{"MailFrom", func() (Action, error) { return c.MailFrom(ctx, "<from@example.com>") }},
		{"RcptTo", func() (Action, error) { return c.RcptTo(ctx, "<to@example.com>") }},
		{"Header", func() (Action, error) { return c.Header(ctx, "Subject", "Test") }},
		{"EndOfHeaders", func() (Action, error) { return c.EndOfHeaders(ctx) }},
		{"BodyChunk", func() (Action, error) { return c.BodyChunk(ctx, []byte("body")) }},
		{"EndOfMessage", func() (Action, error) { return c.EndOfMessage(ctx) }},
	}

	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}

	for _, step := range steps {
		action, err := step.fn()
		if err != nil {
			t.Fatalf("%s: %v", step.name, err)
		}
		if step.name == "EndOfMessage" {
			if action != ActionAccept {
				t.Fatalf("EndOfMessage action = %v, want Accept", action)
			}
		} else {
			if action != ActionContinue {
				t.Fatalf("%s action = %v, want Continue", step.name, action)
			}
		}
	}
}

func TestClientAbort(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respContinue)

	ctx := context.Background()
	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}
	if err := c.Abort(ctx); err != nil {
		t.Fatalf("Abort: %v", err)
	}
}

func TestClientDiscard(t *testing.T) {
	c, srv := newTestClient(t)
	defer c.Close()
	stubServer(t, srv, respDiscard)

	ctx := context.Background()
	if err := c.Negotiate(ctx); err != nil {
		t.Fatalf("Negotiate: %v", err)
	}
	action, err := c.EndOfMessage(ctx)
	if err != nil {
		t.Fatalf("EndOfMessage: %v", err)
	}
	if action != ActionDiscard {
		t.Fatalf("action = %v, want Discard", action)
	}
}

func TestClientConnectFamily(t *testing.T) {
	cases := []struct {
		family byte
		name   string
	}{
		{FamilyIPv4, "IPv4"},
		{FamilyIPv6, "IPv6"},
		{FamilyUnknown, "Unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, srv := newTestClient(t)
			defer c.Close()
			stubServer(t, srv, respContinue)
			ctx := context.Background()
			if err := c.Negotiate(ctx); err != nil {
				t.Fatalf("Negotiate: %v", err)
			}
			_, err := c.Connect(ctx, "host", tc.family, 25, "addr")
			if err != nil {
				t.Fatalf("Connect(%s): %v", tc.name, err)
			}
		})
	}
}
