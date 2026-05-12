package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
	"time"
)

type contactObjectLookupKey struct {
	addressBookID string
	objectName    string
}

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListAddressBookCollections(ctx context.Context, userID string) ([]AddressBook, error)
	LookupAddressBook(ctx context.Context, userID string, addressBookID string) (AddressBook, error)
	ListAddressBookObjects(ctx context.Context, userID string, addressBookID string) ([]ContactObject, error)
	LookupContactObject(ctx context.Context, userID string, addressBookID string, objectName string) (ContactObject, error)
}

type AddressBookObjectLimiter interface {
	ListAddressBookObjectsLimit(ctx context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error)
}

type AddressBookObjectBatchStore interface {
	ListContactObjectsByNameGroups(ctx context.Context, userID string, objectNamesByAddressBook map[string][]string, status string) ([]ContactObject, error)
}

type AddressBookCreator interface {
	CreateAddressBookAtPath(ctx context.Context, req CreateAddressBookAtPathRequest) (AddressBook, error)
}

type AddressBookDeleter interface {
	DeleteAddressBook(ctx context.Context, req DeleteAddressBookRequest) (AddressBook, error)
}

type UserResolver func(*http.Request) (string, error)

const (
	ContactsAccessRoleRead   = "read"
	ContactsAccessRoleWrite  = "write"
	ContactsAccessRoleManage = "manage"
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
	AuthorizeAddressBookAccess(ctx context.Context, req AccessRequest) (AccessDecision, error)
}

type Handler struct {
	Store            DiscoveryStore
	ResolveUser      UserResolver
	AccessAuthorizer AccessAuthorizer
	IncludeSync      bool
}

type SyncChangeStore interface {
	ListAddressBookChangesSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error)
}

type AddressBookChangeWithObjectStore interface {
	ListAddressBookChangesWithObjectsSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChangeWithObject, error)
}

type AddressBookUpdater interface {
	UpdateAddressBookProperties(ctx context.Context, req UpdateAddressBookRequest) (AddressBook, error)
}

type ObjectStore interface {
	UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error)
	DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error)
}

type ObjectWalker interface {
	WalkAddressBookObjects(ctx context.Context, userID string, addressBookID string, yield func(ContactObject) (bool, error)) error
}

type AddressBookQueryCandidateWalker interface {
	WalkAddressBookQueryCandidates(ctx context.Context, userID string, addressBookID string, containsText string, yield func(ContactObject) (bool, error)) error
}

type PrincipalSearchStore interface {
	SearchAddressBookObjects(ctx context.Context, userID string, addressBookID string, property string, match string, test string) ([]ContactObject, error)
}

type InvalidSyncTokenError struct {
	Token string
}

func (e InvalidSyncTokenError) Error() string {
	return "CardDAV sync-token is unknown or expired"
}

type TruncatedResultsError struct {
	Operation string
}

func (e TruncatedResultsError) Error() string {
	operation := strings.TrimSpace(e.Operation)
	if operation == "" {
		operation = "CardDAV request"
	}
	return operation + " would truncate results"
}

