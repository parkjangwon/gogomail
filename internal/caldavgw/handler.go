package caldavgw

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
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

type Handler struct {
	Store             DiscoveryStore
	ResolveUser       UserResolver
	AccessAuthorizer  AccessAuthorizer
	IncludeScheduling bool
	metrics           interface{} // GatewayMetrics (optional, typed as interface{} to avoid import)
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
func (h *Handler) SetMetrics(metrics interface{}) {
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
	if m, ok := h.metrics.(interface{ RecordCommand(string, time.Duration) }); ok {
		m.RecordCommand(userID, duration)
	}
}

// recordError records HTTP operation error with optional metrics
func (h *Handler) recordError(userID string) {
	if h == nil || h.metrics == nil {
		return
	}
	if m, ok := h.metrics.(interface{ RecordError(string) }); ok {
		m.RecordError(userID)
	}
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

func (h *Handler) serveTimezone(w http.ResponseWriter, r *http.Request) {
	tzid := strings.TrimPrefix(r.URL.Path, "/caldav/timezones/")
	tzid = strings.TrimSuffix(tzid, "/")
	if tzid == "" {
		http.Error(w, "timezone ID is required", http.StatusBadRequest)
		return
	}
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		http.Error(w, "unsupported timezone", http.StatusNotFound)
		return
	}
	vtimezone, err := buildVTIMEZONE(tzid, loc)
	if err != nil {
		http.Error(w, "failed to build timezone data", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.WriteHeader(http.StatusOK)
	if r.Method == MethodHead {
		return
	}
	w.Write(vtimezone)
}

func buildVTIMEZONE(tzid string, loc *time.Location) ([]byte, error) {
	_, offset := time.Now().In(loc).Zone()
	utcOffset := formatUTCOffset(offset)
	now := time.Now().UTC().Format(time.RFC3339)
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\n")
	b.WriteString("VERSION:2.0\r\n")
	b.WriteString("PRODID:-//gogomail//CalDAV Timezone Service//EN\r\n")
	b.WriteString("CALSCALE:GREGORIAN\r\n")
	b.WriteString("METHOD:PUBLISH\r\n")
	b.WriteString("BEGIN:VTIMEZONE\r\n")
	b.WriteString("TZID:")
	b.WriteString(tzid)
	b.WriteString("\r\n")
	b.WriteString("BEGIN:STANDARD\r\n")
	b.WriteString("TZOFFSETFROM:")
	b.WriteString(utcOffset)
	b.WriteString("\r\n")
	b.WriteString("TZOFFSETTO:")
	b.WriteString(utcOffset)
	b.WriteString("\r\n")
	b.WriteString("TZNAME:")
	b.WriteString(tzid)
	b.WriteString("\r\n")
	b.WriteString("DTSTART:19700101T000000Z\r\n")
	b.WriteString("END:STANDARD\r\n")
	b.WriteString("X-LIC-LOCATION:")
	b.WriteString(tzid)
	b.WriteString("\r\n")
	b.WriteString("END:VTIMEZONE\r\n")
	b.WriteString("X-WR-CALDESC:Generated by gogomail CalDAV Timezone Service\r\n")
	b.WriteString("X-PUBLISHED-LL:")
	b.WriteString(now)
	b.WriteString("\r\n")
	b.WriteString("END:VCALENDAR\r\n")
	return []byte(b.String()), nil
}

func formatUTCOffset(seconds int) string {
	abs := seconds
	if abs < 0 {
		abs = -abs
	}
	hours := abs / 3600
	minutes := (abs % 3600) / 60
	sign := "+"
	if seconds < 0 {
		sign = "-"
	}
	return fmt.Sprintf("%s%02d%02d", sign, hours, minutes)
}

func (h *Handler) serveProppatch(w http.ResponseWriter, r *http.Request) {
	userID, resource, actorUserID, _, ok := h.resolveResourceRequest(w, r, CalendarAccessRoleWrite)
	if !ok {
		return
	}
	if resource.Kind != ResourceCalendarCollection {
		http.Error(w, "PROPPATCH requires a calendar collection path", http.StatusForbidden)
		return
	}
	store, ok := h.Store.(CalendarUpdater)
	if !ok {
		http.Error(w, "caldav calendar updater is not configured", http.StatusNotImplemented)
		return
	}
	observedETag, ok := h.checkCalendarCollectionPreconditions(w, r, userID, resource.CalendarID)
	if !ok {
		return
	}
	patch, err := ParseProppatch(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	href, err := CalendarCollectionPath(userID, resource.CalendarID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(patch.Unsupported) > 0 || len(patch.Protected) > 0 {
		body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchFailureResponse(href, patch)})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write(body)
		return
	}
	calendar, err := store.UpdateCalendarProperties(r.Context(), UpdateCalendarRequest{
		UserID:          userID,
		ActorUserID:     actorUserID,
		CalendarID:      resource.CalendarID,
		Name:            patch.Name,
		NameLang:        patch.NameLang,
		Slug:            patch.Slug,
		Timezone:        patch.Timezone,
		Color:           patch.Color,
		Description:     patch.Description,
		DescriptionLang: patch.DescriptionLang,
		ObservedETag:    observedETag,
	})
	if err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "caldav calendar collection precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	href, err = CalendarCollectionPath(userID, calendar.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchResponse(href, calendar, patch.Properties)})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(body)
}

func (h *Handler) serveMkcalendar(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return
	}
	store, ok := h.Store.(CalendarCreator)
	if !ok {
		http.Error(w, "caldav calendar creator is not configured", http.StatusNotImplemented)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return
	}
	actorUserID := userID
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil || resource.Kind != ResourceCalendarCollection {
		http.Error(w, "MKCALENDAR requires a calendar collection path", http.StatusConflict)
		return
	}
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleManage)
	if !ok {
		return
	}
	userID = ownerID
	if _, err := h.Store.LookupPrincipal(r.Context(), userID); err != nil {
		http.Error(w, "caldav calendar home not found", http.StatusConflict)
		return
	}
	calendar, err := h.lookupCalendar(r.Context(), userID, resource.CalendarID)
	if err == nil {
		if !h.checkCalendarCollectionCreatePreconditions(w, r, userID, calendar, true) {
			return
		}
		http.Error(w, "caldav calendar already exists", http.StatusMethodNotAllowed)
		return
	}
	if !h.checkCalendarCollectionCreatePreconditions(w, r, userID, Calendar{}, false) {
		return
	}
	if ok := validateDAVXMLContentType(w, r, "MKCALENDAR"); !ok {
		return
	}
	req, err := ParseMKCalendar(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	invalidProperties := invalidMKCalendarProperties(req)
	if len(req.Unsupported) > 0 || len(invalidProperties) > 0 {
		body, err := BuildMKCalendarResponseXML(mkcalendarFailurePropStats(req, invalidProperties))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusMultiStatus)
		_, _ = w.Write(body)
		return
	}
	var calendarID string
	var slug *string
	if _, err := ValidateCalendarPathID(resource.CalendarID); err != nil {
		calendarID = uuid.NewString()
		slugStr := resource.CalendarID
		slug = &slugStr
	} else {
		calendarID = resource.CalendarID
	}
	if req.Slug != nil && *req.Slug != "" {
		slug = req.Slug
	}
	calendar, err = store.CreateCalendarAtPath(r.Context(), CreateCalendarAtPathRequest{
		UserID:          userID,
		ActorUserID:     actorUserID,
		CalendarID:      calendarID,
		Name:            req.DisplayName,
		NameLang:        req.DisplayNameLang,
		Slug:            slug,
		Timezone:        req.Timezone,
		Color:           req.Color,
		Description:     req.Description,
		DescriptionLang: req.DescriptionLang,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	location, err := CalendarCollectionPath(userID, calendar.ID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Location", location)
	w.Header().Set("Cache-Control", "no-store, no-cache")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusCreated)
}

func validateDAVXMLContentType(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.ContentLength == 0 {
		return true
	}
	values := r.Header.Values("Content-Type")
	if len(values) == 0 {
		return true
	}
	if len(values) > 1 {
		http.Error(w, method+" Content-Type must be specified at most once", http.StatusBadRequest)
		return false
	}
	value := strings.TrimSpace(values[0])
	if strings.ContainsAny(value, "\r\n") {
		http.Error(w, method+" Content-Type must not contain line breaks", http.StatusBadRequest)
		return false
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		http.Error(w, method+" Content-Type is invalid", http.StatusBadRequest)
		return false
	}
	if !isXMLMediaType(mediaType) {
		http.Error(w, method+" Content-Type must be application/xml or text/xml", http.StatusUnsupportedMediaType)
		return false
	}
	return true
}

func isXMLMediaType(mediaType string) bool {
	mediaType = strings.ToLower(strings.TrimSpace(mediaType))
	return mediaType == "application/xml" || mediaType == "text/xml" || strings.HasSuffix(mediaType, "+xml")
}

func invalidMKCalendarProperties(req MKCalendarRequest) []XMLName {
	var invalid []XMLName
	for _, property := range req.Properties {
		switch property {
		case PropDisplayName:
			if strings.TrimSpace(req.DisplayName) != "" {
				if _, err := ValidateCalendarName(req.DisplayName); err != nil {
					invalid = append(invalid, PropDisplayName)
				}
			}
		case PropCalendarDescription:
			if _, err := ValidateCalendarDescription(req.Description); err != nil {
				invalid = append(invalid, PropCalendarDescription)
			}
		case PropCalendarColor:
			if _, err := ValidateCalendarColor(req.Color); err != nil {
				invalid = append(invalid, PropCalendarColor)
			}
		case PropCalendarSlug:
			if req.Slug != nil {
				if _, err := NormalizeSlug(*req.Slug); err != nil {
					invalid = append(invalid, PropCalendarSlug)
				}
			}
		case PropCalendarTimezone:
			if req.Timezone != nil {
				if _, err := NormalizeTimezone(*req.Timezone); err != nil {
					invalid = append(invalid, PropCalendarTimezone)
				}
			}
		}
	}
	return invalid
}

func mkcalendarFailurePropStats(req MKCalendarRequest, invalidProperties []XMLName) []PropStatus {
	stats := make([]PropStatus, 0, 3)
	failed := make(map[XMLName]struct{}, len(req.Unsupported)+len(invalidProperties))
	if len(req.Unsupported) > 0 {
		unsupported := make([]PropertyResult, 0, len(req.Unsupported))
		for _, name := range req.Unsupported {
			unsupported = append(unsupported, PropertyResult{Name: name})
			failed[name] = struct{}{}
		}
		sortPropertyResults(unsupported)
		stats = append(stats, PropStatus{StatusCode: http.StatusForbidden, Properties: unsupported})
	}
	if len(invalidProperties) > 0 {
		invalid := make([]PropertyResult, 0, len(invalidProperties))
		status := PropStatus{StatusCode: http.StatusConflict}
		for _, name := range invalidProperties {
			invalid = append(invalid, PropertyResult{Name: name})
			failed[name] = struct{}{}
			if name == PropCalendarTimezone {
				status.Error = XMLName{Space: CalDAVNamespace, Local: "valid-calendar-data"}
				status.ResponseDescription = "calendar-timezone is not supported by this server"
			}
		}
		sortPropertyResults(invalid)
		status.Properties = invalid
		stats = append(stats, status)
	}
	dependencies := make([]PropertyResult, 0, len(req.Properties))
	for _, name := range req.Properties {
		if _, ok := failed[name]; ok {
			continue
		}
		dependencies = append(dependencies, PropertyResult{Name: name})
	}
	if len(dependencies) > 0 {
		sortPropertyResults(dependencies)
		stats = append(stats, PropStatus{StatusCode: http.StatusFailedDependency, Properties: dependencies})
	}
	return stats
}

func (h *Handler) serveGetObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, _, ok := h.resolveObjectRequest(w, r, CalendarAccessRoleRead)
	if !ok {
		return
	}
	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ifModifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Modified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	useMetadata := r.Method == MethodHead || ifMatch != "" || ifNoneMatch != "" || ifHeader != "" || ifModifiedSince != "" || ifUnmodifiedSince != ""
	if !useMetadata {
		object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			http.Error(w, "caldav object not found", http.StatusNotFound)
			return
		}
		writeCalendarObjectHeaders(w, object)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(object.ICS)
		return
	}

	object, err := h.lookupCalendarObjectForReport(r.Context(), userID, resource.CalendarID, resource.ObjectName, false)
	if err != nil {
		http.Error(w, "caldav object not found", http.StatusNotFound)
		return
	}
	if ifMatch != "" {
		ifMatchResult, err := ifMatchMatches(ifMatch, object.ETag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !ifMatchResult {
			http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
			return
		}
	}
	if ifHeader != "" {
		matches, err := webDAVIfHeaderMatches(ifHeader, object.ETag, r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !matches {
			http.Error(w, "caldav object If header precondition failed", http.StatusPreconditionFailed)
			return
		}
	}
	if ifNoneMatch != "" {
		ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, object.ETag, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ifNoneMatchResult {
			writeCalendarObjectNotModifiedHeaders(w, object)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if ifUnmodifiedSince != "" && objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
		http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	if ifNoneMatch == "" && ifModifiedSince != "" && objectNotModifiedSince(ifModifiedSince, object.UpdatedAt) {
		writeCalendarObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if r.Method == MethodHead {
		writeCalendarObjectHeaders(w, object)
		w.WriteHeader(http.StatusOK)
		return
	}
	object, err = h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
	if err != nil {
		http.Error(w, "caldav object not found", http.StatusNotFound)
		return
	}
	writeCalendarObjectHeaders(w, object)
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(object.ICS)
}

func (h *Handler) servePutObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, actorUserID, ok := h.resolveObjectRequest(w, r, CalendarAccessRoleWrite)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "caldav object store is not configured", http.StatusNotImplemented)
		return
	}
	if err := validateCalendarPutContentTypeHeader(r.Header); err != nil {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}

	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	needsPreconditionMetadata := ifMatch != "" || ifNoneMatch != "" || ifHeader != "" || ifUnmodifiedSince != ""

	var existing CalendarObject
	var lookupErr error
	existed := false
	if needsPreconditionMetadata {
		existing, lookupErr, existed = h.lookupCalendarObjectMetadataForWrite(r.Context(), userID, resource.CalendarID, resource.ObjectName)
		if lookupErr != nil {
			existing = CalendarObject{}
		}
		if ifMatch == "" && ifUnmodifiedSince == "" {
			// If-None-Match-only exists. If object exists, ensure it is not rejected by matching etag.
			if existed && ifNoneMatch != "" {
				ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, existing.ETag, false)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if ifNoneMatchResult {
					http.Error(w, "caldav object already exists", http.StatusPreconditionFailed)
					return
				}
			}
			if ifHeader != "" {
				matches, err := webDAVIfHeaderMatches(ifHeader, existing.ETag, r.URL.Path)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !matches {
					http.Error(w, "caldav object If header precondition failed", http.StatusPreconditionFailed)
					return
				}
			}
		} else {
			if !existed {
				http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
				return
			}
		}
		if existed {
			if ifNoneMatch != "" {
				ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, existing.ETag, false)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if ifNoneMatchResult {
					http.Error(w, "caldav object already exists", http.StatusPreconditionFailed)
					return
				}
			}
			if ifMatch != "" {
				ifMatchResult, err := ifMatchMatches(ifMatch, existing.ETag)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !ifMatchResult {
					http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
					return
				}
			}
			if ifHeader != "" {
				matches, err := webDAVIfHeaderMatches(ifHeader, existing.ETag, r.URL.Path)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !matches {
					http.Error(w, "caldav object If header precondition failed", http.StatusPreconditionFailed)
					return
				}
			}
			if ifUnmodifiedSince != "" && objectModifiedSince(ifUnmodifiedSince, existing.UpdatedAt) {
				http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
				return
			}
		}
	}

	observedETag := ""
	if ifMatch != "" || (ifHeader != "" && existed) {
		if !existed {
			http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
			return
		}
		observedETag = existing.ETag
	}
	body, err := readBoundedCalendarBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}

	object, err := store.UpsertObject(r.Context(), UpsertObjectRequest{
		UserID:       userID,
		ActorUserID:  actorUserID,
		CalendarID:   resource.CalendarID,
		ObjectName:   resource.ObjectName,
		ICS:          body,
		ObservedETag: observedETag,
	})
	if err != nil {
		if needsPreconditionMetadata && (strings.Contains(err.Error(), "CalDAV object not found") || strings.Contains(err.Error(), "CalDAV object etag mismatch")) {
			http.Error(w, err.Error(), http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeCalendarObjectHeaders(w, object)
	if needsPreconditionMetadata {
		if existed {
			w.WriteHeader(http.StatusNoContent)
		} else {
			w.WriteHeader(http.StatusCreated)
		}
		return
	}

	if !object.CreatedAt.Equal(object.UpdatedAt) {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

func (h *Handler) serveDeleteObject(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return
	}
	actorUserID := userID
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if resource.Kind == ResourceCalendarCollection {
		ownerID, _, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleManage)
		if !ok {
			return
		}
		userID = ownerID
		h.deleteCalendarCollection(w, r, userID, actorUserID, resource)
		return
	}
	if resource.Kind != ResourceCalendarObject {
		http.Error(w, "DELETE requires a calendar collection or object path", http.StatusForbidden)
		return
	}
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleWrite)
	if !ok {
		return
	}
	userID = ownerID
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "caldav object store is not configured", http.StatusNotImplemented)
		return
	}
	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	observedETag := ""
	if ifMatch != "" || ifNoneMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
		object, lookupErr, exists := h.lookupCalendarObjectMetadataForWrite(r.Context(), userID, resource.CalendarID, resource.ObjectName)
		if lookupErr != nil || !exists {
			if ifMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
				http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
				return
			}
		} else {
			if ifNoneMatch != "" {
				ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, object.ETag, false)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if ifNoneMatchResult {
					http.Error(w, "caldav object if-none-match precondition failed", http.StatusPreconditionFailed)
					return
				}
			}
			if ifMatch != "" {
				ifMatchResult, err := ifMatchMatches(ifMatch, object.ETag)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !ifMatchResult {
					http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
					return
				}
			}
			if ifHeader != "" {
				matches, err := webDAVIfHeaderMatches(ifHeader, object.ETag, r.URL.Path)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return
				}
				if !matches {
					http.Error(w, "caldav object If header precondition failed", http.StatusPreconditionFailed)
					return
				}
			}
			if ifMatch != "" || ifHeader != "" {
				observedETag = object.ETag
			}
			if objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
				http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
				return
			}
		}
	}
	if _, err := store.DeleteObject(r.Context(), DeleteObjectRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: resource.CalendarID, ObjectName: resource.ObjectName, ObservedETag: observedETag}); err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "caldav object precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) lookupCalendarObjectMetadataForWrite(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error, bool) {
	if h == nil || h.Store == nil {
		return CalendarObject{}, nil, false
	}
	if metadataStore, ok := h.Store.(CalendarObjectMetadataStore); ok {
		object, err := metadataStore.LookupCalendarObjectMetadata(ctx, userID, calendarID, objectName)
		if err != nil {
			return CalendarObject{}, err, false
		}
		return object, nil, true
	}
	if discoveryStore, ok := h.Store.(DiscoveryStore); ok {
		object, err := discoveryStore.LookupCalendarObject(ctx, userID, calendarID, objectName)
		if err != nil {
			return CalendarObject{}, err, false
		}
		return object, nil, true
	}
	return CalendarObject{}, nil, false
}

