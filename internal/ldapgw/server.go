package ldapgw

import (
	"context"
	"crypto/sha1"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
)

// maxBERMessageSize is the maximum allowed BER-encoded PDU body size (16 MB).
const maxBERMessageSize = 16 * 1024 * 1024

const ldapFeatureAllOperationalAttributes = "1.3.6.1.4.1.4203.1.5.1"

const ldapSearchCandidateBatchSize = 100
const ldapMaxCandidateScan = 10000

var errUnsupportedBindAuth = errors.New("unsupported bind authentication choice")

type LDAPAuthenticator interface {
	AuthenticateLDAP(ctx context.Context, username, password string) (bool, error)
}

type DirectoryQuerier interface {
	SearchPrincipals(ctx context.Context, req DirectorySearchRequest) ([]PrincipalEntry, error)
}

type PrincipalEntry struct {
	DN           string
	Kind         string
	CN           string
	Mail         string
	UID          string
	OU           string
	DisplayName  string
	GivenName    string
	SN           string
	ResourceType string
	Members      []string
	MemberOf     []string
}

type ldapSearchEntry struct {
	principal PrincipalEntry
	attrs     map[string][]string
}

type DirectorySearchRequest struct {
	BaseDN string
	Scope  int
	Filter string
	Attrs  []string
	Kinds  []string
	Limit  int
	Offset int
}

type LDAPServer struct {
	ln             net.Listener
	auth           LDAPAuthenticator
	quer           DirectoryQuerier
	tlsConfig      *tls.Config
	namingContexts []string
	referralURLs   []string
	metrics        Metrics
	closed         bool
	closeMu        sync.Mutex
	ctx            context.Context
	cancel         context.CancelFunc
}

type ServerOptions struct {
	TLSConfig      *tls.Config
	NamingContexts []string
	ReferralURLs   []string
	Metrics        Metrics
}

func NewServer(ln net.Listener, auth LDAPAuthenticator, quer DirectoryQuerier) *LDAPServer {
	return NewServerWithOptions(ln, auth, quer, ServerOptions{})
}

func NewServerWithOptions(ln net.Listener, auth LDAPAuthenticator, quer DirectoryQuerier, opts ServerOptions) *LDAPServer {
	ctx, cancel := context.WithCancel(context.Background())
	var tlsConfig *tls.Config
	if opts.TLSConfig != nil {
		tlsConfig = opts.TLSConfig.Clone()
		if tlsConfig.MinVersion == 0 {
			tlsConfig.MinVersion = tls.VersionTLS12
		}
	}
	namingContexts := make([]string, 0, len(opts.NamingContexts))
	for _, dc := range opts.NamingContexts {
		if dc = strings.TrimSpace(dc); dc != "" {
			namingContexts = append(namingContexts, dc)
		}
	}
	referralURLs := make([]string, 0, len(opts.ReferralURLs))
	for _, u := range opts.ReferralURLs {
		if u = strings.TrimSpace(u); u != "" {
			referralURLs = append(referralURLs, u)
		}
	}
	return &LDAPServer{
		ln:             ln,
		auth:           auth,
		quer:           quer,
		tlsConfig:      tlsConfig,
		namingContexts: namingContexts,
		referralURLs:   referralURLs,
		metrics:        metricsOrDefault(opts.Metrics),
		ctx:            ctx,
		cancel:         cancel,
	}
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
	buf := make([]byte, 0, 8192)
	readBuf := make([]byte, 8192)
	tlsActive := false
	authenticated := false
	authzID := ""

	for {
		// Check context before blocking on read.
		select {
		case <-ctx.Done():
			return
		default:
		}

		n, err := conn.Read(readBuf)
		if n > 0 {
			buf = append(buf, readBuf[:n]...)
			if len(buf) > maxBERMessageSize+8 {
				return
			}
		}
		if err != nil {
			return
		}

		for len(buf) > 0 {
			// Check context before processing each PDU.
			select {
			case <-ctx.Done():
				return
			default:
			}

			pduLen, headerLen, pduErr := parsePDULengthWithError(buf)
			if pduErr != nil {
				// Declared length exceeds the safety cap — drop the connection.
				return
			}
			if pduLen == 0 {
				break
			}
			totalLen := headerLen + pduLen
			if len(buf) < totalLen {
				break
			}
			pdu := make([]byte, totalLen)
			copy(pdu, buf[:totalLen])
			buf = buf[totalLen:]

			msgID, opTag, opData, controls, err := decodeLDAPPacketWithControls(pdu)
			if err != nil {
				resp := encodeLDAPResponse(0, opTag, mustEncodeNotSupported())
				conn.Write(resp) //nolint:errcheck
				s.observe(ctx, opTag, resultUnwillingToPerform, 0, conn.RemoteAddr(), err)
				return
			}
			if ctrl, ok := firstUnsupportedCriticalControl(controls); ok {
				result := resultUnavailableCriticalExtension
				resp := encodeControlErrorResponse(msgID, opTag, result, "unsupported critical control: "+ctrl.Type)
				if _, err := conn.Write(resp); err != nil {
					return
				}
				s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
				continue
			}
			if opTag == opExtendedRequest && isStartTLSRequest(opData) {
				if tlsActive {
					result := resultOperationsError
					resp := encodeExtendedResponse(msgID, result, "", "TLS already active")
					if _, err := conn.Write(resp); err != nil {
						return
					}
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					continue
				}
				if s.tlsConfig == nil {
					result := resultUnavailable
					resp := encodeExtendedResponse(msgID, result, "", "StartTLS not configured")
					if _, err := conn.Write(resp); err != nil {
						return
					}
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					continue
				}
				if len(buf) != 0 {
					result := resultProtocolError
					resp := encodeExtendedResponse(msgID, result, "", "StartTLS must be the last plaintext operation")
					conn.Write(resp) //nolint:errcheck
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					return
				}
				result := resultSuccess
				resp := encodeExtendedResponse(msgID, result, "", "")
				if _, err := conn.Write(resp); err != nil {
					return
				}
				s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
				tlsConn := tls.Server(conn, s.tlsConfig.Clone())
				if err := tlsConn.HandshakeContext(ctx); err != nil {
					return
				}
				conn = tlsConn
				tlsActive = true
				continue
			}
			resp, resultCode, entries, authOK := s.handleOperation(ctx, msgID, opTag, opData, controls, authenticated, authzID)
			if opTag == opBindRequest {
				authenticated = false
				authzID = ""
				if authOK {
					authenticated = true
					if req, err := decodeBindRequestData(opData); err == nil {
						authzID = "dn:" + req.name
					}
				}
			}
			if len(resp) > 0 {
				if _, err := conn.Write(resp); err != nil {
					return
				}
			}
			s.observe(ctx, opTag, resultCode, entries, conn.RemoteAddr(), nil)
			if opTag == opUnbindRequest {
				return
			}
		}
	}
}

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

func (s *LDAPServer) handleOperation(ctx context.Context, msgID int, opTag int, opData []byte, controls []control, authenticated bool, authzID string) ([]byte, int, int, bool) {
	switch opTag {
	case opBindRequest:
		resp, result := s.handleBindRequest(ctx, msgID, opData)
		return resp, result, 0, result == resultSuccess
	case opSearchRequest:
		if !authenticated && !isPublicDiscoverySearch(opData) {
			result := resultInsufficientAccessRights
			return encodeSearchResultDone(msgID, result, "", "bind required"), result, 0, false
		}
		if err := authorizeProxiedAuthorization(controls, authzID); err != nil {
			result := resultAuthorizationDenied
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0, false
		}
		resp, result, entries := s.handleSearchRequest(ctx, msgID, opData, controls)
		return resp, result, entries, false
	case opCompareRequest:
		if !authenticated {
			result := resultInsufficientAccessRights
			return encodeCompareResponse(msgID, result, "", "bind required"), result, 0, false
		}
		if err := authorizeProxiedAuthorization(controls, authzID); err != nil {
			result := resultAuthorizationDenied
			return encodeCompareResponse(msgID, result, "", err.Error()), result, 0, false
		}
		resp, result := s.handleCompareRequest(ctx, msgID, opData)
		return resp, result, 0, false
	case opUnbindRequest:
		return nil, resultSuccess, 0, false
	case opAbandonRequest:
		return nil, resultSuccess, 0, false
	case opExtendedRequest:
		resp, result := s.handleExtendedRequest(msgID, opData, authenticated, authzID)
		return resp, result, 0, false
	case opModifyRequest, opAddRequest, opDeleteRequest, opModDNRequest:
		result := resultUnwillingToPerform
		return encodeReadOnlyWriteResponse(msgID, opTag, result, "", "read-only LDAP gateway"), result, 0, false
	default:
		result := resultUnwillingToPerform
		return encodeLDAPResponse(msgID, opTag, mustEncodeNotSupported()), result, 0, false
	}
}

func (s *LDAPServer) handleExtendedRequest(msgID int, opData []byte, authenticated bool, authzID string) ([]byte, int) {
	name, err := decodeExtendedRequestName(opData)
	if err != nil {
		result := resultProtocolError
		return encodeExtendedResponse(msgID, result, "", "malformed extended request"), result
	}
	switch name {
	case whoAmIOID:
		if !authenticated {
			result := resultInsufficientAccessRights
			return encodeExtendedResponse(msgID, result, "", "bind required"), result
		}
		return encodeExtendedResponseWithValue(msgID, resultSuccess, "", "", authzID), resultSuccess
	default:
		result := resultUnwillingToPerform
		return encodeExtendedResponse(msgID, result, "", "extended operation not supported"), result
	}
}

