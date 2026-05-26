package ldapgw

import (
	"fmt"
	"sort"
	"strings"
)

func firstUnsupportedCriticalControl(controls []control) (control, bool) {
	for _, ctrl := range controls {
		if ctrl.Critical && !isSupportedControl(ctrl.Type) {
			return ctrl, true
		}
	}
	return control{}, false
}

func parsePagedResultsControl(controls []control) (pageSize int, offset int, ok bool, err error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlPagedResults {
			continue
		}
		ok = true
		pageSize, offset, err = decodePagedResultsControlValue(ctrl.Value)
		if err != nil {
			return 0, 0, true, err
		}
		return pageSize, offset, true, nil
	}
	return 0, 0, false, nil
}

func decodePagedResultsControlValue(value []byte) (pageSize int, offset int, err error) {
	if len(value) == 0 {
		return 0, 0, fmt.Errorf("paged results control value is required")
	}
	if value[0] != tagSequence {
		return 0, 0, fmt.Errorf("paged results control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return 0, 0, err
	}
	pageSize, rest, err := decodeInt(content)
	if err != nil {
		return 0, 0, fmt.Errorf("paged results size: %w", err)
	}
	if pageSize < 0 {
		return 0, 0, fmt.Errorf("paged results size must not be negative")
	}
	cookie, rest, err := decodeOctetString(rest)
	if err != nil {
		return 0, 0, fmt.Errorf("paged results cookie: %w", err)
	}
	if len(rest) != 0 {
		return 0, 0, fmt.Errorf("paged results control has trailing data")
	}
	if strings.TrimSpace(cookie) == "" {
		return pageSize, 0, nil
	}
	for _, r := range cookie {
		if r < '0' || r > '9' {
			return 0, 0, fmt.Errorf("paged results cookie is invalid")
		}
		offset = offset*10 + int(r-'0')
	}
	return pageSize, offset, nil
}

func pagedResultsResponseControl(cookie string) control {
	var value []byte
	value = append(value, encodeInt(0)...)
	value = append(value, encodeOctetString(cookie)...)
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(value))...)
	seq = append(seq, value...)
	return control{Type: controlPagedResults, Value: seq}
}

type sortKey struct {
	Attribute string
	Reverse   bool
}

func parseServerSideSortControl(controls []control) ([]sortKey, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlServerSideSortRequest {
			continue
		}
		keys, err := decodeServerSideSortControlValue(ctrl.Value)
		if err != nil {
			return nil, true, err
		}
		return keys, true, nil
	}
	return nil, false, nil
}

