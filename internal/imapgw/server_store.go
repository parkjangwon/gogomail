package imapgw

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
)

func (s *Server) handleStore(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 5 {
		_, err := writer.WriteString(tag + " BAD STORE requires sequence set, mode, and flags\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
		return false, err
	}
	if message, ok := imapStoreArgumentsSyntaxError("STORE", fields[3:]); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
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
	sequenceNumbers, ok := parseIMAPSequenceSetForState(fields[2], state.selectedMessages, state)
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE requires a valid message sequence set\r\n")
		return false, err
	}
	unchangedSince, unchangedSinceSet, storeFields, ok := imapStoreUnchangedSince(fields[3:])
	if !ok || len(storeFields) < 2 {
		_, err := writer.WriteString(tag + " BAD STORE UNCHANGEDSINCE modifier is invalid\r\n")
		return false, err
	}
	if imapStoreUnchangedSincePresent(fields[3:]) && !state.selectedSupportsPersistentModSeq() {
		return s.rejectSelectedNoModSeq(writer, tag, state, "STORE")
	}
	mode, silent, ok := imapStoreMode(storeFields[0])
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE mode is unsupported\r\n")
		return false, err
	}
	flags, requestedFlags, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " "))
	if !ok {
		_, err := writer.WriteString(tag + " BAD STORE flags are unsupported\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	if !imapPermanentFlagsAllow(state.permanentFlags, requestedFlags, mode) {
		_, err := writer.WriteString(tag + " NO STORE flags are not permitted\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(state.ctx, state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO STORE failed\r\n")
		return false, writeErr
	}
	return s.writeStoreResponses(writer, tag, state, uids, flags, mode, silent, unchangedSince, unchangedSinceSet, "STORE")
}

func imapStoreArgumentsSyntaxError(command string, fields []string) (string, bool) {
	_, _, storeFields, ok := imapStoreUnchangedSince(fields)
	if !ok || len(storeFields) < 2 {
		return command + " UNCHANGEDSINCE modifier is invalid", true
	}
	if _, _, ok := imapStoreMode(storeFields[0]); !ok {
		return command + " mode is unsupported", true
	}
	if _, _, ok := imapStoreFlagsWithNames(strings.Join(storeFields[1:], " ")); !ok {
		return command + " flags are unsupported", true
	}
	return "", false
}

