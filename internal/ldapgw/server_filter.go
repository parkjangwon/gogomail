package ldapgw

import (
	"fmt"
	"strings"
)

func ldapAttributeValueMatchesFilter(attrName, attrValue string, filter []byte) bool {
	if len(filter) == 0 || filter[0]&0x80 == 0 {
		return false
	}
	filterType := int(filter[0] & 0x1f)
	content, err := decodeContent(filter[1:])
	if err != nil {
		return false
	}
	switch filterType {
	case filterAnd:
		matchedAny := false
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if !ldapAttributeValueMatchesFilter(attrName, attrValue, child) {
				return false
			}
			matchedAny = true
			content = rest
		}
		return matchedAny
	case filterOr:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if ldapAttributeValueMatchesFilter(attrName, attrValue, child) {
				return true
			}
			content = rest
		}
		return false
	case filterNot:
		child, _, err := readRawTLV(content)
		return err == nil && !ldapAttributeValueMatchesFilter(attrName, attrValue, child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valRest, err := decodeOctetString(content)
		if err != nil || !strings.EqualFold(ldapAttributeType(attr), ldapAttributeType(attrName)) {
			return false
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return false
		}
		return ldapAttributeValueEqual(attrName, attrValue, val)
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil || !strings.EqualFold(ldapAttributeType(attr), ldapAttributeType(attrName)) {
			return false
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return false
		}
		return ldapSubstringMatches(attrValue, parts)
	case filterPresent:
		return strings.EqualFold(ldapAttributeType(string(content)), ldapAttributeType(attrName))
	case filterExtensible:
		attr, value, ok, err := decodeExtensibleMatch(content)
		return err == nil && ok && strings.EqualFold(ldapAttributeType(attr), ldapAttributeType(attrName)) && ldapAttributeValueEqual(attrName, attrValue, value)
	default:
		return false
	}
}

func ldapEntryAttributesMatchFilter(attrs map[string][]string, filter []byte) bool {
	if len(filter) == 0 || filter[0]&0x80 == 0 {
		return false
	}
	filterType := int(filter[0] & 0x1f)
	content, err := decodeContent(filter[1:])
	if err != nil {
		return false
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if !ldapEntryAttributesMatchFilter(attrs, child) {
				return false
			}
			content = rest
		}
		return true
	case filterOr:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return false
			}
			if ldapEntryAttributesMatchFilter(attrs, child) {
				return true
			}
			content = rest
		}
		return false
	case filterNot:
		child, _, err := readRawTLV(content)
		return err == nil && !ldapEntryAttributesMatchFilter(attrs, child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return false
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, attr, func(candidate string) bool {
			switch filterType {
			case filterGreaterOrEqual:
				return strings.ToLower(candidate) >= strings.ToLower(val)
			case filterLessOrEqual:
				return strings.ToLower(candidate) <= strings.ToLower(val)
			default:
				return ldapAttributeValueEqual(attr, candidate, val)
			}
		})
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil {
			return false
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, attr, func(candidate string) bool {
			return ldapSubstringMatches(candidate, parts)
		})
	case filterPresent:
		_, ok := ldapEntryAttributeValues(attrs, string(content))
		return ok
	case filterExtensible:
		match, ok, err := decodeExtensibleMatchDetail(content)
		if err != nil || !ok {
			return false
		}
		return ldapEntryAttributeHasValue(attrs, match.Attr, func(candidate string) bool {
			return ldapExtensibleValueMatches(candidate, match)
		})
	default:
		return false
	}
}

func ldapEntryAttributeHasValue(attrs map[string][]string, attr string, match func(string) bool) bool {
	values, ok := ldapEntryAttributeValues(attrs, attr)
	if !ok {
		return false
	}
	for _, value := range values {
		if match(value) {
			return true
		}
	}
	return false
}

