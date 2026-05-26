package caldavgw

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

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
