package ldapgw

import (
	"strings"
	"testing"
)

func TestBerEncodeBindRequest(t *testing.T) {
	req := bindRequest{
		version: 3,
		name:    "cn=admin,dc=example,dc=com",
		auth:    simpleAuth("secret"),
	}

	data, err := req.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("encode returned empty data")
	}
}

func TestBerDecodeBindRequest(t *testing.T) {
	req := bindRequest{
		version: 3,
		name:    "cn=admin,dc=example,dc=com",
		auth:    simpleAuth("secret"),
	}

	data, err := req.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	decoded, err := decodeBindRequest(data)
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if decoded.version != 3 {
		t.Fatalf("version = %d, want 3", decoded.version)
	}
	if decoded.name != "cn=admin,dc=example,dc=com" {
		t.Fatalf("name = %s, want cn=admin,dc=example,dc=com", decoded.name)
	}
	if string(decoded.auth) != "secret" {
		t.Fatalf("auth = %s, want secret", string(decoded.auth))
	}
}

func TestIsReadOnlyOperation(t *testing.T) {
	tests := []struct {
		op       int
		readOnly bool
	}{
		{opBindRequest, true},
		{opSearchRequest, true},
		{opUnbindRequest, true},
		{opModifyRequest, false},
		{opAddRequest, false},
		{opDeleteRequest, false},
		{opModDNRequest, false},
		{opCompareRequest, true},
		{opAbandonRequest, true},
	}

	for _, tt := range tests {
		got := isReadOnlyOperation(tt.op)
		if got != tt.readOnly {
			t.Errorf("isReadOnlyOperation(%d) = %v, want %v", tt.op, got, tt.readOnly)
		}
	}
}

func TestEncodeSearchResultEntryUsesApplicationPayloadDirectly(t *testing.T) {
	pdu, err := encodeSearchResultEntry(7, "uid=alice,ou=users,dc=example,dc=com", map[string][]string{
		"cn": {"Alice"},
	})
	if err != nil {
		t.Fatalf("encodeSearchResultEntry returned error: %v", err)
	}
	_, opTag, opData, err := decodeLDAPPacket(pdu)
	if err != nil {
		t.Fatalf("decodeLDAPPacket returned error: %v", err)
	}
	if opTag != opSearchResultEntry {
		t.Fatalf("opTag = %d, want %d", opTag, opSearchResultEntry)
	}
	if len(opData) == 0 || opData[0] != tagOctetString {
		t.Fatalf("SearchResultEntry payload starts with 0x%02x, want LDAPDN octet string", opData[0])
	}
	dn, _, err := decodeOctetString(opData)
	if err != nil {
		t.Fatalf("decode entry DN returned error: %v", err)
	}
	if dn != "uid=alice,ou=users,dc=example,dc=com" {
		t.Fatalf("entry DN = %q", dn)
	}
}

func TestEncodeLDAPResponsePreservesLargeMessageID(t *testing.T) {
	const messageID = 300
	pdu := encodeBindResponse(messageID, resultSuccess, "", "")
	got, opTag, _, err := decodeLDAPPacket(pdu)
	if err != nil {
		t.Fatalf("decodeLDAPPacket returned error: %v", err)
	}
	if opTag != opBindResponse {
		t.Fatalf("opTag = %d, want %d", opTag, opBindResponse)
	}
	if got != messageID {
		t.Fatalf("messageID = %d, want %d", got, messageID)
	}
}

func TestDecodeLDAPPacketRejectsInvalidMessageID(t *testing.T) {
	for _, tc := range []struct {
		name      string
		messageID []byte
	}{
		{name: "zero", messageID: []byte{tagInteger, 0x01, 0x00}},
		{name: "negative", messageID: []byte{tagInteger, 0x01, 0xff}},
		{name: "above-maxInt", messageID: []byte{tagInteger, 0x05, 0x00, 0x80, 0x00, 0x00, 0x00}},
		{name: "too-long", messageID: []byte{tagInteger, 0x09, 0x00, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			op := []byte{byte(opUnbindRequest), 0x00}
			content := append(append([]byte{}, tc.messageID...), op...)
			pdu := append([]byte{tagSequence}, encodeLength(len(content))...)
			pdu = append(pdu, content...)
			if _, _, _, err := decodeLDAPPacket(pdu); err == nil {
				t.Fatal("decodeLDAPPacket accepted invalid messageID")
			}
		})
	}
}

