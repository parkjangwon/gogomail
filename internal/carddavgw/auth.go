package carddavgw

import (
	"fmt"
	"net/http"
	"strings"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

const (
	maxCardDAVBasicUsernameBytes = 1024
	maxCardDAVBasicPasswordBytes = 4096
)

type BasicAuthResolver struct {
	Authenticator smtpd.SubmissionAuthenticator
	AllowInsecure bool
}

func NewBasicAuthResolver(authenticator smtpd.SubmissionAuthenticator, allowInsecure bool) BasicAuthResolver {
	return BasicAuthResolver{Authenticator: authenticator, AllowInsecure: allowInsecure}
}

func (r BasicAuthResolver) Resolve(req *http.Request) (string, error) {
	if r.Authenticator == nil {
		return "", fmt.Errorf("carddav authenticator is required")
	}
	if req == nil {
		return "", fmt.Errorf("request is required")
	}
	if !r.AllowInsecure && !requestUsesTLS(req) {
		return "", fmt.Errorf("carddav basic authentication requires TLS")
	}
	username, password, ok := req.BasicAuth()
	if !ok {
		return "", fmt.Errorf("basic authentication is required")
	}
	username, err := validateBasicCredential("username", username, maxCardDAVBasicUsernameBytes)
	if err != nil {
		return "", err
	}
	if _, err := validateBasicCredential("password", password, maxCardDAVBasicPasswordBytes); err != nil {
		return "", err
	}
	user, err := r.Authenticator.AuthenticatePlain(req.Context(), "", username, password)
	if err != nil {
		return "", fmt.Errorf("invalid carddav credentials")
	}
	if strings.TrimSpace(user.UserID) == "" || strings.ContainsAny(user.UserID, "\r\n") {
		return "", fmt.Errorf("authenticated carddav user id is invalid")
	}
	return strings.TrimSpace(user.UserID), nil
}

func requestUsesTLS(req *http.Request) bool {
	if req.TLS != nil {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")), "https")
}

func validateBasicCredential(field string, value string, maxBytes int) (string, error) {
	if value == "" {
		return "", fmt.Errorf("basic auth %s is required", field)
	}
	if len(value) > maxBytes {
		return "", fmt.Errorf("basic auth %s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("basic auth %s must not contain line breaks", field)
	}
	return value, nil
}
