package outbound

import (
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/mail"
)

const (
	maxVERPAddressBytes          = 512
	maxVERPLocalPartBytes        = 320
	maxVERPEncodedRecipientBytes = 512
	maxVERPTokenBytes            = 128
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
	if address == "" || len(address) > maxVERPAddressBytes {
		return VERPAddress{}, false
	}
	local, domain, ok := strings.Cut(address, "@")
	if !ok {
		return VERPAddress{}, false
	}
	local = strings.TrimSpace(local)
	domain = strings.ToLower(strings.TrimSpace(domain))
	if local == "" || domain == "" || len(local) > maxVERPLocalPartBytes {
		return VERPAddress{}, false
	}
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
	if len(token) > maxVERPTokenBytes || len(encoded) > maxVERPEncodedRecipientBytes {
		return VERPAddress{}, false
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
			if builder.Len() >= maxVERPTokenBytes {
				break
			}
		}
	}
	return builder.String()
}
