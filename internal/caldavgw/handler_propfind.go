package caldavgw

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

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
	depth, err := parseDepthHeader(r.Header, DepthInfinity)
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
	objectPrincipalPath, err := PrincipalPath(userID)
	if err != nil {
		return nil, err
	}
	switch resource.Kind {
	case ResourceRoot:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		props, err := withCurrentUserPrincipal(ServiceRootProperties(principal), actorUserID)
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
	case ResourceCalendarHome:
		home, err := CalendarHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarHomeProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarHome, currentUserPrivileges)
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
				props, err := CalendarCollectionProperties(userID, calendar, h.includeSyncCollection())
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceCalendarCollection, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarCollection:
		calendar, err := h.lookupCalendar(ctx, userID, resource.CalendarID)
		if err != nil {
			return nil, err
		}
		href, err := CalendarCollectionPath(userID, calendar.ID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarCollectionProperties(userID, calendar, h.includeSyncCollection())
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarCollection, currentUserPrivileges)
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.listCalendarObjectsBounded(ctx, userID, calendar.ID, MaxWebDAVReportLimit+1, false)
			if err != nil {
				return nil, err
			}
			if len(objects) > MaxWebDAVReportLimit {
				return nil, TruncatedResultsError{Operation: "calendar collection PROPFIND"}
			}
			for _, object := range objects {
				href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
				if err != nil {
					return nil, err
				}
				props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
				if err != nil {
					return nil, err
				}
				props, err = withCurrentUserPrincipal(props, actorUserID)
				if err != nil {
					return nil, err
				}
				props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarObject:
		if depth != DepthZero {
			return nil, fmt.Errorf("calendar object PROPFIND requires Depth: 0")
		}
		includeCalendarData := containsXMLName(propfind.Properties, PropCalendarData) || containsXMLName(propfind.Include, PropCalendarData)
		object, err := h.lookupCalendarObjectForReport(ctx, userID, resource.CalendarID, resource.ObjectName, includeCalendarData)
		if err != nil {
			return nil, err
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		props, err := CalendarObjectPropertiesWithPrincipalPath(userID, object, objectPrincipalPath)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceCalendarObject, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	case ResourceInbox:
		href, err := ScheduleInboxPath(userID)
		if err != nil {
			return nil, err
		}
		props, err := SchedulingInboxProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceInbox, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	case ResourceOutbox:
		href, err := ScheduleOutboxPath(userID)
		if err != nil {
			return nil, err
		}
		props, err := SchedulingOutboxProperties(userID)
		if err != nil {
			return nil, err
		}
		props, err = withCurrentUserPrincipal(props, actorUserID)
		if err != nil {
			return nil, err
		}
		props = withCurrentUserPrivileges(props, ResourceOutbox, currentUserPrivileges)
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CalDAV resource")
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
	role := CalendarAccessRoleRead
	for _, privilege := range privileges {
		if privilege == PrivilegeBind || privilege == PrivilegeUnbind {
			role = CalendarAccessRoleManage
			break
		}
		if privilege == PrivilegeWriteContent || privilege == PrivilegeWriteProps {
			role = CalendarAccessRoleWrite
		}
	}
	switch kind {
	case ResourceCalendarHome:
		if role == CalendarAccessRoleManage {
			return calendarHomePrivileges()
		}
		return readOnlyPrivileges()
	case ResourceCalendarCollection:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return calendarCollectionPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceCalendarObject:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return writableObjectPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceInbox:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return schedulingInboxPrivileges()
		}
		return readOnlyPrivileges()
	case ResourceOutbox:
		if role == CalendarAccessRoleWrite || role == CalendarAccessRoleManage {
			return schedulingOutboxPrivileges()
		}
		return readOnlyPrivileges()
	default:
		return readOnlyPrivileges()
	}
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
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
	writePreconditionError(w, status, DAVNamespace, "D", precondition, message)
}

func writeCalDAVPreconditionError(w http.ResponseWriter, status int, precondition string, message string) {
	writePreconditionError(w, status, CalDAVNamespace, "C", precondition, message)
}

func writePreconditionError(w http.ResponseWriter, status int, namespace string, prefix string, precondition string, message string) {
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if prefix == "D" {
		_, _ = fmt.Fprintf(w,
			`<?xml version="1.0" encoding="UTF-8"?>`+
				`<D:error xmlns:D="%s"><D:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
			namespace,
			precondition,
			xmlEscapeText(message),
		)
		return
	}
	_, _ = fmt.Fprintf(w,
		`<?xml version="1.0" encoding="UTF-8"?>`+
			`<D:error xmlns:D="%s" xmlns:%s="%s"><%s:%s/><D:responsedescription>%s</D:responsedescription></D:error>`,
		DAVNamespace,
		prefix,
		namespace,
		prefix,
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

func parseDepthHeader(header http.Header, fallback Depth) (Depth, error) {
	values := header.Values("Depth")
	if len(values) > 1 {
		return "", fmt.Errorf("Depth header must not be repeated")
	}
	if len(values) == 0 {
		return ParseDepth("", fallback)
	}
	return ParseDepth(values[0], fallback)
}
