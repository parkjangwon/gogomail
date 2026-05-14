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

type blockingDirectoryQuerier struct{}

func (blockingDirectoryQuerier) SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error) {
	<-ctx.Done()
	return nil, ctx.Err()
}

type abandonAwareDirectoryQuerier struct {
	started      chan struct{}
	canceled     chan struct{}
	startedOnce  sync.Once
	canceledOnce sync.Once
}

func (q *abandonAwareDirectoryQuerier) SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error) {
	q.startedOnce.Do(func() {
		close(q.started)
	})
	<-ctx.Done()
	q.canceledOnce.Do(func() {
		close(q.canceled)
	})
	return nil, ctx.Err()
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

func buildSASLBindRequest(version int, name string) []byte {
	var content []byte
	content = append(content, encodeInt(version)...)
	content = append(content, encodeOctetString(name)...)
	content = append(content, 0xa3)
	content = append(content, encodeLength(0)...)
	return content
}

func buildSearchRequest(baseDN string, scope int, filter []byte) []byte {
	return buildSearchRequestWithParams(baseDN, scope, derefAliasesNever, filter)
}

func buildSearchRequestWithSizeLimit(baseDN string, scope int, sizeLimit int, filter []byte) []byte {
	return buildSearchRequestWithLimits(baseDN, scope, sizeLimit, 0, filter)
}

func buildSearchRequestWithTimeLimit(baseDN string, scope int, timeLimit int, filter []byte) []byte {
	return buildSearchRequestWithLimits(baseDN, scope, 0, timeLimit, filter)
}

func buildSearchRequestWithLimits(baseDN string, scope int, sizeLimit int, timeLimit int, filter []byte) []byte {
	var content []byte
	content = append(content, encodeOctetString(baseDN)...)
	content = append(content, encodeEnumerated(scope)...)
	content = append(content, encodeEnumerated(derefAliasesNever)...)
	content = append(content, encodeInt(sizeLimit)...)
	content = append(content, encodeInt(timeLimit)...)
	content = append(content, tagBoolean, 0x01, 0x00)
	content = append(content, filter...)
	var attrList []byte
	attrList = append(attrList, tagSequence)
	attrList = append(attrList, encodeLength(0)...)
	content = append(content, attrList...)
	return content
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

func buildExtendedRequestWithValue(name string, value []byte) []byte {
	content := buildExtendedRequest(name)
	content = append(content, 0x81)
	content = append(content, encodeLength(len(value))...)
	content = append(content, value...)
	return content
}

func buildEqualityFilter(attr, value string) []byte {
	filterContent := append(encodeOctetString(attr), encodeOctetString(value)...)
	filterData := []byte{tagContextSpecific | filterEqualityMatch}
	filterData = append(filterData, encodeLength(len(filterContent))...)
	return append(filterData, filterContent...)
}

func buildOrderingFilter(filterType int, attr, value string) []byte {
	filterContent := append(encodeOctetString(attr), encodeOctetString(value)...)
	filterData := []byte{tagContextSpecific | byte(filterType)}
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

func buildExtensibleFilterWithRule(attr, matchingRule, value string) []byte {
	var filterContent []byte
	filterContent = append(filterContent, 0x81)
	filterContent = append(filterContent, encodeLength(len(matchingRule))...)
	filterContent = append(filterContent, []byte(matchingRule)...)
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

func buildAndFilter(children ...[]byte) []byte {
	var content []byte
	for _, child := range children {
		content = append(content, child...)
	}
	filterData := []byte{tagContextSpecific | 0x20 | filterAnd}
	filterData = append(filterData, encodeLength(len(content))...)
	return append(filterData, content...)
}

func buildNotFilter(child []byte) []byte {
	filterData := []byte{tagContextSpecific | 0x20 | filterNot}
	filterData = append(filterData, encodeLength(len(child))...)
	return append(filterData, child...)
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

func readSearchResultCodeUntilDone(t *testing.T, conn net.Conn) (int, int) {
	t.Helper()
	entries := 0
	for {
		resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
		if err != nil {
			t.Fatalf("read search response: %v", err)
		}
		_, opTag, opData, err := decodeLDAPPacket(resp)
		if err != nil {
			t.Fatalf("decode search response: %v", err)
		}
		switch opTag {
		case opSearchResultEntry:
			entries++
		case opSearchResultDone:
			return entries, decodeEnumerated(opData)
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

func TestLDAPServerHandlesBindPDUOverInitialReadBuffer(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	longPassword := strings.Repeat("s", 9000)
	auth := newFakeLDAPAuth()
	auth.addUser("admin@example.com", longPassword)
	srv := NewServer(ln, auth, newFakeDirectoryQuerier())
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	bindReq := buildLDAPPacket(45, opBindRequest, buildBindRequest(3, "admin@example.com", longPassword))
	if len(bindReq) <= 8192 {
		t.Fatalf("test bind PDU length = %d, want >8192", len(bindReq))
	}
	if err := sendPDU(conn, bindReq); err != nil {
		t.Fatal(err)
	}

	resp, err := readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	msgID, opTag, opData, err := decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if msgID != 45 || opTag != opBindResponse || decodeEnumerated(opData) != resultSuccess {
		t.Fatalf("large bind response msg/op/result = %d/%d/%d, want 45/%d/%d", msgID, opTag, decodeEnumerated(opData), opBindResponse, resultSuccess)
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

func TestLDAPServerFailedRebindClearsAuthentication(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	auth.addUser("admin@example.com", "secret")
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()

	okBind := buildLDAPPacket(41, opBindRequest, buildBindRequest(3, "admin@example.com", "secret"))
	if err := sendPDU(conn, okBind); err != nil {
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
		t.Fatalf("initial bind op/result = %d/%d, want success", opTag, decodeEnumerated(opData))
	}

	badBind := buildLDAPPacket(42, opBindRequest, buildBindRequest(3, "admin@example.com", "wrong"))
	if err := sendPDU(conn, badBind); err != nil {
		t.Fatal(err)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err = decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode failed bind response: %v", err)
	}
	if opTag != opBindResponse || decodeEnumerated(opData) != resultInvalidCredentials {
		t.Fatalf("failed rebind op/result = %d/%d, want invalidCredentials", opTag, decodeEnumerated(opData))
	}

	searchReq := buildLDAPPacket(43, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
	)
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	resp, err = readFullPDU(conn, time.Now().Add(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	_, opTag, opData, err = decodeLDAPPacket(resp)
	if err != nil {
		t.Fatalf("decode search response: %v", err)
	}
	if opTag != opSearchResultDone || decodeEnumerated(opData) != resultInsufficientAccessRights {
		t.Fatalf("post-failed-bind search op/result = %d/%d, want bind required", opTag, decodeEnumerated(opData))
	}
}

func TestLDAPServerRejectsUnsupportedBindAuthenticationChoice(t *testing.T) {
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

	bindReq := buildLDAPPacket(44, opBindRequest, buildSASLBindRequest(3, "admin@example.com"))
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
	if opTag != opBindResponse || decodeEnumerated(opData) != resultAuthMethodNotSupported {
		t.Fatalf("unsupported bind auth op/result = %d/%d, want %d/%d", opTag, decodeEnumerated(opData), opBindResponse, resultAuthMethodNotSupported)
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

func TestDecodeCompareRequestRejectsTrailingAssertionSequenceData(t *testing.T) {
	data := append(buildCompareRequest("uid=alice,ou=users,dc=example,dc=com", "mail", "alice@example.com"), 0x00)
	if _, err := decodeCompareRequestData(data); err == nil {
		t.Fatal("decodeCompareRequestData accepted trailing data after assertion sequence")
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

func TestDecodeAbandonRequestMessageIDRejectsOutOfRangeTargets(t *testing.T) {
	if got, ok := decodeAbandonRequestMessageID([]byte{0x32}); !ok || got != 50 {
		t.Fatalf("decodeAbandonRequestMessageID raw = %d/%v, want 50/true", got, ok)
	}
	for _, tc := range [][]byte{
		{},
		{0x00},
		{0xff},
		{0x00, 0x80, 0x00, 0x00, 0x00},
		{0x00, 0xff, 0xff, 0xff, 0xff, 0xff},
		encodeInt(ldapMaxMessageID + 1),
	} {
		if target, ok := decodeAbandonRequestMessageID(tc); ok {
			t.Fatalf("decodeAbandonRequestMessageID(%x) = %d/true, want false", tc, target)
		}
	}
}

func TestLDAPServerAbandonRequestCancelsOutstandingSearch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := &abandonAwareDirectoryQuerier{
		started:  make(chan struct{}),
		canceled: make(chan struct{}),
	}
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	searchReq := buildLDAPPacket(50, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person")),
	)
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dir.started:
	case <-time.After(2 * time.Second):
		t.Fatal("search did not start")
	}
	abandonReq := buildLDAPPacket(51, opAbandonRequest, encodeInt(50))
	if err := sendPDU(conn, abandonReq); err != nil {
		t.Fatal(err)
	}
	select {
	case <-dir.canceled:
	case <-time.After(2 * time.Second):
		t.Fatal("AbandonRequest did not cancel outstanding search")
	}

	buf := make([]byte, 1)
	conn.SetReadDeadline(time.Now().Add(250 * time.Millisecond))
	if _, err := conn.Read(buf); err == nil {
		t.Fatal("abandoned search produced a response, want no response")
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

func TestLDAPServerSearchSizeLimitResultCode(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", UID: "alice"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", UID: "bob"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := buildEqualityFilter("objectClass", "person")
	exactReq := buildLDAPPacket(33, opSearchRequest, buildSearchRequestWithSizeLimit("ou=users,dc=example,dc=com", scopeWholeSubtree, 2, filter))
	if err := sendPDU(conn, exactReq); err != nil {
		t.Fatal(err)
	}
	entries, result := readSearchResultCodeUntilDone(t, conn)
	if entries != 2 || result != resultSuccess {
		t.Fatalf("exact sizeLimit search entries/result = %d/%d, want 2/%d", entries, result, resultSuccess)
	}

	limitedReq := buildLDAPPacket(34, opSearchRequest, buildSearchRequestWithSizeLimit("ou=users,dc=example,dc=com", scopeWholeSubtree, 1, filter))
	if err := sendPDU(conn, limitedReq); err != nil {
		t.Fatal(err)
	}
	entries, result = readSearchResultCodeUntilDone(t, conn)
	if entries != 1 || result != resultSizeLimitExceeded {
		t.Fatalf("exceeded sizeLimit search entries/result = %d/%d, want 1/%d", entries, result, resultSizeLimitExceeded)
	}
}

func TestLDAPServerSearchTimeLimitResultCode(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	srv := NewServer(ln, auth, blockingDirectoryQuerier{})
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacket(49, opSearchRequest,
		buildSearchRequestWithTimeLimit("ou=users,dc=example,dc=com", scopeWholeSubtree, 1, buildEqualityFilter("objectClass", "person")),
	)
	start := time.Now()
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	entries, result := readSearchResultCodeUntilDone(t, conn)
	if entries != 0 || result != resultTimeLimitExceeded {
		t.Fatalf("timeLimit search entries/result = %d/%d, want 0/%d", entries, result, resultTimeLimitExceeded)
	}
	if elapsed := time.Since(start); elapsed < time.Second || elapsed > 3*time.Second {
		t.Fatalf("timeLimit elapsed = %v, want roughly one second", elapsed)
	}
}

func TestLDAPServerAppliesFullSearchFilterToEntries(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", Mail: "alice@example.com", UID: "alice"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", Mail: "bob@example.com", UID: "bob"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := buildAndFilter(
		buildEqualityFilter("mail", "alice@example.com"),
		buildEqualityFilter("uid", "bob"),
	)
	req := buildLDAPPacket(35, opSearchRequest, buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, filter))
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	entries, result := readSearchResultCodeUntilDone(t, conn)
	if entries != 0 || result != resultSuccess {
		t.Fatalf("conflicting AND filter entries/result = %d/%d, want 0/%d", entries, result, resultSuccess)
	}
}

func TestLDAPServerDoesNotUnderReturnOrSearchFilters(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", Mail: "alice@example.com", UID: "alice"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=bob,ou=users,dc=example,dc=com", Kind: "user", CN: "Bob", Mail: "bob@example.com", UID: "bob"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=dana,ou=users,dc=example,dc=com", Kind: "user", CN: "Dana", Mail: "dana@example.com", UID: "dana"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := buildOrFilter(
		buildEqualityFilter("mail", "alice@example.com"),
		buildEqualityFilter("mail", "bob@example.com"),
	)
	req := buildLDAPPacket(37, opSearchRequest, buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, filter))
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, _ := readSearchDNsUntilDone(t, conn)
	if len(dns) != 2 ||
		!containsStringFold(dns, "uid=alice,ou=users,dc=example,dc=com") ||
		!containsStringFold(dns, "uid=bob,ou=users,dc=example,dc=com") ||
		containsStringFold(dns, "uid=dana,ou=users,dc=example,dc=com") {
		t.Fatalf("OR filter DNs = %#v, want alice and bob only", dns)
	}
}

func TestLDAPServerAppliesOrderingFiltersWithoutRepositoryHint(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", Mail: "alice@example.com", UID: "alice"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=mara,ou=users,dc=example,dc=com", Kind: "user", CN: "Mara", Mail: "mara@example.com", UID: "mara"})
	dir.addPrincipal(PrincipalEntry{DN: "uid=zoe,ou=users,dc=example,dc=com", Kind: "user", CN: "Zoe", Mail: "zoe@example.com", UID: "zoe"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	req := buildLDAPPacket(39, opSearchRequest,
		buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, buildOrderingFilter(filterGreaterOrEqual, "cn", "M")),
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, _ := readSearchDNsUntilDone(t, conn)
	if len(dns) != 2 ||
		containsStringFold(dns, "uid=alice,ou=users,dc=example,dc=com") ||
		!containsStringFold(dns, "uid=mara,ou=users,dc=example,dc=com") ||
		!containsStringFold(dns, "uid=zoe,ou=users,dc=example,dc=com") {
		t.Fatalf("ordering filter DNs = %#v, want Mara and Zoe only", dns)
	}
}

func TestLDAPServerAppliesADStyleUserSearchFilter(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	auth := newFakeLDAPAuth()
	dir := newFakeDirectoryQuerier()
	dir.addPrincipal(PrincipalEntry{DN: "uid=alice,ou=users,dc=example,dc=com", Kind: "user", CN: "Alice", Mail: "alice@example.com", UID: "alice"})
	srv := NewServer(ln, auth, dir)
	go srv.Serve()
	defer srv.Close()

	conn, err := net.Dial("tcp", ln.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	bindTestConnection(t, conn, auth)

	filter := buildAndFilter(
		buildEqualityFilter("objectCategory", "person"),
		buildEqualityFilter("objectClass", "user"),
		buildNotFilter(buildExtensibleFilterWithRule("userAccountControl", "1.2.840.113556.1.4.803", "2")),
		buildEqualityFilter("sAMAccountName", "alice"),
	)
	req := buildLDAPPacket(36, opSearchRequest, buildSearchRequest("ou=users,dc=example,dc=com", scopeWholeSubtree, filter))
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	entries, result := readSearchResultCodeUntilDone(t, conn)
	if entries != 1 || result != resultSuccess {
		t.Fatalf("AD user filter entries/result = %d/%d, want 1/%d", entries, result, resultSuccess)
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

func TestDecodeSearchRequestRejectsNegativeLimits(t *testing.T) {
	if _, _, _, _, _, _, _, err := decodeSearchRequest(buildSearchRequestWithLimits("dc=example,dc=com", scopeWholeSubtree, -1, 0, buildEqualityFilter("objectClass", "person"))); err == nil {
		t.Fatal("decodeSearchRequest accepted negative sizeLimit")
	}
	if _, _, _, _, _, _, _, err := decodeSearchRequest(buildSearchRequestWithLimits("dc=example,dc=com", scopeWholeSubtree, 0, -1, buildEqualityFilter("objectClass", "person"))); err == nil {
		t.Fatal("decodeSearchRequest accepted negative timeLimit")
	}
	if got, _, err := decodeInt(encodeInt(-1)); err != nil || got != -1 {
		t.Fatalf("decodeInt negative = %d, %v; want -1", got, err)
	}
}

func TestDecodeSearchRequestRejectsMalformedAttributeList(t *testing.T) {
	valid := buildSearchRequestWithAttrs("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"), "cn")
	if _, _, _, _, _, _, _, err := decodeSearchRequest(append(valid, 0x00)); err == nil {
		t.Fatal("decodeSearchRequest accepted trailing data after attribute list")
	}
	base := buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, buildEqualityFilter("objectClass", "person"))
	malformed := append(append([]byte{}, base[:len(base)-2]...), tagSequence, 0x01, tagBoolean)
	if _, _, _, _, _, _, _, err := decodeSearchRequest(malformed); err == nil {
		t.Fatal("decodeSearchRequest accepted malformed attribute list")
	}
	truncatedAttr := append(append([]byte{}, base[:len(base)-2]...), tagSequence, 0x03, tagOctetString, 0x02, 'c')
	if _, _, _, _, _, _, _, err := decodeSearchRequest(truncatedAttr); err == nil {
		t.Fatal("decodeSearchRequest accepted truncated attribute description")
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

func TestLDAPServerPagedResultsFetchesEnoughCandidatesForPostFilter(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	for i := 1; i <= 5; i++ {
		uid := fmt.Sprintf("user%d", i)
		mail := uid + "@example.com"
		if i >= 4 {
			mail = "match" + fmt.Sprint(i) + "@example.com"
		}
		dir.addPrincipal(PrincipalEntry{
			DN:          fmt.Sprintf("uid=%s,ou=users,dc=example,dc=com", uid),
			CN:          uid,
			Mail:        mail,
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

	filter := buildOrFilter(
		buildEqualityFilter("mail", "match4@example.com"),
		buildEqualityFilter("mail", "match5@example.com"),
	)
	req := buildLDAPPacketWithControls(38, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, filter),
		[]control{buildPagedResultsControl(1, "")},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, controls := readSearchDNsUntilDone(t, conn)
	if len(dns) != 1 || dns[0] != "uid=user4,ou=users,dc=example,dc=com" {
		t.Fatalf("first sparse paged result DNs = %#v, want user4", dns)
	}
	if cookie := pagedResponseCookie(t, controls); cookie != "1" {
		t.Fatalf("first sparse paged cookie = %q, want 1", cookie)
	}
}

func TestLDAPServerPagedResultsScansBeyondFirstCandidateBatch(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	dir := newFakeDirectoryQuerier()
	for i := 1; i <= 125; i++ {
		uid := fmt.Sprintf("user%03d", i)
		mail := uid + "@example.com"
		if i >= 120 {
			mail = "late" + fmt.Sprint(i) + "@example.com"
		}
		dir.addPrincipal(PrincipalEntry{
			DN:          fmt.Sprintf("uid=%s,ou=users,dc=example,dc=com", uid),
			CN:          uid,
			Mail:        mail,
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

	filter := buildOrFilter(
		buildEqualityFilter("mail", "late120@example.com"),
		buildEqualityFilter("mail", "late121@example.com"),
	)
	req := buildLDAPPacketWithControls(40, opSearchRequest,
		buildSearchRequest("dc=example,dc=com", scopeWholeSubtree, filter),
		[]control{buildPagedResultsControl(1, "")},
	)
	if err := sendPDU(conn, req); err != nil {
		t.Fatal(err)
	}
	dns, controls := readSearchDNsUntilDone(t, conn)
	if len(dns) != 1 || dns[0] != "uid=user120,ou=users,dc=example,dc=com" {
		t.Fatalf("late sparse paged result DNs = %#v, want user120", dns)
	}
	if cookie := pagedResponseCookie(t, controls); cookie != "1" {
		t.Fatalf("late sparse paged cookie = %q, want 1", cookie)
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

func TestLDAPServerContainerBaseObjectAppliesSearchFilter(t *testing.T) {
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

	searchReq := buildLDAPPacket(48, opSearchRequest,
		buildSearchRequestWithAttrs("ou=organizations,dc=example,dc=com", scopeBaseObject, buildEqualityFilter("objectClass", "person"), "objectClass"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 0 {
		t.Fatalf("container person filter entries = %d, want 0", entries)
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

func TestLDAPServerOpenLDAPADMetadataCompatibility(t *testing.T) {
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
		"(canonicalName=example.com/users/alice)",
		"canonicalName",
		"instanceType",
		"whenCreated",
		"whenChanged",
		"uSNCreated",
		"uSNChanged",
		"userAccountControl",
		"accountExpires",
		"primaryGroupID",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch AD metadata failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, expected := range []string{
		"canonicalName: example.com/users/alice",
		"instanceType: 4",
		"whenCreated: 19700101000000.0Z",
		"whenChanged: 19700101000000.0Z",
		"uSNCreated:",
		"uSNChanged:",
		"userAccountControl: 512",
		"accountExpires: 9223372036854775807",
		"primaryGroupID: 513",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("ldapsearch AD metadata output missing %q:\n%s", expected, output)
		}
	}
}

func TestLDAPServerOpenLDAPADUserFilterCompatibility(t *testing.T) {
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
	dir.addPrincipal(PrincipalEntry{
		DN:          "uid=bob,ou=users,dc=example,dc=com",
		Kind:        "user",
		CN:          "Bob",
		Mail:        "bob@example.com",
		UID:         "bob",
		DisplayName: "Bob",
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
		"(&(objectCategory=person)(objectClass=user)(!(userAccountControl:1.2.840.113556.1.4.803:=2))(sAMAccountName=alice))",
		"cn",
		"objectClass",
		"userAccountControl",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ldapsearch AD user filter failed: %v\n%s", err, out)
	}
	output := string(out)
	for _, expected := range []string{
		"dn: uid=alice,ou=users,dc=example,dc=com",
		"cn: Alice",
		"objectClass: user",
		"userAccountControl: 512",
	} {
		if !strings.Contains(output, expected) {
			t.Fatalf("ldapsearch AD user filter output missing %q:\n%s", expected, output)
		}
	}
	if strings.Contains(output, "dn: uid=bob,ou=users,dc=example,dc=com") {
		t.Fatalf("ldapsearch AD user filter output included nonmatching user:\n%s", output)
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

func TestDecodeExtendedRequestNameValidatesOIDAndRequestValue(t *testing.T) {
	if name, err := decodeExtendedRequestName(buildExtendedRequestWithValue(whoAmIOID, []byte("opaque"))); err != nil || name != whoAmIOID {
		t.Fatalf("decodeExtendedRequestName valid value = %q, %v", name, err)
	}
	for _, tc := range []struct {
		name string
		data []byte
	}{
		{name: "empty oid", data: buildExtendedRequest("")},
		{name: "descriptor oid", data: buildExtendedRequest("whoami")},
		{name: "malformed oid", data: buildExtendedRequest("1..2")},
		{name: "unexpected trailing", data: append(buildExtendedRequest(whoAmIOID), 0x04, 0x00)},
		{name: "truncated value", data: append(buildExtendedRequest(whoAmIOID), 0x81, 0x02, 0x00)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := decodeExtendedRequestName(tc.data); err == nil {
				t.Fatal("decodeExtendedRequestName accepted malformed request")
			}
		})
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
	searchReq := buildLDAPPacket(6, opSearchRequest, buildSearchRequestWithAttrs("", scopeBaseObject, filter, "namingContexts", "defaultNamingContext", "rootDomainNamingContext", "configurationNamingContext", "schemaNamingContext", "supportedExtension", "supportedFeatures", "supportedCapabilities", "subschemaSubentry", "dnsHostName", "serverName", "dsServiceName", "currentTime", "highestCommittedUSN", "domainControllerFunctionality", "isGlobalCatalogReady", "isSynchronized"))
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
	for _, want := range []string{
		"defaultNamingContext",
		"rootDomainNamingContext",
		"configurationNamingContext",
		"cn=Configuration,dc=example,dc=com",
		"schemaNamingContext",
		"cn=Schema,cn=Configuration,dc=example,dc=com",
		"supportedCapabilities",
		"1.2.840.113556.1.4.800",
		"dnsHostName",
		"ldap.example.com",
		"serverName",
		"cn=ldap,cn=Servers,cn=Default-First-Site-Name,cn=Sites,cn=Configuration,dc=example,dc=com",
		"dsServiceName",
		"cn=NTDS Settings,cn=ldap,cn=Servers,cn=Default-First-Site-Name,cn=Sites,cn=Configuration,dc=example,dc=com",
		"currentTime",
		"highestCommittedUSN",
		"domainControllerFunctionality",
		"isGlobalCatalogReady",
		"isSynchronized",
	} {
		if !bytesContains(opData, []byte(want)) {
			t.Fatalf("root DSE response did not include %q: %x", want, opData)
		}
	}
	if !bytesContains(opData, []byte(startTLSOID)) {
		t.Fatalf("root DSE response did not include StartTLS OID: %x", opData)
	}
	if !bytesContains(opData, []byte(ldapFeatureAllOperationalAttributes)) {
		t.Fatalf("root DSE response did not include all-operational-attrs feature OID: %x", opData)
	}
	if !bytesContains(opData, []byte("cn=Subschema")) {
		t.Fatalf("root DSE response did not include subschemaSubentry: %x", opData)
	}
}

func TestRootDSEAttributesDeriveADDiscoveryFromNamingContext(t *testing.T) {
	attrs := rootDSEAttributes([]string{"dc=example,dc=com", "dc=example,dc=net"}, false)
	if attrs["defaultNamingContext"][0] != "dc=example,dc=com" ||
		attrs["rootDomainNamingContext"][0] != "dc=example,dc=com" ||
		attrs["configurationNamingContext"][0] != "cn=Configuration,dc=example,dc=com" ||
		attrs["schemaNamingContext"][0] != "cn=Schema,cn=Configuration,dc=example,dc=com" ||
		attrs["dnsHostName"][0] != "ldap.example.com" ||
		attrs["serverName"][0] != "cn=ldap,cn=Servers,cn=Default-First-Site-Name,cn=Sites,cn=Configuration,dc=example,dc=com" ||
		attrs["dsServiceName"][0] != "cn=NTDS Settings,cn=ldap,cn=Servers,cn=Default-First-Site-Name,cn=Sites,cn=Configuration,dc=example,dc=com" ||
		attrs["highestCommittedUSN"][0] == "" {
		t.Fatalf("AD discovery attrs = %#v", attrs)
	}
	if got := attrs["currentTime"][0]; len(got) != len("20260514010203.0Z") || !strings.HasSuffix(got, ".0Z") {
		t.Fatalf("currentTime = %q, want generalized time", got)
	}
	if got := attrs["namingContexts"]; len(got) != 2 || got[1] != "dc=example,dc=net" {
		t.Fatalf("namingContexts = %#v, want all configured contexts", got)
	}
	if len(attrs["supportedCapabilities"]) < 3 {
		t.Fatalf("supportedCapabilities = %#v, want AD compatibility OIDs", attrs["supportedCapabilities"])
	}
	if attrs["domainControllerFunctionality"][0] != "7" ||
		attrs["domainFunctionality"][0] != "7" ||
		attrs["forestFunctionality"][0] != "7" ||
		attrs["isGlobalCatalogReady"][0] != "TRUE" ||
		attrs["isSynchronized"][0] != "TRUE" {
		t.Fatalf("AD functionality attrs = %#v", attrs)
	}
}

func TestLDAPDNSDomainFromNamingContext(t *testing.T) {
	if got := ldapDNSDomainFromNamingContext(`dc=example\,corp,dc=com`); got != "example,corp.com" {
		t.Fatalf("ldapDNSDomainFromNamingContext escaped = %q", got)
	}
	if got := ldapDNSDomainFromNamingContext("o=example"); got != "gogomail.local" {
		t.Fatalf("ldapDNSDomainFromNamingContext fallback = %q", got)
	}
}

func TestLDAPServerRootDSEAppliesSearchFilter(t *testing.T) {
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

	searchReq := buildLDAPPacket(46, opSearchRequest,
		buildSearchRequestWithAttrs("", scopeBaseObject, buildEqualityFilter("objectClass", "person"), "objectClass"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 0 {
		t.Fatalf("RootDSE person filter entries = %d, want 0", entries)
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

func TestLDAPServerSubschemaAppliesSearchFilter(t *testing.T) {
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

	searchReq := buildLDAPPacket(47, opSearchRequest,
		buildSearchRequestWithAttrs("cn=Subschema", scopeBaseObject, buildEqualityFilter("objectClass", "person"), "objectClass"),
	)
	if err := sendPDU(conn, searchReq); err != nil {
		t.Fatal(err)
	}
	entries, _ := readSearchUntilDone(t, conn)
	if entries != 0 {
		t.Fatalf("Subschema person filter entries = %d, want 0", entries)
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

func TestParseLDAPFilterDoesNotUseOrAsRepositoryHint(t *testing.T) {
	filter := buildOrFilter(
		buildEqualityFilter("objectClass", "person"),
		buildSubstringFilter("cn", "ali"),
		buildSubstringFilter("mail", "ali"),
	)
	got, err := parseLDAPFilter(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseLDAPFilter = %q, want no unsafe OR repository hint", got)
	}
}

func TestParseLDAPFilterIgnoresNegatedSearchCandidates(t *testing.T) {
	filter := buildAndFilter(
		buildEqualityFilter("objectCategory", "person"),
		buildNotFilter(buildEqualityFilter("cn", "Disabled")),
		buildEqualityFilter("mail", "alice@example.com"),
	)
	got, err := parseLDAPFilter(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "(mail=alice@example.com)" {
		t.Fatalf("parseLDAPFilter = %q, want positive mail candidate", got)
	}

	kinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(kinds) != 1 || kinds[0] != "user" {
		t.Fatalf("kinds = %#v, want only positive user kind", kinds)
	}
}

func TestParseLDAPFilterIgnoresNegatedPrincipalKinds(t *testing.T) {
	filter := buildAndFilter(
		buildNotFilter(buildEqualityFilter("objectClass", "groupOfNames")),
		buildSubstringFilter("cn", "team"),
	)
	kinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(kinds) != 0 {
		t.Fatalf("kinds = %#v, want no positive kind narrowing from NOT", kinds)
	}
}

func TestParseLDAPFilterDoesNotUseOrderingFiltersAsRepositoryHints(t *testing.T) {
	filter := buildAndFilter(
		buildOrderingFilter(filterGreaterOrEqual, "cn", "M"),
		buildEqualityFilter("mail", "alice@example.com"),
	)
	got, err := parseLDAPFilter(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "(mail=alice@example.com)" {
		t.Fatalf("parseLDAPFilter = %q, want equality hint after unsafe ordering filter", got)
	}

	got, err = parseLDAPFilter(buildOrderingFilter(filterLessOrEqual, "cn", "M"))
	if err != nil {
		t.Fatalf("parseLDAPFilter returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("parseLDAPFilter = %q, want no ordering repository hint", got)
	}
}

func TestParseLDAPFilterSupportsExtensibleMatch(t *testing.T) {
	filter := buildAndFilter(
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
	filter := buildAndFilter(
		buildEqualityFilter("objectClass", "organizationalUnit"),
		buildSubstringFilter("ou", "Research"),
	)
	got, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(got) != 1 || got[0] != "organization" {
		t.Fatalf("kinds = %#v, want organization", got)
	}
}

func TestParseLDAPFilterPrincipalKindsFromObjectCategory(t *testing.T) {
	filter := buildAndFilter(
		buildEqualityFilter("objectCategory", "person"),
		buildSubstringFilter("cn", "Team"),
	)
	got, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(got) != 1 || got[0] != "user" {
		t.Fatalf("kinds = %#v, want user", got)
	}
}

func TestParseLDAPFilterPrincipalKindsDoesNotNarrowMixedOr(t *testing.T) {
	filter := buildOrFilter(
		buildEqualityFilter("objectClass", "person"),
		buildSubstringFilter("cn", "Team"),
	)
	got, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		t.Fatalf("parseLDAPFilterPrincipalKinds returned error: %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("kinds = %#v, want no unsafe OR kind narrowing", got)
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

func TestLDAPContainerAttributesExposeSubordinateHints(t *testing.T) {
	attrs, ok := ldapContainerAttributes("ou=users,dc=example,dc=com")
	if !ok {
		t.Fatal("ldapContainerAttributes did not recognize users container")
	}
	if attrs["hasSubordinates"][0] != "TRUE" || attrs["numSubordinates"][0] != "0" {
		t.Fatalf("container subordinate attrs = %#v", attrs)
	}
	leaf := ldapOperationalAttributes("uid=alice,ou=users,dc=example,dc=com")
	if leaf["hasSubordinates"][0] != "FALSE" || leaf["numSubordinates"][0] != "0" {
		t.Fatalf("leaf subordinate attrs = %#v", leaf)
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
	if !containsStringFold(userAttrs["objectClass"], "user") {
		t.Fatalf("user objectClass = %#v, want AD-compatible user class", userAttrs["objectClass"])
	}
	if userAttrs["name"][0] != "Alice" ||
		userAttrs["sAMAccountName"][0] != "alice" ||
		userAttrs["userPrincipalName"][0] != "alice@example.com" ||
		userAttrs["mailNickname"][0] != "alice" ||
		userAttrs["proxyAddresses"][0] != "SMTP:alice@example.com" {
		t.Fatalf("user AD aliases missing: %#v", userAttrs)
	}
	if userAttrs["distinguishedName"][0] != "uid=alice,ou=users,dc=example,dc=com" ||
		userAttrs["canonicalName"][0] != "example.com/users/alice" ||
		userAttrs["instanceType"][0] != "4" ||
		userAttrs["objectCategory"][0] != "person" ||
		len(userAttrs["objectGUID"][0]) != 16 ||
		!isLDAPBinarySID(userAttrs["objectSid"][0]) {
		t.Fatalf("user AD identity attrs missing: %#v", userAttrs)
	}
	if userAttrs["whenCreated"][0] != "19700101000000.0Z" ||
		userAttrs["whenChanged"][0] != "19700101000000.0Z" ||
		userAttrs["uSNCreated"][0] == "" ||
		userAttrs["uSNChanged"][0] != userAttrs["uSNCreated"][0] ||
		userAttrs["accountExpires"][0] != "9223372036854775807" ||
		userAttrs["primaryGroupID"][0] != "513" ||
		userAttrs["userAccountControl"][0] != "512" {
		t.Fatalf("user AD metadata attrs missing: %#v", userAttrs)
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
	if groupAttrs["canonicalName"][0] != "example.com/groups/team" ||
		groupAttrs["instanceType"][0] != "4" ||
		groupAttrs["uSNChanged"][0] == "" {
		t.Fatalf("group AD metadata attrs missing: %#v", groupAttrs)
	}
	if !isLDAPBinarySID(groupAttrs["objectSid"][0]) {
		t.Fatalf("group objectSid = %#v, want stable binary SID", groupAttrs["objectSid"])
	}
}

func isLDAPBinarySID(value string) bool {
	b := []byte(value)
	if len(b) < 28 {
		return false
	}
	return b[0] == 1 &&
		int(b[1]) == 5 &&
		b[2] == 0 &&
		b[3] == 0 &&
		b[4] == 0 &&
		b[5] == 0 &&
		b[6] == 0 &&
		b[7] == 5
}

func TestLDAPBinaryADAttributesUseExactFilterMatching(t *testing.T) {
	attrs := map[string][]string{
		"objectGUID": {string([]byte("ABCDEFGHIJKLMNOP"))},
		"objectSid":  {string([]byte{1, 5, 0, 0, 0, 0, 0, 5, 'A', 0, 0, 0, 1, 0, 0, 0})},
	}
	if !ldapEntryAttributesMatchFilter(attrs, buildEqualityFilter("objectGUID", string([]byte("ABCDEFGHIJKLMNOP")))) {
		t.Fatalf("objectGUID exact binary equality did not match")
	}
	if ldapEntryAttributesMatchFilter(attrs, buildEqualityFilter("objectGUID", string([]byte("aBCDEFGHIJKLMNOP")))) {
		t.Fatalf("objectGUID binary equality matched case-folded bytes")
	}
	if !ldapAttributeValueMatchesFilter("objectSid", attrs["objectSid"][0], buildEqualityFilter("objectSid", attrs["objectSid"][0])) {
		t.Fatalf("objectSid matched-values binary equality did not match")
	}
	if ldapAttributeValueMatchesFilter("objectSid", attrs["objectSid"][0], buildEqualityFilter("objectSid", string([]byte{1, 5, 0, 0, 0, 0, 0, 5, 'a', 0, 0, 0, 1, 0, 0, 0}))) {
		t.Fatalf("objectSid matched-values binary equality matched case-folded bytes")
	}
}

func TestLDAPCanonicalName(t *testing.T) {
	got := ldapCanonicalName(`cn=Team\, Ops,ou=groups,dc=example,dc=com`)
	if got != "example.com/groups/Team, Ops" {
		t.Fatalf("ldapCanonicalName = %q, want escaped RDN value in canonical path", got)
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
