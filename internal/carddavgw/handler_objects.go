package carddavgw

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
	userID, resource, _, ok := h.resolveObjectRequest(w, r, ContactsAccessRoleRead)
	if !ok {
		return
	}
	object, err := h.Store.LookupContactObject(r.Context(), userID, resource.AddressBookID, resource.ObjectName)
	if err != nil {
		http.Error(w, "carddav contact object not found", http.StatusNotFound)
		return
	}
	if ifMatch := conditionalHeaderValue(r.Header, "If-Match"); ifMatch != "" {
		ifMatchResult, err := ifMatchMatches(ifMatch, object.ETag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !ifMatchResult {
			http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
			return
		}
	}
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ifHeader != "" {
		matches, err := webDAVIfHeaderMatches(ifHeader, object.ETag, r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !matches {
			http.Error(w, "carddav contact object If header precondition failed", http.StatusPreconditionFailed)
			return
		}
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	if ifNoneMatch != "" {
		ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, object.ETag, true)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ifNoneMatchResult {
			writeContactObjectNotModifiedHeaders(w, object)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	if ifUnmodifiedSince != "" && objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
		http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	if ifNoneMatch == "" {
		ifModifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Modified-Since")
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if objectNotModifiedSince(ifModifiedSince, object.UpdatedAt) {
			writeContactObjectNotModifiedHeaders(w, object)
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}
	writeContactObjectHeaders(w, object)
	w.WriteHeader(http.StatusOK)
	if r.Method != MethodHead {
		_, _ = w.Write(object.VCard)
	}
}

func (h *Handler) servePutObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, actorUserID, ok := h.resolveObjectRequest(w, r, ContactsAccessRoleWrite)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "carddav object store is not configured", http.StatusNotImplemented)
		return
	}
	contentTypeVersion, err := validateVCardPutContentTypeHeader(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	ifHeader, err := conditionalIfHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	existed := false
	var existing ContactObject
	if object, err := h.Store.LookupContactObject(r.Context(), userID, resource.AddressBookID, resource.ObjectName); err == nil {
		existed = true
		existing = object
	}
	if existed && ifNoneMatch != "" {
		ifNoneMatchResult, err := ifNoneMatchMatches(ifNoneMatch, existing.ETag, false)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if ifNoneMatchResult {
			http.Error(w, "carddav contact object already exists", http.StatusPreconditionFailed)
			return
		}
	}
	observedETag := conditionalHeaderValue(r.Header, "If-Match")
	if observedETag == "*" {
		if !existed {
			http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
			return
		}
		observedETag = existing.ETag
	} else if observedETag != "" && !existed {
		http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
		return
	} else if observedETag != "" {
		ifMatchResult, err := ifMatchMatches(observedETag, existing.ETag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !ifMatchResult {
			http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
			return
		}
		observedETag = existing.ETag
	}
	if ifHeader != "" {
		currentETag := ""
		if existed {
			currentETag = existing.ETag
		}
		matches, err := webDAVIfHeaderMatches(ifHeader, currentETag, r.URL.Path)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if !matches {
			http.Error(w, "carddav contact object If header precondition failed", http.StatusPreconditionFailed)
			return
		}
		if existed {
			observedETag = existing.ETag
		}
	}
	ifUnmodifiedSince, err := conditionalDateHeaderValue(r.Header, "If-Unmodified-Since")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if ifUnmodifiedSince != "" && !existed {
		http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
		return
	}
	if objectModifiedSince(ifUnmodifiedSince, existing.UpdatedAt) {
		http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	body, err := readBoundedContactObjectBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	if contentTypeVersion != "" {
		bodyVersion, err := vCardBodyVersion(string(body))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if bodyVersion != contentTypeVersion {
			http.Error(w, "contact object content type version does not match vcard VERSION", http.StatusBadRequest)
			return
		}
	}
	object, err := store.UpsertContactObject(r.Context(), UpsertContactObjectRequest{
		UserID:        userID,
		ActorUserID:   actorUserID,
		AddressBookID: resource.AddressBookID,
		ObjectName:    resource.ObjectName,
		VCard:         body,
		ObservedETag:  observedETag,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeContactObjectHeaders(w, object)
	if existed {
		w.WriteHeader(http.StatusNoContent)
	} else {
		w.WriteHeader(http.StatusCreated)
	}
}

func (h *Handler) serveDeleteObject(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "carddav store is not configured", http.StatusInternalServerError)
		return
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCardDAVUnauthorized(w, err)
		return
	}
	actorUserID := userID
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if resource.Kind == ResourceAddressBookCollection {
		ownerID, _, ok := h.authorizeResource(w, r, userID, resource, ContactsAccessRoleManage)
		if !ok {
			return
		}
		userID = ownerID
		h.deleteAddressBookCollection(w, r, userID, actorUserID, resource)
		return
	}
	if resource.Kind != ResourceContactObject {
		http.Error(w, "DELETE requires an address-book collection or contact object path", http.StatusForbidden)
		return
	}
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, ContactsAccessRoleWrite)
	if !ok {
		return
	}
	userID = ownerID
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "carddav object store is not configured", http.StatusNotImplemented)
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
		object, err := h.Store.LookupContactObject(r.Context(), userID, resource.AddressBookID, resource.ObjectName)
		if err != nil {
			if ifMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
				http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
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
					http.Error(w, "carddav contact object if-none-match precondition failed", http.StatusPreconditionFailed)
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
					http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
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
					http.Error(w, "carddav contact object If header precondition failed", http.StatusPreconditionFailed)
					return
				}
			}
			if ifMatch != "" || ifHeader != "" {
				observedETag = object.ETag
			}
			if objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
				http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
				return
			}
		}
	}
	if _, err := store.DeleteContactObject(r.Context(), DeleteContactObjectRequest{
		UserID:        userID,
		ActorUserID:   actorUserID,
		AddressBookID: resource.AddressBookID,
		ObjectName:    resource.ObjectName,
		ObservedETag:  observedETag,
	}); err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "carddav contact object precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) resolveObjectRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, string, bool) {
	userID, resource, actorUserID, ok := h.resolveResourceRequest(w, r, requiredRole)
	if !ok {
		return "", ResourcePath{}, "", false
	}
	if resource.Kind != ResourceContactObject {
		http.Error(w, "carddav contact object path is required", http.StatusNotFound)
		return "", ResourcePath{}, "", false
	}
	return userID, resource, actorUserID, true
}

