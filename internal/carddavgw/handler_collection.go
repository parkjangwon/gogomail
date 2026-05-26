package carddavgw

import (
	"mime"
	"net/http"
	"strings"
)

func (h *Handler) serveMkcol(w http.ResponseWriter, r *http.Request) {
	if h.Store == nil {
		http.Error(w, "carddav store is not configured", http.StatusInternalServerError)
		return
	}
	store, ok := h.Store.(AddressBookCreator)
	if !ok {
		http.Error(w, "carddav address-book creator is not configured", http.StatusNotImplemented)
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
	if err != nil || resource.Kind != ResourceAddressBookCollection {
		http.Error(w, "MKCOL requires an address-book collection path", http.StatusConflict)
		return
	}
	ownerID, _, ok := h.authorizeResource(w, r, userID, resource, ContactsAccessRoleManage)
	if !ok {
		return
	}
	userID = ownerID
	if _, err := h.Store.LookupPrincipal(r.Context(), userID); err != nil {
		http.Error(w, "carddav address-book home not found", http.StatusConflict)
		return
	}
	book, err := h.Store.LookupAddressBook(r.Context(), userID, resource.AddressBookID)
	if err == nil {
		if !h.checkAddressBookCollectionCreatePreconditions(w, r, userID, book, true) {
			return
		}
		http.Error(w, "carddav address book already exists", http.StatusMethodNotAllowed)
		return
	}
	if _, err := ValidateAddressBookPathID(resource.AddressBookID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if !h.checkAddressBookCollectionCreatePreconditions(w, r, userID, AddressBook{}, false) {
		return
	}
	if ok := validateDAVXMLContentType(w, r, "MKCOL"); !ok {
		return
	}
	req, err := ParseMKAddressBook(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if req.InvalidResourceType || len(req.Unsupported) > 0 {
		body, err := BuildMKCOLResponseXML(mkcolFailurePropStats(req))
		if err != nil {
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store, no-cache")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write(body)
		return
	}
	if !req.HasResourceType {
		http.Error(w, "MKCOL requires DAV:resourcetype with DAV:collection and CARDDAV:addressbook", http.StatusForbidden)
		return
	}
	book, err = store.CreateAddressBookAtPath(r.Context(), CreateAddressBookAtPathRequest{
		UserID:          userID,
		ActorUserID:     actorUserID,
		AddressBookID:   resource.AddressBookID,
		Name:            req.DisplayName,
		NameLang:        req.DisplayNameLang,
		Description:     req.Description,
		DescriptionLang: req.DescriptionLang,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	location, err := AddressBookCollectionPath(userID, book.ID)
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

func mkcolFailurePropStats(req MKAddressBookRequest) []PropStatus {
	failed := make([]PropertyResult, 0, 1+len(req.Unsupported))
	if req.InvalidResourceType {
		failed = append(failed, PropertyResult{Name: PropResourceType})
	}
	for _, name := range req.Unsupported {
		failed = append(failed, PropertyResult{Name: name})
	}

	dependencies := make([]PropertyResult, 0, len(req.Properties))
	for _, name := range req.Properties {
		if req.InvalidResourceType && name == PropResourceType {
			continue
		}
		dependencies = append(dependencies, PropertyResult{Name: name})
	}

	stats := make([]PropStatus, 0, 2)
	if len(failed) > 0 {
		sortPropertyResults(failed)
		status := PropStatus{StatusCode: http.StatusForbidden, Properties: failed}
		if req.InvalidResourceType {
			status.Error = XMLName{Space: DAVNamespace, Local: "valid-resourcetype"}
			status.ResponseDescription = "Resource type is not supported by this server"
		}
		stats = append(stats, status)
	}
	if len(dependencies) > 0 {
		sortPropertyResults(dependencies)
		stats = append(stats, PropStatus{StatusCode: http.StatusFailedDependency, Properties: dependencies})
	}
	return stats
}

func (h *Handler) serveProppatch(w http.ResponseWriter, r *http.Request) {
	userID, resource, actorUserID, ok := h.resolveResourceRequest(w, r, ContactsAccessRoleWrite)
	if !ok {
		return
	}
	if resource.Kind != ResourceAddressBookCollection {
		http.Error(w, "PROPPATCH requires an address-book collection path", http.StatusForbidden)
		return
	}
	store, ok := h.Store.(AddressBookUpdater)
	if !ok {
		http.Error(w, "carddav address-book updater is not configured", http.StatusNotImplemented)
		return
	}
	observedETag, ok := h.checkAddressBookCollectionPreconditions(w, r, userID, resource.AddressBookID)
	if !ok {
		return
	}
	patch, err := ParseProppatch(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	href, err := AddressBookCollectionPath(userID, resource.AddressBookID)
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
	book, err := store.UpdateAddressBookProperties(r.Context(), UpdateAddressBookRequest{
		UserID:          userID,
		ActorUserID:     actorUserID,
		AddressBookID:   resource.AddressBookID,
		Name:            patch.Name,
		NameLang:        patch.NameLang,
		Description:     patch.Description,
		DescriptionLang: patch.DescriptionLang,
		ObservedETag:    observedETag,
	})
	if err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "carddav address book collection precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	href, err = AddressBookCollectionPath(userID, book.ID)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchResponse(href, book, patch.Properties)})
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

func (h *Handler) deleteAddressBookCollection(w http.ResponseWriter, r *http.Request, userID string, actorUserID string, resource ResourcePath) {
	store, ok := h.Store.(AddressBookDeleter)
	if !ok {
		http.Error(w, "carddav address-book deleter is not configured", http.StatusNotImplemented)
		return
	}
	observedETag, ok := h.checkAddressBookCollectionPreconditions(w, r, userID, resource.AddressBookID)
	if !ok {
		return
	}
	if _, err := store.DeleteAddressBook(r.Context(), DeleteAddressBookRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: resource.AddressBookID, ObservedETag: observedETag}); err != nil {
		if observedETag != "" && (strings.Contains(err.Error(), "etag mismatch") || strings.Contains(err.Error(), "not found")) {
			http.Error(w, "carddav address book collection precondition failed", http.StatusPreconditionFailed)
			return
		}
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) checkAddressBookCollectionPreconditions(w http.ResponseWriter, r *http.Request, userID string, addressBookID string) (string, bool) {
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
		book, err := h.Store.LookupAddressBook(r.Context(), userID, addressBookID)
		if err != nil {
			if ifMatch != "" || ifHeader != "" || ifUnmodifiedSince != "" {
				http.Error(w, "carddav address book not found", http.StatusPreconditionFailed)
				return "", false
			}
			return "", true
		}
		etag, err := AddressBookCollectionETag(userID, book)
		if err != nil {
			http.Error(w, "carddav address book collection etag unavailable", http.StatusPreconditionFailed)
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
					http.Error(w, "carddav address book collection if-none-match precondition failed", http.StatusPreconditionFailed)
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
					http.Error(w, "carddav address book collection etag mismatch", http.StatusPreconditionFailed)
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
				http.Error(w, "carddav address book collection If header precondition failed", http.StatusPreconditionFailed)
				return "", false
			}
		}
		if objectModifiedSince(ifUnmodifiedSince, book.UpdatedAt) {
			http.Error(w, "carddav address book modified since precondition", http.StatusPreconditionFailed)
			return "", false
		}
		return etag, true
	}
	return "", true
}

func (h *Handler) checkAddressBookCollectionCreatePreconditions(w http.ResponseWriter, r *http.Request, userID string, book AddressBook, exists bool) bool {
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
				http.Error(w, "carddav address book collection If header precondition failed", http.StatusPreconditionFailed)
				return false
			}
		}
		if ifMatch != "" || ifUnmodifiedSince != "" {
			http.Error(w, "carddav address book create precondition failed", http.StatusPreconditionFailed)
			return false
		}
		return true
	}
	if ifMatch != "" || ifNoneMatch != "" || ifHeader != "" {
		etag, err := AddressBookCollectionETag(userID, book)
		if err != nil {
			http.Error(w, "carddav address book collection etag unavailable", http.StatusPreconditionFailed)
			return false
		}
		if ifMatch != "" {
			ifMatchResult, err := ifMatchMatches(ifMatch, etag)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return false
			}
			if !ifMatchResult {
				http.Error(w, "carddav address book collection etag mismatch", http.StatusPreconditionFailed)
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
				http.Error(w, "carddav address book collection if-none-match precondition failed", http.StatusPreconditionFailed)
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
				http.Error(w, "carddav address book collection If header precondition failed", http.StatusPreconditionFailed)
				return false
			}
		}
	}
	if objectModifiedSince(ifUnmodifiedSince, book.UpdatedAt) {
		http.Error(w, "carddav address book modified since precondition", http.StatusPreconditionFailed)
		return false
	}
	return true
}

func proppatchResponse(href string, book AddressBook, properties []XMLName) MultiStatusResponse {
	uniqueProperties := uniqueXMLNames(properties)
	results := make([]PropertyResult, 0, len(uniqueProperties))
	for _, prop := range uniqueProperties {
		switch prop {
		case PropDisplayName:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: book.Name, Lang: book.NameLang}, Found: true})
		case PropAddressBookDescription:
			results = append(results, PropertyResult{Name: prop, Value: PropertyValue{Text: book.Description, Lang: book.DescriptionLang}, Found: true})
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
		failedProperties := uniqueXMLNames(patch.Properties)
		failed := make([]PropertyResult, 0, len(failedProperties))
		for _, prop := range failedProperties {
			failed = append(failed, PropertyResult{Name: prop})
		}
		propStats = append(propStats, PropStatus{StatusCode: http.StatusFailedDependency, Properties: failed})
	}
	return MultiStatusResponse{Href: href, PropStats: propStats}
}

func uniqueXMLNames(names []XMLName) []XMLName {
	if len(names) < 2 {
		return names
	}
	seen := make(map[XMLName]struct{}, len(names))
	unique := make([]XMLName, 0, len(names))
	for _, name := range names {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		unique = append(unique, name)
	}
	return unique
}
