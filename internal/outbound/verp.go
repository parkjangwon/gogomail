package outbound

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/mail"
)

type VERPAddress struct {
	BaseLocal string
	Domain    string
	Recipient string
	Token     string
}

func BuildVERPReturnPath(base string, recipient string, token string) (string, error) {
	normalizedBase, err := mail.NormalizeAddress(base)
	if err != nil {
		return "", fmt.Errorf("invalid verp base address: %w", err)
	}
	normalizedRecipient, err := mail.NormalizeAddress(recipient)
	if err != nil {
		return "", fmt.Errorf("invalid verp recipient address: %w", err)
	}
	local, domain, ok := strings.Cut(normalizedBase, "@")
	if !ok || local == "" || domain == "" {
		return "", fmt.Errorf("invalid verp base address")
	}
	encodedRecipient := base64.RawURLEncoding.EncodeToString([]byte(normalizedRecipient))
	token = sanitizeVERPToken(token)
	if token != "" {
		return local + "+" + token + "--" + encodedRecipient + "@" + domain, nil
	}
	return local + "+" + encodedRecipient + "@" + domain, nil
}

func ParseVERPReturnPath(address string) (VERPAddress, bool) {
	address = strings.Trim(strings.TrimSpace(address), "<>")
	local, domain, ok := strings.Cut(address, "@")
	if !ok {
		return VERPAddress{}, false
	}
	local = strings.TrimSpace(local)
	domain = strings.ToLower(strings.TrimSpace(domain))
	baseLocal, encoded, ok := strings.Cut(local, "+")
	if !ok || baseLocal == "" || encoded == "" {
		return VERPAddress{}, false
	}
	baseLocal = strings.ToLower(baseLocal)
	token := ""
	if before, after, ok := strings.Cut(encoded, "--"); ok {
		token = before
		encoded = after
	}
	rawRecipient, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil {
		return VERPAddress{}, false
	}
	recipient, err := mail.NormalizeAddress(string(rawRecipient))
	if err != nil {
		return VERPAddress{}, false
	}
	return VERPAddress{
		BaseLocal: baseLocal,
		Domain:    domain,
		Recipient: recipient,
		Token:     token,
	}, true
}

func sanitizeVERPToken(token string) string {
	token = strings.ToLower(strings.TrimSpace(token))
	var builder strings.Builder
	for _, r := range token {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			builder.WriteRune(r)
		}
	}
	return builder.String()
}
