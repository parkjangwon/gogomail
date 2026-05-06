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
	DAVNamespace            = "DAV:"
	CardDAVNamespace        = "urn:ietf:params:xml:ns:carddav"
	CalendarServerNamespace = "http://calendarserver.org/ns/"

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

type ProppatchRequest struct {
	Name        *string
	Description *string
	Properties  []XMLName
}

type MKAddressBookRequest struct {
	DisplayName string
	Description string
}

type ReportKind string

const (
	ReportAddressBookQuery ReportKind = "addressbook-query"
	ReportAddressBookMulti ReportKind = "addressbook-multiget"
	ReportSyncCollection   ReportKind = "sync-collection"
)

type ReportRequest struct {
	Kind                  ReportKind
	Properties            []XMLName
	AddressDataProperties []string
	Hrefs                 []string
	SyncToken             string
	HasSyncToken          bool
	SyncLevel             string
	Limit                 int
	HasFilter             bool
	Filter                AddressBookQueryFilter
}

type UnsupportedAddressDataError struct {
	Attribute string
	Value     string
}

func (e UnsupportedAddressDataError) Error() string {
	return fmt.Sprintf("unsupported CardDAV address-data %s %q", e.Attribute, e.Value)
}

type UnsupportedCollationError struct {
	Value string
}

func (e UnsupportedCollationError) Error() string {
	return fmt.Sprintf("unsupported CardDAV text-match collation %q", e.Value)
}

type UnsupportedFilterElementError struct {
	Name XMLName
}

func (e UnsupportedFilterElementError) Error() string {
	return fmt.Sprintf("unsupported CardDAV filter element {%s}%s", e.Name.Space, e.Name.Local)
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

func ParseProppatch(r io.Reader) (ProppatchRequest, error) {
	body, err := readBoundedXMLBody(r)
	if err != nil {
		return ProppatchRequest{}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return ProppatchRequest{}, fmt.Errorf("PROPPATCH body is required")
	}
	dec := newWebDAVXMLDecoder(body)
	root, err := nextStart(dec)
	if err != nil {
		return ProppatchRequest{}, err
	}
	if !sameXMLName(root.Name, DAVNamespace, "propertyupdate") {
		return ProppatchRequest{}, fmt.Errorf("unsupported PROPPATCH root {%s}%s", root.Name.Space, root.Name.Local)
	}
	var req ProppatchRequest
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return ProppatchRequest{}, fmt.Errorf("unterminated PROPPATCH body")
		}
		if err != nil {
			return ProppatchRequest{}, fmt.Errorf("decode PROPPATCH body: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "set"):
				if err := parseProppatchSet(dec, tok.Name, &req); err != nil {
					return ProppatchRequest{}, err
				}
			case sameXMLName(tok.Name, DAVNamespace, "remove"):
				if err := parseProppatchRemove(dec, tok.Name, &req); err != nil {
					return ProppatchRequest{}, err
				}
			default:
				return ProppatchRequest{}, fmt.Errorf("unsupported PROPPATCH element {%s}%s", tok.Name.Space, tok.Name.Local)
			}
		case xml.EndElement:
			if sameName(tok.Name, root.Name) {
				if err := rejectTrailingXML(dec); err != nil {
					return ProppatchRequest{}, err
				}
				if len(req.Properties) == 0 {
					return ProppatchRequest{}, fmt.Errorf("PROPPATCH must include at least one supported property")
				}
				return req, nil
			}
		}
	}
}

