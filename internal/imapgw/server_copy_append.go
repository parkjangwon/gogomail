package imapgw

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleCopy(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD COPY requires sequence set and destination mailbox\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
		return false, err
	}
	destMailbox, destValid, destNonEmpty := imapDecodeRequiredMailboxName(fields[3])
	if !destValid {
		_, err := writer.WriteString(tag + " BAD COPY destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !destNonEmpty {
		_, err := writer.WriteString(tag + " BAD COPY destination mailbox name is empty\r\n")
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
		_, err := writer.WriteString(tag + " BAD COPY requires a valid message sequence set\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(state.ctx, state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO COPY failed\r\n")
		return false, writeErr
	}
	return s.writeCopyResponse(writer, tag, state, uids, MailboxID(destMailbox), "COPY")
}

func (s *Server) handleMove(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD MOVE requires sequence set and destination mailbox\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
		return false, err
	}
	destMailbox, destValid, destNonEmpty := imapDecodeRequiredMailboxName(fields[3])
	if !destValid {
		_, err := writer.WriteString(tag + " BAD MOVE destination mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !destNonEmpty {
		_, err := writer.WriteString(tag + " BAD MOVE destination mailbox name is empty\r\n")
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
		_, err := writer.WriteString(tag + " BAD MOVE requires a valid message sequence set\r\n")
		return false, err
	}
	if state.readOnly {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(state.ctx, state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO MOVE failed\r\n")
		return false, writeErr
	}
	return s.writeMoveResponse(writer, tag, state, uids, MailboxID(destMailbox), "MOVE")
}

func (s *Server) handleAppend(writer *bufio.Writer, tag string, fields []string, literals []string, state *imapConnState) (bool, error) {
	if len(literals) == 0 {
		_, err := writer.WriteString(tag + " BAD APPEND requires mailbox and literal\r\n")
		return false, err
	}
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD APPEND requires mailbox and literal\r\n")
		return false, err
	}
	flags, internalDate, ok := imapAppendOptions(fields[3 : len(fields)-1])
	if !ok {
		_, err := writer.WriteString(tag + " BAD APPEND options are unsupported\r\n")
		return false, err
	}
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD APPEND mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD APPEND mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] APPEND mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO APPEND failed\r\n")
		return false, writeErr
	}
	if state.readOnly && mailbox.ID == state.selectedMailbox {
		_, err := writer.WriteString(tag + " NO mailbox is read-only\r\n")
		return false, err
	}
	body := literals[len(literals)-1]
	result, err := s.options.Backend.AppendMessage(state.ctx, AppendMessageRequest{
		UserID:       state.session.UserID,
		MailboxID:    mailbox.ID,
		Flags:        flags,
		InternalDate: internalDate,
		Size:         int64(len(body)),
		Body:         strings.NewReader(body),
	})
	if err != nil {
		if errors.Is(err, ErrUnsupportedAppend) {
			_, writeErr := writer.WriteString(tag + " NO APPEND is not supported\r\n")
			return false, writeErr
		}
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] APPEND mailbox does not exist\r\n")
			return false, writeErr
		}
		if errors.Is(err, ErrOverQuota) {
			_, writeErr := writer.WriteString(tag + " NO [OVERQUOTA] APPEND would exceed quota\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO APPEND failed\r\n")
		return false, writeErr
	}
	summary := result.Summary
	if summary.MailboxID == state.selectedMailbox {
		state.selectedMessages = imapAppendExistsCount(state.selectedMessages, summary)
		state.observeHighestModSeq(summary.ModSeq)
		if err := writeIMAPUintLine(writer, "* ", uint64(state.selectedMessages), " EXISTS\r\n"); err != nil {
			return false, err
		}
	}
	responseCode := ""
	if !result.UIDNotSticky && result.UIDValidity != 0 && summary.UID != 0 {
		responseCode = imapAppendUIDResponseCode(result.UIDValidity, summary.UID)
	}
	_, err = writer.WriteString(tag + " OK" + responseCode + " APPEND completed\r\n")
	return false, err
}

