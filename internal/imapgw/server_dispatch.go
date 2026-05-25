package imapgw

import (
	"bufio"
	"strings"
)

func (s *Server) handleLineWithLiteral(writer *bufio.Writer, line string, literals []string, state *imapConnState) (bool, error) {
	trimmedLine := strings.TrimRight(line, "\r\n")
	if state.pendingIdleTag != "" {
		return s.handleIdleDone(writer, trimmedLine, state)
	}
	if state.pendingAuthTag != "" {
		return s.handleAuthenticatePlainResponse(writer, trimmedLine, state)
	}
	fields, parseErr := parseIMAPFieldsWithLiteral(trimmedLine, literals)
	if parseErr != nil {
		_, err := writer.WriteString(imapMalformedCommandResponse(trimmedLine))
		return false, err
	}
	if len(fields) < 2 {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	if !imapRawFieldIsAtom(trimmedLine, 0) {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	tag := fields[0]
	if !imapTagValid(tag) {
		_, err := writer.WriteString("* BAD malformed command\r\n")
		return false, err
	}
	if !imapRawFieldIsAtom(trimmedLine, 1) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	if !imapAtomValid(fields[1]) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	command := strings.ToUpper(fields[1])
	if command == "UID" && len(fields) >= 3 && !imapRawFieldIsAtom(trimmedLine, 2) {
		_, err := writer.WriteString(tag + " BAD malformed command\r\n")
		return false, err
	}
	if handled, done, err := imapRejectNonAtomAuthenticateArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomSequenceSetArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomStoreControlArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectStringParenthesizedControlListArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomFetchDataItemArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if handled, done, err := imapRejectNonAtomEnableCapabilityArgument(writer, tag, trimmedLine, fields, command); handled {
		return done, err
	}
	if imapCommandShouldDrainSelectedEvents(command) {
		if err := s.drainMailboxEvents(writer, state); err != nil {
			return false, err
		}
	}
	switch command {
	case "CAPABILITY":
		return s.handleCapability(writer, tag, fields, state)
	case "ENABLE":
		return s.handleEnable(writer, tag, fields, state)
	case "NOOP":
		return s.handleNoop(writer, tag, fields, state)
	case "ID":
		return s.handleID(writer, tag, trimmedLine, literals, state)
	case "STARTTLS":
		return s.handleStartTLS(writer, tag, fields, state)
	case "NAMESPACE":
		return s.handleNamespace(writer, tag, fields, state)
	case "LOGIN":
		return s.handleLogin(writer, tag, fields, state)
	case "AUTHENTICATE":
		return s.handleAuthenticate(writer, tag, fields, state)
	case "SELECT", "EXAMINE":
		return s.handleSelect(writer, tag, command, fields, state)
	case "LIST":
		return s.handleList(writer, tag, fields, state, false)
	case "LSUB":
		return s.handleList(writer, tag, fields, state, true)
	case "CREATE":
		return s.handleCreate(writer, tag, fields, state)
	case "DELETE":
		return s.handleDeleteMailbox(writer, tag, fields, state)
	case "RENAME":
		return s.handleRenameMailbox(writer, tag, fields, state)
	case "SUBSCRIBE", "UNSUBSCRIBE":
		return s.handleSubscriptionCommand(writer, tag, fields, state, command)
	case "STATUS":
		return s.handleStatus(writer, tag, fields, state)
	case "UID":
		return s.handleUIDLine(writer, tag, fields, state)
	case "FETCH":
		return s.handleFetch(writer, tag, fields, state)
	case "SEARCH":
		return s.handleSearch(writer, tag, fields, state, false)
	case "ESEARCH":
		_, err := writer.WriteString(tag + " BAD ESEARCH command requires MULTISEARCH capability\r\n")
		return false, err
	case "SORT":
		return s.handleSort(writer, tag, fields, state, false)
	case "THREAD":
		return s.handleThread(writer, tag, fields, state, false)
	case "STORE":
		return s.handleStore(writer, tag, fields, state)
	case "COPY":
		return s.handleCopy(writer, tag, fields, state)
	case "CHECK":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD CHECK does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		_, err := writer.WriteString(tag + " OK CHECK completed\r\n")
		return false, err
	case "IDLE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD IDLE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.pendingIdleTag = tag
		_, err := writer.WriteString("+ idling\r\n")
		return false, err
	case "CLOSE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD CLOSE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		return s.handleClose(writer, tag, state)
	case "UNSELECT":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD UNSELECT does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		state.deselectMailbox()
		_, err := writer.WriteString(tag + " OK UNSELECT completed\r\n")
		return false, err
	case "EXPUNGE":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD EXPUNGE does not accept arguments\r\n")
			return false, err
		}
		if state.session == nil {
			_, err := writer.WriteString(tag + " NO authentication required\r\n")
			return false, err
		}
		if state.selectedMailbox == "" {
			_, err := writer.WriteString(tag + " NO mailbox must be selected\r\n")
			return false, err
		}
		if state.readOnly {
			_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
			return false, err
		}
		return s.writeExpungeResponses(writer, tag, state, nil, "EXPUNGE")
	case "MOVE":
		return s.handleMove(writer, tag, fields, state)
	case "APPEND":
		return s.handleAppend(writer, tag, fields, literals, state)
	case "LOGOUT":
		if len(fields) != 2 {
			_, err := writer.WriteString(tag + " BAD LOGOUT does not accept arguments\r\n")
			return false, err
		}
		if _, err := writer.WriteString("* BYE gogomail IMAP4rev1 server logging out\r\n"); err != nil {
			return false, err
		}
		_, err := writer.WriteString(tag + " OK LOGOUT completed\r\n")
		return true, err
	default:
		_, err := writer.WriteString(tag + " BAD unsupported command\r\n")
		return false, err
	}
}

func imapRejectNonAtomSequenceSetArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	switch command {
	case "SEARCH":
		if imapSearchHasNonAtomSequenceSetArgument(line, fields, 2, 2) {
			_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
			return true, false, err
		}
	case "SORT":
		if criteriaStart := imapSortSearchCriteriaStart(fields, 2); criteriaStart >= 0 {
			if !imapRawFieldIsAtom(line, criteriaStart-1) {
				_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
				return true, false, err
			}
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
				_, err := writer.WriteString(tag + " BAD SORT criteria are unsupported\r\n")
				return true, false, err
			}
		}
	case "THREAD":
		if criteriaStart := imapThreadSearchCriteriaStart(fields, 2); criteriaStart >= 0 {
			if !imapRawFieldIsAtom(line, criteriaStart-1) {
				_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
				return true, false, err
			}
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
				_, err := writer.WriteString(tag + " BAD THREAD criteria are unsupported\r\n")
				return true, false, err
			}
		}
	case "FETCH":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "STORE":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "COPY":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "MOVE":
		if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
			_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
			return true, false, err
		}
	case "UID":
		if len(fields) < 4 || !imapAtomValid(fields[2]) {
			return false, false, nil
		}
		switch strings.ToUpper(fields[2]) {
		case "SEARCH":
			if imapSearchHasNonAtomSequenceSetArgument(line, fields, 3, 3) {
				_, err := writer.WriteString(tag + " BAD SEARCH criteria are unsupported\r\n")
				return true, false, err
			}
		case "SORT":
			if criteriaStart := imapSortSearchCriteriaStart(fields, 3); criteriaStart >= 0 {
				if !imapRawFieldIsAtom(line, criteriaStart-1) {
					_, err := writer.WriteString(tag + " BAD SORT arguments are unsupported\r\n")
					return true, false, err
				}
				if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
					_, err := writer.WriteString(tag + " BAD SORT criteria are unsupported\r\n")
					return true, false, err
				}
			}
		case "THREAD":
			if criteriaStart := imapThreadSearchCriteriaStart(fields, 3); criteriaStart >= 0 {
				if !imapRawFieldIsAtom(line, criteriaStart-1) {
					_, err := writer.WriteString(tag + " BAD THREAD arguments are unsupported\r\n")
					return true, false, err
				}
				if imapSearchHasNonAtomSequenceSetArgument(line, fields, criteriaStart, criteriaStart) {
					_, err := writer.WriteString(tag + " BAD THREAD criteria are unsupported\r\n")
					return true, false, err
				}
			}
		case "FETCH":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID FETCH requires a positive UID set\r\n")
				return true, false, err
			}
		case "STORE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID STORE requires a positive UID set\r\n")
				return true, false, err
			}
		case "EXPUNGE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID EXPUNGE requires a positive UID set\r\n")
				return true, false, err
			}
		case "COPY":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID COPY requires a positive UID set\r\n")
				return true, false, err
			}
		case "MOVE":
			if !imapRawFieldIsAtom(line, 3) {
				_, err := writer.WriteString(tag + " BAD UID MOVE requires a positive UID set\r\n")
				return true, false, err
			}
		}
	}
	return false, false, nil
}

func imapRejectNonAtomAuthenticateArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	if command != "AUTHENTICATE" {
		return false, false, nil
	}
	if len(fields) >= 3 && !imapRawFieldIsAtom(line, 2) {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE mechanism is malformed\r\n")
		return true, false, err
	}
	if len(fields) >= 4 && !imapRawFieldIsAtom(line, 3) {
		_, err := writer.WriteString(tag + " BAD AUTHENTICATE PLAIN response is malformed\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRejectNonAtomEnableCapabilityArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	if command != "ENABLE" || len(fields) < 3 {
		return false, false, nil
	}
	for i := 2; i < len(fields); i++ {
		if !imapRawFieldIsAtom(line, i) {
			_, err := writer.WriteString(tag + " BAD ENABLE capability is malformed\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapRejectStringParenthesizedControlListArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	switch command {
	case "STORE":
		return imapRejectStringStoreFlagListArgument(writer, tag, line, fields, 3, "STORE")
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "STORE") {
			return imapRejectStringStoreFlagListArgument(writer, tag, line, fields, 4, "UID STORE")
		}
		if len(fields) >= 3 {
			switch strings.ToUpper(fields[2]) {
			case "SEARCH", "SORT", "THREAD":
				subcommand := strings.ToUpper(fields[2])
				if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 3, subcommand); handled {
					return true, done, err
				}
				if subcommand == "SORT" {
					return imapRejectStringSortCriterionListArgument(writer, tag, line, fields, 3)
				}
				if subcommand == "THREAD" {
					return imapRejectStringThreadAlgorithmArgument(writer, tag, line, fields, 3)
				}
			}
		}
	case "APPEND":
		if len(fields) >= 5 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD APPEND options are unsupported\r\n")
			return true, false, err
		}
	case "STATUS":
		if len(fields) >= 4 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD STATUS requires parenthesized item list\r\n")
			return true, false, err
		}
	case "SEARCH":
		return imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "SEARCH")
	case "SORT":
		if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "SORT"); handled {
			return true, done, err
		}
		return imapRejectStringSortCriterionListArgument(writer, tag, line, fields, 2)
	case "THREAD":
		if handled, done, err := imapRejectStringSearchReturnControlArgument(writer, tag, line, fields, 2, "THREAD"); handled {
			return true, done, err
		}
		return imapRejectStringThreadAlgorithmArgument(writer, tag, line, fields, 2)
	case "LIST":
		if len(fields) >= 3 && strings.HasPrefix(fields[2], "(") && imapRawFieldIsStringLike(line, 2) {
			_, err := writer.WriteString(tag + " BAD LIST requires reference and mailbox pattern atoms\r\n")
			return true, false, err
		}
		for i := 4; i+1 < len(fields); i++ {
			if strings.EqualFold(fields[i], "RETURN") {
				if imapRawFieldIsStringLike(line, i) {
					_, err := writer.WriteString(tag + " BAD LIST requires return options atom\r\n")
					return true, false, err
				}
				if strings.HasPrefix(fields[i+1], "(") && imapRawFieldIsStringLike(line, i+1) {
					_, err := writer.WriteString(tag + " BAD LIST requires parenthesized return options\r\n")
					return true, false, err
				}
			}
		}
	case "SELECT", "EXAMINE":
		if len(fields) >= 4 && strings.HasPrefix(fields[3], "(") && imapRawFieldIsStringLike(line, 3) {
			_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
			return true, false, err
		}
	}
	return false, false, nil
}

