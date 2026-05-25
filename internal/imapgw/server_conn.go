package imapgw

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func acquireIMAPConnectionSlot(slots chan struct{}) bool {
	if slots == nil {
		return true
	}
	select {
	case slots <- struct{}{}:
		return true
	default:
		return false
	}
}

func releaseIMAPConnectionSlot(slots chan struct{}) {
	if slots == nil {
		return
	}
	<-slots
}

func rejectIMAPConnectionLimit(conn net.Conn) {
	defer conn.Close()
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, _ = io.WriteString(conn, "* BYE [ALERT] gogomail IMAP4rev1 server connection limit reached\r\n")
}

func (s *Server) setReadDeadline(conn net.Conn, timeout time.Duration) error {
	if conn == nil {
		return nil
	}
	if timeout <= 0 {
		return conn.SetReadDeadline(time.Time{})
	}
	return conn.SetReadDeadline(time.Now().Add(timeout))
}

func (s *Server) setWriteDeadline(conn net.Conn) error {
	if conn == nil {
		return nil
	}
	if s.options.WriteTimeout <= 0 {
		return conn.SetWriteDeadline(time.Time{})
	}
	return conn.SetWriteDeadline(time.Now().Add(s.options.WriteTimeout))
}

func (s *Server) setHandshakeDeadline(conn net.Conn) error {
	timeout := s.options.ReadTimeout
	if s.options.WriteTimeout > timeout {
		timeout = s.options.WriteTimeout
	}
	if conn == nil || timeout <= 0 {
		return nil
	}
	return conn.SetDeadline(time.Now().Add(timeout))
}

func (s *Server) readCommandLine(reader *bufio.Reader, writer *bufio.Writer, state *imapConnState) (string, []string, error) {
	line, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
	if err != nil {
		if errors.Is(err, errIMAPCommandLineTooLong) {
			return "", nil, imapProtocolFramingError{message: "command line is too long"}
		}
		return "", nil, err
	}
	if !imapLineHasCRLF(line) {
		if state != nil && state.pendingAuthTag != "" {
			return "", nil, imapProtocolFramingError{line: state.pendingAuthTag + " AUTHENTICATE", message: "command line must end with CRLF"}
		}
		return "", nil, imapProtocolFramingError{line: strings.TrimRight(line, "\n"), message: "command line must end with CRLF"}
	}
	if state != nil && (state.pendingIdleTag != "" || state.pendingAuthTag != "") {
		return line, nil, nil
	}
	var command strings.Builder
	command.WriteString(strings.TrimRight(line, "\r\n"))
	literals := make([]string, 0, 1)
	totalLiteralBytes := 0
	for {
		literalSize, nonSync, ok, err := imapCommandLiteralSize(command.String())
		if err != nil {
			if errors.Is(err, errIMAPCommandLiteralTooLarge) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
			}
			if errors.Is(err, errIMAPCommandLiteralInvalid) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal size is invalid"}
			}
			return command.String(), literals, err
		}
		if !ok {
			return command.String(), literals, nil
		}
		if literalSize > maxIMAPCommandLiteralBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
		}
		if totalLiteralBytes+literalSize > maxIMAPCommandLiteralBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command literal is too large"}
		}
		if !nonSync {
			if _, err := writer.WriteString("+ Ready for literal data\r\n"); err != nil {
				return "", nil, err
			}
			if err := writer.Flush(); err != nil {
				return "", nil, err
			}
		}
		literal := make([]byte, literalSize)
		if _, err := io.ReadFull(reader, literal); err != nil {
			return "", nil, err
		}
		totalLiteralBytes += literalSize
		literals = append(literals, string(literal))
		suffix, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
		if err != nil {
			if errors.Is(err, errIMAPCommandLineTooLong) {
				return "", nil, imapProtocolFramingError{line: command.String(), message: "command line is too long"}
			}
			return "", nil, err
		}
		if !imapLineHasCRLF(suffix) {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command line must end with CRLF"}
		}
		if suffix == "\r\n" {
			return command.String(), literals, nil
		}
		if command.Len()+len(suffix) > maxIMAPCommandLineBytes {
			return "", nil, imapProtocolFramingError{line: command.String(), message: "command line is too long"}
		}
		command.WriteString(strings.TrimRight(suffix, "\r\n"))
	}
}

