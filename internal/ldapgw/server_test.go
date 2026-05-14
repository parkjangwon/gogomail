package ldapgw

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net"
	"os"
	"os/exec"
	"strings"
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

type fakeLDAPMetrics struct {
	mu     sync.Mutex
	events []MetricEvent
}

func (f *fakeLDAPMetrics) ObserveLDAP(_ context.Context, event MetricEvent) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.events = append(f.events, event)
}

func (f *fakeLDAPMetrics) snapshot() []MetricEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]MetricEvent, len(f.events))
	copy(out, f.events)
	return out
}

func newFakeDirectoryQuerier() *fakeDirectoryQuerier {
	return &fakeDirectoryQuerier{principals: make([]PrincipalEntry, 0)}
}

func (f *fakeDirectoryQuerier) SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var filtered []PrincipalEntry
	for _, p := range f.principals {
		if len(req.Kinds) > 0 && !containsStringFold(req.Kinds, firstNonEmpty(p.Kind, "user")) {
			continue
		}
		filtered = append(filtered, p)
	}
	start := req.Offset
	if start > len(filtered) {
		start = len(filtered)
	}
	end := len(filtered)
	if req.Limit > 0 && start+req.Limit < end {
		end = start + req.Limit
	}
	var result []PrincipalEntry
	for _, p := range filtered[start:end] {
		result = append(result, p)
	}
	return result, nil
}

func (f *fakeDirectoryQuerier) addPrincipal(p PrincipalEntry) {
	f.mu.Lock()
	f.principals = append(f.principals, p)
	f.mu.Unlock()
}

func containsStringFold(values []string, want string) bool {
	for _, value := range values {
		if strings.EqualFold(value, want) {
			return true
		}
	}
	return false
}

func buildLDAPPacket(msgID int, opTag int, opData []byte) []byte {
	return buildLDAPPacketWithControls(msgID, opTag, opData, nil)
}

func buildLDAPPacketWithControls(msgID int, opTag int, opData []byte, controls []control) []byte {
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
	if len(controls) > 0 {
		seqContent = append(seqContent, encodeTestControls(controls)...)
	}

	result := make([]byte, 0, 2+len(seqContent))
	result = append(result, tagSequence)
	result = append(result, encodeLength(len(seqContent))...)
	result = append(result, seqContent...)
	return result
}

func encodeTestControls(controls []control) []byte {
	var controlsContent []byte
	for _, ctrl := range controls {
		var ctrlContent []byte
		ctrlContent = append(ctrlContent, encodeOctetString(ctrl.Type)...)
		if ctrl.Critical {
			ctrlContent = append(ctrlContent, tagBoolean, 0x01, 0xff)
		}
		if ctrl.Value != nil {
			ctrlContent = append(ctrlContent, tagOctetString)
			ctrlContent = append(ctrlContent, encodeLength(len(ctrl.Value))...)
			ctrlContent = append(ctrlContent, ctrl.Value...)
		}
		controlsContent = append(controlsContent, tagSequence)
		controlsContent = append(controlsContent, encodeLength(len(ctrlContent))...)
		controlsContent = append(controlsContent, ctrlContent...)
	}
	var wrapped []byte
	wrapped = append(wrapped, 0xa0)
	wrapped = append(wrapped, encodeLength(len(controlsContent))...)
	wrapped = append(wrapped, controlsContent...)
	return wrapped
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
	return buildSearchRequestWithParams(baseDN, scope, derefAliasesNever, filter)
}

func buildSearchRequestWithParams(baseDN string, scope int, derefAliases int, filter []byte) []byte {
	var content []byte
	content = append(content, encodeOctetString(baseDN)...)
	content = append(content, encodeEnumerated(scope)...)
	content = append(content, encodeEnumerated(derefAliases)...)
	content = append(content, encodeInt(0)...)
	content = append(content, encodeInt(0)...)
	content = append(content, tagBoolean, 0x01, 0x00)
	content = append(content, filter...)
	var attrList []byte
	attrList = append(attrList, tagSequence)
	attrList = append(attrList, encodeLength(0)...)
	content = append(content, attrList...)
	return content
}

func buildSearchRequestWithAttrs(baseDN string, scope int, filter []byte, attrs ...string) []byte {
	var content []byte
	content = append(content, encodeOctetString(baseDN)...)
	content = append(content, encodeEnumerated(scope)...)
	content = append(content, encodeEnumerated(0)...)
	content = append(content, encodeInt(0)...)
	content = append(content, encodeInt(0)...)
	content = append(content, tagBoolean, 0x01, 0x00)
	content = append(content, filter...)
	var attrContent []byte
	for _, attr := range attrs {
		attrContent = append(attrContent, encodeOctetString(attr)...)
	}
	content = append(content, tagSequence)
	content = append(content, encodeLength(len(attrContent))...)
	content = append(content, attrContent...)
	return content
}

func buildExtendedRequest(name string) []byte {
	var content []byte
	content = append(content, 0x80)
	content = append(content, encodeLength(len(name))...)
	content = append(content, []byte(name)...)
	return content
}

func buildEqualityFilter(attr, value string) []byte {
	filterContent := append(encodeOctetString(attr), encodeOctetString(value)...)
	filterData := []byte{tagContextSpecific | filterEqualityMatch}
	filterData = append(filterData, encodeLength(len(filterContent))...)
	return append(filterData, filterContent...)
}

func buildSubstringFilter(attr string, parts ...string) []byte {
	var substrings []byte
	for _, part := range parts {
		substrings = append(substrings, 0x81)
		substrings = append(substrings, encodeLength(len(part))...)
		substrings = append(substrings, []byte(part)...)
	}
	substringSeq := []byte{tagSequence}
	substringSeq = append(substringSeq, encodeLength(len(substrings))...)
	substringSeq = append(substringSeq, substrings...)
	filterContent := append(encodeOctetString(attr), substringSeq...)
	filterData := []byte{tagContextSpecific | filterSubstrings}
	filterData = append(filterData, encodeLength(len(filterContent))...)
	return append(filterData, filterContent...)
}

func buildExtensibleFilter(attr, value string) []byte {
	var filterContent []byte
	filterContent = append(filterContent, 0x82)
	filterContent = append(filterContent, encodeLength(len(attr))...)
	filterContent = append(filterContent, []byte(attr)...)
	filterContent = append(filterContent, 0x83)
	filterContent = append(filterContent, encodeLength(len(value))...)
	filterContent = append(filterContent, []byte(value)...)
	filterData := []byte{tagContextSpecific | 0x20 | filterExtensible}
	filterData = append(filterData, encodeLength(len(filterContent))...)
	return append(filterData, filterContent...)
}

func buildOrFilter(children ...[]byte) []byte {
	var content []byte
	for _, child := range children {
		content = append(content, child...)
	}
	filterData := []byte{tagContextSpecific | 0x20 | filterOr}
	filterData = append(filterData, encodeLength(len(content))...)
	return append(filterData, content...)
}

func buildPagedResultsControl(pageSize int, cookie string) control {
	var value []byte
	value = append(value, encodeInt(pageSize)...)
	value = append(value, encodeOctetString(cookie)...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(value))...)
	seq = append(seq, value...)
	return control{Type: controlPagedResults, Value: seq}
}

func buildServerSideSortControl(attr string, reverse bool) control {
	keyContent := encodeOctetString(attr)
	if reverse {
		keyContent = append(keyContent, 0x81, 0x01, 0xff)
	}
	var keySeq []byte
	keySeq = append(keySeq, tagSequence)
	keySeq = append(keySeq, encodeLength(len(keyContent))...)
	keySeq = append(keySeq, keyContent...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(keySeq))...)
	seq = append(seq, keySeq...)
	return control{Type: controlServerSideSortRequest, Value: seq}
}

