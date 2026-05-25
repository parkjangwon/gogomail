package imapgw

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"mime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

func (s *Server) handleSearch(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	returnOptions, searchFields, ok := imapSearchReturnOptions(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD SEARCH return options are unsupported\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if !imapSearchFieldsContainCriteria(searchFields) {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	criteria, charsetOK := imapSearchCriteria(searchFields)
	if !charsetOK {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] SEARCH charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(criteria) == 0 {
		_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	requestsModSeq := imapSearchRequestsModSeq(criteria)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "SEARCH")
		}
		state.condstoreAware = true
	}

	// Fast path: UID SEARCH with criteria fully satisfied by the search index.
	// Avoids loading the full mailbox from Postgres when all predicates can be
	// answered by OpenSearch directly.
	if uidMode && !requestsModSeq && !returnOptions.save {
		if handled, err := s.imapSearchUIDFastPath(state.ctx, state, criteria, returnOptions, tag, writer); handled {
			return false, err
		}
	}

	// Slow path: load all message summaries from Postgres, then filter.
	messages, err := s.options.Backend.ListMessages(state.ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	results, highestModSeq, ok, err := s.imapSearchResults(state.ctx, state, criteria, messages, uidMode, requestsModSeq)
	if err != nil {
		if returnOptions.save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SEARCH failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
		return false, err
	}
	if returnOptions.save {
		state.savedSearch = imapSavedSearchResults(results, returnOptions.options)
	}
	if returnOptions.extended && len(returnOptions.options) > 0 {
		if _, err := writer.WriteString(imapESearchResponse(tag, results, uidMode, returnOptions.options, highestModSeq, requestsModSeq) + "\r\n"); err != nil {
			return false, err
		}
	} else if !returnOptions.save {
		if _, err := writer.WriteString("* SEARCH" + imapSearchResultSuffix(imapSearchResultValues(results), highestModSeq, requestsModSeq) + "\r\n"); err != nil {
			return false, err
		}
	}
	completion := "SEARCH"
	if uidMode {
		completion = "UID SEARCH"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

func (s *Server) handleSort(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	save, sortFields, ok := imapSearchSaveReturnOption(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT return options are unsupported\r\n")
		return false, err
	}
	if len(sortFields) < 3 {
		_, err := writer.WriteString(tag + " BAD SORT requires sort criteria, charset, and search criteria\r\n")
		return false, err
	}
	sortCriteria, searchFields, charsetOK, ok := imapSortCommandArguments(sortFields)
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
		return false, err
	}
	if charsetOK {
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD SORT " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
			return false, err
		}
	}
	if !charsetOK {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] SORT charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD SORT requires search criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(state.ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SORT failed\r\n")
		return false, writeErr
	}
	requestsModSeq := imapSearchRequestsModSeq(searchFields)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "SORT")
		}
		state.condstoreAware = true
	}
	results, _, ok, err := s.imapSearchResults(state.ctx, state, searchFields, messages, uidMode, false)
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO SORT failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD SORT search criteria are unsupported\r\n")
		return false, err
	}
	imapSortMatches(results, sortCriteria)
	if save {
		state.savedSearch = imapSavedSearchMatches(results)
	}
	if _, err := writer.WriteString("* SORT" + imapSearchResultSuffix(imapSearchResultValues(results), 0, false) + "\r\n"); err != nil {
		return false, err
	}
	completion := "SORT"
	if uidMode {
		completion = "UID SORT"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

func (s *Server) handleThread(writer *bufio.Writer, tag string, fields []string, state *imapConnState, uidMode bool) (bool, error) {
	save, threadFields, ok := imapSearchSaveReturnOption(fields[2:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD return options are unsupported\r\n")
		return false, err
	}
	if len(threadFields) < 3 {
		_, err := writer.WriteString(tag + " BAD THREAD requires algorithm, charset, and search criteria\r\n")
		return false, err
	}
	algorithm, searchFields, charsetOK, ok := imapThreadCommandArguments(threadFields)
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
		return false, err
	}
	if charsetOK {
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD THREAD " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
			return false, err
		}
	}
	if !imapThreadAlgorithmIsSupported(algorithm) {
		_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
		return false, err
	}
	if !charsetOK {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO [BADCHARSET (US-ASCII UTF-8)] THREAD charset is unsupported\r\n")
		return false, err
	}
	if state.session == nil {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if len(searchFields) == 0 {
		_, err := writer.WriteString(tag + " BAD THREAD requires search criteria\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		if save {
			state.savedSearch = nil
		}
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	messages, err := s.options.Backend.ListMessages(state.ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO THREAD failed\r\n")
		return false, writeErr
	}
	requestsModSeq := imapSearchRequestsModSeq(searchFields)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, "THREAD")
		}
		state.condstoreAware = true
	}
	results, _, ok, err := s.imapSearchResults(state.ctx, state, searchFields, messages, uidMode, false)
	if err != nil {
		if save {
			state.savedSearch = nil
		}
		_, writeErr := writer.WriteString(tag + " NO THREAD failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD THREAD search criteria are unsupported\r\n")
		return false, err
	}
	if save {
		state.savedSearch = imapSavedSearchMatches(results)
	}
	if _, err := writer.WriteString(imapOrderedSubjectThreadResponse(results) + "\r\n"); err != nil {
		return false, err
	}
	completion := "THREAD"
	if uidMode {
		completion = "UID THREAD"
	}
	_, err = writer.WriteString(tag + " OK " + completion + " completed\r\n")
	return false, err
}

type imapSearchReturnSpec struct {
	extended bool
	save     bool
	options  []string
}

func imapSearchReturnOptions(fields []string) (imapSearchReturnSpec, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return imapSearchReturnSpec{}, fields, true
	}
	if len(fields) < 2 {
		return imapSearchReturnSpec{}, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return imapSearchReturnSpec{}, nil, false
	}
	if len(tokens) == 0 {
		tokens = []string{"ALL"}
	}
	seen := make(map[string]struct{}, len(tokens))
	options := make([]string, 0, len(tokens))
	for _, token := range tokens {
		option := strings.ToUpper(token)
		switch option {
		case "MIN", "MAX", "ALL", "COUNT", "SAVE":
		default:
			return imapSearchReturnSpec{}, nil, false
		}
		if _, ok := seen[option]; ok {
			return imapSearchReturnSpec{}, nil, false
		}
		seen[option] = struct{}{}
		if option == "SAVE" {
			continue
		}
		options = append(options, option)
	}
	_, save := seen["SAVE"]
	return imapSearchReturnSpec{extended: true, save: save, options: options}, rest, true
}

func imapSearchSaveReturnOption(fields []string) (bool, []string, bool) {
	if len(fields) == 0 || !strings.EqualFold(fields[0], "RETURN") {
		return false, fields, true
	}
	if len(fields) < 2 {
		return false, nil, false
	}
	optionFields, rest, ok := imapConsumeParenthesizedFields(fields[1:])
	if !ok {
		return false, nil, false
	}
	tokens, ok := imapSearchReturnOptionTokens(optionFields)
	if !ok {
		return false, nil, false
	}
	if len(tokens) != 1 || !strings.EqualFold(tokens[0], "SAVE") {
		return false, nil, false
	}
	return true, rest, true
}

