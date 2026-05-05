package caldavgw

import (
	"fmt"
	"net/url"
	"path"
	"strings"
	"unicode"
)

const (
	WellKnownCalDAVPath = "/.well-known/caldav"
	RootPath            = "/caldav"
	PrincipalsPrefix    = "/caldav/principals"
	CalendarsPrefix     = "/caldav/calendars"

	maxSegmentBytes = 200
)

func PrincipalPath(userID string) (string, error) {
	userID, err := validateSegment("user_id", userID)
	if err != nil {
		return "", err
	}
	return PrincipalsPrefix + "/" + url.PathEscape(userID) + "/", nil
}

func CalendarHomePath(userID string) (string, error) {
	userID, err := validateSegment("user_id", userID)
	if err != nil {
		return "", err
	}
	return CalendarsPrefix + "/" + url.PathEscape(userID) + "/", nil
}

func CalendarCollectionPath(userID string, calendarID string) (string, error) {
	home, err := CalendarHomePath(userID)
	if err != nil {
		return "", err
	}
	calendarID, err = validateSegment("calendar_id", calendarID)
	if err != nil {
		return "", err
	}
	return home + url.PathEscape(calendarID) + "/", nil
}

func CalendarObjectPath(userID string, calendarID string, objectName string) (string, error) {
	collection, err := CalendarCollectionPath(userID, calendarID)
	if err != nil {
		return "", err
	}
	objectName, err = validateICSObjectName(objectName)
	if err != nil {
		return "", err
	}
	return collection + url.PathEscape(objectName), nil
}

func ParseResourcePath(raw string) (ResourcePath, error) {
	if raw == "" {
		return ResourcePath{}, fmt.Errorf("caldav path is required")
	}
	if strings.ContainsAny(raw, "\r\n") {
		return ResourcePath{}, fmt.Errorf("caldav path must not contain line breaks")
	}
	parsed, err := url.PathUnescape(raw)
	if err != nil {
		return ResourcePath{}, fmt.Errorf("decode caldav path: %w", err)
	}
	if parsed != "/" && strings.HasSuffix(parsed, "/") {
		parsed = strings.TrimRight(parsed, "/") + "/"
	}
	cleaned := path.Clean(parsed)
	if strings.HasSuffix(parsed, "/") && cleaned != "/" {
		cleaned += "/"
	}
	if cleaned != parsed {
		return ResourcePath{}, fmt.Errorf("caldav path must be canonical")
	}
	if parsed == WellKnownCalDAVPath {
		return ResourcePath{Kind: ResourceWellKnown}, nil
	}
	if parsed == RootPath || parsed == RootPath+"/" {
		return ResourcePath{Kind: ResourceRoot}, nil
	}
	segments := splitPathSegments(parsed)
	if len(segments) == 0 || segments[0] != "caldav" {
		return ResourcePath{}, fmt.Errorf("unsupported caldav path")
	}
	if len(segments) == 3 && segments[1] == "principals" {
		userID, err := validateSegment("user_id", segments[2])
		if err != nil {
			return ResourcePath{}, err
		}
		return ResourcePath{Kind: ResourcePrincipal, UserID: userID}, nil
	}
	if len(segments) >= 3 && segments[1] == "calendars" {
		userID, err := validateSegment("user_id", segments[2])
		if err != nil {
			return ResourcePath{}, err
		}
		if len(segments) == 3 {
			return ResourcePath{Kind: ResourceCalendarHome, UserID: userID}, nil
		}
		calendarID, err := validateSegment("calendar_id", segments[3])
		if err != nil {
			return ResourcePath{}, err
		}
		if len(segments) == 4 {
			return ResourcePath{Kind: ResourceCalendarCollection, UserID: userID, CalendarID: calendarID}, nil
		}
		if len(segments) == 5 {
			objectName, err := validateICSObjectName(segments[4])
			if err != nil {
				return ResourcePath{}, err
			}
			return ResourcePath{Kind: ResourceCalendarObject, UserID: userID, CalendarID: calendarID, ObjectName: objectName}, nil
		}
	}
	return ResourcePath{}, fmt.Errorf("unsupported caldav path")
}

func splitPathSegments(value string) []string {
	value = strings.Trim(value, "/")
	if value == "" {
		return nil
	}
	return strings.Split(value, "/")
}

func validateICSObjectName(value string) (string, error) {
	value, err := validateSegment("object_name", value)
	if err != nil {
		return "", err
	}
	if !strings.HasSuffix(strings.ToLower(value), ".ics") {
		return "", fmt.Errorf("object_name must end with .ics")
	}
	return value, nil
}

func validateSegment(field string, value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("%s is required", field)
	}
	if len(value) > maxSegmentBytes {
		return "", fmt.Errorf("%s is too long", field)
	}
	if value == "." || value == ".." {
		return "", fmt.Errorf("%s is reserved", field)
	}
	if strings.ContainsAny(value, `/\`) {
		return "", fmt.Errorf("%s must not contain path separators", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("%s must not contain control characters", field)
		}
	}
	return value, nil
}
