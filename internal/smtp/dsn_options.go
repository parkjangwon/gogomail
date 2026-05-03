package smtpd

import (
	"strings"

	gosmtp "github.com/emersion/go-smtp"
)

func normalizeDSNReturn(opts *gosmtp.MailOptions) string {
	if opts == nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(opts.Return)))
}

func normalizeDSNEnvelopeID(opts *gosmtp.MailOptions) string {
	if opts == nil {
		return ""
	}
	return strings.TrimSpace(opts.EnvelopeID)
}

func normalizeDSNRecipientOptions(address string, opts *gosmtp.RcptOptions) DSNRecipientOptions {
	recipient := DSNRecipientOptions{Address: strings.ToLower(strings.TrimSpace(address))}
	if opts == nil {
		return recipient
	}
	if len(opts.Notify) > 0 {
		recipient.Notify = make([]string, 0, len(opts.Notify))
		seen := make(map[string]struct{}, len(opts.Notify))
		for _, notify := range opts.Notify {
			value := strings.ToUpper(strings.TrimSpace(string(notify)))
			if value == "" {
				continue
			}
			if _, ok := seen[value]; ok {
				continue
			}
			seen[value] = struct{}{}
			recipient.Notify = append(recipient.Notify, value)
		}
	}
	recipient.OriginalRecipient = strings.TrimSpace(opts.OriginalRecipient)
	return recipient
}

func cloneDSNOptions(opts DSNOptions) DSNOptions {
	clone := DSNOptions{
		Return:     opts.Return,
		EnvelopeID: opts.EnvelopeID,
	}
	if len(opts.Recipients) > 0 {
		clone.Recipients = make([]DSNRecipientOptions, len(opts.Recipients))
		for i, recipient := range opts.Recipients {
			clone.Recipients[i] = recipient
			clone.Recipients[i].Notify = append([]string(nil), recipient.Notify...)
		}
	}
	return clone
}
