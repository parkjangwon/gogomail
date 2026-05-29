package smtpd

import (
	"context"
	"io"
	"strings"

	gosmtp "github.com/emersion/go-smtp"

	"github.com/gogomail/gogomail/internal/message"
)

type AuthResult string

const (
	AuthResultNone      AuthResult = "none"
	AuthResultPass      AuthResult = "pass"
	AuthResultFail      AuthResult = "fail"
	AuthResultNeutral   AuthResult = "neutral"
	AuthResultTemporary AuthResult = "temperror"
	AuthResultPermanent AuthResult = "permerror"
)

type AuthCheckResult struct {
	Result     AuthResult
	Reason     string
	Domain     string
	Identifier string
	Policy     string
}

type AuthenticationResults struct {
	AuthservID string
	SPF        AuthCheckResult
	DKIM       AuthCheckResult
	DMARC      AuthCheckResult
}

type AuthenticationRequest struct {
	RemoteAddr   string
	EnvelopeFrom string
	Recipients   []string
	Parsed       message.ParsedMessage
	RawMessage   io.Reader
	Size         int64
}

type AuthenticationVerifier interface {
	VerifyAuthentication(ctx context.Context, req AuthenticationRequest) (AuthenticationResults, error)
}

// enforceDMARCPolicy rejects messages when the DMARC check failed and the
// domain policy calls for rejection.  Returns (quarantine=true, nil) when the
// policy is "quarantine" — callers should route the message to the Spam folder.
// It is a no-op when enforce is false or when no DMARC result is available.
func enforceDMARCPolicy(enforce bool, results AuthenticationResults) (quarantine bool, err error) {
	if !enforce || results.DMARC.Result != AuthResultFail {
		return false, nil
	}
	policy := strings.ToLower(strings.TrimSpace(results.DMARC.Policy))
	switch policy {
	case "reject":
		domain := results.DMARC.Domain
		if domain == "" {
			domain = "sender domain"
		}
		return false, smtpPermanent(550, gosmtp.EnhancedCode{5, 7, 1},
			"message rejected due to DMARC policy for %s", domain)
	case "quarantine":
		return true, nil
	}
	return false, nil
}
