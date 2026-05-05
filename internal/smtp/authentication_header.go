package smtpd

import (
	"strings"
	"unicode/utf8"

	"github.com/emersion/go-msgauth/authres"
)

const (
	maxAuthservIDBytes          = 255
	maxAuthenticationValueBytes = 512
)

func FormatAuthenticationResults(results AuthenticationResults) string {
	authservID := sanitizeAuthenticationResultText(results.AuthservID, maxAuthservIDBytes)
	if authservID == "" {
		authservID = "localhost"
	}
	formatted := authres.Format(authservID, []authres.Result{
		&authres.SPFResult{
			Value:  authResultValue(results.SPF.Result),
			Reason: sanitizeAuthenticationResultText(results.SPF.Reason, maxAuthenticationValueBytes),
			From:   sanitizeAuthenticationResultText(results.SPF.Identifier, maxAuthenticationValueBytes),
		},
		&authres.DKIMResult{
			Value:      authResultValue(results.DKIM.Result),
			Reason:     sanitizeAuthenticationResultText(results.DKIM.Reason, maxAuthenticationValueBytes),
			Domain:     sanitizeAuthenticationResultText(results.DKIM.Domain, maxAuthenticationValueBytes),
			Identifier: sanitizeAuthenticationResultText(results.DKIM.Identifier, maxAuthenticationValueBytes),
		},
		&authres.DMARCResult{
			Value:  authResultValue(results.DMARC.Result),
			Reason: sanitizeAuthenticationResultText(results.DMARC.Reason, maxAuthenticationValueBytes),
			From:   sanitizeAuthenticationResultText(results.DMARC.Domain, maxAuthenticationValueBytes),
		},
	})
	return foldHeaderLine("Authentication-Results: " + formatted)
}

func authResultValue(result AuthResult) authres.ResultValue {
	switch result {
	case AuthResultPass:
		return authres.ResultPass
	case AuthResultFail:
		return authres.ResultFail
	case AuthResultNeutral:
		return authres.ResultNeutral
	case AuthResultTemporary:
		return authres.ResultTempError
	case AuthResultPermanent:
		return authres.ResultPermError
	default:
		return authres.ResultNone
	}
}

func sanitizeAuthenticationResultText(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	value = strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == '\t' || r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	cut := maxBytes
	for cut > 0 && !utf8.ValidString(value[:cut]) {
		cut--
	}
	return strings.TrimSpace(value[:cut])
}

func foldHeaderLine(line string) string {
	const softLimit = 78
	if len(line) <= softLimit {
		return line + "\r\n"
	}
	var builder strings.Builder
	for len(line) > softLimit {
		cut := strings.LastIndexByte(line[:softLimit], ' ')
		if cut <= 0 {
			cut = softLimit
		}
		builder.WriteString(strings.TrimRight(line[:cut], " "))
		builder.WriteString("\r\n ")
		line = strings.TrimLeft(line[cut:], " ")
	}
	builder.WriteString(line)
	builder.WriteString("\r\n")
	return builder.String()
}
