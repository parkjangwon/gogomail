package ldapgw

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"strings"
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
	buf := make([]byte, 8192)
	readOffset := 0
	tlsActive := false
	authenticated := false

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
				if readOffset != 0 {
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
			resp, resultCode, entries, authOK := s.handleOperation(ctx, msgID, opTag, opData, controls, authenticated)
			if authOK {
				authenticated = true
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

func (s *LDAPServer) handleOperation(ctx context.Context, msgID int, opTag int, opData []byte, controls []control, authenticated bool) ([]byte, int, int, bool) {
	switch opTag {
	case opBindRequest:
		resp, result := s.handleBindRequest(ctx, msgID, opData)
		return resp, result, 0, result == resultSuccess
	case opSearchRequest:
		if !authenticated && !isPublicDiscoverySearch(opData) {
			result := resultInsufficientAccessRights
			return encodeSearchResultDone(msgID, result, "", "bind required"), result, 0, false
		}
		resp, result, entries := s.handleSearchRequest(ctx, msgID, opData, controls)
		return resp, result, entries, false
	case opUnbindRequest:
		return nil, resultSuccess, 0, false
	case opExtendedRequest:
		result := resultUnwillingToPerform
		return encodeExtendedResponse(msgID, result, "", "extended operation not supported"), result, 0, false
	default:
		result := resultUnwillingToPerform
		return encodeLDAPResponse(msgID, opTag, mustEncodeNotSupported()), result, 0, false
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

func (s *LDAPServer) handleSearchRequest(ctx context.Context, msgID int, opData []byte, controls []control) ([]byte, int, int) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "operation timed out"), result, 0
	default:
	}

	baseObject, scope, filter, attrs, sizeLimit, _, typesOnly, err := decodeSearchRequest(opData)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	if baseObject == "" && scope == scopeBaseObject {
		entry, err := encodeSearchResultEntry(msgID, "", selectLDAPAttributes(rootDSEAttributes(s.namingContexts, s.tlsConfig != nil), attrs, typesOnly))
		if err != nil {
			result := resultUnwillingToPerform
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
		}
		return append(entry, encodeSearchResultDone(msgID, resultSuccess, "", "")...), resultSuccess, 1
	}
	if normalizeDNForCompare(baseObject) == "cn=subschema" && scope == scopeBaseObject {
		entry, err := encodeSearchResultEntry(msgID, "cn=Subschema", selectLDAPAttributes(subschemaAttributes(), attrs, typesOnly))
		if err != nil {
			result := resultUnwillingToPerform
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
		}
		return append(entry, encodeSearchResultDone(msgID, resultSuccess, "", "")...), resultSuccess, 1
	}
	if containerAttrs, ok := ldapContainerAttributes(baseObject); ok && scope == scopeBaseObject {
		entry, err := encodeSearchResultEntry(msgID, baseObject, selectLDAPAttributes(containerAttrs, attrs, typesOnly))
		if err != nil {
			result := resultUnwillingToPerform
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
		}
		return append(entry, encodeSearchResultDone(msgID, resultSuccess, "", "")...), resultSuccess, 1
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

	if err := validateFilter(filter); err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", fmt.Sprintf("invalid filter: %v", err)), result, 0
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

	limit := 100
	if paged && pageSize > 0 {
		limit = pageSize + 1
	}
	principals, err := s.quer.SearchPrincipals(ctx, DirectorySearchRequest{
		BaseDN: baseObject,
		Scope:  scope,
		Filter: ldapFilter,
		Attrs:  attrs,
		Kinds:  kinds,
		Limit:  limit,
		Offset: pageOffset,
	})
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	principals = filterPrincipalEntriesByScope(principals, baseObject, scope)
	if sizeLimit > 0 && len(principals) > sizeLimit {
		principals = principals[:sizeLimit]
	}
	nextCookie := ""
	if paged && pageSize > 0 && len(principals) > pageSize {
		principals = principals[:pageSize]
		nextCookie = fmt.Sprintf("%d", pageOffset+pageSize)
	}

	resp := make([]byte, 0, 4096)
	for _, p := range principals {
		attrMap := principalLDAPAttributes(p)
		entry, err := encodeSearchResultEntry(msgID, p.DN, selectLDAPAttributes(attrMap, attrs, typesOnly))
		if err != nil {
			continue
		}
		resp = append(resp, entry...)
	}
	result := resultSuccess
	if sizeLimit > 0 && len(principals) == sizeLimit {
		result = resultSizeLimitExceeded
	}
	if paged {
		resp = append(resp, encodeSearchResultDoneWithControls(msgID, result, "", "", []control{pagedResultsResponseControl(nextCookie)})...)
	} else {
		resp = append(resp, encodeSearchResultDone(msgID, result, "", "")...)
	}
	return resp, result, len(principals)
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

func parentDN(dn string) string {
	parts := strings.SplitN(dn, ",", 2)
	if len(parts) != 2 {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func ldapContainerAttributes(dn string) (map[string][]string, bool) {
	switch firstRDNValue(normalizeDNForCompare(dn), "ou") {
	case "users":
		return map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"users"}, "cn": {"users"}, "displayName": {"Users"}}, true
	case "organizations":
		return map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"organizations"}, "cn": {"organizations"}, "displayName": {"Organizations"}}, true
	case "groups":
		return map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"groups"}, "cn": {"groups"}, "displayName": {"Groups"}}, true
	case "resources":
		return map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"resources"}, "cn": {"resources"}, "displayName": {"Resources"}}, true
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
	attrs := map[string][]string{
		"cn":          {cn},
		"uid":         {p.UID},
		"displayName": {firstNonEmpty(p.DisplayName, cn)},
	}
	switch kind {
	case "organization":
		attrs["objectClass"] = []string{"top", "organizationalUnit"}
		attrs["ou"] = []string{firstNonEmpty(p.OU, p.DisplayName, cn)}
	case "group":
		attrs["objectClass"] = []string{"top", "groupOfNames"}
		attrs["member"] = []string{firstNonEmpty(p.DN, "cn=placeholder")}
	case "resource":
		attrs["objectClass"] = []string{"top", "device"}
		if p.ResourceType != "" {
			attrs["description"] = []string{p.ResourceType}
		}
	default:
		attrs["objectClass"] = []string{"top", "person", "organizationalPerson", "inetOrgPerson"}
		if p.Mail != "" {
			attrs["mail"] = []string{p.Mail}
		}
		if p.GivenName != "" {
			attrs["givenName"] = []string{p.GivenName}
		}
		attrs["sn"] = []string{firstNonEmpty(p.SN, p.DisplayName, cn)}
	}
	return attrs
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

