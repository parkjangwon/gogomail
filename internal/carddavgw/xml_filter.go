package carddavgw

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

func parseAddressBookFilter(dec *xml.Decoder, el xml.StartElement) (AddressBookQueryFilter, error) {
	test, err := filterTestAttribute(el, "filter")
	if err != nil {
		return AddressBookQueryFilter{}, err
	}
	filter := AddressBookQueryFilter{Test: test}
	return parseAddressBookFilterChildren(dec, el.Name, 1, filter)
}

func parseAddressBookFilterChildren(dec *xml.Decoder, filterName xml.Name, depth int, filter AddressBookQueryFilter) (AddressBookQueryFilter, error) {
	if depth > MaxWebDAVXMLDepth {
		return AddressBookQueryFilter{}, fmt.Errorf("WebDAV XML exceeds maximum depth")
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return AddressBookQueryFilter{}, fmt.Errorf("unterminated filter element")
		}
		if err != nil {
			return AddressBookQueryFilter{}, fmt.Errorf("decode filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CardDAVNamespace, "prop-filter"):
				propFilter, err := parsePropFilterElement(dec, tok, depth+1)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				filter.PropFilters = append(filter.PropFilters, propFilter)
				if len(filter.PropFilters) > MaxWebDAVProperties {
					return AddressBookQueryFilter{}, fmt.Errorf("too many CardDAV prop-filter elements")
				}
			case tok.Name.Space == CardDAVNamespace:
				return AddressBookQueryFilter{}, UnsupportedFilterElementError{
					Name: XMLName{Space: tok.Name.Space, Local: tok.Name.Local},
				}
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return AddressBookQueryFilter{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, filterName) {
				return filter, nil
			}
		}
	}
}

func parsePropFilterElement(dec *xml.Decoder, el xml.StartElement, depth int) (CardDAVPropFilter, error) {
	if depth > MaxWebDAVXMLDepth {
		return CardDAVPropFilter{}, fmt.Errorf("WebDAV XML exceeds maximum depth")
	}
	name, err := propFilterName(el)
	if err != nil {
		return CardDAVPropFilter{}, err
	}
	test, err := filterTestAttribute(el, "prop-filter")
	if err != nil {
		return CardDAVPropFilter{}, err
	}
	filter := CardDAVPropFilter{Name: name, Test: test}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return CardDAVPropFilter{}, fmt.Errorf("unterminated prop-filter element")
		}
		if err != nil {
			return CardDAVPropFilter{}, fmt.Errorf("decode prop-filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CardDAVNamespace, "text-match"):
				if filter.IsNotDefined {
					return CardDAVPropFilter{}, fmt.Errorf("CardDAV prop-filter cannot mix is-not-defined and match conditions")
				}
				if len(filter.TextMatches) > 0 {
					return CardDAVPropFilter{}, fmt.Errorf("CardDAV prop-filter must not contain multiple text-match elements")
				}
				match, err := parseTextMatchElement(dec, tok)
				if err != nil {
					return CardDAVPropFilter{}, err
				}
				filter.TextMatches = append(filter.TextMatches, match)
			case sameXMLName(tok.Name, CardDAVNamespace, "param-filter"):
				if filter.IsNotDefined {
					return CardDAVPropFilter{}, fmt.Errorf("CardDAV prop-filter cannot mix is-not-defined and match conditions")
				}
				paramFilter, err := parseParamFilterElement(dec, tok)
				if err != nil {
					return CardDAVPropFilter{}, err
				}
				filter.ParamFilters = append(filter.ParamFilters, paramFilter)
			case sameXMLName(tok.Name, CardDAVNamespace, "is-not-defined"):
				if len(filter.TextMatches) > 0 || len(filter.ParamFilters) > 0 {
					return CardDAVPropFilter{}, fmt.Errorf("CardDAV prop-filter cannot mix is-not-defined and match conditions")
				}
				filter.IsNotDefined = true
				if err := skipElement(dec, tok.Name); err != nil {
					return CardDAVPropFilter{}, err
				}
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return CardDAVPropFilter{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, el.Name) {
				return filter, nil
			}
		}
	}
}

func propFilterName(el xml.StartElement) (string, error) {
	return filterNameAttribute(el, "prop-filter")
}

