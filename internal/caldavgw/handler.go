package caldavgw

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
)

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error)
	LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error)
	ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error)
	LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error)
}

type ObjectStore interface {
	UpsertObject(ctx context.Context, req UpsertObjectRequest) (CalendarObject, error)
	DeleteObject(ctx context.Context, req DeleteObjectRequest) (CalendarObject, error)
}

type CalendarCreator interface {
	CreateCalendarAtPath(ctx context.Context, req CreateCalendarAtPathRequest) (Calendar, error)
}

type UserResolver func(*http.Request) (string, error)

type Handler struct {
	Store             DiscoveryStore
	ResolveUser       UserResolver
	IncludeScheduling bool
}

type InvalidSyncTokenError struct {
	Token string
}

func (e InvalidSyncTokenError) Error() string {
	return "CalDAV sync-token is no longer valid"
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
	switch r.Method {
	case MethodOptions:
		h.serveOptions(w)
	case MethodPropfind:
		h.servePropfind(w, r)
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
	if resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return
	}
	if _, err := h.Store.LookupPrincipal(r.Context(), userID); err != nil {
		http.Error(w, "caldav calendar home not found", http.StatusConflict)
		return
	}
	if _, err := h.Store.LookupCalendar(r.Context(), userID, resource.CalendarID); err == nil {
		http.Error(w, "caldav calendar already exists", http.StatusMethodNotAllowed)
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
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
	if err != nil {
		http.Error(w, "caldav object not found", http.StatusNotFound)
		return
	}
	writeCalendarObjectHeaders(w, object)
	w.WriteHeader(http.StatusOK)
	if r.Method != MethodHead {
		_, _ = w.Write(object.ICS)
	}
}

func (h *Handler) servePutObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "caldav object store is not configured", http.StatusNotImplemented)
		return
	}
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
	existed := false
	if _, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName); err == nil {
		existed = true
	}
	if ifNoneMatch == "*" && existed {
		http.Error(w, "caldav object already exists", http.StatusPreconditionFailed)
		return
	}
	observedETag := strings.TrimSpace(r.Header.Get("If-Match"))
	if observedETag == "*" {
		observedETag = ""
	} else if observedETag != "" && !existed {
		http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
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
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "caldav object store is not configured", http.StatusNotImplemented)
		return
	}
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	if ifMatch != "" && ifMatch != "*" {
		object, err := h.Store.LookupCalendarObject(r.Context(), userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			http.Error(w, "caldav object not found", http.StatusPreconditionFailed)
			return
		}
		if object.ETag != ifMatch {
			http.Error(w, "caldav object etag mismatch", http.StatusPreconditionFailed)
			return
		}
	}
	if _, err := store.DeleteObject(r.Context(), DeleteObjectRequest{UserID: userID, CalendarID: resource.CalendarID, ObjectName: resource.ObjectName}); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveObjectRequest(w http.ResponseWriter, r *http.Request) (string, ResourcePath, bool) {
	if h.Store == nil {
		http.Error(w, "caldav store is not configured", http.StatusInternalServerError)
		return "", ResourcePath{}, false
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return "", ResourcePath{}, false
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil || resource.Kind != ResourceCalendarObject {
		http.Error(w, "caldav object path is required", http.StatusNotFound)
		return "", ResourcePath{}, false
	}
	if resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return "", ResourcePath{}, false
	}
	return userID, resource, true
}

