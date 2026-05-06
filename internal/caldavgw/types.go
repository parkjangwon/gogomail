package caldavgw

import "time"

const (
	RFCWebDAV           = "RFC 4918"
	RFCCalDAV           = "RFC 4791"
	RFCICalendar        = "RFC 5545"
	RFCCalDAVScheduling = "RFC 6638"
	RFCWebDAVSync       = "RFC 6578"
	RFCCalDAVDiscovery  = "RFC 6764"
	RFCCalDAVTimeZones  = "RFC 7809"
)

const (
	MethodOptions    = "OPTIONS"
	MethodPropfind   = "PROPFIND"
	MethodProppatch  = "PROPPATCH"
	MethodReport     = "REPORT"
	MethodMkcalendar = "MKCALENDAR"
	MethodGet        = "GET"
	MethodHead       = "HEAD"
	MethodPut        = "PUT"
	MethodDelete     = "DELETE"
	MethodMove       = "MOVE"
)

const (
	DAVClass1           = "1"
	DAVClass3           = "3"
	DAVCalendarAccess   = "calendar-access"
	DAVCalendarSchedule = "calendar-auto-schedule"
	DAVSyncCollection   = "sync-collection"
)

type ResourceKind string

const (
	ResourceUnknown             ResourceKind = "unknown"
	ResourceWellKnown           ResourceKind = "well_known"
	ResourceRoot                ResourceKind = "root"
	ResourcePrincipalCollection ResourceKind = "principal_collection"
	ResourcePrincipal           ResourceKind = "principal"
	ResourceCalendarHome        ResourceKind = "calendar_home"
	ResourceCalendarCollection  ResourceKind = "calendar_collection"
	ResourceCalendarObject      ResourceKind = "calendar_object"
)

type ResourcePath struct {
	Kind       ResourceKind
	UserID     string
	CalendarID string
	ObjectName string
}

type Principal struct {
	UserID             string
	DisplayName        string
	CalendarHomePath   string
	PrincipalPath      string
	ScheduleInboxPath  string
	ScheduleOutboxPath string
}

type Calendar struct {
	ID          string
	UserID      string
	Name        string
	Color       string
	Description string
	SyncToken   string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type CalendarObject struct {
	ID         string
	UserID     string
	CalendarID string
	ObjectName string
	UID        string
	Component  string
	ETag       string
	Size       int64
	ICS        []byte
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type CalendarChange struct {
	ID         int64
	UserID     string
	CalendarID string
	ObjectName string
	ETag       string
	Action     string
	SyncToken  string
	ChangedAt  time.Time
}

type Store interface {
	GetPrincipal(userID string) (Principal, error)
	ListCalendars(userID string) ([]Calendar, error)
	GetCalendar(userID string, calendarID string) (Calendar, error)
	GetObject(userID string, calendarID string, objectName string) (CalendarObject, error)
}

func Standards() []string {
	return []string{
		RFCWebDAV,
		RFCCalDAV,
		RFCICalendar,
		RFCCalDAVScheduling,
		RFCWebDAVSync,
		RFCCalDAVDiscovery,
		RFCCalDAVTimeZones,
	}
}

func AdvertisedDAVTokens(includeScheduling bool) []string {
	tokens := []string{DAVClass1, DAVClass3, DAVCalendarAccess, DAVSyncCollection}
	if includeScheduling {
		tokens = append(tokens, DAVCalendarSchedule)
	}
	return tokens
}

func ImplementedMethods() []string {
	return []string{
		MethodOptions,
		MethodPropfind,
		MethodProppatch,
		MethodReport,
		MethodMkcalendar,
		MethodGet,
		MethodHead,
		MethodPut,
		MethodDelete,
	}
}
