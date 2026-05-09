package drive

import "errors"

var (
	ErrQuotaExceeded = errors.New("drive quota exceeded")
)
