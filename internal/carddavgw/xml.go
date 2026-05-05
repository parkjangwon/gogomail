package carddavgw

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	DAVNamespace     = "DAV:"
	CardDAVNamespace = "urn:ietf:params:xml:ns:carddav"

	MaxWebDAVXMLBodyBytes = 1 << 20
	MaxWebDAVXMLDepth     = 64
	MaxWebDAVProperties   = 256
	MaxWebDAVHrefs        = 2048
	MaxWebDAVReportLimit  = 1000
)

type XMLName struct {
	Space string
	Local string
}

type Depth string

const (
	DepthZero     Depth = "0"
	DepthOne      Depth = "1"
	DepthInfinity Depth = "infinity"
)

func ParseDepth(value string, fallback Depth) (Depth, error) {
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("depth must not contain line breaks")
	}
	value = strings.TrimSpace(value)
	if value == "" {
		if fallback == "" {
			return "", fmt.Errorf("depth is required")
		}
		return fallback, nil
	}
	switch strings.ToLower(value) {
	case string(DepthZero):
		return DepthZero, nil
	case string(DepthOne):
		return DepthOne, nil
	case string(DepthInfinity):
		return DepthInfinity, nil
	default:
		return "", fmt.Errorf("unsupported depth %q", value)
	}
}

type PropfindKind string

const (
	PropfindAllProp  PropfindKind = "allprop"
	PropfindPropName PropfindKind = "propname"
	PropfindProp     PropfindKind = "prop"
)

type PropfindRequest struct {
	Kind       PropfindKind
	Properties []XMLName
	Include    []XMLName
}

type ReportKind string

const (
	ReportAddressBookQuery ReportKind = "addressbook-query"
	ReportAddressBookMulti ReportKind = "addressbook-multiget"
	ReportSyncCollection   ReportKind = "sync-collection"
)

type ReportRequest struct {
	Kind       ReportKind
	Properties []XMLName
	Hrefs      []string
	SyncToken  string
	SyncLevel  string
	Limit      int
	HasFilter  bool
	Filter     AddressBookQueryFilter
}

func ParsePropfind(r io.Reader) (PropfindRequest, error) {
	body, err := readBoundedXMLBody(r)
	if err != nil {
		return PropfindRequest{}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return PropfindRequest{Kind: PropfindAllProp}, nil
	}

	dec := newWebDAVXMLDecoder(body)
	root, err := nextStart(dec)
	if err != nil {
		return PropfindRequest{}, err
	}
	if !sameXMLName(root.Name, DAVNamespace, "propfind") {
		return PropfindRequest{}, fmt.Errorf("unsupported PROPFIND root {%s}%s", root.Name.Space, root.Name.Local)
	}

	var req PropfindRequest
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return PropfindRequest{}, fmt.Errorf("unterminated PROPFIND body")
		}
		if err != nil {
			return PropfindRequest{}, fmt.Errorf("decode PROPFIND body: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "allprop"):
				if req.Kind != "" {
					return PropfindRequest{}, fmt.Errorf("PROPFIND body must contain one request mode")
				}
				req.Kind = PropfindAllProp
				if err := skipElement(dec, tok.Name); err != nil {
					return PropfindRequest{}, err
				}
			case sameXMLName(tok.Name, DAVNamespace, "propname"):
				if req.Kind != "" {
					return PropfindRequest{}, fmt.Errorf("PROPFIND body must contain one request mode")
				}
				req.Kind = PropfindPropName
				if err := skipElement(dec, tok.Name); err != nil {
					return PropfindRequest{}, err
				}
			case sameXMLName(tok.Name, DAVNamespace, "prop"):
				if req.Kind != "" {
					return PropfindRequest{}, fmt.Errorf("PROPFIND body must contain one request mode")
				}
				req.Kind = PropfindProp
				properties, err := parsePropElement(dec, tok.Name)
				if err != nil {
					return PropfindRequest{}, err
				}
				req.Properties = properties
			case sameXMLName(tok.Name, DAVNamespace, "include"):
				if req.Kind != PropfindAllProp {
					return PropfindRequest{}, fmt.Errorf("PROPFIND include is only supported with allprop")
				}
				include, err := parsePropElement(dec, tok.Name)
				if err != nil {
					return PropfindRequest{}, err
				}
				req.Include = append(req.Include, include...)
				if len(req.Include) > MaxWebDAVProperties {
					return PropfindRequest{}, fmt.Errorf("too many WebDAV include properties")
				}
			default:
				return PropfindRequest{}, fmt.Errorf("unsupported PROPFIND element {%s}%s", tok.Name.Space, tok.Name.Local)
			}
		case xml.EndElement:
			if sameName(tok.Name, root.Name) {
				if req.Kind == "" {
					req.Kind = PropfindAllProp
				}
				if req.Kind == PropfindProp && len(req.Properties) == 0 {
					return PropfindRequest{}, fmt.Errorf("PROPFIND prop request must include at least one property")
				}
				if err := rejectTrailingXML(dec); err != nil {
					return PropfindRequest{}, err
				}
				return req, nil
			}
		}
	}
}

