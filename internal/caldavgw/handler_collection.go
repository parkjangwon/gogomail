package caldavgw

import (
	"context"
	"mime"
	"net/http"
	"strings"

	"github.com/google/uuid"
)

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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	if len(patch.Unsupported) > 0 || len(patch.Protected) > 0 {
		body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchFailureResponse(href, patch)})
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchResponse(href, calendar, patch.Properties)})
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
			http.Error(w, "internal server error", http.StatusInternalServerError)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
