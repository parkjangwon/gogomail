package httpapi

import "testing"

func TestSanitizeCSVCell(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		// Formula prefixes must be neutralised
		{"=HYPERLINK(\"http://evil.com\",\"click\")", "\t=HYPERLINK(\"http://evil.com\",\"click\")"},
		{"+1234", "\t+1234"},
		{"-1234", "\t-1234"},
		{"@SUM(A1:A10)", "\t@SUM(A1:A10)"},
		// Ordinary values must pass through unchanged
		{"user@example.com", "user@example.com"},
		{"Alice Smith", "Alice Smith"},
		{"", ""},
		{"normal string", "normal string"},
		// Literal tab / CR at start (already neutralised or harmless) also get prefixed
		{"\tformula", "\t\tformula"},
		{"\rformula", "\t\rformula"},
	}
	for _, c := range cases {
		got := sanitizeCSVCell(c.in)
		if got != c.want {
			t.Errorf("sanitizeCSVCell(%q) = %q; want %q", c.in, got, c.want)
		}
	}
}