func decodeServerSideSortControlValue(value []byte) ([]sortKey, error) {
	if len(value) == 0 {
		return nil, fmt.Errorf("server-side sort control value is required")
	}
	if value[0] != tagSequence {
		return nil, fmt.Errorf("server-side sort control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return nil, err
	}
	var keys []sortKey
	for len(content) > 0 {
		item, rest, err := readRawTLV(content)
		if err != nil {
			return nil, err
		}
		key, err := decodeSortKey(item)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
		content = rest
	}
	if len(keys) == 0 {
		return nil, fmt.Errorf("server-side sort control requires at least one key")
	}
	return keys, nil
}

func decodeSortKey(data []byte) (sortKey, error) {
	if len(data) == 0 || data[0] != tagSequence {
		return sortKey{}, fmt.Errorf("server-side sort key must be a sequence")
	}
	content, err := decodeContent(data[1:])
	if err != nil {
		return sortKey{}, err
	}
	attr, rest, err := decodeOctetString(content)
	if err != nil {
		return sortKey{}, fmt.Errorf("server-side sort attribute: %w", err)
	}
	key := sortKey{Attribute: ldapAttributeType(attr)}
	if key.Attribute == "" {
		return sortKey{}, fmt.Errorf("server-side sort attribute is empty")
	}
	for len(rest) > 0 {
		tag := rest[0]
		length, tail, err := decodeLength(rest[1:])
		if err != nil {
			return sortKey{}, err
		}
		if len(tail) < length {
			return sortKey{}, fmt.Errorf("server-side sort key value truncated")
		}
		value := tail[:length]
		switch tag {
		case 0x80:
			// orderingRule is accepted but repository entries use local string ordering.
		case 0x81:
			if len(value) != 1 {
				return sortKey{}, fmt.Errorf("server-side sort reverseOrder malformed")
			}
			key.Reverse = value[0] != 0
		default:
			return sortKey{}, fmt.Errorf("unsupported server-side sort key tag 0x%02x", tag)
		}
		rest = tail[length:]
	}
	return key, nil
}

func sortPrincipalEntries(principals []PrincipalEntry, keys []sortKey) {
	entries := ldapSearchEntries(principals)
	sortLDAPSearchEntries(entries, keys)
	for i, entry := range entries {
		principals[i] = entry.principal
	}
}

func sortLDAPSearchEntries(entries []ldapSearchEntry, keys []sortKey) {
	if len(keys) == 0 {
		return
	}
	sort.SliceStable(entries, func(i, j int) bool {
		leftAttrs := entries[i].attrs
		rightAttrs := entries[j].attrs
		for _, key := range keys {
			left := firstLDAPAttributeValue(leftAttrs, key.Attribute)
			right := firstLDAPAttributeValue(rightAttrs, key.Attribute)
			cmp := strings.Compare(strings.ToLower(left), strings.ToLower(right))
			if cmp == 0 {
				continue
			}
			if key.Reverse {
				return cmp > 0
			}
			return cmp < 0
		}
		return normalizeDNForCompare(entries[i].principal.DN) < normalizeDNForCompare(entries[j].principal.DN)
	})
}

func firstLDAPAttributeValue(attrs map[string][]string, attr string) string {
	for name, values := range attrs {
		if strings.EqualFold(name, attr) && len(values) > 0 {
			return values[0]
		}
	}
	return ""
}

func serverSideSortResponseControl(resultCode int, attributeType string) control {
	var content []byte
	content = append(content, encodeEnumerated(resultCode)...)
	if strings.TrimSpace(attributeType) != "" {
		value := []byte(attributeType)
		content = append(content, 0x80)
		content = append(content, encodeLength(len(value))...)
		content = append(content, value...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlServerSideSortResponse, Value: seq}
}

type virtualListViewRequest struct {
	BeforeCount int
	AfterCount  int
	Offset      int
}

func parseVirtualListViewControl(controls []control) (virtualListViewRequest, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlVirtualListViewRequest {
			continue
		}
		req, err := decodeVirtualListViewControlValue(ctrl.Value)
		if err != nil {
			return virtualListViewRequest{}, true, err
		}
		return req, true, nil
	}
	return virtualListViewRequest{}, false, nil
}

func decodeVirtualListViewControlValue(value []byte) (virtualListViewRequest, error) {
	if len(value) == 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view control value is required")
	}
	if value[0] != tagSequence {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return virtualListViewRequest{}, err
	}
	before, rest, err := decodeInt(content)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view beforeCount: %w", err)
	}
	after, rest, err := decodeInt(rest)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view afterCount: %w", err)
	}
	if before < 0 || after < 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view counts must not be negative")
	}
	if len(rest) == 0 || rest[0] != 0xa0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view byOffset target is required")
	}
	target, rest, err := readRawTLV(rest)
	if err != nil {
		return virtualListViewRequest{}, err
	}
	targetContent, err := decodeContent(target[1:])
	if err != nil {
		return virtualListViewRequest{}, err
	}
	offset, targetRest, err := decodeInt(targetContent)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view offset: %w", err)
	}
	_, targetRest, err = decodeInt(targetRest)
	if err != nil {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view contentCount: %w", err)
	}
	if len(targetRest) != 0 {
		return virtualListViewRequest{}, fmt.Errorf("virtual list view target has trailing data")
	}
	if len(rest) > 0 {
		_, rest, err = decodeOctetString(rest)
		if err != nil {
			return virtualListViewRequest{}, fmt.Errorf("virtual list view contextID: %w", err)
		}
		if len(rest) != 0 {
			return virtualListViewRequest{}, fmt.Errorf("virtual list view control has trailing data")
		}
	}
	if offset < 1 {
		offset = 1
	}
	return virtualListViewRequest{BeforeCount: before, AfterCount: after, Offset: offset}, nil
}