func ParseReport(r io.Reader) (ReportRequest, error) {
	body, err := readBoundedXMLBody(r)
	if err != nil {
		return ReportRequest{}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return ReportRequest{}, fmt.Errorf("REPORT body is required")
	}
	dec := newWebDAVXMLDecoder(body)
	root, err := nextStart(dec)
	if err != nil {
		return ReportRequest{}, err
	}
	req, err := classifyReportRoot(root.Name)
	if err != nil {
		return ReportRequest{}, err
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return ReportRequest{}, fmt.Errorf("unterminated REPORT body")
		}
		if err != nil {
			return ReportRequest{}, fmt.Errorf("decode REPORT body: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "prop"):
				properties, err := parsePropElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Properties = append(req.Properties, properties...)
				if len(req.Properties) > MaxWebDAVProperties {
					return ReportRequest{}, fmt.Errorf("too many WebDAV properties")
				}
			case sameXMLName(tok.Name, DAVNamespace, "href"):
				href, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				if len(req.Hrefs) >= MaxWebDAVHrefs {
					return ReportRequest{}, fmt.Errorf("too many WebDAV hrefs")
				}
				req.Hrefs = append(req.Hrefs, strings.TrimSpace(href))
			case sameXMLName(tok.Name, DAVNamespace, "sync-token"):
				token, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.SyncToken = strings.TrimSpace(token)
			case sameXMLName(tok.Name, DAVNamespace, "sync-level"):
				level, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.SyncLevel = strings.TrimSpace(level)
			case sameXMLName(tok.Name, DAVNamespace, "limit"):
				limit, err := parseLimitElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Limit = limit
			case sameXMLName(tok.Name, CardDAVNamespace, "filter"):
				req.HasFilter = true
				filter, err := parseAddressBookFilter(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Filter = filter
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return ReportRequest{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, root.Name) {
				if err := rejectTrailingXML(dec); err != nil {
					return ReportRequest{}, err
				}
				if err := validateReportRequest(req); err != nil {
					return ReportRequest{}, err
				}
				return req, nil
			}
		}
	}
}

func classifyReportRoot(name xml.Name) (ReportRequest, error) {
	switch {
	case sameXMLName(name, CardDAVNamespace, "addressbook-query"):
		return ReportRequest{Kind: ReportAddressBookQuery}, nil
	case sameXMLName(name, CardDAVNamespace, "addressbook-multiget"):
		return ReportRequest{Kind: ReportAddressBookMulti}, nil
	case sameXMLName(name, DAVNamespace, "sync-collection"):
		return ReportRequest{Kind: ReportSyncCollection}, nil
	default:
		return ReportRequest{}, fmt.Errorf("unsupported REPORT root {%s}%s", name.Space, name.Local)
	}
}

func validateReportRequest(req ReportRequest) error {
	switch req.Kind {
	case ReportAddressBookQuery:
		if !req.HasFilter {
			return fmt.Errorf("addressbook-query REPORT requires a filter element")
		}
	case ReportAddressBookMulti:
		if len(req.Hrefs) == 0 {
			return fmt.Errorf("addressbook-multiget REPORT requires at least one href")
		}
	case ReportSyncCollection:
		if req.SyncLevel == "" {
			return fmt.Errorf("sync-collection REPORT requires sync-level")
		}
		if req.SyncLevel != "1" {
			return fmt.Errorf("unsupported sync-level %q", req.SyncLevel)
		}
		if len(req.Properties) == 0 {
			return fmt.Errorf("sync-collection REPORT requires at least one property")
		}
	}
	return nil
}

func readBoundedXMLBody(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("XML body reader is required")
	}
	limited := io.LimitReader(r, MaxWebDAVXMLBodyBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read WebDAV XML body: %w", err)
	}
	if len(body) > MaxWebDAVXMLBodyBytes {
		return nil, fmt.Errorf("WebDAV XML body exceeds %d bytes", MaxWebDAVXMLBodyBytes)
	}
	return body, nil
}