func imapRejectStringSortCriterionListArgument(writer *bufio.Writer, tag string, line string, fields []string, argumentStart int) (bool, bool, error) {
	criterionRawIndex := imapSortCriterionRawIndex(fields, argumentStart)
	if criterionRawIndex < 0 || !strings.HasPrefix(fields[criterionRawIndex], "(") || !imapRawFieldIsStringLike(line, criterionRawIndex) {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD SORT requires parenthesized sort criteria\r\n")
	return true, false, err
}

func imapSortCriterionRawIndex(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+2 {
			return -1
		}
		return argumentStart + 2
	}
	return argumentStart
}

func imapRejectStringThreadAlgorithmArgument(writer *bufio.Writer, tag string, line string, fields []string, argumentStart int) (bool, bool, error) {
	algorithmRawIndex := imapThreadAlgorithmRawIndex(fields, argumentStart)
	if algorithmRawIndex < 0 || !imapRawFieldIsStringLike(line, algorithmRawIndex) {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD THREAD algorithm is unsupported\r\n")
	return true, false, err
}

func imapThreadAlgorithmRawIndex(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+2 {
			return -1
		}
		return argumentStart + 2
	}
	return argumentStart
}

func imapRejectStringSearchReturnControlArgument(writer *bufio.Writer, tag string, line string, fields []string, returnRawIndex int, commandName string) (bool, bool, error) {
	if len(fields) <= returnRawIndex || !strings.EqualFold(fields[returnRawIndex], "RETURN") {
		return false, false, nil
	}
	if imapRawFieldIsStringLike(line, returnRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " requires return options atom\r\n")
		return true, false, err
	}
	if len(fields) > returnRawIndex+1 && strings.HasPrefix(fields[returnRawIndex+1], "(") && imapRawFieldIsStringLike(line, returnRawIndex+1) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " requires parenthesized return options\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRejectStringStoreFlagListArgument(writer *bufio.Writer, tag string, line string, fields []string, storeStart int, commandName string) (bool, bool, error) {
	if len(fields) <= storeStart {
		return false, false, nil
	}
	modeRawIndex := storeStart
	if imapStoreUnchangedSincePresent(fields[storeStart:]) {
		modeRawIndex++
	}
	flagRawIndex := modeRawIndex + 1
	if imapRawFieldIsStringLike(line, flagRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " flags are unsupported\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapRawFieldIsStringLike(line string, fieldIndex int) bool {
	kind, ok := imapRawFieldKind(line, fieldIndex)
	return ok && (kind == imapRawFieldQuoted || kind == imapRawFieldLiteral)
}

func imapRejectNonAtomFetchDataItemArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	dataStart := -1
	switch command {
	case "FETCH":
		dataStart = 3
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "FETCH") {
			dataStart = 4
		}
	}
	if dataStart < 0 || len(fields) <= dataStart {
		return false, false, nil
	}
	kind, ok := imapRawFieldKind(line, dataStart)
	if !ok || (kind != imapRawFieldQuoted && kind != imapRawFieldLiteral) {
		return false, false, nil
	}
	if _, hasSyntaxError := imapFetchDataItemsSyntaxError(fields[dataStart:]); hasSyntaxError {
		return false, false, nil
	}
	_, err := writer.WriteString(tag + " BAD FETCH data item is unsupported\r\n")
	return true, false, err
}

func imapRejectNonAtomStoreControlArgument(writer *bufio.Writer, tag string, line string, fields []string, command string) (bool, bool, error) {
	commandName := ""
	storeStart := -1
	switch command {
	case "STORE":
		commandName = "STORE"
		storeStart = 3
	case "UID":
		if len(fields) >= 3 && strings.EqualFold(fields[2], "STORE") {
			commandName = "UID STORE"
			storeStart = 4
		}
	}
	if storeStart < 0 || len(fields) <= storeStart {
		return false, false, nil
	}
	modeRawIndex := storeStart
	if imapStoreUnchangedSincePresent(fields[storeStart:]) {
		kind, ok := imapRawFieldKind(line, storeStart)
		if !ok || kind == imapRawFieldQuoted || kind == imapRawFieldLiteral {
			_, err := writer.WriteString(tag + " BAD " + commandName + " UNCHANGEDSINCE modifier is invalid\r\n")
			return true, false, err
		}
		modeRawIndex++
	}
	if !imapRawFieldIsAtom(line, modeRawIndex) {
		_, err := writer.WriteString(tag + " BAD " + commandName + " mode is unsupported\r\n")
		return true, false, err
	}
	return false, false, nil
}

