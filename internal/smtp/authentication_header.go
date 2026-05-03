package smtpd

import (
	"strings"

	"github.com/emersion/go-msgauth/authres"
)

func FormatAuthenticationResults(results AuthenticationResults) string {
	authservID := strings.TrimSpace(results.AuthservID)
	if authservID == "" {
		authservID = "localhost"
	}
	formatted := authres.Format(authservID, []authres.Result{
		&authres.SPFResult{
			Value:  authResultValue(results.SPF.Result),
			Reason: results.SPF.Reason,
			From:   results.SPF.Identifier,
		},
		&authres.DKIMResult{
			Value:      authResultValue(results.DKIM.Result),
			Reason:     results.DKIM.Reason,
			Domain:     results.DKIM.Domain,
			Identifier: results.DKIM.Identifier,
		},
		&authres.DMARCResult{
			Value:  authResultValue(results.DMARC.Result),
			Reason: results.DMARC.Reason,
			From:   results.DMARC.Domain,
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