func isPublicDiscoverySearch(opData []byte) bool {
	baseObject, scope, _, _, _, _, _, err := decodeSearchRequest(opData)
	if err != nil {
		return false
	}
	if baseObject == "" && scope == scopeBaseObject {
		return true
	}
	return normalizeDNForCompare(baseObject) == "cn=subschema" && scope == scopeBaseObject
}

func (s *LDAPServer) handleBindRequest(ctx context.Context, msgID int, opData []byte) ([]byte, int) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeBindResponse(msgID, result, "", "operation timed out"), result
	default:
	}

	req, err := decodeBindRequestData(opData)
	if err != nil {
		if errors.Is(err, errUnsupportedBindAuth) {
			result := resultAuthMethodNotSupported
			return encodeBindResponse(msgID, result, "", "unsupported bind authentication method"), result
		}
		result := resultUnwillingToPerform
		return encodeBindResponse(msgID, result, "", "malformed bind request"), result
	}
	if req.version != ldapV3 {
		result := resultAuthMethodNotSupported
		return encodeBindResponse(msgID, result, "", "unsupported LDAP version"), result
	}

	ok, err := s.authenticateBindIdentity(ctx, req.name, string(req.auth))
	if err != nil || !ok {
		result := resultInvalidCredentials
		return encodeBindResponse(msgID, result, "", "invalid credentials"), result
	}
	return encodeBindResponse(msgID, resultSuccess, "", ""), resultSuccess
}

func (s *LDAPServer) authenticateBindIdentity(ctx context.Context, name, password string) (bool, error) {
	for _, candidate := range bindIdentityCandidates(name) {
		ok, err := s.auth.AuthenticateLDAP(ctx, candidate, password)
		if err != nil || ok {
			return ok, err
		}
	}
	return false, nil
}

func bindIdentityCandidates(name string) []string {
	name = strings.TrimSpace(name)
	if name == "" {
		return []string{""}
	}
	candidates := []string{name}
	if attr, value, ok := firstDNAttributeValue(name); ok {
		switch strings.ToLower(attr) {
		case "uid", "mail", "cn":
			candidates = append(candidates, value)
		}
	}
	out := make([]string, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		key := strings.ToLower(candidate)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, candidate)
	}
	return out
}

func firstDNAttributeValue(dn string) (string, string, bool) {
	first := strings.TrimSpace(strings.Split(dn, ",")[0])
	parts := strings.SplitN(first, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	attr := strings.TrimSpace(parts[0])
	value, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
	return attr, value, ok
}

func unescapeDNValue(value string) (string, bool) {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		if value[i] != '\\' {
			b.WriteByte(value[i])
			continue
		}
		if i+1 >= len(value) {
			return "", false
		}
		if i+2 < len(value) && isHexByte(value[i+1]) && isHexByte(value[i+2]) {
			b.WriteByte(fromHexPair(value[i+1], value[i+2]))
			i += 2
			continue
		}
		i++
		b.WriteByte(value[i])
	}
	return b.String(), true
}

func isHexByte(b byte) bool {
	return ('0' <= b && b <= '9') || ('a' <= b && b <= 'f') || ('A' <= b && b <= 'F')
}

func fromHexPair(a, b byte) byte {
	return fromHexNibble(a)<<4 | fromHexNibble(b)
}

func fromHexNibble(b byte) byte {
	switch {
	case '0' <= b && b <= '9':
		return b - '0'
	case 'a' <= b && b <= 'f':
		return b - 'a' + 10
	default:
		return b - 'A' + 10
	}
}

func (s *LDAPServer) handleCompareRequest(ctx context.Context, msgID int, opData []byte) ([]byte, int) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeCompareResponse(msgID, result, "", "operation timed out"), result
	default:
	}

	req, err := decodeCompareRequestData(opData)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeCompareResponse(msgID, result, "", "malformed compare request"), result
	}
	principals, err := s.quer.SearchPrincipals(ctx, DirectorySearchRequest{
		BaseDN: req.entry,
		Scope:  scopeBaseObject,
		Attrs:  []string{req.attr},
		Kinds:  principalKindsForBaseDN(req.entry),
		Limit:  1,
	})
	if err != nil {
		result := resultUnwillingToPerform
		return encodeCompareResponse(msgID, result, "", err.Error()), result
	}
	principals = filterPrincipalEntriesByScope(principals, req.entry, scopeBaseObject)
	if len(principals) == 0 {
		result := resultNoSuchObject
		return encodeCompareResponse(msgID, result, "", "compare entry not found"), result
	}
	attrs := principalLDAPAttributes(principals[0])
	values, ok := lookupLDAPAttributeValues(attrs, req.attr)
	if !ok {
		return encodeCompareResponse(msgID, resultCompareFalse, "", ""), resultCompareFalse
	}
	for _, value := range values {
		if strings.EqualFold(value, req.value) {
			return encodeCompareResponse(msgID, resultCompareTrue, "", ""), resultCompareTrue
		}
	}
	return encodeCompareResponse(msgID, resultCompareFalse, "", ""), resultCompareFalse
}

type compareRequest struct {
	entry string
	attr  string
	value string
}

func decodeCompareRequestData(data []byte) (compareRequest, error) {
	entry, rest, err := decodeOctetString(data)
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare entry: %w", err)
	}
	if len(rest) == 0 || rest[0] != tagSequence {
		return compareRequest{}, fmt.Errorf("compare assertion missing")
	}
	assertion, err := decodeContent(rest[1:])
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare assertion: %w", err)
	}
	attr, valueRest, err := decodeOctetString(assertion)
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare attribute: %w", err)
	}
	value, trailing, err := decodeOctetString(valueRest)
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare value: %w", err)
	}
	if len(trailing) != 0 {
		return compareRequest{}, fmt.Errorf("compare assertion trailing data")
	}
	return compareRequest{entry: entry, attr: attr, value: value}, nil
}

func lookupLDAPAttributeValues(attrs map[string][]string, attr string) ([]string, bool) {
	for name, values := range attrs {
		if strings.EqualFold(name, strings.TrimSpace(attr)) {
			return values, true
		}
	}
	return nil, false
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
	if len(rest) == 0 || rest[0] != 0x80 {
		return nil, errUnsupportedBindAuth
	}
	authLen, authRest, err := decodeLength(rest[1:])
	if err != nil {
		return nil, err
	}
	if len(authRest) < authLen {
		return nil, fmt.Errorf("bind simple auth value truncated")
	}
	auth = authRest[:authLen]
	if len(authRest[authLen:]) != 0 {
		return nil, fmt.Errorf("bind request has trailing data")
	}
	return &bindRequest{version: version, name: name, auth: auth}, nil
}

