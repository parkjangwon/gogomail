package ldapgw

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"
	"testing"
	"time"
)

type fakeLDAPAuth struct {
	mu    sync.Mutex
	creds map[string]string
}

func newFakeLDAPAuth() *fakeLDAPAuth {
	return &fakeLDAPAuth{creds: make(map[string]string)}
}

func (f *fakeLDAPAuth) AuthenticateLDAP(ctx context.Context, username, password string) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if pwd, ok := f.creds[username]; ok && pwd == password {
		return true, nil
	}
	return false, nil
}

func (f *fakeLDAPAuth) addUser(username, password string) {
	f.mu.Lock()
	f.creds[username] = password
	f.mu.Unlock()
}

type fakeDirectoryQuerier struct {
	mu         sync.Mutex
	principals []PrincipalEntry
}

func newFakeDirectoryQuerier() *fakeDirectoryQuerier {
	return &fakeDirectoryQuerier{principals: make([]PrincipalEntry, 0)}
}

func (f *fakeDirectoryQuerier) SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var result []PrincipalEntry
	for _, p := range f.principals {
		result = append(result, p)
	}
	return result, nil
}

func (f *fakeDirectoryQuerier) addPrincipal(p PrincipalEntry) {
	f.mu.Lock()
	f.principals = append(f.principals, p)
	f.mu.Unlock()
}

func buildLDAPPacket(msgID int, opTag int, opData []byte) []byte {
	var opContent []byte
	opContent = append([]byte{byte(opTag)}, encodeLength(len(opData))...)
	opContent = append(opContent, opData...)

	var msgIDContent []byte
	msgIDContent = append(msgIDContent, tagInteger)
	msgIDContent = append(msgIDContent, encodeLength(1)...)
	msgIDContent = append(msgIDContent, byte(msgID))

	var seqContent []byte
	seqContent = append(seqContent, msgIDContent...)
	seqContent = append(seqContent, opContent...)

	result := make([]byte, 0, 2+len(seqContent))
	result = append(result, tagSequence)
	result = append(result, encodeLength(len(seqContent))...)
	result = append(result, seqContent...)
	return result
}

func buildBindRequest(version int, name, password string) []byte {
	// Returns raw bind request content (version, name, authentication).
	// The APPLICATION tag (opBindRequest=0x60) is added by buildLDAPPacket.
	var content []byte
	content = append(content, encodeInt(version)...)
	content = append(content, encodeOctetString(name)...)
	content = append(content, 0x80) // context-specific simple authentication tag
	content = append(content, encodeLength(len(password))...)
	content = append(content, []byte(password)...)
	return content
}

func buildSearchRequest(baseDN string, scope int, filter []byte) []byte {
	var content []byte
	content = append(content, encodeOctetString(baseDN)...)
	content = append(content, encodeInt(scope)...)
	content = append(content, encodeInt(0)...)
	content = append(content, filter...)
	content = append(content, encodeInt(0)...)
	content = append(content, encodeInt(0)...)
	var attrList []byte
	attrList = append(attrList, tagSequence)
	attrList = append(attrList, encodeLength(0)...)
	content = append(content, attrList...)
	return content
}

func sendPDU(conn net.Conn, pdu []byte) error {
	_, err := conn.Write(pdu)
	return err
}