func applyVirtualListView[T any](entries []T, req virtualListViewRequest) ([]T, int) {
	if len(entries) == 0 {
		return nil, 0
	}
	target := req.Offset
	if target > len(entries) {
		target = len(entries)
	}
	start := target - 1 - req.BeforeCount
	if start < 0 {
		start = 0
	}
	end := target + req.AfterCount
	if end > len(entries) {
		end = len(entries)
	}
	return entries[start:end], target
}

func virtualListViewResponseControl(targetPosition int, contentCount int, resultCode int, contextID string) control {
	var content []byte
	content = append(content, encodeInt(targetPosition)...)
	content = append(content, encodeInt(contentCount)...)
	content = append(content, encodeEnumerated(resultCode)...)
	if contextID != "" {
		content = append(content, encodeOctetString(contextID)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlVirtualListViewResponse, Value: seq}
}

func parseMatchedValuesControl(controls []control) ([][]byte, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlMatchedValues {
			continue
		}
		if len(ctrl.Value) == 0 {
			return nil, true, fmt.Errorf("matched values control value is required")
		}
		if ctrl.Value[0]&0x80 != 0 {
			if err := validateFilter(ctrl.Value); err != nil {
				return nil, true, fmt.Errorf("matched values filter: %w", err)
			}
			return [][]byte{ctrl.Value}, true, nil
		}
		if ctrl.Value[0] != tagSequence {
			return nil, true, fmt.Errorf("matched values control value must be a sequence")
		}
		content, err := decodeContent(ctrl.Value[1:])
		if err != nil {
			return nil, true, err
		}
		var filters [][]byte
		for len(content) > 0 {
			item, rest, err := readRawTLV(content)
			if err != nil {
				return nil, true, err
			}
			if err := validateFilter(item); err != nil {
				return nil, true, fmt.Errorf("matched values filter: %w", err)
			}
			filters = append(filters, item)
			content = rest
		}
		if len(filters) == 0 {
			return nil, true, fmt.Errorf("matched values control requires at least one filter")
		}
		return filters, true, nil
	}
	return nil, false, nil
}

func applyMatchedValuesFilter(attrs map[string][]string, filters [][]byte) map[string][]string {
	filtered := make(map[string][]string, len(attrs))
	for name, values := range attrs {
		var kept []string
		for _, value := range values {
			for _, filter := range filters {
				if ldapAttributeValueMatchesFilter(name, value, filter) {
					kept = append(kept, value)
					break
				}
			}
		}
		if len(kept) > 0 {
			filtered[name] = kept
		}
	}
	return filtered
}

func parseAssertionControl(controls []control) ([]byte, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlAssertion {
			continue
		}
		if len(ctrl.Value) == 0 {
			return nil, true, fmt.Errorf("assertion control value is required")
		}
		if err := validateFilter(ctrl.Value); err != nil {
			return nil, true, fmt.Errorf("assertion control filter: %w", err)
		}
		return ctrl.Value, true, nil
	}
	return nil, false, nil
}

func evaluateSearchAssertion(filter []byte, baseObject string) (bool, error) {
	filterKinds, err := parseLDAPFilterPrincipalKinds(filter)
	if err != nil {
		return false, err
	}
	if _, noKindMatch := intersectPrincipalKinds(filterKinds, principalKindsForBaseDN(baseObject)); noKindMatch {
		return false, nil
	}
	attr, value, ok, err := parseLDAPFilterCandidate(filter)
	if err != nil {
		return false, err
	}
	if !ok {
		return true, nil
	}
	if strings.EqualFold(strings.TrimSpace(attr), "objectClass") && strings.TrimSpace(value) == "*" {
		return true, nil
	}
	return true, nil
}