func buildVirtualListViewControl(before, after, offset, contentCount int) control {
	var target []byte
	target = append(target, encodeInt(offset)...)
	target = append(target, encodeInt(contentCount)...)
	var targetChoice []byte
	targetChoice = append(targetChoice, 0xa0)
	targetChoice = append(targetChoice, encodeLength(len(target))...)
	targetChoice = append(targetChoice, target...)
	var value []byte
	value = append(value, encodeInt(before)...)
	value = append(value, encodeInt(after)...)
	value = append(value, targetChoice...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(value))...)
	seq = append(seq, value...)
	return control{Type: controlVirtualListViewRequest, Value: seq}
}

func buildAssertionControl(filter []byte, critical bool) control {
	return control{Type: controlAssertion, Critical: critical, Value: filter}
}

func buildMatchedValuesControl(filter []byte, critical bool) control {
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(filter))...)
	seq = append(seq, filter...)
	return control{Type: controlMatchedValues, Critical: critical, Value: seq}
}

func buildSubentriesControl(value bool, critical bool) control {
	encoded := []byte{tagBoolean, 0x01, 0x00}
	if value {
		encoded[2] = 0xff
	}
	return control{Type: controlSubentries, Critical: critical, Value: encoded}
}

func buildSyncRequestControl(mode int, cookie string, critical bool) control {
	var value []byte
	value = append(value, encodeEnumerated(mode)...)
	if cookie != "" {
		value = append(value, encodeOctetString(cookie)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(value))...)
	seq = append(seq, value...)
	return control{Type: controlSyncRequest, Critical: critical, Value: seq}
}

func buildProxiedAuthorizationControl(authzID string, critical bool) control {
	return control{Type: controlProxiedAuthorization, Critical: critical, Value: []byte(authzID)}
}

func buildDereferenceControl(derefAttr string, attrs ...string) control {
	var attrList []byte
	for _, attr := range attrs {
		attrList = append(attrList, encodeOctetString(attr)...)
	}
	var attrSeq []byte
	attrSeq = append(attrSeq, tagSequence)
	attrSeq = append(attrSeq, encodeLength(len(attrList))...)
	attrSeq = append(attrSeq, attrList...)
	specContent := append(encodeOctetString(derefAttr), attrSeq...)
	var specSeq []byte
	specSeq = append(specSeq, tagSequence)
	specSeq = append(specSeq, encodeLength(len(specContent))...)
	specSeq = append(specSeq, specContent...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(specSeq))...)
	seq = append(seq, specSeq...)
	return control{Type: controlDereferenceRequest, Critical: true, Value: seq}
}

func buildCompareRequest(entry, attr, value string) []byte {
	assertion := append(encodeOctetString(attr), encodeOctetString(value)...)
	var assertionSeq []byte
	assertionSeq = append(assertionSeq, tagSequence)
	assertionSeq = append(assertionSeq, encodeLength(len(assertion))...)
	assertionSeq = append(assertionSeq, assertion...)
	return append(encodeOctetString(entry), assertionSeq...)
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

func readSearchUntilDone(t *testing.T, conn net.Conn) (int, []control) {
	t.Helper()
	entries := 0
	for {
		resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
		if err != nil {
			t.Fatalf("read search response: %v", err)
		}
		_, opTag, _, controls, err := decodeLDAPPacketWithControls(resp)
		if err != nil {
			t.Fatalf("decode search response: %v", err)
		}
		switch opTag {
		case opSearchResultEntry:
			entries++
		case opSearchResultDone:
			return entries, controls
		default:
			t.Fatalf("unexpected search response opTag = %d", opTag)
		}
	}
}

func bindTestConnection(t *testing.T, conn net.Conn, auth *fakeLDAPAuth) {
	t.Helper()
	auth.addUser("tester", "secret")
	bindReq := buildLDAPPacket(90, opBindRequest, buildBindRequest(ldapV3, "tester", "secret"))
	if err := sendPDU(conn, bindReq); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode bind response: %v", err)
	}
	if opTag != opBindResponse || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("bind response op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opBindResponse, resultSuccess)
	}
}

func readSearchDNsUntilDone(t *testing.T, conn net.Conn) ([]string, []control) {
	t.Helper()
	var dns []string
	for {
		resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
		if err != nil {
			t.Fatalf("read search response: %v", err)
		}
		_, opTag, opData, controls, err := decodeLDAPPacketWithControls(resp)
		if err != nil {
			t.Fatalf("decode search response: %v", err)
		}
		switch opTag {
		case opSearchResultEntry:
			dn, _, err := decodeOctetString(opData)
			if err != nil {
				t.Fatalf("decode search result DN: %v", err)
			}
			dns = append(dns, dn)
		case opSearchResultDone:
			return dns, controls
		case opSearchResultReference:
			continue
		default:
			t.Fatalf("unexpected search response opTag %d", opTag)
		}
	}
}

func pagedResponseCookie(t *testing.T, controls []control) string {
	t.Helper()
	for _, ctrl := range controls {
		if ctrl.Type != controlPagedResults {
			continue
		}
		if len(ctrl.Value) == 0 || ctrl.Value[0] != tagSequence {
			t.Fatalf("paged response control value is not a sequence: %x", ctrl.Value)
		}
		content, err := decodeContent(ctrl.Value[1:])
		if err != nil {
			t.Fatalf("decode paged response control: %v", err)
		}
		_, rest, err := decodeInt(content)
		if err != nil {
			t.Fatalf("decode paged response size: %v", err)
		}
		cookie, rest, err := decodeOctetString(rest)
		if err != nil {
			t.Fatalf("decode paged response cookie: %v", err)
		}
		if len(rest) != 0 {
			t.Fatalf("paged response control has trailing data: %x", rest)
		}
		return cookie
	}
	t.Fatalf("missing paged results response control: %+v", controls)
	return ""
}

func hasServerSideSortResponseControl(controls []control) bool {
	for _, ctrl := range controls {
		if ctrl.Type != controlServerSideSortResponse {
			continue
		}
		if len(ctrl.Value) == 0 || ctrl.Value[0] != tagSequence {
			return false
		}
		content, err := decodeContent(ctrl.Value[1:])
		if err != nil {
			return false
		}
		return len(content) == 3 && decodeEnumerated(content) == resultSuccess
	}
	return false
}

func virtualListViewResponse(t *testing.T, controls []control) (targetPosition int, contentCount int) {
	t.Helper()
	for _, ctrl := range controls {
		if ctrl.Type != controlVirtualListViewResponse {
			continue
		}
		if len(ctrl.Value) == 0 || ctrl.Value[0] != tagSequence {
			t.Fatalf("VLV response control value is not a sequence: %x", ctrl.Value)
		}
		content, err := decodeContent(ctrl.Value[1:])
		if err != nil {
			t.Fatalf("decode VLV response: %v", err)
		}
		targetPosition, rest, err := decodeInt(content)
		if err != nil {
			t.Fatalf("decode VLV target position: %v", err)
		}
		contentCount, rest, err = decodeInt(rest)
		if err != nil {
			t.Fatalf("decode VLV content count: %v", err)
		}
		if got := decodeEnumerated(rest); got != resultSuccess {
			t.Fatalf("VLV result = %d, want %d", got, resultSuccess)
		}
		return targetPosition, contentCount
	}
	t.Fatalf("missing VLV response control: %+v", controls)
	return 0, 0
}

