package smtpd

import (
	"fmt"

	gosmtp "github.com/emersion/go-smtp"
)

func smtpPermanent(code int, enhanced gosmtp.EnhancedCode, format string, args ...any) *gosmtp.SMTPError {
	return &gosmtp.SMTPError{Code: code, EnhancedCode: enhanced, Message: fmt.Sprintf(format, args...)}
}

func smtpTemporary(code int, enhanced gosmtp.EnhancedCode, format string, args ...any) *gosmtp.SMTPError {
	return &gosmtp.SMTPError{Code: code, EnhancedCode: enhanced, Message: fmt.Sprintf(format, args...)}
}

func smtpBadSequence(command string) *gosmtp.SMTPError {
	return smtpPermanent(503, gosmtp.EnhancedCode{5, 5, 1}, "%s command is out of sequence", command)
}

func smtpAlreadyAuthenticated() *gosmtp.SMTPError {
	return smtpPermanent(503, gosmtp.EnhancedCode{5, 7, 0}, "session is already authenticated")
}

func smtpMailboxUnavailable(format string, args ...any) *gosmtp.SMTPError {
	return smtpPermanent(550, gosmtp.EnhancedCode{5, 1, 1}, format, args...)
}

func smtpPolicyReject(format string, args ...any) *gosmtp.SMTPError {
	return smtpPermanent(550, gosmtp.EnhancedCode{5, 7, 1}, format, args...)
}

func smtpTooManyRecipients(max int) *gosmtp.SMTPError {
	return smtpTemporary(452, gosmtp.EnhancedCode{4, 5, 3}, "too many recipients; max %d", max)
}

func smtpRateLimited(recipient string) *gosmtp.SMTPError {
	return smtpTemporary(451, gosmtp.EnhancedCode{4, 7, 1}, "rate limit exceeded for recipient %q", recipient)
}

func smtpBackpressure() *gosmtp.SMTPError {
	return smtpTemporary(421, gosmtp.EnhancedCode{4, 3, 2}, "service temporarily unavailable")
}
