package imapgw

import "testing"

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
