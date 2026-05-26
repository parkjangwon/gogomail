package carddavgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
)

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
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(http.StatusMultiStatus)
	_, _ = w.Write(body)
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
		return nil, fmt.Errorf("unsupported REPORT %s", report.Kind)
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
		if r == '%' || r == '_' || r == '\\' {
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
	if filter.TextMatch.Negate {
		for _, value := range values {
			if textMatchMatches(value, filter.TextMatch) {
				return false
			}
		}
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
	matched := textMatchMatches(value, match)
	if match.Negate {
		return !matched
	}
	return matched
}

func textMatchMatches(value string, match CardDAVTextMatch) bool {
	needle := normalizeTextMatchValue(match.Text, match.Collation)
	haystack := normalizeTextMatchValue(value, match.Collation)
	switch match.MatchType {
	case TextMatchEquals:
		return haystack == needle
	case TextMatchStartsWith:
		return strings.HasPrefix(haystack, needle)
	case TextMatchEndsWith:
		return strings.HasSuffix(haystack, needle)
	default:
		return strings.Contains(haystack, needle)
	}
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
