package ldapgw

import (
	"fmt"
	"strings"
)

func encodeControlErrorResponse(msgID int, opTag int, resultCode int, message string) []byte {
	switch opTag {
	case opBindRequest:
		return encodeBindResponse(msgID, resultCode, "", message)
	case opSearchRequest:
		return encodeSearchResultDone(msgID, resultCode, "", message)
	case opExtendedRequest:
		return encodeExtendedResponse(msgID, resultCode, "", message)
	case opModifyRequest, opAddRequest, opDeleteRequest, opModDNRequest:
		return encodeReadOnlyWriteResponse(msgID, opTag, resultCode, "", message)
	default:
		return encodeLDAPResponse(msgID, opTag, encodeLDAPResult(resultCode, "", message))
	}
}

func decodeAbandonRequestMessageID(opData []byte) (int, bool) {
	if len(opData) == 0 {
		return 0, false
	}
	if opData[0] == tagInteger {
		target, rest, err := decodeInt(opData)
		return target, err == nil && len(rest) == 0 && target > 0 && target <= ldapMaxMessageID
	}
	if len(opData) > 5 {
		return 0, false
	}
	if len(opData) > 0 && opData[0]&0x80 != 0 {
		return 0, false
	}
	var target int
	for _, b := range opData {
		target = target<<8 | int(b)
	}
	return target, target > 0 && target <= ldapMaxMessageID
}

// parsePDULength is the original two-value wrapper kept for callers that do
// not need the error (e.g. tests that build PDUs they know are valid).
func parsePDULength(data []byte) (pduLen int, headerLen int) {
	pduLen, headerLen, _ = parsePDULengthWithError(data)
	return
}

// parsePDULengthWithError returns an error when the BER-declared length
// exceeds maxBERMessageSize.
func parsePDULengthWithError(data []byte) (pduLen int, headerLen int, err error) {
	if len(data) < 2 || data[0] != tagSequence {
		return 0, 0, nil
	}
	length, rest, decErr := decodeLength(data[1:])
	if decErr != nil {
		if strings.Contains(decErr.Error(), "too short") {
			return 0, 0, nil
		}
		return 0, 0, decErr
	}
	if length > maxBERMessageSize {
		return 0, 0, fmt.Errorf("BER message size %d exceeds maximum %d", length, maxBERMessageSize)
	}
	headerLen = len(data) - len(rest)
	if length < 128 {
		return length, headerLen, nil
	}
	numLenBytes := int(data[1] & 0x7f)
	if len(data) < 2+numLenBytes {
		return 0, 0, nil
	}
	raw := data[2 : 2+numLenBytes]
	pduLen = 0
	for i := 0; i < numLenBytes; i++ {
		pduLen = pduLen<<8 | int(raw[i])
	}
	if pduLen > maxBERMessageSize {
		return 0, 0, fmt.Errorf("BER message size %d exceeds maximum %d", pduLen, maxBERMessageSize)
	}
	return pduLen, 1 + 1 + numLenBytes, nil
}

func decodeSearchRequest(data []byte) (baseDN string, scope int, filter []byte, attrs []string, sizeLimit int, timeLimit int, typesOnly bool, err error) {
	if len(data) < 2 {
		err = fmt.Errorf("search request too short")
		return
	}

	baseDN, rest, err := decodeOctetString(data)
	if err != nil {
		err = fmt.Errorf("decode baseDN: %w", err)
		return
	}
	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing scope")
		return
	}

	scopeVal, rest, err := decodeLDAPIntLike(rest)
	if err != nil {
		err = fmt.Errorf("decode scope: %w", err)
		return
	}
	switch scopeVal {
	case scopeBaseObject, scopeSingleLevel, scopeWholeSubtree:
	default:
		err = fmt.Errorf("search request: invalid scope %d", scopeVal)
		return
	}
	scope = scopeVal

	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing derefAliases")
		return
	}
	derefAliases, rest, err := decodeLDAPIntLike(rest)
	if err != nil {
		err = fmt.Errorf("decode derefAliases: %w", err)
		return
	}
	if derefAliases < derefAliasesNever || derefAliases > derefAliasesAlways {
		err = fmt.Errorf("search request: invalid derefAliases %d", derefAliases)
		return
	}
	sizeLimit, rest, err = decodeInt(rest)
	if err != nil {
		err = fmt.Errorf("decode sizeLimit: %w", err)
		return
	}
	if sizeLimit < 0 {
		err = fmt.Errorf("search request: negative sizeLimit %d", sizeLimit)
		return
	}
	timeLimit, rest, err = decodeInt(rest)
	if err != nil {
		err = fmt.Errorf("decode timeLimit: %w", err)
		return
	}
	if timeLimit < 0 {
		err = fmt.Errorf("search request: negative timeLimit %d", timeLimit)
		return
	}
	typesOnly, rest, err = decodeBoolean(rest)
	if err != nil {
		err = fmt.Errorf("decode typesOnly: %w", err)
		return
	}

	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing filter")
		return
	}
	filterLen, filterRest, err := decodeLength(rest[1:])
	if err != nil {
		err = fmt.Errorf("decode filter length: %w", err)
		return
	}
	filterHeaderLen := len(rest) - len(filterRest)
	if len(rest) < filterHeaderLen+filterLen {
		err = fmt.Errorf("search request: truncated filter")
		return
	}
	filter = rest[:filterHeaderLen+filterLen]
	rest = rest[filterHeaderLen+filterLen:]
	var attrRest []byte
	attrs, attrRest, err = decodeAttributeDescriptionList(rest)
	if err != nil {
		err = fmt.Errorf("decode attributes: %w", err)
		return
	}
	if len(attrRest) != 0 {
		err = fmt.Errorf("search request trailing data")
		return
	}

	return
}

