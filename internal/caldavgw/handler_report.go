package caldavgw

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
		return nil, fmt.Errorf("unsupported REPORT %s", report.Kind)
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