func (h *Handler) deleteCalendarCollection(w http.ResponseWriter, r *http.Request, userID string, actorUserID string, resource ResourcePath) {
	store, ok := h.Store.(CalendarDeleter)
	if !ok {
		http.Error(w, "caldav calendar deleter is not configured", http.StatusNotImplemented)
		return
	}
	observedETag, ok := h.checkCalendarCollectionPreconditions(w, r, userID, resource.CalendarID)
	if !ok {
		return
	}
	if _, err := store.DeleteCalendar(r.Context(), DeleteCalendarRequest{UserID: userID, ActorUserID: actorUserID, CalendarID: resource.CalendarID, ObservedETag: observedETag}); err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "caldav calendar collection precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) lookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error) {
	calendar, err := h.Store.LookupCalendar(ctx, userID, calendarID)
	if err == nil {
		return calendar, nil
	}
	if strings.Contains(err.Error(), "not found") {
		return h.Store.LookupCalendarBySlug(ctx, userID, calendarID)
	}
	return Calendar{}, err
}

func (h *Handler) checkCalendarCollectionPreconditions(w http.ResponseWriter, r *http.Request, userID string, calendarID string) (string, bool) {
	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return "", false
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return "", false
	}
	if ifMatch != "" || ifNoneMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
		calendar, err := h.lookupCalendar(r.Context(), userID, calendarID)
		if err != nil {
			if ifMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
				http.Error(w, "caldav calendar not found", http.StatusPreconditionFailed)
				return "", false
			}
			return "", true
		}
		etag, err := CalendarCollectionETag(userID, calendar)
		if err != nil {
			http.Error(w, "caldav calendar collection etag unavailable", http.StatusPreconditionFailed)
			return "", false
		}
		if ifMatch != "" || ifNoneMatch != "" {
			if ifNoneMatch != "" {
				ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, etag, false)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return "", false
				}
				if ifNoneMatchResult {
					http.Error(w, "caldav calendar collection if-none-match precondition failed", http.StatusPreconditionFailed)
					return "", false
				}
			}
			if ifMatch != "" {
				ifMatchResult, err := ifMatchMatches(ifMatch, etag)
				if err != nil {
					http.Error(w, err.Error(), http.StatusBadRequest)
					return "", false
				}
				if !ifMatchResult {
					http.Error(w, "caldav calendar collection etag mismatch", http.StatusPreconditionFailed)
					return "", false
				}
			}
		}
		if ifHeader != "" {
			matches, err := webDAVIfHeaderMatches(ifHeader, etag, r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return "", false
			}
			if !matches {
				http.Error(w, "caldav calendar collection If header precondition failed", http.StatusPreconditionFailed)
				return "", false
			}
		}
		if objectModifiedSince(ifUnmodifiedSince, calendar.UpdatedAt) {
			http.Error(w, "caldav calendar modified since precondition", http.StatusPreconditionFailed)
			return "", false
		}
		return etag, true
	}
	return "", true
}

