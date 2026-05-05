package carddavgw

import (
	"strings"
	"testing"
)

func TestValidateAddressBookName(t *testing.T) {
	t.Parallel()

	got, err := ValidateAddressBookName(" Personal ")
	if err != nil {
		t.Fatalf("ValidateAddressBookName returned error: %v", err)
	}
	if got != "Personal" {
		t.Fatalf("name = %q", got)
	}
	normalized, err := NormalizeAddressBookName(" Personal ")
	if err != nil {
		t.Fatalf("NormalizeAddressBookName returned error: %v", err)
	}
	if normalized != "personal" {
		t.Fatalf("normalized = %q", normalized)
	}
}

func TestValidateAddressBookNameRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{"", "bad\nname", strings.Repeat("x", MaxAddressBookNameBytes+1)}
	for _, name := range tests {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateAddressBookName(name); err == nil {
				t.Fatalf("ValidateAddressBookName(%q) error = nil, want rejection", name)
			}
		})
	}
}

func TestValidateContactObjectName(t *testing.T) {
	t.Parallel()

	got, err := ValidateContactObjectName(" contact-1.vcf ")
	if err != nil {
		t.Fatalf("ValidateContactObjectName returned error: %v", err)
	}
	if got != "contact-1.vcf" {
		t.Fatalf("object name = %q", got)
	}
}

func TestValidateContactObjectNameRejectsUnsafeInput(t *testing.T) {
	t.Parallel()

	tests := []string{"", "contact.txt", "folder/contact.vcf", "contact\n1.vcf", strings.Repeat("x", MaxContactObjectNameBytes) + ".vcf"}
	for _, name := range tests {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateContactObjectName(name); err == nil {
				t.Fatalf("ValidateContactObjectName(%q) error = nil, want rejection", name)
			}
		})
	}
}

func TestContactObjectETag(t *testing.T) {
	t.Parallel()

	etag, err := ContactObjectETag([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatalf("ContactObjectETag returned error: %v", err)
	}
	if _, err := ValidateContactObjectETag(etag); err != nil {
		t.Fatalf("ValidateContactObjectETag returned error: %v", err)
	}
}

func TestContactObjectETagRejectsOversizedBody(t *testing.T) {
	t.Parallel()

	if _, err := ContactObjectETag(make([]byte, MaxContactObjectBytes+1)); err == nil {
		t.Fatal("ContactObjectETag accepted oversized body")
	}
}

func TestAddressBookSyncToken(t *testing.T) {
	t.Parallel()

	token := AddressBookSyncToken("user-1", "book-1", "object-1")
	if !strings.HasPrefix(token, "sync-") || len(token) != len("sync-")+32 {
		t.Fatalf("sync token = %q", token)
	}
	if token == AddressBookSyncToken("user-1", "book-1", "object-2") {
		t.Fatal("sync token did not change for distinct inputs")
	}
}
