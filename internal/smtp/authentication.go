package smtpd

import (
	"context"
	"io"

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
	SPF   AuthCheckResult
	DKIM  AuthCheckResult
	DMARC AuthCheckResult
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
