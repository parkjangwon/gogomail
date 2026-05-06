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
)

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error)
	LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error)
	ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error)
	LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error)
}

type CalendarObjectLimiter interface {
	ListCalendarObjectsLimit(ctx context.Context, userID string, calendarID string, limit int) ([]CalendarObject, error)
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
	if r.URL.Path == WellKnownCalDAVPath {
		h.serveWellKnown(w, r)
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
	default:
		w.Header().Set("Allow", calDAVAllowHeader())
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
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

func (h *Handler) serveProppatch(w http.ResponseWriter, r *http.Request) {
	userID, resource, _, ok := h.resolveResourceRequest(w, r, CalendarAccessRoleWrite)
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
	if !h.checkCalendarCollectionPreconditions(w, r, userID, resource.CalendarID) {
		return
	}
	patch, err := ParseProppatch(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	calendar, err := store.UpdateCalendarProperties(r.Context(), UpdateCalendarRequest{
		UserID:      userID,
		CalendarID:  resource.CalendarID,
		Name:        patch.Name,
		Color:       patch.Color,
		Description: patch.Description,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	href, err := CalendarCollectionPath(userID, calendar.ID)
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
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
	if _, err := h.Store.LookupCalendar(r.Context(), userID, resource.CalendarID); err == nil {
		http.Error(w, "caldav calendar already exists", http.StatusMethodNotAllowed)
		return
	}
	if _, err := ValidateCalendarPathID(resource.CalendarID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req, err := ParseMKCalendar(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	calendar, err := store.CreateCalendarAtPath(r.Context(), CreateCalendarAtPathRequest{
		UserID:      userID,
		CalendarID:  resource.CalendarID,
		Name:        req.DisplayName,
		Color:       req.Color,
		Description: req.Description,
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
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusCreated)
}

func (h *Handler) serveGetObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := h.resolveObjectRequest(w, r, CalendarAccessRoleRead)
	if !ok {
		return
	}
	object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
	if err != nil {
		http.Error(w, "caldav object not found", http.StatusNotFound)
		return
	}
	if ifMatch := conditionalHeaderValue(r.Header, "If-Match"); ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
		http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
		return
	}
	if objectModifiedSince(r.Header.Get("If-Unmodified-Since"), object.UpdatedAt) {
		http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	if ifNoneMatchMatches(conditionalHeaderValue(r.Header, "If-None-Match"), object.ETag) {
		writeCalendarObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if objectNotModifiedSince(r.Header.Get("If-Modified-Since"), object.UpdatedAt) {
		writeCalendarObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeCalendarObjectHeaders(w, object)
	w.WriteHeader(http.StatusOK)
	if r.Method != MethodHead {
		_, _ = w.Write(object.ICS)
	}
}

func (h *Handler) servePutObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := h.resolveObjectRequest(w, r, CalendarAccessRoleWrite)
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
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	existed := false
	var existing CalendarObject
	if object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName); err == nil {
		existed = true
		existing = object
	}
	if existed && ifNoneMatchMatches(ifNoneMatch, existing.ETag) {
		http.Error(w, "caldav object already exists", http.StatusPreconditionFailed)
		return
	}
	observedETag := conditionalHeaderValue(r.Header, "If-Match")
	if observedETag == "*" {
		if !existed {
			http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
			return
		}
		observedETag = ""
	} else if observedETag != "" && !existed {
		http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
		return
	} else if observedETag != "" && !ifMatchMatches(observedETag, existing.ETag) {
		http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
		return
	} else if observedETag != "" {
		observedETag = existing.ETag
	}
	if existed && objectModifiedSince(r.Header.Get("If-Unmodified-Since"), existing.UpdatedAt) {
		http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	body, err := readBoundedCalendarBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	object, err := store.UpsertObject(r.Context(), UpsertObjectRequest{
		UserID:       userID,
		CalendarID:   resource.CalendarID,
		ObjectName:   resource.ObjectName,
		ICS:          body,
		ObservedETag: observedETag,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeCalendarObjectHeaders(w, object)
	if existed {
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
		http.Error(w, "caldav user is not authenticated", http.StatusUnauthorized)
		return
	}
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
		h.deleteCalendarCollection(w, r, userID, resource)
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
	ifUnmodifiedSince := strings.TrimSpace(r.Header.Get("If-Unmodified-Since"))
	if ifMatch != "" || ifUnmodifiedSince != "" {
		object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
			return
		}
		if ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
			http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
			return
		}
		if objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
			http.Error(w, "caldav object modified since precondition", http.StatusPreconditionFailed)
			return
		}
	}
	if _, err := store.DeleteObject(r.Context(), DeleteObjectRequest{UserID: userID, CalendarID: resource.CalendarID, ObjectName: resource.ObjectName}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) deleteCalendarCollection(w http.ResponseWriter, r *http.Request, userID string, resource ResourcePath) {
	store, ok := h.Store.(CalendarDeleter)
	if !ok {
		http.Error(w, "caldav calendar deleter is not configured", http.StatusNotImplemented)
		return
	}
	if !h.checkCalendarCollectionPreconditions(w, r, userID, resource.CalendarID) {
		return
	}
	if _, err := store.DeleteCalendar(r.Context(), DeleteCalendarRequest{UserID: userID, CalendarID: resource.CalendarID}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) checkCalendarCollectionPreconditions(w http.ResponseWriter, r *http.Request, userID string, calendarID string) bool {
	ifMatch := conditionalHeaderValue(r.Header, "If-Match")
	ifUnmodifiedSince := strings.TrimSpace(r.Header.Get("If-Unmodified-Since"))
	if ifMatch != "" || ifUnmodifiedSince != "" {
		calendar, err := h.Store.LookupCalendar(r.Context(), userID, calendarID)
		if err != nil {
			http.Error(w, "caldav calendar not found", http.StatusPreconditionFailed)
			return false
		}
		if ifMatch != "" {
			etag, err := CalendarCollectionETag(userID, calendar)
			if err != nil || !ifMatchMatches(ifMatch, etag) {
				http.Error(w, "caldav calendar collection etag mismatch", http.StatusPreconditionFailed)
				return false
			}
		}
		if objectModifiedSince(ifUnmodifiedSince, calendar.UpdatedAt) {
			http.Error(w, "caldav calendar modified since precondition", http.StatusPreconditionFailed)
			return false
		}
	}
	return true
}

func (h *Handler) resolveObjectRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, bool) {
	userID, resource, _, ok := h.resolveResourceRequest(w, r, requiredRole)
	if !ok {
		return "", ResourcePath{}, false
	}
	if resource.Kind != ResourceCalendarObject {
		http.Error(w, "caldav object path is required", http.StatusNotFound)
		return "", ResourcePath{}, false
	}
	return userID, resource, true
}

func (h *Handler) resolveResourceRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, AccessDecision, bool) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return "", ResourcePath{}, AccessDecision{}, false
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return "", ResourcePath{}, AccessDecision{}, false
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return "", ResourcePath{}, AccessDecision{}, false
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, requiredRole)
	if !ok {
		return "", ResourcePath{}, AccessDecision{}, false
	}
	return ownerID, resource, decision, true
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

func ifNoneMatchMatches(header string, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" || strings.ContainsAny(header, "\r\n") {
		return false
	}
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if strings.HasPrefix(candidate, "W/") {
			candidate = strings.TrimSpace(strings.TrimPrefix(candidate, "W/"))
		}
		if candidate == etag {
			return true
		}
	}
	return false
}

func ifMatchMatches(header string, etag string) bool {
	header = strings.TrimSpace(header)
	if header == "" || strings.ContainsAny(header, "\r\n") {
		return false
	}
	if header == "*" {
		return true
	}
	for _, candidate := range strings.Split(header, ",") {
		if strings.TrimSpace(candidate) == etag {
			return true
		}
	}
	return false
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.IncludeScheduling), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.WriteHeader(http.StatusNoContent)
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
				props, err := CalendarCollectionProperties(userID, calendar)
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
		calendar, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID)
		if err != nil {
			return nil, err
		}
		href, err := CalendarCollectionPath(userID, calendar.ID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarCollectionProperties(userID, calendar)
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
			objects, err := h.listCalendarObjectsBounded(ctx, userID, calendar.ID, MaxWebDAVReportLimit+1)
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
				props, err := CalendarObjectProperties(userID, object)
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
		object, err := h.Store.LookupCalendarObject(ctx, userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			return nil, err
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
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
	default:
		return readOnlyPrivileges()
	}
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
}

func proppatchResponse(href string, calendar Calendar, properties []XMLName) MultiStatusResponse {
	results := make([]PropertyResult, 0, len(properties))
	for _, prop := range properties {
		switch prop {
		case PropDisplayName:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Name}, Found: true})
		case PropCalendarDescription:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Description}, Found: true})
		case PropCalendarColor:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: calendar.Color}, Found: true})
		}
	}
	return MultiStatusResponse{Href: href, PropStats: []PropStatus{{StatusCode: http.StatusOK, Properties: results}}}
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
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	for _, href := range report.Hrefs {
		resource, err := ParseResourceHref(href)
		if err != nil || resource.Kind != ResourceCalendarObject || resource.UserID != userID || !multigetHrefInScope(requestResource, resource) {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		object, err := h.Store.LookupCalendarObject(ctx, userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropCalendarData) {
			prop, err := CalendarObjectDataProperty(object.ICS, report.CalendarData)
			if err != nil {
				return nil, err
			}
			props = append(props, prop)
		}
		objectHref, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
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
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	objects, err := h.listCalendarObjectsBounded(ctx, userID, resource.CalendarID, limit+1)
	if err != nil {
		return nil, err
	}
	if len(objects) > limit {
		return nil, TruncatedResultsError{Operation: "calendar-query limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		if !calendarObjectMatchesComponent(object, report.Component) {
			continue
		}
		matches, err := CalendarObjectMatchesTimeRange(object.ICS, report.TimeRange)
		if err != nil {
			return nil, err
		}
		if !matches {
			continue
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropCalendarData) {
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

func calendarObjectMatchesComponent(object CalendarObject, component string) bool {
	component = strings.ToUpper(strings.TrimSpace(component))
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
	if _, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID); err != nil {
		return nil, err
	}
	if !includeChildren {
		return BuildFreeBusyCalendar(userID, resource.CalendarID, timeRange, nil)
	}
	limit := requestedLimit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	objects, err := h.listCalendarObjectsBounded(ctx, userID, resource.CalendarID, limit+1)
	if err != nil {
		return nil, err
	}
	if len(objects) > limit {
		return nil, TruncatedResultsError{Operation: "free-busy-query limit"}
	}
	var periods []BusyPeriod
	for _, object := range objects {
		objectPeriods, err := CalendarObjectBusyPeriods(object.ICS, timeRange)
		if err != nil {
			return nil, err
		}
		periods = append(periods, objectPeriods...)
	}
	return BuildFreeBusyCalendar(userID, resource.CalendarID, timeRange, periods)
}

func (h *Handler) syncCollectionReport(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	if resource.Kind != ResourceCalendarCollection {
		return nil, "", fmt.Errorf("sync-collection requires a calendar collection resource")
	}
	calendar, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID)
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
	objects, err := h.listCalendarObjectsBounded(ctx, userID, resource.CalendarID, limit+1)
	if err != nil {
		return nil, "", err
	}
	if len(objects) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropCalendarData) {
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

func (h *Handler) listCalendarObjectsBounded(ctx context.Context, userID string, calendarID string, limit int) ([]CalendarObject, error) {
	if limiter, ok := h.Store.(CalendarObjectLimiter); ok {
		return limiter.ListCalendarObjectsLimit(ctx, userID, calendarID, limit)
	}
	return h.Store.ListCalendarObjects(ctx, userID, calendarID)
}

func (h *Handler) syncChangeResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	store, ok := h.Store.(SyncChangeStore)
	if !ok {
		return nil, "", InvalidSyncTokenError{Token: report.SyncToken}
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	fetchLimit := limit + 1
	changes, err := store.ListCalendarChangesSince(ctx, ListChangesSinceRequest{
		UserID:     userID,
		CalendarID: resource.CalendarID,
		SyncToken:  report.SyncToken,
		Limit:      fetchLimit,
	})
	if err != nil {
		return nil, "", err
	}
	if len(changes) > limit {
		return nil, "", fmt.Errorf("sync-collection limit may truncate change results")
	}
	syncToken := report.SyncToken
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(changes))
	for _, change := range changes {
		if strings.TrimSpace(change.SyncToken) != "" {
			syncToken = strings.TrimSpace(change.SyncToken)
		}
		if change.Action == "collection-deleted" || change.Action == "collection-updated" || change.ObjectName == "" {
			continue
		}
		href, err := CalendarObjectPath(userID, change.CalendarID, change.ObjectName)
		if err != nil {
			return nil, "", err
		}
		if change.Action == "object-deleted" {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		object, err := h.Store.LookupCalendarObject(ctx, userID, change.CalendarID, change.ObjectName)
		if err != nil {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropCalendarData) {
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
