package ldapgw

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestServerWithRealTCP(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := &debugAuth{}
	dir := &debugDir{}
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	// Build a complete bind request PDU
	bindData := []byte{0x02, 0x01, 0x03, 0x04, 0x10, 't', 'e', 's', 't', '@', 'e', 'x', 0x80, 0x06, 'p', 'a', 's', 's', 'w', 'd'}
	pdu := buildLDAPPacket(1, opBindRequest, bindData)
	t.Logf("Sending PDU: %x", pdu)

	conn.SetWriteDeadline(time.Now().Add(3 * time.Second))
	n, err := conn.Write(pdu)
	t.Logf("Wrote %d bytes", n)
	if err != nil {
		t.Fatal(err)
	}

	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	buf := make([]byte, 1024)
	n, err = conn.Read(buf)
	t.Logf("Read %d bytes: %x", n, buf[:n])
	if err != nil {
		t.Logf("Read error: %v", err)
	}
}

type debugAuth struct{}

func (a *debugAuth) AuthenticateLDAP(ctx context.Context, username, password string) (bool, error) {
	return username == "test@example.com" && password == "password", nil
}

type debugDir struct{}

func (d *debugDir) SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error) {
	return nil, nil
}
