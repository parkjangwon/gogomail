package ldapgw

import (
	"bytes"
	"encoding/asn1"
	"fmt"
	"strings"
)

// RFC 4511 operation codes (APPLICATION class tags: 0x20 | tag-number)
const (
	opBindRequest       = 0x60 // APPLICATION 0
	opBindResponse      = 0x61 // APPLICATION 1
	opUnbindRequest     = 0x42 // APPLICATION 2
	opSearchRequest     = 0x63 // APPLICATION 3
	opSearchResultEntry = 0x64 // APPLICATION 4
	opSearchResultDone  = 0x65 // APPLICATION 5
	opModifyRequest     = 0x66 // APPLICATION 6
	opAddRequest        = 0x68 // APPLICATION 8
	opDeleteRequest     = 0x4a // APPLICATION 10
	opModDNRequest      = 0x6c // APPLICATION 12
	opCompareRequest    = 0x6e // APPLICATION 14
	opAbandonRequest    = 0x50 // APPLICATION 16
	opExtendedRequest   = 0x77 // APPLICATION 23
	ldapV3              = 3
)

// LDAP result codes (from RFC 4511)
const (
	resultSuccess           = 0
	resultAuthMethodNotSupported = 48
	resultInvalidCredentials = 49
	resultUnwillingToPerform = 53
	resultNoSuchObject      = 32
	resultSizeLimitExceeded  = 4
)

// BER tag constants
const (
	tagBoolean        = 0x01
	tagInteger        = 0x02
	tagOctetString    = 0x04
	tagSequence      = 0x30
	tagContextSpecific = 0x80
)

// Search scope constants
const (
	scopeBaseObject   = 0
	scopeSingleLevel  = 1
	scopeWholeSubtree = 2
)

// LDAPFilter map from RFC 4511
const (
	filterAnd           = 0
	filterOr            = 1
	filterNot           = 2
	filterEqualityMatch  = 3
	filterSubstrings    = 4
	filterGreaterOrEqual = 5
	filterLessOrEqual   = 6
	filterPresent       = 7
	filterApproxMatch   = 8
)

type bindRequest struct {
	version int
	name    string
	auth    []byte
}

func simpleAuth(password string) []byte {
	return []byte(password)
}

func (r *bindRequest) encode() ([]byte, error) {
	var buf bytes.Buffer
	buf.Write(encodeInt(r.version))
	buf.Write(encodeOctetString(r.name))
	buf.WriteByte(0x80)
	buf.Write(encodeLength(len(r.auth)))
	buf.Write(r.auth)
	return append([]byte{0x60, byte(buf.Len())}, buf.Bytes()...), nil
}

func decodeBindRequest(data []byte) (*bindRequest, error) {
	if len(data) < 2 || data[0] != 0x60 {
		return nil, fmt.Errorf("invalid bind request tag")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return nil, err
	}
	if len(content) < 3 {
		return nil, fmt.Errorf("bind request too short")
	}
	version, rest, err := decodeInt(content)
	if err != nil {
		return nil, err
	}
	name, rest, err := decodeOctetString(rest)
	if err != nil {
		return nil, err
	}
	if len(rest) < 2 || rest[0] != 0x80 {
		return nil, fmt.Errorf("invalid auth tag")
	}
	authLen, rest, err := decodeLength(rest[1:])
	if err != nil {
		return nil, err
	}
	if len(rest) < authLen {
		return nil, fmt.Errorf("auth data too short")
	}
	return &bindRequest{
		version: version,
		name:    name,
		auth:    rest[:authLen],
	}, nil
}

func isReadOnlyOperation(op int) bool {
	switch op {
	case opBindRequest, opSearchRequest, opUnbindRequest, opCompareRequest, opAbandonRequest:
		return true
	default:
		return false
	}
}

