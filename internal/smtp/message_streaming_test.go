package smtpd

import (
	"bufio"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
)

func TestHeaderBufferPrepend(t *testing.T) {
	hb := NewHeaderBuffer()
	hb.AddPrepend("Received: from example.com")
	hb.AddPrepend("Received: from other.com")

	if len(hb.prepend) != 2 {
		t.Errorf("expected 2 prepend headers, got %d", len(hb.prepend))
	}
	if !strings.HasSuffix(hb.prepend[0], "\r\n") {
		t.Error("prepend header should end with CRLF")
	}
}

func TestHeaderBufferAfterTrace(t *testing.T) {
	hb := NewHeaderBuffer()
	hb.AddAfterTrace("Message-ID: <test@example.com>")
	hb.AddAfterTrace("Authentication-Results: example.com")

	if len(hb.afterTrace) != 2 {
		t.Errorf("expected 2 afterTrace headers, got %d", len(hb.afterTrace))
	}
	if !strings.HasSuffix(hb.afterTrace[0], "\r\n") {
		t.Error("afterTrace header should end with CRLF")
	}
}

func TestHeaderBufferApplyToFileWithReceivedChain(t *testing.T) {
	// Create original message with Received chain
	original, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(original.Name())

	originalContent := `Received: from server1.example.com [192.0.2.1]
	by server2.example.com with SMTP id 12345
	for <user@example.com>; Mon, 14 May 2024 10:00:00 +0000
Subject: Test Message
From: sender@example.com
To: user@example.com

Test body
`
	if _, err := io.WriteString(original, originalContent); err != nil {
		t.Fatalf("write original: %v", err)
	}

	// Create header buffer
	hb := NewHeaderBuffer()
	hb.AddPrepend("Received: from test.local [127.0.0.1]\r\n")
	hb.AddAfterTrace("Message-ID: <test@example.com>\r\n")
	hb.AddAfterTrace("Authentication-Results: example.com; pass\r\n")

	// Apply headers
	output, size, err := hb.ApplyToFile(original)
	if err != nil {
		t.Fatalf("apply headers: %v", err)
	}
	defer cleanupSpool(output)

	if size == 0 {
		t.Error("expected non-zero size")
	}

	// Verify output structure
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek to start: %v", err)
	}

	reader := bufio.NewReader(output)
	lines := []string{}
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			lines = append(lines, line)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read line: %v", err)
		}
	}

	if len(lines) == 0 {
		t.Fatal("expected output lines")
	}

	// First line should be the prepended header
	if !strings.HasPrefix(lines[0], "Received: from test.local") {
		t.Errorf("first line should be prepended header, got %q", lines[0])
	}

	// Find where Message-ID appears (should be after Received chain)
	hasMessageID := false
	hasAuthResults := false

	for _, line := range lines {

		if strings.HasPrefix(strings.ToLower(line), "message-id:") {
			hasMessageID = true
		}
		if strings.HasPrefix(strings.ToLower(line), "authentication-results:") {
			hasAuthResults = true
		}
	}

	if !hasMessageID {
		t.Error("expected Message-ID header in output")
	}
	if !hasAuthResults {
		t.Error("expected Authentication-Results header in output")
	}
}

func TestHeaderBufferApplyToFileWithoutReceivedChain(t *testing.T) {
	// Create message without Received headers
	original, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(original.Name())

	originalContent := `Subject: Test
From: sender@example.com
To: user@example.com

Body
`
	if _, err := io.WriteString(original, originalContent); err != nil {
		t.Fatalf("write original: %v", err)
	}

	hb := NewHeaderBuffer()
	hb.AddPrepend("Received: from test.local\r\n")
	hb.AddAfterTrace("Message-ID: <test@example.com>\r\n")

	output, size, err := hb.ApplyToFile(original)
	if err != nil {
		t.Fatalf("apply headers: %v", err)
	}
	defer cleanupSpool(output)

	if size == 0 {
		t.Error("expected non-zero size")
	}

	// Verify both headers are present
	if _, err := output.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("seek to start: %v", err)
	}

	content, err := io.ReadAll(output)
	if err != nil {
		t.Fatalf("read all: %v", err)
	}

	contentStr := string(content)
	if !strings.Contains(contentStr, "Received: from test.local") {
		t.Error("expected Received header in output")
	}
	if !strings.Contains(contentStr, "Message-ID: <test@example.com>") {
		t.Error("expected Message-ID header in output")
	}
}

func TestHeaderBufferApplyToFileRejectsOverlongLine(t *testing.T) {
	t.Parallel()

	original, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer original.Close()
	defer os.Remove(original.Name())

	overlong := "Subject: " + strings.Repeat("x", maxSMTPSpoolLineBytes) + "\r\n\r\nBody\r\n"
	if _, err := io.WriteString(original, overlong); err != nil {
		t.Fatalf("write original: %v", err)
	}

	hb := NewHeaderBuffer()
	hb.AddAfterTrace("Message-ID: <test@example.com>\r\n")
	_, _, err = hb.ApplyToFile(original)
	if !errors.Is(err, errSMTPSpoolLineTooLong) {
		t.Fatalf("ApplyToFile err = %v, want overlong line rejection", err)
	}
}