func (h *Handler) checkCalendarCollectionCreatePreconditions(w http.ResponseWriter, r *http.Request, userID string, calendar Calendar, exists bool) bool {
	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	if !exists {
		if ifHeader != "" {
			matches, err := webDAVIfHeaderMatches(ifHeader, "", r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if !matches {
				http.Error(w, "caldav calendar collection If header precondition failed", http.StatusPreconditionFailed)
				return false
			}
		}
		if ifMatch != "" || ifUnmodifiedSince != "" {
			http.Error(w, "caldav calendar create precondition failed", http.StatusPreconditionFailed)
			return false
		}
		return true
	}
	if ifMatch != "" || ifNoneMatch != "" || ifHeader != "" {
		etag, err := CalendarCollectionETag(userID, calendar)
		if err != nil {
			http.Error(w, "caldav calendar collection etag unavailable", http.StatusPreconditionFailed)
			return false
		}
		if ifMatch != "" {
			ifMatchResult, err := ifMatchMatches(ifMatch, etag)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if !ifMatchResult {
				http.Error(w, "caldav calendar collection etag mismatch", http.StatusPreconditionFailed)
				return false
			}
		}
		if ifNoneMatch != "" {
			ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, etag, false)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if ifNoneMatchResult {
				http.Error(w, "caldav calendar collection if-none-match precondition failed", http.StatusPreconditionFailed)
				return false
			}
		}
		if ifHeader != "" {
			matches, err := webDAVIfHeaderMatches(ifHeader, etag, r.URL.Path)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if !matches {
				http.Error(w, "caldav calendar collection If header precondition failed", http.StatusPreconditionFailed)
				return false
			}
		}
	}
	if objectModifiedSince(ifUnmodifiedSince, calendar.UpdatedAt) {
		http.Error(w, "caldav calendar modified since precondition", http.StatusPreconditionFailed)
		return false
	}
	return true
}

func (h *Handler) resolveObjectRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, string, bool) {
	userID, resource, actorUserID, _, ok := h.resolveResourceRequest(w, r, requiredRole)
	if !ok {
		return "", ResourcePath{}, "", false
	}
	if resource.Kind != ResourceCalendarObject {
		http.Error(w, "caldav object path is required", http.StatusNotFound)
		return "", ResourcePath{}, "", false
	}
	return userID, resource, actorUserID, true
}

func (h *Handler) resolveResourceRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, string, AccessDecision, bool) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return "", ResourcePath{}, "", AccessDecision{}, false
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return "", ResourcePath{}, "", AccessDecision{}, false
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return "", ResourcePath{}, "", AccessDecision{}, false
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, requiredRole)
	if !ok {
		return "", ResourcePath{}, "", AccessDecision{}, false
	}
	return ownerID, resource, userID, decision, true
}

func (h *Handler) authorizeResource(w http.ResponseWriter, r *http.Request, actorID string, resource ResourcePath, requiredRole string) (string, AccessDecision, bool) {
	ownerID := strings.TrimSpace(resource.UserID)
	if ownerID == "" {
		return actorID, AccessDecision{Allowed: true}, true
	}
	if ownerID == actorID {
		return ownerID, AccessDecision{Allowed: true}, true
	}
	if h.AccessAuthorizer == nil {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return "", AccessDecision{}, false
	}
	decision, err := h.AccessAuthorizer.AuthorizeCalendarAccess(r.Context(), AccessRequest{
		ActorUserID:  actorID,
		OwnerUserID:  ownerID,
		Resource:     resource,
		RequiredRole: requiredRole,
	})
	if err != nil {
		http.Error(w, "caldav access policy unavailable", http.StatusInternalServerError)
		return "", AccessDecision{}, false
	}
	if !decision.Allowed {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return "", AccessDecision{}, false
	}
	return ownerID, decision, true
}

func writeCalendarObjectHeaders(w http.ResponseWriter, object CalendarObject) {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", object.ETag)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	if !object.UpdatedAt.IsZero() {
		w.Header().Set("Last-Modified", formatHTTPDate(object.UpdatedAt))
	}
}

func writeCalendarObjectNotModifiedHeaders(w http.ResponseWriter, object CalendarObject) {
	w.Header().Set("ETag", object.ETag)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if !object.UpdatedAt.IsZero() {
		w.Header().Set("Last-Modified", formatHTTPDate(object.UpdatedAt))
	}
}

func conditionalHeaderValue(header http.Header, name string) string {
	return strings.TrimSpace(strings.Join(header.Values(name), ","))
}

func conditionalIfHeaderValue(header http.Header) (string, error) {
	values := header.Values("If")
	if len(values) == 0 {
		return "", nil
	}
	totalLen := 0
	trimmed := make([]string, len(values))
	for i, value := range values {
		value = strings.TrimSpace(value)
		if strings.ContainsAny(value, "\r\n") {
			return "", fmt.Errorf("If header must not contain line breaks")
		}
		if i > 0 {
			totalLen++
		}
		totalLen += len(value)
		if totalLen > maxConditionalIfHeaderBytes {
			return "", fmt.Errorf("If header is too large")
		}
		trimmed[i] = value
	}
	value := strings.TrimSpace(strings.Join(trimmed, " "))
	return value, nil
}

func conditionalDateHeaderValue(header http.Header, name string) (string, error) {
	values := header.Values(name)
	if len(values) > 1 {
		return "", fmt.Errorf("%s header must not be repeated", name)
	}
	if len(values) == 0 {
		return "", nil
	}
	value := strings.TrimSpace(values[0])
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s header must not contain line breaks", name)
	}
	return value, nil
}

