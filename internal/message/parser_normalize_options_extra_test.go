package message

import "testing"

func TestNormalizeParseOptionsAppliesSafeDefaults(t *testing.T) {
	opts := normalizeParseOptions(ParseOptions{})
	if opts.MaxTextBodyBytes != 1<<20 {
		t.Fatalf("MaxTextBodyBytes = %d, want 1MiB", opts.MaxTextBodyBytes)
	}
	if opts.MaxAttachments != 1000 {
		t.Fatalf("MaxAttachments = %d, want 1000", opts.MaxAttachments)
	}
	if opts.MaxParts != 10000 {
		t.Fatalf("MaxParts = %d, want 10000", opts.MaxParts)
	}
}
