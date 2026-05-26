package ldapgw

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

func (s *LDAPServer) handleSearchRequest(ctx context.Context, msgID int, opData []byte, controls []control) ([]byte, int, int) {
	select {
	case <-ctx.Done():
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "operation timed out"), result, 0
	default:
	}

	baseObject, scope, filter, attrs, sizeLimit, timeLimit, typesOnly, err := decodeSearchRequest(opData)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	if timeLimit > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeLimit)*time.Second)
		defer cancel()
	}
	if baseObject == "" && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, "", rootDSEAttributes(s.namingContexts, s.tlsConfig != nil), filter, attrs, typesOnly)
	}
	if normalizeDNForCompare(baseObject) == "cn=subschema" && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, "cn=Subschema", subschemaAttributes(), filter, attrs, typesOnly)
	}
	if containerAttrs, ok := ldapContainerAttributes(baseObject); ok && scope == scopeBaseObject {
		return encodeSyntheticSearchResult(msgID, baseObject, containerAttrs, filter, attrs, typesOnly)
	}
	if !s.baseDNWithinNamingContext(baseObject) && len(s.referralURLs) > 0 {
		resp := encodeSearchResultReference(msgID, s.referralURLs)
		resp = append(resp, encodeSearchResultDone(msgID, resultSuccess, "", "")...)
		return resp, resultSuccess, 0
	}
	pageSize, pageOffset, paged, err := parsePagedResultsControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	sortKeys, sorted, err := parseServerSideSortControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	vlv, hasVLV, err := parseVirtualListViewControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	matchedValuesFilters, hasMatchedValues, err := parseMatchedValuesControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	assertionFilter, hasAssertion, err := parseAssertionControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	syncReq, hasSync, err := parseSyncRequestControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	_, err = parseDereferenceControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	// Gogomail has no DN-valued relationship expansion repository yet; accept
	// well-formed dereference requests as a read-only no-op for client parity.
	subentriesOnly, err := parseSubentriesControl(controls)
	if err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}

	if err := validateFilter(filter); err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", fmt.Sprintf("invalid filter: %v", err)), result, 0
	}
	if hasAssertion {
		ok, err := evaluateSearchAssertion(assertionFilter, baseObject)
		if err != nil {
			result := resultProtocolError
			return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
		}
		if !ok {
			result := resultAssertionFailed
			return encodeSearchResultDone(msgID, result, "", "assertion failed"), result, 0
		}
	}

	ldapFilter, err := parseLDAPFilter(filter)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "malformed filter"), result, 0
	}
	kinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", "malformed objectClass filter"), result, 0
	}
	kinds, noKindMatch := intersectPrincipalKinds(kinds, principalKindsForBaseDN(baseObject))
	if noKindMatch {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}
	if subentriesOnly {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}

	targetEntries := ldapSearchCandidateBatchSize
	scanAllCandidates := sorted || hasVLV
	if paged && pageSize > 0 {
		targetEntries = max(targetEntries, pageOffset+pageSize+1)
	}
	if sizeLimit > 0 {
		targetEntries = max(targetEntries, sizeLimit+1)
	}

	entries, err := s.searchLDAPEntries(ctx, DirectorySearchRequest{
		BaseDN: baseObject,
		Scope:  scope,
		Filter: ldapFilter,
		Attrs:  attrs,
		Kinds:  kinds,
	}, filter, targetEntries, scanAllCandidates)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			result := resultTimeLimitExceeded
			return encodeSearchResultDone(msgID, result, "", "time limit exceeded"), result, 0
		}
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	if sorted {
		sortLDAPSearchEntries(entries, sortKeys)
	}
	sizeLimitExceeded := sizeLimit > 0 && len(entries) > sizeLimit
	if sizeLimit > 0 && len(entries) > sizeLimit {
		entries = entries[:sizeLimit]
	}
	vlvTargetPosition := 0
	vlvContentCount := len(entries)
	if hasVLV {
		entries, vlvTargetPosition = applyVirtualListView(entries, vlv)
	}
	nextCookie := ""
	if paged && pageSize > 0 {
		if pageOffset < len(entries) {
			entries = entries[pageOffset:]
		} else {
			entries = nil
		}
		if len(entries) > pageSize {
			entries = entries[:pageSize]
			nextCookie = fmt.Sprintf("%d", pageOffset+pageSize)
		}
	}

	resp := make([]byte, 0, 4096)
	for _, entry := range entries {
		attrMap := entry.attrs
		if hasMatchedValues {
			attrMap = applyMatchedValuesFilter(attrMap, matchedValuesFilters)
		}
		var entryControls []control
		if hasSync {
			entryControls = append(entryControls, syncStateControl(entry.principal.DN, syncReq.Cookie))
		}
		responseEntry, err := encodeSearchResultEntryWithControls(msgID, entry.principal.DN, selectLDAPAttributes(attrMap, attrs, typesOnly), entryControls)
		if err != nil {
			continue
		}
		resp = append(resp, responseEntry...)
	}
	result := resultSuccess
	if sizeLimitExceeded {
		result = resultSizeLimitExceeded
	}
	responseControls := make([]control, 0, 2)
	if paged {
		responseControls = append(responseControls, pagedResultsResponseControl(nextCookie))
	}
	if sorted {
		responseControls = append(responseControls, serverSideSortResponseControl(resultSuccess, ""))
	}
	if hasVLV {
		responseControls = append(responseControls, virtualListViewResponseControl(vlvTargetPosition, vlvContentCount, resultSuccess, ""))
	}
	if hasSync {
		responseControls = append(responseControls, syncDoneControl(syncReq.Cookie))
	}
	if len(responseControls) > 0 {
		resp = append(resp, encodeSearchResultDoneWithControls(msgID, result, "", "", responseControls)...)
	} else {
		resp = append(resp, encodeSearchResultDone(msgID, result, "", "")...)
	}
	return resp, result, len(entries)
}

