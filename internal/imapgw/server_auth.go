package imapgw

import (
	"bufio"
	"encoding/base64"
	"strings"
	"sync"
	"time"
)

func imapMalformedCommandResponse(line string) string {
	if tag := imapTagFromCommandLine(line); tag != "" {
		return tag + " BAD malformed command\r\n"
	}
	return "* BAD malformed command\r\n"
}

func (s *Server) handleLogin(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if state.session != nil {
		_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
		return false, err
	}
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD LOGIN requires username and password atoms\r\n")
		return false, err
	}
	if !imapLoginCredentialsValid(fields[2], fields[3]) {
		_, err := writer.WriteString(tag + " BAD LOGIN credentials are malformed\r\n")
		return false, err
	}
	if !s.authAllowed(state) {
		_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for LOGIN\r\n")
		return false, err
	}
	if s.authTracker.isLocked(state.remoteIP) {
		_, err := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] LOGIN failed\r\n")
		return false, err
	}
	authSession, err := s.options.Backend.Authenticate(state.ctx, fields[2], fields[3])
	if err != nil {
		s.authTracker.record(state.remoteIP)
		_, writeErr := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] LOGIN failed\r\n")
		return false, writeErr
	}
	state.session = &authSession
	state.userID = fields[2]
	_, err = writer.WriteString(tag + " OK " + s.authenticatedCapabilityCode(state) + " LOGIN completed\r\n")
	return false, err
}

func (s *Server) handleAuthenticate(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if state.session != nil {
		_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
		return false, err
	}
	if len(fields) != 3 && len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE requires mechanism and optional initial response\r\n")
		return false, err
	}
	if !imapAtomValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is malformed\r\n")
		return false, err
	}
	if !strings.EqualFold(fields[2], "PLAIN") {
		_, err := writer.WriteString(tag + " NO AUTHENTICATE mechanism is unsupported\r\n")
		return false, err
	}
	if !s.authAllowed(state) {
		_, err := writer.WriteString(tag + " NO [PRIVACYREQUIRED] TLS is required for AUTHENTICATE\r\n")
		return false, err
	}
	if len(fields) == 4 {
		if _, _, ok := decodeSASLPlain(fields[3]); !ok {
			_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
			return false, err
		}
	}
	if len(fields) == 4 {
		return s.completeAuthenticatePlain(writer, tag, fields[3], state)
	}
	state.pendingAuthTag = tag
	_, err := writer.WriteString("+ \r\n")
	return false, err
}

func (s *Server) handleStartTLS(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 2 {
		_, err := writer.WriteString(tag + " BAD STARTTLS does not accept arguments\r\n")
		return false, err
	}
	if state.session != nil {
		_, err := writer.WriteString(tag + " BAD already authenticated\r\n")
		return false, err
	}
	if state.tlsActive || s.options.TLSConfig == nil {
		_, err := writer.WriteString(tag + " BAD STARTTLS is unavailable\r\n")
		return false, err
	}
	state.startTLS = true
	tlsState := *state
	tlsState.startTLS = false
	tlsState.tlsActive = true
	_, err := writer.WriteString(tag + " OK [CAPABILITY " + strings.Join(s.imapCapabilities(&tlsState), " ") + "] Begin TLS negotiation now\r\n")
	return false, err
}

func (s *Server) handleAuthenticatePlainResponse(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingAuthTag
	state.pendingAuthTag = ""
	if line == "*" {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE canceled\r\n")
		return false, err
	}
	return s.completeAuthenticatePlain(writer, tag, line, state)
}

func (s *Server) completeAuthenticatePlain(writer *bufio.Writer, tag string, value string, state *imapConnState) (bool, error) {
	username, password, ok := decodeSASLPlain(value)
	if !ok {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
		return false, err
	}
	if s.authTracker.isLocked(state.remoteIP) {
		_, writeErr := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] AUTHENTICATE failed\r\n")
		return false, writeErr
	}
	authSession, err := s.options.Backend.Authenticate(state.ctx, username, password)
	if err != nil {
		s.authTracker.record(state.remoteIP)
		_, writeErr := writer.WriteString(tag + " NO [AUTHENTICATIONFAILED] AUTHENTICATE failed\r\n")
		return false, writeErr
	}
	state.session = &authSession
	state.userID = username
	_, err = writer.WriteString(tag + " OK " + s.authenticatedCapabilityCode(state) + " AUTHENTICATE completed\r\n")
	return false, err
}

func decodeSASLPlain(value string) (string, string, bool) {
	if value == "" || strings.TrimSpace(value) != value {
		return "", "", false
	}
	if len(value) > maxIMAPSASLPlainEncodedBytes {
		return "", "", false
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return "", "", false
	}
	if len(decoded) > maxIMAPSASLPlainDecodedBytes {
		return "", "", false
	}
	parts := strings.Split(string(decoded), "\x00")
	if len(parts) != 3 || parts[1] == "" {
		return "", "", false
	}
	if parts[0] != "" && parts[0] != parts[1] {
		return "", "", false
	}
	if !imapAuthCredentialsValid(parts[1], parts[2]) {
		return "", "", false
	}
	return parts[1], parts[2], true
}

func imapAuthCredentialsValid(username string, password string) bool {
	return imapAuthCredentialsValidWithEmptyPassword(username, password, false)
}

func imapLoginCredentialsValid(username string, password string) bool {
	return imapAuthCredentialsValidWithEmptyPassword(username, password, true)
}

func imapAuthCredentialsValidWithEmptyPassword(username string, password string, allowEmptyPassword bool) bool {
	if strings.ContainsAny(username, "\r\n") || strings.ContainsAny(password, "\r\n") {
		return false
	}
	username = strings.TrimSpace(username)
	if username == "" || (!allowEmptyPassword && password == "") || len(username) > maxIMAPAuthIdentityBytes || len(password) > maxIMAPAuthPasswordBytes {
		return false
	}
	return true
}

// authFailureTracker tracks per-IP auth failures for brute-force protection.
type authFailureTracker struct {
	mu       sync.Mutex
	failures map[string][]time.Time
	window   time.Duration
	maxFails int
}

func newAuthFailureTracker() *authFailureTracker {
	return &authFailureTracker{
		failures: make(map[string][]time.Time),
		window:   10 * time.Minute,
		maxFails: 10,
	}
}

func (t *authFailureTracker) record(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-t.window)
	prev := t.failures[ip]
	fresh := prev[:0]
	for _, ts := range prev {
		if ts.After(cutoff) {
			fresh = append(fresh, ts)
		}
	}
	fresh = append(fresh, now)
	if len(prev) > 0 && len(fresh) == 1 {
		t.failures[ip] = []time.Time{now}
	} else {
		t.failures[ip] = fresh
	}
	return len(fresh) > t.maxFails
}

func (t *authFailureTracker) isLocked(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	now := time.Now()
	cutoff := now.Add(-t.window)
	var count int
	for _, ts := range t.failures[ip] {
		if ts.After(cutoff) {
			count++
		}
	}
	if count == 0 {
		delete(t.failures, ip)
	}
	return count >= t.maxFails
}