func imapAppendExistsCount(current uint32, summary MessageSummary) uint32 {
	if summary.SequenceNumber > current {
		return summary.SequenceNumber
	}
	return current + 1
}

func imapAppendOptions(fields []string) (MessageFlags, time.Time, bool) {
	var flags MessageFlags
	var internalDate time.Time
	if len(fields) == 0 {
		return flags, internalDate, true
	}
	i := 0
	if strings.HasPrefix(fields[i], "(") {
		var flagParts []string
		for i < len(fields) {
			flagParts = append(flagParts, fields[i])
			done := strings.HasSuffix(fields[i], ")")
			i++
			if done {
				break
			}
		}
		if len(flagParts) == 0 || !strings.HasSuffix(flagParts[len(flagParts)-1], ")") {
			return MessageFlags{}, time.Time{}, false
		}
		parsed, ok := imapStoreFlags(strings.Join(flagParts, " "))
		if !ok {
			return MessageFlags{}, time.Time{}, false
		}
		flags = parsed
	}
	if i < len(fields) {
		parsed, ok := parseIMAPAppendDate(fields[i])
		if !ok {
			return MessageFlags{}, time.Time{}, false
		}
		internalDate = parsed
		i++
	}
	if i != len(fields) {
		return MessageFlags{}, time.Time{}, false
	}
	return flags, internalDate, true
}

func parseIMAPAppendDate(value string) (time.Time, bool) {
	if len(value) < len("01-Jan-2006 00:00:00 +0000") || value[2] != '-' {
		return time.Time{}, false
	}
	if value[0] == ' ' {
		if value[1] < '1' || value[1] > '9' {
			return time.Time{}, false
		}
	} else if value[0] < '0' || value[0] > '9' || value[1] < '0' || value[1] > '9' {
		return time.Time{}, false
	}
	var ok bool
	value, ok = imapCanonicalDateMonth(value)
	if !ok {
		return time.Time{}, false
	}
	for _, layout := range []string{
		"_2-Jan-2006 15:04:05 -0700",
		"02-Jan-2006 15:04:05 -0700",
	} {
		parsed, err := time.Parse(layout, value)
		if err == nil {
			return parsed, true
		}
	}
	return time.Time{}, false
}

func imapCanonicalDateMonth(value string) (string, bool) {
	firstDash := strings.IndexByte(value, '-')
	if firstDash < 1 || len(value) < firstDash+5 {
		return "", false
	}
	monthStart := firstDash + 1
	monthEnd := monthStart + 3
	if value[monthEnd] != '-' {
		return "", false
	}
	month, ok := imapCanonicalMonth(value[monthStart:monthEnd])
	if !ok {
		return "", false
	}
	if value[monthStart:monthEnd] == month {
		return value, true
	}
	return value[:monthStart] + month + value[monthEnd:], true
}

func imapCanonicalMonth(value string) (string, bool) {
	if len(value) != 3 {
		return "", false
	}
	switch [3]byte{imapASCIILower(value[0]), imapASCIILower(value[1]), imapASCIILower(value[2])} {
	case [3]byte{'j', 'a', 'n'}:
		return "Jan", true
	case [3]byte{'f', 'e', 'b'}:
		return "Feb", true
	case [3]byte{'m', 'a', 'r'}:
		return "Mar", true
	case [3]byte{'a', 'p', 'r'}:
		return "Apr", true
	case [3]byte{'m', 'a', 'y'}:
		return "May", true
	case [3]byte{'j', 'u', 'n'}:
		return "Jun", true
	case [3]byte{'j', 'u', 'l'}:
		return "Jul", true
	case [3]byte{'a', 'u', 'g'}:
		return "Aug", true
	case [3]byte{'s', 'e', 'p'}:
		return "Sep", true
	case [3]byte{'o', 'c', 't'}:
		return "Oct", true
	case [3]byte{'n', 'o', 'v'}:
		return "Nov", true
	case [3]byte{'d', 'e', 'c'}:
		return "Dec", true
	default:
		return "", false
	}
}

func imapASCIILower(value byte) byte {
	if value >= 'A' && value <= 'Z' {
		return value + ('a' - 'A')
	}
	return value
}

