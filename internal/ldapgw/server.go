package ldapgw

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net"
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
	if s == nil {
		return errors.New("ldap server is nil")
	}
	if s.ln == nil {
		return errors.New("ldap listener is required")
	}
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
	if s == nil {
		return nil
	}
	s.closeMu.Lock()
	if s.closed {
		s.closeMu.Unlock()
		return nil
	}
	s.closed = true
	ln := s.ln
	cancel := s.cancel
	s.closeMu.Unlock()
	if cancel != nil {
		cancel()
	}
	if ln == nil {
		return nil
	}
	return ln.Close()
}

func (s *LDAPServer) handleConn(ctx context.Context, conn net.Conn) {
	defer conn.Close()
	buf := make([]byte, 0, 8192)
	readBuf := make([]byte, 8192)
	tlsActive := false
	authenticated := false
	authzID := ""
	var writeMu sync.Mutex
	activeOps := make(map[int]context.CancelFunc)
	var activeOpsMu sync.Mutex
	defer func() {
		activeOpsMu.Lock()
		for _, cancel := range activeOps {
			cancel()
		}
		activeOpsMu.Unlock()
	}()
	writeResponse := func(resp []byte) bool {
		if len(resp) == 0 {
			return true
		}
		writeMu.Lock()
		defer writeMu.Unlock()
		_, err := conn.Write(resp)
		return err == nil
	}
	storeActiveOp := func(messageID int, cancel context.CancelFunc) {
		activeOpsMu.Lock()
		activeOps[messageID] = cancel
		activeOpsMu.Unlock()
	}
	removeActiveOp := func(messageID int) {
		activeOpsMu.Lock()
		delete(activeOps, messageID)
		activeOpsMu.Unlock()
	}
	cancelActiveOp := func(messageID int) {
		activeOpsMu.Lock()
		cancel := activeOps[messageID]
		activeOpsMu.Unlock()
		if cancel != nil {
			cancel()
		}
	}
	cancelAllActiveOps := func() {
		activeOpsMu.Lock()
		for _, cancel := range activeOps {
			cancel()
		}
		activeOpsMu.Unlock()
	}
	hasActiveOps := func() bool {
		activeOpsMu.Lock()
		defer activeOpsMu.Unlock()
		return len(activeOps) > 0
	}

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
				s.observe(ctx, 0, resultProtocolError, 0, conn.RemoteAddr(), fmt.Errorf("BER message size exceeds maximum %d", maxBERMessageSize))
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
				s.observe(ctx, 0, resultProtocolError, 0, conn.RemoteAddr(), pduErr)
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
				writeResponse(resp)
				s.observe(ctx, opTag, resultUnwillingToPerform, 0, conn.RemoteAddr(), err)
				return
			}
			if ctrl, ok := firstUnsupportedCriticalControl(controls); ok {
				result := resultUnavailableCriticalExtension
				resp := encodeControlErrorResponse(msgID, opTag, result, "unsupported critical control: "+ctrl.Type)
				if !writeResponse(resp) {
					return
				}
				s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
				continue
			}
			if opTag == opExtendedRequest && isStartTLSRequest(opData) {
				if tlsActive {
					result := resultOperationsError
					resp := encodeExtendedResponse(msgID, result, "", "TLS already active")
					if !writeResponse(resp) {
						return
					}
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					continue
				}
				if hasActiveOps() {
					result := resultOperationsError
					resp := encodeExtendedResponse(msgID, result, "", "operations outstanding")
					if !writeResponse(resp) {
						return
					}
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					continue
				}
				if s.tlsConfig == nil {
					result := resultUnavailable
					resp := encodeExtendedResponse(msgID, result, "", "StartTLS not configured")
					if !writeResponse(resp) {
						return
					}
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					continue
				}
				if len(buf) != 0 {
					result := resultProtocolError
					resp := encodeExtendedResponse(msgID, result, "", "StartTLS must be the last plaintext operation")
					writeResponse(resp)
					s.observe(ctx, opTag, result, 0, conn.RemoteAddr(), nil)
					return
				}
				result := resultSuccess
				resp := encodeExtendedResponse(msgID, result, "", "")
				if !writeResponse(resp) {
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
			if opTag == opAbandonRequest {
				if targetID, ok := decodeAbandonRequestMessageID(opData); ok {
					cancelActiveOp(targetID)
				}
				s.observe(ctx, opTag, resultSuccess, 0, conn.RemoteAddr(), nil)
				continue
			}
			if opTag == opSearchRequest {
				opCtx, cancel := context.WithCancel(ctx)
				storeActiveOp(msgID, cancel)
				authenticatedSnapshot := authenticated
				authzIDSnapshot := authzID
				go func(messageID int, operationTag int, data []byte, ctrls []control, authn bool, authz string) {
					defer removeActiveOp(messageID)
					defer cancel()
					resp, resultCode, entries, _ := s.handleOperation(opCtx, messageID, operationTag, data, ctrls, authn, authz)
					if opCtx.Err() == context.Canceled {
						s.observe(ctx, operationTag, resultSuccess, 0, conn.RemoteAddr(), opCtx.Err())
						return
					}
					if !writeResponse(resp) {
						return
					}
					s.observe(ctx, operationTag, resultCode, entries, conn.RemoteAddr(), nil)
				}(msgID, opTag, opData, controls, authenticatedSnapshot, authzIDSnapshot)
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
			if !writeResponse(resp) {
				return
			}
			s.observe(ctx, opTag, resultCode, entries, conn.RemoteAddr(), nil)
			if opTag == opUnbindRequest {
				cancelAllActiveOps()
				return
			}
		}
	}
}

