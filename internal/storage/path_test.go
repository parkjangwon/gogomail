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
		"mailstore/message-\xff.eml",
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

func TestValidateObjectPrefixAllowsEmptyAndTrailingSlash(t *testing.T) {
	t.Parallel()

	if got, err := ValidateObjectPrefix(" "); err != nil || got != "" {
		t.Fatalf("empty prefix = %q, %v", got, err)
	}
	if got, err := ValidateObjectPrefix(" drive/user-1/ "); err != nil || got != "drive/user-1" {
		t.Fatalf("prefix = %q, %v", got, err)
	}
}

func TestValidateObjectPrefixRejectsUnsafePrefixes(t *testing.T) {
	t.Parallel()

	for _, prefix := range []string{
		"../escape",
		"/drive/user-1",
		`drive\user-1`,
		"drive/user\n1",
		"drive/user-\xff",
		"drive/../user-1",
		"drive//user-1",
		"drive/   /user-1",
	} {
		if _, err := ValidateObjectPrefix(prefix); err == nil {
			t.Fatalf("ValidateObjectPrefix accepted unsafe prefix %q", prefix)
		}
	}
}

func TestValidateListCursorRejectsUnsafeCursor(t *testing.T) {
	t.Parallel()

	if _, err := ValidateListCursor("cursor\n2"); err == nil {
		t.Fatal("ValidateListCursor accepted newline-bearing cursor")
	}
	if _, err := ValidateListCursor(strings.Repeat("x", MaxListCursorBytes+1)); err == nil {
		t.Fatal("ValidateListCursor accepted oversized cursor")
	}
	if _, err := ValidateListCursor("cursor-\xff"); err == nil {
		t.Fatal("ValidateListCursor accepted invalid UTF-8 cursor")
	}
}

func TestValidateRangeRequestRejectsInvalidRanges(t *testing.T) {
	t.Parallel()

	for _, req := range []RangeRequest{
		{Offset: -1, Length: 1},
		{Offset: 0, Length: 0},
		{Offset: 0, Length: -1},
		{Offset: 9223372036854775807, Length: 2},
	} {
		if _, err := ValidateRangeRequest(req); err == nil {
			t.Fatalf("ValidateRangeRequest accepted %+v", req)
		}
	}
	valid, err := ValidateRangeRequest(RangeRequest{Offset: 7, Length: 3})
	if err != nil {
		t.Fatalf("ValidateRangeRequest returned error: %v", err)
	}
	if valid.Offset != 7 || valid.Length != 3 {
		t.Fatalf("range = %+v", valid)
	}
}
