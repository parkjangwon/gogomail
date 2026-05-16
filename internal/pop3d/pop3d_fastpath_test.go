package pop3d

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
)

func TestParsePOP3Command(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		line     string
		wantCmd  string
		wantArg1 string
		wantArg2 string
		wantArgc int
	}{
		{name: "empty", line: "", wantCmd: "", wantArgc: 0},
		{name: "spaces only", line: "   \t  ", wantCmd: "", wantArgc: 0},
		{name: "single command", line: "quit", wantCmd: "QUIT", wantArgc: 0},
		{name: "command with one arg", line: "USER alice", wantCmd: "USER", wantArg1: "alice", wantArgc: 1},
		{name: "command with tabs and spaces", line: "  LIST \t 1  ", wantCmd: "LIST", wantArg1: "1", wantArgc: 1},
		{name: "command with two args", line: "TOP 1 5", wantCmd: "TOP", wantArg1: "1", wantArg2: "5", wantArgc: 2},
		{name: "command with extra args", line: "PASS secret extra", wantCmd: "PASS", wantArg1: "secret", wantArg2: "extra", wantArgc: 2},
		{name: "mixed case", line: "aUtH plain Zm9v", wantCmd: "AUTH", wantArg1: "plain", wantArg2: "Zm9v", wantArgc: 2},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			cmd, arg1, arg2, argc := parsePOP3Command(tt.line)
			if cmd != tt.wantCmd || arg1 != tt.wantArg1 || arg2 != tt.wantArg2 || argc != tt.wantArgc {
				t.Fatalf("parsePOP3Command(%q) = %q %q %q argc=%d, want %q %q %q argc=%d",
					tt.line, cmd, arg1, arg2, argc, tt.wantCmd, tt.wantArg1, tt.wantArg2, tt.wantArgc)
			}
		})
	}
}

func TestWritePOP3MultilineStreamsAndDotStuffs(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sess := &session{writer: bufio.NewWriter(&buf)}

	sess.writeDotStuffedMultiline("Header: value\r\n\r\n.Line one\r\nWorld\r\n")
	if err := sess.writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got, want := buf.String(), "Header: value\r\n\r\n..Line one\r\nWorld\r\n"; got != want {
		t.Fatalf("writeDotStuffedMultiline() = %q, want %q", got, want)
	}
}

func TestWritePOP3TopMultilinePreservesHeaderSeparator(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sess := &session{writer: bufio.NewWriter(&buf)}

	sess.writeTopDotStuffedMultiline("From: a@example.com\r\nSubject: hi\r\n", 0)
	if err := sess.writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got, want := buf.String(), "From: a@example.com\r\nSubject: hi\r\n\r\n"; got != want {
		t.Fatalf("writeTopDotStuffedMultiline() = %q, want %q", got, want)
	}
}

func TestWritePOP3TopMultilineLimitsBodyLines(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	sess := &session{writer: bufio.NewWriter(&buf)}

	sess.writeTopDotStuffedMultiline("From: a@example.com\r\nSubject: hi\r\n\r\n.Line one\r\nWorld\r\nThird\r\n", 1)
	if err := sess.writer.Flush(); err != nil {
		t.Fatalf("flush: %v", err)
	}
	if got, want := buf.String(), "From: a@example.com\r\nSubject: hi\r\n\r\n..Line one\r\n"; got != want {
		t.Fatalf("writeTopDotStuffedMultiline() = %q, want %q", got, want)
	}
}

func BenchmarkParsePOP3Command(b *testing.B) {
	line := "TOP 123 10"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _, _, _ = parsePOP3Command(line)
	}
}

func BenchmarkWritePOP3Multiline(b *testing.B) {
	content := strings.Repeat("Subject: x\r\n", 20) + "\r\n" + strings.Repeat(".line\r\n", 20)
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	sess := &session{writer: writer}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		sess.writeDotStuffedMultiline(content)
		_ = writer.Flush()
		buf.Reset()
	}
}