func (s *LDAPServer) handleSearchRequest(ctx context.Context, msgID int, opData []byte, controls []control) ([]byte, int, int) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "operation timed out"), result, 0
	default:
	}

	baseObject, scope, filter, attrs, sizeLimit, _, typesOnly, err := decodeSearchRequest(opData)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	if baseObject == "" && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, "", rootDSEAttributes(s.namingContexts, s.tlsConfig != nil), filter, attrs, typesOnly)
	}
	if normalizeDNForCompare(baseObject) == "cn=subschema" && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, "cn=Subschema", subschemaAttributes(), filter, attrs, typesOnly)
	}
	if containerAttrs, ok := ldapContainerAttributes(baseObject); ok && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, baseObject, containerAttrs, filter, attrs, typesOnly)
	}
	if !s.baseDNWithinNamingContext(baseObject) && len(s.referralURLs) > 0 {
		resp := encodeSearchResultReference(msgID, s.referralURLs)
		resp = append(resp, encodeSearchResultDone(msgID, resultSuccess, "", "")...)
		return resp, resultSuccess, 0
	}
	pageSize, pageOffset, paged, err := parsePagedResultsControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	sortKeys, sorted, err := parseServerSideSortControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	vlv, hasVLV, err := parseVirtualListViewControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	matchedValuesFilters, hasMatchedValues, err := parseMatchedValuesControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	assertionFilter, hasAssertion, err := parseAssertionControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	syncReq, hasSync, err := parseSyncRequestControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	_, err = parseDereferenceControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	// Gogomail has no DN-valued relationship expansion repository yet; accept
	// well-formed dereference requests as a read-only no-op for client parity.
	subentriesOnly, err := parseSubentriesControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}

	if err := validateFilter(filter); err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", fmt.Sprintf("invalid filter: %v", err)), result, 0
	}
	if hasAssertion {
		ok, err := evaluateSearchAssertion(assertionFilter, baseObject)
		if err != nil {
			result := resultProtocolError
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
		}
		if !ok {
			result := resultAssertionFailed
			return encodeSearchResultDone(msgID, result, "", "assertion failed"), result, 0
		}
	}

	ldapFilter, err := parseLDAPFilter(filter)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "malformed filter"), result, 0
	}
	kinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "malformed objectClass filter"), result, 0
	}
	kinds, noKindMatch := intersectPrincipalKinds(kinds, principalKindsForBaseDN(baseObject))
	if noKindMatch {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}
	if subentriesOnly {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}

	targetEntries := ldapSearchCandidateBatchSize
	scanAllCandidates := sorted || hasVLV
	if paged && pageSize > 0 {
		targetEntries = max(targetEntries, pageOffset+pageSize+1)
	}
	if sizeLimit > 0 {
		targetEntries = max(targetEntries, sizeLimit+1)
	}

	entries, err := s.searchLDAPEntries(ctx, DirectorySearchRequest{
		BaseDN: baseObject,
		Scope:  scope,
		Filter: ldapFilter,
		Attrs:  attrs,
		Kinds:  kinds,
	}, filter, targetEntries, scanAllCandidates)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	if sorted {
		sortLDAPSearchEntries(entries, sortKeys)
	}
	sizeLimitExceeded := sizeLimit > 0 && len(entries) > sizeLimit
	if sizeLimit > 0 && len(entries) > sizeLimit {
		entries = entries[:sizeLimit]
	}
	vlvTargetPosition := 0
	vlvContentCount := len(entries)
	if hasVLV {
		entries, vlvTargetPosition = applyVirtualListView(entries, vlv)
	}
	nextCookie := ""
	if paged && pageSize > 0 {
		if pageOffset < len(entries) {
			entries = entries[pageOffset:]
		} else {
			entries = nil
		}
		if len(entries) > pageSize {
			entries = entries[:pageSize]
			nextCookie = fmt.Sprintf("%d", pageOffset+pageSize)
		}
	}

	resp := make([]byte, 0, 4096)
	for _, entry := range entries {
		attrMap := entry.attrs
		if hasMatchedValues {
			attrMap = applyMatchedValuesFilter(attrMap, matchedValuesFilters)
		}
		var entryControls []control
		if hasSync {
			entryControls = append(entryControls, syncStateControl(entry.principal.DN, syncReq.Cookie))
		}
		responseEntry, err := encodeSearchResultEntryWithControls(msgID, entry.principal.DN, selectLDAPAttributes(attrMap, attrs, typesOnly), entryControls)
		if err != nil {
			continue
		}
		resp = append(resp, responseEntry...)
	}
	result := resultSuccess
	if sizeLimitExceeded {
		result = resultSizeLimitExceeded
	}
	responseControls := make([]control, 0, 2)
	if paged {
		responseControls = append(responseControls, pagedResultsResponseControl(nextCookie))
	}
	if sorted {
		responseControls = append(responseControls, serverSideSortResponseControl(resultSuccess, ""))
	}
	if hasVLV {
		responseControls = append(responseControls, virtualListViewResponseControl(vlvTargetPosition, vlvContentCount, resultSuccess, ""))
	}
	if hasSync {
		responseControls = append(responseControls, syncDoneControl(syncReq.Cookie))
	}
	if len(responseControls) > 0 {
		resp = append(resp, encodeSearchResultDoneWithControls(msgID, result, "", "", responseControls)...)
	} else {
		resp = append(resp, encodeSearchResultDone(msgID, result, "", "")...)
	}
	return resp, result, len(entries)
}

func encodeSyntheticSearchResult(msgID int, dn string, entryAttrs map[string][]string, filter []byte, requested []string, typesOnly bool) ([]byte, int, int) {
	if err := validateFilter(filter); err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", "malformed filter"), result, 0
	}
	if !ldapEntryAttributesMatchFilter(entryAttrs, filter) {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}
	entry, err := encodeSearchResultEntry(msgID, dn, selectLDAPAttributes(entryAttrs, requested, typesOnly))
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	return append(entry, encodeSearchResultDone(msgID, resultSuccess, "", "")...), resultSuccess, 1
}

func filterPrincipalEntriesByScope(principals []PrincipalEntry, baseObject string, scope int) []PrincipalEntry {
	base := normalizeDNForCompare(baseObject)
	if base == "" {
		if scope == scopeBaseObject {
			return nil
		}
		return principals
	}
	filtered := make([]PrincipalEntry, 0, len(principals))
	for _, p := range principals {
		entryDN := normalizeDNForCompare(p.DN)
		switch scope {
		case scopeBaseObject:
			if entryDN == base {
				filtered = append(filtered, p)
			}
		case scopeSingleLevel:
			if parentDN(entryDN) == base {
				filtered = append(filtered, p)
			}
		case scopeWholeSubtree:
			if entryDN == base || strings.HasSuffix(entryDN, ","+base) {
				filtered = append(filtered, p)
			}
		default:
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (s *LDAPServer) searchLDAPEntries(ctx context.Context, req DirectorySearchRequest, filter []byte, targetEntries int, scanAll bool) ([]ldapSearchEntry, error) {
	if targetEntries <= 0 {
		targetEntries = ldapSearchCandidateBatchSize
	}
	var entries []ldapSearchEntry
	offset := 0
	for offset < ldapMaxCandidateScan {
		batchReq := req
		batchReq.Limit = ldapSearchCandidateBatchSize
		batchReq.Offset = offset
		principals, err := s.quer.SearchPrincipals(ctx, batchReq)
		if err != nil {
			return nil, err
		}
		if len(principals) == 0 {
			break
		}
		scoped := filterPrincipalEntriesByScope(principals, req.BaseDN, req.Scope)
		batchEntries := ldapSearchEntries(scoped)
		entries = append(entries, filterLDAPSearchEntriesByFilter(batchEntries, filter)...)
		if !scanAll && len(entries) >= targetEntries {
			break
		}
		if len(principals) < ldapSearchCandidateBatchSize {
			break
		}
		offset += len(principals)
	}
	return entries, nil
}

func ldapSearchEntries(principals []PrincipalEntry) []ldapSearchEntry {
	entries := make([]ldapSearchEntry, 0, len(principals))
	for _, p := range principals {
		entries = append(entries, ldapSearchEntry{principal: p, attrs: principalLDAPAttributes(p)})
	}
	return entries
}

func filterLDAPSearchEntriesByFilter(entries []ldapSearchEntry, filter []byte) []ldapSearchEntry {
	if len(filter) == 0 {
		return entries
	}
	filtered := make([]ldapSearchEntry, 0, len(entries))
	for _, entry := range entries {
		if ldapEntryAttributesMatchFilter(entry.attrs, filter) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func parentDN(dn string) string {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func ldapContainerAttributes(dn string) (map[string][]string, bool) {
	operational := ldapOperationalAttributes(dn)
	withOperational := func(attrs map[string][]string) map[string][]string {
		for k, values := range operational {
			attrs[k] = values
		}
		attrs["hasSubordinates"] = []string{"TRUE"}
		attrs["numSubordinates"] = []string{"0"}
		return attrs
	}
	switch firstRDNValue(normalizeDNForCompare(dn), "ou") {
	case "users":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"users"}, "cn": {"users"}, "displayName": {"Users"}}), true
	case "organizations":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"organizations"}, "cn": {"organizations"}, "displayName": {"Organizations"}}), true
	case "groups":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"groups"}, "cn": {"groups"}, "displayName": {"Groups"}}), true
	case "resources":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"resources"}, "cn": {"resources"}, "displayName": {"Resources"}}), true
	default:
		return nil, false
	}
}

func principalKindsForBaseDN(dn string) []string {
	switch firstRDNValue(normalizeDNForCompare(dn), "ou") {
	case "users":
		return []string{"user"}
	case "organizations":
		return []string{"organization"}
	case "groups":
		return []string{"group"}
	case "resources":
		return []string{"resource"}
	default:
		return nil
	}
}