func escapeLDAPValue(value string) string {
	var buf strings.Builder
	for _, c := range value {
		switch c {
		case '*':
			buf.WriteString("\\2a")
		case '(':
			buf.WriteString("\\28")
		case ')':
			buf.WriteString("\\29")
		case '\\':
			buf.WriteString("\\5c")
		case '\x00':
			buf.WriteString("\\00")
		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

func encodeInt(v int) []byte {
	if v < 128 {
		return []byte{tagInteger, 0x01, byte(v)}
	}
	b, _ := asn1.Marshal(v)
	return b
}

func decodeInt(data []byte) (int, []byte, error) {
	if len(data) < 2 || data[0] != 0x02 {
		return 0, nil, fmt.Errorf("invalid int tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return 0, nil, err
	}
	if len(rest) < length {
		return 0, nil, fmt.Errorf("int data too short")
	}
	var v int64
	for i := 0; i < length; i++ {
		v = v<<8 | int64(rest[i])
	}
	return int(v), rest[length:], nil
}

func encodeOctetString(s string) []byte {
	b := []byte(s)
	return append([]byte{0x04, byte(len(b))}, b...)
}

func decodeOctetString(data []byte) (string, []byte, error) {
	if len(data) < 2 || data[0] != 0x04 {
		return "", nil, fmt.Errorf("invalid octet String tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return "", nil, err
	}
	if len(rest) < length {
		return "", nil, fmt.Errorf("octet String too short")
	}
	return string(rest[:length]), rest[length:], nil
}

func encodeLength(length int) []byte {
	if length < 128 {
		return []byte{byte(length)}
	}
	var buf bytes.Buffer
	tmp := length
	for tmp > 0 {
		buf.WriteByte(byte(tmp & 0xff))
		tmp >>= 8
	}
	b := buf.Bytes()
	for i, j := 0, len(b)-1; i < j; i, j = i+1, j-1 {
		b[i], b[j] = b[j], b[i]
	}
	return append([]byte{0x80 | byte(len(b))}, b...)
}

func decodeLength(data []byte) (int, []byte, error) {
	if len(data) == 0 {
		return 0, nil, fmt.Errorf("empty length")
	}
	if data[0]&0x80 == 0 {
		return int(data[0]), data[1:], nil
	}
	numBytes := int(data[0] & 0x7f)
	if len(data) < numBytes+1 {
		return 0, nil, fmt.Errorf("length data too short")
	}
	length := 0
	for i := 0; i < numBytes; i++ {
		length = length<<8 | int(data[1+i])
	}
	return length, data[1+numBytes:], nil
}

func decodeContent(data []byte) ([]byte, error) {
	length, rest, err := decodeLength(data)
	if err != nil {
		return nil, err
	}
	if len(rest) < length {
		return nil, fmt.Errorf("content too short")
	}
	return rest[:length], nil
}

// decodeLDAPPacket decodes a full LDAP PDU.
// LDAP PDU format: SEQUENCE { messageID INTEGER, operation [operation-specific] }
// Returns messageID, operation tag, operation data.
func decodeLDAPPacket(pdu []byte) (messageID int, opTag int, opData []byte, err error) {
	if len(pdu) < 2 || pdu[0] != tagSequence {
		return 0, 0, nil, fmt.Errorf("invalid LDAP PDU tag: expected 0x30, got 0x%02x", pdu[0])
	}
	content, err := decodeContent(pdu[1:])
	if err != nil {
		return 0, 0, nil, fmt.Errorf("decode PDU content: %w", err)
	}
	msgContent := content

	// First element is messageID (INTEGER)
	if len(msgContent) < 2 || msgContent[0] != tagInteger {
		return 0, 0, nil, fmt.Errorf("missing messageID")
	}
	msgIDLen, msgIDRest, err := decodeLength(msgContent[1:])
	if err != nil {
		return 0, 0, nil, err
	}
	if len(msgIDRest) < msgIDLen {
		return 0, 0, nil, fmt.Errorf("messageID data too short")
	}
	var v int64
	for i := 0; i < msgIDLen; i++ {
		v = v<<8 | int64(msgIDRest[i])
	}
	messageID = int(v)
	msgContent = msgIDRest[msgIDLen:]

	if len(msgContent) < 2 {
		return messageID, 0, nil, fmt.Errorf("missing operation in PDU")
	}
	opTag = int(msgContent[0])
	if len(msgContent) < 2 {
		return messageID, opTag, nil, fmt.Errorf("operation data missing")
	}
	opData = msgContent[2:]
	return messageID, opTag, opData, nil
}

// encodeLDAPResponse encodes a full LDAP response PDU.
func encodeLDAPResponse(messageID int, opTag int, opData []byte) []byte {
	// Build operation-specific content
	var opContent bytes.Buffer
	opContent.WriteByte(byte(opTag))
	opContent.Write(encodeLength(len(opData)))
	opContent.Write(opData)

	// messageID INTEGER
	var msgIDContent bytes.Buffer
	msgIDContent.WriteByte(tagInteger)
	msgIDContent.Write(encodeLength(1))
	msgIDContent.WriteByte(byte(messageID))

	// SEQUENCE { msgID, opContent }
	var seqContent bytes.Buffer
	seqContent.Write(msgIDContent.Bytes())
	seqContent.Write(opContent.Bytes())

	result := make([]byte, 0, 2+len(seqContent.Bytes()))
	result = append(result, tagSequence)
	result = append(result, encodeLength(len(seqContent.Bytes()))...)
	result = append(result, seqContent.Bytes()...)
	return result
}

// encodeSearchResultEntry encodes a SearchResultEntry PDU.
// dn is the distinguished name, attrs is a map of attribute names to values.
func encodeSearchResultEntry(messageID int, dn string, attrs map[string][]string) ([]byte, error) {
	var attrSeq bytes.Buffer
	for name, values := range attrs {
		// AttributeDescription SEQUENCE { type OCTETSTRING, vals SET OF OCTETSTRING }
		var typeAndVals bytes.Buffer
		typeAndVals.Write(encodeOctetString(name))
		// Values SET
		var valsSet bytes.Buffer
		for _, v := range values {
			valsSet.Write(encodeOctetString(v))
		}
		// Prepend SET tag
		valsWithTag := append([]byte{0x31, byte(len(valsSet.Bytes()))}, valsSet.Bytes()...)
		typeAndVals.Write(valsWithTag)

		encodedAttr := make([]byte, 0, 2+len(typeAndVals.Bytes()))
		encodedAttr = append(encodedAttr, 0x30) // SEQUENCE
		encodedAttr = append(encodedAttr, encodeLength(len(typeAndVals.Bytes()))...)
		encodedAttr = append(encodedAttr, typeAndVals.Bytes()...)
		attrSeq.Write(encodedAttr)
	}

	// Partial build of SearchResultEntry SEQUENCE
	var entryContent bytes.Buffer
	entryContent.Write(encodeOctetString(dn))
	entryContent.Write(attrSeq.Bytes())

	entryEncoded := make([]byte, 0, 2+len(entryContent.Bytes()))
	entryEncoded = append(entryEncoded, 0x30)
	entryEncoded = append(entryEncoded, encodeLength(len(entryContent.Bytes()))...)
	entryEncoded = append(entryEncoded, entryContent.Bytes()...)

	return encodeLDAPResponse(messageID, opSearchResultEntry, entryEncoded), nil
}

// encodeSearchResultDone encodes a SearchResultDone response.
func encodeSearchResultDone(messageID int, resultCode int, matchedDN, errorMessage string) []byte {
	var resultContent bytes.Buffer
	resultContent.Write(encodeEnumerated(resultCode))
	resultContent.Write(encodeOctetString(matchedDN))
	resultContent.Write(encodeOctetString(errorMessage))
	return encodeLDAPResponse(messageID, opSearchResultDone, resultContent.Bytes())
}

// encodeBindResponse encodes a BindResponse PDU.
func encodeBindResponse(messageID int, resultCode int, matchedDN, errorMessage string) []byte {
	var respContent bytes.Buffer
	respContent.Write(encodeEnumerated(resultCode))
	respContent.Write(encodeOctetString(matchedDN))
	respContent.Write(encodeOctetString(errorMessage))
	return encodeLDAPResponse(messageID, opBindResponse, respContent.Bytes())
}

func encodeEnumerated(v int) []byte {
	// BER: 0x0A is ENUMERATED
	if v < 128 {
		return []byte{0x0A, 0x01, byte(v)}
	}
	b, _ := asn1.Marshal(asn1.Enumerated(v))
	return b
}

func decodeEnumerated(data []byte) int {
	if len(data) < 3 || data[0] != 0x0A {
		return -1
	}
	return int(data[2])
}