func imapSearchReturnOptionTokens(fields []string) ([]string, bool) {
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return nil, false
	}
	return imapParenthesizedAtomListTokens(inner)
}

func imapSearchFieldsContainCriteria(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	if strings.EqualFold(fields[0], "CHARSET") {
		return len(fields) >= 3
	}
	return true
}

func imapSearchCriteriaSyntaxError(fields []string) (string, bool) {
	criteria, charsetOK := imapSearchCriteria(fields)
	if !charsetOK {
		return "", false
	}
	if !imapSearchCriteriaSyntaxValid(criteria) {
		return "SEARCH criteria are unsupported", true
	}
	return "", false
}

func imapSearchCriteriaSyntaxValid(criteria []string) bool {
	for i := 0; i < len(criteria); {
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
		if !ok || consumed <= 0 {
			return false
		}
		i += consumed
	}
	return true
}

func imapSearchCriterionSyntaxConsumed(criteria []string) (int, bool) {
	if len(criteria) == 0 {
		return 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "(":
		i := 1
		seenSearchKey := false
		for i < len(criteria) {
			if criteria[i] == ")" {
				break
			}
			consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[i:])
			if !ok {
				return 0, false
			}
			seenSearchKey = true
			i += consumed
		}
		if i >= len(criteria) || criteria[i] != ")" || !seenSearchKey {
			return 0, false
		}
		return i + 1, true
	case "ALL", "SEEN", "UNSEEN", "FLAGGED", "UNFLAGGED", "ANSWERED", "UNANSWERED", "DRAFT", "UNDRAFT", "DELETED", "UNDELETED", "RECENT", "OLD", "NEW":
		return 1, true
	case "NOT":
		consumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		return consumed + 1, true
	case "OR":
		leftConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1:])
		if !ok {
			return 0, false
		}
		rightConsumed, ok := imapSearchCriterionSyntaxConsumed(criteria[1+leftConsumed:])
		if !ok {
			return 0, false
		}
		return 1 + leftConsumed + rightConsumed, true
	case "UID":
		if len(criteria) < 2 || !imapUIDSetSyntaxValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "SINCE", "BEFORE", "ON", "SENTSINCE", "SENTBEFORE", "SENTON":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchDate(criteria[1])
		return 2, ok
	case "LARGER", "SMALLER":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := parseIMAPSearchSize(criteria[1])
		return 2, ok
	case "MODSEQ":
		_, consumed, ok := parseIMAPSearchModSeq(criteria)
		return consumed, ok
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return 0, false
		}
		return 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT", "BODY", "TEXT":
		if len(criteria) < 2 {
			return 0, false
		}
		_, ok := imapSearchStringArgument(criteria[1])
		return 2, ok
	case "HEADER":
		if len(criteria) < 3 {
			return 0, false
		}
		fieldName, ok := imapSearchStringArgument(criteria[1])
		if !ok || !imapSearchHeaderFieldNameValid(fieldName) {
			return 0, false
		}
		if _, ok := imapSearchStringArgument(criteria[2]); !ok {
			return 0, false
		}
		return 3, true
	default:
		if imapSearchCriterionLooksLikeSequenceSet(criteria[0]) {
			return 1, imapSequenceSetSyntaxValid(criteria[0])
		}
		return 0, false
	}
}

func imapSearchCriterionLooksLikeSequenceSet(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if value == "*" || value == "$" || strings.ContainsAny(value, ":,") {
		return true
	}
	first := value[0]
	return first == '+' || first == '-' || ('0' <= first && first <= '9')
}

func imapConsumeParenthesizedFields(fields []string) ([]string, []string, bool) {
	if len(fields) == 0 {
		return nil, nil, false
	}
	depth := 0
	for i, field := range fields {
		if i == 0 && !strings.HasPrefix(strings.TrimSpace(field), "(") {
			return nil, nil, false
		}
		depth += strings.Count(field, "(")
		depth -= strings.Count(field, ")")
		if depth == 0 {
			return fields[:i+1], fields[i+1:], true
		}
		if depth < 0 {
			return nil, nil, false
		}
	}
	return nil, nil, false
}

func imapSearchCriteria(criteria []string) ([]string, bool) {
	if len(criteria) >= 2 && strings.EqualFold(criteria[0], "CHARSET") {
		charset, ok := imapSupportedCharset(criteria[1])
		if !ok {
			return nil, false
		}
		switch charset {
		case "US-ASCII", "UTF-8":
			return imapNormalizeSearchCriteria(criteria[2:]), true
		}
	}
	return imapNormalizeSearchCriteria(criteria), true
}

func imapSupportedCharset(value string) (string, bool) {
	if strings.Contains(value, `"`) {
		return "", false
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}
	charset := strings.ToUpper(value)
	switch charset {
	case "US-ASCII", "UTF-8":
		return charset, true
	default:
		return "", false
	}
}

func imapNormalizeSearchCriteria(criteria []string) []string {
	normalized := make([]string, 0, len(criteria))
	for _, token := range criteria {
		if token == "" {
			normalized = append(normalized, imapEmptySearchStringToken)
			continue
		}
		for strings.HasPrefix(token, "(") {
			normalized = append(normalized, "(")
			token = token[1:]
		}
		trailingGroups := 0
		for strings.HasSuffix(token, ")") {
			trailingGroups++
			token = strings.TrimSuffix(token, ")")
		}
		if token != "" {
			normalized = append(normalized, token)
		}
		for ; trailingGroups > 0; trailingGroups-- {
			normalized = append(normalized, ")")
		}
	}
	return normalized
}

type imapSearchMatch struct {
	value          uint32
	uid            UID
	sequenceNumber uint32
	modSeq         uint64
	summary        MessageSummary
}

type imapSearchSavedMessage struct {
	uid            UID
	sequenceNumber uint32
}

func (s *Server) imapSearchResults(ctx context.Context, state *imapConnState, criteria []string, messages []MessageSummary, uidMode bool, requestsModSeq bool) ([]imapSearchMatch, uint64, bool, error) {
	if len(criteria) == 0 {
		return nil, 0, false, nil
	}
	maxUID := imapMaxSummaryUID(messages)
	candidateIDs := s.imapSearchCandidateIDs(ctx, state, criteria)
	predicates := make([]imapSearchPredicate, 0, len(criteria))
	for i := 0; i < len(criteria); {
		predicate, consumed, ok := imapParseSearchPredicate(criteria[i:], state.selectedMessages, maxUID, state)
		if !ok {
			return nil, 0, false, nil
		}
		if predicate != nil {
			predicates = append(predicates, predicate)
		}
		i += consumed
	}
	results := make([]imapSearchMatch, 0, len(messages))
	var highestModSeq uint64
	for i, summary := range messages {
		if candidateIDs != nil {
			if _, ok := candidateIDs[summary.ID]; !ok {
				continue
			}
		}
		matches, err := s.imapMessageMatchesSearchPredicates(ctx, state, summary, i, predicates)
		if err != nil {
			return nil, 0, true, err
		}
		if !matches {
			continue
		}
		if requestsModSeq && summary.ModSeq > highestModSeq {
			highestModSeq = summary.ModSeq
		}
		var value uint32
		if uidMode {
			value = uint32(summary.UID)
		} else {
			value = imapSearchSequenceNumber(summary, i)
		}
		results = append(results, imapSearchMatch{
			value:          value,
			uid:            summary.UID,
			sequenceNumber: imapSearchSequenceNumber(summary, i),
			modSeq:         summary.ModSeq,
			summary:        summary,
		})
	}
	if len(results) == 0 {
		highestModSeq = 0
	}
	return results, highestModSeq, true, nil
}

