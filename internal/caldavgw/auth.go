package caldavgw

import (
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"

	smtpd "github.com/gogomail/gogomail/internal/smtp"
)

const (
	maxCalDAVBasicUsernameBytes = 1024
	maxCalDAVBasicPasswordBytes = 4096
	calDAVWWWAuthenticate       = `Basic realm="CalDAV"`
)

type unauthorizedChallenge struct {
	err       error
	challenge string
}

func (e unauthorizedChallenge) Error() string {
	return e.err.Error()
}

func (e unauthorizedChallenge) Unwrap() error {
	return e.err
}

func (e unauthorizedChallenge) WWWAuthenticate() string {
	return e.challenge
}

func newUnauthorizedChallengeError(err error) error {
	if err == nil {
		return nil
	}
	return unauthorizedChallenge{err: err, challenge: calDAVWWWAuthenticate}
}

type BasicAuthResolver struct {
	Authenticator       smtpd.SubmissionAuthenticator
	AllowInsecure       bool
	TrustForwardedProto bool
	TrustedProxies      []netip.Prefix
}

func NewBasicAuthResolver(authenticator smtpd.SubmissionAuthenticator, allowInsecure bool) BasicAuthResolver {
	return BasicAuthResolver{
		Authenticator:       authenticator,
		AllowInsecure:       allowInsecure,
		TrustForwardedProto: true,
		TrustedProxies:      nil,
	}
}

func (r BasicAuthResolver) WithTrustedProxies(proxies []string) (BasicAuthResolver, error) {
	trustedProxies, err := parseTrustedProxies(proxies)
	if err != nil {
		return BasicAuthResolver{}, err
	}
	r.TrustedProxies = trustedProxies
	return r, nil
}

func (r BasicAuthResolver) Resolve(req *http.Request) (string, error) {
	if r.Authenticator == nil {
		return "", newUnauthorizedChallengeError(fmt.Errorf("caldav authenticator is required"))
	}
	if req == nil {
		return "", newUnauthorizedChallengeError(fmt.Errorf("request is required"))
	}
	if !r.AllowInsecure && !r.requestUsesTLS(req) {
		return "", newUnauthorizedChallengeError(fmt.Errorf("caldav basic authentication requires TLS"))
	}
	username, password, ok := req.BasicAuth()
	if !ok {
		return "", newUnauthorizedChallengeError(fmt.Errorf("basic authentication is required"))
	}
	username, err := validateBasicCredential("username", username, maxCalDAVBasicUsernameBytes)
	if err != nil {
		return "", newUnauthorizedChallengeError(err)
	}
	if _, err := validateBasicCredential("password", password, maxCalDAVBasicPasswordBytes); err != nil {
		return "", newUnauthorizedChallengeError(err)
	}
	user, err := r.Authenticator.AuthenticatePlain(req.Context(), "", username, password)
	if err != nil {
		return "", newUnauthorizedChallengeError(fmt.Errorf("invalid caldav credentials"))
	}
	if strings.TrimSpace(user.UserID) == "" || strings.ContainsAny(user.UserID, "\r\n") {
		return "", newUnauthorizedChallengeError(fmt.Errorf("authenticated caldav user id is invalid"))
	}
	return strings.TrimSpace(user.UserID), nil
}

func (r BasicAuthResolver) requestUsesTLS(req *http.Request) bool {
	if req.TLS != nil {
		return true
	}
	if !r.TrustForwardedProto {
		return false
	}
	if len(r.TrustedProxies) > 0 {
		remoteIP, ok := parseRemoteIP(req.RemoteAddr)
		if !ok {
			return false
		}
		if !trustedProxyMatch(remoteIP, r.TrustedProxies) {
			return false
		}
	}
	forwardedProtoHeader := req.Header.Values("X-Forwarded-Proto")
	if len(forwardedProtoHeader) == 0 {
		return false
	}
	for _, forwardedProto := range forwardedProtoHeader {
		for _, token := range strings.Split(forwardedProto, ",") {
			value := strings.TrimSpace(token)
			if value == "" {
				continue
			}
			if !strings.EqualFold(value, "https") {
				return false
			}
		}
	}
	return true
}

func parseTrustedProxies(values []string) ([]netip.Prefix, error) {
	trusted := make([]netip.Prefix, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := parseTrustedProxyPrefix(value)
		if err != nil {
			return nil, fmt.Errorf("invalid trusted proxy %q: %w", value, err)
		}
		trusted = append(trusted, prefix)
	}
	return trusted, nil
}

func parseTrustedProxyPrefix(value string) (netip.Prefix, error) {
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return netip.Prefix{}, err
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr.Unmap(), 32), nil
	}
	return netip.PrefixFrom(addr.Unmap(), 128), nil
}

func parseRemoteIP(remoteAddr string) (netip.Addr, bool) {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return netip.Addr{}, false
	}
	host := remoteAddr
	if parsed, _, err := net.SplitHostPort(remoteAddr); err == nil {
		host = parsed
	}
	addr, err := netip.ParseAddr(strings.Trim(host, "[]"))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.Unmap(), true
}

func trustedProxyMatch(remote netip.Addr, proxies []netip.Prefix) bool {
	for _, proxy := range proxies {
		if proxy.Contains(remote) {
			return true
		}
	}
	return false
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