func (s *LDAPServer) handleOperation(ctx context.Context, msgID int, opTag int, opData []byte, controls []control, authenticated bool, authzID string) ([]byte, int, int, bool) {
	switch opTag {
	case opBindRequest:
		resp, result, authOK := s.handleBindRequest(ctx, msgID, opData)
		return resp, result, 0, authOK
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

func (s *LDAPServer) handleBindRequest(ctx context.Context, msgID int, opData []byte) ([]byte, int, bool) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeBindResponse(msgID, result, "", "operation timed out"), result, false
	default:
	}

	req, err := decodeBindRequestData(opData)
	if err != nil {
		if errors.Is(err, errUnsupportedBindAuth) {
			result := resultAuthMethodNotSupported
			return encodeBindResponse(msgID, result, "", "unsupported bind authentication method"), result, false
		}
		result := resultUnwillingToPerform
		return encodeBindResponse(msgID, result, "", "malformed bind request"), result, false
	}
	if req.version != ldapV3 {
		result := resultAuthMethodNotSupported
		return encodeBindResponse(msgID, result, "", "unsupported LDAP version"), result, false
	}
	if strings.TrimSpace(req.name) == "" && len(req.auth) == 0 {
		return encodeBindResponse(msgID, resultSuccess, "", ""), resultSuccess, false
	}

	ok, err := s.authenticateBindIdentity(ctx, req.name, string(req.auth))
	if err != nil || !ok {
		result := resultInvalidCredentials
		return encodeBindResponse(msgID, result, "", "invalid credentials"), result, false
	}
	return encodeBindResponse(msgID, resultSuccess, "", ""), resultSuccess, true
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
	components := splitDNComponents(dn)
	if len(components) == 0 {
		return "", "", false
	}
	first := strings.TrimSpace(components[0])
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
	assertionLen, assertionRest, err := decodeLength(rest[1:])
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare assertion: %w", err)
	}
	assertionHeaderLen := len(rest) - len(assertionRest)
	assertionTotalLen := assertionHeaderLen + assertionLen
	if len(rest) < assertionTotalLen {
		return compareRequest{}, fmt.Errorf("compare assertion truncated")
	}
	if len(rest) != assertionTotalLen {
		return compareRequest{}, fmt.Errorf("compare request trailing data")
	}
	assertion := rest[assertionHeaderLen:assertionTotalLen]
	attr, valueRest, err := decodeOctetString(assertion)
	if err != nil {
		return compareRequest{}, fmt.Errorf("decode compare attribute: %w", err)
	}
	if ldapAttributeType(attr) == "" {
		return compareRequest{}, fmt.Errorf("compare attribute description empty")
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
	want := ldapAttributeType(attr)
	for name, values := range attrs {
		if strings.EqualFold(ldapAttributeType(name), want) {
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