func ParseMKAddressBook(r io.Reader) (MKAddressBookRequest, error) {
	body, err := readBoundedXMLBody(r)
	if err != nil {
		return MKAddressBookRequest{}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return MKAddressBookRequest{}, nil
	}
	dec := newWebDAVXMLDecoder(body)
	root, err := nextStart(dec)
	if err != nil {
		return MKAddressBookRequest{}, err
	}
	if !sameXMLName(root.Name, DAVNamespace, "mkcol") {
		return MKAddressBookRequest{}, fmt.Errorf("unsupported MKCOL root {%s}%s", root.Name.Space, root.Name.Local)
	}
	var req MKAddressBookRequest
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return MKAddressBookRequest{}, fmt.Errorf("unterminated MKCOL body")
		}
		if err != nil {
			return MKAddressBookRequest{}, fmt.Errorf("decode MKCOL body: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "set") {
				if err := parseMKAddressBookSet(dec, tok.Name, &req); err != nil {
					return MKAddressBookRequest{}, err
				}
				continue
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return MKAddressBookRequest{}, err
			}
		case xml.EndElement:
			if sameName(tok.Name, root.Name) {
				if err := rejectTrailingXML(dec); err != nil {
					return MKAddressBookRequest{}, err
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
	hasLimit := false
	hasSyncLevel := false
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
				properties, addressDataProperties, err := parseReportPropElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Properties = append(req.Properties, properties...)
				req.AddressDataProperties = append(req.AddressDataProperties, addressDataProperties...)
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
				if req.HasSyncToken {
					return ReportRequest{}, fmt.Errorf("REPORT must not contain duplicate sync-token elements")
				}
				token, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.SyncToken = strings.TrimSpace(token)
				req.HasSyncToken = true
			case sameXMLName(tok.Name, DAVNamespace, "sync-level"):
				if hasSyncLevel {
					return ReportRequest{}, fmt.Errorf("REPORT must not contain duplicate sync-level elements")
				}
				level, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.SyncLevel = strings.TrimSpace(level)
				hasSyncLevel = true
			case sameXMLName(tok.Name, DAVNamespace, "limit"):
				if hasLimit {
					return ReportRequest{}, fmt.Errorf("REPORT must not contain duplicate limit elements")
				}
				limit, err := parseLimitElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Limit = limit
				hasLimit = true
			case sameXMLName(tok.Name, CardDAVNamespace, "filter"):
				req.HasFilter = true
				filter, err := parseAddressBookFilter(dec, tok)
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

func parseProppatchSet(dec *xml.Decoder, setName xml.Name, req *ProppatchRequest) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated PROPPATCH set element")
		}
		if err != nil {
			return fmt.Errorf("decode PROPPATCH set: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "prop") {
				if err := parseProppatchProp(dec, tok.Name, true, req); err != nil {
					return err
				}
				continue
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return err
			}
		case xml.EndElement:
			if sameName(tok.Name, setName) {
				return nil
			}
		}
	}
}

func parseProppatchRemove(dec *xml.Decoder, removeName xml.Name, req *ProppatchRequest) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated PROPPATCH remove element")
		}
		if err != nil {
			return fmt.Errorf("decode PROPPATCH remove: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "prop") {
				if err := parseProppatchProp(dec, tok.Name, false, req); err != nil {
					return err
				}
				continue
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return err
			}
		case xml.EndElement:
			if sameName(tok.Name, removeName) {
				return nil
			}
		}
	}
}

func parseProppatchProp(dec *xml.Decoder, propName xml.Name, set bool, req *ProppatchRequest) error {
	properties := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated PROPPATCH prop element")
		}
		if err != nil {
			return fmt.Errorf("decode PROPPATCH prop: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			properties++
			if properties > MaxWebDAVProperties {
				return fmt.Errorf("too many PROPPATCH properties")
			}
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "displayname"):
				if !set {
					return fmt.Errorf("displayname cannot be removed from a CardDAV address book collection")
				}
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				value := strings.TrimSpace(text)
				req.Name = &value
				req.Properties = append(req.Properties, PropDisplayName)
			case sameXMLName(tok.Name, CardDAVNamespace, "addressbook-description"):
				value := ""
				if set {
					text, err := readSimpleElementText(dec, tok.Name)
					if err != nil {
						return err
					}
					value = strings.TrimSpace(text)
				} else if err := skipElement(dec, tok.Name); err != nil {
					return err
				}
				req.Description = &value
				req.Properties = append(req.Properties, PropAddressBookDescription)
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, propName) {
				return nil
			}
		}
	}
}

func parseMKAddressBookSet(dec *xml.Decoder, setName xml.Name, req *MKAddressBookRequest) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated MKCOL set element")
		}
		if err != nil {
			return fmt.Errorf("decode MKCOL set: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "prop") {
				if err := parseMKAddressBookProp(dec, tok.Name, req); err != nil {
					return err
				}
				continue
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return err
			}
		case xml.EndElement:
			if sameName(tok.Name, setName) {
				return nil
			}
		}
	}
}

func parseMKAddressBookProp(dec *xml.Decoder, propName xml.Name, req *MKAddressBookRequest) error {
	properties := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated MKCOL prop element")
		}
		if err != nil {
			return fmt.Errorf("decode MKCOL prop: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			properties++
			if properties > MaxWebDAVProperties {
				return fmt.Errorf("too many MKCOL properties")
			}
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "resourcetype"):
				if err := parseAddressBookResourceType(dec, tok.Name); err != nil {
					return err
				}
			case sameXMLName(tok.Name, DAVNamespace, "displayname"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				req.DisplayName = strings.TrimSpace(text)
			case sameXMLName(tok.Name, CardDAVNamespace, "addressbook-description"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				req.Description = strings.TrimSpace(text)
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, propName) {
				return nil
			}
		}
	}
}

