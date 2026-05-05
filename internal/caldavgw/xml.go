package caldavgw

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	DAVNamespace    = "DAV:"
	CalDAVNamespace = "urn:ietf:params:xml:ns:caldav"

	MaxWebDAVXMLBodyBytes = 1 << 20
	MaxWebDAVXMLDepth     = 64
	MaxWebDAVProperties   = 256
	MaxWebDAVHrefs        = 2048
	MaxWebDAVReportLimit  = 1000
)

type Depth string

const (
	DepthZero     Depth = "0"
	DepthOne      Depth = "1"
	DepthInfinity Depth = "infinity"
)

func ParseDepth(value string, fallback Depth) (Depth, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if fallback == "" {
			return "", fmt.Errorf("depth is required")
		}
		return fallback, nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("depth must not contain line breaks")
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

type XMLName struct {
	Space string
	Local string
}

func xmlName(name xml.Name) XMLName {
	return XMLName{Space: name.Space, Local: name.Local}
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

type ReportKind string

const (
	ReportCalendarQuery  ReportKind = "calendar-query"
	ReportCalendarMulti  ReportKind = "calendar-multiget"
	ReportFreeBusyQuery  ReportKind = "free-busy-query"
	ReportSyncCollection ReportKind = "sync-collection"
)

type ReportRequest struct {
	Kind       ReportKind
	Properties []XMLName
	Hrefs      []string
	SyncToken  string
	SyncLevel  string
	Limit      int
	TimeRange  *TimeRange
	TimeRanges int
	HasFilter  bool
}

type TimeRange struct {
	Start time.Time
	End   time.Time
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
			case sameXMLName(tok.Name, CalDAVNamespace, "filter"):
				req.HasFilter = true
				timeRange, err := parseFilterElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.TimeRange = timeRange
			case sameXMLName(tok.Name, CalDAVNamespace, "time-range"):
				timeRange, err := parseTimeRangeElement(dec, tok)
				if err != nil {
					return ReportRequest{}, err
				}
				req.TimeRanges++
				req.TimeRange = &timeRange
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

func validateReportRequest(req ReportRequest) error {
	switch req.Kind {
	case ReportCalendarQuery:
		if !req.HasFilter {
			return fmt.Errorf("calendar-query REPORT requires a filter element")
		}
	case ReportCalendarMulti:
		if len(req.Hrefs) == 0 {
			return fmt.Errorf("calendar-multiget REPORT requires at least one href")
		}
	case ReportFreeBusyQuery:
		if req.TimeRange == nil {
			return fmt.Errorf("free-busy-query REPORT requires a time-range")
		}
		if req.TimeRanges != 1 {
			return fmt.Errorf("free-busy-query REPORT requires exactly one time-range")
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

func classifyReportRoot(name xml.Name) (ReportRequest, error) {
	switch {
	case sameXMLName(name, CalDAVNamespace, "calendar-query"):
		return ReportRequest{Kind: ReportCalendarQuery}, nil
	case sameXMLName(name, CalDAVNamespace, "calendar-multiget"):
		return ReportRequest{Kind: ReportCalendarMulti}, nil
	case sameXMLName(name, CalDAVNamespace, "free-busy-query"):
		return ReportRequest{Kind: ReportFreeBusyQuery}, nil
	case sameXMLName(name, DAVNamespace, "sync-collection"):
		return ReportRequest{Kind: ReportSyncCollection}, nil
	default:
		return ReportRequest{}, fmt.Errorf("unsupported REPORT root {%s}%s", name.Space, name.Local)
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
			properties = append(properties, xmlName(tok.Name))
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

func parseFilterElement(dec *xml.Decoder, filterName xml.Name) (*TimeRange, error) {
	var found *TimeRange
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, fmt.Errorf("unterminated filter element")
		}
		if err != nil {
			return nil, fmt.Errorf("decode filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, CalDAVNamespace, "time-range") {
				timeRange, err := parseTimeRangeElement(dec, tok)
				if err != nil {
					return nil, err
				}
				found = &timeRange
				continue
			}
			nested, err := parseFilterElement(dec, tok.Name)
			if err != nil {
				return nil, err
			}
			if nested != nil {
				found = nested
			}
		case xml.EndElement:
			if sameName(tok.Name, filterName) {
				return found, nil
			}
		}
	}
}

func parseTimeRangeElement(dec *xml.Decoder, start xml.StartElement) (TimeRange, error) {
	var rawStart, rawEnd string
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "start":
			rawStart = strings.TrimSpace(attr.Value)
		case "end":
			rawEnd = strings.TrimSpace(attr.Value)
		}
	}
	if rawStart == "" || rawEnd == "" {
		return TimeRange{}, fmt.Errorf("time-range requires start and end")
	}
	if strings.ContainsAny(rawStart+rawEnd, "\r\n") {
		return TimeRange{}, fmt.Errorf("time-range values must not contain line breaks")
	}
	startTime, err := parseICalendarUTC(rawStart)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid time-range start: %w", err)
	}
	endTime, err := parseICalendarUTC(rawEnd)
	if err != nil {
		return TimeRange{}, fmt.Errorf("invalid time-range end: %w", err)
	}
	if !startTime.Before(endTime) {
		return TimeRange{}, fmt.Errorf("time-range start must be before end")
	}
	if err := skipElement(dec, start.Name); err != nil {
		return TimeRange{}, err
	}
	return TimeRange{Start: startTime, End: endTime}, nil
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

func parseICalendarUTC(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("timestamp must be UTC")
	}
	return time.Parse("20060102T150405Z", value)
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