func encodeSyntheticSearchResult(msgID int, dn string, entryAttrs map[string][]string, filter []byte, requested []string, typesOnly bool) ([]byte, int, int) {
	if err := validateFilter(filter); err != nil {
		result := resultProtocolError
		return encodeSearchResultDone(msgID, result, "", "malformed filter"), result, 0
	}
	if !ldapEntryAttributesMatchFilter(entryAttrs, filter) {
		return encodeSearchResultDone(msgID, resultSuccess, "", ""), resultSuccess, 0
	}
	entry, err := encodeSearchResultEntry(msgID, dn, selectLDAPAttributes(entryAttrs, requested, typesOnly))
	if err != nil {
		result := resultUnwillingToPerform
		return encodeSearchResultDone(msgID, result, "", err.Error()), result, 0
	}
	return append(entry, encodeSearchResultDone(msgID, resultSuccess, "", "")...), resultSuccess, 1
}

func filterPrincipalEntriesByScope(principals []PrincipalEntry, baseObject string, scope int) []PrincipalEntry {
	base := normalizeDNForCompare(baseObject)
	if base == "" {
		if scope == scopeBaseObject {
			return nil
		}
		return principals
	}
	filtered := make([]PrincipalEntry, 0, len(principals))
	for _, p := range principals {
		entryDN := normalizeDNForCompare(p.DN)
		switch scope {
		case scopeBaseObject:
			if entryDN == base {
				filtered = append(filtered, p)
			}
		case scopeSingleLevel:
			if parentDN(entryDN) == base {
				filtered = append(filtered, p)
			}
		case scopeWholeSubtree:
			if entryDN == base || strings.HasSuffix(entryDN, ","+base) {
				filtered = append(filtered, p)
			}
		default:
			filtered = append(filtered, p)
		}
	}
	return filtered
}

