package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

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
		writeCardDAVUnauthorized(w, err)
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
		http.Error(w, "internal server error", http.StatusInternalServerError)
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