func webDAVIfHeaderMatches(header string, currentETag string, currentPath string) (bool, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return true, nil
	}
	anyRelevant := false
	anyMatch := false
	for pos := 0; pos < len(header); {
		open := strings.IndexByte(header[pos:], '(')
		if open < 0 {
			if strings.TrimSpace(header[pos:]) != "" {
				return false, fmt.Errorf("If header contains trailing data")
			}
			break
		}
		open += pos
		prefix := strings.TrimSpace(header[pos:open])
		tag := ""
		if prefix != "" {
			if !strings.HasPrefix(prefix, "<") || !strings.HasSuffix(prefix, ">") {
				return false, fmt.Errorf("If header contains a malformed resource tag")
			}
			tag = strings.TrimSpace(prefix[1 : len(prefix)-1])
			if tag == "" || strings.ContainsAny(tag, "<>") {
				return false, fmt.Errorf("If header contains a malformed resource tag")
			}
		}
		close := strings.IndexByte(header[open+1:], ')')
		if close < 0 {
			return false, fmt.Errorf("If header contains an unterminated condition list")
		}
		close += open + 1
		pos = close + 1
		matches, err := webDAVIfConditionListMatches(header[open+1:close], currentETag)
		if err != nil {
			return false, err
		}
		if tag != "" && !webDAVIfTagMatchesPath(tag, currentPath) {
			continue
		}
		anyRelevant = true
		if matches {
			anyMatch = true
		}
	}
	if !anyRelevant {
		return false, nil
	}
	return anyMatch, nil
}

func webDAVIfTagMatchesPath(tag string, currentPath string) bool {
	tag = strings.TrimSpace(tag)
	currentPath = strings.TrimSpace(currentPath)
	if tag == currentPath {
		return true
	}
	if strings.HasPrefix(tag, "http://") || strings.HasPrefix(tag, "https://") {
		if idx := strings.Index(tag[strings.Index(tag, "://")+3:], "/"); idx >= 0 {
			path := tag[strings.Index(tag, "://")+3+idx:]
			return path == currentPath
		}
	}
	return false
}

func webDAVIfConditionListMatches(list string, currentETag string) (bool, error) {
	list = strings.TrimSpace(list)
	if list == "" {
		return false, fmt.Errorf("If header contains an empty condition list")
	}
	for list != "" {
		negated := false
		if strings.HasPrefix(list, "Not") && (len(list) == 3 || list[3] == ' ' || list[3] == '\t') {
			negated = true
			list = strings.TrimSpace(list[3:])
		}
		var matched bool
		switch {
		case strings.HasPrefix(list, "["):
			end := strings.IndexByte(list, ']')
			if end < 0 {
				return false, fmt.Errorf("If header contains an unterminated entity-tag")
			}
			matched = strings.TrimSpace(list[1:end]) == currentETag
			list = strings.TrimSpace(list[end+1:])
		case strings.HasPrefix(list, "<"):
			end := strings.IndexByte(list, '>')
			if end < 0 {
				return false, fmt.Errorf("If header contains an unterminated state-token")
			}
			matched = false
			list = strings.TrimSpace(list[end+1:])
		default:
			return false, fmt.Errorf("If header contains an unsupported condition")
		}
		if negated {
			matched = !matched
		}
		if !matched {
			return false, nil
		}
	}
	return true, nil
}

func ifNoneMatchMatches(header string, etag string, allowWeak bool) (bool, error) {
	candidates, wildcard, err := parseHTTPEntityTagList(header, "If-None-Match")
	if err != nil {
		return false, err
	}
	if wildcard {
		return true, nil
	}
	current, err := parseEntityTag(etag)
	if err != nil {
		return false, nil
	}
	for _, candidate := range candidates {
		if entityTagsMatch(current, candidate, allowWeak) {
			return true, nil
		}
	}
	return false, nil
}

func ifMatchMatches(header string, etag string) (bool, error) {
	candidates, wildcard, err := parseHTTPEntityTagList(header, "If-Match")
	if err != nil {
		return false, err
	}
	if wildcard {
		return true, nil
	}
	current, err := parseEntityTag(etag)
	if err != nil {
		return false, nil
	}
	for _, candidate := range candidates {
		if entityTagsMatch(current, candidate, false) {
			return true, nil
		}
	}
	return false, nil
}

func entityTagsMatch(current entityTag, candidate entityTag, allowWeak bool) bool {
	if !allowWeak && (current.weak || candidate.weak) {
		return false
	}
	return current.value == candidate.value
}

type entityTag struct {
	value string
	weak  bool
}

func parseHTTPEntityTagList(header string, headerName string) ([]entityTag, bool, error) {
	header = strings.TrimSpace(header)
	if header == "" {
		return nil, false, nil
	}
	if strings.ContainsAny(header, "\r\n") {
		return nil, false, fmt.Errorf("%s header must not contain line breaks", headerName)
	}
	parts, err := splitHTTPList(header)
	if err != nil {
		return nil, false, err
	}
	if len(parts) == 1 && strings.TrimSpace(parts[0]) == "*" {
		return nil, true, nil
	}
	tags := make([]entityTag, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "*" {
			return nil, false, fmt.Errorf("%s header contains an invalid entity-tag", headerName)
		}
		tag, err := parseEntityTag(part)
		if err != nil {
			return nil, false, fmt.Errorf("%s header contains an invalid entity-tag", headerName)
		}
		tags = append(tags, tag)
	}
	if len(tags) == 0 {
		return nil, false, nil
	}
	return tags, false, nil
}

func parseEntityTag(value string) (entityTag, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return entityTag{}, fmt.Errorf("entity-tag is empty")
	}
	weak := false
	if strings.HasPrefix(value, "W/") {
		weak = true
		value = strings.TrimSpace(value[2:])
		if value == "" {
			return entityTag{}, fmt.Errorf("entity-tag is weak with missing value")
		}
	}
	if len(value) < 2 || value[0] != '"' || value[len(value)-1] != '"' {
		return entityTag{}, fmt.Errorf("entity-tag must be quoted")
	}
	raw := value[1 : len(value)-1]
	for i := 0; i < len(raw); i++ {
		if raw[i] == '"' {
			return entityTag{}, fmt.Errorf("entity-tag contains quote")
		}
		if raw[i] == '\r' || raw[i] == '\n' {
			return entityTag{}, fmt.Errorf("entity-tag contains line break")
		}
		if raw[i] == '\\' {
			i++
			if i >= len(raw) {
				return entityTag{}, fmt.Errorf("entity-tag contains invalid escaped character")
			}
			if raw[i] != '\\' && raw[i] != '"' {
				return entityTag{}, fmt.Errorf("entity-tag contains invalid escaped character")
			}
		}
	}
	return entityTag{value: raw, weak: weak}, nil
}

func splitHTTPList(value string) ([]string, error) {
	parts := make([]string, 0, 4)
	start := 0
	inQuote := false
	escaped := false
	for i := 0; i < len(value); i++ {
		c := value[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '"' {
			inQuote = !inQuote
			continue
		}
		if c == ',' && !inQuote {
			parts = append(parts, value[start:i])
			start = i + 1
		}
	}
	if escaped {
		return nil, fmt.Errorf("entity-tag header is invalid")
	}
	if inQuote {
		return nil, fmt.Errorf("entity-tag header is invalid")
	}
	parts = append(parts, value[start:])
	return parts, nil
}

func objectNotModifiedSince(header string, updatedAt time.Time) bool {
	header = strings.TrimSpace(header)
	if header == "" || updatedAt.IsZero() || strings.ContainsAny(header, "\r\n") {
		return false
	}
	since, err := http.ParseTime(header)
	if err != nil {
		return false
	}
	lastModified := updatedAt.UTC().Truncate(time.Second)
	return !lastModified.After(since.UTC())
}

func objectModifiedSince(header string, updatedAt time.Time) bool {
	header = strings.TrimSpace(header)
	if header == "" || updatedAt.IsZero() || strings.ContainsAny(header, "\r\n") {
		return false
	}
	since, err := http.ParseTime(header)
	if err != nil {
		return false
	}
	lastModified := updatedAt.UTC().Truncate(time.Second)
	return lastModified.After(since.UTC())
}

func validateCalendarPutContentType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("calendar object content type must not contain line breaks")
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil {
		return fmt.Errorf("calendar object content type is invalid")
	}
	if !strings.EqualFold(mediaType, "text/calendar") {
		return fmt.Errorf("calendar object content type must be text/calendar")
	}
	if version, ok := params["version"]; ok && strings.TrimSpace(version) != "2.0" {
		return fmt.Errorf("calendar object content type version must be 2.0")
	}
	return nil
}

func validateCalendarPutContentTypeHeader(header http.Header) error {
	values := header.Values("Content-Type")
	if len(values) > 1 {
		return fmt.Errorf("calendar object content type must be specified at most once")
	}
	if len(values) == 0 {
		return nil
	}
	return validateCalendarPutContentType(values[0])
}

func readBoundedCalendarBody(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("calendar body is required")
	}
	limited := io.LimitReader(r, MaxCalendarObjectBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read calendar body: %w", err)
	}
	if len(body) > MaxCalendarObjectBytes {
		return nil, fmt.Errorf("calendar body exceeds %d bytes", MaxCalendarObjectBytes)
	}
	return body, nil
}