func decodeLDAPIntLike(data []byte) (int, []byte, error) {
	if len(data) < 2 {
		return 0, nil, fmt.Errorf("integer/enumerated data too short")
	}
	if data[0] == tagInteger {
		return decodeInt(data)
	}
	if data[0] != 0x0A {
		return 0, nil, fmt.Errorf("invalid integer/enumerated tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return 0, nil, err
	}
	if len(rest) < length {
		return 0, nil, fmt.Errorf("enumerated data too short")
	}
	var v int64
	if length > 0 && rest[0]&0x80 != 0 {
		v = -1
	}
	for i := 0; i < length; i++ {
		v = v<<8 | int64(rest[i])
	}
	return int(v), rest[length:], nil
}

func decodeBoolean(data []byte) (bool, []byte, error) {
	if len(data) < 3 || data[0] != tagBoolean {
		return false, nil, fmt.Errorf("invalid boolean tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return false, nil, err
	}
	if length != 1 || len(rest) < 1 {
		return false, nil, fmt.Errorf("invalid boolean length")
	}
	return rest[0] != 0, rest[1:], nil
}

func decodeSequence(data []byte) ([][]byte, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return nil, fmt.Errorf("not a sequence")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return nil, err
	}
	var result [][]byte
	pos := 0
	for pos < len(content) {
		if content[pos]&0x3f == 0x30 {
			elemLen, elemRest, err := decodeLength(content[pos+1:])
			elemHeaderLen := len(content[pos+1:]) - len(elemRest)
			if err != nil || len(elemRest) < elemLen {
				break
			}
			elem := elemRest[:elemLen]
			result = append(result, elem)
			pos += 1 + elemHeaderLen + elemLen
		} else if content[pos]&0x80 != 0 && content[pos]&0x40 != 0 {
			// Context-specific constructed tag (e.g., LDAP filter 0x83).
			elemLen, elemRest, err := decodeLength(content[pos+1:])
			elemHeaderLen := len(content[pos+1:]) - len(elemRest)
			if err != nil || len(elemRest) < elemLen {
				break
			}
			elem := elemRest[:elemLen]
			result = append(result, elem)
			pos += 1 + elemHeaderLen + elemLen
		} else if content[pos] == tagInteger || content[pos] == tagOctetString || content[pos] == 0x0A {
			elemLen, elemRest, err := decodeLength(content[pos+1:])
			elemHeaderLen := len(content[pos+1:]) - len(elemRest)
			if err != nil || len(elemRest) < elemLen {
				break
			}
			elem := elemRest[:elemLen]
			result = append(result, elem)
			pos += 1 + elemHeaderLen + elemLen
		} else {
			break
		}
	}
	return result, nil
}

func decodeAttributeDescriptionList(data []byte) ([]string, []byte, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return nil, data, fmt.Errorf("attribute list missing")
	}
	contentLen, restAfterLen, err := decodeLength(data[1:])
	if err != nil {
		return nil, data, err
	}
	totalLen := len(data) - len(restAfterLen) + contentLen
	if len(data) < totalLen {
		return nil, data, fmt.Errorf("attribute list truncated")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return nil, data, err
	}
	var attrs []string
	pos := 0
	for pos < len(content) {
		if content[pos] == tagOctetString {
			attrLen, attrRest, err := decodeLength(content[pos+1:])
			if err != nil {
				return nil, data, err
			}
			attrHeaderLen := len(content[pos+1:]) - len(attrRest)
			if len(attrRest) < attrLen {
				return nil, data, fmt.Errorf("attribute description truncated")
			}
			attr := string(attrRest[:attrLen])
			if strings.TrimSpace(attr) == "" || ldapAttributeType(attr) == "" {
				return nil, data, fmt.Errorf("attribute description empty")
			}
			attrs = append(attrs, attr)
			pos += 1 + attrHeaderLen + attrLen
		} else {
			return nil, data, fmt.Errorf("malformed attribute description")
		}
	}
	return attrs, data[totalLen:], nil
}

