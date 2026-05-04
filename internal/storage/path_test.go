package storage

import (
	"strings"
	"testing"
)

func TestValidateObjectPathRejectsUnsafeKeys(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		" ",
		"../escape.eml",
		"/var/mail/message.eml",
		`mailstore\message.eml`,
		"mailstore/message\n.eml",
		"mailstore/../message.eml",
		"mailstore/./message.eml",
		"mailstore//message.eml",
		"mailstore/message.eml/",
	}
	for _, objectPath := range tests {
		if _, err := ValidateObjectPath(objectPath); err == nil {
			t.Fatalf("ValidateObjectPath accepted unsafe path %q", objectPath)
		}
	}
}

func TestValidateObjectPathTrimsValidKey(t *testing.T) {
	t.Parallel()

	got, err := ValidateObjectPath(" mailstore/company/domain/message.eml ")
	if err != nil {
		t.Fatalf("ValidateObjectPath returned error: %v", err)
	}
	if got != "mailstore/company/domain/message.eml" {
		t.Fatalf("ValidateObjectPath = %q", got)
	}
}

func TestValidateObjectPathRejectsWhitespaceOnlySegments(t *testing.T) {
	t.Parallel()

	if _, err := ValidateObjectPath("mailstore/   /message.eml"); err == nil {
		t.Fatal("ValidateObjectPath accepted whitespace-only segment")
	}
}

func TestValidateObjectPathRejectsOversizedKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
	}{
		{
			name: "total length",
			key:  "mailstore/" + strings.Repeat("a", MaxObjectPathBytes),
		},
		{
			name: "segment length",
			key:  "mailstore/" + strings.Repeat("a", MaxObjectPathSegmentBytes+1) + "/message.eml",
		},
	}
	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateObjectPath(tt.key); err == nil {
				t.Fatalf("ValidateObjectPath accepted oversized key %q", tt.key)
			}
		})
	}
}

func TestValidateObjectPathAcceptsLongRelativeKey(t *testing.T) {
	t.Parallel()

	key := "mailstore/" + strings.Repeat("a", 128) + "/message.eml"
	if got, err := ValidateObjectPath(key); err != nil || got != key {
		t.Fatalf("ValidateObjectPath = %q, %v", got, err)
	}
}