func (h *Handler) serveReport(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleRead)
	if !ok {
		return
	}
	userID = ownerID
	depth, err := parseDepthHeader(r.Header, DepthZero)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if depth == DepthInfinity {
		http.Error(w, "Depth: infinity is not supported for CalDAV REPORT", http.StatusForbidden)
		return
	}
	report, err := ParseReport(r.Body)
	if err != nil {
		var unsupportedFilter UnsupportedCalendarFilterError
		if errors.As(err, &unsupportedFilter) {
			writeCalDAVPreconditionError(w, http.StatusForbidden, "supported-filter", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if report.Kind == ReportFreeBusyQuery {
		h.serveFreeBusyReport(w, r, userID, resource, report, depth)
		return
	}
	var body []byte
	var responses []MultiStatusResponse
	if report.Kind == ReportSyncCollection {
		if depth != DepthZero {
			http.Error(w, "sync-collection requires Depth: 0", http.StatusBadRequest)
			return
		}
		var syncToken string
		responses, syncToken, err = h.syncCollectionReport(r.Context(), userID, resource, report, decision.Privileges)
		if err != nil {
			var invalidSyncToken InvalidSyncTokenError
			if errors.As(err, &invalidSyncToken) {
				writeDAVPreconditionError(w, http.StatusForbidden, "valid-sync-token", err.Error())
				return
			}
			var truncated TruncatedResultsError
			if errors.As(err, &truncated) {
				body, _ = BuildSyncCollectionTruncatedXML()
				w.Header().Set("Content-Type", "application/xml; charset=utf-8")
				w.Header().Set("Cache-Control", "no-store")
				w.WriteHeader(http.StatusForbidden)
				_, _ = w.Write(body)
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, err = BuildSyncCollectionXML(responses, syncToken)
	} else {
		responses, err = h.reportResponses(r.Context(), userID, resource, depth, report, decision.Privileges)
		if err != nil {
			var invalidSyncToken InvalidSyncTokenError
			if errors.As(err, &invalidSyncToken) {
				writeDAVPreconditionError(w, http.StatusForbidden, "valid-sync-token", err.Error())
				return
			}
			var unsupportedFilter UnsupportedCalendarFilterError
			if errors.As(err, &unsupportedFilter) {
				writeCalDAVPreconditionError(w, http.StatusForbidden, "supported-filter", err.Error())
				return
			}
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, err = BuildMultiStatusXML(responses)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(body)
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

func (h *Handler) servePropfind(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleRead)
	if !ok {
		return
	}
	depth, err := parseDepthHeader(r.Header, DepthInfinity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if depth == DepthInfinity {
		http.Error(w, "Depth: infinity is not supported for CalDAV discovery", http.StatusForbidden)
		return
	}
	propfind, err := ParsePropfind(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	responses, err := h.propfindResponses(r.Context(), ownerID, userID, resource, depth, propfind, decision.Privileges)
	if err != nil {
		var truncated TruncatedResultsError
		if errors.As(err, &truncated) {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	body, err := BuildMultiStatusXML(responses)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(body)
}

func (h *Handler) serveFreeBusyReport(w http.ResponseWriter, r *http.Request, userID string, resource ResourcePath, report ReportRequest, depth Depth) {
	if resource.Kind != ResourceCalendarCollection {
		http.Error(w, "free-busy-query requires a calendar collection resource", http.StatusForbidden)
		return
	}
	if report.TimeRange == nil {
		http.Error(w, "free-busy-query requires a time-range", http.StatusBadRequest)
		return
	}
	body, err := h.freeBusyCalendar(r.Context(), userID, resource, *report.TimeRange, depth == DepthOne, report.Limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(body)
}

func (h *Handler) serveSchedulePost(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return
	}
	if !h.IncludeScheduling {
		http.Error(w, "caldav scheduling is not enabled", http.StatusForbidden)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCalDAVUnauthorized(w, err)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	switch resource.Kind {
	case ResourceInbox:
		h.serveScheduleDeliver(w, r, userID, resource)
	case ResourceOutbox:
		h.serveScheduleSend(w, r, userID, resource)
	default:
		http.Error(w, "POST is not allowed for this resource", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) serveScheduleDeliver(w http.ResponseWriter, r *http.Request, userID string, resource ResourcePath) {
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleWrite)
	if !ok {
		return
	}
	userID = ownerID
	store, ok := h.Store.(SchedulingStore)
	if !ok {
		http.Error(w, "caldav scheduling store is not configured", http.StatusNotImplemented)
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		http.Error(w, "Content-Type header is required for scheduling", http.StatusBadRequest)
		return
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (!strings.EqualFold(mediaType, "text/calendar") && !strings.EqualFold(mediaType, "message/rfc822") && !strings.EqualFold(mediaType, "application/icalendar+json")) {
		http.Error(w, "Content-Type must be text/calendar or message/rfc822 for scheduling", http.StatusUnsupportedMediaType)
		return
	}
	body, err := readBoundedCalendarBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	parsed, err := ParseICalendarObjectForScheduling(body)
	if err != nil {
		http.Error(w, "invalid iCalendar object: "+err.Error(), http.StatusBadRequest)
		return
	}
	methodStr, err := ExtractICSMethod(body)
	if err != nil {
		http.Error(w, "invalid iCalendar method: "+err.Error(), http.StatusBadRequest)
		return
	}
	method := ScheduleMethod(strings.ToUpper(strings.TrimSpace(methodStr)))
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethod(method) {
		http.Error(w, fmt.Sprintf("invalid iTIP method: %s", method), http.StatusBadRequest)
		return
	}
	msg, err := store.DeliverSchedulingMessage(r.Context(), DeliverSchedulingMessageRequest{
		UserID:     userID,
		Recipient:  userID,
		Method:     method,
		UID:        parsed.UID,
		ICSPayload: body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if msg.ResponseCode != "" {
		w.Header().Set("Content-Type", "message/rfc822; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(msg.ICSPayload)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) serveScheduleSend(w http.ResponseWriter, r *http.Request, userID string, resource ResourcePath) {
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, CalendarAccessRoleWrite)
	if !ok {
		return
	}
	userID = ownerID
	store, ok := h.Store.(SchedulingStore)
	if !ok {
		http.Error(w, "caldav scheduling store is not configured", http.StatusNotImplemented)
		return
	}
	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		http.Error(w, "Content-Type header is required for scheduling", http.StatusBadRequest)
		return
	}
	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil || (!strings.EqualFold(mediaType, "text/calendar") && !strings.EqualFold(mediaType, "message/rfc822") && !strings.EqualFold(mediaType, "application/icalendar+json")) {
		http.Error(w, "Content-Type must be text/calendar or message/rfc822 for scheduling", http.StatusUnsupportedMediaType)
		return
	}
	body, err := readBoundedCalendarBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	parsed, err := ParseICalendarObjectForScheduling(body)
	if err != nil {
		http.Error(w, "invalid iCalendar object: "+err.Error(), http.StatusBadRequest)
		return
	}
	methodStr, err := ExtractICSMethod(body)
	if err != nil {
		http.Error(w, "invalid iCalendar method: "+err.Error(), http.StatusBadRequest)
		return
	}
	method := ScheduleMethod(strings.ToUpper(strings.TrimSpace(methodStr)))
	if method == "" {
		method = ScheduleMethodRequest
	}
	if !isValidScheduleMethod(method) {
		http.Error(w, fmt.Sprintf("invalid iTIP method: %s", method), http.StatusBadRequest)
		return
	}
	_, err = store.SendSchedulingMessage(r.Context(), SendSchedulingMessageRequest{
		UserID:     userID,
		Method:     method,
		UID:        parsed.UID,
		ICSPayload: body,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func isValidScheduleMethod(method ScheduleMethod) bool {
	switch method {
	case ScheduleMethodRequest, ScheduleMethodReply, ScheduleMethodCancel,
		ScheduleMethodAdd, ScheduleMethodCounter, ScheduleMethodDeclineCounter,
		ScheduleMethodRefresh, ScheduleMethodPublish:
		return true
	default:
		return false
	}
}

func parseDepthHeader(header http.Header, fallback Depth) (Depth, error) {
	values := header.Values("Depth")
	if len(values) > 1 {
		return "", fmt.Errorf("Depth header must not be repeated")
	}
	if len(values) == 0 {
		return ParseDepth("", fallback)
	}
	return ParseDepth(values[0], fallback)
}

func (h *Handler) propfindResponses(ctx context.Context, userID string, actorUserID string, resource ResourcePath, depth Depth, propfind PropfindRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	switch resource.Kind {
	case ResourceRoot:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		props, err := withCurrentUserPrincipal(ServiceRootProperties(principal), actorUserID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(RootPath+"/", propfind, props)}, nil
	case ResourcePrincipalCollection:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		props, err := withCurrentUserPrincipal(PrincipalCollectionProperties(principal), actorUserID)
		if err != nil {
			return nil, err
		}
		responses := []MultiStatusResponse{responseForProperties(PrincipalsPrefix+"/", propfind, props)}
		if depth == DepthOne {
			props, err := withCurrentUserPrincipal(PrincipalProperties(principal), actorUserID)
			if err != nil {
				return nil, err
			}
			responses = append(responses, responseForProperties(principal.PrincipalPath, propfind, props))
		}
		return responses, nil
	case ResourcePrincipal:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		props, err := withCurrentUserPrincipal(PrincipalProperties(principal), actorUserID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(principal.PrincipalPath, propfind, props)}, nil
	case ResourceCalendarHome:
		home, err := CalendarHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarHomeProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarHome, currentUserPrivileges)
		responses := []MultiStatusResponse{responseForProperties(home, propfind, props)}
		if depth == DepthOne {
			calendars, err := h.Store.ListCalendarCollections(ctx, userID)
			if err != nil {
				return nil, err
			}
			for _, calendar := range calendars {
				href, err := CalendarCollectionPath(userID, calendar.ID)
				if err != nil {
					return nil, err
				}
				props, err := CalendarCollectionProperties(userID, calendar, h.includeSyncCollection())
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceCalendarCollection, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarCollection:
		calendar, err := h.lookupCalendar(ctx, userID, resource.CalendarID)
		if err != nil {
			return nil, err
		}
		href, err := CalendarCollectionPath(userID, calendar.ID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarCollectionProperties(userID, calendar, h.includeSyncCollection())
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarCollection, currentUserPrivileges)
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.listCalendarObjectsBounded(ctx, userID, calendar.ID, MaxWebDAVReportLimit+1, false)
			if err != nil {
				return nil, err
			}
			if len(objects) > MaxWebDAVReportLimit {
				return nil, TruncatedResultsError{Operation: "calendar collection PROPFIND"}
			}
			for _, object := range objects {
				href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
				if err != nil {
					return nil, err
				}
				props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarObject:
		if depth != DepthZero {
			return nil, fmt.Errorf("calendar object PROPFIND requires Depth: 0")
		}
		includeCalendarData := containsXMLName(propfind.Properties, PropCalendarData) || containsXMLName(propfind.Include, PropCalendarData)
		object, err := h.lookupCalendarObjectForReport(ctx, userID, resource.CalendarID, resource.ObjectName, includeCalendarData)
		if err != nil {
			return nil, err
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	case ResourceInbox:
		href, err := ScheduleInboxPath(userID)
		if err != nil {
			return nil, err
		}
		props, err := SchedulingInboxProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceInbox, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	case ResourceOutbox:
		href, err := ScheduleOutboxPath(userID)
		if err != nil {
			return nil, err
		}
		props, err := SchedulingOutboxProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceOutbox, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CalDAV resource")
	}
}

func withCurrentUserPrincipal(props []PropertyResult, actorUserID string) ([]PropertyResult, error) {
	principalPath, err := PrincipalPath(actorUserID)
	if err != nil {
		return nil, err
	}
	next := append([]PropertyResult(nil), props...)
	for i := range next {
		if next[i].Name == PropCurrentUserPrincipal {
			next[i].Value.Hrefs = []string{principalPath}
			next[i].Found = true
			return next, nil
		}
	}
	return next, nil
}

func withCurrentUserPrivileges(props []PropertyResult, kind ResourceKind, privileges []XMLName) []PropertyResult {
	if len(privileges) == 0 {
		return props
	}
	privileges = currentUserPrivilegesForResource(kind, privileges)
	if len(privileges) == 0 {
		return props
	}
	next := append([]PropertyResult(nil), props...)
	for i := range next {
		if next[i].Name == PropCurrentUserPrivileges {
			next[i].Value.Privileges = append([]XMLName(nil), privileges...)
			next[i].Found = true
			return next
		}
	}
	return next
}

func currentUserPrivilegesForResource(kind ResourceKind, privileges []XMLName) []XMLName {
	role := CalendarAccessRoleRead
	for _, privilege := range privileges {
		if privilege == PrivilegeBind || privilege == PrivilegeUnbind {
			role = CalendarAccessRoleManage
			break
		}
		if privilege == PrivilegeWriteContent || privilege == PrivilegeWriteProps {
			role = CalendarAccessRoleWrite
		}
	}
	switch kind {
	case ResourceCalendarHome:
		if role == CalendarAccessRoleManage {
			return calendarHomePrivileges()
		}
		return readOnlyPrivileges()
	case ResourceCalendarCollection:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return calendarCollectionPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceCalendarObject:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return writableObjectPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceInbox:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return schedulingInboxPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceOutbox:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return schedulingOutboxPrivileges()
		}
		return readOnlyPrivileges()
	default:
		return readOnlyPrivileges()
	}
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
}

func proppatchResponse(href string, calendar Calendar, properties []XMLName) MultiStatusResponse {
	uniqueProperties := uniqueXMLNames(properties)
	results := make([]PropertyResult, 0, len(uniqueProperties))
	for _, prop := range uniqueProperties {
		switch prop {
		case PropDisplayName:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Name, Lang: calendar.NameLang}, Found: true})
		case PropCalendarDescription:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Description, Lang: calendar.DescriptionLang}, Found: true})
		case PropCalendarColor:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Color}, Found: true})
		case PropCalendarSlug:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendarSlugValue(calendar.Slug)}, Found: calendar.Slug != nil})
		case PropCalendarTimezone:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendarTimezoneValue(calendar.Timezone)}, Found: calendar.Timezone != nil})
		}
	}
	return MultiStatusResponse{Href: href, PropStats: []PropStatus{{StatusCode: http.StatusOK, Properties: results}}}
}

func proppatchFailureResponse(href string, patch ProppatchRequest) MultiStatusResponse {
	propStats := make([]PropStatus, 0, 2)
	forbidden := make([]PropertyResult, 0, len(patch.Protected)+len(patch.Unsupported))
	for _, prop := range patch.Protected {
		forbidden = append(forbidden, PropertyResult{Name: prop})
	}
	for _, prop := range patch.Unsupported {
		forbidden = append(forbidden, PropertyResult{Name: prop})
	}
	if len(forbidden) > 0 {
		propStats = append(propStats, PropStatus{StatusCode: http.StatusForbidden, Properties: forbidden})
	}
	if len(patch.Properties) > 0 {
		uniqueProperties := uniqueXMLNames(patch.Properties)
		failed := make([]PropertyResult, 0, len(uniqueProperties))
		for _, prop := range uniqueProperties {
			failed = append(failed, PropertyResult{Name: prop})
		}
		propStats = append(propStats, PropStatus{StatusCode: http.StatusFailedDependency, Properties: failed})
	}
	return MultiStatusResponse{Href: href, PropStats: propStats}
}

func uniqueXMLNames(properties []XMLName) []XMLName {
	if len(properties) < 2 {
		return properties
	}
	seen := make(map[XMLName]struct{}, len(properties))
	unique := make([]XMLName, 0, len(properties))
	for _, prop := range properties {
		if _, ok := seen[prop]; ok {
			continue
		}
		seen[prop] = struct{}{}
		unique = append(unique, prop)
	}
	return unique
}

func (h *Handler) reportResponses(ctx context.Context, userID string, resource ResourcePath, depth Depth, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	switch report.Kind {
	case ReportCalendarMulti:
		if resource.Kind != ResourceCalendarCollection && resource.Kind != ResourceCalendarHome {
			return nil, fmt.Errorf("calendar-multiget requires a calendar collection or home resource")
		}
		return h.calendarMultigetResponses(ctx, userID, resource, report, currentUserPrivileges)
	case ReportCalendarQuery:
		if resource.Kind != ResourceCalendarCollection {
			return nil, fmt.Errorf("calendar-query requires a calendar collection resource")
		}
		if depth == DepthZero {
			return nil, nil
		}
		return h.calendarQueryResponses(ctx, userID, resource, report, currentUserPrivileges)
	case ReportSyncCollection:
		responses, _, err := h.syncCollectionReport(ctx, userID, resource, report, currentUserPrivileges)
		return responses, err
	default:
		return nil, fmt.Errorf("REPORT %s is not implemented", report.Kind)
	}
}

func (h *Handler) calendarMultigetResponses(ctx context.Context, userID string, requestResource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	includeCalendarData := containsXMLName(report.Properties, PropCalendarData)
	type requestedKey struct {
		calendarID string
		objectName string
	}
	type requestedObject struct {
		href       string
		calendarID string
		objectName string
	}
	requested := make([]requestedObject, 0, len(report.Hrefs))
	requestedIndex := make(map[requestedKey]struct{}, len(report.Hrefs))

	for _, href := range report.Hrefs {
		resource, err := ParseResourceHref(href)
		if err != nil || resource.Kind != ResourceCalendarObject || resource.UserID != userID || !multigetHrefInScope(requestResource, resource) {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		requested = append(requested, requestedObject{
			href:       href,
			calendarID: resource.CalendarID,
			objectName: resource.ObjectName,
		})
		requestedIndex[requestedKey{calendarID: resource.CalendarID, objectName: resource.ObjectName}] = struct{}{}
	}
	requestedByCalendar := make(map[string][]string)
	for key := range requestedIndex {
		requestedByCalendar[key.calendarID] = append(requestedByCalendar[key.calendarID], key.objectName)
	}
	objectsByKey, err := h.lookupCalendarObjectsByNames(ctx, userID, requestedByCalendar, CalendarStatusActive, includeCalendarData)
	if err != nil {
		return nil, err
	}

	for _, ref := range requested {
		object, ok := objectsByKey[calendarObjectLookupKey{calendarID: ref.calendarID, objectName: ref.objectName}]
		if !ok {
			responses = append(responses, notFoundResponse(ref.href, report.Properties))
			continue
		}
		object.CalendarID = ref.calendarID
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if includeCalendarData {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return nil, err
			}
			props = append(props, prop)
		}
		objectHref, err := CalendarObjectPath(userID, ref.calendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(objectHref, propfind, props))
	}
	return responses, nil
}

func multigetHrefInScope(requestResource ResourcePath, hrefResource ResourcePath) bool {
	switch requestResource.Kind {
	case ResourceCalendarHome:
		return requestResource.UserID == hrefResource.UserID
	case ResourceCalendarCollection:
		return requestResource.UserID == hrefResource.UserID && requestResource.CalendarID == hrefResource.CalendarID
	default:
		return false
	}
}

func (h *Handler) calendarQueryResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	calendar, err := h.lookupCalendar(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	var tz *time.Location
	if calendar.Timezone != nil && *calendar.Timezone != "" {
		tz, err = time.LoadLocation(*calendar.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid calendar timezone %q: %w", *calendar.Timezone, err)
		}
	}
	includeCalendarData := containsXMLName(report.Properties, PropCalendarData)
	component := strings.ToUpper(strings.TrimSpace(report.Component))
	if component == unsupportedCalendarQueryComponent {
		return []MultiStatusResponse{}, nil
	}
	if report.TimeRange != nil {
		return h.calendarQueryTimeRangeResponses(ctx, userID, resource, report, currentUserPrivileges, objectPrincipalPath, component, tz, includeCalendarData)
	}
	objects, componentFilteredInStore, err := h.listCalendarObjectsForQuery(ctx, userID, resource.CalendarID, limit+1, component, includeCalendarData)
	if err != nil {
		return nil, err
	}
	if report.TimeRange == nil && len(objects) > limit {
		return nil, TruncatedResultsError{Operation: "calendar-query limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		if !componentFilteredInStore && !calendarObjectMatchesComponent(object, component) {
			continue
		}
		if report.TimeRange != nil {
			matches, err := CalendarObjectMatchesTimeRange(object.ICS, report.Component, report.TimeRange, tz)
			if err != nil {
				return nil, err
			}
			if !matches {
				continue
			}
		}
		if len(responses) >= limit {
			return nil, TruncatedResultsError{Operation: "calendar-query limit"}
		}
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if includeCalendarData {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return nil, err
			}
			props = append(props, prop)
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
}

func (h *Handler) calendarQueryTimeRangeResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName, objectPrincipalPath string, component string, tz *time.Location, includeCalendarData bool) ([]MultiStatusResponse, error) {
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, limit)
	handleObject := func(object CalendarObject) (bool, error) {
		if !calendarObjectMatchesComponent(object, component) {
			return true, nil
		}
		matches, err := CalendarObjectMatchesTimeRange(object.ICS, report.Component, report.TimeRange, tz)
		if err != nil {
			return false, err
		}
		if !matches {
			return true, nil
		}
		if len(responses) >= limit {
			return false, TruncatedResultsError{Operation: "calendar-query limit"}
		}
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return false, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if includeCalendarData {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return false, err
			}
			props = append(props, prop)
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return false, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
		return true, nil
	}
	if component != "" {
		if walker, ok := h.Store.(CalendarQueryCandidateWalker); ok {
			err := walker.WalkCalendarQueryCandidates(ctx, userID, resource.CalendarID, CalendarStatusActive, component, handleObject)
			return responses, err
		}
	}
	objects, err := h.Store.ListCalendarObjects(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	for _, object := range objects {
		keepGoing, err := handleObject(object)
		if err != nil {
			return nil, err
		}
		if !keepGoing {
			break
		}
	}
	return responses, nil
}

func (h *Handler) listCalendarObjectsForQuery(ctx context.Context, userID string, calendarID string, limit int, component string, includeICS bool) ([]CalendarObject, bool, error) {
	if component == "" {
		objects, err := h.listCalendarObjectsBounded(ctx, userID, calendarID, limit, includeICS)
		return objects, false, err
	}
	if componentLimiter, ok := h.Store.(CalendarObjectComponentStore); ok {
		objects, err := componentLimiter.ListCalendarObjectsByComponentLimit(
			ctx,
			userID,
			calendarID,
			CalendarStatusActive,
			component,
			limit,
			includeICS,
		)
		return objects, true, err
	}
	objects, err := h.listCalendarObjectsBounded(ctx, userID, calendarID, limit, includeICS)
	return objects, false, err
}

func calendarObjectMatchesComponent(object CalendarObject, component string) bool {
	if component == "" {
		return true
	}
	if component == unsupportedCalendarQueryComponent {
		return false
	}
	if object.Component == "" {
		return true
	}
	return strings.EqualFold(object.Component, component)
}

func (h *Handler) freeBusyCalendar(ctx context.Context, userID string, resource ResourcePath, timeRange TimeRange, includeChildren bool, requestedLimit int) ([]byte, error) {
	calendar, err := h.lookupCalendar(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	var tz *time.Location
	if calendar.Timezone != nil && *calendar.Timezone != "" {
		tz, err = time.LoadLocation(*calendar.Timezone)
		if err != nil {
			return nil, fmt.Errorf("invalid calendar timezone %q: %w", *calendar.Timezone, err)
		}
	}
	if !includeChildren {
		return BuildFreeBusyCalendar(userID, resource.CalendarID, timeRange, nil)
	}
	limit := requestedLimit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	objects, err := h.listFreeBusyObjectsBounded(ctx, userID, resource.CalendarID, limit+1, true)
	if err != nil {
		return nil, err
	}
	if len(objects) > limit {
		return nil, TruncatedResultsError{Operation: "free-busy-query limit"}
	}
	var periods []BusyPeriod
	for _, object := range objects {
		objectPeriods, err := CalendarObjectBusyPeriods(object.ICS, timeRange, tz)
		if err != nil {
			return nil, err
		}
		periods = append(periods, objectPeriods...)
	}
	return BuildFreeBusyCalendar(userID, resource.CalendarID, timeRange, periods)
}

func (h *Handler) listFreeBusyObjectsBounded(ctx context.Context, userID string, calendarID string, limit int, includeICS bool) ([]CalendarObject, error) {
	if componentLimiter, ok := h.Store.(CalendarObjectComponentStore); ok {
		objects := make([]CalendarObject, 0, limit)
		for _, component := range []string{ComponentVEVENT, ComponentVFREEBUSY} {
			if len(objects) >= limit {
				break
			}
			more, err := componentLimiter.ListCalendarObjectsByComponentLimit(
				ctx,
				userID,
				calendarID,
				CalendarStatusActive,
				component,
				limit-len(objects),
				includeICS,
			)
			if err != nil {
				return nil, err
			}
			objects = append(objects, more...)
		}
		return objects, nil
	}
	return h.listCalendarObjectsBounded(ctx, userID, calendarID, limit, includeICS)
}

func (h *Handler) syncCollectionReport(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, "", err
	}
	if resource.Kind != ResourceCalendarCollection {
		return nil, "", fmt.Errorf("sync-collection requires a calendar collection resource")
	}
	calendar, err := h.lookupCalendar(ctx, userID, resource.CalendarID)
	if err != nil {
		if report.SyncToken == "" {
			return nil, "", err
		}
		return h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
	}
	if report.SyncToken != "" {
		if report.SyncToken != calendar.SyncToken {
			return h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
		}
		return nil, calendar.SyncToken, nil
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	includeCalendarData := containsXMLName(report.Properties, PropCalendarData)
	objects, err := h.listCalendarObjectsBounded(ctx, userID, resource.CalendarID, limit+1, includeCalendarData)
	if err != nil {
		return nil, "", err
	}
	if len(objects) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if includeCalendarData {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return nil, "", err
			}
			props = append(props, prop)
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, "", err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, calendar.SyncToken, nil
}

func (h *Handler) listCalendarObjectsBounded(ctx context.Context, userID string, calendarID string, limit int, includeICS bool) ([]CalendarObject, error) {
	if includeICS {
		if limiter, ok := h.Store.(CalendarObjectLimiter); ok {
			return limiter.ListCalendarObjectsLimit(ctx, userID, calendarID, limit)
		}
		return h.Store.ListCalendarObjects(ctx, userID, calendarID)
	}
	if metadataLimiter, ok := h.Store.(CalendarObjectMetadataStore); ok {
		return metadataLimiter.ListCalendarObjectMetadataLimit(ctx, userID, calendarID, CalendarStatusActive, limit)
	}
	if limiter, ok := h.Store.(CalendarObjectLimiter); ok {
		return limiter.ListCalendarObjectsLimit(ctx, userID, calendarID, limit)
	}
	return h.Store.ListCalendarObjects(ctx, userID, calendarID)
}

func (h *Handler) lookupCalendarObjectForReport(ctx context.Context, userID string, calendarID string, objectName string, includeCalendarData bool) (CalendarObject, error) {
	if includeCalendarData {
		return h.Store.LookupCalendarObject(ctx, userID, calendarID, objectName)
	}
	if metadataStore, ok := h.Store.(CalendarObjectMetadataStore); ok {
		return metadataStore.LookupCalendarObjectMetadata(ctx, userID, calendarID, objectName)
	}
	return h.Store.LookupCalendarObject(ctx, userID, calendarID, objectName)
}

func (h *Handler) syncChangeResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, "", err
	}
	store, ok := h.Store.(SyncChangeStore)
	if !ok {
		return nil, "", InvalidSyncTokenError{Token: report.SyncToken}
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	fetchLimit := MaxWebDAVReportLimit + 1
	includeCalendarData := containsXMLName(report.Properties, PropCalendarData)
	if changeWithObjectStore, ok := store.(CalendarChangeWithObjectStore); ok {
		changesWithObject, err := changeWithObjectStore.ListCalendarChangesWithObjectsSince(ctx, ListChangesSinceRequest{
			UserID:     userID,
			CalendarID: resource.CalendarID,
			SyncToken:  report.SyncToken,
			Limit:      fetchLimit,
		}, false)
		if err != nil {
			return nil, "", err
		}
		if len(changesWithObject) > MaxWebDAVReportLimit {
			return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
		}
		syncToken := report.SyncToken
		for _, item := range changesWithObject {
			if strings.TrimSpace(item.Change.SyncToken) != "" {
				syncToken = strings.TrimSpace(item.Change.SyncToken)
			}
		}
		changesWithObject = coalesceCalendarChangesWithObjects(changesWithObject)
		if len(changesWithObject) > limit {
			return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
		}
		objectsWithData := map[calendarObjectLookupKey]CalendarObject{}
		if includeCalendarData {
			requestedByCalendar := make(map[string][]string)
			for _, item := range changesWithObject {
				change := item.Change
				if change.Action == "object-deleted" || !item.HasObject {
					continue
				}
				requestedByCalendar[change.CalendarID] = append(requestedByCalendar[change.CalendarID], change.ObjectName)
			}
			objectsWithData, err = h.lookupCalendarObjectsByNames(ctx, userID, requestedByCalendar, CalendarStatusActive, true)
			if err != nil {
				return nil, "", err
			}
		}
		propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
		responses := make([]MultiStatusResponse, 0, len(changesWithObject))
		for _, item := range changesWithObject {
			change := item.Change
			href, err := CalendarObjectPath(userID, change.CalendarID, change.ObjectName)
			if err != nil {
				return nil, "", err
			}
			if change.Action == "object-deleted" || !item.HasObject {
				responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
				continue
			}
			object := item.Object
			object.CalendarID = change.CalendarID
			if includeCalendarData {
				objectWithData, ok := objectsWithData[calendarObjectLookupKey{calendarID: change.CalendarID, objectName: change.ObjectName}]
				if !ok {
					responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
					continue
				}
				object = objectWithData
			}
			props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
			if err != nil {
				return nil, "", err
			}
			props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
			if includeCalendarData {
				prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
				if err != nil {
					return nil, "", err
				}
				props = append(props, prop)
			}
			responses = append(responses, responseForProperties(href, propfind, props))
		}
		return responses, syncToken, nil
	}

	changes, err := store.ListCalendarChangesSince(ctx, ListChangesSinceRequest{
		UserID:     userID,
		CalendarID: resource.CalendarID,
		SyncToken:  report.SyncToken,
		Limit:      fetchLimit,
	})
	if err != nil {
		return nil, "", err
	}
	if len(changes) > MaxWebDAVReportLimit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	syncToken := report.SyncToken
	for _, change := range changes {
		if strings.TrimSpace(change.SyncToken) != "" {
			syncToken = strings.TrimSpace(change.SyncToken)
		}
	}
	changes = coalesceCalendarChanges(changes)
	if len(changes) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	requestedByCalendarNames := make(map[string]map[string]struct{}, len(changes))
	for _, change := range changes {
		names, ok := requestedByCalendarNames[change.CalendarID]
		if !ok {
			names = make(map[string]struct{}, 4)
			requestedByCalendarNames[change.CalendarID] = names
		}
		if _, ok := names[change.ObjectName]; ok {
			continue
		}
		names[change.ObjectName] = struct{}{}
	}

	requestedByCalendar := make(map[string][]string, len(requestedByCalendarNames))
	for calendarID, names := range requestedByCalendarNames {
		objectNames := make([]string, 0, len(names))
		for objectName := range names {
			objectNames = append(objectNames, objectName)
		}
		requestedByCalendar[calendarID] = objectNames
	}

	objectsByKey, err := h.lookupCalendarObjectsByNames(ctx, userID, requestedByCalendar, CalendarStatusActive, includeCalendarData)
	if err != nil {
		return nil, "", err
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(changes))
	for _, change := range changes {
		href, err := CalendarObjectPath(userID, change.CalendarID, change.ObjectName)
		if err != nil {
			return nil, "", err
		}
		if change.Action == "object-deleted" {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		object, ok := objectsByKey[calendarObjectLookupKey{calendarID: change.CalendarID, objectName: change.ObjectName}]
		if !ok {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		object.CalendarID = change.CalendarID
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if includeCalendarData {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return nil, "", err
			}
			props = append(props, prop)
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, syncToken, nil
}

type coalescedCalendarChangeWithObject struct {
	item   CalendarChangeWithObject
	active bool
}

func coalesceCalendarChangesWithObjects(changes []CalendarChangeWithObject) []CalendarChangeWithObject {
	entries := make([]coalescedCalendarChangeWithObject, 0, len(changes))
	latestIndex := make(map[calendarObjectLookupKey]int, len(changes))
	for _, item := range changes {
		if !calendarChangeHasObjectResponse(item.Change) {
			continue
		}
		key := calendarObjectLookupKey{calendarID: item.Change.CalendarID, objectName: item.Change.ObjectName}
		if previous, ok := latestIndex[key]; ok {
			entries[previous].active = false
		}
		latestIndex[key] = len(entries)
		entries = append(entries, coalescedCalendarChangeWithObject{item: item, active: true})
	}
	coalesced := make([]CalendarChangeWithObject, 0, len(latestIndex))
	for _, entry := range entries {
		if entry.active {
			coalesced = append(coalesced, entry.item)
		}
	}
	return coalesced
}

type coalescedCalendarChange struct {
	change CalendarChange
	active bool
}

func coalesceCalendarChanges(changes []CalendarChange) []CalendarChange {
	entries := make([]coalescedCalendarChange, 0, len(changes))
	latestIndex := make(map[calendarObjectLookupKey]int, len(changes))
	for _, change := range changes {
		if !calendarChangeHasObjectResponse(change) {
			continue
		}
		key := calendarObjectLookupKey{calendarID: change.CalendarID, objectName: change.ObjectName}
		if previous, ok := latestIndex[key]; ok {
			entries[previous].active = false
		}
		latestIndex[key] = len(entries)
		entries = append(entries, coalescedCalendarChange{change: change, active: true})
	}
	coalesced := make([]CalendarChange, 0, len(latestIndex))
	for _, entry := range entries {
		if entry.active {
			coalesced = append(coalesced, entry.change)
		}
	}
	return coalesced
}

func calendarChangeHasObjectResponse(change CalendarChange) bool {
	return change.Action != "collection-deleted" && change.Action != "collection-updated" && strings.TrimSpace(change.ObjectName) != ""
}

func (h *Handler) lookupCalendarObjectsByNames(ctx context.Context, userID string, objectNamesByCalendar map[string][]string, status string, includeICS bool) (map[calendarObjectLookupKey]CalendarObject, error) {
	if len(objectNamesByCalendar) == 0 {
		return map[calendarObjectLookupKey]CalendarObject{}, nil
	}
	if len(objectNamesByCalendar) > 1 {
		if crossBatchStore, ok := h.Store.(CalendarObjectCrossCalendarBatchStore); ok {
			objects, err := crossBatchStore.ListCalendarObjectsByNameGroups(ctx, userID, objectNamesByCalendar, status, includeICS)
			if err != nil {
				return nil, err
			}
			objectsByKey := make(map[calendarObjectLookupKey]CalendarObject, len(objects))
			for _, object := range objects {
				objectsByKey[calendarObjectLookupKey{calendarID: object.CalendarID, objectName: object.ObjectName}] = object
			}
			return objectsByKey, nil
		}
	}
	if batchStore, ok := h.Store.(CalendarObjectBatchStore); ok {
		objectsByKey := make(map[calendarObjectLookupKey]CalendarObject)
		for calendarID, objectNames := range objectNamesByCalendar {
			if len(objectNames) == 0 {
				continue
			}
			objects, err := batchStore.ListCalendarObjectsByNames(ctx, userID, calendarID, status, objectNames, includeICS)
			if err != nil {
				return nil, err
			}
			for _, object := range objects {
				objectsByKey[calendarObjectLookupKey{calendarID: object.CalendarID, objectName: object.ObjectName}] = object
			}
		}
		return objectsByKey, nil
	}
	objectsByKey := make(map[calendarObjectLookupKey]CalendarObject)
	for calendarID, objectNames := range objectNamesByCalendar {
		for _, objectName := range objectNames {
			object, err := h.lookupCalendarObjectForReport(ctx, userID, calendarID, objectName, includeICS)
			if err != nil {
				continue
			}
			objectsByKey[calendarObjectLookupKey{calendarID: calendarID, objectName: objectName}] = object
		}
	}
	return objectsByKey, nil
}

func notFoundResponse(href string, properties []XMLName) MultiStatusResponse {
	missing := make([]PropertyResult, 0, len(properties))
	for _, prop := range properties {
		missing = append(missing, PropertyResult{Name: prop})
	}
	return MultiStatusResponse{Href: href, PropStats: []PropStatus{{StatusCode: http.StatusNotFound, Properties: missing}}}
}

func containsXMLName(names []XMLName, target XMLName) bool {
	for _, name := range names {
		if name == target {
			return true
		}
	}
	return false
}

func writeDAVPreconditionError(w http.ResponseWriter, status int, precondition string, message string) {
	writePreconditionError(w, status, DAVNamespace, "D", precondition, message)
}

func writeCalDAVPreconditionError(w http.ResponseWriter, status int, precondition string, message string) {
	writePreconditionError(w, status, CalDAVNamespace, "C", precondition, message)
}

func writePreconditionError(w http.ResponseWriter, status int, namespace string, prefix string, precondition string, message string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if prefix == "D" {
		_, _ = fmt.Fprintf(w,
			`<?xml version="1.0" encoding="UTF-8"?>`+
				`<D:error xmlns:D="%s"><D:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
			namespace,
			precondition,
			xmlEscapeText(message),
		)
		return
	}
	_, _ = fmt.Fprintf(w,
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<D:error xmlns:D="%s" xmlns:%s="%s"><%s:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
		DAVNamespace,
		prefix,
		namespace,
		prefix,
		precondition,
		xmlEscapeText(message),
	)
}

func xmlEscapeText(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return ""
	}
	return b.String()
}

func calDAVAllowHeader() string {
	return strings.Join(ImplementedMethods(), ", ")
}
