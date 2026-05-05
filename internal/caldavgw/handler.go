package caldavgw

import (
	"context"
	"fmt"
	"net/http"
	"strings"
)

type DiscoveryStore interface {
	LookupPrincipal(ctx context.Context, userID string) (Principal, error)
	ListCalendarCollections(ctx context.Context, userID string) ([]Calendar, error)
	LookupCalendar(ctx context.Context, userID string, calendarID string) (Calendar, error)
	ListCalendarObjects(ctx context.Context, userID string, calendarID string) ([]CalendarObject, error)
	LookupCalendarObject(ctx context.Context, userID string, calendarID string, objectName string) (CalendarObject, error)
}

type UserResolver func(*http.Request) (string, error)

type Handler struct {
	Store             DiscoveryStore
	ResolveUser       UserResolver
	IncludeScheduling bool
}

func NewHandler(store DiscoveryStore, resolveUser UserResolver) *Handler {
	return &Handler{Store: store, ResolveUser: resolveUser}
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
		http.Error(w, "caldav handler is not configured", http.StatusInternalServerError)
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
		w.Header().Set("Allow", calDAVAllowHeader())
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return
	}
	report, err := ParseReport(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	responses, err := h.reportResponses(r.Context(), userID, resource, report)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
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

func (h *Handler) serveOptions(w http.ResponseWriter) {
	w.Header().Set("Allow", calDAVAllowHeader())
	w.Header().Set("DAV", strings.Join(AdvertisedDAVTokens(h.IncludeScheduling), ", "))
	w.Header().Set("MS-Author-Via", "DAV")
	w.WriteHeader(http.StatusNoContent)
}

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
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	resource, err := ParseResourcePath(r.URL.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	if resource.UserID != "" && resource.UserID != userID {
		http.Error(w, "caldav resource is not accessible", http.StatusForbidden)
		return
	}
	depth, err := ParseDepth(r.Header.Get("Depth"), DepthInfinity)
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
	case ResourcePrincipal:
		principal, err := h.Store.LookupPrincipal(ctx, userID)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(principal.PrincipalPath, propfind, PrincipalProperties(principal))}, nil
	case ResourceCalendarHome:
		home, err := CalendarHomePath(userID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarHomeProperties(userID)
		if err != nil {
			return nil, err
		}
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
				props, err := CalendarCollectionProperties(userID, calendar)
				if err != nil {
					return nil, err
				}
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarCollection:
		calendar, err := h.Store.LookupCalendar(ctx, userID, resource.CalendarID)
		if err != nil {
			return nil, err
		}
		href, err := CalendarCollectionPath(userID, calendar.ID)
		if err != nil {
			return nil, err
		}
		props, err := CalendarCollectionProperties(userID, calendar)
		if err != nil {
			return nil, err
		}
		responses := []MultiStatusResponse{responseForProperties(href, propfind, props)}
		if depth == DepthOne {
			objects, err := h.Store.ListCalendarObjects(ctx, userID, calendar.ID)
			if err != nil {
				return nil, err
			}
			for _, object := range objects {
				href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
				if err != nil {
					return nil, err
				}
				props, err := CalendarObjectProperties(userID, object)
				if err != nil {
					return nil, err
				}
				responses = append(responses, responseForProperties(href, propfind, props))
			}
		}
		return responses, nil
	case ResourceCalendarObject:
		if depth != DepthZero {
			return nil, fmt.Errorf("calendar object PROPFIND requires Depth: 0")
		}
		object, err := h.Store.LookupCalendarObject(ctx, userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			return nil, err
		}
		href, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		return []MultiStatusResponse{responseForProperties(href, propfind, props)}, nil
	default:
		return nil, fmt.Errorf("unsupported CalDAV resource")
	}
}

func responseForProperties(href string, propfind PropfindRequest, props []PropertyResult) MultiStatusResponse {
	return MultiStatusResponse{Href: href, PropStats: SelectPropfindProperties(propfind, props)}
}

func (h *Handler) reportResponses(ctx context.Context, userID string, resource ResourcePath, report ReportRequest) ([]MultiStatusResponse, error) {
	switch report.Kind {
	case ReportCalendarMulti:
		if resource.Kind != ResourceCalendarCollection && resource.Kind != ResourceCalendarHome {
			return nil, fmt.Errorf("calendar-multiget requires a calendar collection or home resource")
		}
		return h.calendarMultigetResponses(ctx, userID, report)
	default:
		return nil, fmt.Errorf("REPORT %s is not implemented", report.Kind)
	}
}

func (h *Handler) calendarMultigetResponses(ctx context.Context, userID string, report ReportRequest) ([]MultiStatusResponse, error) {
	propfind := PropfindRequest{Kind: PropfindProp, Properties: report.Properties}
	responses := make([]MultiStatusResponse, 0, len(report.Hrefs))
	for _, href := range report.Hrefs {
		resource, err := ParseResourcePath(href)
		if err != nil || resource.Kind != ResourceCalendarObject || resource.UserID != userID {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		object, err := h.Store.LookupCalendarObject(ctx, userID, resource.CalendarID, resource.ObjectName)
		if err != nil {
			responses = append(responses, notFoundResponse(href, report.Properties))
			continue
		}
		props, err := CalendarObjectProperties(userID, object)
		if err != nil {
			return nil, err
		}
		if containsXMLName(report.Properties, PropCalendarData) {
			props = append(props, CalendarObjectDataProperty(object.ICS))
		}
		objectHref, err := CalendarObjectPath(userID, object.CalendarID, object.ObjectName)
		if err != nil {
			return nil, err
		}
		responses = append(responses, responseForProperties(objectHref, propfind, props))
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

func calDAVAllowHeader() string {
	return strings.Join([]string{
		MethodOptions,
		MethodPropfind,
		MethodReport,
		MethodMkcalendar,
		MethodGet,
		MethodHead,
		MethodPut,
		MethodDelete,
	}, ", ")
}
