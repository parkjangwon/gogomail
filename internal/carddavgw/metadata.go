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
	MaxPhotoBytes                  = 5 << 20
	MaxVCardContentLineBytes       = 8192
	MaxVCardContentLines           = 4096
)

type VCardMetadata struct {
	UID     string
	Version string
	FN      string
}

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

func ValidateAddressBookPathID(id string) (string, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if len(id) != 36 {
		return "", fmt.Errorf("address book path id must be a UUID")
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", fmt.Errorf("address book path id must be a UUID")
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				return "", fmt.Errorf("address book path id must be a lowercase UUID")
			}
		}
	}
	return id, nil
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

func ValidateVCardObject(vcard []byte) (VCardMetadata, error) {
	if len(vcard) == 0 {
		return VCardMetadata{}, fmt.Errorf("vcard body is required")
	}
	if len(vcard) > MaxContactObjectBytes {
		return VCardMetadata{}, fmt.Errorf("vcard body is too large")
	}
	if err := validateVCardLineEndings(string(vcard)); err != nil {
		return VCardMetadata{}, err
	}
	lines, err := unfoldVCardLines(string(vcard))
	if err != nil {
		return VCardMetadata{}, err
	}
	if len(lines) < 4 {
		return VCardMetadata{}, fmt.Errorf("vcard requires BEGIN, VERSION, FN, and END lines")
	}
	if !strings.EqualFold(lines[0], "BEGIN:VCARD") {
		return VCardMetadata{}, fmt.Errorf("vcard must begin with BEGIN:VCARD")
	}
	if !strings.EqualFold(lines[len(lines)-1], "END:VCARD") {
		return VCardMetadata{}, fmt.Errorf("vcard must end with END:VCARD")
	}
	var meta VCardMetadata
	for i, line := range lines[1 : len(lines)-1] {
		name, value, err := parseVCardContentLine(line)
		if err != nil {
			return VCardMetadata{}, fmt.Errorf("vcard line %d: %w", i+2, err)
		}
		switch name {
		case "BEGIN", "END":
			return VCardMetadata{}, fmt.Errorf("nested vcard components are not supported")
		case "VERSION":
			if meta.Version != "" {
				return VCardMetadata{}, fmt.Errorf("vcard must contain exactly one VERSION")
			}
			meta.Version = strings.TrimSpace(value)
		case "UID":
			if meta.UID != "" {
				return VCardMetadata{}, fmt.Errorf("vcard must contain at most one UID")
			}
			uid, err := ValidateContactObjectUID(value)
			if err != nil {
				return VCardMetadata{}, err
			}
			meta.UID = uid
		case "FN":
			if strings.TrimSpace(value) == "" {
				return VCardMetadata{}, fmt.Errorf("vcard FN is required")
			}
			if meta.FN == "" {
				meta.FN = strings.TrimSpace(value)
			}
		}
	}
	if meta.Version != "3.0" && meta.Version != "4.0" {
		return VCardMetadata{}, fmt.Errorf("vcard VERSION must be 3.0 or 4.0")
	}
	if meta.UID == "" {
		return VCardMetadata{}, fmt.Errorf("vcard UID is required")
	}
	if meta.FN == "" {
		return VCardMetadata{}, fmt.Errorf("vcard FN is required")
	}
	return meta, nil
}

func validateVCardLineEndings(raw string) error {
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\n':
			if i == 0 || raw[i-1] != '\r' {
				return fmt.Errorf("vcard line endings must be CRLF")
			}
		case '\r':
			if i+1 >= len(raw) || raw[i+1] != '\n' {
				return fmt.Errorf("vcard line endings must be CRLF")
			}
		}
	}
	return nil
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

