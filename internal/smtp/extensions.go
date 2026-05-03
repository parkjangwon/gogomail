package smtpd

import (
	"strings"
	"unicode"
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
	if opts.Return != "" && !validDSNReturnOption(opts.Return) {
		return smtpPermanent(501, gosmtp.EnhancedCode{5, 5, 4}, "invalid DSN RET option")
	}
	if opts.EnvelopeID != "" && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN ENVID is not supported")
	}
	if opts.EnvelopeID != "" && !validDSNEnvelopeID(opts.EnvelopeID) {
		return smtpPermanent(501, gosmtp.EnhancedCode{5, 5, 4}, "invalid DSN ENVID option")
	}
	if opts.Body == gosmtp.BodyBinaryMIME && !support.BinaryMIME {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "BINARYMIME is not supported")
	}
	return nil
}

func validateAnnouncedMessageSize(opts *gosmtp.MailOptions, maxBytes int64) error {
	if opts == nil || opts.Size <= 0 || maxBytes <= 0 {
		return nil
	}
	if opts.Size > maxBytes {
		return gosmtp.ErrDataTooLarge
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
	for _, notify := range opts.Notify {
		if !validDSNNotifyOption(notify) {
			return smtpPermanent(501, gosmtp.EnhancedCode{5, 5, 4}, "invalid DSN NOTIFY option")
		}
	}
	if dsnNotifyNeverCombined(opts.Notify) {
		return smtpPermanent(501, gosmtp.EnhancedCode{5, 5, 4}, "DSN NOTIFY=NEVER cannot be combined")
	}
	if opts.OriginalRecipient != "" && !support.DSN {
		return smtpPermanent(555, gosmtp.EnhancedCode{5, 5, 4}, "DSN ORCPT is not supported")
	}
	if opts.OriginalRecipient != "" && !validRCPTOriginalRecipient(opts) {
		return smtpPermanent(501, gosmtp.EnhancedCode{5, 5, 4}, "invalid DSN ORCPT option")
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

func validDSNReturnOption(value gosmtp.DSNReturn) bool {
	switch gosmtp.DSNReturn(strings.ToUpper(strings.TrimSpace(string(value)))) {
	case gosmtp.DSNReturnFull, gosmtp.DSNReturnHeaders:
		return true
	default:
		return false
	}
}

func validDSNNotifyOption(value gosmtp.DSNNotify) bool {
	switch gosmtp.DSNNotify(strings.ToUpper(strings.TrimSpace(string(value)))) {
	case gosmtp.DSNNotifyNever, gosmtp.DSNNotifySuccess, gosmtp.DSNNotifyFailure, gosmtp.DSNNotifyDelayed:
		return true
	default:
		return false
	}
}

func dsnNotifyNeverCombined(values []gosmtp.DSNNotify) bool {
	if len(values) <= 1 {
		return false
	}
	for _, value := range values {
		if gosmtp.DSNNotify(strings.ToUpper(strings.TrimSpace(string(value)))) == gosmtp.DSNNotifyNever {
			return true
		}
	}
	return false
}

func validDSNEnvelopeID(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > 100 {
		return false
	}
	return validDSNXText(value)
}

func validDSNOriginalRecipient(value string) bool {
	value = strings.TrimSpace(value)
	addrType, encodedAddress, ok := strings.Cut(value, ";")
	if !ok || addrType == "" || encodedAddress == "" {
		return false
	}
	if !validDSNAddressType(addrType) {
		return false
	}
	return validDSNXText(encodedAddress)
}

func validRCPTOriginalRecipient(opts *gosmtp.RcptOptions) bool {
	if opts == nil || strings.TrimSpace(opts.OriginalRecipient) == "" {
		return true
	}
	if strings.TrimSpace(string(opts.OriginalRecipientType)) == "" {
		return validDSNOriginalRecipient(opts.OriginalRecipient)
	}
	if !validDSNAddressType(string(opts.OriginalRecipientType)) {
		return false
	}
	return validDSNXText(encodeDSNXText(strings.TrimSpace(opts.OriginalRecipient)))
}

func validDSNAddressType(value string) bool {
	for _, r := range value {
		if r > 127 || !(unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-') {
			return false
		}
	}
	return true
}

func validDSNXText(value string) bool {
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c < 33 || c > 126 || c == '=' {
			return false
		}
		if c != '+' {
			continue
		}
		if i+2 >= len(value) || !isHexDigit(value[i+1]) || !isHexDigit(value[i+2]) {
			return false
		}
		i += 2
	}
	return true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'A' && c <= 'F') || (c >= 'a' && c <= 'f')
}