func (s *Server) handleClose(writer *bufio.Writer, tag string, state *imapConnState) (bool, error) {
	if !state.readOnly {
		if _, err := s.options.Backend.Expunge(state.ctx, ExpungeRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
		}); err != nil {
			_, writeErr := writer.WriteString(tag + " NO CLOSE failed\r\n")
			return false, writeErr
		}
	}
	state.deselectMailbox()
	_, err := writer.WriteString(tag + " OK CLOSE completed\r\n")
	return false, err
}

func (s *Server) writeCopyResponse(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, destMailboxID MailboxID, completionCommand string) (bool, error) {
	destMailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, destMailboxID)
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	if len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	results, err := s.options.Backend.CopyMessages(state.ctx, CopyMessagesRequest{
		UserID:          state.session.UserID,
		SourceMailboxID: state.selectedMailbox,
		DestMailboxID:   destMailbox.ID,
		UIDs:            uids,
	})
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	summaries := imapCopyDestinationSummaries(results)
	if destMailbox.ID == state.selectedMailbox && len(summaries) > 0 {
		state.selectedMessages = imapSummariesExistsCount(state.selectedMessages, summaries)
		state.observeHighestModSeq(imapHighestSummaryModSeq(summaries))
		if err := writeIMAPUintLine(writer, "* ", uint64(state.selectedMessages), " EXISTS\r\n"); err != nil {
			return false, err
		}
	}
	if copyUID := imapCopyUIDResponse(destMailbox, results); copyUID != "" {
		_, err = writer.WriteString(tag + " OK [" + copyUID + "] " + completionCommand + " completed\r\n")
		return false, err
	}
	_, err = writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) writeMoveResponse(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, destMailboxID MailboxID, completionCommand string) (bool, error) {
	if len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	destMailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, destMailboxID)
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	summaries, err := s.options.Backend.MoveMessages(state.ctx, MoveMessagesRequest{
		UserID:          state.session.UserID,
		SourceMailboxID: state.selectedMailbox,
		DestMailboxID:   destMailbox.ID,
		UIDs:            uids,
	})
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(tag + " NO [TRYCREATE] " + completionCommand + " destination mailbox does not exist\r\n")
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	copyUID := imapMoveCopyUIDResponse(destMailbox, summaries)
	if destMailbox.ID == state.selectedMailbox && len(summaries) > 0 {
		state.selectedMessages = imapSummariesExistsCount(state.selectedMessages, imapMoveDestinationSummaries(summaries))
		if err := writeIMAPUintLine(writer, "* ", uint64(state.selectedMessages), " EXISTS\r\n"); err != nil {
			return false, err
		}
	}
	if highestModSeq := imapMoveHighestModSeq(summaries); highestModSeq > 0 {
		state.observeHighestModSeq(highestModSeq)
		if err := writeIMAPUintLine(writer, "* OK [HIGHESTMODSEQ ", highestModSeq, "] "+completionCommand+" source mod-sequence\r\n"); err != nil {
			return false, err
		}
	}
	if copyUID != "" {
		if _, err := writer.WriteString("* OK [" + copyUID + "] " + completionCommand + " copied UIDs\r\n"); err != nil {
			return false, err
		}
	}
	return s.writeMovedExpungeResponses(writer, tag, state, imapMoveSourceSummaries(summaries), completionCommand, "")
}

func (s *Server) writeExpungeResponses(writer *bufio.Writer, tag string, state *imapConnState, uids []UID, completionCommand string) (bool, error) {
	if uids != nil && len(uids) == 0 {
		_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
		return false, err
	}
	summaries, err := s.options.Backend.Expunge(state.ctx, ExpungeRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      uids,
	})
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
		return false, writeErr
	}
	return s.writeMovedExpungeResponses(writer, tag, state, summaries, completionCommand, "")
}