func AddressBookCollectionETag(userID string, book AddressBook) (string, error) {
	userID = strings.TrimSpace(userID)
	bookID := strings.TrimSpace(book.ID)
	syncToken := strings.TrimSpace(book.SyncToken)
	if userID == "" || bookID == "" || syncToken == "" {
		return "", fmt.Errorf("address book collection etag requires user, address book, and sync token")
	}
	sum := sha256.Sum256([]byte(AddressBookSyncToken("collection-etag", userID, bookID, syncToken)))
	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func unfoldVCardLines(raw string) ([]string, error) {
	raw = strings.TrimSuffix(raw, "\r\n")
	raw = strings.TrimSuffix(raw, "\n")
	if raw == "" {
		return nil, fmt.Errorf("vcard body is required")
	}
	rawLines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if len(rawLines) > MaxVCardContentLines {
		return nil, fmt.Errorf("vcard contains too many content lines")
	}
	lines := make([]string, 0, len(rawLines))
	for i, line := range rawLines {
		if strings.Contains(line, "\r") {
			return nil, fmt.Errorf("vcard line %d contains bare carriage return", i+1)
		}
		if len(line) > MaxVCardContentLineBytes {
			return nil, fmt.Errorf("vcard line %d is too long", i+1)
		}
		if line == "" {
			continue
		}
		if line[0] == ' ' || line[0] == '\t' {
			if len(lines) == 0 {
				return nil, fmt.Errorf("vcard line %d folds without a previous line", i+1)
			}
			unfolded := lines[len(lines)-1] + line[1:]
			if len(unfolded) > MaxVCardContentLineBytes {
				return nil, fmt.Errorf("vcard unfolded line %d is too long", i+1)
			}
			lines[len(lines)-1] = unfolded
			continue
		}
		lines = append(lines, line)
	}
	if len(lines) > MaxVCardContentLines {
		return nil, fmt.Errorf("vcard contains too many content lines")
	}
	return lines, nil
}

func parseVCardContentLine(line string) (string, string, error) {
	parsed, err := parseVCardContentLineParts(line)
	if err != nil {
		return "", "", err
	}
	return parsed.Name, parsed.Value, nil
}

type vCardContentLine struct {
	Name   string
	Params map[string][]string
	Value  string
}

func parseVCardContentLineParts(line string) (vCardContentLine, error) {
	if line == "" {
		return vCardContentLine{}, fmt.Errorf("content line is empty")
	}
	separator, err := findVCardValueSeparator(line)
	if err != nil {
		return vCardContentLine{}, err
	}
	rawName := line[:separator]
	value := line[separator+1:]
	if strings.ContainsAny(rawName, "\r\n") || strings.ContainsAny(value, "\r\n") {
		return vCardContentLine{}, fmt.Errorf("content line contains line breaks")
	}
	segments, err := splitVCardContentLineName(rawName)
	if err != nil {
		return vCardContentLine{}, err
	}
	if len(segments) == 0 {
		return vCardContentLine{}, fmt.Errorf("property name is required")
	}
	namePart := segments[0]
	if dot := strings.LastIndexByte(namePart, '.'); dot >= 0 {
		namePart = namePart[dot+1:]
	}
	namePart = strings.ToUpper(strings.TrimSpace(namePart))
	namePart, err = normalizeVCardName(namePart, "property")
	if err != nil {
		return vCardContentLine{}, err
	}
	params := make(map[string][]string)
	for _, rawParam := range segments[1:] {
		key, values, ok, err := parseVCardParam(rawParam)
		if err != nil {
			return vCardContentLine{}, err
		}
		if !ok {
			continue
		}
		params[key] = append(params[key], values...)
	}
	return vCardContentLine{Name: namePart, Params: params, Value: value}, nil
}

func findVCardValueSeparator(line string) (int, error) {
	quoted := false
	for i, r := range line {
		switch r {
		case '"':
			quoted = !quoted
		case ':':
			if !quoted {
				if i <= 0 {
					return 0, fmt.Errorf("content line missing property name")
				}
				return i, nil
			}
		}
	}
	if quoted {
		return 0, fmt.Errorf("content line parameter quote is unterminated")
	}
	return 0, fmt.Errorf("content line missing value separator")
}

func normalizeVCardName(value string, label string) (string, error) {
	name := strings.ToUpper(strings.TrimSpace(value))
	if name == "" {
		return "", fmt.Errorf("%s name is required", label)
	}
	if len(name) > 64 || strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("%s name is invalid", label)
	}
	for _, r := range name {
		if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
			return "", fmt.Errorf("%s name is invalid", label)
		}
	}
	return name, nil
}

func splitVCardContentLineName(raw string) ([]string, error) {
	var segments []string
	start := 0
	quoted := false
	for i, r := range raw {
		switch r {
		case '"':
			quoted = !quoted
		case ';':
			if !quoted {
				segments = append(segments, raw[start:i])
				start = i + 1
			}
		}
	}
	if quoted {
		return nil, fmt.Errorf("content line parameter quote is unterminated")
	}
	segments = append(segments, raw[start:])
	return segments, nil
}

func parseVCardParam(raw string) (string, []string, bool, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil, false, nil
	}
	eq := strings.IndexByte(raw, '=')
	if eq <= 0 {
		return "", nil, false, nil
	}
	name, err := normalizeVCardName(raw[:eq], "parameter")
	if err != nil {
		return "", nil, false, err
	}
	values, err := splitVCardParamValues(raw[eq+1:])
	if err != nil {
		return "", nil, false, err
	}
	return name, values, true, nil
}

func splitVCardParamValues(raw string) ([]string, error) {
	var values []string
	start := 0
	quoted := false
	for i, r := range raw {
		switch r {
		case '"':
			quoted = !quoted
		case ',':
			if !quoted {
				values = append(values, cleanVCardParamValue(raw[start:i]))
				start = i + 1
			}
		}
	}
	if quoted {
		return nil, fmt.Errorf("parameter value quote is unterminated")
	}
	values = append(values, cleanVCardParamValue(raw[start:]))
	return values, nil
}

func cleanVCardParamValue(raw string) string {
	return strings.Trim(strings.TrimSpace(raw), `"`)
}
