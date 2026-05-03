package smtpd

import (
	"fmt"

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
		return fmt.Errorf("SMTPUTF8 is not supported")
	}
	if opts.RequireTLS && !support.RequireTLS {
		return fmt.Errorf("REQUIRETLS is not supported")
	}
	if opts.Return != "" && !support.DSN {
		return fmt.Errorf("DSN RET is not supported")
	}
	if opts.EnvelopeID != "" && !support.DSN {
		return fmt.Errorf("DSN ENVID is not supported")
	}
	if opts.Body == gosmtp.BodyBinaryMIME && !support.BinaryMIME {
		return fmt.Errorf("BINARYMIME is not supported")
	}
	return nil
}

func validateRcptOptions(opts *gosmtp.RcptOptions, support extensionSupport) error {
	if opts == nil {
		return nil
	}
	if len(opts.Notify) > 0 && !support.DSN {
		return fmt.Errorf("DSN NOTIFY is not supported")
	}
	if opts.OriginalRecipient != "" && !support.DSN {
		return fmt.Errorf("DSN ORCPT is not supported")
	}
	return nil
}
