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

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListAddressBookCollections(ctx context.Context, userID string) ([]AddressBook, error)
	LookupAddressBook(ctx context.Context, userID string, addressBookID string) (AddressBook, error)
	ListAddressBookObjects(ctx context.Context, userID string, addressBookID string) ([]ContactObject, error)
	LookupContactObject(ctx context.Context, userID string, addressBookID string, objectName string) (ContactObject, error)
}

type UserResolver func(*http.Request) (string, error)

type Handler struct {
	Store       DiscoveryStore
	ResolveUser UserResolver
	IncludeSync bool
}

type SyncChangeStore interface {
	ListAddressBookChangesSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error)
}

type ObjectStore interface {
	UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error)
	DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error)
}

type InvalidSyncTokenError struct {
	Token string
}

func (e InvalidSyncTokenError) Error() string {
	return "CardDAV sync-token is no longer valid"
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
	case MethodReport:
		h.serveReport(w, r)
	case MethodGet, MethodHead:
		h.serveGetObject(w, r)
	case MethodPut:
		h.servePutObject(w, r)
	case MethodDelete:
		h.serveDeleteObject(w, r)
	default:
		w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
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

func (h *Handler) serveOptions(w http.ResponseWriter) {
	w.Header().Set("Allow", cardDAVDiscoveryAllowHeader())
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.IncludeSync), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) serveGetObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	object, err := h.Store.LookupContactObject(r.Context(), userID, resource.AddressBookID, resource.ObjectName)
	if err != nil {
		http.Error(w, "carddav contact object not found", http.StatusNotFound)
		return
	}
	if ifMatch := r.Header.Get("If-Match"); ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
		http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
		return
	}
	if objectModifiedSince(r.Header.Get("If-Unmodified-Since"), object.UpdatedAt) {
		http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	if ifNoneMatchMatches(r.Header.Get("If-None-Match"), object.ETag) {
		writeContactObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if objectNotModifiedSince(r.Header.Get("If-Modified-Since"), object.UpdatedAt) {
		writeContactObjectNotModifiedHeaders(w, object)
		w.WriteHeader(http.StatusNotModified)
		return
	}
	writeContactObjectHeaders(w, object)
	w.WriteHeader(http.StatusOK)
	if r.Method != MethodHead {
		_, _ = w.Write(object.VCard)
	}
}

func (h *Handler) servePutObject(w http.ResponseWriter, r *http.Request) {
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "carddav object store is not configured", http.StatusNotImplemented)
		return
	}
	if err := validateVCardPutContentType(r.Header.Get("Content-Type")); err != nil {
		http.Error(w, err.Error(), http.StatusUnsupportedMediaType)
		return
	}
	ifNoneMatch := strings.TrimSpace(r.Header.Get("If-None-Match"))
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
	observedETag := strings.TrimSpace(r.Header.Get("If-Match"))
	if observedETag == "*" {
		if !existed {
			http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
			return
		}
		observedETag = ""
	} else if observedETag != "" && !existed {
		http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
		return
	} else if observedETag != "" && !ifMatchMatches(observedETag, existing.ETag) {
		http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
		return
	}
	if existed && objectModifiedSince(r.Header.Get("If-Unmodified-Since"), existing.UpdatedAt) {
		http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
		return
	}
	body, err := readBoundedContactObjectBody(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusRequestEntityTooLarge)
		return
	}
	object, err := store.UpsertContactObject(r.Context(), UpsertContactObjectRequest{
		UserID:        userID,
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
	userID, resource, ok := h.resolveObjectRequest(w, r)
	if !ok {
		return
	}
	store, ok := h.Store.(ObjectStore)
	if !ok {
		http.Error(w, "carddav object store is not configured", http.StatusNotImplemented)
		return
	}
	ifMatch := strings.TrimSpace(r.Header.Get("If-Match"))
	ifUnmodifiedSince := strings.TrimSpace(r.Header.Get("If-Unmodified-Since"))
	if ifMatch != "" || ifUnmodifiedSince != "" {
		object, err := h.Store.LookupContactObject(r.Context(), userID, resource.AddressBookID, resource.ObjectName)
		if err != nil {
			http.Error(w, "carddav contact object not found", http.StatusPreconditionFailed)
			return
		}
		if ifMatch != "" && !ifMatchMatches(ifMatch, object.ETag) {
			http.Error(w, "carddav contact object etag mismatch", http.StatusPreconditionFailed)
			return
		}
		if objectModifiedSince(ifUnmodifiedSince, object.UpdatedAt) {
			http.Error(w, "carddav contact object modified since precondition", http.StatusPreconditionFailed)
			return
		}
	}
	if _, err := store.DeleteContactObject(r.Context(), DeleteContactObjectRequest{UserID: userID, AddressBookID: resource.AddressBookID, ObjectName: resource.ObjectName}); err != nil {
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
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "carddav resource is not accessible", http.StatusForbidden)
		return
	}
	depth, err := ParseDepth(r.Header.Get("Depth"), DepthZero)
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
		http.Error(w, err.Error(), http.StatusBadRequest)
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
		book, err := h.Store.LookupAddressBook(r.Context(), userID, resource.AddressBookID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		body, err = BuildSyncCollectionXML(responses, book.SyncToken)
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
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "carddav resource is not accessible", http.StatusForbidden)
		return
	}
	depth, err := ParseDepth(r.Header.Get("Depth"), DepthInfinity)
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

