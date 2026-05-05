package webhook

import (
	"fmt"
	"io"
	"strings"
)

const maxWebhookErrorBodyPreviewBytes = 512

func NormalizeWebhookToken(value string, maxBytes int) (string, error) {
	token := strings.ToValidUTF8(strings.TrimSpace(value), "")
	if token == "" {
		return "", nil
	}
	if len(token) > maxBytes {
		return "", fmt.Errorf("token exceeds %d bytes", maxBytes)
	}
	if strings.ContainsAny(token, "\r\n") {
		return "", fmt.Errorf("token must not contain CR or LF")
	}
	return token, nil
}

func ErrorBodyPreview(body io.Reader, maxBytes int64) string {
	raw, _ := io.ReadAll(io.LimitReader(body, maxBytes))
	preview := strings.ToValidUTF8(strings.TrimSpace(string(raw)), "")
	preview = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, preview)
	return strings.Join(strings.Fields(preview), " ")
}
