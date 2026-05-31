package smtpd

import (
	"bufio"
	"errors"
	"fmt"
)

const maxSMTPSpoolLineBytes = 1000

var errSMTPSpoolLineTooLong = errors.New("smtp spool line too long")

func readSMTPSpoolLine(reader *bufio.Reader) (string, error) {
	if reader == nil {
		return "", fmt.Errorf("smtp spool reader is required")
	}
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxSMTPSpoolLineBytes {
			return "", errSMTPSpoolLineTooLong
		}
		line = append(line, fragment...)
		if err == nil {
			return string(line), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return string(line), err
	}
}