func readRawTLV(data []byte) ([]byte, []byte, error) {
	if len(data) < 2 {
		return nil, nil, fmt.Errorf("truncated TLV")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return nil, nil, err
	}
	headerLen := len(data) - len(rest)
	totalLen := headerLen + length
	if len(data) < totalLen {
		return nil, nil, fmt.Errorf("truncated TLV content")
	}
	return data[:totalLen], data[totalLen:], nil
}

func decodeTaggedContent(data []byte, tag byte) ([]byte, []byte, error) {
	if len(data) < 2 || data[0] != tag {
		return nil, nil, fmt.Errorf("invalid tag")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return nil, nil, err
	}
	if len(rest) < length {
		return nil, nil, fmt.Errorf("content too short")
	}
	return rest[:length], rest[length:], nil
}

func encodeOctetStringBytes(value []byte) []byte {
	var out []byte
	out = append(out, tagOctetString)
	out = append(out, encodeLength(len(value))...)
	out = append(out, value...)
	return out
}

func decodeSubstringParts(data []byte) ([]string, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return nil, fmt.Errorf("substring filter missing sequence")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return nil, err
	}
	var parts []string
	for len(content) > 0 {
		tag := content[0]
		if tag != 0x80 && tag != 0x81 && tag != 0x82 {
			return nil, fmt.Errorf("unsupported substring choice tag 0x%02x", tag)
		}
		length, rest, err := decodeLength(content[1:])
		if err != nil {
			return nil, err
		}
		if len(rest) < length {
			return nil, fmt.Errorf("substring value truncated")
		}
		parts = append(parts, string(rest[:length]))
		content = rest[length:]
	}
	return parts, nil
}

type extensibleMatchAssertion struct {
	Attr         string
	Value        string
	MatchingRule string
	DNAttributes bool
}

func decodeExtensibleMatch(content []byte) (attr string, value string, ok bool, err error) {
	match, ok, err := decodeExtensibleMatchDetail(content)
	return match.Attr, match.Value, ok, err
}

func decodeExtensibleMatchDetail(content []byte) (extensibleMatchAssertion, bool, error) {
	var match extensibleMatchAssertion
	for len(content) > 0 {
		tag := content[0]
		length, rest, err := decodeLength(content[1:])
		if err != nil {
			return extensibleMatchAssertion{}, false, err
		}
		if len(rest) < length {
			return extensibleMatchAssertion{}, false, fmt.Errorf("extensibleMatch value truncated")
		}
		val := string(rest[:length])
		switch tag {
		case 0x82: // type [2] AttributeDescription
			match.Attr = val
		case 0x83: // matchValue [3] AssertionValue
			match.Value = val
		case 0x81: // matchingRule [1]
			match.MatchingRule = val
		case 0x84: // dnAttributes [4]
			match.DNAttributes = len(rest[:length]) > 0 && rest[0] != 0
		default:
			return extensibleMatchAssertion{}, false, fmt.Errorf("unsupported extensibleMatch choice tag 0x%02x", tag)
		}
		content = rest[length:]
	}
	return match, strings.TrimSpace(match.Attr) != "" && strings.TrimSpace(match.Value) != "", nil
}

const startTLSOID = "1.3.6.1.4.1.1466.20037"
const whoAmIOID = "1.3.6.1.4.1.4203.1.11.3"

func isStartTLSRequest(data []byte) bool {
	name, err := decodeExtendedRequestName(data)
	return err == nil && name == startTLSOID
}

func decodeExtendedRequestName(data []byte) (string, error) {
	if len(data) < 2 || data[0] != 0x80 {
		return "", fmt.Errorf("extended request missing requestName")
	}
	length, rest, err := decodeLength(data[1:])
	if err != nil {
		return "", err
	}
	if len(rest) < length {
		return "", fmt.Errorf("extended requestName truncated")
	}
	name := string(rest[:length])
	if !isNumericLDAPOID(name) {
		return "", fmt.Errorf("extended requestName invalid LDAPOID")
	}
	rest = rest[length:]
	if len(rest) == 0 {
		return name, nil
	}
	if rest[0] != 0x81 {
		return "", fmt.Errorf("extended request has invalid trailing data")
	}
	valueLen, valueRest, err := decodeLength(rest[1:])
	if err != nil {
		return "", err
	}
	if len(valueRest) != valueLen {
		return "", fmt.Errorf("extended requestValue length mismatch")
	}
	return name, nil
}

func mustEncodeNotSupported() []byte {
	var content []byte
	content = append(content, encodeEnumerated(resultUnwillingToPerform)...)
	content = append(content, encodeOctetString("")...)
	content = append(content, encodeOctetString("operation not supported")...)
	return content
}