// imapSearchUIDFastPath handles UID SEARCH when every criterion is fully
// satisfied by the search index (no MODSEQ, no SAVE). It queries the search
// index for matching message IDs, resolves them to IMAP UIDs via a targeted
// Postgres query, and writes the SEARCH/ESEARCH response, all without
// loading the full mailbox into memory.
//
// Returns (true, err) when the response was written (caller must return),
// (false, nil) to signal that the slow path should be used instead.
func (s *Server) imapSearchUIDFastPath(
	ctx context.Context,
	state *imapConnState,
	criteria []string,
	returnOptions imapSearchReturnSpec,
	tag string,
	writer *bufio.Writer,
) (bool, error) {
	if s == nil || s.options.Backend == nil {
		return false, nil
	}
	// Check that all criteria are satisfied by OpenSearch (ok=true) and at
	// least one filter is active (used=true, i.e. not just "ALL").
	osReq, ok := imapOpenSearchSearchRequest(state, criteria)
	if !ok {
		return false, nil
	}
	source, hasSearch := s.options.Backend.(SearchMessageIDSource)
	lookup, hasLookup := s.options.Backend.(MessageUIDLookup)
	if !hasSearch || !hasLookup {
		return false, nil
	}

	osReq.Limit = maxIMAPSearchFastPathLimit
	messageIDs, err := source.SearchMessageIDs(ctx, osReq)
	if err != nil || len(messageIDs) >= maxIMAPSearchFastPathLimit {
		// Error or result set too large — fall back to slow path.
		return false, nil
	}

	uidMap, err := lookup.LookupMessageUIDs(ctx, state.session.UserID, state.selectedMailbox, messageIDs)
	if err != nil {
		return false, nil
	}

	results := make([]imapSearchMatch, 0, len(uidMap))
	for _, uid := range uidMap {
		results = append(results, imapSearchMatch{
			value: uint32(uid),
			uid:   uid,
		})
	}
	sort.Slice(results, func(i, j int) bool { return results[i].uid < results[j].uid })

	if returnOptions.extended && len(returnOptions.options) > 0 {
		if _, err := writer.WriteString(imapESearchResponse(tag, results, true, returnOptions.options, 0, false) + "\r\n"); err != nil {
			return true, err
		}
	} else {
		if _, err := writer.WriteString("* SEARCH" + imapSearchResultSuffix(imapSearchResultValues(results), 0, false) + "\r\n"); err != nil {
			return true, err
		}
	}
	_, err = writer.WriteString(tag + " OK UID SEARCH completed\r\n")
	return true, err
}

func (s *Server) imapSearchCandidateIDs(ctx context.Context, state *imapConnState, criteria []string) map[MessageID]struct{} {
	req, ok := imapOpenSearchSearchRequest(state, criteria)
	if !ok || s == nil || s.options.Backend == nil {
		return nil
	}
	source, ok := s.options.Backend.(SearchMessageIDSource)
	if !ok {
		return nil
	}
	req.Limit = maxIMAPSearchOpenSearchCandidates
	ids, err := source.SearchMessageIDs(ctx, req)
	if err != nil || len(ids) >= maxIMAPSearchOpenSearchCandidates {
		return nil
	}
	candidates := make(map[MessageID]struct{}, len(ids))
	for _, id := range ids {
		trimmed := strings.TrimSpace(string(id))
		if trimmed == "" {
			continue
		}
		candidates[MessageID(trimmed)] = struct{}{}
	}
	return candidates
}

func imapOpenSearchSearchRequest(state *imapConnState, criteria []string) (SearchMessagesRequest, bool) {
	if state == nil || state.session == nil || state.selectedMailbox == "" {
		return SearchMessagesRequest{}, false
	}
	req := SearchMessagesRequest{
		UserID:        state.session.UserID,
		MailboxID:     state.selectedMailbox,
		Limit:         maxIMAPSearchOpenSearchCandidates,
		HasAttachment: nil,
	}
	used, ok := imapCollectOpenSearchSearchRequest(criteria, &req)
	if !ok || !used {
		return SearchMessagesRequest{}, false
	}
	return req, true
}

func imapCollectOpenSearchSearchRequest(criteria []string, req *SearchMessagesRequest) (bool, bool) {
	used := false
	for i := 0; i < len(criteria); {
		consumed, tokenUsed, ok := imapCollectOpenSearchSearchRequestToken(criteria[i:], req)
		if !ok || consumed <= 0 {
			return false, false
		}
		used = used || tokenUsed
		i += consumed
	}
	return used, true
}

func imapCollectOpenSearchSearchRequestToken(criteria []string, req *SearchMessagesRequest) (int, bool, bool) {
	if len(criteria) == 0 {
		return 0, false, false
	}
	switch token := strings.ToUpper(criteria[0]); token {
	case "(":
		end, ok := imapOpenSearchGroupEnd(criteria)
		if !ok || end < 3 {
			return 0, false, false
		}
		used, ok := imapCollectOpenSearchSearchRequest(criteria[1:end-1], req)
		if !ok {
			return 0, false, false
		}
		return end, used, true
	case "OR", "NOT", "TEXT", "HEADER", "KEYWORD", "UNKEYWORD", "UID", "MODSEQ", "LARGER", "SMALLER":
		return 0, false, false
	case "ALL":
		return 1, false, true
	case "BODY":
		if len(criteria) < 2 {
			return 0, false, false
		}
		value, ok := imapSearchStringArgument(criteria[1])
		if !ok || strings.TrimSpace(value) == "" {
			return 0, false, false
		}
		req.Query = strings.TrimSpace(req.Query + " " + value)
		return 2, true, true
	case "FROM":
		return imapAssignOpenSearchString(&req.From, criteria)
	case "TO":
		return imapAssignOpenSearchString(&req.To, criteria)
	case "CC":
		return imapAssignOpenSearchString(&req.Cc, criteria)
	case "BCC":
		return imapAssignOpenSearchString(&req.Bcc, criteria)
	case "SUBJECT":
		return imapAssignOpenSearchString(&req.Subject, criteria)
	case "SINCE":
		return imapAssignOpenSearchDate(&req.Since, criteria)
	case "BEFORE":
		return imapAssignOpenSearchBeforeDate(&req.Until, criteria)
	case "ON":
		if len(criteria) < 2 {
			return 0, false, false
		}
		if err := imapAssignOpenSearchExactDate(req, criteria[1]); err != nil {
			return 0, false, false
		}
		return 2, true, true
	default:
		return 0, false, false
	}
}

