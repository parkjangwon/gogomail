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

type MKCalendarRequest struct {
	DisplayName string
	Description string
	Color       string
	Slug        *string
}

type ProppatchRequest struct {
	Name        *string
	Description *string
	Color       *string
	Properties  []XMLName
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

func ParseMKCalendar(r io.Reader) (MKCalendarRequest, error) {
	body, err := readBoundedXMLBody(r)
	if err != nil {
		return MKCalendarRequest{}, err
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return MKCalendarRequest{}, nil
	}
	dec := newWebDAVXMLDecoder(body)
	root, err := nextStart(dec)
	if err != nil {
		return MKCalendarRequest{}, err
	}
	if !sameXMLName(root.Name, CalDAVNamespace, "mkcalendar") {
		return MKCalendarRequest{}, fmt.Errorf("unsupported MKCALENDAR root {%s}%s", root.Name.Space, root.Name.Local)
	}
	var req MKCalendarRequest
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return MKCalendarRequest{}, fmt.Errorf("unterminated MKCALENDAR body")
		}
		if err != nil {
			return MKCalendarRequest{}, fmt.Errorf("decode MKCALENDAR body: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "set") {
				if err := parseMKCalendarSet(dec, tok.Name, &req); err != nil {
					return MKCalendarRequest{}, err
				}
				continue
			}
			if err := skipElement(dec, tok.Name); err != nil {
				return MKCalendarRequest{}, err
			}
		case xml.EndElement:
			if sameName(tok.Name, root.Name) {
				if err := rejectTrailingXML(dec); err != nil {
					return MKCalendarRequest{}, err
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
					return fmt.Errorf("displayname cannot be removed from a CalDAV calendar collection")
				}
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				value := strings.TrimSpace(text)
				req.Name = &value
				req.Properties = append(req.Properties, PropDisplayName)
			case sameXMLName(tok.Name, CalDAVNamespace, "calendar-description"):
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
				req.Properties = append(req.Properties, PropCalendarDescription)
			case sameXMLName(tok.Name, CalendarServerNamespace, "calendar-color") ||
				sameXMLName(tok.Name, "http://apple.com/ns/ical/", "calendar-color"):
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
				req.Color = &value
				req.Properties = append(req.Properties, PropCalendarColor)
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

func parseMKCalendarSet(dec *xml.Decoder, setName xml.Name, req *MKCalendarRequest) error {
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated MKCALENDAR set element")
		}
		if err != nil {
			return fmt.Errorf("decode MKCALENDAR set: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			if sameXMLName(tok.Name, DAVNamespace, "prop") {
				if err := parseMKCalendarProp(dec, tok.Name, req); err != nil {
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

func parseMKCalendarProp(dec *xml.Decoder, propName xml.Name, req *MKCalendarRequest) error {
	properties := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return fmt.Errorf("unterminated MKCALENDAR prop element")
		}
		if err != nil {
			return fmt.Errorf("decode MKCALENDAR prop: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			properties++
			if properties > MaxWebDAVProperties {
				return fmt.Errorf("too many MKCALENDAR properties")
			}
			switch {
			case sameXMLName(tok.Name, DAVNamespace, "displayname"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				req.DisplayName = strings.TrimSpace(text)
			case sameXMLName(tok.Name, CalDAVNamespace, "calendar-description"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				req.Description = strings.TrimSpace(text)
			case sameXMLName(tok.Name, CalendarServerNamespace, "calendar-color") ||
				sameXMLName(tok.Name, "http://apple.com/ns/ical/", "calendar-color"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				req.Color = strings.TrimSpace(text)
			case sameXMLName(tok.Name, "http://apple.com/ns/icalendar/", "calendar-slug"):
				text, err := readSimpleElementText(dec, tok.Name)
				if err != nil {
					return err
				}
				slug := strings.TrimSpace(text)
				if slug != "" {
					req.Slug = &slug
				}
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

type ReportKind string

const (
	ReportCalendarQuery  ReportKind = "calendar-query"
	ReportCalendarMulti  ReportKind = "calendar-multiget"
	ReportFreeBusyQuery  ReportKind = "free-busy-query"
	ReportSyncCollection ReportKind = "sync-collection"
)

type ReportRequest struct {
	Kind         ReportKind
	Properties   []XMLName
	CalendarData CalendarDataRequest
	Hrefs        []string
	SyncToken    string
	HasSyncToken bool
	SyncLevel    string
	Limit        int
	TimeRange    *TimeRange
	TimeRanges   int
	HasFilter    bool
	Component    string
}

type CalendarDataRequest struct {
	Requested           bool
	HasProjection       bool
	CalendarProperties  map[string]bool
	Component           string
	ComponentProperties map[string]bool
}

type TimeRange struct {
	Start time.Time
	End   time.Time
}

type ReportFilter struct {
	TimeRange *TimeRange
	Component string
}

const unsupportedCalendarQueryComponent = "__unsupported__"

type UnsupportedCalendarFilterError struct {
	Element XMLName
}

func (e UnsupportedCalendarFilterError) Error() string {
	if e.Element.Local == "" {
		return "unsupported CalDAV calendar-query filter"
	}
	return fmt.Sprintf("unsupported CalDAV calendar-query filter {%s}%s", e.Element.Space, e.Element.Local)
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
				properties, calendarData, err := parseReportPropElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.Properties = append(req.Properties, properties...)
				if calendarData.Requested {
					req.CalendarData = calendarData
				}
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
			case sameXMLName(tok.Name, CalDAVNamespace, "filter"):
				req.HasFilter = true
				filter, err := parseFilterElement(dec, tok.Name)
				if err != nil {
					return ReportRequest{}, err
				}
				req.TimeRange = filter.TimeRange
				req.Component = filter.Component
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

func parseReportPropElement(dec *xml.Decoder, propName xml.Name) ([]XMLName, CalendarDataRequest, error) {
	var properties []XMLName
	var calendarData CalendarDataRequest
	depth := 1
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return nil, CalendarDataRequest{}, fmt.Errorf("unterminated prop element")
		}
		if err != nil {
			return nil, CalendarDataRequest{}, fmt.Errorf("decode prop element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			depth++
			if depth > MaxWebDAVXMLDepth {
				return nil, CalendarDataRequest{}, fmt.Errorf("WebDAV XML exceeds maximum depth")
			}
			if len(properties) >= MaxWebDAVProperties {
				return nil, CalendarDataRequest{}, fmt.Errorf("too many WebDAV properties")
			}
			name := xmlName(tok.Name)
			properties = append(properties, name)
			if name == PropCalendarData {
				parsed, err := parseCalendarDataElement(dec, tok)
				if err != nil {
					return nil, CalendarDataRequest{}, err
				}
				calendarData = parsed
			} else if err := skipElement(dec, tok.Name); err != nil {
				return nil, CalendarDataRequest{}, err
			}
			depth--
		case xml.EndElement:
			if sameName(tok.Name, propName) {
				return properties, calendarData, nil
			}
		}
	}
}

func parseCalendarDataElement(dec *xml.Decoder, start xml.StartElement) (CalendarDataRequest, error) {
	req := CalendarDataRequest{Requested: true}
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "content-type":
			contentType := strings.TrimSpace(attr.Value)
			if contentType != "" && !strings.EqualFold(contentType, "text/calendar") {
				return CalendarDataRequest{}, fmt.Errorf("calendar-data content-type must be text/calendar")
			}
		case "version":
			version := strings.TrimSpace(attr.Value)
			if version != "" && version != "2.0" {
				return CalendarDataRequest{}, fmt.Errorf("calendar-data version must be 2.0")
			}
		}
	}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return CalendarDataRequest{}, fmt.Errorf("unterminated calendar-data element")
		}
		if err != nil {
			return CalendarDataRequest{}, fmt.Errorf("decode calendar-data element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			req.HasProjection = true
			switch {
			case sameXMLName(tok.Name, CalDAVNamespace, "comp"):
				parsed, err := parseCalendarDataComp(dec, tok, "")
				if err != nil {
					return CalendarDataRequest{}, err
				}
				req = mergeCalendarDataRequest(req, parsed)
			case sameXMLName(tok.Name, CalDAVNamespace, "prop"):
				name := strings.ToUpper(strings.TrimSpace(xmlAttr(tok, "name")))
				if name != "" {
					if req.CalendarProperties == nil {
						req.CalendarProperties = make(map[string]bool)
					}
					req.CalendarProperties[name] = true
				}
				if err := skipElement(dec, tok.Name); err != nil {
					return CalendarDataRequest{}, err
				}
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return CalendarDataRequest{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, start.Name) {
				return req, nil
			}
		}
	}
}

func parseCalendarDataComp(dec *xml.Decoder, start xml.StartElement, parent string) (CalendarDataRequest, error) {
	component := strings.ToUpper(strings.TrimSpace(xmlAttr(start, "name")))
	req := CalendarDataRequest{Requested: true, HasProjection: true}
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return CalendarDataRequest{}, fmt.Errorf("unterminated calendar-data comp element")
		}
		if err != nil {
			return CalendarDataRequest{}, fmt.Errorf("decode calendar-data comp element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CalDAVNamespace, "prop"):
				name := strings.ToUpper(strings.TrimSpace(xmlAttr(tok, "name")))
				if name != "" {
					if component == "VCALENDAR" || parent == "" {
						if req.CalendarProperties == nil {
							req.CalendarProperties = make(map[string]bool)
						}
						req.CalendarProperties[name] = true
					} else {
						if req.ComponentProperties == nil {
							req.ComponentProperties = make(map[string]bool)
						}
						req.ComponentProperties[name] = true
					}
				}
				if err := skipElement(dec, tok.Name); err != nil {
					return CalendarDataRequest{}, err
				}
			case sameXMLName(tok.Name, CalDAVNamespace, "comp"):
				nested, err := parseCalendarDataComp(dec, tok, component)
				if err != nil {
					return CalendarDataRequest{}, err
				}
				req = mergeCalendarDataRequest(req, nested)
			default:
				if err := skipElement(dec, tok.Name); err != nil {
					return CalendarDataRequest{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, start.Name) {
				if parent == "VCALENDAR" && component != "" {
					req.Component = component
				}
				return req, nil
			}
		}
	}
}

func mergeCalendarDataRequest(left CalendarDataRequest, right CalendarDataRequest) CalendarDataRequest {
	left.Requested = left.Requested || right.Requested
	left.HasProjection = left.HasProjection || right.HasProjection
	if right.Component != "" {
		left.Component = right.Component
	}
	for name := range right.CalendarProperties {
		if left.CalendarProperties == nil {
			left.CalendarProperties = make(map[string]bool)
		}
		left.CalendarProperties[name] = true
	}
	for name := range right.ComponentProperties {
		if left.ComponentProperties == nil {
			left.ComponentProperties = make(map[string]bool)
		}
		left.ComponentProperties[name] = true
	}
	return left
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

func parseFilterElement(dec *xml.Decoder, filterName xml.Name) (ReportFilter, error) {
	var found ReportFilter
	topLevelComponents := 0
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return ReportFilter{}, fmt.Errorf("unterminated filter element")
		}
		if err != nil {
			return ReportFilter{}, fmt.Errorf("decode filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CalDAVNamespace, "time-range"):
				return ReportFilter{}, fmt.Errorf("calendar-query time-range must be inside a comp-filter")
			case sameXMLName(tok.Name, CalDAVNamespace, "comp-filter"):
				topLevelComponents++
				if topLevelComponents > 1 {
					return ReportFilter{}, fmt.Errorf("calendar-query filter must contain exactly one top-level comp-filter")
				}
				nested, err := parseCompFilterElement(dec, tok, "")
				if err != nil {
					return ReportFilter{}, err
				}
				found = mergeReportFilters(found, nested)
			default:
				if tok.Name.Space == CalDAVNamespace {
					return ReportFilter{}, UnsupportedCalendarFilterError{Element: xmlName(tok.Name)}
				}
				if err := skipElement(dec, tok.Name); err != nil {
					return ReportFilter{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, filterName) {
				if topLevelComponents != 1 {
					return ReportFilter{}, fmt.Errorf("calendar-query filter must contain a VCALENDAR comp-filter")
				}
				return found, nil
			}
		}
	}
}

func parseCompFilterElement(dec *xml.Decoder, start xml.StartElement, parentComponent string) (ReportFilter, error) {
	component := strings.ToUpper(strings.TrimSpace(xmlAttr(start, "name")))
	if component == "" {
		return ReportFilter{}, fmt.Errorf("CalDAV comp-filter name is required")
	}
	if parentComponent == "" && component != "VCALENDAR" {
		return ReportFilter{}, fmt.Errorf("CalDAV top-level comp-filter must be VCALENDAR")
	}
	var found ReportFilter
	if parentComponent == "VCALENDAR" && component != "" {
		if isSupportedCalendarComponent(component) {
			found.Component = component
		} else {
			found.Component = unsupportedCalendarQueryComponent
		}
	}
	hasTimeRange := false
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			return ReportFilter{}, fmt.Errorf("unterminated comp-filter element")
		}
		if err != nil {
			return ReportFilter{}, fmt.Errorf("decode comp-filter element: %w", err)
		}
		switch tok := tok.(type) {
		case xml.StartElement:
			switch {
			case sameXMLName(tok.Name, CalDAVNamespace, "time-range"):
				if hasTimeRange {
					return ReportFilter{}, fmt.Errorf("calendar-query comp-filter must not contain multiple time-range elements")
				}
				timeRange, err := parseTimeRangeElement(dec, tok)
				if err != nil {
					return ReportFilter{}, err
				}
				hasTimeRange = true
				found.TimeRange = &timeRange
			case sameXMLName(tok.Name, CalDAVNamespace, "comp-filter"):
				nested, err := parseCompFilterElement(dec, tok, component)
				if err != nil {
					return ReportFilter{}, err
				}
				found = mergeReportFilters(found, nested)
			default:
				if tok.Name.Space == CalDAVNamespace {
					return ReportFilter{}, UnsupportedCalendarFilterError{Element: xmlName(tok.Name)}
				}
				if err := skipElement(dec, tok.Name); err != nil {
					return ReportFilter{}, err
				}
			}
		case xml.EndElement:
			if sameName(tok.Name, start.Name) {
				return found, nil
			}
		}
	}
}

func mergeReportFilters(left ReportFilter, right ReportFilter) ReportFilter {
	if right.TimeRange != nil {
		left.TimeRange = right.TimeRange
	}
	if right.Component != "" {
		left.Component = right.Component
	}
	return left
}

func xmlAttr(start xml.StartElement, local string) string {
	for _, attr := range start.Attr {
		if attr.Name.Local == local {
			return attr.Value
		}
	}
	return ""
}

func isSupportedCalendarComponent(component string) bool {
	switch strings.ToUpper(strings.TrimSpace(component)) {
	case ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY:
		return true
	default:
		return false
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