func newWebDAVXMLDecoder(body []byte) *xml.Decoder {
	dec := xml.NewDecoder(bytes.NewReader(body))
	dec.Strict = true
	return dec
}

func nextStart(dec *xml.Decoder) (xml.StartElement, error) {
	for {
		tok, err := dec.Token()
		if err != nil {
			return xml.StartElement{}, fmt.Errorf("decode XML root: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			return tok, nil
		case xml.Directive:
			return xml.StartElement{}, fmt.Errorf("XML directives are not supported")
		}
	}
}

func parsePropElement(dec *xml.Decoder, propName xml.Name) ([]XMLName, error) {
	var properties []XMLName
	depth := 1
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unterminated prop element")
		}
		if err != nil {
			return nil, fmt.Errorf("decode prop element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > MaxWebDAVXMLDepth {
				return nil, fmt.Errorf("WebDAV XML exceeds maximum depth")
			}
			if len(properties) >= MaxWebDAVProperties {
				return nil, fmt.Errorf("too many WebDAV properties")
			}
			properties = append(properties, XMLName{Space: tok.Name.Space, Local: tok.Name.Local})
			if err := skipElement(dec, tok.Name); err != nil {
				return nil, err
			}
			depth--
		case xml.EndElement:
			if sameName(tok.Name, propName) {
				return properties, nil
			}
		}
	}
}

type AddressBookQueryFilter struct {
	PropertyName   string
	TextMatch      CardDAVTextMatch
	HasTextMatch   bool
	HasPropFilter  bool
	ParamFilter    CardDAVParamFilter
	HasParamFilter bool
}

type CardDAVParamFilter struct {
	Name         string
	TextMatch    CardDAVTextMatch
	HasTextMatch bool
	IsNotDefined bool
}

type CardDAVTextMatch struct {
	Text      string
	MatchType string
	Collation string
	Negate    bool
}

const (
	TextMatchEquals     = "equals"
	TextMatchContains   = "contains"
	TextMatchStartsWith = "starts-with"
	TextMatchEndsWith   = "ends-with"

	TextMatchUnicodeCasemap = "i;unicode-casemap"
)

func parseAddressBookFilter(dec *xml.Decoder, filterName xml.Name) (AddressBookQueryFilter, error) {
	return parseAddressBookFilterDepth(dec, filterName, 1, "")
}

