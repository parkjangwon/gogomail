package delivery

import (
	"errors"
	"fmt"
	"net/textproto"
	"strings"

	"github.com/gogomail/gogomail/internal/outbound"
)

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