func parseParamFilterElement(dec *xml.Decoder, el xml.StartElement) (CardDAVParamFilter, error) {
	name, err := filterNameAttribute(el, "param-filter")
	if err != nil {
		return CardDAVParamFilter{}, err
	}
	filter := CardDAVParamFilter{Name: name}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return CardDAVParamFilter{}, fmt.Errorf("unterminated param-filter element")
		}
		if err != nil {
			return CardDAVParamFilter{}, fmt.Errorf("decode param-filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CardDAVNamespace, "text-match"):
				match, err := parseTextMatchElement(dec, tok)
				if err != nil {
					return CardDAVParamFilter{}, err
				}
				if filter.IsNotDefined {
					return CardDAVParamFilter{}, fmt.Errorf("CardDAV param-filter cannot mix is-not-defined and text-match")
				}
				if filter.HasTextMatch {
					return CardDAVParamFilter{}, fmt.Errorf("CardDAV param-filter must not contain multiple text-match elements")
				}
				filter.TextMatch = match
				filter.HasTextMatch = true
			case sameXMLName(tok.Name, CardDAVNamespace, "is-not-defined"):
				if filter.HasTextMatch {
					return CardDAVParamFilter{}, fmt.Errorf("CardDAV param-filter cannot mix is-not-defined and text-match")
				}
				filter.IsNotDefined = true
				if err := skipElement(dec, tok.Name); err != nil {
					return CardDAVParamFilter{}, err
				}
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return CardDAVParamFilter{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, el.Name) {
				return filter, nil
			}
		}
	}
}

func filterTestAttribute(el xml.StartElement, element string) (string, error) {
	test := FilterTestAnyOf
	for _, attr := range el.Attr {
		if attr.Name.Local != "test" {
			continue
		}
		if strings.ContainsAny(attr.Value, "\r\n") {
			return "", fmt.Errorf("CardDAV %s test attribute is invalid", element)
		}
		test = strings.ToLower(strings.TrimSpace(attr.Value))
		switch test {
		case FilterTestAnyOf, FilterTestAllOf:
		default:
			return "", fmt.Errorf("unsupported CardDAV %s test %q", element, test)
		}
	}
	return test, nil
}

func parseTextMatchElement(dec *xml.Decoder, el xml.StartElement) (CardDAVTextMatch, error) {
	match := CardDAVTextMatch{MatchType: TextMatchContains, Collation: TextMatchUnicodeCasemap}
	for _, attr := range el.Attr {
		if strings.ContainsAny(attr.Value, "\r\n") {
			return CardDAVTextMatch{}, fmt.Errorf("CardDAV text-match attribute is invalid")
		}
		switch attr.Name.Local {
		case "collation":
			collation := strings.ToLower(strings.TrimSpace(attr.Value))
			if collation == "" || len(collation) > 128 {
				return CardDAVTextMatch{}, fmt.Errorf("CardDAV text-match collation is invalid")
			}
			if !isSupportedTextMatchCollation(collation) {
				return CardDAVTextMatch{}, UnsupportedCollationError{Value: collation}
			}
			match.Collation = collation
		case "match-type":
			matchType := strings.ToLower(strings.TrimSpace(attr.Value))
			switch matchType {
			case TextMatchEquals, TextMatchContains, TextMatchStartsWith, TextMatchEndsWith:
				match.MatchType = matchType
			default:
				return CardDAVTextMatch{}, fmt.Errorf("unsupported CardDAV text-match match-type %q", matchType)
			}
		case "negate-condition":
			negate := strings.ToLower(strings.TrimSpace(attr.Value))
			switch negate {
			case "yes":
				match.Negate = true
			case "no":
				match.Negate = false
			default:
				return CardDAVTextMatch{}, fmt.Errorf("CardDAV text-match negate-condition is invalid")
			}
		}
	}
	text, err := readSimpleElementText(dec, el.Name)
	if err != nil {
		return CardDAVTextMatch{}, err
	}
	text = strings.TrimSpace(text)
	if len(text) > 512 || strings.ContainsAny(text, "\r\n") {
		return CardDAVTextMatch{}, fmt.Errorf("CardDAV text-match is invalid")
	}
	match.Text = text
	return match, nil
}

func filterNameAttribute(el xml.StartElement, element string) (string, error) {
	for _, attr := range el.Attr {
		if attr.Name.Local != "name" {
			continue
		}
		name := strings.ToUpper(strings.TrimSpace(attr.Value))
		if name == "" {
			return "", fmt.Errorf("CardDAV %s name is required", element)
		}
		if len(name) > 64 || strings.ContainsAny(name, "\r\n") {
			return "", fmt.Errorf("CardDAV %s name is invalid", element)
		}
		for _, r := range name {
			if !((r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-') {
				return "", fmt.Errorf("CardDAV %s name is invalid", element)
			}
		}
		return name, nil
	}
	return "", fmt.Errorf("CardDAV %s name is required", element)
}
