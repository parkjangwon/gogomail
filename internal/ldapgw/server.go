package ldapgw

import (
	"context"
	"fmt"
	"net"
	"sync"
)

// maxBERMessageSize is the maximum allowed BER-encoded PDU body size (16 MB).
const maxBERMessageSize = 16 * 1024 * 1024

type LDAPAuthenticator interface {
	AuthenticateLDAP(ctx context.Context, username, password string) (bool, error)
}

type DirectoryQuerier interface {
	SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error)
}

type PrincipalEntry struct {
	DN          string
	CN          string
	Mail        string
	UID         string
	DisplayName string
	GivenName   string
	SN          string
}

type DirectorySearchRequest struct {
	BaseDN string
	Scope  int
	Filter string
	Attrs  []string
	Limit  int
}

type LDAPServer struct {
	ln      net.Listener
	auth    LDAPAuthenticator
	quer    DirectoryQuerier
	closed  bool
	closeMu sync.Mutex
	ctx     context.Context
	cancel  context.CancelFunc
}

func NewServer(ln net.Listener, auth LDAPAuthenticator, quer DirectoryQuerier) *LDAPServer {
	ctx, cancel := context.WithCancel(context.Background())
	return &LDAPServer{ln: ln, auth: auth, quer: quer, ctx: ctx, cancel: cancel}
}

func (s *LDAPServer) Serve() error {
	for {
		conn, err := s.ln.Accept()
		if err != nil {
			s.closeMu.Lock()
			if s.closed {
				s.closeMu.Unlock()
				return nil
			}
			s.closeMu.Unlock()
			return err
		}
		go s.handleConn(s.ctx, conn)
	}
}

func (s *LDAPServer) Close() error {
	s.closeMu.Lock()
	s.closed = true
	s.closeMu.Unlock()
	s.cancel()
	return s.ln.Close()
}

func (s *LDAPServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 8192)
	readOffset := 0

	for {
		// Check context before blocking on read.
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := conn.Read(buf[readOffset:])
		if n > 0 {
			readOffset += n
		}
		if err != nil {
			return
		}

		for readOffset > 0 {
			// Check context before processing each PDU.
			select {
			case <-ctx.Done():
				return
			default:
			}

			pduLen, headerLen, pduErr := parsePDULengthWithError(buf[:readOffset])
			if pduErr != nil {
				// Declared length exceeds the safety cap — drop the connection.
				return
			}
			if pduLen == 0 {
				break
			}
			totalLen := headerLen + pduLen
			if readOffset < totalLen {
				break
			}
			pdu := make([]byte, totalLen)
			copy(pdu, buf[:totalLen])
			copy(buf, buf[totalLen:readOffset])
			readOffset -= totalLen

			msgID, opTag, opData, err := decodeLDAPPacket(pdu)
			if err != nil {
				resp := encodeLDAPResponse(0, opTag, mustEncodeNotSupported())
				conn.Write(resp) //nolint:errcheck
				return
			}
			resp := s.handleOperation(ctx, msgID, opTag, opData)
			if len(resp) > 0 {
				if _, err := conn.Write(resp); err != nil {
					return
				}
			}
			if opTag == opUnbindRequest {
				return
			}
		}
	}
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
		return 0, 0, nil
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

func (s *LDAPServer) handleOperation(ctx context.Context, msgID int, opTag int, opData []byte) []byte {
	switch opTag {
	case opBindRequest:
		return s.handleBindRequest(ctx, msgID, opData)
	case opSearchRequest:
		return s.handleSearchRequest(ctx, msgID, opData)
	case opUnbindRequest:
		return nil
	default:
		return encodeLDAPResponse(msgID, opTag, mustEncodeNotSupported())
	}
}

func (s *LDAPServer) handleBindRequest(ctx context.Context, msgID int, opData []byte) []byte {
	select {
	case <-ctx.Done():
		return encodeBindResponse(msgID, resultUnwillingToPerform, "", "operation timed out")
	default:
	}

	req, err := decodeBindRequestData(opData)
	if err != nil {
		return encodeBindResponse(msgID, resultUnwillingToPerform, "", "malformed bind request")
	}
	if req.version != ldapV3 {
		return encodeBindResponse(msgID, resultAuthMethodNotSupported, "", "unsupported LDAP version")
	}

	ok, err := s.auth.AuthenticateLDAP(ctx, req.name, string(req.auth))
	if err != nil || !ok {
		return encodeBindResponse(msgID, resultInvalidCredentials, "", "invalid credentials")
	}
	return encodeBindResponse(msgID, resultSuccess, "", "")
}