func (s *Server) writeStoreResponses(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, flags MessageFlags, mode StoreFlagsMode, silent bool, unchangedSince uint64, unchangedSinceSet bool, completionCommand string) (bool, error) {
	if unchangedSinceSet {
		state.condstoreAware = true
	}
	if len(uids) == 0 || ((mode == StoreFlagsAdd || mode == StoreFlagsRemove) && imapMessageFlagsEmpty(flags)) {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	summaries, err := s.options.Backend.StoreFlags(state.ctx, StoreFlagsRequest{
		UserID:            state.session.UserID,
		MailboxID:         state.selectedMailbox,
		UIDs:              uids,
		Flags:             flags,
		Mode:              mode,
		UnchangedSince:    unchangedSince,
		UnchangedSinceSet: unchangedSinceSet,
	})
	if err != nil {
		var modified *StoreModifiedError
		if errors.As(err, &modified) {
			successfulSummaries := imapStoreSuccessfulSummaries(summaries, modified)
			state.observeHighestModSeq(imapHighestSummaryModSeq(successfulSummaries))
			if err := s.writeStoreFetchResponses(writer, tag, successfulSummaries, state.condstoreAware, completionCommand); err != nil {
				return false, err
			}
			modifiedSet, err := s.storeModifiedSetResponse(state.ctx, state, modified.UIDs, completionCommand == "UID STORE")
			if err != nil {
				_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
				return false, writeErr
			}
			_, writeErr := writer.WriteString(tag + " OK [MODIFIED " + modifiedSet + "] " + completionCommand + " conditional store completed\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	state.observeHighestModSeq(imapHighestSummaryModSeq(summaries))
	if silent && !unchangedSinceSet {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	if err := s.writeStoreFetchResponses(writer, tag, summaries, state.condstoreAware, completionCommand); err != nil {
		return false, err
	}
	_, err = writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) writeStoreFetchResponses(writer *bufio.Writer, tag string, summaries []MessageSummary, includeModSeq bool, completionCommand string) error {
	for _, summary := range summaries {
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return err
		}
		attributes := []string{
			"UID " + strconv.FormatUint(uint64(summary.UID), 10),
			"FLAGS " + imapFlagList(summary.Flags.IMAPFlags()),
		}
		if includeModSeq {
			attributes = append(attributes, "MODSEQ ("+strconv.FormatUint(summary.ModSeq, 10)+")")
		}
		if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), ")"); err != nil {
			return err
		}
	}
	return nil
}

func imapStoreSuccessfulSummaries(summaries []MessageSummary, modified *StoreModifiedError) []MessageSummary {
	if modified == nil {
		return summaries
	}
	source := modified.Summaries
	if len(source) == 0 {
		source = summaries
	}
	if len(source) == 0 || len(modified.UIDs) == 0 {
		return source
	}
	modifiedUIDs := make(map[UID]struct{}, len(modified.UIDs))
	for _, uid := range modified.UIDs {
		modifiedUIDs[uid] = struct{}{}
	}
	successful := make([]MessageSummary, 0, len(source))
	for _, summary := range source {
		if _, stale := modifiedUIDs[summary.UID]; stale {
			continue
		}
		successful = append(successful, summary)
	}
	return successful
}

func (s *Server) storeModifiedSetResponse(ctx context.Context, state *imapConnState, uids []UID, uidMode bool) (string, error) {
	if uidMode {
		return imapUIDSetResponse(uids), nil
	}
	sequenceNumbers := make([]UID, 0, len(uids))
	for _, uid := range uids {
		message, err := s.options.Backend.FetchMessage(ctx, FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		})
		if err != nil {
			return "", err
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		sequenceNumber, ok := imapSequenceNumber(message.Summary)
		if !ok {
			return "", fmt.Errorf("imap modified sequence number is unavailable")
		}
		sequenceNumbers = append(sequenceNumbers, UID(sequenceNumber))
	}
	return imapUIDSetResponse(sequenceNumbers), nil
}

func imapStoreUnchangedSince(fields []string) (uint64, bool, []string, bool) {
	if len(fields) == 0 {
		return 0, false, fields, true
	}
	first := strings.ToUpper(fields[0])
	if !strings.Contains(first, "UNCHANGEDSINCE") && !strings.HasPrefix(first, "(") {
		return 0, false, fields, true
	}
	if first != "(UNCHANGEDSINCE" || len(fields) < 2 {
		return 0, false, nil, false
	}
	valueToken := fields[1]
	if !strings.HasSuffix(valueToken, ")") || strings.HasSuffix(valueToken, "))") {
		return 0, false, nil, false
	}
	value := strings.TrimSuffix(valueToken, ")")
	threshold, ok := parseIMAPModSeqValzer(value)
	if !ok {
		return 0, false, nil, false
	}
	return threshold, true, fields[2:], true
}

func imapStoreUnchangedSincePresent(fields []string) bool {
	if len(fields) == 0 {
		return false
	}
	return strings.EqualFold(fields[0], "(UNCHANGEDSINCE")
}

func imapSequenceNumber(summary MessageSummary) (uint32, bool) {
	if summary.SequenceNumber == 0 {
		return 0, false
	}
	return summary.SequenceNumber, true
}

func imapStoreMode(value string) (StoreFlagsMode, bool, bool) {
	if strings.TrimSpace(value) != value {
		return "", false, false
	}
	switch strings.ToUpper(value) {
	case "FLAGS":
		return StoreFlagsReplace, false, true
	case "FLAGS.SILENT":
		return StoreFlagsReplace, true, true
	case "+FLAGS":
		return StoreFlagsAdd, false, true
	case "+FLAGS.SILENT":
		return StoreFlagsAdd, true, true
	case "-FLAGS":
		return StoreFlagsRemove, false, true
	case "-FLAGS.SILENT":
		return StoreFlagsRemove, true, true
	default:
		return "", false, false
	}
}

func imapStoreFlags(value string) (MessageFlags, bool) {
	flags, _, ok := imapStoreFlagsWithNames(value)
	return flags, ok
}

func imapStoreFlagsWithNames(value string) (MessageFlags, []string, bool) {
	var flags MessageFlags
	if strings.TrimSpace(value) != value {
		return MessageFlags{}, nil, false
	}
	if value != "()" && (!strings.HasPrefix(value, "(") || !strings.HasSuffix(value, ")")) {
		return MessageFlags{}, nil, false
	}
	inner := strings.TrimSuffix(strings.TrimPrefix(value, "("), ")")
	tokens, ok := imapFlagListTokens(inner)
	if !ok {
		return MessageFlags{}, nil, false
	}
	if len(tokens) == 0 {
		return flags, nil, value == "()"
	}
	names := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, raw := range tokens {
		name := CanonicalIMAPFlag(raw)
		switch name {
		case FlagSeen:
			flags.Read = true
		case FlagFlagged:
			flags.Starred = true
		case FlagAnswered:
			flags.Answered = true
		case FlagForwarded:
			flags.Forwarded = true
		case FlagDraft:
			flags.Draft = true
		case FlagDeleted:
			flags.Deleted = true
		default:
			if !IMAPKeywordFlagValid(name) {
				return MessageFlags{}, nil, false
			}
			flags.Keywords = append(flags.Keywords, name)
		}
		if _, ok := seen[name]; ok {
			return MessageFlags{}, nil, false
		}
		seen[name] = struct{}{}
		names = append(names, name)
	}
	return flags, names, true
}

func imapFlagListTokens(inner string) ([]string, bool) {
	if inner == "" {
		return nil, true
	}
	if strings.TrimSpace(inner) != inner {
		return nil, false
	}
	tokens := strings.Split(inner, " ")
	for _, token := range tokens {
		if token == "" || strings.TrimSpace(token) != token {
			return nil, false
		}
	}
	return tokens, true
}

func imapPermanentFlagSet(flags []string) map[string]struct{} {
	permitted := make(map[string]struct{}, len(flags))
	for _, name := range imapCanonicalPermanentFlags(flags) {
		permitted[name] = struct{}{}
	}
	return permitted
}

func imapCanonicalPermanentFlags(flags []string) []string {
	if len(flags) == 0 {
		return nil
	}
	present := make(map[string]struct{}, len(flags))
	custom := make([]string, 0)
	for _, raw := range flags {
		switch name := CanonicalIMAPFlag(raw); name {
		case FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDraft, FlagDeleted:
			present[name] = struct{}{}
		default:
			if !IMAPKeywordFlagValid(name) {
				continue
			}
			if _, ok := present[name]; ok {
				continue
			}
			present[name] = struct{}{}
			custom = append(custom, name)
		}
	}
	if len(present) == 0 {
		return nil
	}
	ordered := []string{FlagSeen, FlagFlagged, FlagAnswered, FlagForwarded, FlagDraft, FlagDeleted}
	canonical := make([]string, 0, len(present))
	for _, name := range ordered {
		if _, ok := present[name]; ok {
			canonical = append(canonical, name)
		}
	}
	canonical = append(canonical, custom...)
	return canonical
}

func imapPermanentFlagsAllow(permitted map[string]struct{}, requested []string, mode StoreFlagsMode) bool {
	if len(requested) == 0 {
		return mode != StoreFlagsReplace || len(permitted) > 0
	}
	for _, flag := range requested {
		if _, ok := permitted[flag]; !ok {
			return false
		}
	}
	return true
}

func imapMessageFlagsEmpty(flags MessageFlags) bool {
	return !flags.Read &&
		!flags.Starred &&
		!flags.Answered &&
		!flags.Forwarded &&
		!flags.Draft &&
		!flags.Deleted &&
		len(flags.Keywords) == 0 &&
		strings.TrimSpace(flags.Status) == ""
}