func parseSubentriesControl(controls []control) (bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlSubentries {
			continue
		}
		if len(ctrl.Value) == 0 {
			return true, nil
		}
		value, rest, err := decodeBoolean(ctrl.Value)
		if err != nil {
			return false, fmt.Errorf("subentries control: %w", err)
		}
		if len(rest) != 0 {
			return false, fmt.Errorf("subentries control has trailing data")
		}
		return value, nil
	}
	return false, nil
}

func authorizeProxiedAuthorization(controls []control, boundAuthzID string) error {
	for _, ctrl := range controls {
		if ctrl.Type != controlProxiedAuthorization {
			continue
		}
		requested := strings.TrimSpace(string(ctrl.Value))
		if requested == "" {
			return fmt.Errorf("proxied authorization identity is required")
		}
		if !strings.EqualFold(requested, strings.TrimSpace(boundAuthzID)) {
			return fmt.Errorf("proxied authorization denied")
		}
	}
	return nil
}

type syncRequest struct {
	Mode   int
	Cookie string
}

func parseSyncRequestControl(controls []control) (syncRequest, bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlSyncRequest {
			continue
		}
		req, err := decodeSyncRequestControlValue(ctrl.Value)
		if err != nil {
			return syncRequest{}, true, err
		}
		return req, true, nil
	}
	return syncRequest{}, false, nil
}

func decodeSyncRequestControlValue(value []byte) (syncRequest, error) {
	if len(value) == 0 || value[0] != tagSequence {
		return syncRequest{}, fmt.Errorf("sync request control value must be a sequence")
	}
	content, err := decodeContent(value[1:])
	if err != nil {
		return syncRequest{}, err
	}
	mode, rest, err := decodeEnumeratedWithRest(content)
	if err != nil {
		return syncRequest{}, fmt.Errorf("sync request mode: %w", err)
	}
	switch mode {
	case syncModeRefreshOnly, syncModeRefreshAndPersist:
	default:
		return syncRequest{}, fmt.Errorf("sync request mode %d is unsupported", mode)
	}
	req := syncRequest{Mode: mode}
	if len(rest) > 0 && rest[0] == tagOctetString {
		cookie, next, err := decodeOctetString(rest)
		if err != nil {
			return syncRequest{}, fmt.Errorf("sync request cookie: %w", err)
		}
		req.Cookie = cookie
		rest = next
	}
	if len(rest) > 0 && rest[0] == tagBoolean {
		_, next, err := decodeBoolean(rest)
		if err != nil {
			return syncRequest{}, fmt.Errorf("sync request reloadHint: %w", err)
		}
		rest = next
	}
	if len(rest) != 0 {
		return syncRequest{}, fmt.Errorf("sync request control has trailing data")
	}
	return req, nil
}