func ldapEntryAttributeValues(attrs map[string][]string, attr string) ([]string, bool) {
	want := ldapAttributeType(attr)
	for name, values := range attrs {
		if strings.EqualFold(ldapAttributeType(name), want) {
			return values, true
		}
	}
	return nil, false
}

func ldapAttributeValueEqual(attr, candidate, assertion string) bool {
	if ldapAttributeUsesBinaryMatch(attr) {
		return candidate == assertion
	}
	return strings.EqualFold(candidate, assertion)
}

func ldapAttributeUsesBinaryMatch(attr string) bool {
	switch strings.ToLower(ldapAttributeType(attr)) {
	case "objectguid", "objectsid":
		return true
	default:
		return false
	}
}

func ldapAttributeType(attr string) string {
	attr = strings.TrimSpace(attr)
	if idx := strings.IndexByte(attr, ';'); idx >= 0 {
		attr = attr[:idx]
	}
	return strings.TrimSpace(attr)
}

func ldapExtensibleValueMatches(candidate string, match extensibleMatchAssertion) bool {
	switch strings.TrimSpace(match.MatchingRule) {
	case "":
		return ldapAttributeValueEqual(match.Attr, candidate, match.Value)
	case "1.2.840.113556.1.4.803": // LDAP_MATCHING_RULE_BIT_AND
		candidateInt, ok1 := parseLDAPInt64(candidate)
		assertionInt, ok2 := parseLDAPInt64(match.Value)
		return ok1 && ok2 && candidateInt&assertionInt == assertionInt
	case "1.2.840.113556.1.4.804": // LDAP_MATCHING_RULE_BIT_OR
		candidateInt, ok1 := parseLDAPInt64(candidate)
		assertionInt, ok2 := parseLDAPInt64(match.Value)
		return ok1 && ok2 && candidateInt&assertionInt != 0
	default:
		return ldapAttributeValueEqual(match.Attr, candidate, match.Value)
	}
}

func parseLDAPInt64(value string) (int64, bool) {
	var n int64
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
		n = n*10 + int64(r-'0')
	}
	return n, true
}

func ldapSubstringMatches(value string, parts []string) bool {
	if len(parts) == 0 {
		return false
	}
	value = strings.ToLower(value)
	pos := 0
	for _, part := range parts {
		part = strings.ToLower(part)
		idx := strings.Index(value[pos:], part)
		if idx < 0 {
			return false
		}
		pos += idx + len(part)
	}
	return true
}

func parseLDAPFilter(data []byte) (string, error) {
	attr, value, ok, err := parseLDAPFilterCandidate(data)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", nil
	}
	return fmt.Sprintf("(%s=%s)", attr, value), nil
}

func parseLDAPFilterPrincipalKinds(data []byte) ([]string, error) {
	seen := map[string]struct{}{}
	if err := collectLDAPFilterPrincipalKinds(data, seen); err != nil {
		return nil, err
	}
	if len(seen) == 0 {
		return nil, nil
	}
	order := []string{"user", "organization", "group", "resource"}
	kinds := make([]string, 0, len(seen))
	for _, kind := range order {
		if _, ok := seen[kind]; ok {
			kinds = append(kinds, kind)
		}
	}
	return kinds, nil
}

