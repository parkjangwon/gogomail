package imapgw

import (
	"context"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"
)

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
