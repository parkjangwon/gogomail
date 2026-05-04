package mail

import "errors"

// ErrMailboxFull is returned when a write would exceed the recipient's storage quota.
var ErrMailboxFull = errors.New("mailbox full")