func collectLDAPFilterPrincipalKinds(data []byte, seen map[string]struct{}) error {
	if len(data) == 0 {
		return nil
	}
	if data[0]&0x80 == 0 {
		return fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	content, err := decodeContent(data[1:])
	if err != nil {
		return err
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return err
			}
			if err := collectLDAPFilterPrincipalKinds(child, seen); err != nil {
				return err
			}
			content = rest
		}
	case filterOr:
		for len(content) > 0 {
			_, rest, err := readRawTLV(content)
			if err != nil {
				return err
			}
			content = rest
		}
		return nil
	case filterNot:
		if _, _, err := readRawTLV(content); err != nil {
			return err
		}
		return nil
	case filterEqualityMatch, filterApproxMatch:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return err
		}
		value, _, err := decodeOctetString(valRest)
		if err != nil {
			return err
		}
		if principalKindAttr := ldapAttributeType(attr); strings.EqualFold(principalKindAttr, "objectClass") || strings.EqualFold(principalKindAttr, "objectCategory") {
			for _, kind := range principalKindsForObjectClass(value) {
				seen[kind] = struct{}{}
			}
		}
	case filterExtensible:
		attr, value, ok, err := decodeExtensibleMatch(content)
		if err != nil {
			return err
		}
		if principalKindAttr := ldapAttributeType(attr); ok && (strings.EqualFold(principalKindAttr, "objectClass") || strings.EqualFold(principalKindAttr, "objectCategory")) {
			for _, kind := range principalKindsForObjectClass(value) {
				seen[kind] = struct{}{}
			}
		}
	case filterPresent, filterSubstrings, filterGreaterOrEqual, filterLessOrEqual:
		return nil
	default:
		return fmt.Errorf("unsupported filter type: %d", filterType)
	}
	return nil
}

func principalKindsForObjectClass(value string) []string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "person", "organizationalperson", "inetorgperson", "user":
		return []string{"user"}
	case "organizationalunit", "organizational-unit", "organization":
		return []string{"organization"}
	case "group", "groupofnames", "groupofuniquenames", "posixgroup":
		return []string{"group"}
	case "device", "resource":
		return []string{"resource"}
	default:
		return nil
	}
}

func parseLDAPFilterCandidate(data []byte) (attr string, value string, ok bool, err error) {
	if len(data) == 0 {
		return "", "", false, nil
	}
	if data[0]&0x80 == 0 {
		return "", "", false, fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	content, err := decodeContent(data[1:])
	if err != nil {
		return "", "", false, err
	}
	switch filterType {
	case filterAnd:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return "", "", false, err
			}
			childAttr, childValue, childOK, err := parseLDAPFilterCandidate(child)
			if err != nil {
				return "", "", false, err
			}
			if childOK && isDirectorySearchAttribute(childAttr) && strings.Trim(childValue, "*") != "" {
				return childAttr, childValue, true, nil
			}
			content = rest
		}
		return "", "", false, nil
	case filterOr:
		for len(content) > 0 {
			_, rest, err := readRawTLV(content)
			if err != nil {
				return "", "", false, err
			}
			content = rest
		}
		return "", "", false, nil
	case filterNot:
		if _, _, err := readRawTLV(content); err != nil {
			return "", "", false, err
		}
		return "", "", false, nil
	case filterEqualityMatch, filterApproxMatch:
		attr, valRest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed attribute assertion: %w", err)
		}
		val, _, err := decodeOctetString(valRest)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed assertion value: %w", err)
		}
		return ldapAttributeType(attr), val, true, nil
	case filterGreaterOrEqual, filterLessOrEqual:
		_, valRest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed attribute assertion: %w", err)
		}
		if _, _, err := decodeOctetString(valRest); err != nil {
			return "", "", false, fmt.Errorf("malformed assertion value: %w", err)
		}
		return "", "", false, nil
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil {
			return "", "", false, fmt.Errorf("malformed substring attribute: %w", err)
		}
		parts, err := decodeSubstringParts(rest)
		if err != nil {
			return "", "", false, err
		}
		return ldapAttributeType(attr), strings.Join(parts, "*"), len(parts) > 0, nil
	case filterPresent:
		return ldapAttributeType(string(content)), "*", true, nil
	case filterExtensible:
		attr, value, ok, err := decodeExtensibleMatch(content)
		return ldapAttributeType(attr), value, ok, err
	default:
		return "", "", false, fmt.Errorf("unsupported filter type: %d", filterType)
	}
}

