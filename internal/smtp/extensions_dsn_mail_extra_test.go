package smtpd

import (
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestValidateMailOptionsAllowsDSNWhenSupported(t *testing.T) {
	opts := &gosmtp.MailOptions{Return: "FULL", EnvelopeID: "abc"}
	if err := validateMailOptions(opts, extensionSupport{DSN: true}); err != nil {
		t.Fatalf("validateMailOptions returned error: %v", err)
	}
}
