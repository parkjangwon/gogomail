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

func TestValidateOptionsRejectsInvalidDSNValues(t *testing.T) {
	t.Parallel()

	support := extensionSupport{DSN: true}
	if err := validateMailOptions(&gosmtp.MailOptions{Return: "BODY"}, support); err == nil {
		t.Fatal("validateMailOptions accepted invalid DSN RET")
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{Notify: []gosmtp.DSNNotify{"MAYBE"}}, support); err == nil {
		t.Fatal("validateRcptOptions accepted invalid DSN NOTIFY")
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{Notify: []gosmtp.DSNNotify{gosmtp.DSNNotifyNever, gosmtp.DSNNotifyFailure}}, support); err == nil {
		t.Fatal("validateRcptOptions accepted NOTIFY=NEVER combined with another value")
	}
	if err := validateMailOptions(&gosmtp.MailOptions{EnvelopeID: "bad id"}, support); err == nil {
		t.Fatal("validateMailOptions accepted ENVID with whitespace")
	}
	if err := validateMailOptions(&gosmtp.MailOptions{EnvelopeID: "bad\r\nid"}, support); err == nil {
		t.Fatal("validateMailOptions accepted ENVID with control characters")
	}
	if err := validateMailOptions(&gosmtp.MailOptions{EnvelopeID: "bad+escape"}, support); err == nil {
		t.Fatal("validateMailOptions accepted ENVID with invalid xtext escape")
	}
	if err := validateMailOptions(&gosmtp.MailOptions{EnvelopeID: "bad=value"}, support); err == nil {
		t.Fatal("validateMailOptions accepted ENVID with raw equals")
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{OriginalRecipient: "user@example.com"}, support); err == nil {
		t.Fatal("validateRcptOptions accepted ORCPT without address type")
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{OriginalRecipient: "rfc822;bad address@example.com"}, support); err == nil {
		t.Fatal("validateRcptOptions accepted ORCPT with whitespace")
	}
}

func TestValidateOptionsAcceptsRFCShapedDSNIdentityValues(t *testing.T) {
	t.Parallel()

	support := extensionSupport{DSN: true}
	if err := validateMailOptions(&gosmtp.MailOptions{EnvelopeID: "queue+2Btoken"}, support); err != nil {
		t.Fatalf("validateMailOptions rejected xtext ENVID: %v", err)
	}
	if err := validateRcptOptions(&gosmtp.RcptOptions{OriginalRecipient: "utf-8;user+40example.com"}, support); err != nil {
		t.Fatalf("validateRcptOptions rejected typed ORCPT: %v", err)
	}
}
