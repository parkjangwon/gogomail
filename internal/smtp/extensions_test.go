package smtpd

import (
	"testing"

	gosmtp "github.com/emersion/go-smtp"
)

func TestValidateMailOptionsRejectsUnsupportedExtensions(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		opts *gosmtp.MailOptions
	}{
		{name: "smtputf8", opts: &gosmtp.MailOptions{UTF8: true}},
		{name: "requiretls", opts: &gosmtp.MailOptions{RequireTLS: true}},
		{name: "dsn ret", opts: &gosmtp.MailOptions{Return: gosmtp.DSNReturnHeaders}},
		{name: "dsn envid", opts: &gosmtp.MailOptions{EnvelopeID: "env-1"}},
		{name: "binarymime", opts: &gosmtp.MailOptions{Body: gosmtp.BodyBinaryMIME}},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if err := validateMailOptions(tt.opts, extensionSupport{}); err == nil {
				t.Fatal("validateMailOptions accepted unsupported extension")
			}
		})
	}
}

func TestValidateRcptOptionsRejectsUnsupportedDSN(t *testing.T) {
	t.Parallel()

	if err := validateRcptOptions(&gosmtp.RcptOptions{Notify: []gosmtp.DSNNotify{gosmtp.DSNNotifyFailure}}, extensionSupport{}); err == nil {
		t.Fatal("validateRcptOptions accepted NOTIFY without DSN support")
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{OriginalRecipient: "rfc822;user@example.com"}, extensionSupport{}); err == nil {
		t.Fatal("validateRcptOptions accepted ORCPT without DSN support")
	}
}

func TestValidateOptionsAcceptsSupportedExtensions(t *testing.T) {
	t.Parallel()

	support := extensionSupport{SMTPUTF8: true, RequireTLS: true, DSN: true, BinaryMIME: true}
	if err := validateMailOptions(&gosmtp.MailOptions{
		UTF8:       true,
		RequireTLS: true,
		Return:     gosmtp.DSNReturnFull,
		EnvelopeID: "env-1",
		Body:       gosmtp.BodyBinaryMIME,
	}, support); err != nil {
		t.Fatalf("validateMailOptions returned error: %v", err)
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{
		Notify:            []gosmtp.DSNNotify{gosmtp.DSNNotifyFailure},
		OriginalRecipient: "rfc822;user@example.com",
	}, support); err != nil {
		t.Fatalf("validateRcptOptions returned error: %v", err)
	}
}
