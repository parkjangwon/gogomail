package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
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

func cardDAVDiscoveryAllowHeader() string {
	return strings.Join([]string{MethodOptions, MethodPropfind, MethodReport}, ", ")
}
