package carddavgw

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"
)

const (
	WellKnownCardDAVPath = "/.well-known/carddav"
	RootPath             = "/carddav"
	PrincipalsPrefix     = "/carddav/principals"
	AddressBooksPrefix   = "/carddav/addressbooks"

	maxSegmentBytes = 200
)

func PrincipalPath(userID string) (string, error) {
	userID, err := validateSegment("user_id", userID)
	if err != nil {
		return "", err
	}
	return PrincipalsPrefix + "/" + url.PathEscape(userID) + "/", nil
}

func AddressBookHomePath(userID string) (string, error) {
	userID, err := validateSegment("user_id", userID)
	if err != nil {
		return "", err
	}
	return AddressBooksPrefix + "/" + url.PathEscape(userID) + "/", nil
}

func AddressBookCollectionPath(userID string, addressBookID string) (string, error) {
	home, err := AddressBookHomePath(userID)
	if err != nil {
		return "", err
	}
	addressBookID, err = validateSegment("addressbook_id", addressBookID)
	if err != nil {
		return "", err
	}
	return home + url.PathEscape(addressBookID) + "/", nil
}

func ContactObjectPath(userID string, addressBookID string, objectName string) (string, error) {
	collection, err := AddressBookCollectionPath(userID, addressBookID)
	if err != nil {
		return "", err
	}
	objectName, err = validateVCardObjectName(objectName)
	if err != nil {
		return "", err
	}
	return collection + url.PathEscape(objectName), nil
}

func ParseResourcePath(raw string) (ResourcePath, error) {
	if raw == "" {
		return ResourcePath{}, fmt.Errorf("carddav path is required")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return ResourcePath{}, fmt.Errorf("carddav path must not contain line breaks")
	}
	parsed, err := url.PathUnescape(raw)
	if err != nil {
		return ResourcePath{}, fmt.Errorf("decode carddav path: %w", err)
	}
	if parsed != "/" && strings.HasSuffix(parsed, "/") {
		parsed = strings.TrimRight(parsed, "/") + "/"
	}
	cleaned := path.Clean(parsed)
	if strings.HasSuffix(parsed, "/") && cleaned != "/" {
		cleaned += "/"
	}
	if cleaned != parsed {
		return ResourcePath{}, fmt.Errorf("carddav path must be canonical")
	}
	if parsed == WellKnownCardDAVPath {
		return ResourcePath{Kind: ResourceWellKnown}, nil
	}
	if parsed == RootPath || parsed == RootPath+"/" {
		return ResourcePath{Kind: ResourceRoot}, nil
	}
	segments := splitPathSegments(parsed)
	if len(segments) == 0 || segments[0] != "carddav" {
		return ResourcePath{}, fmt.Errorf("unsupported carddav path")
	}
	if len(segments) == 2 && segments[1] == "principals" {
		return ResourcePath{Kind: ResourcePrincipalCollection}, nil
	}
	if len(segments) == 3 && segments[1] == "principals" {
		userID, err := validateSegment("user_id", segments[2])
		if err != nil {
			return ResourcePath{}, err
		}
		return ResourcePath{Kind: ResourcePrincipal, UserID: userID}, nil
	}
	if len(segments) >= 3 && segments[1] == "addressbooks" {
		userID, err := validateSegment("user_id", segments[2])
		if err != nil {
			return ResourcePath{}, err
		}
		if len(segments) == 3 {
			return ResourcePath{Kind: ResourceAddressBookHome, UserID: userID}, nil
		}
		addressBookID, err := validateSegment("addressbook_id", segments[3])
		if err != nil {
			return ResourcePath{}, err
		}
		if len(segments) == 4 {
			return ResourcePath{Kind: ResourceAddressBookCollection, UserID: userID, AddressBookID: addressBookID}, nil
		}
		if len(segments) == 5 {
			objectName, err := validateVCardObjectName(segments[4])
			if err != nil {
				return ResourcePath{}, err
			}
			return ResourcePath{Kind: ResourceContactObject, UserID: userID, AddressBookID: addressBookID, ObjectName: objectName}, nil
		}
	}
	return ResourcePath{}, fmt.Errorf("unsupported carddav path")
}

func ParseResourceHref(raw string) (ResourcePath, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ResourcePath{}, fmt.Errorf("carddav href is required")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return ResourcePath{}, fmt.Errorf("carddav href must not contain line breaks")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return ResourcePath{}, fmt.Errorf("decode carddav href: %w", err)
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "http" && parsed.Scheme != "https" {
			return ResourcePath{}, fmt.Errorf("carddav href absolute URI scheme must be http or https")
		}
		if parsed.Host == "" || parsed.User != nil {
			return ResourcePath{}, fmt.Errorf("carddav href absolute URI authority is not supported")
		}
		if parsed.RawQuery != "" || parsed.Fragment != "" || parsed.Opaque != "" {
			return ResourcePath{}, fmt.Errorf("carddav href absolute URI must not contain query, fragment, or opaque data")
		}
		return ParseResourcePath(parsed.EscapedPath())
	}
	if strings.Contains(raw, "://") {
		return ResourcePath{}, fmt.Errorf("carddav href must be a path or HTTP(S) absolute URI")
	}
	return ParseResourcePath(raw)
}

func validateVCardObjectName(name string) (string, error) {
	name, err := validateSegment("object_name", name)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(name), ".vcf") {
		return "", fmt.Errorf("carddav contact object name must end with .vcf")
	}
	return name, nil
}

func validateSegment(field string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if len(value) > maxSegmentBytes {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	if strings.Contains(value, "/") || strings.Contains(value, "\\") {
		return "", fmt.Errorf("%s must not contain path separators", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%s must not contain control characters", field)
		}
	}
	return value, nil
}

func splitPathSegments(raw string) []string {
	raw = strings.Trim(raw, "/")
	if raw == "" {
		return nil
	}
	return strings.Split(raw, "/")
}
