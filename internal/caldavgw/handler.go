package caldavgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

const maxConditionalIfHeaderBytes = 8192

type calendarObjectLookupKey struct {
	calendarID string
	objectName string
}

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error)
	LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error)
	LookupCalendarBySlug(ctx context.Context, userID string, slug string) (Calendar, error)
	ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error)
	LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error)
}

type CalendarObjectLimiter interface {
	ListCalendarObjectsLimit(ctx context.Context, userID string, calendarID string, limit int) ([]CalendarObject, error)
}

type CalendarObjectMetadataStore interface {
	LookupCalendarObjectMetadata(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error)
	ListCalendarObjectMetadataLimit(ctx context.Context, userID string, calendarID string, status string, limit int) ([]CalendarObject, error)
}

type CalendarObjectBatchStore interface {
	ListCalendarObjectsByNames(ctx context.Context, userID string, calendarID string, status string, objectNames []string, includeICS bool) ([]CalendarObject, error)
}

type CalendarObjectComponentStore interface {
	ListCalendarObjectsByComponentLimit(ctx context.Context, userID string, calendarID string, status string, component string, limit int, includeICS bool) ([]CalendarObject, error)
}

type CalendarQueryCandidateWalker interface {
	WalkCalendarQueryCandidates(ctx context.Context, userID string, calendarID string, status string, component string, yield func(CalendarObject) (bool, error)) error
}

type CalendarObjectCrossCalendarBatchStore interface {
	ListCalendarObjectsByNameGroups(ctx context.Context, userID string, objectNamesByCalendar map[string][]string, status string, includeICS bool) ([]CalendarObject, error)
}

type CalendarChangeWithObjectStore interface {
	ListCalendarChangesWithObjectsSince(ctx context.Context, req ListChangesSinceRequest, includeICS bool) ([]CalendarChangeWithObject, error)
}

type ObjectStore interface {
	UpsertObject(ctx context.Context, req UpsertObjectRequest) (CalendarObject, error)
	DeleteObject(ctx context.Context, req DeleteObjectRequest) (CalendarObject, error)
}

type CalendarCreator interface {
	CreateCalendarAtPath(ctx context.Context, req CreateCalendarAtPathRequest) (Calendar, error)
}

type CalendarDeleter interface {
	DeleteCalendar(ctx context.Context, req DeleteCalendarRequest) (Calendar, error)
}

type CalendarUpdater interface {
	UpdateCalendarProperties(ctx context.Context, req UpdateCalendarRequest) (Calendar, error)
}

type SyncChangeStore interface {
	ListCalendarChangesSince(ctx context.Context, req ListChangesSinceRequest) ([]CalendarChange, error)
}

type SchedulingStore interface {
	DeliverSchedulingMessage(ctx context.Context, req DeliverSchedulingMessageRequest) (SchedulingMessage, error)
	SendSchedulingMessage(ctx context.Context, req SendSchedulingMessageRequest) (SchedulingMessage, error)
}

type DeliverSchedulingMessageRequest struct {
	UserID     string
	Recipient  string
	Method     ScheduleMethod
	UID        string
	ICSPayload []byte
}

type SendSchedulingMessageRequest struct {
	UserID     string
	Method     ScheduleMethod
	UID        string
	ICSPayload []byte
}

type UserResolver func(*http.Request) (string, error)

const (
	CalendarAccessRoleRead   = "read"
	CalendarAccessRoleWrite  = "write"
	CalendarAccessRoleManage = "manage"
)

type AccessRequest struct {
	ActorUserID  string
	OwnerUserID  string
	Resource     ResourcePath
	RequiredRole string
}

type AccessDecision struct {
	Allowed    bool
	Privileges []XMLName
}

type AccessAuthorizer interface {
	AuthorizeCalendarAccess(ctx context.Context, req AccessRequest) (AccessDecision, error)
}

// gatewayMetrics is the minimal interface caldavgw uses for observability.
// *protocolmetrics.GatewayMetrics satisfies this interface.
type gatewayMetrics interface {
	RecordCommand(userID string, duration time.Duration)
	RecordError(userID string)
}

type Handler struct {
	Store             DiscoveryStore
	ResolveUser       UserResolver
	AccessAuthorizer  AccessAuthorizer
	IncludeScheduling bool
	metrics           gatewayMetrics
}

type InvalidSyncTokenError struct {
	Token string
}

func (e InvalidSyncTokenError) Error() string {
	return "CalDAV sync-token is unknown or expired"
}