func firstRDNValue(dn, attr string) string {
	if dn == "" {
		return ""
	}
	first := strings.TrimSpace(strings.Split(dn, ",")[0])
	parts := strings.SplitN(first, "=", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), attr) {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func intersectPrincipalKinds(filterKinds, baseKinds []string) ([]string, bool) {
	if len(baseKinds) == 0 {
		return filterKinds, false
	}
	if len(filterKinds) == 0 {
		return baseKinds, false
	}
	base := make(map[string]struct{}, len(baseKinds))
	for _, kind := range baseKinds {
		base[kind] = struct{}{}
	}
	out := make([]string, 0, len(filterKinds))
	for _, kind := range filterKinds {
		if _, ok := base[kind]; ok {
			out = append(out, kind)
		}
	}
	if len(out) == 0 {
		return nil, true
	}
	return out, false
}

func principalLDAPAttributes(p PrincipalEntry) map[string][]string {
	kind := strings.ToLower(strings.TrimSpace(p.Kind))
	if kind == "" {
		kind = "user"
	}
	cn := firstNonEmpty(p.CN, p.DisplayName, p.UID)
	objectCategory := ldapObjectCategory(kind)
	attrs := map[string][]string{
		"cn":                {cn},
		"name":              {cn},
		"uid":               {p.UID},
		"displayName":       {firstNonEmpty(p.DisplayName, cn)},
		"canonicalName":     nonEmptyLDAPValues([]string{ldapCanonicalName(p.DN)}),
		"distinguishedName": nonEmptyLDAPValues([]string{p.DN}),
		"instanceType":      {"4"},
		"objectCategory":    {objectCategory},
		"objectGUID":        {string(ldapEntryUUIDBytes(p.DN))},
		"mailNickname":      nonEmptyLDAPValues([]string{p.UID}),
		"proxyAddresses":    ldapProxyAddresses(p.Mail),
		"sAMAccountName":    {p.UID},
		"userPrincipalName": nonEmptyLDAPValues([]string{p.Mail}),
		"whenCreated":       {"19700101000000.0Z"},
		"whenChanged":       {"19700101000000.0Z"},
		"uSNCreated":        {ldapUpdateSequenceNumber(p.DN)},
		"uSNChanged":        {ldapUpdateSequenceNumber(p.DN)},
	}
	if len(attrs["canonicalName"]) == 0 {
		delete(attrs, "canonicalName")
	}
	if len(attrs["distinguishedName"]) == 0 {
		delete(attrs, "distinguishedName")
	}
	if sid := ldapObjectSIDBytes(p.DN, kind); len(sid) > 0 {
		attrs["objectSid"] = []string{string(sid)}
	}
	if len(attrs["userPrincipalName"]) == 0 {
		delete(attrs, "userPrincipalName")
	}
	if memberOf := nonEmptyLDAPValues(p.MemberOf); len(memberOf) > 0 {
		attrs["memberOf"] = memberOf
	}
	switch kind {
	case "organization":
		attrs["objectClass"] = []string{"top", "organizationalUnit"}
		attrs["ou"] = []string{firstNonEmpty(p.OU, p.DisplayName, cn)}
	case "group":
		attrs["objectClass"] = []string{"top", "groupOfNames"}
		members := nonEmptyLDAPValues(p.Members)
		if len(members) == 0 {
			members = []string{firstNonEmpty(p.DN, "cn=placeholder")}
		}
		attrs["member"] = members
	case "resource":
		attrs["objectClass"] = []string{"top", "device"}
		if p.ResourceType != "" {
			attrs["description"] = []string{p.ResourceType}
		}
	default:
		attrs["objectClass"] = []string{"top", "person", "organizationalPerson", "inetOrgPerson", "user"}
		attrs["accountExpires"] = []string{"9223372036854775807"}
		attrs["primaryGroupID"] = []string{"513"}
		attrs["userAccountControl"] = []string{"512"}
		if p.Mail != "" {
			attrs["mail"] = []string{p.Mail}
		}
		if p.GivenName != "" {
			attrs["givenName"] = []string{p.GivenName}
		}
		attrs["sn"] = []string{firstNonEmpty(p.SN, p.DisplayName, cn)}
	}
	for k, values := range ldapOperationalAttributes(p.DN) {
		attrs[k] = values
	}
	return attrs
}

func ldapProxyAddresses(mail string) []string {
	mail = strings.TrimSpace(mail)
	if mail == "" {
		return nil
	}
	return []string{"SMTP:" + mail}
}

func ldapObjectCategory(kind string) string {
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "organization":
		return "organizationalUnit"
	case "group":
		return "group"
	case "resource":
		return "device"
	default:
		return "person"
	}
}

func nonEmptyLDAPValues(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}

func ldapOperationalAttributes(dn string) map[string][]string {
	dn = strings.TrimSpace(dn)
	if dn == "" {
		return nil
	}
	return map[string][]string{
		"entryDN":         {dn},
		"entryUUID":       {ldapEntryUUID(dn)},
		"hasSubordinates": {"FALSE"},
		"numSubordinates": {"0"},
		"createTimestamp": {"19700101000000Z"},
		"modifyTimestamp": {"19700101000000Z"},
		"creatorsName":    {"cn=gogomail"},
		"modifiersName":   {"cn=gogomail"},
	}
}

func ldapEntryUUID(dn string) string {
	b := ldapEntryUUIDBytes(dn)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func ldapEntryUUIDBytes(dn string) []byte {
	sum := sha1.Sum([]byte(normalizeDNForCompare(dn)))
	b := sum[:16]
	b[6] = (b[6] & 0x0f) | 0x50
	b[8] = (b[8] & 0x3f) | 0x80
	out := make([]byte, 16)
	copy(out, b)
	return out
}

func ldapObjectSID(dn, kind string) string {
	a, c, d, rid, ok := ldapObjectSIDParts(dn, kind)
	if !ok {
		return ""
	}
	return fmt.Sprintf("S-1-5-21-%d-%d-%d-%d", a, c, d, rid)
}

func ldapObjectSIDBytes(dn, kind string) []byte {
	a, c, d, rid, ok := ldapObjectSIDParts(dn, kind)
	if !ok {
		return nil
	}
	out := []byte{1, 5, 0, 0, 0, 0, 0, 5}
	for _, v := range []uint32{21, a, c, d, rid} {
		out = append(out, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
	}
	return out
}

func ldapObjectSIDParts(dn, kind string) (uint32, uint32, uint32, uint32, bool) {
	if strings.TrimSpace(dn) == "" {
		return 0, 0, 0, 0, false
	}
	b := ldapEntryUUIDBytes(dn)
	a := uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3])
	c := uint32(b[4])<<24 | uint32(b[5])<<16 | uint32(b[6])<<8 | uint32(b[7])
	d := uint32(b[8])<<24 | uint32(b[9])<<16 | uint32(b[10])<<8 | uint32(b[11])
	rid := uint32(b[12])<<24 | uint32(b[13])<<16 | uint32(b[14])<<8 | uint32(b[15])
	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "group":
		rid = 2000000000 + rid%999999999
	case "organization":
		rid = 3000000000 + rid%999999999
	case "resource":
		rid = 4000000000 + rid%294967295
	default:
		rid = 1000000000 + rid%999999999
	}
	return a, c, d, rid, true
}

func ldapUpdateSequenceNumber(dn string) string {
	b := ldapEntryUUIDBytes(dn)
	var n uint64
	for i := 0; i < 8 && i < len(b); i++ {
		n = (n << 8) | uint64(b[i])
	}
	if n == 0 {
		n = 1
	}
	return fmt.Sprintf("%d", n)
}

func ldapCanonicalName(dn string) string {
	parts := splitDNComponents(strings.TrimSpace(dn))
	if len(parts) == 0 {
		return ""
	}
	domain := make([]string, 0, 2)
	path := make([]string, 0, len(parts))
	for _, part := range parts {
		attr, value, ok := splitDNAttributeValue(part)
		if !ok || value == "" {
			return ""
		}
		if strings.EqualFold(attr, "dc") {
			domain = append(domain, value)
			continue
		}
		path = append([]string{value}, path...)
	}
	if len(domain) == 0 {
		return strings.Join(path, "/")
	}
	canonical := strings.Join(domain, ".")
	if len(path) > 0 {
		canonical += "/" + strings.Join(path, "/")
	}
	return canonical
}

func splitDNComponents(dn string) []string {
	if dn == "" {
		return nil
	}
	parts := make([]string, 0, 4)
	start := 0
	escaped := false
	for i := 0; i < len(dn); i++ {
		switch {
		case escaped:
			escaped = false
		case dn[i] == '\\':
			escaped = true
		case dn[i] == ',':
			parts = append(parts, strings.TrimSpace(dn[start:i]))
			start = i + 1
		}
	}
	parts = append(parts, strings.TrimSpace(dn[start:]))
	return parts
}

func splitDNAttributeValue(part string) (string, string, bool) {
	parts := strings.SplitN(strings.TrimSpace(part), "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	value, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
	return strings.TrimSpace(parts[0]), value, ok
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *LDAPServer) baseDNWithinNamingContext(baseDN string) bool {
	baseDN = normalizeDNForCompare(baseDN)
	if baseDN == "" || len(s.namingContexts) == 0 {
		return true
	}
	for _, namingContext := range s.namingContexts {
		ctxDN := normalizeDNForCompare(namingContext)
		if baseDN == ctxDN || strings.HasSuffix(baseDN, ","+ctxDN) {
			return true
		}
	}
	return false
}

func firstUnsupportedCriticalControl(controls []control) (control, bool) {
	for _, ctrl := range controls {
		if ctrl.Critical && !isSupportedControl(ctrl.Type) {
			return ctrl, true
		}
	}
	return control{}, false
}

func parsePagedResultsControl(controls []control) (pageSize int, offset int, ok bool, err error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlPagedResults {
			continue
		}
		ok = true
		pageSize, offset, err = decodePagedResultsControlValue(ctrl.Value)
		if err != nil {
			return 0, 0, true, err
		}
		return pageSize, offset, true, nil
	}
	return 0, 0, false, nil
}

func decodePagedResultsControlValue(value []byte) (pageSize int, offset int, err error) {
	if len(value) == 0 {
		return 0, 0, fmt.Errorf("paged results control value is required")
	}
	if value[0] != tagSequence {
		return 0, 0, fmt.Errorf("paged results control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return 0, 0, err
	}
	pageSize, rest, err := decodeInt(content)
	if err != nil {
		return 0, 0, fmt.Errorf("paged results size: %w", err)
	}
	if pageSize < 0 {
		return 0, 0, fmt.Errorf("paged results size must not be negative")
	}
	cookie, rest, err := decodeOctetString(rest)
	if err != nil {
		return 0, 0, fmt.Errorf("paged results cookie: %w", err)
	}
	if len(rest) != 0 {
		return 0, 0, fmt.Errorf("paged results control has trailing data")
	}
	if strings.TrimSpace(cookie) == "" {
		return pageSize, 0, nil
	}
	for _, r := range cookie {
		if r < '0' || r > '9' {
			return 0, 0, fmt.Errorf("paged results cookie is invalid")
		}
		offset = offset*10 + int(r-'0')
	}
	return pageSize, offset, nil
}

func pagedResultsResponseControl(cookie string) control {
	var value []byte
	value = append(value, encodeInt(0)...)
	value = append(value, encodeOctetString(cookie)...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(value))...)
	seq = append(seq, value...)
	return control{Type: controlPagedResults, Value: seq}
}

