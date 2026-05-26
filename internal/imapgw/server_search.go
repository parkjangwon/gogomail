package imapgw

import (
	"bufio"
	"strings"
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

