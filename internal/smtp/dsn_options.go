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
		recipient.Notify = orderDSNNotify(recipient.Notify)
	}
	recipient.OriginalRecipient = normalizeDSNOriginalRecipient(opts)
	return recipient
}

func normalizeDSNOriginalRecipient(opts *gosmtp.RcptOptions) string {
	if opts == nil {
		return ""
	}
	address := strings.TrimSpace(opts.OriginalRecipient)
	if address == "" {
		return ""
	}
	addressType := strings.TrimSpace(string(opts.OriginalRecipientType))
	if addressType == "" {
		return address
	}
	return strings.ToUpper(addressType) + ";" + encodeDSNXText(address)
}

func encodeDSNXText(value string) string {
	var b strings.Builder
	for _, r := range value {
		if r >= 33 && r <= 126 && r != '+' && r != '=' {
			b.WriteRune(r)
			continue
		}
		for _, c := range []byte(string(r)) {
			b.WriteByte('+')
			const hex = "0123456789ABCDEF"
			b.WriteByte(hex[c>>4])
			b.WriteByte(hex[c&0x0f])
		}
	}
	return b.String()
}

func orderDSNNotify(values []string) []string {
	if len(values) <= 1 {
		return values
	}
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	ordered := values[:0]
	for _, value := range []string{"NEVER", "SUCCESS", "FAILURE", "DELAY"} {
		if _, ok := seen[value]; ok {
			ordered = append(ordered, value)
		}
	}
	return ordered
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