func (s *Server) writeMovedExpungeResponses(writer *bufio.Writer, tag string, state *imapConnState, summaries []MessageSummary, completionCommand string, responseCode string) (bool, error) {
	sort.Slice(summaries, func(i, j int) bool {
		return summaries[i].SequenceNumber < summaries[j].SequenceNumber
	})
	for i, summary := range summaries {
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return false, err
		}
		adjusted := sequenceNumber - uint32(i)
		if adjusted == 0 {
			adjusted = 1
		}
		if err := writeIMAPUintLine(writer, "* ", uint64(adjusted), " EXPUNGE\r\n"); err != nil {
			return false, err
		}
	}
	state.removeExpungedFromSavedSearch(summaries)
	if uint32(len(summaries)) >= state.selectedMessages {
		state.selectedMessages = 0
	} else {
		state.selectedMessages -= uint32(len(summaries))
	}
	if responseCode != "" {
		_, err := writer.WriteString(tag + " OK [" + responseCode + "] " + completionCommand + " completed\r\n")
		return false, err
	}
	_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (state *imapConnState) removeExpungedFromSavedSearch(summaries []MessageSummary) {
	if state == nil || len(state.savedSearch) == 0 || len(summaries) == 0 {
		return
	}
	expunged := make([]uint32, 0, len(summaries))
	for _, summary := range summaries {
		if summary.SequenceNumber > 0 {
			expunged = append(expunged, summary.SequenceNumber)
		}
	}
	sort.Slice(expunged, func(i, j int) bool {
		return expunged[i] < expunged[j]
	})
	for i, sequenceNumber := range expunged {
		adjusted := sequenceNumber - uint32(i)
		if adjusted == 0 {
			adjusted = 1
		}
		next := state.savedSearch[:0]
		for _, saved := range state.savedSearch {
			switch {
			case saved.sequenceNumber == adjusted:
				continue
			case saved.sequenceNumber > adjusted:
				saved.sequenceNumber--
			}
			next = append(next, saved)
		}
		state.savedSearch = next
	}
	if len(state.savedSearch) == 0 {
		state.savedSearch = nil
	}
}

func (state *imapConnState) observeHighestModSeq(modSeq uint64) {
	if state == nil || modSeq == 0 || modSeq <= state.selectedHighestModSeq {
		return
	}
	state.selectedHighestModSeq = modSeq
}

func (state *imapConnState) selectedSupportsPersistentModSeq() bool {
	return state != nil && state.selectedMailbox != "" && !state.selectedNoModSeq
}

func (s *Server) rejectSelectedNoModSeq(writer *bufio.Writer, tag string, state *imapConnState, command string) (bool, error) {
	if state != nil && !state.condstoreAware {
		state.condstoreAware = true
		if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
			return false, err
		}
	}
	_, err := writer.WriteString(tag + " BAD " + command + " requires persistent mod-sequences\r\n")
	return false, err
}

func (s *Server) uidsForSequenceNumbers(ctx context.Context, state *imapConnState, sequenceNumbers []uint32) ([]UID, error) {
	messages, err := s.options.Backend.ListMessages(ctx, ListMessagesRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		Limit:     int(state.selectedMessages),
	})
	if err != nil {
		return nil, err
	}
	bySequence := make(map[uint32]UID, len(messages))
	for i, summary := range messages {
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		if summary.UID != 0 {
			bySequence[sequenceNumber] = summary.UID
		}
	}
	uids := make([]UID, 0, len(sequenceNumbers))
	for _, sequenceNumber := range sequenceNumbers {
		uid, ok := bySequence[sequenceNumber]
		if !ok {
			return nil, fmt.Errorf("sequence number %d not found", sequenceNumber)
		}
		uids = append(uids, uid)
	}
	return uids, nil
}

func imapCopyUIDResponse(destMailbox Mailbox, results []CopyMessageResult) string {
	if destMailbox.UIDNotSticky || destMailbox.UIDValidity == 0 || len(results) == 0 {
		return ""
	}
	sourceUIDs := make([]UID, 0, len(results))
	destUIDs := make([]UID, 0, len(results))
	for _, result := range results {
		if result.SourceUID == 0 || result.Destination.UID == 0 {
			return ""
		}
		sourceUIDs = append(sourceUIDs, result.SourceUID)
		destUIDs = append(destUIDs, result.Destination.UID)
	}
	return imapCopyUIDResponseCode(destMailbox.UIDValidity, sourceUIDs, destUIDs)
}