func syncStateControl(dn, cookie string) control {
	var content []byte
	content = append(content, encodeEnumerated(syncStateAdd)...)
	content = append(content, encodeOctetStringBytes(ldapEntryUUIDBytes(dn))...)
	if cookie != "" {
		content = append(content, encodeOctetString(cookie)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlSyncState, Value: seq}
}

func syncDoneControl(cookie string) control {
	var content []byte
	if cookie != "" {
		content = append(content, encodeOctetString(cookie)...)
	}
	var seq []byte
	seq = append(seq, tagSequence)
	seq = append(seq, encodeLength(len(content))...)
	seq = append(seq, content...)
	return control{Type: controlSyncDone, Value: seq}
}

func parseDereferenceControl(controls []control) (bool, error) {
	for _, ctrl := range controls {
		if ctrl.Type != controlDereferenceRequest {
			continue
		}
		if err := decodeDereferenceControlValue(ctrl.Value); err != nil {
			return true, err
		}
		return true, nil
	}
	return false, nil
}

func decodeDereferenceControlValue(value []byte) error {
	content, rest, err := decodeTaggedContent(value, tagSequence)
	if err != nil {
		return fmt.Errorf("dereference control value must be a sequence: %w", err)
	}
	if len(rest) != 0 {
		return fmt.Errorf("dereference control has trailing data")
	}
	if len(content) == 0 {
		return fmt.Errorf("dereference control requires at least one dereference spec")
	}
	for len(content) > 0 {
		var spec []byte
		spec, content, err = decodeTaggedContent(content, tagSequence)
		if err != nil {
			return fmt.Errorf("dereference spec: %w", err)
		}
		derefAttr, specRest, err := decodeOctetString(spec)
		if err != nil {
			return fmt.Errorf("dereference attribute: %w", err)
		}
		if strings.TrimSpace(derefAttr) == "" {
			return fmt.Errorf("dereference attribute is required")
		}
		attrs, specRest, err := decodeTaggedContent(specRest, tagSequence)
		if err != nil {
			return fmt.Errorf("dereference attribute list: %w", err)
		}
		for len(attrs) > 0 {
			attr, next, err := decodeOctetString(attrs)
			if err != nil {
				return fmt.Errorf("dereference returned attribute: %w", err)
			}
			if strings.TrimSpace(attr) == "" {
				return fmt.Errorf("dereference returned attribute is required")
			}
			attrs = next
		}
		if len(specRest) != 0 {
			return fmt.Errorf("dereference spec has trailing data")
		}
	}
	return nil
}

func isSupportedControl(controlType string) bool {
	switch strings.TrimSpace(controlType) {
	case controlManageDsaIT, controlPagedResults, controlServerSideSortRequest, controlVirtualListViewRequest, controlAssertion, controlMatchedValues,
		controlDomainScope, controlDontUseCopy, controlDontUseCopyOpenLDAP, controlSubentries, controlSyncRequest, controlProxiedAuthorization,
		controlDereferenceRequest, controlRelax, controlNoOp, controlPreRead, controlPostRead, controlPasswordPolicy, controlSessionTracking:
		return true
	default:
		return false
	}
}

const (
	controlManageDsaIT             = "2.16.840.1.113730.3.4.2"
	controlPagedResults            = "1.2.840.113556.1.4.319"
	controlServerSideSortRequest   = "1.2.840.113556.1.4.473"
	controlServerSideSortResponse  = "1.2.840.113556.1.4.474"
	controlVirtualListViewRequest  = "2.16.840.1.113730.3.4.9"
	controlVirtualListViewResponse = "2.16.840.1.113730.3.4.10"
	controlAssertion               = "1.3.6.1.1.12"
	controlMatchedValues           = "1.2.826.0.1.3344810.2.3"
	controlDomainScope             = "1.2.840.113556.1.4.1339"
	controlDontUseCopy             = "1.3.6.1.1.22"
	controlDontUseCopyOpenLDAP     = "1.3.6.1.4.1.4203.666.5.15"
	controlSubentries              = "1.3.6.1.4.1.4203.1.10.1"
	controlSyncRequest             = "1.3.6.1.4.1.4203.1.9.1.1"
	controlSyncState               = "1.3.6.1.4.1.4203.1.9.1.2"
	controlSyncDone                = "1.3.6.1.4.1.4203.1.9.1.3"
	controlProxiedAuthorization    = "2.16.840.1.113730.3.4.18"
	controlDereferenceRequest      = "1.3.6.1.4.1.4203.666.5.16"
	controlRelax                   = "1.3.6.1.4.1.4203.666.5.12"
	controlNoOp                    = "1.3.6.1.4.1.4203.666.5.2"
	controlPreRead                 = "1.3.6.1.1.13.1"
	controlPostRead                = "1.3.6.1.1.13.2"
	controlPasswordPolicy          = "1.3.6.1.4.1.42.2.27.8.5.1"
	controlSessionTracking         = "1.3.6.1.4.1.21008.108.63.1"
	syncModeRefreshOnly            = 1
	syncModeRefreshAndPersist      = 3
	syncStateAdd                   = 1
)