type sortKey struct {
	Attribute string
	Reverse   bool
}

func parseServerSideSortControl(controls []control) ([]sortKey, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlServerSideSortRequest {
			continue
		}
		keys, err := decodeServerSideSortControlValue(ctrl.Value)
		if err != nil {
			return nil, true, err
		}
		return keys, true, nil
	}
	return nil, false, nil
}

func decodeServerSideSortControlValue(value []byte) ([]sortKey, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("server-side sort control value is required")
	}
	if value[0] != tagSequence {
		return nil, fmt.Errorf("server-side sort control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return nil, err
	}
	var keys []sortKey
	for len(content) > 0 {
		item, rest, err := readRawTLV(content)
		if err != nil {
			return nil, err
		}
		key, err := decodeSortKey(item)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		content = rest
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("server-side sort control requires at least one key")
	}
	return keys, nil
}

func decodeSortKey(data []byte) (sortKey, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return sortKey{}, fmt.Errorf("server-side sort key must be a sequence")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return sortKey{}, err
	}
	attr, rest, err := decodeOctetString(content)
	if err != nil {
		return sortKey{}, fmt.Errorf("server-side sort attribute: %w", err)
	}
	key := sortKey{Attribute: strings.TrimSpace(attr)}
	if key.Attribute == "" {
		return sortKey{}, fmt.Errorf("server-side sort attribute is empty")
	}
	for len(rest) > 0 {
		tag := rest[0]
		length, tail, err := decodeLength(rest[1:])
		if err != nil {
			return sortKey{}, err
		}
		if len(tail) < length {
			return sortKey{}, fmt.Errorf("server-side sort key value truncated")
		}
		value := tail[:length]
		switch tag {
		case 0x80:
			// orderingRule is accepted but repository entries use local string ordering.
		case 0x81:
			if len(value) != 1 {
				return sortKey{}, fmt.Errorf("server-side sort reverseOrder malformed")
			}
			key.Reverse = value[0] != 0
		default:
			return sortKey{}, fmt.Errorf("unsupported server-side sort key tag 0x%02x", tag)
		}
		rest = tail[length:]
	}
	return key, nil
}

func sortPrincipalEntries(principals []PrincipalEntry, keys []sortKey) {
	entries := ldapSearchEntries(principals)
	sortLDAPSearchEntries(entries, keys)
	for i, entry := range entries {
		principals[i] = entry.principal
	}
}

func sortLDAPSearchEntries(entries []ldapSearchEntry, keys []sortKey) {
	if len(keys) == 0 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		leftAttrs := entries[i].attrs
		rightAttrs := entries[j].attrs
		for _, key := range keys {
			left := firstLDAPAttributeValue(leftAttrs, key.Attribute)
			right := firstLDAPAttributeValue(rightAttrs, key.Attribute)
			cmp := strings.Compare(strings.ToLower(left), strings.ToLower(right))
			if cmp == 0 {
				continue
			}
			if key.Reverse {
				return cmp > 0
			}
			return cmp < 0
		}
		return normalizeDNForCompare(entries[i].principal.DN) < normalizeDNForCompare(entries[j].principal.DN)
	})
}

func firstLDAPAttributeValue(attrs map[string][]string, attr string) string {
	for name, values := range attrs {
		if strings.EqualFold(name, attr) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func serverSideSortResponseControl(resultCode int, attributeType string) control {
	var content []byte
	content = append(content, encodeEnumerated(resultCode)...)
	if strings.TrimSpace(attributeType) != "" {
		value := []byte(attributeType)
		content = append(content, 0x80)
		content = append(content, encodeLength(len(value))...)
		content = append(content, value...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlServerSideSortResponse, Value: seq}
}

type virtualListViewRequest struct {
	BeforeCount int
	AfterCount  int
	Offset      int
}

func parseVirtualListViewControl(controls []control) (virtualListViewRequest, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlVirtualListViewRequest {
			continue
		}
		req, err := decodeVirtualListViewControlValue(ctrl.Value)
		if err != nil {
			return virtualListViewRequest{}, true, err
		}
		return req, true, nil
	}
	return virtualListViewRequest{}, false, nil
}

func decodeVirtualListViewControlValue(value []byte) (virtualListViewRequest, error) {
	if len(value) == 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view control value is required")
	}
	if value[0] != tagSequence {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return virtualListViewRequest{}, err
	}
	before, rest, err := decodeInt(content)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view beforeCount: %w", err)
	}
	after, rest, err := decodeInt(rest)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view afterCount: %w", err)
	}
	if before < 0 || after < 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view counts must not be negative")
	}
	if len(rest) == 0 || rest[0] != 0xa0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view byOffset target is required")
	}
	target, rest, err := readRawTLV(rest)
	if err != nil {
		return virtualListViewRequest{}, err
	}
	targetContent, err := decodeContent(target[1:])
	if err != nil {
		return virtualListViewRequest{}, err
	}
	offset, targetRest, err := decodeInt(targetContent)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view offset: %w", err)
	}
	_, targetRest, err = decodeInt(targetRest)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view contentCount: %w", err)
	}
	if len(targetRest) != 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view target has trailing data")
	}
	if len(rest) > 0 {
		_, rest, err = decodeOctetString(rest)
		if err != nil {
			return virtualListViewRequest{}, fmt.Errorf("virtual list view contextID: %w", err)
		}
		if len(rest) != 0 {
			return virtualListViewRequest{}, fmt.Errorf("virtual list view control has trailing data")
		}
	}
	if offset < 1 {
		offset = 1
	}
	return virtualListViewRequest{BeforeCount: before, AfterCount: after, Offset: offset}, nil
}

func applyVirtualListView[T any](entries []T, req virtualListViewRequest) ([]T, int) {
	if len(entries) == 0 {
		return nil, 0
	}
	target := req.Offset
	if target > len(entries) {
		target = len(entries)
	}
	start := target - 1 - req.BeforeCount
	if start < 0 {
		start = 0
	}
	end := target + req.AfterCount
	if end > len(entries) {
		end = len(entries)
	}
	return entries[start:end], target
}

func virtualListViewResponseControl(targetPosition int, contentCount int, resultCode int, contextID string) control {
	var content []byte
	content = append(content, encodeInt(targetPosition)...)
	content = append(content, encodeInt(contentCount)...)
	content = append(content, encodeEnumerated(resultCode)...)
	if contextID != "" {
		content = append(content, encodeOctetString(contextID)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlVirtualListViewResponse, Value: seq}
}

func parseMatchedValuesControl(controls []control) ([][]byte, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlMatchedValues {
			continue
		}
		if len(ctrl.Value) == 0 {
			return nil, true, fmt.Errorf("matched values control value is required")
		}
		if ctrl.Value[0]&0x80 != 0 {
			if err := validateFilter(ctrl.Value); err != nil {
				return nil, true, fmt.Errorf("matched values filter: %w", err)
			}
			return [][]byte{ctrl.Value}, true, nil
		}
		if ctrl.Value[0] != tagSequence {
			return nil, true, fmt.Errorf("matched values control value must be a sequence")
		}
		content, err := decodeContent(ctrl.Value[1:])
		if err != nil {
			return nil, true, err
		}
		var filters [][]byte
		for len(content) > 0 {
			item, rest, err := readRawTLV(content)
			if err != nil {
				return nil, true, err
			}
			if err := validateFilter(item); err != nil {
				return nil, true, fmt.Errorf("matched values filter: %w", err)
			}
			filters = append(filters, item)
			content = rest
		}
		if len(filters) == 0 {
			return nil, true, fmt.Errorf("matched values control requires at least one filter")
		}
		return filters, true, nil
	}
	return nil, false, nil
}

func applyMatchedValuesFilter(attrs map[string][]string, filters [][]byte) map[string][]string {
	filtered := make(map[string][]string, len(attrs))
	for name, values := range attrs {
		var kept []string
		for _, value := range values {
			for _, filter := range filters {
				if ldapAttributeValueMatchesFilter(name, value, filter) {
					kept = append(kept, value)
					break
				}
			}
		}
		if len(kept) > 0 {
			filtered[name] = kept
		}
	}
	return filtered
}

func ldapAttributeValueMatchesFilter(attrName, attrValue string, filter []byte) bool {
	if len(filter) == 0 || filter[0]&0x80 == 0 {
		return false
	}
	filterType := int(filter[0] & 0x1f)
	content, err := decodeContent(filter[1:])
	if err != nil {
		return false
	}
	switch filterType {
	case filterAnd:
		matchedAny := false
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if !ldapAttributeValueMatchesFilter(attrName, attrValue, child) {
				return false
			}
			matchedAny = true
			content = rest
		}
		return matchedAny
	case filterOr:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if ldapAttributeValueMatchesFilter(attrName, attrValue, child) {
				return true
			}
			content = rest
		}
		return false
	case filterNot:
		child, _, err := readRawTLV(content)
		return err == nil && !ldapAttributeValueMatchesFilter(attrName, attrValue, child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valRest, err := decodeOctetString(content)
		if err != nil || !strings.EqualFold(strings.TrimSpace(attr), attrName) {
			return false
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return false
		}
		return ldapAttributeValueEqual(attrName, attrValue, val)
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil || !strings.EqualFold(strings.TrimSpace(attr), attrName) {
			return false
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return false
		}
		return ldapSubstringMatches(attrValue, parts)
	case filterPresent:
		return strings.EqualFold(strings.TrimSpace(string(content)), attrName)
	case filterExtensible:
		attr, value, ok, err := decodeExtensibleMatch(content)
		return err == nil && ok && strings.EqualFold(strings.TrimSpace(attr), attrName) && ldapAttributeValueEqual(attrName, attrValue, value)
	default:
		return false
	}
}

