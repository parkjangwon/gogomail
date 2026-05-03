package smtpd

import (
	"unicode/utf8"

	gosmtp "github.com/emersion/go-smtp"
)

type extensionSupport struct {
	SMTPUTF8   bool
	RequireTLS bool
	DSN        bool
	BinaryMIME bool
}

func validateMailOptions(opts *gosmtp.MailOptions, support extensionSupport) error {
	if opts == nil {
		return nil
	}
	if opts.UTF8 && !support.SMTPUTF8 {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "SMTPUTF8 is not supported")
	}
	if opts.RequireTLS && !support.RequireTLS {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "REQUIRETLS is not supported")
	}
	if opts.Return != "" && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN RET is not supported")
	}
	if opts.EnvelopeID != "" && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN ENVID is not supported")
	}
	if opts.Body == gosmtp.BodyBinaryMIME && !support.BinaryMIME {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "BINARYMIME is not supported")
	}
	return nil
}

func validateRcptOptions(opts *gosmtp.RcptOptions, support extensionSupport) error {
	if opts == nil {
		return nil
	}
	if len(opts.Notify) > 0 && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN NOTIFY is not supported")
	}
	if opts.OriginalRecipient != "" && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN ORCPT is not supported")
	}
	return nil
}

func mailOptionsUTF8(opts *gosmtp.MailOptions) bool {
	return opts != nil && opts.UTF8
}

func validateSMTPUTF8Address(raw string, normalized string, transactionUTF8 bool, supportSMTPUTF8 bool) error {
	if !containsNonASCII(raw) && !containsNonASCII(normalized) {
		return nil
	}
	if !supportSMTPUTF8 {
		return smtpPermanent(553, gosmtp.EnhancedCode{5, 6, 7}, "SMTPUTF8 is required for internationalized addresses")
	}
	if !transactionUTF8 {
		return smtpPermanent(553, gosmtp.EnhancedCode{5, 6, 7}, "MAIL FROM must declare SMTPUTF8 for internationalized addresses")
	}
	return nil
}

func containsNonASCII(value string) bool {
	for len(value) > 0 {
		r, size := utf8.DecodeRuneInString(value)
		if r == utf8.RuneError && size == 1 {
			return true
		}
		if r > 127 {
			return true
		}
		value = value[size:]
	}
	return false
}