func TestInsertHeaderAfterTraceHeadersRejectsOverlongLine(t *testing.T) {
	t.Parallel()

	original, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer original.Close()
	defer os.Remove(original.Name())

	overlong := "Received: " + strings.Repeat("x", maxSMTPSpoolLineBytes) + "\r\n\r\nBody\r\n"
	if _, err := io.WriteString(original, overlong); err != nil {
		t.Fatalf("write original: %v", err)
	}

	_, _, err = insertHeaderAfterTraceHeaders(original, "Message-ID: <test@example.com>\r\n")
	if !errors.Is(err, errSMTPSpoolLineTooLong) {
		t.Fatalf("insertHeaderAfterTraceHeaders err = %v, want overlong line rejection", err)
	}
}

func TestChunkedAuthenticationReader(t *testing.T) {
	// Create a test file with known content
	file, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(file.Name())

	testData := strings.Repeat("A", 1000) // 1000 bytes of 'A'
	if _, err := io.WriteString(file, testData); err != nil {
		t.Fatalf("write test data: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	// Reopen for reading
	file, err = os.Open(file.Name())
	if err != nil {
		t.Fatalf("reopen file: %v", err)
	}

	reader := NewChunkedAuthenticationReader(file, 100) // 100 byte chunks

	chunks := [][]byte{}
	for {
		chunk, err := reader.ReadChunk()
		if len(chunk) > 0 {
			chunks = append(chunks, chunk)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read chunk: %v", err)
		}
	}

	if len(chunks) != 10 {
		t.Errorf("expected 10 chunks, got %d", len(chunks))
	}

	// Verify all chunks can be reassembled
	var reassembled strings.Builder
	for _, chunk := range chunks {
		reassembled.Write(chunk)
	}

	if reassembled.String() != testData {
		t.Error("reassembled data does not match original")
	}
}

func TestChunkedAuthenticationReaderSeek(t *testing.T) {
	file, err := os.CreateTemp("", "test-*.eml")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	defer os.Remove(file.Name())

	testData := "Test data for seeking"
	if _, err := io.WriteString(file, testData); err != nil {
		t.Fatalf("write test data: %v", err)
	}

	if err := file.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}

	// Reopen for reading
	file, err = os.Open(file.Name())
	if err != nil {
		t.Fatalf("reopen file: %v", err)
	}

	reader := NewChunkedAuthenticationReader(file, 10)

	// Read first chunk
	chunk1, _ := reader.ReadChunk()
	if len(chunk1) == 0 {
		t.Fatal("expected first chunk")
	}

	// Seek back to start
	if err := reader.Seek(); err != nil {
		t.Fatalf("seek failed: %v", err)
	}

	// Read again should give same result
	chunk2, _ := reader.ReadChunk()
	if string(chunk1) != string(chunk2) {
		t.Errorf("chunks differ after seek: %q vs %q", chunk1, chunk2)
	}
}

func TestHeaderBufferMultipleHeaders(t *testing.T) {
	hb := NewHeaderBuffer()

	// Add multiple headers
	headers := []string{
		"Received: from server1",
		"Received: from server2",
		"Message-ID: <test@example.com>",
		"Authentication-Results: example.com; pass",
	}

	for _, h := range headers {
		if strings.HasPrefix(h, "Received:") {
			hb.AddPrepend(h)
		} else {
			hb.AddAfterTrace(h)
		}
	}

	if len(hb.prepend) != 2 {
		t.Errorf("expected 2 prepend headers, got %d", len(hb.prepend))
	}
	if len(hb.afterTrace) != 2 {
		t.Errorf("expected 2 afterTrace headers, got %d", len(hb.afterTrace))
	}
}

func BenchmarkHeaderBufferApplyToFile(b *testing.B) {
	// Create a test message once
	template, err := os.CreateTemp("", "bench-*.eml")
	if err != nil {
		b.Fatalf("create temp: %v", err)
	}
	defer os.Remove(template.Name())

	largeBody := strings.Repeat("X", 1024*1024) // 1MB body
	if _, err := io.WriteString(template, "Subject: Test\r\n\r\n"+largeBody); err != nil {
		b.Fatalf("write template: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		// Copy template for each iteration
		testFile, _ := os.CreateTemp("", "bench-*.eml")
		template.Seek(0, io.SeekStart)
		io.Copy(testFile, template)

		hb := NewHeaderBuffer()
		hb.AddPrepend("Received: from test\r\n")
		hb.AddAfterTrace("Message-ID: <test@example.com>\r\n")

		output, _, err := hb.ApplyToFile(testFile)
		if err != nil {
			b.Fatalf("apply headers: %v", err)
		}
		os.Remove(output.Name())
	}
}