func writeCalendarObjectHeaders(w http.ResponseWriter, object CalendarObject) {
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("ETag", object.ETag)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
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
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return
	}
	report, err := ParseReport(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if report.Kind == ReportFreeBusyQuery {
		h.serveFreeBusyReport(w, r, userID, resource, report)
		return
	}
	responses, err := h.reportResponses(r.Context(), userID, resource, report)
	if err != nil {
		var invalidSyncToken InvalidSyncTokenError
		if errors.As(err, &invalidSyncToken) {
			writeDAVPreconditionError(w, http.StatusForbidden, "valid-sync-token", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body []byte
	if report.Kind == ReportSyncCollection {
		calendar, err := h.Store.LookupCalendar(r.Context(), userID, resource.CalendarID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, err = BuildSyncCollectionXML(responses, calendar.SyncToken)
	} else {
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
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return
	}
	depth, err := ParseDepth(r.Header.Get("Depth"), DepthInfinity)
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
	responses, err := h.propfindResponses(r.Context(), userID, resource, depth, propfind)
	if err != nil {
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

func (h *Handler) serveFreeBusyReport(w http.ResponseWriter, r *http.Request, userID string, resource ResourcePath, report ReportRequest) {
	if resource.Kind != ResourceCalendarCollection {
		http.Error(w, "free-busy-query requires a calendar collection resource", http.StatusForbidden)
		return
	}
	if report.TimeRange == nil {
		http.Error(w, "free-busy-query requires a time-range", http.StatusBadRequest)
		return
	}
	depth, err := ParseDepth(r.Header.Get("Depth"), DepthZero)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if depth == DepthInfinity {
		http.Error(w, "Depth: infinity is not supported for free-busy-query", http.StatusForbidden)
		return
	}
	body, err := h.freeBusyCalendar(r.Context(), userID, resource, *report.TimeRange, depth == DepthOne)
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

func (h *Handler) propfindResponses(ctx context.Context, userID string, resource ResourcePath, depth Depth, propfind PropfindRequest) ([]MultiStatusResponse, error) {
	switch resource.Kind {
	case ResourcePrincipal:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(principal.PrincipalPath, propfind, PrincipalProperties(principal))}, nil
	case ResourceCalendarHome:
		home, err := CalendarHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarHomeProperties(userID)
		if err != nil {
			return nil, err
		}
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
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.Store.ListCalendarObjects(ctx, userID, calendar.ID)
			if err != nil {
				return nil, err
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
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CalDAV resource")
	}
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
}

func (h *Handler) reportResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	switch report.Kind {
	case ReportCalendarMulti:
		if resource.Kind != ResourceCalendarCollection && resource.Kind != ResourceCalendarHome {
			return nil, fmt.Errorf("calendar-multiget requires a calendar collection or home resource")
		}
		return h.calendarMultigetResponses(ctx, userID, report)
	case ReportCalendarQuery:
		if resource.Kind != ResourceCalendarCollection {
			return nil, fmt.Errorf("calendar-query requires a calendar collection resource")
		}
		return h.calendarQueryResponses(ctx, userID, resource, report)
	case ReportSyncCollection:
		if resource.Kind != ResourceCalendarCollection {
			return nil, fmt.Errorf("sync-collection requires a calendar collection resource")
		}
		return h.syncCollectionResponses(ctx, userID, resource, report)
	default:
		return nil, fmt.Errorf("REPORT %s is not implemented", report.Kind)
	}
}

func (h *Handler) calendarMultigetResponses(ctx context.Context, userID string, report ReportRequest) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	for _, href := range report.Hrefs {
		resource, err := ParseResourcePath(href)
		if err != nil || resource.Kind != ResourceCalendarObject || resource.UserID != userID {
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
		if containsXMLName(report.Properties, PropCalendarData) {
			props = append(props, CalendarObjectDataProperty(object.ICS))
		}
		objectHref, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(objectHref, propfind, props))
	}
	return responses, nil
}

func (h *Handler) calendarQueryResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	objects, err := h.Store.ListCalendarObjects(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
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
		if containsXMLName(report.Properties, PropCalendarData) {
			props = append(props, CalendarObjectDataProperty(object.ICS))
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
}

func (h *Handler) freeBusyCalendar(ctx context.Context, userID string, resource ResourcePath, timeRange TimeRange, includeChildren bool) ([]byte, error) {
	if _, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID); err != nil {
		return nil, err
	}
	if !includeChildren {
		return BuildFreeBusyCalendar(userID, resource.CalendarID, timeRange, nil)
	}
	objects, err := h.Store.ListCalendarObjects(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
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

func (h *Handler) syncCollectionResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	calendar, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	if report.SyncToken != "" {
		if report.SyncToken != calendar.SyncToken {
			return nil, InvalidSyncTokenError{Token: report.SyncToken}
		}
		return nil, nil
	}
	objects, err := h.Store.ListCalendarObjects(ctx, userID, resource.CalendarID)
	if err != nil {
		return nil, err
	}
	if report.Limit > 0 && report.Limit < len(objects) {
		return nil, fmt.Errorf("sync-collection limit would truncate results")
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		if containsXMLName(report.Properties, PropCalendarData) {
			props = append(props, CalendarObjectDataProperty(object.ICS))
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
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
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<D:error xmlns:D="%s"><D:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
		DAVNamespace,
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
	return strings.Join([]string{
		MethodOptions,
		MethodPropfind,
		MethodReport,
		MethodMkcalendar,
		MethodGet,
		MethodHead,
		MethodPut,
		MethodDelete,
	}, ", ")
}
