package caldavgw

import (
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