func ldapEntryAttributesMatchFilter(attrs map[string][]string, filter []byte) bool {
	if len(filter) == 0 || filter[0]&0x80 == 0 {
		return false
	}
	filterType := int(filter[0] & 0x1f)
	content, err := decodeContent(filter[1:])
	if err != nil {
		return false
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if !ldapEntryAttributesMatchFilter(attrs, child) {
				return false
			}
			content = rest
		}
		return true
	case filterOr:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if ldapEntryAttributesMatchFilter(attrs, child) {
				return true
			}
			content = rest
		}
		return false
	case filterNot:
		child, _, err := readRawTLV(content)
		return err == nil && !ldapEntryAttributesMatchFilter(attrs, child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return false
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, attr, func(candidate string) bool {
			switch filterType {
			case filterGreaterOrEqual:
				return strings.ToLower(candidate) >= strings.ToLower(val)
			case filterLessOrEqual:
				return strings.ToLower(candidate) <= strings.ToLower(val)
			default:
				return ldapAttributeValueEqual(attr, candidate, val)
			}
		})
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil {
			return false
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, attr, func(candidate string) bool {
			return ldapSubstringMatches(candidate, parts)
		})
	case filterPresent:
		_, ok := ldapEntryAttributeValues(attrs, string(content))
		return ok
	case filterExtensible:
		match, ok, err := decodeExtensibleMatchDetail(content)
		if err != nil || !ok {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, match.Attr, func(candidate string) bool {
			return ldapExtensibleValueMatches(candidate, match)
		})
	default:
		return false
	}
}

func ldapEntryAttributeHasValue(attrs map[string][]string, attr string, match func(string) bool) bool {
	values, ok := ldapEntryAttributeValues(attrs, attr)
	if !ok {
		return false
	}
	for _, value := range values {
		if match(value) {
			return true
		}
	}
	return false
}

func ldapEntryAttributeValues(attrs map[string][]string, attr string) ([]string, bool) {
	for name, values := range attrs {
		if strings.EqualFold(strings.TrimSpace(name), strings.TrimSpace(attr)) {
			return values, true
		}
	}
	return nil, false
}

func ldapAttributeValueEqual(attr, candidate, assertion string) bool {
	if ldapAttributeUsesBinaryMatch(attr) {
		return candidate == assertion
	}
	return strings.EqualFold(candidate, assertion)
}

func ldapAttributeUsesBinaryMatch(attr string) bool {
	switch strings.ToLower(strings.TrimSpace(attr)) {
	case "objectguid", "objectsid":
		return true
	default:
		return false
	}
}

func ldapExtensibleValueMatches(candidate string, match extensibleMatchAssertion) bool {
	switch strings.TrimSpace(match.MatchingRule) {
	case "":
		return ldapAttributeValueEqual(match.Attr, candidate, match.Value)
	case "1.2.840.113556.1.4.803": // LDAP_MATCHING_RULE_BIT_AND
		candidateInt, ok1 := parseLDAPInt64(candidate)
		assertionInt, ok2 := parseLDAPInt64(match.Value)
		return ok1 && ok2 && candidateInt&assertionInt == assertionInt
	case "1.2.840.113556.1.4.804": // LDAP_MATCHING_RULE_BIT_OR
		candidateInt, ok1 := parseLDAPInt64(candidate)
		assertionInt, ok2 := parseLDAPInt64(match.Value)
		return ok1 && ok2 && candidateInt&assertionInt != 0
	default:
		return ldapAttributeValueEqual(match.Attr, candidate, match.Value)
	}
}

func parseLDAPInt64(value string) (int64, bool) {
	var n int64
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}

func ldapSubstringMatches(value string, parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	value = strings.ToLower(value)
	pos := 0
	for _, part := range parts {
		part = strings.ToLower(part)
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}

func parseAssertionControl(controls []control) ([]byte, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlAssertion {
			continue
		}
		if len(ctrl.Value) == 0 {
			return nil, true, fmt.Errorf("assertion control value is required")
		}
		if err := validateFilter(ctrl.Value); err != nil {
			return nil, true, fmt.Errorf("assertion control filter: %w", err)
		}
		return ctrl.Value, true, nil
	}
	return nil, false, nil
}

func evaluateSearchAssertion(filter []byte, baseObject string) (bool, error) {
	filterKinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		return false, err
	}
	if _, noKindMatch := intersectPrincipalKinds(filterKinds, principalKindsForBaseDN(baseObject)); noKindMatch {
		return false, nil
	}
	attr, value, ok, err := parseLDAPFilterCandidate(filter)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	if strings.EqualFold(strings.TrimSpace(attr), "objectClass") && strings.TrimSpace(value) == "*" {
		return true, nil
	}
	return true, nil
}

func parseSubentriesControl(controls []control) (bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlSubentries {
			continue
		}
		if len(ctrl.Value) == 0 {
			return true, nil
		}
		value, rest, err := decodeBoolean(ctrl.Value)
		if err != nil {
			return false, fmt.Errorf("subentries control: %w", err)
		}
		if len(rest) != 0 {
			return false, fmt.Errorf("subentries control has trailing data")
		}
		return value, nil
	}
	return false, nil
}

func authorizeProxiedAuthorization(controls []control, boundAuthzID string) error {
	for _, ctrl := range controls {
		if ctrl.Type != controlProxiedAuthorization {
			continue
		}
		requested := strings.TrimSpace(string(ctrl.Value))
		if requested == "" {
			return fmt.Errorf("proxied authorization identity is required")
		}
		if !strings.EqualFold(requested, strings.TrimSpace(boundAuthzID)) {
			return fmt.Errorf("proxied authorization denied")
		}
	}
	return nil
}

type syncRequest struct {
	Mode   int
	Cookie string
}

func parseSyncRequestControl(controls []control) (syncRequest, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlSyncRequest {
			continue
		}
		req, err := decodeSyncRequestControlValue(ctrl.Value)
		if err != nil {
			return syncRequest{}, true, err
		}
		return req, true, nil
	}
	return syncRequest{}, false, nil
}

