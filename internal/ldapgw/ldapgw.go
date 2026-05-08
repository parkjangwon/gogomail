package ldapgw

import (
	"bytes"
	"encoding/asn1"
	"fmt"
	"strings"
)

const (
	opBindRequest     = 0
	opBindResponse    = 1
	opUnbindRequest   = 2
	opSearchRequest   = 3
	opSearchResultEntry = 4
	opSearchResultDone  = 5
	opModifyRequest   = 6
	opAddRequest      = 8
	opDeleteRequest   = 10
	opModDNRequest    = 12
	opCompareRequest  = 14
	opAbandonRequest  = 16
	ldapV3            = 3
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
	buf.WriteByte(0x02)
	vBytes := encodeInt(r.version)
	buf.Write(encodeLength(len(vBytes)))
	buf.Write(vBytes)
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
		case '(': buf.WriteString("\\28")
		case ')': buf.WriteString("\\29")
		case '\\': buf.WriteString("\\5c")
		case '\x00': buf.WriteString("\\00")
		default:
			buf.WriteRune(c)
		}
	}
	return buf.String()
}

func encodeInt(v int) []byte {
	if v < 128 {
		return []byte{byte(v)}
	}
	b, _ := asn1.Marshal(v)
	return b[2:]
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
		return "", nil, fmt.Errorf("invalid octet string tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return "", nil, err
	}
	if len(rest) < length {
		return "", nil, fmt.Errorf("octet string too short")
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
