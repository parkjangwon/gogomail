package ldapgw

import (
	"context"
	"encoding/binary"
	"net"
	"testing"
	"time"
)

// TestBERLargeMessageRejected verifies that a PDU whose declared BER length
// exceeds maxBERMessageSize causes the server to close the connection.
func TestBERLargeMessageRejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := NewServer(ln, newFakeLDAPAuth(), newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Build SEQUENCE header with declared length = 32 MB (> 16 MB cap).
	// Long-form: 0x84 followed by 4 big-endian bytes.
	const declaredLen = 32 * 1024 * 1024
	var lb [4]byte
	binary.BigEndian.PutUint32(lb[:], uint32(declaredLen))
	oversized := []byte{
		tagSequence,
		0x84, lb[0], lb[1], lb[2], lb[3],
		0x01, 0x02, 0x03, // a few body bytes — server must reject on length alone
	}

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := conn.Write(oversized); err != nil {
		t.Fatal(err)
	}

	// Server must close without sending a response.
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 64)
	n, readErr := conn.Read(buf)
	if n > 0 {
		t.Errorf("expected no response to oversized message, got %d bytes: %x", n, buf[:n])
	}
	if readErr == nil {
		t.Error("expected connection closed by server, but Read returned nil error")
	}
}

// TestContextDeadlineEnforced verifies that cancelling the server context
// prevents it from returning a successful bind response.
func TestContextDeadlineEnforced(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("user@example.com", "pass")

	ctx, cancel := context.WithCancel(context.Background())
	srv := &LDAPServer{
		ln:     ln,
		auth:   auth,
		quer:   newFakeDirectoryQuerier(),
		ctx:    ctx,
		cancel: cancel,
	}
	go srv.Serve()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Cancel context before any PDU is processed.
	cancel()
	time.Sleep(50 * time.Millisecond)

	bindPDU := buildLDAPPacket(1, opBindRequest, buildBindRequest(3, "user@example.com", "pass"))
	conn.SetWriteDeadline(time.Now().Add(time.Second))
	conn.Write(bindPDU) //nolint:errcheck — connection may already be closed

	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	buf := make([]byte, 256)
	n, _ := conn.Read(buf)
	if n > 0 {
		_, opTag, opData, decErr := decodeLDAPPacket(buf[:n])
		if decErr == nil && opTag == opBindResponse {
			if decodeEnumerated(opData) == resultSuccess {
				t.Error("got resultSuccess after context cancellation — deadline not enforced")
			}
		}
	}
	// n==0 means connection was closed by server — also correct.
}

// TestMalformedFilterRejected verifies that a search request containing a
// truncated filter receives SearchResultDone with a non-success result code.
func TestMalformedFilterRejected(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := NewServer(ln, newFakeLDAPAuth(), newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Filter declares 32 bytes of content but only 4 bytes follow.
	malformedFilter := []byte{
		tagContextSpecific | filterEqualityMatch, // 0x83
		0x20,                                     // declares 32 bytes
		0x04, 0x02, 'c', 'n',                    // only 4 bytes
	}
	pdu := buildLDAPPacket(7, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, malformedFilter))

	conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if err := sendPDU(conn, pdu); err != nil {
		t.Fatal(err)
	}

	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatalf("readFullPDU: %v", err)
	}
	_, opTag, opData, decErr := decodeLDAPPacket(resp)
	if decErr != nil {
		t.Fatalf("decodeLDAPPacket: %v", decErr)
	}
	if opTag != opSearchResultDone {
		t.Fatalf("opTag = 0x%02x, want opSearchResultDone (0x%02x)", opTag, opSearchResultDone)
	}
	if decodeEnumerated(opData) == resultSuccess {
		t.Error("expected non-success result for malformed filter, got resultSuccess")
	}
}

// TestValidateFilter unit-tests the validateFilter helper directly.
func TestValidateFilter(t *testing.T) {
	validEqualityFilter := func() []byte {
		content := append(encodeOctetString("cn"), encodeOctetString("alice")...)
		hdr := []byte{tagContextSpecific | filterEqualityMatch}
		hdr = append(hdr, encodeLength(len(content))...)
		return append(hdr, content...)
	}()

	cases := []struct {
		name    string
		data    []byte
		wantErr bool
	}{
		{"empty", []byte{}, true},
		{"non-context-specific tag", []byte{0x04, 0x02, 'c', 'n'}, true},
		{"unsupported filter type 31", []byte{0x9f, 0x02, 0x00, 0x00}, true},
		{"truncated content", []byte{tagContextSpecific | filterPresent, 0x10, 0x04, 0x01}, true},
		{"valid equality", validEqualityFilter, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateFilter(tc.data)
			if tc.wantErr && err == nil {
				t.Error("expected error, got nil")
			}
			if !tc.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
