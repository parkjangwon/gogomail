package smtpd

import gosmtp "github.com/emersion/go-smtp"

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
