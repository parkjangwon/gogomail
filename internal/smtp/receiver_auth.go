package smtpd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/emersion/go-sasl"
	gosmtp "github.com/emersion/go-smtp"
)

func (s *session) authorizeRelay() error {
	if s.receiver.relayAuthorizer == nil {
		return nil
	}
	allowed, err := s.receiver.relayAuthorizer.AllowRelay(s.ctx, s.remoteAddr)
	if err != nil {
		return fmt.Errorf("authorize smtp relay: %w", err)
	}
	if !allowed {
		return smtpPolicyReject("remote address %q is not trusted for this SMTP boundary", s.remoteAddr)
	}
	return nil
}

func (s *session) AuthMechanisms() []string {
	if s.receiver.authenticator == nil {
		return nil
	}
	return []string{sasl.Plain}
}

func (s *session) Auth(mech string) (sasl.Server, error) {
	if s.receiver.authenticator == nil || !strings.EqualFold(mech, sasl.Plain) {
		return nil, gosmtp.ErrAuthUnsupported
	}
	if s.authenticated {
		return nil, smtpAlreadyAuthenticated()
	}
	if s.receiver.authFailures.isLocked(s.remoteAddr) {
		return nil, &gosmtp.SMTPError{Code: 421, EnhancedCode: gosmtp.EnhancedCode{4, 7, 0}, Message: "Too many authentication failures, try later"}
	}
	return sasl.NewPlainServer(func(identity, username, password string) error {
		var authErr error
		defer func() {
			s.observe(s.ctx, MetricEvent{
				Stage:  StageAuthenticated,
				Result: metricResult(authErr),
				Error:  metricError(authErr),
			})
		}()
		if withRole, ok := s.receiver.authenticator.(AuthenticatorWithRole); ok {
			userRole, err := withRole.AuthenticatePlainWithRole(s.ctx, identity, username, password)
			if err != nil {
				authErr = gosmtp.ErrAuthFailed
				s.receiver.authFailures.record(s.remoteAddr)
				return gosmtp.ErrAuthFailed
			}
			s.authenticatedUserRole = userRole
		} else if err := s.receiver.authenticator.AuthenticatePlain(s.ctx, identity, username, password); err != nil {
			authErr = gosmtp.ErrAuthFailed
			s.receiver.authFailures.record(s.remoteAddr)
			return gosmtp.ErrAuthFailed
		}
		s.authenticated = true
		s.authenticatedUser = username
		if err := s.emit(s.ctx, Event{
			Stage:      StageAuthenticated,
			RemoteAddr: s.remoteAddr,
		}); err != nil {
			authErr = err
			return err
		}
		return nil
	}), nil
}

// authFailureTracker provides in-process brute-force protection for SMTP AUTH.
// It tracks per-IP failure timestamps and rejects attempts that exceed the
// threshold within the window. The map is cleaned up lazily on every check.
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

// record adds a failure for the given IP. Returns true if the IP is now locked out.
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
	// Release old backing array when all prior entries expired.
	if len(prev) > 0 && len(fresh) == 1 {
		t.failures[ip] = []time.Time{now}
	} else {
		t.failures[ip] = fresh
	}
	return len(fresh) > t.maxFails
}

// isLocked returns true when the IP has too many recent failures.
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