func hasSyncDoneControl(controls []control) bool {
	for _, ctrl := range controls {
		if ctrl.Type == controlSyncDone {
			return len(ctrl.Value) > 0 && ctrl.Value[0] == tagSequence
		}
	}
	return false
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

func TestLDAPServerBindAcceptsUserDNIdentity(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	srv := NewServer(ln, auth, newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	bindReq := buildLDAPPacket(25, opBindRequest, buildBindRequest(3, "uid=alice,ou=users,dc=example,dc=com", "secret"))
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
	if opTag != opBindResponse || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("DN bind response op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opBindResponse, resultSuccess)
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

func TestLDAPServerRejectsUnauthenticatedDirectorySearch(t *testing.T) {
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

	searchReq := buildLDAPPacket(26, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultInsufficientAccessRights {
		t.Fatalf("unauthenticated search op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultInsufficientAccessRights)
	}
}

func TestLDAPServerCompareRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	trueReq := buildLDAPPacket(27, opCompareRequest, buildCompareRequest("uid=alice,ou=users,dc=example,dc=com", "mail", "alice@example.com"))
	if err := sendPDU(conn, trueReq); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode compare response: %v", err)
	}
	if opTag != opCompareResponse || decodeEnumerated(opData) != resultCompareTrue {
		t.Fatalf("compare true op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opCompareResponse, resultCompareTrue)
	}

	falseReq := buildLDAPPacket(28, opCompareRequest, buildCompareRequest("uid=alice,ou=users,dc=example,dc=com", "mail", "bob@example.com"))
	if err := sendPDU(conn, falseReq); err != nil {
		t.Fatal(err)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err = decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode compare false response: %v", err)
	}
	if opTag != opCompareResponse || decodeEnumerated(opData) != resultCompareFalse {
		t.Fatalf("compare false op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opCompareResponse, resultCompareFalse)
	}
}

func TestBindIdentityCandidatesUnescapesDNValues(t *testing.T) {
	got := bindIdentityCandidates(`uid=alice\2eops,ou=users,dc=example,dc=com`)
	want := []string{`uid=alice\2eops,ou=users,dc=example,dc=com`, "alice.ops"}
	if len(got) != len(want) {
		t.Fatalf("bindIdentityCandidates = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("bindIdentityCandidates = %#v, want %#v", got, want)
		}
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

	cases := []struct {
		name       string
		msgID      int
		requestOp  int
		responseOp int
		payload    []byte
	}{
		{name: "modify", msgID: 3, requestOp: opModifyRequest, responseOp: opModifyResponse, payload: []byte{0x01}},
		{name: "add", msgID: 4, requestOp: opAddRequest, responseOp: opAddResponse, payload: []byte{0x01}},
		{name: "delete", msgID: 5, requestOp: opDeleteRequest, responseOp: opDeleteResponse, payload: encodeOctetString("uid=alice,dc=example,dc=com")},
		{name: "modify dn", msgID: 6, requestOp: opModDNRequest, responseOp: opModDNResponse, payload: []byte{0x01}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := buildLDAPPacket(tc.msgID, tc.requestOp, tc.payload)
			if err := sendPDU(conn, req); err != nil {
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
			if opTag != tc.responseOp {
				t.Fatalf("opTag = %d, want %d", opTag, tc.responseOp)
			}
			if got := decodeEnumerated(opData); got != resultUnwillingToPerform {
				t.Fatalf("result = %d, want %d", got, resultUnwillingToPerform)
			}
		})
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

func TestLDAPServerAbandonRequestHasNoResponse(t *testing.T) {
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

	abandonReq := buildLDAPPacket(30, opAbandonRequest, encodeInt(5))
	if err := sendPDU(conn, abandonReq); err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(150 * time.Millisecond))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("AbandonRequest produced a response, want no response")
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
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		CN:          "alice",
		Mail:        "alice@example.com",
		UID:         "alice",
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
	bindTestConnection(t, conn, auth)

	filterData := buildEqualityFilter("mail", "alice@example.com")

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

func TestDecodeSearchRequestRejectsInvalidEnums(t *testing.T) {
	if _, _, _, _, _, _, _, err := decodeSearchRequest(buildSearchRequestWithParams("dc=example,dc=com", 9, derefAliasesNever, buildEqualityFilter("objectClass", "person"))); err == nil {
		t.Fatal("decodeSearchRequest accepted invalid scope")
	}
	if _, _, _, _, _, _, _, err := decodeSearchRequest(buildSearchRequestWithParams("dc=example,dc=com", scopeWholeSubtree, 9, buildEqualityFilter("objectClass", "person"))); err == nil {
		t.Fatal("decodeSearchRequest accepted invalid derefAliases")
	}
	if _, scope, _, _, _, _, _, err := decodeSearchRequest(buildSearchRequestWithParams("dc=example,dc=com", scopeSingleLevel, derefAliasesAlways, buildEqualityFilter("objectClass", "person"))); err != nil || scope != scopeSingleLevel {
		t.Fatalf("decodeSearchRequest valid enums scope=%d err=%v", scope, err)
	}
}

func TestLDAPServerIgnoresSupportedCriticalControlAndRecordsMetrics(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	metrics := &fakeLDAPMetrics{}
	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		CN:          "alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice User",
	})
	srv := NewServerWithOptions(ln, auth, dir, ServerOptions{Metrics: metrics})
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	searchReq := buildLDAPPacketWithControls(10, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("mail", "alice@example.com")),
		[]control{{Type: controlManageDsaIT, Critical: true}},
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	events := metrics.snapshot()
	last := events[len(events)-1]
	if len(events) != 2 || last.Operation != "search" || last.Result != MetricAccepted || last.Entries != 1 {
		t.Fatalf("metrics = %+v, want accepted search with one entry", events)
	}
}

func TestLDAPServerRejectsUnsupportedCriticalControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	metrics := &fakeLDAPMetrics{}
	srv := NewServerWithOptions(ln, newFakeLDAPAuth(), newFakeDirectoryQuerier(), ServerOptions{Metrics: metrics})
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	searchReq := buildLDAPPacketWithControls(11, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("mail", "alice@example.com")),
		[]control{{Type: "1.2.3.4.5", Critical: true}},
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultDone {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultDone)
	}
	if got := decodeEnumerated(opData); got != resultUnavailableCriticalExtension {
		t.Fatalf("result = %d, want %d", got, resultUnavailableCriticalExtension)
	}
	events := metrics.snapshot()
	if len(events) != 1 || events[0].Result != MetricRejected || events[0].ResultCode != resultUnavailableCriticalExtension {
		t.Fatalf("metrics = %+v, want rejected critical-control event", events)
	}
}

func TestLDAPServerAssertionControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	okReq := buildLDAPPacketWithControls(12, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{buildAssertionControl(buildEqualityFilter("objectClass", "person"), true)},
	)
	if err := sendPDU(conn, okReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 1 {
		t.Fatalf("assertion accepted entries = %d, want 1", entries)
	}

	failReq := buildLDAPPacketWithControls(13, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{buildAssertionControl(buildEqualityFilter("objectClass", "organizationalUnit"), true)},
	)
	if err := sendPDU(conn, failReq); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode assertion failed response: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultAssertionFailed {
		t.Fatalf("assertion failed op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultAssertionFailed)
	}
}

func TestLDAPServerMatchedValuesControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(14, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "objectClass"),
		[]control{buildMatchedValuesControl(buildEqualityFilter("objectClass", "inetOrgPerson"), true)},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode matched-values entry: %v", err)
	}
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want SearchResultEntry", opTag)
	}
	if !bytesContains(opData, []byte("inetOrgPerson")) {
		t.Fatalf("matched-values entry missing inetOrgPerson: %x", opData)
	}
	for _, unexpected := range [][]byte{[]byte("person"), []byte("organizationalPerson")} {
		if bytesContains(opData, unexpected) {
			t.Fatalf("matched-values entry contained unrequested value %q: %x", unexpected, opData)
		}
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err = decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode matched-values done: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("matched-values done op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultSuccess)
	}
}

func TestLDAPServerAdditionalSearchControls(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	normalReq := buildLDAPPacketWithControls(15, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{
			{Type: controlDomainScope, Critical: true},
			{Type: controlDontUseCopy, Critical: true},
			buildSubentriesControl(false, true),
		},
	)
	if err := sendPDU(conn, normalReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 1 {
		t.Fatalf("domainScope/dontUseCopy/subentries=false entries = %d, want 1", entries)
	}

	subentriesReq := buildLDAPPacketWithControls(16, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{buildSubentriesControl(true, true)},
	)
	if err := sendPDU(conn, subentriesReq); err != nil {
		t.Fatal(err)
	}
	entries, _ = readSearchUntilDone(t, conn)
	if entries != 0 {
		t.Fatalf("subentries=true entries = %d, want 0", entries)
	}
}

func TestLDAPServerAdditionalGeneralControls(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(21, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{
			{Type: controlRelax, Critical: true},
			{Type: controlNoOp, Critical: true},
			{Type: controlPreRead, Critical: true},
			{Type: controlPostRead, Critical: true},
			{Type: controlPasswordPolicy, Critical: true},
			{Type: controlSessionTracking, Critical: true},
		},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 1 {
		t.Fatalf("additional general controls entries = %d, want 1", entries)
	}
}

func TestLDAPServerSyncRefreshOnlyControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(17, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{buildSyncRequestControl(syncModeRefreshOnly, "", true)},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, _, controls, err := decodeLDAPPacketWithControls(resp)
	if err != nil {
		t.Fatalf("decode sync entry: %v", err)
	}
	if opTag != opSearchResultEntry {
		t.Fatalf("sync entry opTag = %d, want SearchResultEntry", opTag)
	}
	foundState := false
	for _, ctrl := range controls {
		if ctrl.Type == controlSyncState {
			foundState = true
		}
	}
	if !foundState {
		t.Fatalf("sync entry missing Sync State control: %+v", controls)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, controls, err := decodeLDAPPacketWithControls(resp)
	if err != nil {
		t.Fatalf("decode sync done: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("sync done op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultSuccess)
	}
	if !hasSyncDoneControl(controls) {
		t.Fatalf("sync done missing Sync Done control: %+v", controls)
	}
}

func TestLDAPServerDereferenceControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "cn=team,ou=groups,dc=example,dc=com", Kind: "group", CN: "Team", UID: "team", DisplayName: "Team"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(29, opSearchRequest,
		buildSearchRequestWithAttrs("ou=groups,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "groupOfNames"), "cn"),
		[]control{buildDereferenceControl("member", "cn", "mail")},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, _, controls, err := decodeLDAPPacketWithControls(resp)
	if err != nil {
		t.Fatalf("decode deref entry: %v", err)
	}
	if opTag != opSearchResultEntry {
		t.Fatalf("deref entry opTag = %d, want SearchResultEntry", opTag)
	}
	if len(controls) != 0 {
		t.Fatalf("deref read-only no-op returned unexpected entry controls: %+v", controls)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, _, err := decodeLDAPPacketWithControls(resp)
	if err != nil {
		t.Fatalf("decode deref done: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("deref done op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultSuccess)
	}
}

func TestLDAPServerProxiedAuthorizationControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	auth := newFakeLDAPAuth()
	auth.addUser("uid=alice,ou=users,dc=example,dc=com", "secret")
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	bindReq := buildLDAPPacket(18, opBindRequest, buildBindRequest(3, "uid=alice,ou=users,dc=example,dc=com", "secret"))
	if err := sendPDU(conn, bindReq); err != nil {
		t.Fatal(err)
	}
	if _, err := readFullPDU(conn, time.Now().Add(3*time.Second)); err != nil {
		t.Fatal(err)
	}

	okReq := buildLDAPPacketWithControls(19, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{buildProxiedAuthorizationControl("dn:uid=alice,ou=users,dc=example,dc=com", true)},
	)
	if err := sendPDU(conn, okReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 1 {
		t.Fatalf("matching proxied authz entries = %d, want 1", entries)
	}

	deniedReq := buildLDAPPacketWithControls(20, opSearchRequest,
		buildSearchRequestWithAttrs("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn"),
		[]control{buildProxiedAuthorizationControl("dn:uid=bob,ou=users,dc=example,dc=com", true)},
	)
	if err := sendPDU(conn, deniedReq); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode proxied authz denied response: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultAuthorizationDenied {
		t.Fatalf("proxied authz denied op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opSearchResultDone, resultAuthorizationDenied)
	}
}

func TestLDAPServerSimplePagedResultsControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	for i := 1; i <= 3; i++ {
		uid := fmt.Sprintf("user%d", i)
		dir.addPrincipal(PrincipalEntry{
			DN:          fmt.Sprintf("uid=%s,ou=users,dc=example,dc=com", uid),
			CN:          uid,
			Mail:        uid + "@example.com",
			UID:         uid,
			DisplayName: "User " + fmt.Sprint(i),
		})
	}
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	firstPageReq := buildLDAPPacketWithControls(20, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{buildPagedResultsControl(2, "")},
	)
	if err := sendPDU(conn, firstPageReq); err != nil {
		t.Fatal(err)
	}
	entries, controls := readSearchUntilDone(t, conn)
	if entries != 2 {
		t.Fatalf("first page entries = %d, want 2", entries)
	}
	cookie := pagedResponseCookie(t, controls)
	if cookie != "2" {
		t.Fatalf("first page cookie = %q, want %q", cookie, "2")
	}

	secondPageReq := buildLDAPPacketWithControls(21, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{buildPagedResultsControl(2, cookie)},
	)
	if err := sendPDU(conn, secondPageReq); err != nil {
		t.Fatal(err)
	}
	entries, controls = readSearchUntilDone(t, conn)
	if entries != 1 {
		t.Fatalf("second page entries = %d, want 1", entries)
	}
	if cookie := pagedResponseCookie(t, controls); cookie != "" {
		t.Fatalf("second page cookie = %q, want empty", cookie)
	}
}

func TestLDAPServerServerSideSortControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	for _, p := range []PrincipalEntry{
		{DN: "uid=charlie,ou=users,dc=example,dc=com", Kind: "user", CN: "Charlie", UID: "charlie", DisplayName: "Charlie"},
		{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"},
		{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", UID: "bob", DisplayName: "Bob"},
	} {
		dir.addPrincipal(p)
	}
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(31, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{buildServerSideSortControl("cn", false)},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, controls := readSearchDNsUntilDone(t, conn)
	want := []string{
		"uid=alice,ou=users,dc=example,dc=com",
		"uid=bob,ou=users,dc=example,dc=com",
		"uid=charlie,ou=users,dc=example,dc=com",
	}
	if strings.Join(dns, "|") != strings.Join(want, "|") {
		t.Fatalf("sorted DNs = %#v, want %#v", dns, want)
	}
	if !hasServerSideSortResponseControl(controls) {
		t.Fatalf("missing successful server-side sort response control: %+v", controls)
	}
}

func TestLDAPServerVirtualListViewControl(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	for _, p := range []PrincipalEntry{
		{DN: "uid=charlie,ou=users,dc=example,dc=com", Kind: "user", CN: "Charlie", UID: "charlie", DisplayName: "Charlie"},
		{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"},
		{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", UID: "bob", DisplayName: "Bob"},
		{DN: "uid=dana,ou=users,dc=example,dc=com", Kind: "user", CN: "Dana", UID: "dana", DisplayName: "Dana"},
	} {
		dir.addPrincipal(p)
	}
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacketWithControls(32, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
		[]control{
			buildServerSideSortControl("cn", false),
			buildVirtualListViewControl(0, 1, 2, 0),
		},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, controls := readSearchDNsUntilDone(t, conn)
	want := []string{
		"uid=bob,ou=users,dc=example,dc=com",
		"uid=charlie,ou=users,dc=example,dc=com",
	}
	if strings.Join(dns, "|") != strings.Join(want, "|") {
		t.Fatalf("VLV DNs = %#v, want %#v", dns, want)
	}
	target, count := virtualListViewResponse(t, controls)
	if target != 2 || count != 4 {
		t.Fatalf("VLV response target/count = %d/%d, want 2/4", target, count)
	}
}

func TestLDAPServerOrganizationalUnitSearchReturnsOrganizationEntries(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=user-1,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "user-1",
		DisplayName: "Alice",
	})
	dir.addPrincipal(PrincipalEntry{
		DN:          "ou=org-1,ou=organizations,dc=example,dc=com",
		Kind:        "organization",
		CN:          "Research",
		UID:         "org-1",
		OU:          "Research",
		DisplayName: "Research",
	})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	searchReq := buildLDAPPacket(22, opSearchRequest,
		buildSearchRequestWithAttrs("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "organizationalUnit"), "objectClass", "ou", "mail"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if !bytesContains(opData, []byte("ou=org-1,ou=organizations,dc=example,dc=com")) {
		t.Fatalf("organization DN missing from LDAP entry: %x", opData)
	}
	if !bytesContains(opData, []byte("organizationalUnit")) || !bytesContains(opData, []byte("Research")) {
		t.Fatalf("organization LDAP attrs missing objectClass/ou: %x", opData)
	}
	if bytesContains(opData, []byte("alice@example.com")) {
		t.Fatalf("organizationalUnit search returned user mail attribute: %x", opData)
	}
}

func TestLDAPServerOrganizationBaseDNRestrictsSubtreeSearch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=user-1,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", Mail: "alice@example.com", UID: "user-1", DisplayName: "Alice"})
	dir.addPrincipal(PrincipalEntry{DN: "ou=org-1,ou=organizations,dc=example,dc=com", Kind: "organization", CN: "Research", UID: "org-1", OU: "Research", DisplayName: "Research"})
	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := []byte{tagContextSpecific | filterPresent}
	filter = append(filter, encodeLength(len("objectClass"))...)
	filter = append(filter, []byte("objectClass")...)
	searchReq := buildLDAPPacket(23, opSearchRequest,
		buildSearchRequestWithAttrs("ou=organizations,dc=example,dc=com", scopeWholeSubtree, filter, "objectClass", "ou", "mail"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if !bytesContains(opData, []byte("ou=org-1,ou=organizations,dc=example,dc=com")) {
		t.Fatalf("organization subtree did not return organization entry: %x", opData)
	}
	if bytesContains(opData, []byte("alice@example.com")) {
		t.Fatalf("organization subtree returned user entry: %x", opData)
	}
}

func TestLDAPServerContainerBaseObjectSearch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := []byte{tagContextSpecific | filterPresent}
	filter = append(filter, encodeLength(len("objectClass"))...)
	filter = append(filter, []byte("objectClass")...)
	searchReq := buildLDAPPacket(24, opSearchRequest,
		buildSearchRequestWithAttrs("ou=organizations,dc=example,dc=com", scopeBaseObject, filter, "objectClass", "ou", "displayName"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if !bytesContains(opData, []byte("ou=organizations,dc=example,dc=com")) || !bytesContains(opData, []byte("Organizations")) {
		t.Fatalf("container base-object response missing organization container data: %x", opData)
	}
}

func TestLDAPServerOpenLDAPSearchCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "ou=org-1,ou=organizations,dc=example,dc=com",
		Kind:        "organization",
		CN:          "Research",
		UID:         "org-1",
		OU:          "Research",
		DisplayName: "Research",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=organizations,dc=example,dc=com",
		"(objectClass=organizationalUnit)",
		"objectClass",
		"ou",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: ou=org-1,ou=organizations,dc=example,dc=com") ||
		!strings.Contains(output, "objectClass: organizationalUnit") ||
		!strings.Contains(output, "ou: Research") {
		t.Fatalf("ldapsearch output missing organization entry:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPAssertionControlCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-e", "!assert=(objectClass=person)",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch assertion control failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "cn: Alice") {
		t.Fatalf("ldapsearch assertion output missing user entry:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPMatchedValuesCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "!mv=(objectClass=inetOrgPerson)",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"objectClass",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch matched-values control failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "objectClass: inetOrgPerson") {
		t.Fatalf("ldapsearch matched-values output missing inetOrgPerson:\n%s", output)
	}
	for _, unexpected := range []string{"objectClass: person", "objectClass: organizationalPerson"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("ldapsearch matched-values output included %q:\n%s", unexpected, output)
		}
	}
}

func TestLDAPServerOpenLDAPSyncRefreshOnlyCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "!sync=ro",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch sync refreshOnly failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "control: 1.3.6.1.4.1.4203.1.9.1.2") ||
		!strings.Contains(output, "control: 1.3.6.1.4.1.4203.1.9.1.3") {
		t.Fatalf("ldapsearch sync output missing entry/state/done:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPProxiedAuthorizationCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	bindDN := "uid=alice,ou=users,dc=example,dc=com"
	auth := newFakeLDAPAuth()
	auth.addUser(bindDN, "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: bindDN, Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", bindDN,
		"-w", "secret",
		"-e", "!authzid=dn:"+bindDN,
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch proxied authorization failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "cn: Alice") {
		t.Fatalf("ldapsearch proxied authorization output missing user:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPAdditionalGeneralControlsCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	bindDN := "uid=alice,ou=users,dc=example,dc=com"
	auth := newFakeLDAPAuth()
	auth.addUser(bindDN, "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: bindDN, Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", bindDN,
		"-w", "secret",
		"-e", "!relax",
		"-e", "ppolicy",
		"-e", "sessiontracking",
		"-e", "!preread=cn",
		"-e", "!postread=cn",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch additional general controls failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "cn: Alice") {
		t.Fatalf("ldapsearch additional general controls output missing user:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPDereferenceCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	bindDN := "uid=alice,ou=users,dc=example,dc=com"
	auth := newFakeLDAPAuth()
	auth.addUser(bindDN, "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "cn=team,ou=groups,dc=example,dc=com", Kind: "group", CN: "Team", UID: "team", DisplayName: "Team"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", bindDN,
		"-w", "secret",
		"-E", "!deref=member:cn,mail",
		"-b", "ou=groups,dc=example,dc=com",
		"(objectClass=groupOfNames)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch dereference control failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: cn=team,ou=groups,dc=example,dc=com") ||
		!strings.Contains(output, "cn: Team") {
		t.Fatalf("ldapsearch dereference output missing group:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPAdditionalSearchControlsCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "!domainScope",
		"-E", "!dontUseCopy",
		"-E", "!subentries=false",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch additional controls failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapsearch additional controls output missing user:\n%s", output)
	}

	subentriesCmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "!subentries=true",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err = subentriesCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch subentries=true failed: %v\n%s", err, out)
	}
	output = string(out)
	if strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapsearch subentries=true returned normal entry:\n%s", output)
	}
	if !strings.Contains(output, "result: 0 Success") || strings.Contains(output, "# numEntries:") {
		t.Fatalf("ldapsearch subentries=true output did not look like empty success:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPExtensibleMatchCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "ou=org-1,ou=organizations,dc=example,dc=com",
		Kind:        "organization",
		CN:          "Research",
		UID:         "org-1",
		OU:          "Research",
		DisplayName: "Research",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=organizations,dc=example,dc=com",
		"(&(objectClass:caseIgnoreMatch:=organizationalUnit)(ou:caseIgnoreMatch:=Research))",
		"ou",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch extensibleMatch failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: ou=org-1,ou=organizations,dc=example,dc=com") ||
		!strings.Contains(output, "ou: Research") {
		t.Fatalf("ldapsearch extensibleMatch output missing organization entry:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPServerSideSortCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	for _, p := range []PrincipalEntry{
		{DN: "uid=charlie,ou=users,dc=example,dc=com", Kind: "user", CN: "Charlie", UID: "charlie", DisplayName: "Charlie"},
		{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"},
		{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", UID: "bob", DisplayName: "Bob"},
	} {
		dir.addPrincipal(p)
	}
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "sss=cn",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch server-side sort failed: %v\n%s", err, out)
	}
	output := string(out)
	alice := strings.Index(output, "dn: uid=alice,ou=users,dc=example,dc=com")
	bob := strings.Index(output, "dn: uid=bob,ou=users,dc=example,dc=com")
	charlie := strings.Index(output, "dn: uid=charlie,ou=users,dc=example,dc=com")
	if alice < 0 || bob < 0 || charlie < 0 || !(alice < bob && bob < charlie) {
		t.Fatalf("ldapsearch server-side sort output not sorted:\n%s", output)
	}
	if !strings.Contains(output, "control: 1.2.840.113556.1.4.474") || !strings.Contains(output, "sortResult: (0) Success") {
		t.Fatalf("ldapsearch output missing sort response control:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPVirtualListViewCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	for _, p := range []PrincipalEntry{
		{DN: "uid=charlie,ou=users,dc=example,dc=com", Kind: "user", CN: "Charlie", UID: "charlie", DisplayName: "Charlie"},
		{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice", DisplayName: "Alice"},
		{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", UID: "bob", DisplayName: "Bob"},
		{DN: "uid=dana,ou=users,dc=example,dc=com", Kind: "user", CN: "Dana", UID: "dana", DisplayName: "Dana"},
	} {
		dir.addPrincipal(p)
	}
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-E", "sss=cn",
		"-E", "vlv=0/1/2/0",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"cn",
	)
	cmd.Stdin = strings.NewReader("q\n")
	out, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		t.Fatalf("ldapsearch VLV timed out\n%s", out)
	}
	output := string(out)
	if err != nil && !strings.Contains(output, "vlvResult: pos=2 count=4") {
		t.Fatalf("ldapsearch VLV failed: %v\n%s", err, out)
	}
	bob := strings.Index(output, "dn: uid=bob,ou=users,dc=example,dc=com")
	charlie := strings.Index(output, "dn: uid=charlie,ou=users,dc=example,dc=com")
	if bob < 0 || charlie < 0 || bob > charlie ||
		strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		strings.Contains(output, "dn: uid=dana,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapsearch VLV output did not return the requested sorted window:\n%s", output)
	}
	if !strings.Contains(output, "control: 2.16.840.1.113730.3.4.10") {
		t.Fatalf("ldapsearch output missing VLV response control:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPAttributeSelectionCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(mail=alice@example.com)",
		"mail",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch attribute selection failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "mail: alice@example.com") {
		t.Fatalf("ldapsearch output missing requested mail attribute:\n%s", output)
	}
	for _, unexpected := range []string{"cn: Alice", "uid: alice", "objectClass: inetOrgPerson", "displayName: Alice"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("ldapsearch output included unrequested attribute %q:\n%s", unexpected, output)
		}
	}
}

func TestLDAPServerOpenLDAPNoAttributesCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(mail=alice@example.com)",
		"1.1",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch no-attributes selection failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapsearch output missing entry DN:\n%s", output)
	}
	for _, unexpected := range []string{"mail: alice@example.com", "cn: Alice", "uid: alice", "objectClass:"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("ldapsearch output included attribute despite 1.1 request %q:\n%s", unexpected, output)
		}
	}
}

func TestLDAPServerOpenLDAPOperationalAttributesCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(mail=alice@example.com)",
		"+",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch operational attrs failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, want := range []string{
		"entryDN: uid=alice,ou=users,dc=example,dc=com",
		"entryUUID:",
		"createTimestamp: 19700101000000Z",
		"modifyTimestamp: 19700101000000Z",
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("ldapsearch operational attrs output missing %q:\n%s", want, output)
		}
	}
	for _, unexpected := range []string{"mail: alice@example.com", "cn: Alice", "objectClass:"} {
		if strings.Contains(output, unexpected) {
			t.Fatalf("ldapsearch operational attrs output included user attr %q:\n%s", unexpected, output)
		}
	}
}

func TestLDAPServerOpenLDAPObjectClassRequiredAttributesCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice User",
	})
	dir.addPrincipal(PrincipalEntry{
		DN:          "cn=team,ou=groups,dc=example,dc=com",
		Kind:        "group",
		CN:          "Team",
		UID:         "team",
		DisplayName: "Team",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	userCmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(objectClass=person)",
		"objectClass",
		"cn",
		"sn",
	)
	userOut, err := userCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch user required attrs failed: %v\n%s", err, userOut)
	}
	if output := string(userOut); !strings.Contains(output, "objectClass: person") || !strings.Contains(output, "sn: Alice User") {
		t.Fatalf("ldapsearch user output missing person/sn:\n%s", output)
	}

	groupCmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=groups,dc=example,dc=com",
		"(objectClass=groupOfNames)",
		"objectClass",
		"cn",
		"member",
	)
	groupOut, err := groupCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch group required attrs failed: %v\n%s", err, groupOut)
	}
	if output := string(groupOut); !strings.Contains(output, "objectClass: groupOfNames") ||
		!strings.Contains(output, "member: cn=team,ou=groups,dc=example,dc=com") {
		t.Fatalf("ldapsearch group output missing groupOfNames/member:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPCompareCompatibility(t *testing.T) {
	ldapcompare, err := exec.LookPath("ldapcompare")
	if err != nil {
		t.Skip("ldapcompare is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapcompare,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"uid=alice,ou=users,dc=example,dc=com",
		"mail:alice@example.com",
	)
	out, err := cmd.CombinedOutput()
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == resultCompareTrue {
		err = nil
	}
	if err != nil {
		t.Fatalf("ldapcompare failed: %v\n%s", err, out)
	}
	if output := string(out); !strings.Contains(output, "TRUE") {
		t.Fatalf("ldapcompare output = %q, want TRUE", output)
	}
}

func TestLDAPServerWhoAmIExtendedRequest(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacket(29, opExtendedRequest, buildExtendedRequest(whoAmIOID))
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode WhoAmI response: %v", err)
	}
	if opTag != opExtendedResponse || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("WhoAmI op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opExtendedResponse, resultSuccess)
	}
	if !bytesContains(opData, []byte("dn:tester")) {
		t.Fatalf("WhoAmI response missing authzID: %x", opData)
	}
}

func TestLDAPServerOpenLDAPWhoAmICompatibility(t *testing.T) {
	ldapwhoami, err := exec.LookPath("ldapwhoami")
	if err != nil {
		t.Skip("ldapwhoami is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	srv := NewServer(ln, auth, newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapwhoami,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapwhoami failed: %v\n%s", err, out)
	}
	if output := string(out); !strings.Contains(output, "dn:uid=alice,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapwhoami output = %q, want bind DN authzID", output)
	}
}

func TestLDAPServerOpenLDAPStartTLSCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServerWithOptions(ln, auth, dir, ServerOptions{TLSConfig: testLDAPTLSConfig(t)})
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-ZZ",
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(mail=alice@example.com)",
		"mail",
		"cn",
	)
	cmd.Env = append(os.Environ(), "LDAPTLS_REQCERT=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch StartTLS failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "mail: alice@example.com") ||
		!strings.Contains(output, "cn: Alice") {
		t.Fatalf("ldapsearch StartTLS output missing user entry:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPLDAPSCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	rawLn, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer rawLn.Close()
	ln := tls.NewListener(rawLn, testLDAPTLSConfig(t))

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice",
	})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldaps://"+rawLn.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"(mail=alice@example.com)",
		"mail",
		"cn",
	)
	cmd.Env = append(os.Environ(), "LDAPTLS_REQCERT=never")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch LDAPS failed: %v\n%s", err, out)
	}
	output := string(out)
	if !strings.Contains(output, "dn: uid=alice,ou=users,dc=example,dc=com") ||
		!strings.Contains(output, "mail: alice@example.com") ||
		!strings.Contains(output, "cn: Alice") {
		t.Fatalf("ldapsearch LDAPS output missing user entry:\n%s", output)
	}
}

func TestLDAPServerOpenLDAPPagedResultsCompatibility(t *testing.T) {
	ldapsearch, err := exec.LookPath("ldapsearch")
	if err != nil {
		t.Skip("ldapsearch is not installed")
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("alice", "secret")
	dir := newFakeDirectoryQuerier()
	for i := 1; i <= 3; i++ {
		uid := fmt.Sprintf("user%d", i)
		dir.addPrincipal(PrincipalEntry{
			DN:          fmt.Sprintf("uid=%s,ou=users,dc=example,dc=com", uid),
			Kind:        "user",
			CN:          "User " + fmt.Sprint(i),
			Mail:        uid + "@example.com",
			UID:         uid,
			DisplayName: "User " + fmt.Sprint(i),
		})
	}
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	cmd := exec.Command(ldapsearch,
		"-x",
		"-H", "ldap://"+ln.Addr().String(),
		"-D", "uid=alice,ou=users,dc=example,dc=com",
		"-w", "secret",
		"-b", "ou=users,dc=example,dc=com",
		"-E", "pr=1/noprompt",
		"(objectClass=person)",
		"mail",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch paged results failed: %v\n%s", err, out)
	}
	output := string(out)
	for i := 1; i <= 3; i++ {
		wantDN := fmt.Sprintf("dn: uid=user%d,ou=users,dc=example,dc=com", i)
		wantMail := fmt.Sprintf("mail: user%d@example.com", i)
		if !strings.Contains(output, wantDN) || !strings.Contains(output, wantMail) {
			t.Fatalf("ldapsearch paged output missing %q/%q:\n%s", wantDN, wantMail, output)
		}
	}
	if got := strings.Count(output, "pagedresults: cookie="); got < 2 {
		t.Fatalf("ldapsearch output did not show multiple paged-results exchanges:\n%s", output)
	}
}

func TestLDAPServerRootDSEAdvertisesNamingContextAndStartTLS(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	srv := NewServerWithOptions(ln, newFakeLDAPAuth(), newFakeDirectoryQuerier(), ServerOptions{
		TLSConfig:      testLDAPTLSConfig(t),
		NamingContexts: []string{"dc=example,dc=com"},
	})
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	filter := []byte{tagContextSpecific | filterPresent}
	filter = append(filter, encodeLength(len("objectClass"))...)
	filter = append(filter, []byte("objectClass")...)
	searchReq := buildLDAPPacket(6, opSearchRequest, buildSearchRequestWithAttrs("", scopeBaseObject, filter, "namingContexts", "supportedExtension", "subschemaSubentry"))
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if !bytesContains(opData, []byte("dc=example,dc=com")) {
		t.Fatalf("root DSE response did not include naming context: %x", opData)
	}
	if !bytesContains(opData, []byte(startTLSOID)) {
		t.Fatalf("root DSE response did not include StartTLS OID: %x", opData)
	}
	if !bytesContains(opData, []byte("cn=Subschema")) {
		t.Fatalf("root DSE response did not include subschemaSubentry: %x", opData)
	}
}

func TestLDAPServerReturnsSubschemaDiscovery(t *testing.T) {
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

	filter := []byte{tagContextSpecific | filterPresent}
	filter = append(filter, encodeLength(len("objectClass"))...)
	filter = append(filter, []byte("objectClass")...)
	searchReq := buildLDAPPacket(12, opSearchRequest, buildSearchRequestWithAttrs("cn=Subschema", scopeBaseObject, filter, "objectClasses", "attributeTypes"))
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if !bytesContains(opData, []byte("inetOrgPerson")) || !bytesContains(opData, []byte("displayName")) {
		t.Fatalf("subschema response missing expected directory schema: %x", opData)
	}
}

func TestLDAPServerStartTLSAllowsSearchAfterUpgrade(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		CN:          "alice",
		Mail:        "alice@example.com",
		UID:         "alice",
		DisplayName: "Alice User",
	})
	auth := newFakeLDAPAuth()
	srv := NewServerWithOptions(ln, auth, dir, ServerOptions{TLSConfig: testLDAPTLSConfig(t)})
	go srv.Serve()
	defer srv.Close()

	rawConn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer rawConn.Close()

	startTLSReq := buildLDAPPacket(7, opExtendedRequest, buildExtendedRequest(startTLSOID))
	if err := sendPDU(rawConn, startTLSReq); err != nil {
		t.Fatal(err)
	}
	resp, err := readFullPDU(rawConn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if opTag != opExtendedResponse || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("StartTLS response op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opExtendedResponse, resultSuccess)
	}

	conn := tls.Client(rawConn, &tls.Config{InsecureSkipVerify: true, MinVersion: tls.VersionTLS12})
	if err := conn.Handshake(); err != nil {
		t.Fatalf("TLS handshake: %v", err)
	}
	bindTestConnection(t, conn, auth)

	filterData := buildEqualityFilter("mail", "alice@example.com")
	searchReq := buildLDAPPacket(8, opSearchRequest, buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, filterData))
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, _, err = decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag after StartTLS = %d, want %d", opTag, opSearchResultEntry)
	}
}

func TestLDAPServerReturnsSearchReferenceForForeignNamingContext(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	srv := NewServerWithOptions(ln, auth, newFakeDirectoryQuerier(), ServerOptions{
		NamingContexts: []string{"dc=example,dc=com"},
		ReferralURLs:   []string{"ldap://directory.example.net/dc=example,dc=net"},
	})
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	searchReq := buildLDAPPacket(9, opSearchRequest, buildSearchRequest("dc=example,dc=net", scopeWholeSubtree, buildEqualityFilter("mail", "alice@example.net")))
	if err := sendPDU(conn, searchReq); err != nil {
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
	if opTag != opSearchResultReference {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultReference)
	}
	if !bytesContains(opData, []byte("ldap://directory.example.net/dc=example,dc=net")) {
		t.Fatalf("referral response did not include URL: %x", opData)
	}
}

func TestParseLDAPFilterSupportsClientOrSubstringSearch(t *testing.T) {
	filter := buildOrFilter(
		buildEqualityFilter("objectClass", "person"),
		buildSubstringFilter("cn", "ali"),
		buildSubstringFilter("mail", "ali"),
	)
	got, err := parseLDAPFilter(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "(cn=ali)" {
		t.Fatalf("parseLDAPFilter = %q, want first searchable substring candidate", got)
	}
}

func TestParseLDAPFilterSupportsExtensibleMatch(t *testing.T) {
	filter := buildOrFilter(
		buildExtensibleFilter("objectClass", "organizationalUnit"),
		buildExtensibleFilter("ou", "Research"),
	)
	got, err := parseLDAPFilter(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "(ou=Research)" {
		t.Fatalf("parseLDAPFilter = %q, want extensible type/value candidate", got)
	}

	kinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(kinds) != 1 || kinds[0] != "organization" {
		t.Fatalf("kinds = %#v, want organization", kinds)
	}
}

func TestParseLDAPFilterPrincipalKindsFromObjectClass(t *testing.T) {
	filter := buildOrFilter(
		buildEqualityFilter("objectClass", "organizationalUnit"),
		buildEqualityFilter("objectClass", "groupOfNames"),
		buildSubstringFilter("ou", "Research"),
	)
	got, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	want := []string{"organization", "group"}
	if len(got) != len(want) {
		t.Fatalf("kinds = %#v, want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("kinds = %#v, want %#v", got, want)
		}
	}
}

func TestFilterPrincipalEntriesByLDAPScope(t *testing.T) {
	principals := []PrincipalEntry{
		{DN: "uid=user-1,ou=users,dc=example,dc=com"},
		{DN: "uid=user-2,ou=users,dc=example,dc=com"},
		{DN: "ou=org-1,ou=organizations,dc=example,dc=com"},
	}
	base := filterPrincipalEntriesByScope(principals, "uid=user-1,ou=users,dc=example,dc=com", scopeBaseObject)
	if len(base) != 1 || base[0].DN != "uid=user-1,ou=users,dc=example,dc=com" {
		t.Fatalf("base scope = %+v, want only user-1", base)
	}
	oneLevel := filterPrincipalEntriesByScope(principals, "ou=users,dc=example,dc=com", scopeSingleLevel)
	if len(oneLevel) != 2 {
		t.Fatalf("one-level scope = %+v, want two direct users", oneLevel)
	}
	subtree := filterPrincipalEntriesByScope(principals, "dc=example,dc=com", scopeWholeSubtree)
	if len(subtree) != 3 {
		t.Fatalf("subtree scope = %+v, want all entries", subtree)
	}
}

func TestSelectLDAPAttributesHonorsSpecialSelectors(t *testing.T) {
	attrs := map[string][]string{
		"cn":                   {"Alice"},
		"mail":                 {"alice@example.com"},
		"supportedLDAPVersion": {"3"},
		"entryDN":              {"uid=alice,ou=users,dc=example,dc=com"},
	}
	if got := selectLDAPAttributes(attrs, []string{"1.1"}, false); len(got) != 0 {
		t.Fatalf("1.1 selected attrs = %#v, want none", got)
	}
	got := selectLDAPAttributes(attrs, []string{"+"}, false)
	if len(got) != 2 || got["supportedLDAPVersion"][0] != "3" || got["entryDN"][0] == "" {
		t.Fatalf("+ selected attrs = %#v, want only operational attrs", got)
	}
	got = selectLDAPAttributes(attrs, []string{"*", "+"}, false)
	if len(got) != len(attrs) || got["cn"][0] != "Alice" || got["supportedLDAPVersion"][0] != "3" {
		t.Fatalf("*,+ selected attrs = %#v, want user and operational attrs", got)
	}
}

func TestPrincipalLDAPAttributesSatisfyDeclaredObjectClassRequirements(t *testing.T) {
	userAttrs := principalLDAPAttributes(PrincipalEntry{
		DN:          "uid=alice,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Alice",
		UID:         "alice",
		Mail:        "alice@example.com",
		DisplayName: "Alice User",
	})
	if userAttrs["sn"][0] != "Alice User" {
		t.Fatalf("user sn = %#v, want display-name fallback for person MUST sn", userAttrs["sn"])
	}
	if userAttrs["name"][0] != "Alice" ||
		userAttrs["sAMAccountName"][0] != "alice" ||
		userAttrs["userPrincipalName"][0] != "alice@example.com" {
		t.Fatalf("user AD aliases missing: %#v", userAttrs)
	}
	if userAttrs["distinguishedName"][0] != "uid=alice,ou=users,dc=example,dc=com" ||
		userAttrs["objectCategory"][0] != "person" ||
		userAttrs["objectGUID"][0] == "" ||
		!strings.HasPrefix(userAttrs["objectSid"][0], "S-1-5-21-") {
		t.Fatalf("user AD identity attrs missing: %#v", userAttrs)
	}
	if userAttrs["entryDN"][0] != "uid=alice,ou=users,dc=example,dc=com" || userAttrs["entryUUID"][0] == "" {
		t.Fatalf("user operational attrs missing: %#v", userAttrs)
	}
	userAttrs = principalLDAPAttributes(PrincipalEntry{
		DN:       "uid=alice,ou=users,dc=example,dc=com",
		Kind:     "user",
		CN:       "Alice",
		UID:      "alice",
		MemberOf: []string{"cn=team,ou=groups,dc=example,dc=com", "  "},
	})
	if got := userAttrs["memberOf"]; len(got) != 1 || got[0] != "cn=team,ou=groups,dc=example,dc=com" {
		t.Fatalf("user memberOf = %#v, want concrete group DN", got)
	}
	groupAttrs := principalLDAPAttributes(PrincipalEntry{
		DN:          "cn=team,ou=groups,dc=example,dc=com",
		Kind:        "group",
		CN:          "Team",
		UID:         "team",
		DisplayName: "Team",
	})
	if groupAttrs["member"][0] != "cn=team,ou=groups,dc=example,dc=com" {
		t.Fatalf("group member = %#v, want DN fallback for groupOfNames MUST member", groupAttrs["member"])
	}
	groupAttrs = principalLDAPAttributes(PrincipalEntry{
		DN:      "cn=team,ou=groups,dc=example,dc=com",
		Kind:    "group",
		CN:      "Team",
		UID:     "team",
		Members: []string{"uid=alice,ou=users,dc=example,dc=com", "  ", "ou=ops,ou=organizations,dc=example,dc=com"},
	})
	if got := groupAttrs["member"]; len(got) != 2 ||
		got[0] != "uid=alice,ou=users,dc=example,dc=com" ||
		got[1] != "ou=ops,ou=organizations,dc=example,dc=com" {
		t.Fatalf("group member = %#v, want concrete member DNs", got)
	}
	if groupAttrs["objectCategory"][0] != "group" {
		t.Fatalf("group objectCategory = %#v, want group", groupAttrs["objectCategory"])
	}
	if !strings.HasPrefix(groupAttrs["objectSid"][0], "S-1-5-21-") {
		t.Fatalf("group objectSid = %#v, want stable SID", groupAttrs["objectSid"])
	}
}

func bytesContains(haystack, needle []byte) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if string(haystack[i:i+len(needle)]) == string(needle) {
			return true
		}
	}
	return false
}

func testLDAPTLSConfig(t *testing.T) *tls.Config {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("GenerateKey returned error: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(time.Now().UnixNano()),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("CreateCertificate returned error: %v", err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		t.Fatalf("X509KeyPair returned error: %v", err)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}, MinVersion: tls.VersionTLS12}
}