func imapOpenSearchGroupEnd(criteria []string) (int, bool) {
	depth := 0
	for i, token := range criteria {
		switch token {
		case "(":
			depth++
		case ")":
			depth--
			if depth == 0 {
				return i + 1, true
			}
			if depth < 0 {
				return 0, false
			}
		}
	}
	return 0, false
}

func imapAssignOpenSearchString(dst *string, criteria []string) (int, bool, bool) {
	if len(criteria) < 2 {
		return 0, false, false
	}
	value, ok := imapSearchStringArgument(criteria[1])
	if !ok {
		return 0, false, false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false, false
	}
	if strings.TrimSpace(*dst) != "" && !strings.EqualFold(strings.TrimSpace(*dst), value) {
		return 0, false, false
	}
	*dst = value
	return 2, true, true
}

func imapAssignOpenSearchDate(dst *string, criteria []string) (int, bool, bool) {
	if len(criteria) < 2 {
		return 0, false, false
	}
	value := strings.TrimSpace(criteria[1])
	if value == "" {
		return 0, false, false
	}
	if _, ok := parseIMAPSearchDate(value); !ok {
		return 0, false, false
	}
	if strings.TrimSpace(*dst) != "" && !strings.EqualFold(strings.TrimSpace(*dst), value) {
		return 0, false, false
	}
	*dst = value
	return 2, true, true
}

func imapAssignOpenSearchBeforeDate(dst *string, criteria []string) (int, bool, bool) {
	if len(criteria) < 2 {
		return 0, false, false
	}
	value := strings.TrimSpace(criteria[1])
	if value == "" {
		return 0, false, false
	}
	if _, ok := parseIMAPSearchDate(value); !ok {
		return 0, false, false
	}
	if strings.TrimSpace(*dst) != "" && !strings.EqualFold(strings.TrimSpace(*dst), value) {
		return 0, false, false
	}
	*dst = value
	return 2, true, true
}

func imapAssignOpenSearchExactDate(req *SearchMessagesRequest, value string) error {
	if _, ok := parseIMAPSearchDate(value); !ok {
		return fmt.Errorf("invalid IMAP search date")
	}
	if strings.TrimSpace(req.Since) != "" && !strings.EqualFold(strings.TrimSpace(req.Since), value) {
		return fmt.Errorf("conflicting IMAP search date")
	}
	if strings.TrimSpace(req.Until) != "" && !strings.EqualFold(strings.TrimSpace(req.Until), value) {
		return fmt.Errorf("conflicting IMAP search date")
	}
	req.Since = value
	req.Until = value
	return nil
}

func imapMaxSummaryUID(messages []MessageSummary) UID {
	var maxUID UID
	for _, message := range messages {
		if message.UID > maxUID {
			maxUID = message.UID
		}
	}
	return maxUID
}

type imapSortCriterion struct {
	key     string
	reverse bool
}

func imapSortCommandArguments(fields []string) ([]imapSortCriterion, []string, bool, bool) {
	sortFields, rest, ok := imapConsumeParenthesizedFields(fields)
	if !ok || len(rest) < 2 {
		return nil, nil, true, false
	}
	criteria, ok := imapSortCriteria(sortFields)
	if !ok || len(criteria) == 0 {
		return nil, nil, true, false
	}
	if _, ok := imapSupportedCharset(rest[0]); !ok {
		return nil, nil, false, true
	}
	return criteria, imapNormalizeSearchCriteria(rest[1:]), true, true
}

func imapThreadCommandArguments(fields []string) (string, []string, bool, bool) {
	if len(fields) < 3 {
		return "", nil, true, false
	}
	algorithm, ok := imapThreadAlgorithm(fields[0])
	if !ok {
		return "", nil, true, true
	}
	if _, ok := imapSupportedCharset(fields[1]); !ok {
		return algorithm, nil, false, true
	}
	return algorithm, imapNormalizeSearchCriteria(fields[2:]), true, true
}

func imapThreadAlgorithm(value string) (string, bool) {
	if strings.Contains(value, `"`) {
		return "", false
	}
	if strings.TrimSpace(value) != value {
		return "", false
	}
	algorithm := strings.ToUpper(value)
	if algorithm == "" {
		return "", false
	}
	return algorithm, true
}

func imapThreadAlgorithmIsSupported(algorithm string) bool {
	return algorithm == "ORDEREDSUBJECT"
}

func imapSortCriteria(fields []string) ([]imapSortCriterion, bool) {
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return nil, false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return nil, false
	}
	criteria := make([]imapSortCriterion, 0, len(tokens))
	for i := 0; i < len(tokens); i++ {
		reverse := false
		token := strings.ToUpper(tokens[i])
		if token == "REVERSE" {
			reverse = true
			i++
			if i >= len(tokens) {
				return nil, false
			}
			token = strings.ToUpper(tokens[i])
		}
		switch token {
		case "ARRIVAL", "CC", "DATE", "FROM", "SIZE", "SUBJECT", "TO":
			criteria = append(criteria, imapSortCriterion{key: token, reverse: reverse})
		default:
			return nil, false
		}
	}
	return criteria, true
}

func imapSortMatches(matches []imapSearchMatch, criteria []imapSortCriterion) {
	collator := collate.New(language.Und, collate.IgnoreCase)
	sort.SliceStable(matches, func(i, j int) bool {
		left := matches[i]
		right := matches[j]
		for _, criterion := range criteria {
			cmp := imapCompareSortCriterion(collator, left.summary, right.summary, criterion.key)
			if cmp == 0 {
				continue
			}
			if criterion.reverse {
				return cmp > 0
			}
			return cmp < 0
		}
		return left.sequenceNumber < right.sequenceNumber
	})
}

type imapThreadGroup struct {
	subject string
	matches []imapSearchMatch
}

func imapOrderedSubjectThreadResponse(matches []imapSearchMatch) string {
	if len(matches) == 0 {
		return "* THREAD"
	}
	criteria := []imapSortCriterion{{key: "SUBJECT"}, {key: "DATE"}}
	imapSortMatches(matches, criteria)
	groups := make([]imapThreadGroup, 0, len(matches))
	groupIndex := make(map[string]int)
	for _, match := range matches {
		subject := imapBaseSubject(match.summary.Envelope.Subject)
		key := strings.ToLower(subject)
		if index, ok := groupIndex[key]; ok {
			groups[index].matches = append(groups[index].matches, match)
			continue
		}
		groupIndex[key] = len(groups)
		groups = append(groups, imapThreadGroup{subject: key, matches: []imapSearchMatch{match}})
	}
	sort.SliceStable(groups, func(i, j int) bool {
		left := groups[i].matches[0]
		right := groups[j].matches[0]
		cmp := imapCompareTime(imapSortSentDate(left.summary), imapSortSentDate(right.summary))
		if cmp != 0 {
			return cmp < 0
		}
		return left.sequenceNumber < right.sequenceNumber
	})
	threads := make([]string, 0, len(groups))
	for _, group := range groups {
		threads = append(threads, imapOrderedSubjectThread(group.matches))
	}
	return "* THREAD " + strings.Join(threads, "")
}