func decodeBindRequestData(data []byte) (*bindRequest, error) {
	if len(data) < 2 || data[0] != tagInteger {
		return nil, fmt.Errorf("invalid bind request: expected INTEGER tag")
	}
	content := data
	version, rest, err := decodeInt(content)
	if err != nil {
		return nil, err
	}
	name, rest, err := decodeOctetString(rest)
	if err != nil {
		return nil, err
	}
	auth := []byte{}
	if len(rest) > 0 && rest[0] == 0x80 {
		authLen, authRest, err := decodeLength(rest[1:])
		if err == nil && len(authRest) >= authLen {
			auth = authRest[:authLen]
		}
	}
	return &bindRequest{version: version, name: name, auth: auth}, nil
}

func (s *LDAPServer) handleSearchRequest(ctx context.Context, msgID int, opData []byte) []byte {
	select {
	case <-ctx.Done():
		return encodeSearchResultDone(msgID, resultUnwillingToPerform, "", "operation timed out")
	default:
	}

	baseObject, scope, filter, _, err := decodeSearchRequest(opData)
	if err != nil {
		return encodeSearchResultDone(msgID, resultUnwillingToPerform, "", err.Error())
	}

	if err := validateFilter(filter); err != nil {
		return encodeSearchResultDone(msgID, resultUnwillingToPerform, "", fmt.Sprintf("invalid filter: %v", err))
	}

	ldapFilter, err := parseLDAPFilter(filter)
	if err != nil {
		return encodeSearchResultDone(msgID, resultUnwillingToPerform, "", "malformed filter")
	}

	principals, err := s.quer.SearchPrincipals(ctx, DirectorySearchRequest{
		BaseDN: baseObject,
		Scope:  scope,
		Filter: ldapFilter,
		Limit:  100,
	})
	if err != nil {
		return encodeSearchResultDone(msgID, resultUnwillingToPerform, "", err.Error())
	}

	resp := make([]byte, 0, 4096)
	for _, p := range principals {
		attrMap := map[string][]string{
			"cn":          {p.CN},
			"mail":        {p.Mail},
			"uid":         {p.UID},
			"displayName": {p.DisplayName},
		}
		if p.GivenName != "" {
			attrMap["givenName"] = []string{p.GivenName}
		}
		if p.SN != "" {
			attrMap["sn"] = []string{p.SN}
		}
		entry, err := encodeSearchResultEntry(msgID, p.DN, attrMap)
		if err != nil {
			continue
		}
		resp = append(resp, entry...)
	}
	resp = append(resp, encodeSearchResultDone(msgID, resultSuccess, "", "")...)
	return resp
}

func decodeSearchRequest(data []byte) (baseDN string, scope int, filter []byte, attrs []string, err error) {
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

	scopeVal, rest, err := decodeInt(rest)
	if err != nil {
		err = fmt.Errorf("decode scope: %w", err)
		return
	}
	scope = scopeVal

	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing derefAliases")
		return
	}
	_, rest, err = decodeInt(rest)
	if err != nil {
		err = fmt.Errorf("decode derefAliases: %w", err)
		return
	}

	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing filter")
		return
	}
	filter = rest

	return
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
			elemLen, elemHeader, err := decodeLength(content[pos+1:])
			if err != nil || pos+1+len(content[pos+1:])-len(elemHeader) < elemLen {
				break
			}
			elem := content[pos+1+len(elemHeader) : pos+1+len(elemHeader)+elemLen]
			result = append(result, elem)
			pos += 1 + len(elemHeader) + elemLen
		} else if content[pos]&0x80 != 0 && content[pos]&0x40 != 0 {
			// Context-specific constructed tag (e.g., LDAP filter 0x83).
			elemLen, elemHeader, err := decodeLength(content[pos+1:])
			if err != nil || pos+1+len(content[pos+1:])-len(elemHeader) < elemLen {
				break
			}
			elem := content[pos+1+len(elemHeader) : pos+1+len(elemHeader)+elemLen]
			result = append(result, elem)
			pos += 1 + len(elemHeader) + elemLen
		} else if content[pos] == tagInteger || content[pos] == tagOctetString || content[pos] == 0x0A {
			elemLen, elemHeader, err := decodeLength(content[pos+1:])
			if err != nil || pos+1+len(content[pos+1:])-len(elemHeader) < elemLen {
				break
			}
			elem := content[pos+1+len(elemHeader) : pos+1+len(elemHeader)+elemLen]
			result = append(result, elem)
			pos += 1 + len(elemHeader) + elemLen
		} else {
			break
		}
	}
	return result, nil
}