func decodeSyncRequestControlValue(value []byte) (syncRequest, error) {
	if len(value) == 0 || value[0] != tagSequence {
		return syncRequest{}, fmt.Errorf("sync request control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return syncRequest{}, err
	}
	mode, rest, err := decodeEnumeratedWithRest(content)
	if err != nil {
		return syncRequest{}, fmt.Errorf("sync request mode: %w", err)
	}
	switch mode {
	case syncModeRefreshOnly, syncModeRefreshAndPersist:
	default:
		return syncRequest{}, fmt.Errorf("sync request mode %d is unsupported", mode)
	}
	req := syncRequest{Mode: mode}
	if len(rest) > 0 && rest[0] == tagOctetString {
		cookie, next, err := decodeOctetString(rest)
		if err != nil {
			return syncRequest{}, fmt.Errorf("sync request cookie: %w", err)
		}
		req.Cookie = cookie
		rest = next
	}
	if len(rest) > 0 && rest[0] == tagBoolean {
		_, next, err := decodeBoolean(rest)
		if err != nil {
			return syncRequest{}, fmt.Errorf("sync request reloadHint: %w", err)
		}
		rest = next
	}
	if len(rest) != 0 {
		return syncRequest{}, fmt.Errorf("sync request control has trailing data")
	}
	return req, nil
}

func syncStateControl(dn, cookie string) control {
	var content []byte
	content = append(content, encodeEnumerated(syncStateAdd)...)
	content = append(content, encodeOctetStringBytes(ldapEntryUUIDBytes(dn))...)
	if cookie != "" {
		content = append(content, encodeOctetString(cookie)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlSyncState, Value: seq}
}

func syncDoneControl(cookie string) control {
	var content []byte
	if cookie != "" {
		content = append(content, encodeOctetString(cookie)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlSyncDone, Value: seq}
}

func parseDereferenceControl(controls []control) (bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlDereferenceRequest {
			continue
		}
		if err := decodeDereferenceControlValue(ctrl.Value); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func decodeDereferenceControlValue(value []byte) error {
	content, rest, err := decodeTaggedContent(value, tagSequence)
	if err != nil {
		return fmt.Errorf("dereference control value must be a sequence: %w", err)
	}
	if len(rest) != 0 {
		return fmt.Errorf("dereference control has trailing data")
	}
	if len(content) == 0 {
		return fmt.Errorf("dereference control requires at least one dereference spec")
	}
	for len(content) > 0 {
		var spec []byte
		spec, content, err = decodeTaggedContent(content, tagSequence)
		if err != nil {
			return fmt.Errorf("dereference spec: %w", err)
		}
		derefAttr, specRest, err := decodeOctetString(spec)
		if err != nil {
			return fmt.Errorf("dereference attribute: %w", err)
		}
		if strings.TrimSpace(derefAttr) == "" {
			return fmt.Errorf("dereference attribute is required")
		}
		attrs, specRest, err := decodeTaggedContent(specRest, tagSequence)
		if err != nil {
			return fmt.Errorf("dereference attribute list: %w", err)
		}
		for len(attrs) > 0 {
			attr, next, err := decodeOctetString(attrs)
			if err != nil {
				return fmt.Errorf("dereference returned attribute: %w", err)
			}
			if strings.TrimSpace(attr) == "" {
				return fmt.Errorf("dereference returned attribute is required")
			}
			attrs = next
		}
		if len(specRest) != 0 {
			return fmt.Errorf("dereference spec has trailing data")
		}
	}
	return nil
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

func isSupportedControl(controlType string) bool {
	switch strings.TrimSpace(controlType) {
	case controlManageDsaIT, controlPagedResults, controlServerSideSortRequest, controlVirtualListViewRequest, controlAssertion, controlMatchedValues,
		controlDomainScope, controlDontUseCopy, controlDontUseCopyOpenLDAP, controlSubentries, controlSyncRequest, controlProxiedAuthorization,
		controlDereferenceRequest, controlRelax, controlNoOp, controlPreRead, controlPostRead, controlPasswordPolicy, controlSessionTracking:
		return true
	default:
		return false
	}
}

const (
	controlManageDsaIT             = "2.16.840.1.113730.3.4.2"
	controlPagedResults            = "1.2.840.113556.1.4.319"
	controlServerSideSortRequest   = "1.2.840.113556.1.4.473"
	controlServerSideSortResponse  = "1.2.840.113556.1.4.474"
	controlVirtualListViewRequest  = "2.16.840.1.113730.3.4.9"
	controlVirtualListViewResponse = "2.16.840.1.113730.3.4.10"
	controlAssertion               = "1.3.6.1.1.12"
	controlMatchedValues           = "1.2.826.0.1.3344810.2.3"
	controlDomainScope             = "1.2.840.113556.1.4.1339"
	controlDontUseCopy             = "1.3.6.1.1.22"
	controlDontUseCopyOpenLDAP     = "1.3.6.1.4.1.4203.666.5.15"
	controlSubentries              = "1.3.6.1.4.1.4203.1.10.1"
	controlSyncRequest             = "1.3.6.1.4.1.4203.1.9.1.1"
	controlSyncState               = "1.3.6.1.4.1.4203.1.9.1.2"
	controlSyncDone                = "1.3.6.1.4.1.4203.1.9.1.3"
	controlProxiedAuthorization    = "2.16.840.1.113730.3.4.18"
	controlDereferenceRequest      = "1.3.6.1.4.1.4203.666.5.16"
	controlRelax                   = "1.3.6.1.4.1.4203.666.5.12"
	controlNoOp                    = "1.3.6.1.4.1.4203.666.5.2"
	controlPreRead                 = "1.3.6.1.1.13.1"
	controlPostRead                = "1.3.6.1.1.13.2"
	controlPasswordPolicy          = "1.3.6.1.4.1.42.2.27.8.5.1"
	controlSessionTracking         = "1.3.6.1.4.1.21008.108.63.1"
	syncModeRefreshOnly            = 1
	syncModeRefreshAndPersist      = 3
	syncStateAdd                   = 1
)

func (s *LDAPServer) observe(ctx context.Context, opTag int, resultCode int, entries int, remoteAddr net.Addr, err error) {
	result := MetricAccepted
	errText := ""
	if resultCode != resultSuccess && resultCode != resultSizeLimitExceeded {
		result = MetricRejected
	}
	if err != nil {
		result = MetricRejected
		errText = err.Error()
	}
	remote := ""
	if remoteAddr != nil {
		remote = remoteAddr.String()
	}
	metricsOrDefault(s.metrics).ObserveLDAP(ctx, MetricEvent{
		Operation:  ldapOperationName(opTag),
		Result:     result,
		ResultCode: resultCode,
		RemoteAddr: remote,
		Entries:    entries,
		Error:      errText,
	})
}

func ldapOperationName(opTag int) string {
	switch opTag {
	case opBindRequest:
		return "bind"
	case opSearchRequest:
		return "search"
	case opUnbindRequest:
		return "unbind"
	case opExtendedRequest:
		return "extended"
	case opModifyRequest:
		return "modify"
	case opAddRequest:
		return "add"
	case opDeleteRequest:
		return "delete"
	case opModDNRequest:
		return "modify_dn"
	case opCompareRequest:
		return "compare"
	case opAbandonRequest:
		return "abandon"
	default:
		return fmt.Sprintf("op_%02x", opTag)
	}
}

func normalizeDNForCompare(dn string) string {
	parts := strings.Split(strings.TrimSpace(dn), ",")
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	return strings.Join(parts, ",")
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
	timeLimit, rest, err = decodeInt(rest)
	if err != nil {
		err = fmt.Errorf("decode timeLimit: %w", err)
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
	attrs, _, err = decodeAttributeDescriptionList(rest)
	if err != nil {
		err = fmt.Errorf("decode attributes: %w", err)
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
		return nil, data, nil
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
			attrHeaderLen := len(content[pos+1:]) - len(attrRest)
			if err != nil || len(attrRest) < attrLen {
				break
			}
			attr := string(attrRest[:attrLen])
			attrs = append(attrs, attr)
			pos += 1 + attrHeaderLen + attrLen
		} else {
			break
		}
	}
	return attrs, data[totalLen:], nil
}

func rootDSEAttributes(namingContexts []string, startTLSEnabled bool) map[string][]string {
	if len(namingContexts) == 0 {
		namingContexts = []string{"dc=local"}
	}
	defaultNamingContext := namingContexts[0]
	configurationNamingContext := "cn=Configuration," + defaultNamingContext
	attrs := map[string][]string{
		"objectClass":                   {"top", "OpenLDAProotDSE"},
		"namingContexts":                namingContexts,
		"defaultNamingContext":          {defaultNamingContext},
		"rootDomainNamingContext":       {defaultNamingContext},
		"configurationNamingContext":    {configurationNamingContext},
		"schemaNamingContext":           {"cn=Schema," + configurationNamingContext},
		"subschemaSubentry":             {"cn=Subschema"},
		"supportedLDAPVersion":          {"3"},
		"supportedControl":              {controlManageDsaIT, controlPagedResults, controlServerSideSortRequest, controlVirtualListViewRequest, controlAssertion, controlMatchedValues, controlDomainScope, controlDontUseCopy, controlDontUseCopyOpenLDAP, controlSubentries, controlSyncRequest, controlProxiedAuthorization, controlDereferenceRequest, controlRelax, controlNoOp, controlPreRead, controlPostRead, controlPasswordPolicy, controlSessionTracking},
		"supportedExtension":            {whoAmIOID},
		"supportedFeatures":             {ldapFeatureAllOperationalAttributes},
		"supportedCapabilities":         {"1.2.840.113556.1.4.800", "1.2.840.113556.1.4.1670", "1.2.840.113556.1.4.1791"},
		"vendorName":                    {"gogomail"},
		"dnsHostName":                   {"ldap." + ldapDNSDomainFromNamingContext(defaultNamingContext)},
		"domainControllerFunctionality": {"7"},
		"domainFunctionality":           {"7"},
		"forestFunctionality":           {"7"},
		"isGlobalCatalogReady":          {"TRUE"},
		"isSynchronized":                {"TRUE"},
	}
	if startTLSEnabled {
		attrs["supportedExtension"] = append(attrs["supportedExtension"], startTLSOID)
	}
	return attrs
}

func ldapDNSDomainFromNamingContext(namingContext string) string {
	var labels []string
	for _, rdn := range splitDNComponents(namingContext) {
		parts := strings.SplitN(rdn, "=", 2)
		if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), "dc") {
			continue
		}
		label, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
		if !ok {
			continue
		}
		label = strings.TrimSpace(label)
		if label != "" {
			labels = append(labels, label)
		}
	}
	if len(labels) == 0 {
		return "gogomail.local"
	}
	return strings.Join(labels, ".")
}

func subschemaAttributes() map[string][]string {
	return map[string][]string{
		"objectClass": {"top", "subschema"},
		"objectClasses": {
			"( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )",
			"( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )",
			"( 2.5.6.7 NAME 'organizationalPerson' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )",
			"( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ givenName $ displayName $ name $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ mailNickname $ proxyAddresses $ sAMAccountName $ userPrincipalName $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged $ accountExpires $ primaryGroupID $ userAccountControl ) )",
			"( 1.2.840.113556.1.5.9 NAME 'user' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ givenName $ displayName $ name $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ mailNickname $ proxyAddresses $ sAMAccountName $ userPrincipalName $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged $ accountExpires $ primaryGroupID $ userAccountControl ) )",
			"( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
			"( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description $ memberOf $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
			"( 2.5.6.14 NAME 'device' SUP top STRUCTURAL MUST cn MAY ( serialNumber $ seeAlso $ owner $ ou $ o $ l $ description $ memberOf $ canonicalName $ distinguishedName $ instanceType $ objectCategory $ objectGUID $ objectSid $ whenCreated $ whenChanged $ uSNCreated $ uSNChanged ) )",
		},
		"attributeTypes": {
			"( 2.5.4.3 NAME 'cn' SUP name )",
			"( 2.5.4.4 NAME 'sn' SUP name )",
			"( 2.5.4.11 NAME 'ou' SUP name )",
			"( 2.5.4.13 NAME 'description' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 2.5.4.42 NAME 'givenName' SUP name )",
			"( 2.5.4.41 NAME 'name' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 0.9.2342.19200300.100.1.1 NAME 'uid' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 0.9.2342.19200300.100.1.3 NAME 'mail' EQUALITY caseIgnoreIA5Match SUBSTR caseIgnoreIA5SubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.26 )",
			"( 2.16.840.1.113730.3.1.241 NAME 'displayName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.221 NAME 'sAMAccountName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.656 NAME 'userPrincipalName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.7000.102.1 NAME 'mailNickname' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.2.210 NAME 'proxyAddresses' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.916 NAME 'canonicalName' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 2.5.4.49 NAME 'distinguishedName' EQUALITY distinguishedNameMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.12 )",
			"( 1.2.840.113556.1.2.1 NAME 'instanceType' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.782 NAME 'objectCategory' EQUALITY caseIgnoreMatch SUBSTR caseIgnoreSubstringsMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.15 )",
			"( 1.2.840.113556.1.4.2 NAME 'objectGUID' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )",
			"( 1.2.840.113556.1.4.146 NAME 'objectSid' EQUALITY octetStringMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.40 )",
			"( 1.2.840.113556.1.2.2 NAME 'whenCreated' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )",
			"( 1.2.840.113556.1.2.3 NAME 'whenChanged' EQUALITY generalizedTimeMatch ORDERING generalizedTimeOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.24 )",
			"( 1.2.840.113556.1.2.19 NAME 'uSNCreated' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.2.120 NAME 'uSNChanged' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.159 NAME 'accountExpires' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.98 NAME 'primaryGroupID' EQUALITY integerMatch ORDERING integerOrderingMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
			"( 1.2.840.113556.1.4.8 NAME 'userAccountControl' EQUALITY integerMatch SYNTAX 1.3.6.1.4.1.1466.115.121.1.27 )",
		},
	}
}

func selectLDAPAttributes(attrs map[string][]string, requested []string, typesOnly bool) map[string][]string {
	selected := make(map[string][]string, len(attrs))
	if containsLDAPAttribute(requested, "1.1") {
		return selected
	}
	if len(requested) == 0 || containsLDAPAttribute(requested, "*") {
		copyLDAPAttributeSet(selected, attrs, typesOnly, false)
	}
	if containsLDAPAttribute(requested, "+") {
		copyLDAPAttributeSet(selected, attrs, typesOnly, true)
	}
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if name == "*" || name == "+" {
			continue
		}
		for attrName, values := range attrs {
			if strings.EqualFold(name, attrName) {
				if typesOnly {
					selected[attrName] = nil
				} else {
					selected[attrName] = values
				}
			}
		}
	}
	return selected
}

func copyLDAPAttributeSet(dst map[string][]string, attrs map[string][]string, typesOnly bool, operational bool) {
	for k, values := range attrs {
		if operational != isOperationalLDAPAttribute(k) {
			continue
		}
		if typesOnly {
			dst[k] = nil
		} else {
			dst[k] = values
		}
	}
}

func isOperationalLDAPAttribute(attr string) bool {
	switch strings.ToLower(strings.TrimSpace(attr)) {
	case "subschemasubentry", "supportedldapversion", "supportedcontrol", "supportedextension", "supportedfeatures", "namingcontexts", "vendorname",
		"defaultnamingcontext", "rootdomainnamingcontext", "configurationnamingcontext", "schemanamingcontext", "supportedcapabilities",
		"dnshostname", "domaincontrollerfunctionality", "domainfunctionality", "forestfunctionality", "isglobalcatalogready", "issynchronized",
		"entrydn", "entryuuid", "createtimestamp", "modifytimestamp", "creatorsname", "modifiersname",
		"distinguishedname", "objectguid", "objectsid", "instancetype", "whencreated", "whenchanged",
		"usncreated", "usnchanged", "hassubordinates", "numsubordinates":
		return true
	default:
		return false
	}
}

func containsLDAPAttribute(attrs []string, want string) bool {
	for _, attr := range attrs {
		if strings.EqualFold(strings.TrimSpace(attr), want) {
			return true
		}
	}
	return false
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
	return string(rest[:length]), nil
}

func parseLDAPFilter(data []byte) (string, error) {
	attr, value, ok, err := parseLDAPFilterCandidate(data)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return fmt.Sprintf("(%s=%s)", attr, value), nil
}

func parseLDAPFilterPrincipalKinds(data []byte) ([]string, error) {
	seen := map[string]struct{}{}
	if err := collectLDAPFilterPrincipalKinds(data, seen); err != nil {
		return nil, err
	}
	if len(seen) == 0 {
		return nil, nil
	}
	order := []string{"user", "organization", "group", "resource"}
	kinds := make([]string, 0, len(seen))
	for _, kind := range order {
		if _, ok := seen[kind]; ok {
			kinds = append(kinds, kind)
		}
	}
	return kinds, nil
}

func collectLDAPFilterPrincipalKinds(data []byte, seen map[string]struct{}) error {
	if len(data) == 0 {
		return nil
	}
	if data[0]&0x80 == 0 {
		return fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	content, err := decodeContent(data[1:])
	if err != nil {
		return err
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return err
			}
			if err := collectLDAPFilterPrincipalKinds(child, seen); err != nil {
				return err
			}
			content = rest
		}
	case filterOr:
		for len(content) > 0 {
			_, rest, err := readRawTLV(content)
			if err != nil {
				return err
			}
			content = rest
		}
		return nil
	case filterNot:
		if _, _, err := readRawTLV(content); err != nil {
			return err
		}
		return nil
	case filterEqualityMatch, filterApproxMatch:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return err
		}
		value, _, err := decodeOctetString(valRest)
		if err != nil {
			return err
		}
		if principalKindAttr := strings.TrimSpace(attr); strings.EqualFold(principalKindAttr, "objectClass") || strings.EqualFold(principalKindAttr, "objectCategory") {
			for _, kind := range principalKindsForObjectClass(value) {
				seen[kind] = struct{}{}
			}
		}
	case filterExtensible:
		attr, value, ok, err := decodeExtensibleMatch(content)
		if err != nil {
			return err
		}
		if principalKindAttr := strings.TrimSpace(attr); ok && (strings.EqualFold(principalKindAttr, "objectClass") || strings.EqualFold(principalKindAttr, "objectCategory")) {
			for _, kind := range principalKindsForObjectClass(value) {
				seen[kind] = struct{}{}
			}
		}
	case filterPresent, filterSubstrings, filterGreaterOrEqual, filterLessOrEqual:
		return nil
	default:
		return fmt.Errorf("unsupported filter type: %d", filterType)
	}
	return nil
}

