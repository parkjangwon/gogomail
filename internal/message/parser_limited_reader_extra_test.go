package message

import (
	"io"
	"strings"
	"testing"
)

func TestReadLimitedTextDoesNotReadPastTruncationProbe(t *testing.T) {
	reader := &countingReader{src: strings.NewReader(strings.Repeat("x", 1<<20))}

	body, truncated, err := readLimitedText(reader, 32)
	if err != nil {
		t.Fatalf("readLimitedText returned error: %v", err)
	}
	if !truncated {
		t.Fatal("readLimitedText did not report truncation")
	}
	if len(body) != 32 {
		t.Fatalf("body length = %d, want 32", len(body))
	}
	if reader.bytesRead != 33 {
		t.Fatalf("bytes read = %d, want max+1 truncation probe", reader.bytesRead)
	}
}

type countingReader struct {
	src       io.Reader
	bytesRead int
}

func (r *countingReader) Read(p []byte) (int, error) {
	n, err := r.src.Read(p)
	r.bytesRead += n
	return n, err
}