type imapProtocolFramingError struct {
	line    string
	message string
}

func (err imapProtocolFramingError) Error() string {
	if err.message == "" {
		return "imap protocol framing error"
	}
	return "imap " + err.message
}

func writeIMAPFramingError(writer *bufio.Writer, line string, message string) error {
	if message == "" {
		message = "protocol framing error"
	}
	if tag := imapTagFromCommandLine(line); tag != "" {
		if _, err := writer.WriteString(tag + " BAD " + message + "\r\n"); err != nil {
			return err
		}
	} else {
		if _, err := writer.WriteString("* BAD " + message + "\r\n"); err != nil {
			return err
		}
	}
	_, err := writer.WriteString("* BYE gogomail IMAP4rev1 server closing connection after framing error\r\n")
	return err
}

func imapTagFromCommandLine(line string) string {
	start := 0
	for start < len(line) && isIMAPSpace(line[start]) {
		start++
	}
	end := len(line)
	for end > start && isIMAPSpace(line[end-1]) {
		end--
	}
	if start >= end {
		return ""
	}
	for i := start; i < end; i++ {
		if isIMAPSpace(line[i]) {
			tag := line[start:i]
			if !imapTagValid(tag) {
				return ""
			}
			return tag
		}
	}
	if !imapTagValid(line[start:end]) {
		return ""
	}
	return line[start:end]
}

func writeIMAPUintLine(writer *bufio.Writer, prefix string, value uint64, suffix string) error {
	var buf [64]byte
	out := append(buf[:0], prefix...)
	out = strconv.AppendUint(out, value, 10)
	out = append(out, suffix...)
	_, err := writer.Write(out)
	return err
}

func isIMAPSpace(b byte) bool {
	switch b {
	case ' ', '\t', '\v', '\f':
		return true
	default:
		return false
	}
}

func readIMAPLine(reader *bufio.Reader, maxBytes int) (string, error) {
	if reader == nil {
		return "", fmt.Errorf("imap reader is required")
	}
	if maxBytes <= 0 {
		return "", fmt.Errorf("imap line limit is invalid")
	}
	var line []byte
	for {
		fragment, err := reader.ReadSlice('\n')
		if len(line)+len(fragment) > maxBytes {
			return "", errIMAPCommandLineTooLong
		}
		line = append(line, fragment...)
		if err == nil {
			return string(line), nil
		}
		if errors.Is(err, bufio.ErrBufferFull) {
			continue
		}
		return "", err
	}
}

func imapLineHasCRLF(line string) bool {
	return strings.HasSuffix(line, "\r\n")
}

func imapCommandLiteralSize(line string) (int, bool, bool, error) {
	trimmed := strings.TrimRight(line, "\r\n")
	if !strings.HasSuffix(trimmed, "}") {
		return 0, false, false, nil
	}
	start := strings.LastIndex(trimmed, "{")
	if start < 0 {
		return 0, false, false, nil
	}
	if start > 0 && trimmed[start-1] != ' ' && trimmed[start-1] != '\t' && trimmed[start-1] != '(' {
		return 0, false, false, nil
	}
	value := trimmed[start+1 : len(trimmed)-1]
	if value == "" {
		return 0, false, false, fmt.Errorf("imap literal size is required")
	}
	nonSync := strings.HasSuffix(value, "+")
	if nonSync {
		value = strings.TrimSuffix(value, "+")
		if value == "" {
			return 0, false, true, fmt.Errorf("imap literal size is required")
		}
	}
	if len(value) > 1 && value[0] == '0' {
		return 0, nonSync, true, errIMAPCommandLiteralInvalid
	}
	var size int64
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return 0, nonSync, true, errIMAPCommandLiteralInvalid
		}
		size = size*10 + int64(value[i]-'0')
		if size > maxIMAPCommandLiteralBytes {
			return 0, nonSync, true, errIMAPCommandLiteralTooLarge
		}
	}
	return int(size), nonSync, true, nil
}

// authFailureTracker tracks per-IP auth failures for brute-force protection.

func imapRemoteAddrIP(addr net.Addr) string {
	if addr == nil {
		return ""
	}
	host, _, err := net.SplitHostPort(addr.String())
	if err != nil {
		return addr.String()
	}
	return host
}