func decodeAttributeDescriptionList(data []byte) ([]string, []byte, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return nil, data, nil
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return nil, data, err
	}
	var attrs []string
	pos := 0
	for pos < len(content) {
		if content[pos] == tagOctetString {
			attrLen, attrHeader, err := decodeLength(content[pos+1:])
			if err != nil || pos+1+len(content[pos+1:])-len(attrHeader) < attrLen {
				break
			}
			attr := string(content[pos+1+len(attrHeader) : pos+1+len(attrHeader)+attrLen])
			attrs = append(attrs, attr)
			pos += 1 + len(attrHeader) + attrLen
		} else {
			break
		}
	}
	return attrs, data, nil
}

func parseLDAPFilter(data []byte) (string, error) {
	if len(data) == 0 {
		return "", nil
	}
	if data[0] == tagContextSpecific || (data[0]&0x80) != 0 {
		filterType := int(data[0] & 0x3f)
		content, err := decodeContent(data[1:])
		if err != nil {
			return "", err
		}
		switch filterType {
		case filterEqualityMatch:
			if len(content) < 2 || content[0] != tagOctetString {
				return "", fmt.Errorf("malformed equality match")
			}
			attrLen, _, err := decodeLength(content[1:])
			if err != nil {
				return "", err
			}
			rest := content[2:]
			if len(rest) < attrLen {
				return "", fmt.Errorf("truncated attr")
			}
			attr := string(rest[:attrLen])
			valRest := rest[attrLen:]
			if len(valRest) < 2 || valRest[0] != tagOctetString {
				return "", fmt.Errorf("malformed equality value")
			}
			valLen, _, err := decodeLength(valRest[1:])
			if err != nil {
				return "", err
			}
			val := string(valRest[2 : 2+valLen])
			return fmt.Sprintf("(%s=%s)", attr, val), nil
		case filterPresent:
			attr, _, err := decodeOctetString(content)
			if err != nil {
				return "", err
			}
			return fmt.Sprintf("(%s=*)", attr), nil
		default:
			return "", fmt.Errorf("unsupported filter type: %d", filterType)
		}
	}
	return "", nil
}

// validateFilter checks that filter data is well-formed before processing.
// It rejects empty input, non-context-specific tags, unrecognised filter
// types, and content whose declared length exceeds the available bytes.
func validateFilter(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("filter is empty")
	}
	if data[0]&0x80 == 0 {
		return fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	switch filterType {
	case filterAnd, filterOr, filterNot,
		filterEqualityMatch, filterSubstrings,
		filterGreaterOrEqual, filterLessOrEqual,
		filterPresent, filterApproxMatch:
		// valid
	default:
		return fmt.Errorf("unsupported filter type %d", filterType)
	}
	if len(data) < 2 {
		return fmt.Errorf("filter too short")
	}
	declLen, _, err := decodeLength(data[1:])
	if err != nil {
		return fmt.Errorf("filter length: %w", err)
	}
	// Compute how many bytes the tag+length header occupies.
	headerLen := 2 // tag byte + one length byte (short form)
	if data[1]&0x80 != 0 {
		headerLen = 1 + 1 + int(data[1]&0x7f) // tag + 0x8n + n bytes
	}
	if len(data) < headerLen+declLen {
		return fmt.Errorf("filter truncated: need %d bytes after header, have %d", declLen, len(data)-headerLen)
	}
	return nil
}

func mustEncodeNotSupported() []byte {
	var content []byte
	content = append(content, encodeEnumerated(resultUnwillingToPerform)...)
	content = append(content, encodeOctetString("")...)
	content = append(content, encodeOctetString("operation not supported")...)
	return content
}