func imapOrderedSubjectThread(matches []imapSearchMatch) string {
	if len(matches) == 0 {
		return "()"
	}
	if len(matches) == 1 {
		return "(" + strconv.FormatUint(uint64(matches[0].value), 10) + ")"
	}
	if len(matches) == 2 {
		return "(" + strconv.FormatUint(uint64(matches[0].value), 10) + " " + strconv.FormatUint(uint64(matches[1].value), 10) + ")"
	}
	children := make([]string, 0, len(matches)-1)
	for _, match := range matches[1:] {
		children = append(children, "("+strconv.FormatUint(uint64(match.value), 10)+")")
	}
	return "(" + strconv.FormatUint(uint64(matches[0].value), 10) + " " + strings.Join(children, "") + ")"
}

func imapCompareSortCriterion(collator *collate.Collator, left MessageSummary, right MessageSummary, key string) int {
	switch key {
	case "ARRIVAL":
		return imapCompareTime(left.InternalDate, right.InternalDate)
	case "DATE":
		return imapCompareTime(imapSortSentDate(left), imapSortSentDate(right))
	case "SIZE":
		return imapCompareInt64(left.Size, right.Size)
	case "FROM":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.From), imapSortFirstMailbox(right.Envelope.From))
	case "TO":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.To), imapSortFirstMailbox(right.Envelope.To))
	case "CC":
		return collator.CompareString(imapSortFirstMailbox(left.Envelope.Cc), imapSortFirstMailbox(right.Envelope.Cc))
	case "SUBJECT":
		return collator.CompareString(imapBaseSubject(left.Envelope.Subject), imapBaseSubject(right.Envelope.Subject))
	default:
		return 0
	}
}

func imapSortSentDate(summary MessageSummary) time.Time {
	if !summary.Envelope.Date.IsZero() {
		return summary.Envelope.Date
	}
	return summary.InternalDate
}

func imapSortFirstMailbox(addresses []Address) string {
	if len(addresses) == 0 {
		return ""
	}
	return strings.TrimSpace(addresses[0].Mailbox)
}

func imapCompareTime(left time.Time, right time.Time) int {
	if left.Equal(right) {
		return 0
	}
	if left.Before(right) {
		return -1
	}
	return 1
}

func imapCompareInt64(left int64, right int64) int {
	if left == right {
		return 0
	}
	if left < right {
		return -1
	}
	return 1
}

func imapBaseSubject(subject string) string {
	subject = imapDecodeSubjectHeader(subject)
	subject = imapCollapseWhitespace(subject)
	for {
		previous := subject
		subject = imapStripSubjectTrailers(subject)
		for {
			stripped := imapStripSubjectLeader(subject)
			if stripped == subject {
				break
			}
			subject = stripped
			subject = imapStripSubjectTrailers(subject)
		}
		if inner, ok := imapStripForwardWrapper(subject); ok {
			subject = strings.TrimSpace(inner)
			continue
		}
		if subject == previous {
			return subject
		}
	}
}

func imapDecodeSubjectHeader(subject string) string {
	decoded, err := new(mime.WordDecoder).DecodeHeader(subject)
	if err != nil {
		return subject
	}
	return decoded
}

func imapStripSubjectTrailers(subject string) string {
	for {
		trimmed := strings.TrimSpace(subject)
		if strings.HasSuffix(strings.ToLower(trimmed), "(fwd)") {
			subject = strings.TrimSpace(trimmed[:len(trimmed)-5])
			continue
		}
		return trimmed
	}
}

func imapStripSubjectLeader(subject string) string {
	subject = strings.TrimSpace(subject)
	for {
		stripped, ok := imapStripSubjectBlob(subject)
		if !ok {
			break
		}
		subject = strings.TrimSpace(stripped)
	}
	lower := strings.ToLower(subject)
	for _, prefix := range []string{"re", "fw", "fwd"} {
		if !strings.HasPrefix(lower, prefix) {
			continue
		}
		rest := subject[len(prefix):]
		rest = strings.TrimLeft(rest, " \t")
		if strings.HasPrefix(rest, "[") {
			withoutBlob, ok := imapStripSubjectBlob(rest)
			if ok {
				rest = strings.TrimLeft(withoutBlob, " \t")
			}
		}
		if strings.HasPrefix(rest, ":") {
			return strings.TrimSpace(rest[1:])
		}
	}
	return subject
}

func imapCollapseWhitespace(value string) string {
	if value == "" {
		return ""
	}
	out := make([]byte, 0, len(value))
	inSpace := false
	for i := 0; i < len(value); i++ {
		switch value[i] {
		case ' ', '\t', '\r', '\n':
			if len(out) > 0 {
				inSpace = true
			}
		default:
			if inSpace {
				out = append(out, ' ')
				inSpace = false
			}
			out = append(out, value[i])
		}
	}
	return string(out)
}

func imapStripSubjectBlob(subject string) (string, bool) {
	if !strings.HasPrefix(subject, "[") {
		return subject, false
	}
	if strings.HasPrefix(strings.ToLower(subject), "[fwd:") {
		return subject, false
	}
	end := strings.Index(subject, "]")
	if end <= 0 {
		return subject, false
	}
	return subject[end+1:], true
}

func imapStripForwardWrapper(subject string) (string, bool) {
	trimmed := strings.TrimSpace(subject)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "[fwd:") && strings.HasSuffix(trimmed, "]") {
		return trimmed[5 : len(trimmed)-1], true
	}
	return subject, false
}

type imapSearchPredicate func(context.Context, *Server, *imapConnState, MessageSummary, int) (bool, error)