func (h *Handler) resolveResourceRequest(w http.ResponseWriter, r *http.Request, requiredRole string) (string, ResourcePath, string, bool) {
	if h.Store == nil {
		http.Error(w, "carddav store is not configured", http.StatusInternalServerError)
		return "", ResourcePath{}, "", false
	}
	resolve := h.ResolveUser
	if resolve == nil {
		resolve = QueryUserResolver
	}
	userID, err := resolve(r)
	if err != nil {
		writeCardDAVUnauthorized(w, err)
		return "", ResourcePath{}, "", false
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return "", ResourcePath{}, "", false
	}
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, requiredRole)
	if !ok {
		return "", ResourcePath{}, "", false
	}
	return ownerID, resource, userID, true
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
		http.Error(w, "carddav resource is not accessible", http.StatusForbidden)
		return "", AccessDecision{}, false
	}
	decision, err := h.AccessAuthorizer.AuthorizeAddressBookAccess(r.Context(), AccessRequest{
		ActorUserID:  actorID,
		OwnerUserID:  ownerID,
		Resource:     resource,
		RequiredRole: requiredRole,
	})
	if err != nil {
		http.Error(w, "carddav access policy unavailable", http.StatusInternalServerError)
		return "", AccessDecision{}, false
	}
	if !decision.Allowed {
		http.Error(w, "carddav resource is not accessible", http.StatusForbidden)
		return "", AccessDecision{}, false
	}
	return ownerID, decision, true
}

func writeContactObjectHeaders(w http.ResponseWriter, object ContactObject) {
	w.Header().Set("Content-Type", "text/vcard; charset=utf-8")
	w.Header().Set("ETag", object.ETag)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Length", strconv.FormatInt(object.Size, 10))
	if !object.UpdatedAt.IsZero() {
		w.Header().Set("Last-Modified", formatHTTPDate(object.UpdatedAt))
	}
}

func writeContactObjectNotModifiedHeaders(w http.ResponseWriter, object ContactObject) {
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

func validateVCardPutContentType(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("contact object content type must not contain line breaks")
	}
	mediaType, params, err := mime.ParseMediaType(value)
	if err != nil {
		return "", fmt.Errorf("contact object content type is invalid")
	}
	if !strings.EqualFold(mediaType, "text/vcard") {
		return "", fmt.Errorf("contact object content type must be text/vcard")
	}
	version := strings.TrimSpace(params["version"])
	if version == "" {
		return "", nil
	}
	if version != "3.0" && version != "4.0" {
		return "", fmt.Errorf("contact object content type version must be 3.0 or 4.0")
	}
	return version, nil
}

func validateVCardPutContentTypeHeader(header http.Header) (string, error) {
	values := header.Values("Content-Type")
	if len(values) > 1 {
		return "", fmt.Errorf("contact object content type must be specified at most once")
	}
	if len(values) == 0 {
		return "", nil
	}
	return validateVCardPutContentType(values[0])
}

func readBoundedContactObjectBody(r io.Reader) ([]byte, error) {
	if r == nil {
		return nil, fmt.Errorf("contact object body is required")
	}
	limited := io.LimitReader(r, MaxContactObjectBytes+1)
	body, err := io.ReadAll(limited)
	if err != nil {
		return nil, fmt.Errorf("read contact object body: %w", err)
	}
	if len(body) > MaxContactObjectBytes {
		return nil, fmt.Errorf("contact object body exceeds %d bytes", MaxContactObjectBytes)
	}
	return body, nil
}
