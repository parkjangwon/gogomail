package maildb

import (
	"strings"
	"testing"
)

func TestValidateFolderNameRejectsUnsafeNames(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"Projects/2026",
		"Projects\\2026",
		"Projects\r\nInjected",
		strings.Repeat("x", maxFolderNameBytes+1),
	}
	for _, name := range tests {
		if err := validateFolderName(name); err == nil {
			t.Fatalf("validateFolderName accepted %q", name)
		}
	}
}

func TestValidateFolderNameAcceptsTrimmedDisplayName(t *testing.T) {
	t.Parallel()

	if err := validateFolderName(" Projects "); err != nil {
		t.Fatalf("validateFolderName returned error: %v", err)
	}
}
