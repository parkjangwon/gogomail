package delivery

import (
	"errors"
	"fmt"
	"net/textproto"
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