type TruncatedResultsError struct {
	Operation string
}

func (e TruncatedResultsError) Error() string {
	operation := strings.TrimSpace(e.Operation)
	if operation == "" {
		operation = "CalDAV request"
	}
	return operation + " would truncate results"
}

func NewHandler(store DiscoveryStore, resolveUser UserResolver) *Handler {
	return &Handler{Store: store, ResolveUser: resolveUser}
}

// SetMetrics sets optional metrics collector for gateway observability
func (h *Handler) SetMetrics(metrics gatewayMetrics) {
	if h == nil {
		return
	}
	h.metrics = metrics
}

// recordCommand records HTTP operation processing with optional metrics
func (h *Handler) recordCommand(userID string, duration time.Duration) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordCommand(userID, duration)
}

// recordError records HTTP operation error with optional metrics
func (h *Handler) recordError(userID string) {
	if h == nil || h.metrics == nil {
		return
	}
	h.metrics.RecordError(userID)
}

type caldavUnauthorizedChallenge interface {
	WWWAuthenticate() string
}

func writeCalDAVUnauthorized(w http.ResponseWriter, err error) {
	authorizeChallenge := calDAVWWWAuthenticate
	if err != nil {
		for e := err; e != nil; e = errors.Unwrap(e) {
			if challenge, ok := e.(caldavUnauthorizedChallenge); ok {
				if value := strings.TrimSpace(challenge.WWWAuthenticate()); value != "" {
					authorizeChallenge = value
				}
				break
			}
		}
		w.Header().Set("WWW-Authenticate", authorizeChallenge)
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	http.Error(w, "unauthorized", http.StatusUnauthorized)
}

func QueryUserResolver(r *http.Request) (string, error) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	if userID == "" {
		return "", fmt.Errorf("user_id is required")
	}
	if strings.ContainsAny(userID, "\r\n") {
		return "", fmt.Errorf("user_id must not contain line breaks")
	}
	return userID, nil
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, "caldav handler is not configured", http.StatusInternalServerError)
		return
	}

	// Extract userID for metrics (skip for well-known paths)
	userID := "unknown"
	if !strings.HasPrefix(r.URL.Path, "/.well-known") {
		resolve := h.ResolveUser
		if resolve == nil {
			resolve = QueryUserResolver
		}
		if id, err := resolve(r); err == nil {
			userID = id
		}
	}

	cmdStart := time.Now()

	if r.URL.Path == WellKnownCalDAVPath {
		h.serveWellKnown(w, r)
		h.recordCommand(userID, time.Since(cmdStart))
		return
	}
	if r.URL.Path == "/.well-known/caldav-timezones" {
		http.Redirect(w, r, "/caldav/timezones/", http.StatusMovedPermanently)
		h.recordCommand(userID, time.Since(cmdStart))
		return
	}
	if strings.HasPrefix(r.URL.Path, "/caldav/timezones/") && (r.Method == MethodGet || r.Method == MethodHead) {
		h.serveTimezone(w, r)
		h.recordCommand(userID, time.Since(cmdStart))
		return
	}
	switch r.Method {
	case MethodOptions:
		h.serveOptions(w)
	case MethodPropfind:
		h.servePropfind(w, r)
	case MethodProppatch:
		h.serveProppatch(w, r)
	case MethodReport:
		h.serveReport(w, r)
	case MethodGet, MethodHead:
		h.serveGetObject(w, r)
	case MethodPut:
		h.servePutObject(w, r)
	case MethodDelete:
		h.serveDeleteObject(w, r)
	case MethodMkcalendar:
		h.serveMkcalendar(w, r)
	case MethodPost:
		h.serveSchedulePost(w, r)
	default:
		w.Header().Set("Allow", calDAVAllowHeader())
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}

	h.recordCommand(userID, time.Since(cmdStart))
}

func (h *Handler) serveWellKnown(w http.ResponseWriter, r *http.Request) {
	target := RootPath + "/"
	if r.URL.RawQuery != "" {
		target += "?" + r.URL.RawQuery
	}
	w.Header().Set("Location", target)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusMovedPermanently)
}

func (h *Handler) serveOptions(w http.ResponseWriter) {
	w.Header().Set("Allow", calDAVAllowHeader())
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.IncludeScheduling, h.includeSyncCollection()), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) includeSyncCollection() bool {
	_, ok := h.Store.(SyncChangeStore)
	return ok
}

func calDAVAllowHeader() string {
	return strings.Join(ImplementedMethods(), ", ")
}
