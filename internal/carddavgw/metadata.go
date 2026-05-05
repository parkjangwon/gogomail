package carddavgw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const (
	AddressBookStatusActive  = "active"
	AddressBookStatusDeleted = "deleted"

	MaxAddressBookNameBytes        = 255
	MaxAddressBookDescriptionBytes = 2048
	MaxContactObjectUIDBytes       = 255
	MaxContactObjectNameBytes      = 200
	MaxContactObjectBytes          = 5 << 20
)

func NormalizeAddressBookName(name string) (string, error) {
	name, err := ValidateAddressBookName(name)
	if err != nil {
		return "", err
	}
	return strings.ToLower(name), nil
}

func ValidateAddressBookName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("address book name is required")
	}
	if len(name) > MaxAddressBookNameBytes {
		return "", fmt.Errorf("address book name is too long")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("address book name must not contain line breaks")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("address book name must not contain control characters")
		}
	}
	return name, nil
}

func ValidateAddressBookDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len(description) > MaxAddressBookDescriptionBytes {
		return "", fmt.Errorf("address book description is too long")
	}
	if strings.ContainsAny(description, "\r\n") {
		return "", fmt.Errorf("address book description must not contain line breaks")
	}
	for _, r := range description {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("address book description must not contain control characters")
		}
	}
	return description, nil
}

func ValidateAddressBookStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return AddressBookStatusActive, nil
	}
	switch status {
	case AddressBookStatusActive, AddressBookStatusDeleted:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported address book status %q", status)
	}
}

func ValidateContactObjectUID(uid string) (string, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "", fmt.Errorf("contact object uid is required")
	}
	if len(uid) > MaxContactObjectUIDBytes {
		return "", fmt.Errorf("contact object uid is too long")
	}
	if strings.ContainsAny(uid, "\r\n") {
		return "", fmt.Errorf("contact object uid must not contain line breaks")
	}
	for _, r := range uid {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("contact object uid must not contain control characters")
		}
	}
	return uid, nil
}

func ValidateContactObjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("contact object name is required")
	}
	if len(name) > MaxContactObjectNameBytes {
		return "", fmt.Errorf("contact object name is too long")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("contact object name must not contain line breaks")
	}
	if strings.Contains(name, "/") || strings.Contains(name, "\\") {
		return "", fmt.Errorf("contact object name must not contain path separators")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".vcf") {
		return "", fmt.Errorf("contact object name must end with .vcf")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("contact object name must not contain control characters")
		}
	}
	return name, nil
}

func ContactObjectETag(vcard []byte) (string, error) {
	if len(vcard) == 0 {
		return "", fmt.Errorf("contact object body is required")
	}
	if len(vcard) > MaxContactObjectBytes {
		return "", fmt.Errorf("contact object body is too large")
	}
	sum := sha256.Sum256(vcard)
	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func ValidateContactObjectETag(etag string) (string, error) {
	etag = strings.TrimSpace(etag)
	if len(etag) != 66 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		return "", fmt.Errorf("contact object etag must be a quoted sha256 value")
	}
	for _, r := range etag[1 : len(etag)-1] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", fmt.Errorf("contact object etag must be lowercase hex")
		}
	}
	return etag, nil
}

func AddressBookSyncToken(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "sync-" + hex.EncodeToString(sum[:])[:32]
}