func readFullPDU(conn net.Conn, deadline time.Time) ([]byte, error) {
	conn.SetReadDeadline(deadline)
	tag := make([]byte, 1)
	if _, err := io.ReadFull(conn, tag); err != nil {
		return nil, err
	}
	if tag[0] != tagSequence {
		return nil, fmt.Errorf("expected SEQUENCE tag, got 0x%02x", tag[0])
	}
	firstLen := make([]byte, 1)
	if _, err := io.ReadFull(conn, firstLen); err != nil {
		return nil, err
	}
	header := []byte{tag[0], firstLen[0]}
	var bodyLen int
	if firstLen[0]&0x80 == 0 {
		bodyLen = int(firstLen[0])
	} else {
		numBytes := int(firstLen[0] & 0x7f)
		extra := make([]byte, numBytes)
		if _, err := io.ReadFull(conn, extra); err != nil {
			return nil, err
		}
		header = append(header, extra...)
		for _, b := range extra {
			bodyLen = bodyLen<<8 | int(b)
		}
	}
	if bodyLen > 65536 {
		return nil, fmt.Errorf("PDU too large: %d", bodyLen)
	}
	body := make([]byte, bodyLen)
	if _, err := io.ReadFull(conn, body); err != nil {
		return nil, err
	}
	return append(header, body...), nil
}

func TestLDAPServerBindSuccess(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("admin@example.com", "secret")
	dir := newFakeDirectoryQuerier()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	bindReq := buildLDAPPacket(1, opBindRequest, buildBindRequest(3, "admin@example.com", "secret"))
	if err := sendPDU(conn, bindReq); err != nil {
		t.Fatal(err)
	}

	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	msgID, opTag, _, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msgID != 1 {
		t.Errorf("msgID = %d, want 1", msgID)
	}
	if opTag != opBindResponse {
		t.Errorf("opTag = %d, want %d (BindResponse)", opTag, opBindResponse)
	}
}

func TestLDAPServerBindInvalidCredentials(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("admin@example.com", "secret")
	dir := newFakeDirectoryQuerier()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	bindReq := buildLDAPPacket(2, opBindRequest, buildBindRequest(3, "admin@example.com", "wrongpassword"))
	if err := sendPDU(conn, bindReq); err != nil {
		t.Fatal(err)
	}

	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if opTag != opBindResponse {
		t.Fatalf("opTag = %d, want %d", opTag, opBindResponse)
	}
	resultCode := decodeEnumerated(opData)
	if resultCode != resultInvalidCredentials {
		t.Errorf("resultCode = %d, want %d (InvalidCredentials)", resultCode, resultInvalidCredentials)
	}
}

func TestLDAPServerReadOnlyEnforcement(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	modifyReq := buildLDAPPacket(3, opModifyRequest, []byte{0x01})
	if err := sendPDU(conn, modifyReq); err != nil {
		t.Fatal(err)
	}

	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, _, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if opTag != opModifyRequest {
		t.Errorf("opTag = %d, want %d", opTag, opModifyRequest)
	}
}

func TestLDAPServerUnbindClosesConnection(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}

	unbindReq := buildLDAPPacket(4, opUnbindRequest, []byte{})
	if err := sendPDU(conn, unbindReq); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	_, err = conn.Read(buf)
	if err == nil {
		t.Error("connection should be closed after unbind")
	}
}

func TestLDAPServerSearchRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("admin@example.com", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:   "uid=alice,ou=users,dc=example,dc=com",
		CN:   "alice",
		Mail: "alice@example.com",
		UID:  "alice",
		DisplayName: "Alice User",
	})

	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	filterData := []byte{tagContextSpecific | filterEqualityMatch}
	filterContent := append(encodeOctetString("mail"), encodeOctetString("alice@example.com")...)
	filterData = append(filterData, encodeLength(len(filterContent))...)
	filterData = append(filterData, filterContent...)

	searchReq := buildLDAPPacket(5, opSearchRequest, buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, filterData))
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}

	found := false
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
		if err != nil {
			if err == io.EOF {
				break
			}
			continue
		}
		msgID, opTag, opData, _ := decodeLDAPPacket(resp)
		t.Logf("response: msgID=%d, opTag=%d, opData len=%d", msgID, opTag, len(opData))
		if opTag == opSearchResultEntry {
			found = true
			break
		}
		if opTag == opSearchResultDone {
			break
		}
	}
	if !found {
		t.Error("expected SearchResultEntry after search request")
	}
}