func principalKindsForObjectClass(value string) []string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "person", "organizationalperson", "inetorgperson", "user":
		return []string{"user"}
	case "organizationalunit", "organizational-unit", "organization":
		return []string{"organization"}
	case "group", "groupofnames", "groupofuniquenames", "posixgroup":
		return []string{"group"}
	case "device", "resource":
		return []string{"resource"}
	default:
		return nil
	}
}

func parseLDAPFilterCandidate(data []byte) (attr string, value string, ok bool, err error) {
	if len(data) == 0 {
		return "", "", false, nil
	}
	if data[0]&0x80 == 0 {
		return "", "", false, fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	content, err := decodeContent(data[1:])
	if err != nil {
		return "", "", false, err
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return "", "", false, err
			}
			childAttr, childValue, childOK, err := parseLDAPFilterCandidate(child)
			if err != nil {
				return "", "", false, err
			}
			if childOK && isDirectorySearchAttribute(childAttr) && strings.Trim(childValue, "*") != "" {
				return childAttr, childValue, true, nil
			}
			content = rest
		}
		return "", "", false, nil
	case filterOr:
		for len(content) > 0 {
			_, rest, err := readRawTLV(content)
			if err != nil {
				return "", "", false, err
			}
			content = rest
		}
		return "", "", false, nil
	case filterNot:
		if _, _, err := readRawTLV(content); err != nil {
			return "", "", false, err
		}
		return "", "", false, nil
	case filterEqualityMatch, filterApproxMatch:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed attribute assertion: %w", err)
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed assertion value: %w", err)
		}
		return attr, val, true, nil
	case filterGreaterOrEqual, filterLessOrEqual:
		_, valRest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed attribute assertion: %w", err)
		}
		if _, _, err := decodeOctetString(valRest); err != nil {
			return "", "", false, fmt.Errorf("malformed assertion value: %w", err)
		}
		return "", "", false, nil
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed substring attribute: %w", err)
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return "", "", false, err
		}
		return attr, strings.Join(parts, "*"), len(parts) > 0, nil
	case filterPresent:
		return string(content), "*", true, nil
	case filterExtensible:
		return decodeExtensibleMatch(content)
	default:
		return "", "", false, fmt.Errorf("unsupported filter type: %d", filterType)
	}
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

func isDirectorySearchAttribute(attr string) bool {
	switch strings.ToLower(strings.TrimSpace(attr)) {
	case "cn", "mail", "uid", "displayname", "givenname", "sn", "ou", "description", "name", "canonicalname", "samaccountname", "userprincipalname", "mailnickname", "proxyaddresses":
		return true
	default:
		return false
	}
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
		filterPresent, filterApproxMatch, filterExtensible:
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