func parseAddressBookResourceType(dec *xml.Decoder, typeName xml.Name) error {
	var hasCollection, hasAddressBook bool
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated MKCOL resourcetype element")
		}
		if err != nil {
			return fmt.Errorf("decode MKCOL resourcetype: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "collection"):
				hasCollection = true
			case sameXMLName(tok.Name, CardDAVNamespace, "addressbook"):
				hasAddressBook = true
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return err
			}
		case xml.EndElement:
			if sameName(tok.Name, typeName) {
				if !hasCollection || !hasAddressBook {
					return fmt.Errorf("MKCOL resourcetype must include DAV:collection and CARDDAV:addressbook")
				}
				return nil
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
		if !req.HasSyncToken {
			return fmt.Errorf("sync-collection REPORT requires sync-token")
		}
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
	properties, _, err := parsePropElementDetailed(dec, propName)
	return properties, err
}

func parseReportPropElement(dec *xml.Decoder, propName xml.Name) ([]XMLName, []string, error) {
	return parsePropElementDetailed(dec, propName)
}

func parsePropElementDetailed(dec *xml.Decoder, propName xml.Name) ([]XMLName, []string, error) {
	var properties []XMLName
	var addressDataProperties []string
	depth := 1
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, nil, fmt.Errorf("unterminated prop element")
		}
		if err != nil {
			return nil, nil, fmt.Errorf("decode prop element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > MaxWebDAVXMLDepth {
				return nil, nil, fmt.Errorf("WebDAV XML exceeds maximum depth")
			}
			if len(properties) >= MaxWebDAVProperties {
				return nil, nil, fmt.Errorf("too many WebDAV properties")
			}
			properties = append(properties, XMLName{Space: tok.Name.Space, Local: tok.Name.Local})
			if sameXMLName(tok.Name, CardDAVNamespace, "address-data") {
				names, err := parseAddressDataElement(dec, tok)
				if err != nil {
					return nil, nil, err
				}
				addressDataProperties = append(addressDataProperties, names...)
			} else if err := skipElement(dec, tok.Name); err != nil {
				return nil, nil, err
			}
			depth--
		case xml.EndElement:
			if sameName(tok.Name, propName) {
				return properties, addressDataProperties, nil
			}
		}
	}
}

func parseAddressDataElement(dec *xml.Decoder, el xml.StartElement) ([]string, error) {
	if err := validateAddressDataAttributes(el); err != nil {
		return nil, err
	}
	var properties []string
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unterminated address-data element")
		}
		if err != nil {
			return nil, fmt.Errorf("decode address-data element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if !sameXMLName(tok.Name, CardDAVNamespace, "prop") {
				if err := skipElement(dec, tok.Name); err != nil {
					return nil, err
				}
				continue
			}
			propName, err := filterNameAttribute(tok, "address-data prop")
			if err != nil {
				return nil, err
			}
			if len(properties) >= MaxWebDAVProperties {
				return nil, fmt.Errorf("too many CardDAV address-data properties")
			}
			properties = append(properties, propName)
			if err := skipElement(dec, tok.Name); err != nil {
				return nil, err
			}
		case xml.EndElement:
			if sameName(tok.Name, el.Name) {
				return properties, nil
			}
		}
	}
}

func validateAddressDataAttributes(el xml.StartElement) error {
	for _, attr := range el.Attr {
		if strings.ContainsAny(attr.Value, "\r\n") {
			return fmt.Errorf("CardDAV address-data attribute is invalid")
		}
		switch attr.Name.Local {
		case "content-type":
			contentType := strings.ToLower(strings.TrimSpace(attr.Value))
			if contentType != "text/vcard" {
				return UnsupportedAddressDataError{Attribute: "content-type", Value: contentType}
			}
		case "version":
			version := strings.TrimSpace(attr.Value)
			if version != "3.0" && version != "4.0" {
				return UnsupportedAddressDataError{Attribute: "version", Value: version}
			}
		}
	}
	return nil
}

type AddressBookQueryFilter struct {
	Test        string
	PropFilters []CardDAVPropFilter
}

type CardDAVPropFilter struct {
	Name         string
	Test         string
	TextMatches  []CardDAVTextMatch
	ParamFilters []CardDAVParamFilter
	IsNotDefined bool
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
	FilterTestAnyOf = "anyof"
	FilterTestAllOf = "allof"

	TextMatchEquals     = "equals"
	TextMatchContains   = "contains"
	TextMatchStartsWith = "starts-with"
	TextMatchEndsWith   = "ends-with"

	TextMatchASCIICasemap   = "i;ascii-casemap"
	TextMatchUnicodeCasemap = "i;unicode-casemap"
)

func SupportedTextMatchCollations() []string {
	return []string{TextMatchASCIICasemap, TextMatchUnicodeCasemap}
}

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

func isSupportedTextMatchCollation(collation string) bool {
	for _, supported := range SupportedTextMatchCollations() {
		if collation == supported {
			return true
		}
	}
	return false
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
	hasNResults := false
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
			if hasNResults {
				return 0, fmt.Errorf("limit must not contain duplicate nresults elements")
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
			hasNResults = true
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