func imapSortSearchCriteriaStart(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+4 {
			return -1
		}
		return argumentStart + 4
	}
	if len(fields) <= argumentStart+2 {
		return -1
	}
	return argumentStart + 2
}

func imapThreadSearchCriteriaStart(fields []string, argumentStart int) int {
	if len(fields) <= argumentStart {
		return -1
	}
	if strings.EqualFold(fields[argumentStart], "RETURN") {
		if len(fields) <= argumentStart+4 {
			return -1
		}
		return argumentStart + 4
	}
	if len(fields) <= argumentStart+2 {
		return -1
	}
	return argumentStart + 2
}

func imapSearchHasNonAtomSequenceSetArgument(line string, fields []string, criteriaStart int, rawCriteriaStart int) bool {
	for i := criteriaStart; i < len(fields); i++ {
		field := fields[i]
		if !imapSearchFieldRequiresAtomSet(fields, i, criteriaStart) {
			continue
		}
		if imapSearchCriterionLooksLikeSequenceSet(field) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomNumeric(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomDate(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomCharset(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
		if imapSearchFieldRequiresAtomKeyword(fields, i, criteriaStart) && !imapRawFieldIsAtom(line, rawCriteriaStart+i-criteriaStart) {
			return true
		}
	}
	return false
}

func imapSearchFieldRequiresAtomSet(fields []string, index int, criteriaStart int) bool {
	if index > criteriaStart {
		switch strings.ToUpper(fields[index-1]) {
		case "FROM", "TO", "CC", "BCC", "SUBJECT", "BODY", "TEXT":
			return false
		}
	}
	if index > criteriaStart+1 && strings.EqualFold(fields[index-2], "HEADER") {
		return false
	}
	return true
}

func imapSearchFieldRequiresAtomNumeric(fields []string, index int, criteriaStart int) bool {
	if index > criteriaStart {
		switch strings.ToUpper(fields[index-1]) {
		case "LARGER", "SMALLER":
			return true
		case "MODSEQ":
			return imapDecimalToken(fields[index])
		}
	}
	if index > criteriaStart+1 && strings.EqualFold(fields[index-2], "MODSEQ") {
		return true
	}
	if index > criteriaStart+2 && strings.EqualFold(fields[index-3], "MODSEQ") {
		return true
	}
	return false
}

func imapSearchFieldRequiresAtomDate(fields []string, index int, criteriaStart int) bool {
	if index <= criteriaStart {
		return false
	}
	switch strings.ToUpper(fields[index-1]) {
	case "SINCE", "BEFORE", "ON", "SENTSINCE", "SENTBEFORE", "SENTON":
		return true
	default:
		return false
	}
}

func imapSearchFieldRequiresAtomCharset(fields []string, index int, criteriaStart int) bool {
	return index > criteriaStart && strings.EqualFold(fields[index-1], "CHARSET")
}

func imapSearchFieldRequiresAtomKeyword(fields []string, index int, criteriaStart int) bool {
	if index <= criteriaStart {
		return false
	}
	switch strings.ToUpper(fields[index-1]) {
	case "KEYWORD", "UNKEYWORD":
		return true
	default:
		return false
	}
}

func imapDecimalToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