func (h *Handler) propfindResponses(ctx context.Context, userID string, resource ResourcePath, depth Depth, propfind PropfindRequest) ([]MultiStatusResponse, error) {
	switch resource.Kind {
	case ResourceRoot:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(RootPath+"/", propfind, PrincipalProperties(principal))}, nil
	case ResourcePrincipal:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(principal.PrincipalPath, propfind, PrincipalProperties(principal))}, nil
	case ResourceAddressBookHome:
		home, err := AddressBookHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := AddressBookHomeProperties(userID)
		if err != nil {
			return nil, err
		}
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
				props, err := AddressBookCollectionProperties(userID, book)
				if err != nil {
					return nil, err
				}
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
		props, err := AddressBookCollectionProperties(userID, book)
		if err != nil {
			return nil, err
		}
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.Store.ListAddressBookObjects(ctx, userID, book.ID)
			if err != nil {
				return nil, err
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
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CardDAV resource")
	}
}

func (h *Handler) resolveObjectRequest(w http.ResponseWriter, r *http.Request) (string, ResourcePath, bool) {
	userID, resource, ok := h.resolveResourceRequest(w, r)
	if !ok {
		return "", ResourcePath{}, false
	}
	if resource.Kind != ResourceContactObject {
		http.Error(w, "carddav contact object path is required", http.StatusNotFound)
		return "", ResourcePath{}, false
	}
	return userID, resource, true
}

func (h *Handler) resolveResourceRequest(w http.ResponseWriter, r *http.Request) (string, ResourcePath, bool) {
	if h.Store == nil {
		http.Error(w, "carddav store is not configured", http.StatusInternalServerError)
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
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return "", ResourcePath{}, false
	}
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "carddav resource is not accessible", http.StatusForbidden)
		return "", ResourcePath{}, false
	}
	return userID, resource, true
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
}

func (h *Handler) reportResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	switch report.Kind {
	case ReportAddressBookMulti:
		if resource.Kind != ResourceAddressBookCollection && resource.Kind != ResourceAddressBookHome {
			return nil, fmt.Errorf("addressbook-multiget requires an address-book collection or home resource")
		}
		return h.addressBookMultigetResponses(ctx, userID, resource, report)
	case ReportAddressBookQuery:
		if resource.Kind != ResourceAddressBookCollection {
			return nil, fmt.Errorf("addressbook-query requires an address-book collection resource")
		}
		return h.addressBookQueryResponses(ctx, userID, resource, report)
	case ReportSyncCollection:
		if resource.Kind != ResourceAddressBookCollection {
			return nil, fmt.Errorf("sync-collection requires an address-book collection resource")
		}
		return h.syncCollectionResponses(ctx, userID, resource, report)
	default:
		return nil, fmt.Errorf("REPORT %s is not implemented", report.Kind)
	}
}

