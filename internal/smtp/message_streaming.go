package smtpd

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"
)

// HeaderBuffer collects headers to be inserted into a message without rewriting
// the entire file multiple times. This is a memory optimization to reduce I/O pressure.
type HeaderBuffer struct {
	prepend       []string // Headers to prepend (e.g., Received)
	afterTrace    []string // Headers to insert after Received chain (e.g., Authentication-Results)
	afterTracePos int64    // Position in file where afterTrace headers should be inserted
}

// NewHeaderBuffer creates a new header buffer for efficient header insertion.
func NewHeaderBuffer() *HeaderBuffer {
	return &HeaderBuffer{
		prepend:    []string{},
		afterTrace: []string{},
	}
}

// AddPrepend queues a header to be prepended to the message.
// Typical use: Received header (must be first).
func (hb *HeaderBuffer) AddPrepend(header string) {
	if !strings.HasSuffix(header, "\r\n") {
		header += "\r\n"
	}
	hb.prepend = append(hb.prepend, header)
}

// AddAfterTrace queues a header to be inserted after Received chain.
// Typical use: Message-ID, Authentication-Results headers.
func (hb *HeaderBuffer) AddAfterTrace(header string) {
	if !strings.HasSuffix(header, "\r\n") {
		header += "\r\n"
	}
	hb.afterTrace = append(hb.afterTrace, header)
}

// ApplyToFile efficiently inserts all buffered headers in a single pass,
// avoiding multiple file rewrites. Returns new file handle and total size.
func (hb *HeaderBuffer) ApplyToFile(original *os.File) (*os.File, int64, error) {
	if _, err := original.Seek(0, io.SeekStart); err != nil {
		return nil, 0, fmt.Errorf("rewind message for header apply: %w", err)
	}

	output, err := os.CreateTemp("", "gogomail-spool-*.eml")
	if err != nil {
		return nil, 0, fmt.Errorf("create output spool: %w", err)
	}

	var totalWritten int64

	// Write prepended headers
	for _, header := range hb.prepend {
		n, err := io.WriteString(output, header)
		if err != nil {
			cleanupSpool(output)
			return nil, 0, fmt.Errorf("write prepend header: %w", err)
		}
		totalWritten += int64(n)
	}

	// Scan original message and insert afterTrace headers after Received chain
	reader := bufio.NewReader(original)
	inTrace := false
	afterTraceInserted := false

	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			isContinuation := len(line) > 0 && (line[0] == ' ' || line[0] == '\t')
			isReceived := strings.HasPrefix(strings.ToLower(line), "received:")

			// Insert afterTrace headers when we transition out of Received chain
			// or before the first header if there are no Received headers
			if !afterTraceInserted && !isReceived && !(inTrace && isContinuation) {
				for _, header := range hb.afterTrace {
					n, writeErr := io.WriteString(output, header)
					if writeErr != nil {
						cleanupSpool(output)
						return nil, 0, fmt.Errorf("write afterTrace header: %w", writeErr)
					}
					totalWritten += int64(n)
				}
				afterTraceInserted = true
			}

			n, writeErr := io.WriteString(output, line)
			if writeErr != nil {
				cleanupSpool(output)
				return nil, 0, fmt.Errorf("copy message line: %w", writeErr)
			}
			totalWritten += int64(n)

			inTrace = isReceived || (inTrace && isContinuation)
		}

		if err != nil {
			if err == io.EOF {
				break
			}
			cleanupSpool(output)
			return nil, 0, fmt.Errorf("read message for header apply: %w", err)
		}
	}

	// Safety check: if we still haven't inserted afterTrace (shouldn't happen),
	// insert them now
	if !afterTraceInserted && len(hb.afterTrace) > 0 {
		for _, header := range hb.afterTrace {
			n, writeErr := io.WriteString(output, header)
			if writeErr != nil {
				cleanupSpool(output)
				return nil, 0, fmt.Errorf("write afterTrace header (safety): %w", writeErr)
			}
			totalWritten += int64(n)
		}
	}

	if err := original.Close(); err != nil {
		cleanupSpool(output)
		return nil, 0, fmt.Errorf("close original spool: %w", err)
	}

	return output, totalWritten, nil
}

// ChunkedAuthenticationReader wraps a file reader to verify authentication
// without loading the entire message into memory. It reads and verifies in chunks.
type ChunkedAuthenticationReader struct {
	file      *os.File
	chunkSize int64
}

// NewChunkedAuthenticationReader creates a reader for memory-efficient auth verification.
func NewChunkedAuthenticationReader(file *os.File, chunkSize int64) *ChunkedAuthenticationReader {
	if chunkSize <= 0 {
		chunkSize = 64 * 1024 // 64KB default chunk
	}
	return &ChunkedAuthenticationReader{
		file:      file,
		chunkSize: chunkSize,
	}
}

// ReadChunk returns the next chunk of data from the message file.
// Returns empty slice and io.EOF when done.
func (car *ChunkedAuthenticationReader) ReadChunk() ([]byte, error) {
	buf := make([]byte, car.chunkSize)
	n, err := car.file.Read(buf)
	if n > 0 {
		return buf[:n], nil
	}
	return nil, err
}

// Seek resets the reader position to the start.
func (car *ChunkedAuthenticationReader) Seek() error {
	_, err := car.file.Seek(0, io.SeekStart)
	return err
}
