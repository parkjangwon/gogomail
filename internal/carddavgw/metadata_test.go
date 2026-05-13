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

func TestValidateVCardObject(t *testing.T) {
	t.Parallel()

	meta, err := ValidateVCardObject([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatalf("ValidateVCardObject returned error: %v", err)
	}
	if meta.UID != "contact-1" || meta.Version != "4.0" || meta.FN != "Contact One" {
		t.Fatalf("metadata = %+v", meta)
	}
}

func TestValidateVCardObjectAcceptsVCard30(t *testing.T) {
	t.Parallel()

	meta, err := ValidateVCardObject([]byte("BEGIN:VCARD\r\nVERSION:3.0\r\nUID:contact-1\r\nFN:Contact One\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatalf("ValidateVCardObject returned error: %v", err)
	}
	if meta.UID != "contact-1" || meta.Version != "3.0" || meta.FN != "Contact One" {
		t.Fatalf("metadata = %+v", meta)
	}
}

func TestValidateVCardObjectAcceptsFoldedFN(t *testing.T) {
	t.Parallel()

	meta, err := ValidateVCardObject([]byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact\r\n  One\r\nEND:VCARD\r\n"))
	if err != nil {
		t.Fatalf("ValidateVCardObject returned error: %v", err)
	}
	if meta.FN != "Contact One" {
		t.Fatalf("FN = %q", meta.FN)
	}
}

func TestValidateVCardObjectAcceptsColonInQuotedParameter(t *testing.T) {
	t.Parallel()

	body := "BEGIN:VCARD\r\nVERSION:3.0\r\nUID:contact-1\r\nFN:Contact One\r\nADR;LABEL=\"Office: HQ\":;;1 Example St;;;12345;KR\r\nEND:VCARD\r\n"
	meta, err := ValidateVCardObject([]byte(body))
	if err != nil {
		t.Fatalf("ValidateVCardObject returned error: %v", err)
	}
	if meta.Version != "3.0" || meta.UID != "contact-1" {
		t.Fatalf("metadata = %+v", meta)
	}
}

func TestValidateVCardObjectRejectsMalformedCards(t *testing.T) {
	t.Parallel()

	tests := []string{
		"",
		"VERSION:4.0\r\nUID:contact-1\r\nFN:Contact\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:2.1\r\nUID:contact-1\r\nFN:Contact\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nFN:Contact\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:bad\nuid\r\nFN:Contact\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\nVERSION:4.0\nUID:contact-1\nFN:Contact\nEND:VCARD\n",
		"BEGIN:VCARD\r\nVERSION:4.0\nUID:contact-1\r\nFN:Contact\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact\r\nBEGIN:VCARD\r\nEND:VCARD\r\nEND:VCARD\r\n",
		"BEGIN:VCARD\r\nVERSION:4.0\r\nUID:contact-1\r\nFN:Contact\r\nNOTE\rbad\r\nEND:VCARD\r\n",
	}
	for _, body := range tests {
		body := body
		t.Run(strings.ReplaceAll(body, "\r\n", "|"), func(t *testing.T) {
			t.Parallel()

			if _, err := ValidateVCardObject([]byte(body)); err == nil {
				t.Fatalf("ValidateVCardObject(%q) error = nil, want rejection", body)
			}
		})
	}
}

func TestParseVCardContentLinePartsCollectsParameters(t *testing.T) {
	t.Parallel()

	line, err := parseVCardContentLineParts(`item1.EMAIL;TYPE=home,voice;PREF=1;LABEL="Desk, Main":person@example.com`)
	if err != nil {
		t.Fatalf("parseVCardContentLineParts returned error: %v", err)
	}
	if line.Name != "EMAIL" || line.Value != "person@example.com" {
		t.Fatalf("line = %+v", line)
	}
	if got := line.Params["TYPE"]; len(got) != 2 || got[0] != "home" || got[1] != "voice" {
		t.Fatalf("TYPE params = %+v", got)
	}
	if got := line.Params["LABEL"]; len(got) != 1 || got[0] != "Desk, Main" {
		t.Fatalf("LABEL params = %+v", got)
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

func TestAddressBookCollectionETag(t *testing.T) {
	t.Parallel()

	etag, err := AddressBookCollectionETag("user-1", AddressBook{ID: "book-1", SyncToken: "sync-123"})
	if err != nil {
		t.Fatalf("AddressBookCollectionETag returned error: %v", err)
	}
	if _, err := ValidateContactObjectETag(etag); err != nil {
		t.Fatalf("collection etag is not a strong quoted hash: %v", err)
	}
	changed, err := AddressBookCollectionETag("user-1", AddressBook{ID: "book-1", SyncToken: "sync-456"})
	if err != nil {
		t.Fatalf("AddressBookCollectionETag returned error: %v", err)
	}
	if etag == changed {
		t.Fatal("collection etag did not change with sync token")
	}
	if _, err := AddressBookCollectionETag("", AddressBook{ID: "book-1", SyncToken: "sync-123"}); err == nil {
		t.Fatal("AddressBookCollectionETag accepted missing user")
	}
}