func NewHandler(store DiscoveryStore, resolveUser UserResolver) *Handler {
	return &Handler{Store: store, ResolveUser: resolveUser, IncludeSync: true}
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
		http.Error(w, "carddav handler is not configured", http.StatusInternalServerError)
		return
	}
	if r.URL.Path == WellKnownCardDAVPath {
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
	case MethodMkcol:
		h.serveMkcol(w, r)
	default:
		w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
			http.Error(w, err.Error(), http.StatusInternalServerError)
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
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	body, err := BuildMultiStatusXML([]MultiStatusResponse{proppatchResponse(href, book, patch.Properties)})
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
	w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.includeSyncCollection()), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) includeSyncCollection() bool {
	if !h.IncludeSync {
		return false
	}
	_, ok := h.Store.(SyncChangeStore)
	return ok
}

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
	if ifMatch := conditionalHeaderValue(r.Header, "If-Match"); ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
		http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
		return
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
	if objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
		http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	ifNoneMatch := conditionalHeaderValue(r.Header, "If-None-Match")
	if ifNoneMatchMatches(ifNoneMatch, object.ETag) {
		writeContactObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
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
	if existed && ifNoneMatchMatches(ifNoneMatch, existing.ETag) {
		http.Error(w, "carddav contact object already exists", http.StatusPreconditionFailed)
		return
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
	} else if observedETag != "" && !ifMatchMatches(observedETag, existing.ETag) {
		http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
		return
	} else if observedETag != "" {
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
			if ifNoneMatch != "" && ifNoneMatchMatches(ifNoneMatch, object.ETag) {
				http.Error(w, "carddav contact object if-none-match precondition failed", http.StatusPreconditionFailed)
				return
			}
			if ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
				http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
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

func (h *Handler) serveReport(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, ContactsAccessRoleRead)
	if !ok {
		return
	}
	userID = ownerID
	depthHeader, err := depthHeaderValue(r.Header)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	depthHeaderPresent := strings.TrimSpace(depthHeader) != ""
	depth, err := ParseDepth(depthHeader, DepthZero)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if depth == DepthInfinity {
		http.Error(w, "Depth: infinity is not supported for CardDAV REPORT", http.StatusForbidden)
		return
	}
	report, err := ParseReport(r.Body)
	if err != nil {
		var unsupportedAddressData UnsupportedAddressDataError
		if errors.As(err, &unsupportedAddressData) {
			writeCardDAVPreconditionError(w, http.StatusForbidden, "supported-address-data", err.Error())
			return
		}
		var unsupportedCollation UnsupportedCollationError
		if errors.As(err, &unsupportedCollation) {
			writeCardDAVPreconditionError(w, http.StatusForbidden, "supported-collation", err.Error())
			return
		}
		var unsupportedFilterElement UnsupportedFilterElementError
		if errors.As(err, &unsupportedFilterElement) {
			writeCardDAVPreconditionError(w, http.StatusForbidden, "supported-filter", err.Error())
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	var body []byte
	if report.Kind == ReportSyncCollection {
		if depth != DepthZero {
			http.Error(w, "sync-collection requires Depth: 0", http.StatusBadRequest)
			return
		}
		responses, syncToken, err := h.syncCollectionReport(r.Context(), userID, resource, report, decision.Privileges)
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
		responses, err := h.reportResponses(r.Context(), userID, resource, depth, depthHeaderPresent, report, decision.Privileges)
		if err != nil {
			var unsupportedFilter UnsupportedFilterError
			if errors.As(err, &unsupportedFilter) {
				writeCardDAVPreconditionError(w, http.StatusForbidden, "supported-filter", err.Error())
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

func (h *Handler) servePropfind(w http.ResponseWriter, r *http.Request) {
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	ownerID, decision, ok := h.authorizeResource(w, r, userID, resource, ContactsAccessRoleRead)
	if !ok {
		return
	}
	depth, err := parseDepthHeader(r.Header, DepthInfinity)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if depth == DepthInfinity {
		http.Error(w, "Depth: infinity is not supported for CardDAV discovery", http.StatusForbidden)
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

func (h *Handler) propfindResponses(ctx context.Context, userID string, actorUserID string, resource ResourcePath, depth Depth, propfind PropfindRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	switch resource.Kind {
	case ResourceRoot:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		props, err := withCurrentUserPrincipal(PrincipalProperties(principal), actorUserID)
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
	case ResourceAddressBookHome:
		home, err := AddressBookHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := AddressBookHomeProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceAddressBookHome, currentUserPrivileges)
		responses := []MultiStatusResponse{responseForProperties(home, propfind, props)}
		if depth == DepthOne {
			books, err := h.Store.ListAddressBookCollections(ctx, userID)
			if err != nil {
				return nil, err
			}
			for _, book := range books {
				href, err := AddressBookCollectionPath(userID, book.ID)
				if err != nil {
					return nil, err
				}
				props, err := AddressBookCollectionProperties(userID, book, h.includeSyncCollection())
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceAddressBookCollection, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceAddressBookCollection:
		book, err := h.Store.LookupAddressBook(ctx, userID, resource.AddressBookID)
		if err != nil {
			return nil, err
		}
		href, err := AddressBookCollectionPath(userID, book.ID)
		if err != nil {
			return nil, err
		}
		props, err := AddressBookCollectionProperties(userID, book, h.includeSyncCollection())
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceAddressBookCollection, currentUserPrivileges)
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.listAddressBookObjectsBounded(ctx, userID, book.ID, MaxWebDAVReportLimit+1)
			if err != nil {
				return nil, err
			}
			if len(objects) > MaxWebDAVReportLimit {
				return nil, TruncatedResultsError{Operation: "address-book collection PROPFIND"}
			}
			for _, object := range objects {
				href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
				if err != nil {
					return nil, err
				}
				props, err := ContactObjectProperties(userID, object)
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceContactObject:
		if depth != DepthZero {
			return nil, fmt.Errorf("contact object PROPFIND requires Depth: 0")
		}
		object, err := h.Store.LookupContactObject(ctx, userID, resource.AddressBookID, resource.ObjectName)
		if err != nil {
			return nil, err
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CardDAV resource")
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
			if ifNoneMatch != "" && ifNoneMatchMatches(ifNoneMatch, etag) {
				http.Error(w, "carddav address book collection if-none-match precondition failed", http.StatusPreconditionFailed)
				return "", false
			}
			if ifMatch != "" && !ifMatchMatches(ifMatch, etag) {
				http.Error(w, "carddav address book collection etag mismatch", http.StatusPreconditionFailed)
				return "", false
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
		if ifMatch != "" || ifUnmodifiedSince != "" {
			http.Error(w, "carddav address book create precondition failed", http.StatusPreconditionFailed)
			return false
		}
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
		return true
	}
	if ifMatch != "" || ifNoneMatch != "" || ifHeader != "" {
		etag, err := AddressBookCollectionETag(userID, book)
		if err != nil {
			http.Error(w, "carddav address book collection etag unavailable", http.StatusPreconditionFailed)
			return false
		}
		if ifMatch != "" && !ifMatchMatches(ifMatch, etag) {
			http.Error(w, "carddav address book collection etag mismatch", http.StatusPreconditionFailed)
			return false
		}
		if ifNoneMatch != "" && ifNoneMatchMatches(ifNoneMatch, etag) {
			http.Error(w, "carddav address book collection if-none-match precondition failed", http.StatusPreconditionFailed)
			return false
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
		http.Error(w, err.Error(), http.StatusUnauthorized)
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
	role := ContactsAccessRoleRead
	for _, privilege := range privileges {
		if privilege == PrivilegeBind || privilege == PrivilegeUnbind {
			role = ContactsAccessRoleManage
			break
		}
		if privilege == PrivilegeWriteContent || privilege == PrivilegeWriteProperties {
			role = ContactsAccessRoleWrite
		}
	}
	switch kind {
	case ResourceAddressBookHome:
		if role == ContactsAccessRoleManage {
			return addressBookHomePrivileges()
		}
		return readOnlyPrivileges()
	case ResourceAddressBookCollection:
		if role == ContactsAccessRoleWrite || role == ContactsAccessRoleManage {
			return addressBookCollectionPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceContactObject:
		if role == ContactsAccessRoleWrite || role == ContactsAccessRoleManage {
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

func depthHeaderValue(header http.Header) (string, error) {
	values := header.Values("Depth")
	if len(values) > 1 {
		return "", fmt.Errorf("Depth header must not be repeated")
	}
	if len(values) == 0 {
		return "", nil
	}
	return values[0], nil
}

func parseDepthHeader(header http.Header, fallback Depth) (Depth, error) {
	value, err := depthHeaderValue(header)
	if err != nil {
		return "", err
	}
	return ParseDepth(value, fallback)
}

func (h *Handler) reportResponses(ctx context.Context, userID string, resource ResourcePath, depth Depth, depthHeaderPresent bool, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	switch report.Kind {
	case ReportAddressBookMulti:
		if resource.Kind != ResourceAddressBookCollection && resource.Kind != ResourceAddressBookHome {
			return nil, fmt.Errorf("addressbook-multiget requires an address-book collection or home resource")
		}
		if !depthHeaderPresent {
			return nil, fmt.Errorf("addressbook-multiget requires a Depth header")
		}
		return h.addressBookMultigetResponses(ctx, userID, resource, report, currentUserPrivileges)
	case ReportAddressBookQuery:
		if resource.Kind != ResourceAddressBookCollection {
			return nil, fmt.Errorf("addressbook-query requires an address-book collection resource")
		}
		if !depthHeaderPresent {
			return nil, fmt.Errorf("addressbook-query requires a Depth header")
		}
		if err := validateAddressBookQueryFilterSupport(report.Filter); err != nil {
			return nil, err
		}
		if depth == DepthZero {
			return nil, nil
		}
		return h.addressBookQueryResponses(ctx, userID, resource, report, currentUserPrivileges)
	case ReportSyncCollection:
		if resource.Kind != ResourceAddressBookCollection {
			return nil, fmt.Errorf("sync-collection requires an address-book collection resource")
		}
		responses, _, err := h.syncCollectionReport(ctx, userID, resource, report, currentUserPrivileges)
		return responses, err
	case ReportPrincipalPropertySearch:
		if resource.Kind != ResourcePrincipal && resource.Kind != ResourcePrincipalCollection {
			return nil, fmt.Errorf("principal-property-search requires a principal resource")
		}
		return h.principalPropertySearchResponses(ctx, userID, resource, report, currentUserPrivileges)
	default:
		return nil, fmt.Errorf("REPORT %s is not implemented", report.Kind)
	}
}

func (h *Handler) addressBookMultigetResponses(ctx context.Context, userID string, requestResource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	type requestedContactObject struct {
		href          string
		addressBookID string
		objectName    string
		valid         bool
	}
	requested := make([]requestedContactObject, 0, len(report.Hrefs))
	requestedIndex := make(map[contactObjectLookupKey]struct{}, len(report.Hrefs))
	for _, href := range report.Hrefs {
		resource, err := ParseResourceHref(href)
		if err != nil || resource.Kind != ResourceContactObject || resource.UserID != userID || !multigetHrefInScope(requestResource, resource) {
			requested = append(requested, requestedContactObject{href: href})
			continue
		}
		requested = append(requested, requestedContactObject{
			href:          href,
			addressBookID: resource.AddressBookID,
			objectName:    resource.ObjectName,
			valid:         true,
		})
		requestedIndex[contactObjectLookupKey{addressBookID: resource.AddressBookID, objectName: resource.ObjectName}] = struct{}{}
	}
	requestedByAddressBook := make(map[string][]string)
	for key := range requestedIndex {
		requestedByAddressBook[key.addressBookID] = append(requestedByAddressBook[key.addressBookID], key.objectName)
	}
	objectsByKey, err := h.lookupContactObjectsByNames(ctx, userID, requestedByAddressBook)
	if err != nil {
		return nil, err
	}
	for _, ref := range requested {
		if !ref.valid {
			responses = append(responses, notFoundResponse(ref.href, report.Properties))
			continue
		}
		object, ok := objectsByKey[contactObjectLookupKey{addressBookID: ref.addressBookID, objectName: ref.objectName}]
		if !ok {
			responses = append(responses, notFoundResponse(ref.href, report.Properties))
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, err
			}
			props = append(props, dataProp)
		}
		objectHref, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(objectHref, propfind, props))
	}
	return responses, nil
}

func (h *Handler) lookupContactObjectsByNames(ctx context.Context, userID string, objectNamesByAddressBook map[string][]string) (map[contactObjectLookupKey]ContactObject, error) {
	objectsByKey := make(map[contactObjectLookupKey]ContactObject)
	if len(objectNamesByAddressBook) == 0 {
		return objectsByKey, nil
	}
	if batchStore, ok := h.Store.(AddressBookObjectBatchStore); ok {
		objects, err := batchStore.ListContactObjectsByNameGroups(ctx, userID, objectNamesByAddressBook, AddressBookStatusActive)
		if err != nil {
			return nil, err
		}
		for _, object := range objects {
			objectsByKey[contactObjectLookupKey{addressBookID: object.AddressBookID, objectName: object.ObjectName}] = object
		}
		return objectsByKey, nil
	}
	for addressBookID, objectNames := range objectNamesByAddressBook {
		for _, objectName := range objectNames {
			object, err := h.Store.LookupContactObject(ctx, userID, addressBookID, objectName)
			if err != nil {
				continue
			}
			objectsByKey[contactObjectLookupKey{addressBookID: object.AddressBookID, objectName: object.ObjectName}] = object
		}
	}
	return objectsByKey, nil
}

func multigetHrefInScope(requestResource ResourcePath, hrefResource ResourcePath) bool {
	switch requestResource.Kind {
	case ResourceAddressBookHome:
		return requestResource.UserID == hrefResource.UserID
	case ResourceAddressBookCollection:
		return requestResource.UserID == hrefResource.UserID && requestResource.AddressBookID == hrefResource.AddressBookID
	default:
		return false
	}
}

func (h *Handler) addressBookQueryResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	if candidateWalker, ok := h.Store.(AddressBookQueryCandidateWalker); ok {
		if containsText, ok := addressBookQueryCandidateText(report.Filter); ok {
			return h.walkAddressBookQueryCandidates(ctx, candidateWalker, userID, resource, report, currentUserPrivileges, containsText)
		}
	}
	if walker, ok := h.Store.(ObjectWalker); ok {
		return h.walkAddressBookQueryResponses(ctx, walker, userID, resource, report, currentUserPrivileges)
	}
	objects, err := h.Store.ListAddressBookObjects(ctx, userID, resource.AddressBookID)
	if err != nil {
		return nil, err
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	for _, object := range objects {
		if len(responses) >= limit {
			break
		}
		if !contactObjectMatchesFilter(object, report.Filter) {
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, err
			}
			props = append(props, dataProp)
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
}

func (h *Handler) walkAddressBookQueryCandidates(ctx context.Context, walker AddressBookQueryCandidateWalker, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName, containsText string) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	responses := make([]MultiStatusResponse, 0, limit)
	err := walker.WalkAddressBookQueryCandidates(ctx, userID, resource.AddressBookID, containsText, func(object ContactObject) (bool, error) {
		if len(responses) >= limit {
			return false, nil
		}
		if !contactObjectMatchesFilter(object, report.Filter) {
			return true, nil
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return false, err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return false, err
			}
			props = append(props, dataProp)
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return false, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
		return len(responses) < limit, nil
	})
	if err != nil {
		return nil, err
	}
	return responses, nil
}

func (h *Handler) walkAddressBookQueryResponses(ctx context.Context, walker ObjectWalker, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	responses := make([]MultiStatusResponse, 0, limit)
	err := walker.WalkAddressBookObjects(ctx, userID, resource.AddressBookID, func(object ContactObject) (bool, error) {
		if len(responses) >= limit {
			return false, nil
		}
		if !contactObjectMatchesFilter(object, report.Filter) {
			return true, nil
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return false, err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return false, err
			}
			props = append(props, dataProp)
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return false, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
		return len(responses) < limit, nil
	})
	if err != nil {
		return nil, err
	}
	return responses, nil
}

func addressBookQueryCandidateText(filter AddressBookQueryFilter) (string, bool) {
	if len(filter.PropFilters) == 0 {
		return "", false
	}
	if filter.Test != FilterTestAllOf && len(filter.PropFilters) != 1 {
		return "", false
	}
	for _, propFilter := range filter.PropFilters {
		if text, ok := necessaryPropFilterCandidateText(propFilter); ok {
			return text, true
		}
	}
	return "", false
}

func necessaryPropFilterCandidateText(filter CardDAVPropFilter) (string, bool) {
	if filter.IsNotDefined {
		return "", false
	}
	conditionCount := len(filter.TextMatches) + len(filter.ParamFilters)
	if conditionCount == 0 {
		return "", false
	}
	if filter.Test != FilterTestAllOf && conditionCount != 1 {
		return "", false
	}
	for _, match := range filter.TextMatches {
		if textMatchCanSeedAddressBookQuery(match) {
			return match.Text, true
		}
	}
	for _, paramFilter := range filter.ParamFilters {
		if paramFilter.IsNotDefined || !paramFilter.HasTextMatch {
			continue
		}
		if textMatchCanSeedAddressBookQuery(paramFilter.TextMatch) {
			return paramFilter.TextMatch.Text, true
		}
	}
	return "", false
}

func textMatchCanSeedAddressBookQuery(match CardDAVTextMatch) bool {
	if match.Negate || match.Text == "" {
		return false
	}
	for _, r := range match.Text {
		if r > 0x7f {
			return false
		}
	}
	return true
}

func (h *Handler) principalPropertySearchResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, error) {
	store, ok := h.Store.(PrincipalSearchStore)
	if !ok {
		return nil, fmt.Errorf("carddav principal search store is not configured")
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0)
	books, err := h.Store.ListAddressBookCollections(ctx, userID)
	if err != nil {
		return nil, err
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	for _, book := range books {
		if len(responses) >= limit {
			break
		}
		objects, err := store.SearchAddressBookObjects(ctx, userID, book.ID, report.PrincipalPropertySearchMatch, report.PrincipalPropertySearchTest, report.PrincipalPropertySearchMatch)
		if err != nil {
			return nil, err
		}
		for _, object := range objects {
			if len(responses) >= limit {
				break
			}
			props, err := ContactObjectProperties(userID, object)
			if err != nil {
				return nil, err
			}
			props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
			if containsXMLName(report.Properties, PropAddressData) {
				dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
				if err != nil {
					return nil, err
				}
				props = append(props, dataProp)
			}
			href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
			if err != nil {
				return nil, err
			}
			responses = append(responses, responseForProperties(href, propfind, props))
		}
	}
	return responses, nil
}

func contactObjectMatchesFilter(object ContactObject, filter AddressBookQueryFilter) bool {
	if len(filter.PropFilters) == 0 {
		return true
	}
	lines, err := unfoldVCardLines(string(object.VCard))
	if err != nil {
		return false
	}
	parsedLines := make([]vCardContentLine, 0, len(lines))
	for _, line := range lines {
		parsed, err := parseVCardContentLineParts(line)
		if err != nil {
			continue
		}
		parsedLines = append(parsedLines, parsed)
	}
	if filter.Test == FilterTestAllOf {
		for _, propFilter := range filter.PropFilters {
			if !vCardPropFilterApplies(parsedLines, propFilter) {
				return false
			}
		}
		return true
	}
	for _, propFilter := range filter.PropFilters {
		if vCardPropFilterApplies(parsedLines, propFilter) {
			return true
		}
	}
	return false
}

type UnsupportedFilterError struct {
	Name string
}

func (e UnsupportedFilterError) Error() string {
	return fmt.Sprintf("unsupported CardDAV filter name %q", e.Name)
}

var supportedVCardFilterProperties = map[string]struct{}{
	"ADR": {}, "ANNIVERSARY": {}, "BDAY": {}, "CALADRURI": {}, "CALURI": {},
	"CATEGORIES": {}, "CLIENTPIDMAP": {}, "EMAIL": {}, "FBURL": {}, "FN": {},
	"GENDER": {}, "GEO": {}, "IMPP": {}, "KEY": {}, "KIND": {}, "LANG": {},
	"LOGO": {}, "MEMBER": {}, "N": {}, "NICKNAME": {}, "NOTE": {}, "ORG": {},
	"PHOTO": {}, "PRODID": {}, "RELATED": {}, "REV": {}, "ROLE": {}, "SOUND": {},
	"SOURCE": {}, "TEL": {}, "TITLE": {}, "TZ": {}, "UID": {}, "URL": {},
	"VERSION": {}, "XML": {},
}

var supportedVCardFilterParameters = map[string]struct{}{
	"ALTID": {}, "CALSCALE": {}, "GEO": {}, "LABEL": {}, "LANGUAGE": {},
	"MEDIATYPE": {}, "PID": {}, "PREF": {}, "SORT-AS": {}, "TYPE": {},
	"TZ": {}, "VALUE": {},
}

func validateAddressBookQueryFilterSupport(filter AddressBookQueryFilter) error {
	for _, propFilter := range filter.PropFilters {
		if _, ok := supportedVCardFilterProperties[propFilter.Name]; !ok {
			return UnsupportedFilterError{Name: propFilter.Name}
		}
		for _, paramFilter := range propFilter.ParamFilters {
			if _, ok := supportedVCardFilterParameters[paramFilter.Name]; !ok {
				return UnsupportedFilterError{Name: paramFilter.Name}
			}
		}
	}
	return nil
}

func vCardPropFilterApplies(lines []vCardContentLine, filter CardDAVPropFilter) bool {
	propertyExists := false
	for _, line := range lines {
		if line.Name != filter.Name {
			continue
		}
		propertyExists = true
		if vCardPropertyMatchesConditions(line, filter) {
			return true
		}
	}
	if filter.IsNotDefined {
		return !propertyExists
	}
	return propertyExists && len(filter.TextMatches) == 0 && len(filter.ParamFilters) == 0
}

func vCardPropertyMatchesConditions(line vCardContentLine, filter CardDAVPropFilter) bool {
	conditionCount := len(filter.TextMatches) + len(filter.ParamFilters)
	if conditionCount == 0 {
		return true
	}
	if filter.Test == FilterTestAllOf {
		for _, match := range filter.TextMatches {
			if !textMatchApplies(line.Value, match) {
				return false
			}
		}
		for _, paramFilter := range filter.ParamFilters {
			if !vCardParamFilterApplies(line.Params, paramFilter) {
				return false
			}
		}
		return true
	}
	for _, match := range filter.TextMatches {
		if textMatchApplies(line.Value, match) {
			return true
		}
	}
	for _, paramFilter := range filter.ParamFilters {
		if vCardParamFilterApplies(line.Params, paramFilter) {
			return true
		}
	}
	return false
}

func vCardParamFilterApplies(params map[string][]string, filter CardDAVParamFilter) bool {
	values, exists := params[strings.ToUpper(strings.TrimSpace(filter.Name))]
	if filter.IsNotDefined {
		return !exists
	}
	if !exists {
		return false
	}
	if !filter.HasTextMatch {
		return true
	}
	for _, value := range values {
		if textMatchApplies(value, filter.TextMatch) {
			return true
		}
	}
	return false
}

func textMatchApplies(value string, match CardDAVTextMatch) bool {
	needle := normalizeTextMatchValue(match.Text, match.Collation)
	haystack := normalizeTextMatchValue(value, match.Collation)
	var matched bool
	switch match.MatchType {
	case TextMatchEquals:
		matched = haystack == needle
	case TextMatchStartsWith:
		matched = strings.HasPrefix(haystack, needle)
	case TextMatchEndsWith:
		matched = strings.HasSuffix(haystack, needle)
	default:
		matched = strings.Contains(haystack, needle)
	}
	if match.Negate {
		return !matched
	}
	return matched
}

func normalizeTextMatchValue(value string, collation string) string {
	if collation == TextMatchASCIICasemap {
		return strings.Map(func(r rune) rune {
			if r >= 'A' && r <= 'Z' {
				return r + ('a' - 'A')
			}
			return r
		}, value)
	}
	return strings.ToLower(value)
}

func (h *Handler) syncCollectionReport(ctx context.Context, userID string, resource ResourcePath, report ReportRequest, currentUserPrivileges []XMLName) ([]MultiStatusResponse, string, error) {
	if resource.Kind != ResourceAddressBookCollection {
		return nil, "", fmt.Errorf("sync-collection requires an address-book collection resource")
	}
	book, err := h.Store.LookupAddressBook(ctx, userID, resource.AddressBookID)
	if err != nil {
		if report.SyncToken == "" {
			return nil, "", err
		}
		responses, syncToken, changeErr := h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
		if changeErr != nil {
			return nil, "", changeErr
		}
		return responses, syncToken, nil
	}
	if report.SyncToken != "" {
		if report.SyncToken != book.SyncToken {
			responses, syncToken, err := h.syncChangeResponses(ctx, userID, resource, report, currentUserPrivileges)
			if err != nil {
				return nil, "", err
			}
			return responses, syncToken, nil
		}
		return nil, book.SyncToken, nil
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	objects, err := h.listAddressBookObjectsBounded(ctx, userID, resource.AddressBookID, limit+1)
	if err != nil {
		return nil, "", err
	}
	if len(objects) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, "", err
			}
			props = append(props, dataProp)
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, "", err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, book.SyncToken, nil
}

func (h *Handler) listAddressBookObjectsBounded(ctx context.Context, userID string, addressBookID string, limit int) ([]ContactObject, error) {
	if limiter, ok := h.Store.(AddressBookObjectLimiter); ok {
		return limiter.ListAddressBookObjectsLimit(ctx, userID, addressBookID, limit)
	}
	return h.Store.ListAddressBookObjects(ctx, userID, addressBookID)
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
	if changeWithObjectStore, ok := store.(AddressBookChangeWithObjectStore); ok {
		changesWithObject, err := changeWithObjectStore.ListAddressBookChangesWithObjectsSince(ctx, ListAddressBookChangesSinceRequest{
			UserID:        userID,
			AddressBookID: resource.AddressBookID,
			SyncToken:     report.SyncToken,
			Limit:         fetchLimit,
		})
		if err != nil {
			return nil, "", err
		}
		if len(changesWithObject) > limit {
			return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
		}
		syncToken := report.SyncToken
		propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
		responses := make([]MultiStatusResponse, 0, len(changesWithObject))
		for _, item := range changesWithObject {
			change := item.Change
			if strings.TrimSpace(change.SyncToken) != "" {
				syncToken = strings.TrimSpace(change.SyncToken)
			}
			if change.Action == "addressbook-created" || change.Action == "addressbook-updated" || change.Action == "addressbook-deleted" || change.ObjectName == "" {
				continue
			}
			href, err := ContactObjectPath(userID, change.AddressBookID, change.ObjectName)
			if err != nil {
				return nil, "", err
			}
			if change.Action == "contact-deleted" || !item.HasObject {
				responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
				continue
			}
			props, err := ContactObjectProperties(userID, item.Object)
			if err != nil {
				return nil, "", err
			}
			props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
			if containsXMLName(report.Properties, PropAddressData) {
				dataProp, err := ContactObjectDataPropertyWithProperties(item.Object.VCard, report.AddressDataProperties)
				if err != nil {
					return nil, "", err
				}
				props = append(props, dataProp)
			}
			responses = append(responses, responseForProperties(href, propfind, props))
		}
		return responses, syncToken, nil
	}
	changes, err := store.ListAddressBookChangesSince(ctx, ListAddressBookChangesSinceRequest{
		UserID:        userID,
		AddressBookID: resource.AddressBookID,
		SyncToken:     report.SyncToken,
		Limit:         fetchLimit,
	})
	if err != nil {
		return nil, "", err
	}
	if len(changes) > limit {
		return nil, "", TruncatedResultsError{Operation: "sync-collection limit"}
	}
	syncToken := report.SyncToken
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(changes))
	for _, change := range changes {
		if strings.TrimSpace(change.SyncToken) != "" {
			syncToken = strings.TrimSpace(change.SyncToken)
		}
		if change.Action == "addressbook-created" || change.Action == "addressbook-updated" || change.Action == "addressbook-deleted" || change.ObjectName == "" {
			continue
		}
		href, err := ContactObjectPath(userID, change.AddressBookID, change.ObjectName)
		if err != nil {
			return nil, "", err
		}
		if change.Action == "contact-deleted" {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		object, err := h.Store.LookupContactObject(ctx, userID, change.AddressBookID, change.ObjectName)
		if err != nil {
			responses = append(responses, MultiStatusResponse{Href: href, Status: http.StatusNotFound})
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, "", err
		}
		props = withCurrentUserPrivileges(props, ResourceContactObject, currentUserPrivileges)
		if containsXMLName(report.Properties, PropAddressData) {
			dataProp, err := ContactObjectDataPropertyWithProperties(object.VCard, report.AddressDataProperties)
			if err != nil {
				return nil, "", err
			}
			props = append(props, dataProp)
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

func writeCardDAVPreconditionError(w http.ResponseWriter, status int, precondition string, message string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, _ = fmt.Fprintf(w, `<?xml version="1.0" encoding="UTF-8"?>`+
		`<D:error xmlns:D="%s" xmlns:C="%s"><C:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
		DAVNamespace,
		CardDAVNamespace,
		precondition,
		xmlEscapeText(message),
	)
}

func xmlEscapeText(value string) string {
	replacer := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&#34;",
		"'", "&#39;",
	)
	return replacer.Replace(value)
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
	value := strings.TrimSpace(strings.Join(values, " "))
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("If header must not contain line breaks")
	}
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

func cardDAVDiscoveryAllowHeader() string {
	return strings.Join(ImplementedMethods(), ", ")
}
