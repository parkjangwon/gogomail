package message

import "testing"

func TestNormalizeParseOptionsKeepsPositiveLimits(t *testing.T) {
	opts := normalizeParseOptions(ParseOptions{MaxTextBodyBytes: 64, MaxAttachments: 2, MaxParts: 3})
	if opts.MaxTextBodyBytes != 64 || opts.MaxAttachments != 2 || opts.MaxParts != 3 {
		t.Fatalf("normalizeParseOptions = %+v, want caller limits preserved", opts)
	}
}