func TestDecodeLDAPPacketRejectsIndefiniteLength(t *testing.T) {
	pdu := []byte{
		tagSequence, 0x80,
		tagInteger, 0x01, 0x01,
		byte(opUnbindRequest), 0x00,
		0x00, 0x00,
	}
	if _, _, _, err := decodeLDAPPacket(pdu); err == nil {
		t.Fatal("decodeLDAPPacket accepted indefinite-length BER")
	}
	if _, _, err := decodeLength([]byte{0x80, 0x00, 0x00}); err == nil {
		t.Fatal("decodeLength accepted indefinite length")
	}
}

func TestDecodeLDAPPacketRejectsTrailingDataAfterControls(t *testing.T) {
	content := append(encodeInt(1), byte(opUnbindRequest), 0x00)
	content = append(content, 0xa0, 0x00, 0x00)
	pdu := append([]byte{tagSequence}, encodeLength(len(content))...)
	pdu = append(pdu, content...)
	if _, _, _, err := decodeLDAPPacket(pdu); err == nil {
		t.Fatal("decodeLDAPPacket accepted trailing data after controls")
	}
}

func TestDecodeControlRejectsInvalidLDAPOID(t *testing.T) {
	for _, oid := range []string{"", "pagedResults", "1.", ".1", "1..2", "1.2.a"} {
		t.Run(oid, func(t *testing.T) {
			if _, err := decodeControl(encodeOctetString(oid)); err == nil {
				t.Fatal("decodeControl accepted invalid LDAPOID")
			}
		})
	}
	if ctrl, err := decodeControl(encodeOctetString("1.2.840.113556.1.4.319")); err != nil || ctrl.Type != "1.2.840.113556.1.4.319" {
		t.Fatalf("decodeControl valid LDAPOID = %+v, %v", ctrl, err)
	}
}

func TestDecodeLengthRejectsOversizedLengthForms(t *testing.T) {
	if _, _, err := decodeLength([]byte{0x85, 0x00, 0x00, 0x00, 0x00, 0x01}); err == nil {
		t.Fatal("decodeLength accepted overlong length-of-length")
	}
	if _, _, err := decodeLength([]byte{0x84, 0x01, 0x00, 0x00, 0x01}); err == nil {
		t.Fatal("decodeLength accepted length above maxBERMessageSize")
	}
}

func TestEncodeOctetStringUsesLongFormLength(t *testing.T) {
	value := strings.Repeat("x", 300)
	encoded := encodeOctetString(value)
	if len(encoded) < 4 || encoded[0] != tagOctetString || encoded[1] != 0x82 {
		prefixLen := len(encoded)
		if prefixLen > 4 {
			prefixLen = 4
		}
		t.Fatalf("encoded octet string prefix = %x, want long-form length", encoded[:prefixLen])
	}
	got, rest, err := decodeOctetString(encoded)
	if err != nil {
		t.Fatalf("decodeOctetString returned error: %v", err)
	}
	if got != value || len(rest) != 0 {
		t.Fatalf("decodeOctetString = len %d rest %d, want len %d rest 0", len(got), len(rest), len(value))
	}
}

func TestBindRequestEncodeUsesLongFormLength(t *testing.T) {
	req := bindRequest{
		version: 3,
		name:    "uid=" + strings.Repeat("a", 180) + ",dc=example,dc=com",
		auth:    simpleAuth(strings.Repeat("s", 180)),
	}
	encoded, err := req.encode()
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	if len(encoded) < 3 || encoded[0] != opBindRequest || encoded[1] != 0x82 {
		prefixLen := len(encoded)
		if prefixLen > 4 {
			prefixLen = 4
		}
		t.Fatalf("bind request prefix = %x, want application long-form length", encoded[:prefixLen])
	}
	decoded, err := decodeBindRequest(encoded)
	if err != nil {
		t.Fatalf("decodeBindRequest returned error: %v", err)
	}
	if decoded.name != req.name || string(decoded.auth) != string(req.auth) {
		t.Fatalf("decoded bind request name/auth lengths = %d/%d, want %d/%d", len(decoded.name), len(decoded.auth), len(req.name), len(req.auth))
	}
}

func TestEscapeLDAPValue(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"hello", "hello"},
		{"te*st", "te\\2ast"},
		{"te(st)", "te\\28st\\29"},
		{"te\\st", "te\\5cst"},
		{"te\x00st", "te\\00st"},
	}

	for _, tt := range tests {
		got := escapeLDAPValue(tt.input)
		if got != tt.expected {
			t.Errorf("escapeLDAPValue(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