func imapParseSearchPredicate(criteria []string, maxSequence uint32, maxUID UID, state *imapConnState) (imapSearchPredicate, int, bool) {
	if len(criteria) == 0 {
		return nil, 0, false
	}
	criterion := strings.ToUpper(criteria[0])
	switch criterion {
	case "(":
		predicates := make([]imapSearchPredicate, 0, len(criteria))
		i := 1
		seenSearchKey := false
		for i < len(criteria) {
			if criteria[i] == ")" {
				break
			}
			predicate, consumed, ok := imapParseSearchPredicate(criteria[i:], maxSequence, maxUID, state)
			if !ok {
				return nil, 0, false
			}
			if predicate != nil {
				predicates = append(predicates, predicate)
			}
			seenSearchKey = true
			i += consumed
		}
		if i >= len(criteria) || criteria[i] != ")" || !seenSearchKey {
			return nil, 0, false
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			for _, predicate := range predicates {
				matches, err := imapSearchPredicateMatches(ctx, server, state, predicate, summary, index)
				if err != nil {
					return false, err
				}
				if !matches {
					return false, nil
				}
			}
			return true, nil
		}, i + 1, true
	case "ALL":
		return nil, 1, true
	case "NOT":
		predicate, consumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		if predicate == nil {
			return func(context.Context, *Server, *imapConnState, MessageSummary, int) (bool, error) { return false, nil }, consumed + 1, true
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			matches, err := predicate(ctx, server, state, summary, index)
			if err != nil {
				return false, err
			}
			return !matches, nil
		}, consumed + 1, true
	case "OR":
		left, leftConsumed, ok := imapParseSearchPredicate(criteria[1:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		right, rightConsumed, ok := imapParseSearchPredicate(criteria[1+leftConsumed:], maxSequence, maxUID, state)
		if !ok {
			return nil, 0, false
		}
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, index int) (bool, error) {
			leftMatches, err := imapSearchPredicateMatches(ctx, server, state, left, summary, index)
			if err != nil {
				return false, err
			}
			if leftMatches {
				return true, nil
			}
			return imapSearchPredicateMatches(ctx, server, state, right, summary, index)
		}, 1 + leftConsumed + rightConsumed, true
	case "SEEN", "UNSEEN", "FLAGGED", "UNFLAGGED", "ANSWERED", "UNANSWERED", "DRAFT", "UNDRAFT", "DELETED", "UNDELETED", "RECENT", "OLD", "NEW":
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesFlagSearch(summary, criterion), nil
		}, 1, true
	case "UID":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		var ranges []imapUIDRange
		if criteria[1] == "$" {
			uids := imapSavedSearchUIDs(state)
			ranges = make([]imapUIDRange, 0, len(uids))
			for _, uid := range uids {
				ranges = append(ranges, imapUIDRange{start: uid, end: uid})
			}
		} else {
			var ok bool
			ranges, ok = parseIMAPUIDSetRanges(criteria[1], maxUID)
			if !ok {
				return nil, 0, false
			}
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapUIDMatchesRanges(summary.UID, ranges), nil
		}, 2, true
	case "SINCE", "BEFORE", "ON":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		day, ok := parseIMAPSearchDate(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesDateSearch(summary, criterion, day), nil
		}, 2, true
	case "SENTSINCE", "SENTBEFORE", "SENTON":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		day, ok := parseIMAPSearchDate(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesSentDateSearch(summary, criterion, day), nil
		}, 2, true
	case "LARGER", "SMALLER":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		size, ok := parseIMAPSearchSize(criteria[1])
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesSizeSearch(summary, criterion, size), nil
		}, 2, true
	case "MODSEQ":
		threshold, consumed, ok := parseIMAPSearchModSeq(criteria)
		if !ok {
			return nil, 0, false
		}
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return summary.ModSeq >= threshold, nil
		}, consumed, true
	case "KEYWORD", "UNKEYWORD":
		if len(criteria) < 2 || !imapSearchKeywordValid(criteria[1]) {
			return nil, 0, false
		}
		keyword := criteria[1]
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			matches := imapMessageMatchesKeywordSearch(summary, keyword)
			if criterion == "UNKEYWORD" {
				return !matches, nil
			}
			return matches, nil
		}, 2, true
	case "FROM", "TO", "CC", "BCC", "SUBJECT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[1])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return imapMessageMatchesTextSearch(summary, criterion, query), nil
		}, 2, true
	case "HEADER":
		if len(criteria) < 3 {
			return nil, 0, false
		}
		fieldName, ok := imapSearchStringArgument(criteria[1])
		if !ok || !imapSearchHeaderFieldNameValid(fieldName) {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[2])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesHeaderSearch(ctx, state, summary, fieldName, query)
		}, 3, true
	case "BODY", "TEXT":
		if len(criteria) < 2 {
			return nil, 0, false
		}
		query, ok := imapSearchStringArgument(criteria[1])
		if !ok {
			return nil, 0, false
		}
		query = strings.ToLower(query)
		return func(ctx context.Context, server *Server, state *imapConnState, summary MessageSummary, _ int) (bool, error) {
			return server.imapMessageMatchesBodySearch(ctx, state, summary, criterion, query)
		}, 2, true
	default:
		sequenceNumbers, ok := parseIMAPSequenceSetForState(criteria[0], maxSequence, state)
		if ok {
			allowed := make(map[uint32]struct{}, len(sequenceNumbers))
			for _, sequenceNumber := range sequenceNumbers {
				allowed[sequenceNumber] = struct{}{}
			}
			return func(_ context.Context, _ *Server, _ *imapConnState, summary MessageSummary, index int) (bool, error) {
				_, ok := allowed[imapSearchSequenceNumber(summary, index)]
				return ok, nil
			}, 1, true
		}
		return nil, 0, false
	}
}

func imapSearchKeywordValid(value string) bool {
	return imapAtomValid(value)
}

func imapMessageMatchesKeywordSearch(summary MessageSummary, keyword string) bool {
	keyword = CanonicalIMAPFlag(keyword)
	for _, flag := range summary.Flags.IMAPFlags() {
		if flag == keyword {
			return true
		}
	}
	return false
}

func imapSearchStringArgument(value string) (string, bool) {
	if value == imapEmptySearchStringToken {
		return "", true
	}
	return value, true
}

func imapSearchRequestsModSeq(criteria []string) bool {
	for _, criterion := range criteria {
		if strings.EqualFold(criterion, "MODSEQ") {
			return true
		}
	}
	return false
}

func imapSearchPredicateMatches(ctx context.Context, server *Server, state *imapConnState, predicate imapSearchPredicate, summary MessageSummary, index int) (bool, error) {
	if predicate == nil {
		return true, nil
	}
	return predicate(ctx, server, state, summary, index)
}

func (s *Server) imapMessageMatchesSearchPredicates(ctx context.Context, state *imapConnState, summary MessageSummary, index int, predicates []imapSearchPredicate) (bool, error) {
	for _, predicate := range predicates {
		matches, err := imapSearchPredicateMatches(ctx, s, state, predicate, summary, index)
		if err != nil {
			return false, err
		}
		if !matches {
			return false, nil
		}
	}
	return true, nil
}

func imapSearchSequenceNumber(summary MessageSummary, index int) uint32 {
	if summary.SequenceNumber != 0 {
		return summary.SequenceNumber
	}
	return uint32(index + 1)
}

func imapSearchFlagResults(messages []MessageSummary, uidMode bool, criterion string) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesFlagSearch(summary, criterion) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesFlagSearch(summary MessageSummary, criterion string) bool {
	switch criterion {
	case "SEEN":
		return summary.Flags.Read
	case "UNSEEN":
		return !summary.Flags.Read
	case "FLAGGED":
		return summary.Flags.Starred
	case "UNFLAGGED":
		return !summary.Flags.Starred
	case "ANSWERED":
		return summary.Flags.Answered
	case "UNANSWERED":
		return !summary.Flags.Answered
	case "DRAFT":
		return summary.Flags.Draft || strings.EqualFold(strings.TrimSpace(summary.Flags.Status), "draft")
	case "UNDRAFT":
		return !summary.Flags.Draft && !strings.EqualFold(strings.TrimSpace(summary.Flags.Status), "draft")
	case "DELETED":
		return summary.Flags.Deleted
	case "UNDELETED":
		return !summary.Flags.Deleted
	case "RECENT":
		return summary.Recent
	case "NEW":
		return summary.Recent && !summary.Flags.Read
	case "OLD":
		return !summary.Recent
	default:
		return false
	}
}

