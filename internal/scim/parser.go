package scim

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// emailRE is a simple RFC 5322-compatible email pattern.
var emailRE = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// ParseBool parses a boolean value from a raw JSON token.
// Accepts:
//   - JSON boolean true/false
//   - string "true"/"false" (case-insensitive)
//   - JSON null → false
//
// Returns an error for any other value.
func ParseBool(raw json.RawMessage) (bool, error) {
	if raw == nil || string(raw) == "null" {
		return false, nil
	}

	// Try JSON boolean first.
	var b bool
	if err := json.Unmarshal(raw, &b); err == nil {
		return b, nil
	}

	// Try quoted string.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch strings.ToLower(s) {
		case "true":
			return true, nil
		case "false":
			return false, nil
		default:
			return false, fmt.Errorf("scim: cannot parse bool from string %q", s)
		}
	}

	return false, fmt.Errorf("scim: cannot parse bool from %s", string(raw))
}

// ValidateUserResource validates a UserResource for required fields and
// well-formed values per RFC 7644.
//
// Rules:
//   - userName must be non-empty (null / empty string rejected)
//   - Each email value, if present, must be a valid email address
func ValidateUserResource(u *UserResource) error {
	if strings.TrimSpace(u.UserName) == "" {
		return fmt.Errorf("scim: userName is required and must not be empty")
	}
	for i, e := range u.Emails {
		if e.Value != "" && !emailRE.MatchString(e.Value) {
			return fmt.Errorf("scim: emails[%d].value %q is not a valid email address", i, e.Value)
		}
	}
	return nil
}

// AttributeMatches reports whether two SCIM attribute names are equal
// using RFC 7644 case-insensitive comparison.
func AttributeMatches(a, b string) bool {
	return strings.EqualFold(a, b)
}
