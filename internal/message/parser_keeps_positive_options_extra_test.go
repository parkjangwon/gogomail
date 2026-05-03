package message

import "testing"

func TestNormalizeParseOptionsKeepsPositiveLimits(t *testing.T) {
	opts := normalizeParseOptions(ParseOptions{MaxTextBodyBytes: 64, MaxAttachments: 2})
	if opts.MaxTextBodyBytes != 64 || opts.MaxAttachments != 2 {
		t.Fatalf("normalizeParseOptions = %+v, want caller limits preserved", opts)
	}
}