func imapSearchAllResults(messages []MessageSummary, uidMode bool) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapSearchDateResults(messages []MessageSummary, uidMode bool, criterion string, day time.Time) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesDateSearch(summary, criterion, day) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesDateSearch(summary MessageSummary, criterion string, day time.Time) bool {
	date := summary.InternalDate
	if date.IsZero() {
		return false
	}
	messageDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	switch criterion {
	case "SINCE":
		return !messageDay.Before(day)
	case "BEFORE":
		return messageDay.Before(day)
	case "ON":
		return messageDay.Equal(day)
	default:
		return false
	}
}

func imapSearchSentDateResults(messages []MessageSummary, uidMode bool, criterion string, day time.Time) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesSentDateSearch(summary, criterion, day) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesSentDateSearch(summary MessageSummary, criterion string, day time.Time) bool {
	date := summary.Envelope.Date
	if date.IsZero() {
		return false
	}
	messageDay := time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
	switch criterion {
	case "SENTSINCE":
		return !messageDay.Before(day)
	case "SENTBEFORE":
		return messageDay.Before(day)
	case "SENTON":
		return messageDay.Equal(day)
	default:
		return false
	}
}

func parseIMAPSearchDate(value string) (time.Time, bool) {
	if strings.Contains(value, `"`) {
		return time.Time{}, false
	}
	if strings.TrimSpace(value) != value {
		return time.Time{}, false
	}
	var ok bool
	value, ok = imapCanonicalDateMonth(value)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{"02-Jan-2006", "2-Jan-2006"} {
		day, err := time.Parse(layout, value)
		if err == nil {
			return day, true
		}
	}
	return time.Time{}, false
}

func parseIMAPSearchSize(value string) (int64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomRFC3501(value) {
		return 0, false
	}
	size, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, false
	}
	return int64(size), true
}

func parseIMAPSearchModSeq(criteria []string) (uint64, int, bool) {
	if len(criteria) < 2 || !strings.EqualFold(criteria[0], "MODSEQ") {
		return 0, 0, false
	}
	if threshold, ok := parseIMAPModSeqValue(criteria[1]); ok {
		return threshold, 2, true
	}
	if len(criteria) < 4 || !imapSearchModSeqEntryTypeValid(criteria[2]) {
		return 0, 0, false
	}
	threshold, ok := parseIMAPModSeqValue(criteria[3])
	if !ok {
		return 0, 0, false
	}
	return threshold, 4, true
}

func parseIMAPModSeqValue(value string) (uint64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomDigitsOnly(value) {
		return 0, false
	}
	modseq, err := strconv.ParseUint(value, 10, 64)
	if err != nil || modseq == 0 {
		return 0, false
	}
	return modseq, true
}

func parseIMAPModSeqValzer(value string) (uint64, bool) {
	if strings.TrimSpace(value) != value {
		return 0, false
	}
	if !imapNumberAtomDigitsOnly(value) {
		return 0, false
	}
	modseq, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return modseq, true
}

func imapSearchModSeqEntryTypeValid(value string) bool {
	if strings.Contains(value, `"`) {
		return false
	}
	if strings.TrimSpace(value) != value {
		return false
	}
	switch strings.ToUpper(value) {
	case "SHARED", "PRIV", "ALL":
		return true
	default:
		return false
	}
}

func imapNumberAtomDigitsOnly(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		if value[i] < '0' || value[i] > '9' {
			return false
		}
	}
	return true
}

func imapNZNumberAtomDigitsOnly(value string) bool {
	if !imapNumberAtomDigitsOnly(value) {
		return false
	}
	return value[0] != '0'
}

func imapNumberAtomRFC3501(value string) bool {
	if !imapNumberAtomDigitsOnly(value) {
		return false
	}
	return value == "0" || value[0] != '0'
}

func imapSearchSizeResults(messages []MessageSummary, uidMode bool, criterion string, size int64) []uint32 {
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesSizeSearch(summary, criterion, size) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesSizeSearch(summary MessageSummary, criterion string, size int64) bool {
	switch criterion {
	case "LARGER":
		return summary.Size > size
	case "SMALLER":
		return summary.Size < size
	default:
		return false
	}
}

func imapSearchTextResults(messages []MessageSummary, uidMode bool, criterion string, query string) []uint32 {
	query = strings.ToLower(query)
	results := make([]uint32, 0, len(messages))
	for i, summary := range messages {
		if !imapMessageMatchesTextSearch(summary, criterion, query) {
			continue
		}
		if uidMode {
			results = append(results, uint32(summary.UID))
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		results = append(results, sequenceNumber)
	}
	return results
}

func imapMessageMatchesTextSearch(summary MessageSummary, criterion string, query string) bool {
	switch criterion {
	case "SUBJECT":
		return strings.Contains(strings.ToLower(summary.Envelope.Subject), query)
	case "FROM":
		return imapAddressListContains(summary.Envelope.From, query)
	case "TO":
		return imapAddressListContains(summary.Envelope.To, query)
	case "CC":
		return imapAddressListContains(summary.Envelope.Cc, query)
	case "BCC":
		return imapAddressListContains(summary.Envelope.Bcc, query)
	default:
		return false
	}
}

func (s *Server) imapMessageMatchesBodySearch(ctx context.Context, state *imapConnState, summary MessageSummary, criterion string, query string) (bool, error) {
	if s == nil || state == nil || state.session == nil || summary.UID == 0 {
		return false, nil
	}
	message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UID:       summary.UID,
	})
	if err != nil {
		return false, err
	}
	if message.Body == nil {
		return false, nil
	}
	defer message.Body.Close()
	literal, err := readIMAPSearchLiteral(message.Body, strings.EqualFold(criterion, "BODY"))
	if err != nil {
		return false, err
	}
	return strings.Contains(strings.ToLower(string(literal)), query), nil
}

func (s *Server) imapMessageMatchesHeaderSearch(ctx context.Context, state *imapConnState, summary MessageSummary, fieldName string, query string) (bool, error) {
	if s == nil || state == nil || state.session == nil || strings.TrimSpace(fieldName) == "" || summary.UID == 0 {
		return false, nil
	}
	message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UID:       summary.UID,
	})
	if err != nil {
		return false, err
	}
	if message.Body == nil {
		return false, nil
	}
	defer message.Body.Close()
	header, err := readIMAPSearchHeader(message.Body)
	if err != nil {
		return false, err
	}
	fieldLiteral := filterIMAPHeaderFields(header, []string{fieldName}, false)
	if strings.TrimSpace(string(fieldLiteral)) == "" {
		return false, nil
	}
	return strings.Contains(strings.ToLower(string(fieldLiteral)), query), nil
}

func readIMAPSearchHeader(reader io.Reader) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		data = data[:maxIMAPSearchLiteralBytes]
	}
	if end := imapHeaderEnd(data); end >= 0 {
		return data[:end], nil
	}
	return data, nil
}

