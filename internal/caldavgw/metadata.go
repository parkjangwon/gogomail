package caldavgw

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
)

const (
	CalendarStatusActive  = "active"
	CalendarStatusDeleted = "deleted"

	ComponentVEVENT    = "VEVENT"
	ComponentVTODO     = "VTODO"
	ComponentVJOURNAL  = "VJOURNAL"
	ComponentVFREEBUSY = "VFREEBUSY"

	MaxCalendarNameBytes        = 255
	MaxCalendarDescriptionBytes = 2048
	MaxCalendarObjectUIDBytes   = 255
	MaxCalendarObjectNameBytes  = 200
	MaxCalendarObjectBytes      = 10 << 20
)

func NormalizeCalendarName(name string) (string, error) {
	name, err := ValidateCalendarName(name)
	if err != nil {
		return "", err
	}
	return strings.ToLower(name), nil
}

func ValidateCalendarName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("calendar name is required")
	}
	if len(name) > MaxCalendarNameBytes {
		return "", fmt.Errorf("calendar name is too long")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("calendar name must not contain line breaks")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("calendar name must not contain control characters")
		}
	}
	return name, nil
}

func ValidateCalendarColor(color string) (string, error) {
	color = strings.TrimSpace(color)
	if color == "" {
		return "", nil
	}
	if len(color) != 7 || color[0] != '#' {
		return "", fmt.Errorf("calendar color must be #RRGGBB")
	}
	for _, r := range color[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return "", fmt.Errorf("calendar color must be #RRGGBB")
		}
	}
	return strings.ToUpper(color), nil
}

func ValidateCalendarDescription(description string) (string, error) {
	description = strings.TrimSpace(description)
	if len(description) > MaxCalendarDescriptionBytes {
		return "", fmt.Errorf("calendar description is too long")
	}
	if strings.ContainsAny(description, "\r\n") {
		return "", fmt.Errorf("calendar description must not contain line breaks")
	}
	for _, r := range description {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("calendar description must not contain control characters")
		}
	}
	return description, nil
}

func ValidateCalendarStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	if status == "" {
		return CalendarStatusActive, nil
	}
	switch status {
	case CalendarStatusActive, CalendarStatusDeleted:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported calendar status %q", status)
	}
}

func ValidateCalendarObjectUID(uid string) (string, error) {
	uid = strings.TrimSpace(uid)
	if uid == "" {
		return "", fmt.Errorf("calendar object uid is required")
	}
	if len(uid) > MaxCalendarObjectUIDBytes {
		return "", fmt.Errorf("calendar object uid is too long")
	}
	if strings.ContainsAny(uid, "\r\n") {
		return "", fmt.Errorf("calendar object uid must not contain line breaks")
	}
	for _, r := range uid {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("calendar object uid must not contain control characters")
		}
	}
	return uid, nil
}

func ValidateCalendarObjectName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("calendar object name is required")
	}
	if len(name) > MaxCalendarObjectNameBytes {
		return "", fmt.Errorf("calendar object name is too long")
	}
	if strings.ContainsAny(name, "\r\n/\\") {
		return "", fmt.Errorf("calendar object name must be a single path segment")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("calendar object name must not contain control characters")
		}
	}
	if !strings.HasSuffix(strings.ToLower(name), ".ics") {
		return "", fmt.Errorf("calendar object name must end with .ics")
	}
	return name, nil
}

func ValidateCalendarComponent(component string) (string, error) {
	component = strings.TrimSpace(strings.ToUpper(component))
	if component == "" {
		return ComponentVEVENT, nil
	}
	switch component {
	case ComponentVEVENT, ComponentVTODO, ComponentVJOURNAL, ComponentVFREEBUSY:
		return component, nil
	default:
		return "", fmt.Errorf("unsupported calendar component %q", component)
	}
}

func ValidateCalendarPathID(id string) (string, error) {
	id = strings.TrimSpace(strings.ToLower(id))
	if len(id) != 36 {
		return "", fmt.Errorf("calendar path id must be a UUID")
	}
	for i, r := range id {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return "", fmt.Errorf("calendar path id must be a UUID")
			}
		default:
			if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
				return "", fmt.Errorf("calendar path id must be a lowercase UUID")
			}
		}
	}
	return id, nil
}

func CalendarSyncToken(parts ...string) string {
	h := sha256.New()
	for _, part := range parts {
		_, _ = h.Write([]byte(part))
		_, _ = h.Write([]byte{0})
	}
	return "sync-" + hex.EncodeToString(h.Sum(nil))
}

func CalendarCollectionETag(userID string, calendar Calendar) (string, error) {
	userID = strings.TrimSpace(userID)
	calendarID := strings.TrimSpace(calendar.ID)
	syncToken := strings.TrimSpace(calendar.SyncToken)
	if userID == "" || calendarID == "" || syncToken == "" {
		return "", fmt.Errorf("calendar collection etag requires user, calendar, and sync token")
	}
	sum := sha256.Sum256([]byte(CalendarSyncToken("collection-etag", userID, calendarID, syncToken)))
	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func StrongETag(body []byte) (string, error) {
	if len(body) == 0 {
		return "", fmt.Errorf("calendar object body is required")
	}
	if len(body) > MaxCalendarObjectBytes {
		return "", fmt.Errorf("calendar object body exceeds %d bytes", MaxCalendarObjectBytes)
	}
	sum := sha256.Sum256(body)
	return `"` + hex.EncodeToString(sum[:]) + `"`, nil
}

func ValidateStrongETag(etag string) (string, error) {
	etag = strings.TrimSpace(etag)
	if len(etag) != 66 || etag[0] != '"' || etag[len(etag)-1] != '"' {
		return "", fmt.Errorf("etag must be a quoted sha256 value")
	}
	for _, r := range etag[1 : len(etag)-1] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return "", fmt.Errorf("etag must be a quoted lowercase sha256 value")
		}
	}
	return etag, nil
}

func SyncTokenForETag(etag string) (string, error) {
	etag, err := ValidateStrongETag(etag)
	if err != nil {
		return "", err
	}
	return "sync-" + strings.Trim(etag, `"`), nil
}
