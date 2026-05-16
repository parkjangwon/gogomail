package imapgw

import (
	"bufio"
	"bytes"
	"testing"
)

func TestIMAPTagFromCommandLine(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		line string
		want string
	}{
		{name: "empty", line: "", want: ""},
		{name: "spaces only", line: "   \t  ", want: ""},
		{name: "tag only", line: "a1", want: "a1"},
		{name: "tag with command", line: "a2 NOOP", want: "a2"},
		{name: "leading trailing spaces", line: "  a3 LOGOUT  ", want: "a3"},
		{name: "invalid tag", line: "a+b NOOP", want: ""},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := imapTagFromCommandLine(tt.line); got != tt.want {
				t.Fatalf("imapTagFromCommandLine(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func BenchmarkIMAPTagFromCommandLine(b *testing.B) {
	line := "a12345 NOOP"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = imapTagFromCommandLine(line)
	}
}

func BenchmarkIMAPUIDSetResponse(b *testing.B) {
	uids := []UID{1, 2, 3, 5, 6, 10, 11, 12, 20}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = imapUIDSetResponse(uids)
	}
}

func BenchmarkIMAPFetchLine(b *testing.B) {
	var buf bytes.Buffer
	writer := bufio.NewWriter(&buf)
	attrs := "UID 7 FLAGS (\\Seen \\Flagged) RFC822.SIZE 1024"
	tail := ")"
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if err := writeIMAPFetchLine(writer, 42, attrs, tail); err != nil {
			b.Fatalf("writeIMAPFetchLine: %v", err)
		}
		if err := writer.Flush(); err != nil {
			b.Fatalf("flush: %v", err)
		}
		buf.Reset()
	}
}