func readIMAPSearchLiteral(reader io.Reader, bodyOnly bool) ([]byte, error) {
	if reader == nil {
		return nil, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		data = data[:maxIMAPSearchLiteralBytes]
	}
	if !bodyOnly {
		return data, nil
	}
	if end := imapHeaderEnd(data); end >= 0 {
		return data[end:], nil
	}
	return nil, nil
}

func imapAddressListContains(addresses []Address, query string) bool {
	for _, address := range addresses {
		if strings.Contains(strings.ToLower(address.Name), query) ||
			strings.Contains(strings.ToLower(address.Mailbox), query) ||
			strings.Contains(strings.ToLower(address.Host), query) {
			return true
		}
	}
	return false
}

func imapSearchResultSuffix(results []uint32, highestModSeq uint64, includeModSeq bool) string {
	if len(results) == 0 {
		return ""
	}
	parts := make([]string, 0, len(results))
	for _, result := range results {
		parts = append(parts, strconv.FormatUint(uint64(result), 10))
	}
	suffix := " " + strings.Join(parts, " ")
	if includeModSeq {
		suffix += " (MODSEQ " + strconv.FormatUint(highestModSeq, 10) + ")"
	}
	return suffix
}

func imapSearchResultValues(results []imapSearchMatch) []uint32 {
	values := make([]uint32, 0, len(results))
	for _, result := range results {
		values = append(values, result.value)
	}
	return values
}

func imapESearchResponse(tag string, results []imapSearchMatch, uidMode bool, options []string, highestModSeq uint64, includeModSeq bool) string {
	parts := []string{"* ESEARCH", "(TAG " + imapQuotedString(tag) + ")"}
	if uidMode {
		parts = append(parts, "UID")
	}
	values := imapSearchResultValues(results)
	for _, option := range options {
		switch option {
		case "MIN":
			if len(values) > 0 {
				parts = append(parts, "MIN", strconv.FormatUint(uint64(imapMinSearchResult(values)), 10))
			}
		case "MAX":
			if len(values) > 0 {
				parts = append(parts, "MAX", strconv.FormatUint(uint64(imapMaxSearchResult(values)), 10))
			}
		case "ALL":
			if len(values) > 0 {
				parts = append(parts, "ALL", imapSearchSequenceSet(values))
			}
		case "COUNT":
			parts = append(parts, "COUNT", strconv.FormatUint(uint64(len(values)), 10))
		}
	}
	if includeModSeq {
		if modSeq := imapESearchModSeq(results, options, highestModSeq); modSeq > 0 {
			parts = append(parts, "MODSEQ", strconv.FormatUint(modSeq, 10))
		}
	}
	return strings.Join(parts, " ")
}

func imapMinSearchResult(values []uint32) uint32 {
	minValue := values[0]
	for _, value := range values[1:] {
		if value < minValue {
			minValue = value
		}
	}
	return minValue
}

func imapMaxSearchResult(values []uint32) uint32 {
	maxValue := values[0]
	for _, value := range values[1:] {
		if value > maxValue {
			maxValue = value
		}
	}
	return maxValue
}

func imapSearchSequenceSet(values []uint32) string {
	if len(values) == 0 {
		return ""
	}
	compact := append([]uint32(nil), values...)
	sort.Slice(compact, func(i, j int) bool {
		return compact[i] < compact[j]
	})
	parts := make([]string, 0, len(compact))
	start := compact[0]
	prev := compact[0]
	for _, value := range compact[1:] {
		if value == prev {
			continue
		}
		if value == prev+1 {
			prev = value
			continue
		}
		parts = append(parts, imapSearchSequenceRange(start, prev))
		start = value
		prev = value
	}
	parts = append(parts, imapSearchSequenceRange(start, prev))
	return strings.Join(parts, ",")
}

func imapSearchSequenceRange(start uint32, end uint32) string {
	if start == end {
		return strconv.FormatUint(uint64(start), 10)
	}
	return strconv.FormatUint(uint64(start), 10) + ":" + strconv.FormatUint(uint64(end), 10)
}

func imapESearchModSeq(results []imapSearchMatch, options []string, highestModSeq uint64) uint64 {
	if len(results) == 0 {
		return 0
	}
	if len(options) == 1 && options[0] == "MIN" {
		return imapSearchResultModSeq(results, imapMinSearchResult(imapSearchResultValues(results)))
	}
	if len(options) == 1 && options[0] == "MAX" {
		return imapSearchResultModSeq(results, imapMaxSearchResult(imapSearchResultValues(results)))
	}
	if len(options) == 2 && imapSearchReturnHas(options, "MIN") && imapSearchReturnHas(options, "MAX") {
		minModSeq := imapSearchResultModSeq(results, imapMinSearchResult(imapSearchResultValues(results)))
		maxModSeq := imapSearchResultModSeq(results, imapMaxSearchResult(imapSearchResultValues(results)))
		if maxModSeq > minModSeq {
			return maxModSeq
		}
		return minModSeq
	}
	return highestModSeq
}

func imapSearchReturnHas(options []string, want string) bool {
	for _, option := range options {
		if option == want {
			return true
		}
	}
	return false
}

func imapSearchResultModSeq(results []imapSearchMatch, value uint32) uint64 {
	for _, result := range results {
		if result.value == value {
			return result.modSeq
		}
	}
	return 0
}

func imapSavedSearchResults(results []imapSearchMatch, options []string) []imapSearchSavedMessage {
	if len(results) == 0 {
		return nil
	}
	values := imapSearchResultValues(results)
	saveAll := len(options) == 0 || imapSearchReturnHas(options, "ALL") || imapSearchReturnHas(options, "COUNT")
	if saveAll {
		return imapSavedSearchMatches(results)
	}
	saved := make([]imapSearchSavedMessage, 0, 2)
	if imapSearchReturnHas(options, "MIN") {
		if result, ok := imapSearchResultForValue(results, imapMinSearchResult(values)); ok {
			saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
		}
	}
	if imapSearchReturnHas(options, "MAX") {
		if result, ok := imapSearchResultForValue(results, imapMaxSearchResult(values)); ok && !imapSavedSearchContains(saved, result) {
			saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
		}
	}
	return saved
}

func imapSavedSearchMatches(results []imapSearchMatch) []imapSearchSavedMessage {
	saved := make([]imapSearchSavedMessage, 0, len(results))
	for _, result := range results {
		saved = append(saved, imapSearchSavedMessage{uid: result.uid, sequenceNumber: result.sequenceNumber})
	}
	return saved
}

func imapSearchResultForValue(results []imapSearchMatch, value uint32) (imapSearchMatch, bool) {
	for _, result := range results {
		if result.value == value {
			return result, true
		}
	}
	return imapSearchMatch{}, false
}

func imapSavedSearchContains(saved []imapSearchSavedMessage, result imapSearchMatch) bool {
	for _, existing := range saved {
		if existing.uid == result.uid && existing.sequenceNumber == result.sequenceNumber {
			return true
		}
	}
	return false
}

