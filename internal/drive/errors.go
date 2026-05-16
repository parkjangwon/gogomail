package drive

import "errors"

var (
	ErrQuotaExceeded          = errors.New("drive quota exceeded")
	ErrDriveNodeAlreadyExists = errors.New("drive node already exists")
)