func isDirectorySearchAttribute(attr string) bool {
	switch strings.ToLower(ldapAttributeType(attr)) {
	case "cn", "mail", "uid", "displayname", "givenname", "sn", "ou", "description", "name", "canonicalname", "samaccountname", "userprincipalname", "mailnickname", "proxyaddresses":
		return true
	default:
		return false
	}
}

// validateFilter checks that filter data is well-formed before processing.
// It rejects empty input, non-context-specific tags, unrecognised filter
// types, and content whose declared length exceeds the available bytes.
func validateFilter(data []byte) error {
	if len(data) == 0 {
		return fmt.Errorf("filter is empty")
	}
	if data[0]&0x80 == 0 {
		return fmt.Errorf("filter tag 0x%02x is not context-specific", data[0])
	}
	filterType := int(data[0] & 0x1f)
	switch filterType {
	case filterAnd, filterOr, filterNot,
		filterEqualityMatch, filterSubstrings,
		filterGreaterOrEqual, filterLessOrEqual,
		filterPresent, filterApproxMatch, filterExtensible:
		// valid
	default:
		return fmt.Errorf("unsupported filter type %d", filterType)
	}
	if len(data) < 2 {
		return fmt.Errorf("filter too short")
	}
	declLen, _, err := decodeLength(data[1:])
	if err != nil {
		return fmt.Errorf("filter length: %w", err)
	}
	// Compute how many bytes the tag+length header occupies.
	headerLen := 2 // tag byte + one length byte (short form)
	if data[1]&0x80 != 0 {
		headerLen = 1 + 1 + int(data[1]&0x7f) // tag + 0x8n + n bytes
	}
	if len(data) < headerLen+declLen {
		return fmt.Errorf("filter truncated: need %d bytes after header, have %d", declLen, len(data)-headerLen)
	}
	if len(data) != headerLen+declLen {
		return fmt.Errorf("filter trailing data")
	}
	return validateFilterContent(filterType, data[headerLen:headerLen+declLen])
}

func validateFilterContent(filterType int, content []byte) error {
	switch filterType {
	case filterAnd, filterOr:
		for len(content) > 0 {
			child, rest, err := readRawTLV(content)
			if err != nil {
				return err
			}
			if err := validateFilter(child); err != nil {
				return err
			}
			content = rest
		}
		return nil
	case filterNot:
		child, rest, err := readRawTLV(content)
		if err != nil {
			return err
		}
		if len(rest) != 0 {
			return fmt.Errorf("not filter trailing data")
		}
		return validateFilter(child)
	case filterEqualityMatch, filterApproxMatch, filterGreaterOrEqual, filterLessOrEqual:
		attr, valueRest, err := decodeOctetString(content)
		if err != nil {
			return fmt.Errorf("filter attribute: %w", err)
		}
		if ldapAttributeType(attr) == "" {
			return fmt.Errorf("filter attribute description empty")
		}
		_, trailing, err := decodeOctetString(valueRest)
		if err != nil {
			return fmt.Errorf("filter assertion value: %w", err)
		}
		if len(trailing) != 0 {
			return fmt.Errorf("filter assertion trailing data")
		}
		return nil
	case filterSubstrings:
		attr, rest, err := decodeOctetString(content)
		if err != nil {
			return fmt.Errorf("substring filter attribute: %w", err)
		}
		if ldapAttributeType(attr) == "" {
			return fmt.Errorf("substring filter attribute description empty")
		}
		if _, err := decodeSubstringParts(rest); err != nil {
			return err
		}
		return nil
	case filterPresent:
		if ldapAttributeType(string(content)) == "" {
			return fmt.Errorf("present filter attribute description empty")
		}
		return nil
	case filterExtensible:
		match, ok, err := decodeExtensibleMatchDetail(content)
		if err != nil {
			return err
		}
		if !ok || ldapAttributeType(match.Attr) == "" {
			return fmt.Errorf("extensible filter attribute description empty")
		}
		return nil
	default:
		return fmt.Errorf("unsupported filter type %d", filterType)
	}
}
