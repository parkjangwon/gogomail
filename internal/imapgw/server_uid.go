package imapgw

import (
	"bufio"
	"strings"
)

func (s *Server) handleUIDLine(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD UID requires subcommand\r\n")
		return false, err
	}
	if !imapAtomValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	subcommand := strings.ToUpper(fields[2])
	if subcommand == "ESEARCH" {
		_, err := writer.WriteString(tag + " BAD ESEARCH command requires MULTISEARCH capability\r\n")
		return false, err
	}
	if !imapUIDSubcommandKnown(subcommand) {
		_, err := writer.WriteString(tag + " BAD unsupported UID command\r\n")
		return false, err
	}
	if handled, done, err := s.validateUIDSubcommandSyntax(writer, tag, fields, subcommand); handled {
		return done, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if state.selectedMailbox == "" {
		_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
		return false, err
	}
	if err := s.drainMailboxEvents(writer, state); err != nil {
		return false, err
	}
	switch subcommand {
	case "FETCH":
		return s.handleUIDFetch(writer, tag, fields, state)
	case "SEARCH":
		return s.handleSearch(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "SORT":
		return s.handleSort(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "THREAD":
		return s.handleThread(writer, tag, append([]string{fields[0], fields[2]}, fields[3:]...), state, true)
	case "STORE":
		return s.handleUIDStore(writer, tag, fields, state)
	case "EXPUNGE":
		return s.handleUIDExpunge(writer, tag, fields, state)
	case "COPY":
		return s.handleUIDCopy(writer, tag, fields, state)
	case "MOVE":
		return s.handleUIDMove(writer, tag, fields, state)
	default:
		_, err := writer.WriteString(tag + " BAD unsupported UID command\r\n")
		return false, err
	}
}

func (s *Server) validateUIDSubcommandSyntax(writer *bufio.Writer, tag string, fields []string, subcommand string) (bool, bool, error) {
	switch subcommand {
	case "FETCH":
		if len(fields) < 5 {
			_, err := writer.WriteString(tag + " BAD UID FETCH requires UID set and data items\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
			return true, false, err
		}
		if message, ok := imapFetchDataItemsSyntaxError(fields[4:]); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "SEARCH":
		if len(fields) < 4 {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		_, searchFields, ok := imapSearchReturnOptions(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD SEARCH return options are unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		if !imapSearchFieldsContainCriteria(searchFields) {
			_, err := writer.WriteString(tag + " BAD SEARCH requires criteria\r\n")
			return true, false, err
		}
		if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "SORT":
		_, sortFields, ok := imapSearchSaveReturnOption(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD SORT return options are unsupported\r\n")
			return true, false, err
		}
		if len(sortFields) < 3 {
			_, err := writer.WriteString(tag + " BAD SORT requires sort criteria, charset, and search criteria\r\n")
			return true, false, err
		}
		_, searchFields, charsetOK, ok := imapSortCommandArguments(sortFields)
		if !ok {
			_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD SORT requires search criteria\r\n")
			return true, false, err
		}
		if charsetOK {
			if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
				_, err := writer.WriteString(tag + " BAD SORT " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
				return true, false, err
			}
		}
	case "THREAD":
		_, threadFields, ok := imapSearchSaveReturnOption(fields[3:])
		if !ok {
			_, err := writer.WriteString(tag + " BAD THREAD return options are unsupported\r\n")
			return true, false, err
		}
		if len(threadFields) < 3 {
			_, err := writer.WriteString(tag + " BAD THREAD requires algorithm, charset, and search criteria\r\n")
			return true, false, err
		}
		algorithm, searchFields, charsetOK, ok := imapThreadCommandArguments(threadFields)
		if !ok {
			_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
			return true, false, err
		}
		if !imapThreadAlgorithmIsSupported(algorithm) {
			_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
			return true, false, err
		}
		if len(searchFields) == 0 {
			_, err := writer.WriteString(tag + " BAD THREAD requires search criteria\r\n")
			return true, false, err
		}
		if charsetOK {
			if message, ok := imapSearchCriteriaSyntaxError(searchFields); ok {
				_, err := writer.WriteString(tag + " BAD THREAD " + strings.TrimPrefix(message, "SEARCH ") + "\r\n")
				return true, false, err
			}
		}
	case "STORE":
		if len(fields) < 6 {
			_, err := writer.WriteString(tag + " BAD UID STORE requires UID, mode, and flags\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
			return true, false, err
		}
		if message, ok := imapStoreArgumentsSyntaxError("UID STORE", fields[4:]); ok {
			_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
			return true, false, err
		}
	case "EXPUNGE":
		if len(fields) != 4 {
			_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires UID set\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
			return true, false, err
		}
	case "COPY":
		if len(fields) != 5 {
			_, err := writer.WriteString(tag + " BAD UID COPY requires UID set and destination mailbox\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
			return true, false, err
		}
		if _, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[4]); !valid {
			_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n")
			return true, false, err
		} else if !nonEmpty {
			_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is empty\r\n")
			return true, false, err
		}
	case "MOVE":
		if len(fields) != 5 {
			_, err := writer.WriteString(tag + " BAD UID MOVE requires UID set and destination mailbox\r\n")
			return true, false, err
		}
		if !imapUIDSetSyntaxValid(fields[3]) {
			_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
			return true, false, err
		}
		if _, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[4]); !valid {
			_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n")
			return true, false, err
		} else if !nonEmpty {
			_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is empty\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapUIDSubcommandKnown(subcommand string) bool {
	switch subcommand {
	case "FETCH", "SEARCH", "SORT", "THREAD", "STORE", "EXPUNGE", "COPY", "MOVE":
		return true
	default:
		return false
	}
}

func (s *Server) handleUIDFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 5 {
		_, err := writer.WriteString(tag + " BAD UID FETCH requires UID set and data items\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(state.ctx, state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID FETCH failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
		return false, err
	}
	return s.writeFetchResponses(writer, tag, fields[4:], state, uids, "UID FETCH")
}

func (s *Server) handleUIDCopy(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 5 {
		_, err := writer.WriteString(tag + " BAD UID COPY requires UID set and destination mailbox\r\n")
		return false, err
	}
	destMailbox, destValid, destNonEmpty := imapDecodeRequiredMailboxName(fields[4])
	if !destValid {
		_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !destNonEmpty {
		_, err := writer.WriteString(tag + " BAD UID COPY destination mailbox name is empty\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(state.ctx, state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID COPY failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
		return false, err
	}
	return s.writeCopyResponse(writer, tag, state, uids, MailboxID(destMailbox), "UID COPY")
}

func (s *Server) handleUIDMove(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 5 {
		_, err := writer.WriteString(tag + " BAD UID MOVE requires UID set and destination mailbox\r\n")
		return false, err
	}
	destMailbox, destValid, destNonEmpty := imapDecodeRequiredMailboxName(fields[4])
	if !destValid {
		_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !destNonEmpty {
		_, err := writer.WriteString(tag + " BAD UID MOVE destination mailbox name is empty\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(state.ctx, state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID MOVE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	return s.writeMoveResponse(writer, tag, state, uids, MailboxID(destMailbox), "UID MOVE")
}

func (s *Server) handleUIDExpunge(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires UID set\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(state.ctx, state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID EXPUNGE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	return s.writeExpungeResponses(writer, tag, state, uids, "UID EXPUNGE")
}

func (s *Server) handleUIDStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 6 {
		_, err := writer.WriteString(tag + " BAD UID STORE requires UID, mode, and flags\r\n")
		return false, err
	}
	uids, ok, err := s.uidsForUIDSet(state.ctx, state, fields[3])
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO UID STORE failed\r\n")
		return false, writeErr
	}
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
		return false, err
	}
	unchangedSince, unchangedSinceSet, storeFields, ok := imapStoreUnchangedSince(fields[4:])
	if !ok || len(storeFields) < 2 {
		_, err := writer.WriteString(tag + " BAD UID STORE UNCHANGEDSINCE modifier is invalid\r\n")
		return false, err
	}
	if imapStoreUnchangedSincePresent(fields[4:]) && !state.selectedSupportsPersistentModSeq() {
		return s.rejectSelectedNoModSeq(writer, tag, state, "UID STORE")
	}
	mode, silent, ok := imapStoreMode(storeFields[0])
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE mode is unsupported\r\n")
		return false, err
	}
	flags, requestedFlags, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD UID STORE flags are unsupported\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	if !imapPermanentFlagsAllow(state.permanentFlags, requestedFlags, mode) {
		_, err := writer.WriteString(tag + " NO UID STORE flags are not permitted\r\n")
		return false, err
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, unchangedSince, unchangedSinceSet, "UID STORE")
}

