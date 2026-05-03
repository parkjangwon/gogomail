package mail

import (
	"fmt"
	"net/mail"
	"strings"
)

func NormalizeAddress(raw string) (string, error) {
	address := strings.TrimSpace(raw)
	parsed, err := mail.ParseAddress(address)
	if err != nil {
		return "", fmt.Errorf("invalid email address %q: %w", raw, err)
	}

	addr := strings.TrimSpace(parsed.Address)
	local, domain, ok := strings.Cut(addr, "@")
	if !ok || local == "" || domain == "" {
		return "", fmt.Errorf("invalid email address %q", raw)
	}

	return strings.ToLower(local) + "@" + strings.ToLower(domain), nil
}
