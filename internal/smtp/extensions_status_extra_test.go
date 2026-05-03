package smtpd

import (
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestValidateMailOptionsUnsupportedExtensionsReturn555(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts *gosmtp.MailOptions
	}{
		{name: "SMTPUTF8", opts: &gosmtp.MailOptions{UTF8: true}},
		{name: "REQUIRETLS", opts: &gosmtp.MailOptions{RequireTLS: true}},
		{name: "RET", opts: &gosmtp.MailOptions{Return: gosmtp.DSNReturnHeaders}},
		{name: "ENVID", opts: &gosmtp.MailOptions{EnvelopeID: "env-1"}},
		{name: "BINARYMIME", opts: &gosmtp.MailOptions{Body: gosmtp.BodyBinaryMIME}},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			requireSMTPStatus(t, validateMailOptions(tt.opts, extensionSupport{}), 555, gosmtp.EnhancedCode{5, 5, 4})
		})
	}
}

func TestValidateRcptOptionsUnsupportedDSNReturns555(t *testing.T) {
	t.Parallel()

	requireSMTPStatus(t, validateRcptOptions(&gosmtp.RcptOptions{
		Notify: []gosmtp.DSNNotify{gosmtp.DSNNotifyFailure},
	}, extensionSupport{}), 555, gosmtp.EnhancedCode{5, 5, 4})
	requireSMTPStatus(t, validateRcptOptions(&gosmtp.RcptOptions{
		OriginalRecipient: "rfc822;user@example.com",
	}, extensionSupport{}), 555, gosmtp.EnhancedCode{5, 5, 4})
}
