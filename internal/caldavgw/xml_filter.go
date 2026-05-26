package caldavgw

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

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

func parseICalendarUTC(value string) (time.Time, error) {
	if !strings.HasSuffix(value, "Z") {
		return time.Time{}, fmt.Errorf("timestamp must be UTC")
	}
	return time.Parse("20060102T150405Z", value)
}