func (h *Handler) addressBookMultigetResponses(ctx context.Context, userID string, requestResource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	for _, href := range report.Hrefs {
		resource, err := ParseResourceHref(href)
		if err != nil || resource.Kind != ResourceContactObject || resource.UserID != userID || !multigetHrefInScope(requestResource, resource) {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		object, err := h.Store.LookupContactObject(ctx, userID, resource.AddressBookID, resource.ObjectName)
		if err != nil {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		if containsXMLName(report.Properties, PropAddressData) {
			props = append(props, ContactObjectDataProperty(object.VCard))
		}
		objectHref, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(objectHref, propfind, props))
	}
	return responses, nil
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

func (h *Handler) addressBookQueryResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	objects, err := h.Store.ListAddressBookObjects(ctx, userID, resource.AddressBookID)
	if err != nil {
		return nil, err
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		if !contactObjectMatchesText(object, report.TextMatch) {
			continue
		}
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		if containsXMLName(report.Properties, PropAddressData) {
			props = append(props, ContactObjectDataProperty(object.VCard))
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
}

func contactObjectMatchesText(object ContactObject, text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return true
	}
	return strings.Contains(strings.ToLower(string(object.VCard)), strings.ToLower(text))
}

func (h *Handler) syncCollectionResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	book, err := h.Store.LookupAddressBook(ctx, userID, resource.AddressBookID)
	if err != nil {
		return nil, err
	}
	if report.SyncToken != "" {
		if report.SyncToken != book.SyncToken {
			return h.syncChangeResponses(ctx, userID, resource, report)
		}
		return nil, nil
	}
	objects, err := h.Store.ListAddressBookObjects(ctx, userID, resource.AddressBookID)
	if err != nil {
		return nil, err
	}
	if report.Limit > 0 && report.Limit < len(objects) {
		return nil, fmt.Errorf("sync-collection limit would truncate results")
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(objects))
	for _, object := range objects {
		props, err := ContactObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		if containsXMLName(report.Properties, PropAddressData) {
			props = append(props, ContactObjectDataProperty(object.VCard))
		}
		href, err := ContactObjectPath(userID, object.AddressBookID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(href, propfind, props))
	}
	return responses, nil
}

func (h *Handler) syncChangeResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	store, ok := h.Store.(SyncChangeStore)
	if !ok {
		return nil, InvalidSyncTokenError{Token: report.SyncToken}
	}
	limit := report.Limit
	if limit <= 0 {
		limit = MaxWebDAVReportLimit
	}
	changes, err := store.ListAddressBookChangesSince(ctx, ListAddressBookChangesSinceRequest{
		UserID:        userID,
		AddressBookID: resource.AddressBookID,
		SyncToken:     report.SyncToken,
		Limit:         limit,
	})
	if err != nil {
		return nil, err
	}
	if report.Limit > 0 && len(changes) == report.Limit {
		return nil, fmt.Errorf("sync-collection limit may truncate change results")
	}
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(changes))
	for _, change := range changes {
		if change.Action == "addressbook-created" || change.Action == "addressbook-updated" || change.Action == "addressbook-deleted" || change.ObjectName == "" {
			continue
		}
		href, err := ContactObjectPath(userID, change.AddressBookID, change.ObjectName)
		if err != nil {
			return nil, err
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
			return nil, err
		}
		if containsXMLName(report.Properties, PropAddressData) {
			props = append(props, ContactObjectDataProperty(object.VCard))
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

func validateVCardPutContentType(value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	if strings.ContainsAny(value, "\r\n") {
		return fmt.Errorf("contact object content type must not contain line breaks")
	}
	mediaType, _, err := mime.ParseMediaType(value)
	if err != nil {
		return fmt.Errorf("contact object content type is invalid")
	}
	if !strings.EqualFold(mediaType, "text/vcard") {
		return fmt.Errorf("contact object content type must be text/vcard")
	}
	return nil
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
	return strings.Join([]string{MethodOptions, MethodPropfind, MethodReport, MethodGet, MethodHead, MethodPut, MethodDelete}, ", ")
}
