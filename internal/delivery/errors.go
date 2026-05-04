package delivery

import (
	"errors"
	"fmt"
	"net/textproto"
	"regexp"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

var enhancedStatusPattern = regexp.MustCompile(`\b[245]\.[0-9]{1,3}\.[0-9]{1,3}\b`)

type SMTPStatusError struct {
	Op      string
	Code    int
	Message string
	Err     error
}

func (e *SMTPStatusError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code > 0 {
		return fmt.Sprintf("smtp %s failed with %d: %s", e.Op, e.Code, e.Message)
	}
	if e.Err != nil {
		return fmt.Sprintf("smtp %s failed: %v", e.Op, e.Err)
	}
	return fmt.Sprintf("smtp %s failed", e.Op)
}

func (e *SMTPStatusError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func (e *SMTPStatusError) Permanent() bool {
	return e != nil && e.Code >= 500 && e.Code <= 599
}

func (e *SMTPStatusError) Temporary() bool {
	return e != nil && e.Code >= 400 && e.Code <= 499
}

func WrapSMTPError(op string, err error) error {
	if err == nil {
		return nil
	}
	var textErr *textproto.Error
	if errors.As(err, &textErr) {
		return &SMTPStatusError{
			Op:      op,
			Code:    textErr.Code,
			Message: textErr.Msg,
			Err:     err,
		}
	}
	return &SMTPStatusError{Op: op, Err: err}
}

func IsPermanentFailure(err error) bool {
	var smtpErr *SMTPStatusError
	return errors.As(err, &smtpErr) && smtpErr.Permanent()
}

func IsTemporaryFailure(err error) bool {
	var smtpErr *SMTPStatusError
	return errors.As(err, &smtpErr) && smtpErr.Temporary()
}

func enhancedStatusForAttempt(status AttemptStatus, err error) string {
	if code := enhancedStatusFromError(err); code != "" {
		return code
	}
	switch status {
	case AttemptDelivered:
		return "2.0.0"
	case AttemptBounced:
		return "5.0.0"
	default:
		return "4.0.0"
	}
}

func enhancedStatusFromError(err error) string {
	var smtpErr *SMTPStatusError
	if !errors.As(err, &smtpErr) {
		return ""
	}
	class := smtpErr.Code / 100
	for _, code := range enhancedStatusPattern.FindAllString(smtpErr.Message, -1) {
		if validEnhancedDeliveryStatus(code) && int(code[0]-'0') == class {
			return code
		}
	}
	return ""
}

func validEnhancedDeliveryStatus(status string) bool {
	parts := strings.Split(status, ".")
	if len(parts) != 3 || len(parts[0]) != 1 {
		return false
	}
	if parts[0][0] < '2' || parts[0][0] > '5' || parts[0][0] == '3' {
		return false
	}
	for _, part := range parts[1:] {
		if part == "" || len(part) > 3 {
			return false
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return false
			}
		}
	}
	return true
}

type RecipientDeliveryError struct {
	Recipient outbound.Address
	Err       error
}

func (e RecipientDeliveryError) Error() string {
	if e.Err == nil {
		return fmt.Sprintf("recipient %s failed", e.Recipient.Email)
	}
	return fmt.Sprintf("recipient %s: %v", e.Recipient.Email, e.Err)
}

func (e RecipientDeliveryError) Unwrap() error {
	return e.Err
}

type PartialDeliveryError struct {
	Delivered []outbound.Address
	Failed    []RecipientDeliveryError
}

func (e *PartialDeliveryError) Error() string {
	if e == nil {
		return ""
	}
	parts := make([]string, 0, len(e.Failed))
	for _, failure := range e.Failed {
		parts = append(parts, failure.Error())
	}
	return fmt.Sprintf("partial delivery: %d delivered, %d failed: %s", len(e.Delivered), len(e.Failed), strings.Join(parts, "; "))
}

func (e *PartialDeliveryError) TemporaryFailures() []outbound.Address {
	if e == nil {
		return nil
	}
	recipients := make([]outbound.Address, 0, len(e.Failed))
	for _, failure := range e.Failed {
		if !IsPermanentFailure(failure.Err) {
			recipients = append(recipients, failure.Recipient)
		}
	}
	return recipients
}
