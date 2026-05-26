package config

import (
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
)

func validatePublicBaseURL(value string, production bool) error {
	value = strings.TrimRight(strings.TrimSpace(value), "/")
	if value == "" {
		if production {
			return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must not be empty in production")
		}
		return nil
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme == "" || parsed.Host == "" || parsed.RawQuery != "" || parsed.Fragment != "" {
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must be an absolute URL without query or fragment")
	}
	switch parsed.Scheme {
	case "http":
		if production {
			return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must use https in production")
		}
	case "https":
	default:
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must use http or https")
	}
	if production && isLocalPublicBaseURLHost(parsed.Hostname()) {
		return fmt.Errorf("GOGOMAIL_PUBLIC_BASE_URL must not point to localhost in production")
	}
	return nil
}

func isLocalPublicBaseURLHost(host string) bool {
	return isLocalProductionHostname(host)
}

func isLocalProductionHostname(host string) bool {
	host = strings.Trim(strings.ToLower(strings.TrimSpace(host)), "[]")
	if host == "localhost" || host == "localhost.localdomain" {
		return true
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	return ip.IsLoopback() || ip.IsUnspecified()
}

func validateEnum(name string, value string, allowed ...string) error {
	normalized := strings.ToLower(strings.TrimSpace(value))
	for _, candidate := range allowed {
		if normalized == candidate {
			return nil
		}
	}
	return fmt.Errorf("%s has unsupported value %q", name, value)
}

func validateStorageBackendCompatLabel(value string) error {
	const name = "GOGOMAIL_STORAGE_BACKEND_COMPAT_LABELS"
	label := strings.ToLower(strings.TrimSpace(value))
	if label == "" {
		return fmt.Errorf("%s label is required", name)
	}
	if len(label) > 64 {
		return fmt.Errorf("%s label is too long", name)
	}
	for _, r := range label {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			continue
		}
		return fmt.Errorf("%s label %q must contain only lowercase letters, digits, dot, underscore, or hyphen", name, value)
	}
	return nil
}

func validateTCPAddr(name string, value string, required bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return fmt.Errorf("%s is required", name)
		}
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	_, port, err := net.SplitHostPort(value)
	if err != nil {
		return fmt.Errorf("%s must be a TCP host:port address: %w", name, err)
	}
	parsedPort, err := strconv.Atoi(port)
	if err != nil || parsedPort < 1 || parsedPort > 65535 {
		return fmt.Errorf("%s must include a TCP port between 1 and 65535", name)
	}
	return nil
}

func validateHTTPSURL(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if parsed.Scheme != "https" || parsed.Host == "" {
		return fmt.Errorf("%s must be an https URL", name)
	}
	return nil
}

func validateHTTPURL(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be an http or https URL", name)
	}
	return nil
}

func validateOpenSearchIndexName(name string, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, `/\?#*:,"<>| `) || strings.HasPrefix(value, ".") || strings.HasPrefix(value, "_") {
		return fmt.Errorf("%s is invalid", name)
	}
	return nil
}

func validateWebhookURL(name string, value string, requireHTTPS bool) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("%s must be a valid URL: %w", name, err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return fmt.Errorf("%s must be an http or https URL", name)
	}
	if requireHTTPS && parsed.Scheme != "https" {
		return fmt.Errorf("%s must be an https URL in production", name)
	}
	return nil
}

func validateOptionalSecret(name string, value string) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	if len(value) > maxWebhookTokenBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateBoundedNoCRLF(name string, value string, maxBytes int) error {
	value = strings.TrimSpace(value)
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("%s cannot contain line breaks", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateRequiredBoundedNoCRLF(name string, value string, maxBytes int) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("%s is required", name)
	}
	return validateBoundedNoCRLF(name, value, maxBytes)
}

func validateTrustedProxies(name string, values []string) error {
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, err := netip.ParsePrefix(value); err == nil {
			continue
		}
		if _, err := netip.ParseAddr(value); err == nil {
			continue
		}
		return fmt.Errorf("%s contains invalid trusted proxy %q", name, value)
	}
	return nil
}

func validateS3CredentialNoWhitespace(name string, value string, maxBytes int, required bool) error {
	if required && value == "" {
		return fmt.Errorf("%s is required", name)
	}
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return fmt.Errorf("%s cannot contain whitespace", name)
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s is too long", name)
	}
	return nil
}

func validateLDAPReferralURL(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS cannot contain line breaks")
	}
	if len(value) > 4096 {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entry is too long")
	}
	u, err := url.Parse(value)
	if err != nil {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entry is invalid: %w", err)
	}
	if u.Scheme != "ldap" && u.Scheme != "ldaps" {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entries must use ldap or ldaps")
	}
	if strings.TrimSpace(u.Host) == "" {
		return fmt.Errorf("GOGOMAIL_LDAP_REFERRAL_URLS entries must include a host")
	}
	return nil
}

func validateCACertFile(name string, path string) error {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", name, err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(data) {
		return fmt.Errorf("%s must contain at least one PEM-encoded certificate", name)
	}
	return nil
}

func validateExportManifestSignerKeyID(value string, backend string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID is required for %s signer", backend)
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID cannot contain line breaks")
	}
	if len(value) > maxExportManifestSignerKeyIDBytes {
		return fmt.Errorf("GOGOMAIL_API_USAGE_EXPORT_MANIFEST_SIGNER_KEY_ID is too long")
	}
	return nil
}

func decodeBase64Key(name string, value string, size int) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, fmt.Errorf("%s is required", name)
	}
	if len(value) > base64.StdEncoding.EncodedLen(size) {
		return nil, fmt.Errorf("%s is too long", name)
	}
	decoded, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("%s must be base64: %w", name, err)
	}
	if len(decoded) != size {
		return nil, fmt.Errorf("%s must decode to %d bytes", name, size)
	}
	return decoded, nil
}

func stringBytesEqual(a []byte, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var diff byte
	for i := range a {
		diff |= a[i] ^ b[i]
	}
	return diff == 0
}