func isSupportedControl(controlType string) bool {
	switch strings.TrimSpace(controlType) {
	case controlManageDsaIT, controlPagedResults:
		return true
	default:
		return false
	}
}

const (
	controlManageDsaIT  = "2.16.840.1.113730.3.4.2"
	controlPagedResults = "1.2.840.113556.1.4.319"
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
	scope = scopeVal

	if len(rest) < 2 {
		err = fmt.Errorf("search request: missing derefAliases")
		return
	}
	_, rest, err = decodeLDAPIntLike(rest)
	if err != nil {
		err = fmt.Errorf("decode derefAliases: %w", err)
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
	attrs := map[string][]string{
		"objectClass":          {"top", "OpenLDAProotDSE"},
		"namingContexts":       namingContexts,
		"subschemaSubentry":    {"cn=Subschema"},
		"supportedLDAPVersion": {"3"},
		"supportedControl":     {controlManageDsaIT, controlPagedResults},
		"vendorName":           {"gogomail"},
	}
	if startTLSEnabled {
		attrs["supportedExtension"] = []string{startTLSOID}
	}
	return attrs
}

func subschemaAttributes() map[string][]string {
	return map[string][]string{
		"objectClass": {"top", "subschema"},
		"objectClasses": {
			"( 2.5.6.0 NAME 'top' ABSTRACT MUST objectClass )",
			"( 2.5.6.6 NAME 'person' SUP top STRUCTURAL MUST ( sn $ cn ) MAY ( userPassword $ telephoneNumber $ seeAlso $ description ) )",
			"( 2.5.6.7 NAME 'organizationalPerson' SUP person STRUCTURAL MAY ( title $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ ou $ st $ l ) )",
			"( 2.16.840.1.113730.3.2.2 NAME 'inetOrgPerson' SUP organizationalPerson STRUCTURAL MAY ( mail $ uid $ givenName $ displayName ) )",
			"( 2.5.6.5 NAME 'organizationalUnit' SUP top STRUCTURAL MUST ou MAY ( userPassword $ searchGuide $ seeAlso $ businessCategory $ x121Address $ registeredAddress $ destinationIndicator $ preferredDeliveryMethod $ telexNumber $ teletexTerminalIdentifier $ telephoneNumber $ internationaliSDNNumber $ facsimileTelephoneNumber $ street $ postOfficeBox $ postalCode $ postalAddress $ physicalDeliveryOfficeName $ st $ l $ description ) )",
			"( 2.5.6.9 NAME 'groupOfNames' SUP top STRUCTURAL MUST ( member $ cn ) MAY ( businessCategory $ seeAlso $ owner $ ou $ o $ description ) )",
			"( 2.5.6.14 NAME 'device' SUP top STRUCTURAL MUST cn MAY ( serialNumber $ seeAlso $ owner $ ou $ o $ l $ description ) )",
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
	case "subschemasubentry", "supportedldapversion", "supportedcontrol", "supportedextension", "namingcontexts", "vendorname":
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
	case filterAnd, filterOr:
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
	case filterNot:
		child, _, err := readRawTLV(content)
		if err != nil {
			return err
		}
		return collectLDAPFilterPrincipalKinds(child, seen)
	case filterEqualityMatch, filterApproxMatch:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return err
		}
		value, _, err := decodeOctetString(valRest)
		if err != nil {
			return err
		}
		if strings.EqualFold(strings.TrimSpace(attr), "objectClass") {
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
	case "person", "organizationalperson", "inetorgperson":
		return []string{"user"}
	case "organizationalunit":
		return []string{"organization"}
	case "groupofnames", "groupofuniquenames", "posixgroup":
		return []string{"group"}
	case "device":
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
	case filterAnd, filterOr:
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
	case filterNot:
		child, _, err := readRawTLV(content)
		if err != nil {
			return "", "", false, err
		}
		return parseLDAPFilterCandidate(child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed attribute assertion: %w", err)
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed assertion value: %w", err)
		}
		return attr, val, true, nil
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

func isDirectorySearchAttribute(attr string) bool {
	switch strings.ToLower(strings.TrimSpace(attr)) {
	case "cn", "mail", "uid", "displayname", "givenname", "sn", "ou", "description":
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
