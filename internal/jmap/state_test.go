package jmap

import (
	"testing"
)

func TestStateParseModSeq(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"42", 42},
		{"0", 0},
		{"", 0},
		{"bad", 0},
		{"9999999", 9999999},
		{"-1", -1},
	}
	for _, c := range cases {
		got := ParseModSeqState(c.in)
		if got != c.want {
			t.Errorf("ParseModSeqState(%q) = %d, want %d", c.in, got, c.want)
		}
	}
}

func TestStateEmailStateForSignature(t *testing.T) {
	// Verify the function exists and has the expected signature.
	_ = EmailStateFor
}

func TestStateMailboxStateForSignature(t *testing.T) {
	// Verify the function exists and has the expected signature.
	_ = MailboxStateFor
}
