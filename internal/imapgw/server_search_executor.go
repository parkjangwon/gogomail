package imapgw

import (
	"bufio"
	"context"
	"fmt"
	"mime"
	"sort"
	"strconv"
	"strings"
	"time"

	"golang.org/x/text/collate"
	"golang.org/x/text/language"
)

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