func imapMoveCopyUIDResponse(destMailbox Mailbox, results []MoveMessageResult) string {
	if destMailbox.UIDNotSticky || destMailbox.UIDValidity == 0 || len(results) == 0 {
		return ""
	}
	sourceUIDs := make([]UID, 0, len(results))
	destUIDs := make([]UID, 0, len(results))
	for _, result := range results {
		if result.Source.UID == 0 || result.Destination.UID == 0 {
			return ""
		}
		sourceUIDs = append(sourceUIDs, result.Source.UID)
		destUIDs = append(destUIDs, result.Destination.UID)
	}
	return imapCopyUIDResponseCode(destMailbox.UIDValidity, sourceUIDs, destUIDs)
}

func imapMoveSourceSummaries(results []MoveMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Source)
	}
	return summaries
}

func imapMoveDestinationSummaries(results []MoveMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Destination)
	}
	return summaries
}

func imapCopyDestinationSummaries(results []CopyMessageResult) []MessageSummary {
	summaries := make([]MessageSummary, 0, len(results))
	for _, result := range results {
		summaries = append(summaries, result.Destination)
	}
	return summaries
}

func imapHighestSummaryModSeq(summaries []MessageSummary) uint64 {
	var highest uint64
	for _, summary := range summaries {
		if summary.ModSeq > highest {
			highest = summary.ModSeq
		}
	}
	return highest
}

func imapSummariesExistsCount(current uint32, summaries []MessageSummary) uint32 {
	maxSequence := current
	for _, summary := range summaries {
		if summary.SequenceNumber > maxSequence {
			maxSequence = summary.SequenceNumber
		}
	}
	if maxSequence > current {
		return maxSequence
	}
	return current + uint32(len(summaries))
}

func imapMoveHighestModSeq(results []MoveMessageResult) uint64 {
	var highest uint64
	for _, result := range results {
		if result.SourceHighestModSeq > highest {
			highest = result.SourceHighestModSeq
		}
	}
	return highest
}

func imapUIDSetResponse(uids []UID) string {
	if len(uids) == 0 {
		return ""
	}
	buf := make([]byte, 0, len(uids)*4)
	for i := 0; i < len(uids); {
		if len(buf) > 0 {
			buf = append(buf, ',')
		}
		start := uids[i]
		end := start
		j := i + 1
		for j < len(uids) && uids[j] == end+1 {
			end = uids[j]
			j++
		}
		if end > start {
			buf = strconv.AppendUint(buf, uint64(start), 10)
			buf = append(buf, ':')
			buf = strconv.AppendUint(buf, uint64(end), 10)
		} else {
			buf = strconv.AppendUint(buf, uint64(start), 10)
		}
		i = j
	}
	return string(buf)
}

func imapAppendUIDResponseCode(uidValidity uint32, uid UID) string {
	if uidValidity == 0 || uid == 0 {
		return ""
	}
	buf := make([]byte, 0, 24)
	buf = append(buf, " [APPENDUID "...)
	buf = strconv.AppendUint(buf, uint64(uidValidity), 10)
	buf = append(buf, ' ')
	buf = strconv.AppendUint(buf, uint64(uid), 10)
	buf = append(buf, ']')
	return string(buf)
}

func imapCopyUIDResponseCode(uidValidity uint32, sourceUIDs, destUIDs []UID) string {
	if uidValidity == 0 || len(sourceUIDs) == 0 || len(destUIDs) == 0 {
		return ""
	}
	buf := make([]byte, 0, 32+len(sourceUIDs)*4+len(destUIDs)*4)
	buf = append(buf, "COPYUID "...)
	buf = strconv.AppendUint(buf, uint64(uidValidity), 10)
	buf = append(buf, ' ')
	buf = append(buf, imapUIDSetResponse(sourceUIDs)...)
	buf = append(buf, ' ')
	buf = append(buf, imapUIDSetResponse(destUIDs)...)
	return string(buf)
}