func parseAddressBookFilterDepth(dec *xml.Decoder, filterName xml.Name, depth int, currentProperty string) (AddressBookQueryFilter, error) {
	if depth > MaxWebDAVXMLDepth {
		return AddressBookQueryFilter{}, fmt.Errorf("WebDAV XML exceeds maximum depth")
	}
	var filter AddressBookQueryFilter
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
				propName, err := propFilterName(tok)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				nested, err := parseAddressBookFilterDepth(dec, tok.Name, depth+1, propName)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				if !nested.HasPropFilter {
					nested.PropertyName = propName
					nested.HasPropFilter = true
				}
				if addressBookQueryFilterEmpty(filter) {
					filter = nested
				}
			case sameXMLName(tok.Name, CardDAVNamespace, "param-filter"):
				if currentProperty == "" {
					return AddressBookQueryFilter{}, fmt.Errorf("CardDAV param-filter requires a prop-filter parent")
				}
				paramFilter, err := parseParamFilterElement(dec, tok)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				if !filter.HasParamFilter {
					filter.PropertyName = currentProperty
					filter.HasPropFilter = true
					filter.ParamFilter = paramFilter
					filter.HasParamFilter = true
				}
			case sameXMLName(tok.Name, CardDAVNamespace, "text-match"):
				match, err := parseTextMatchElement(dec, tok)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				if !filter.HasTextMatch {
					filter = AddressBookQueryFilter{
						PropertyName:  currentProperty,
						TextMatch:     match,
						HasTextMatch:  true,
						HasPropFilter: currentProperty != "",
					}
				}
			default:
				nested, err := parseAddressBookFilterDepth(dec, tok.Name, depth+1, currentProperty)
				if err != nil {
					return AddressBookQueryFilter{}, err
				}
				if addressBookQueryFilterEmpty(filter) {
					filter = nested
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, filterName) {
				return filter, nil
			}
		}
	}
}

func addressBookQueryFilterEmpty(filter AddressBookQueryFilter) bool {
	return !filter.HasPropFilter && !filter.HasTextMatch && !filter.HasParamFilter
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
				if !filter.HasTextMatch {
					filter.TextMatch = match
					filter.HasTextMatch = true
				}
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
			if collation != TextMatchUnicodeCasemap {
				return CardDAVTextMatch{}, fmt.Errorf("unsupported CardDAV text-match collation %q", collation)
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

func parseLimitElement(dec *xml.Decoder, name xml.Name) (int, error) {
	var limit int
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return 0, fmt.Errorf("unterminated limit element")
		}
		if err != nil {
			return 0, err
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if !sameXMLName(tok.Name, DAVNamespace, "nresults") {
				if err := skipElement(dec, tok.Name); err != nil {
					return 0, err
				}
				continue
			}
			raw, err := readSimpleElementText(dec, tok.Name)
			if err != nil {
				return 0, err
			}
			value, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil || value <= 0 {
				return 0, fmt.Errorf("limit nresults must be a positive integer")
			}
			if value > MaxWebDAVReportLimit {
				return 0, fmt.Errorf("limit nresults exceeds %d", MaxWebDAVReportLimit)
			}
			limit = value
		case xml.EndElement:
			if sameName(tok.Name, name) {
				return limit, nil
			}
		}
	}
}

func readSimpleElementText(dec *xml.Decoder, name xml.Name) (string, error) {
	var b strings.Builder
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return "", fmt.Errorf("unterminated {%s}%s element", name.Space, name.Local)
		}
		if err != nil {
			return "", err
		}
		switch tok := tok.(type) {
		case xml.CharData:
			b.Write([]byte(tok))
		case xml.StartElement:
			return "", fmt.Errorf("{%s}%s must not contain nested elements", name.Space, name.Local)
		case xml.EndElement:
			if sameName(tok.Name, name) {
				return b.String(), nil
			}
		}
	}
}

func skipElement(dec *xml.Decoder, name xml.Name) error {
	depth := 1
	for depth > 0 {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated {%s}%s element", name.Space, name.Local)
		}
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			depth++
			if depth > MaxWebDAVXMLDepth {
				return fmt.Errorf("WebDAV XML exceeds maximum depth")
			}
		case xml.EndElement:
			depth--
		}
	}
	return nil
}

func rejectTrailingXML(dec *xml.Decoder) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode trailing XML: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			return fmt.Errorf("multiple XML root elements are not supported")
		case xml.CharData:
			if len(bytes.TrimSpace(tok)) != 0 {
				return fmt.Errorf("unexpected trailing XML character data")
			}
		}
	}
}

func sameXMLName(name xml.Name, space string, local string) bool {
	return name.Space == space && name.Local == local
}

func sameName(a xml.Name, b xml.Name) bool {
	return a.Space == b.Space && a.Local == b.Local
}
