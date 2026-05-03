package smtpd

import (
	"fmt"
	"net"
	"strings"
	"time"
)

func BuildReceivedHeader(remoteAddr string, byDomain string, id string, at time.Time) string {
	from := sanitizeReceivedToken(remoteHost(remoteAddr))
	if from == "" {
		from = "unknown"
	}
	by := sanitizeReceivedToken(byDomain)
	if by == "" {
		by = "localhost"
	}
	id = sanitizeReceivedToken(id)
	if id == "" {
		id = fmt.Sprintf("%d", at.UnixNano())
	}
	if at.IsZero() {
		at = time.Now()
	}
	return fmt.Sprintf(
		"Received: from %s by %s with ESMTP id %s; %s\r\n",
		from,
		by,
		id,
		at.UTC().Format(time.RFC1123Z),
	)
}

func remoteHost(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return host
	}
	return remoteAddr
}

func sanitizeReceivedToken(value string) string {
	value = strings.TrimSpace(value)
	value = strings.ReplaceAll(value, "\r", "")
	value = strings.ReplaceAll(value, "\n", "")
	return value
}