func (s *LDAPServer) searchLDAPEntries(ctx context.Context, req DirectorySearchRequest, filter []byte, targetEntries int, scanAll bool) ([]ldapSearchEntry, error) {
	if targetEntries <= 0 {
		targetEntries = ldapSearchCandidateBatchSize
	}
	var entries []ldapSearchEntry
	offset := 0
	for offset < ldapMaxCandidateScan {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		batchReq := req
		batchReq.Limit = ldapSearchCandidateBatchSize
		batchReq.Offset = offset
		principals, err := s.quer.SearchPrincipals(ctx, batchReq)
		if err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if len(principals) == 0 {
			break
		}
		scoped := filterPrincipalEntriesByScope(principals, req.BaseDN, req.Scope)
		batchEntries := ldapSearchEntries(scoped)
		entries = append(entries, filterLDAPSearchEntriesByFilter(batchEntries, filter)...)
		if !scanAll && len(entries) >= targetEntries {
			break
		}
		if len(principals) < ldapSearchCandidateBatchSize {
			break
		}
		offset += len(principals)
	}
	return entries, nil
}

func ldapSearchEntries(principals []PrincipalEntry) []ldapSearchEntry {
	entries := make([]ldapSearchEntry, 0, len(principals))
	for _, p := range principals {
		entries = append(entries, ldapSearchEntry{principal: p, attrs: principalLDAPAttributes(p)})
	}
	return entries
}

func filterLDAPSearchEntriesByFilter(entries []ldapSearchEntry, filter []byte) []ldapSearchEntry {
	if len(filter) == 0 {
		return entries
	}
	filtered := make([]ldapSearchEntry, 0, len(entries))
	for _, entry := range entries {
		if ldapEntryAttributesMatchFilter(entry.attrs, filter) {
			filtered = append(filtered, entry)
		}
	}
	return filtered
}

func parentDN(dn string) string {
	parts := splitDNComponents(dn)
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[1:], ",")
}

func ldapContainerAttributes(dn string) (map[string][]string, bool) {
	operational := ldapOperationalAttributes(dn)
	withOperational := func(attrs map[string][]string) map[string][]string {
		for k, values := range operational {
			attrs[k] = values
		}
		attrs["hasSubordinates"] = []string{"TRUE"}
		attrs["numSubordinates"] = []string{"0"}
		return attrs
	}
	switch firstRDNValue(normalizeDNForCompare(dn), "ou") {
	case "users":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"users"}, "cn": {"users"}, "displayName": {"Users"}}), true
	case "organizations":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"organizations"}, "cn": {"organizations"}, "displayName": {"Organizations"}}), true
	case "groups":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"groups"}, "cn": {"groups"}, "displayName": {"Groups"}}), true
	case "resources":
		return withOperational(map[string][]string{"objectClass": {"top", "organizationalUnit"}, "ou": {"resources"}, "cn": {"resources"}, "displayName": {"Resources"}}), true
	default:
		return nil, false
	}
}

func principalKindsForBaseDN(dn string) []string {
	switch firstRDNValue(normalizeDNForCompare(dn), "ou") {
	case "users":
		return []string{"user"}
	case "organizations":
		return []string{"organization"}
	case "groups":
		return []string{"group"}
	case "resources":
		return []string{"resource"}
	default:
		return nil
	}
}

func firstRDNValue(dn, attr string) string {
	if dn == "" {
		return ""
	}
	components := splitDNComponents(dn)
	if len(components) == 0 {
		return ""
	}
	first := strings.TrimSpace(components[0])
	parts := strings.SplitN(first, "=", 2)
	if len(parts) != 2 || !strings.EqualFold(strings.TrimSpace(parts[0]), attr) {
		return ""
	}
	value, ok := unescapeDNValue(strings.TrimSpace(parts[1]))
	if !ok {
		return ""
	}
	return strings.ToLower(value)
}

func intersectPrincipalKinds(filterKinds, baseKinds []string) ([]string, bool) {
	if len(baseKinds) == 0 {
		return filterKinds, false
	}
	if len(filterKinds) == 0 {
		return baseKinds, false
	}
	base := make(map[string]struct{}, len(baseKinds))
	for _, kind := range baseKinds {
		base[kind] = struct{}{}
	}
	out := make([]string, 0, len(filterKinds))
	for _, kind := range filterKinds {
		if _, ok := base[kind]; ok {
			out = append(out, kind)
		}
	}
	if len(out) == 0 {
		return nil, true
	}
	return out, false
}
