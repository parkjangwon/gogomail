package imapgw

import (
	"bufio"
	"errors"
	"strconv"
	"strings"
)

func (s *Server) handleSelect(writer *bufio.Writer, tag string, command string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 3 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
		return false, err
	}
	condstore, ok := imapSelectCondstore(fields[3:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom and optional CONDSTORE parameter\r\n")
		return false, err
	}
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	mailboxState, err := s.options.Backend.SelectMailbox(state.ctx, SelectMailboxRequest{
		UserID:    state.session.UserID,
		MailboxID: MailboxID(mailboxName),
		ReadOnly:  command == "EXAMINE",
	})
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, command))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	events, cancel, err := s.options.Backend.Subscribe(state.ctx, state.session.UserID, mailboxState.ID)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	subscriptionInstalled := false
	defer func() {
		if !subscriptionInstalled && cancel != nil {
			cancel()
		}
	}()
	permanentFlags := imapCanonicalPermanentFlags(mailboxState.PermanentFlags)
	if _, err := writer.WriteString("* FLAGS " + imapFlagList(permanentFlags) + "\r\n"); err != nil {
		return false, err
	}
	if err := writeIMAPUintLine(writer, "* ", uint64(mailboxState.Messages), " EXISTS\r\n"); err != nil {
		return false, err
	}
	if err := writeIMAPUintLine(writer, "* ", uint64(mailboxState.Recent), " RECENT\r\n"); err != nil {
		return false, err
	}
	if unseenSequence := s.firstUnseenSequenceNumber(state.ctx, state.session.UserID, mailboxState); unseenSequence > 0 {
		if err := writeIMAPUnseenLine(writer, unseenSequence); err != nil {
			return false, err
		}
	}
	if err := writeIMAPUintLine(writer, "* OK [UIDVALIDITY ", uint64(mailboxState.UIDValidity), "] UIDs valid\r\n"); err != nil {
		return false, err
	}
	if err := writeIMAPUintLine(writer, "* OK [UIDNEXT ", uint64(mailboxState.UIDNext), "] Predicted next UID\r\n"); err != nil {
		return false, err
	}
	if mailboxState.UIDNotSticky {
		if _, err := writer.WriteString("* OK [UIDNOTSTICKY] UIDs are not sticky\r\n"); err != nil {
			return false, err
		}
	}
	if mailboxState.HighestModSeq > 0 {
		if err := writeIMAPUintLine(writer, "* OK [HIGHESTMODSEQ ", mailboxState.HighestModSeq, "] Highest mod-sequence\r\n"); err != nil {
			return false, err
		}
	} else if condstore || state.condstoreAware {
		if _, err := writer.WriteString("* OK [NOMODSEQ] No persistent mod-sequences\r\n"); err != nil {
			return false, err
		}
	}
	state.deselectMailbox()
	state.selectedMailbox = mailboxState.ID
	state.selectedMessages = mailboxState.Messages
	state.selectedHighestModSeq = mailboxState.HighestModSeq
	state.selectedNoModSeq = mailboxState.HighestModSeq == 0 && (condstore || state.condstoreAware)
	state.readOnly = command == "EXAMINE"
	if state.readOnly {
		state.permanentFlags = nil
	} else {
		state.permanentFlags = imapPermanentFlagSet(permanentFlags)
	}
	state.savedSearch = nil
	if condstore {
		state.condstoreAware = true
	}
	state.events = events
	state.cancelEvents = cancel
	subscriptionInstalled = true
	if state.readOnly {
		if _, err := writer.WriteString("* OK [PERMANENTFLAGS ()] No permanent flags permitted\r\n"); err != nil {
			return false, err
		}
		_, err = writer.WriteString(tag + " OK [READ-ONLY] EXAMINE completed\r\n")
		return false, err
	}
	if _, err := writer.WriteString("* OK [PERMANENTFLAGS " + imapFlagList(permanentFlags) + "] Permanent flags\r\n"); err != nil {
		return false, err
	}
	_, err = writer.WriteString(tag + " OK [READ-WRITE] SELECT completed\r\n")
	return false, err
}

func (s *Server) handleStatus(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD STATUS requires mailbox and status item atoms\r\n")
		return false, err
	}
	if !imapStatusItemListIsParenthesized(fields[3:]) {
		_, err := writer.WriteString(tag + " BAD STATUS requires parenthesized item list\r\n")
		return false, err
	}
	if imapStatusItemListIsEmpty(fields[3:]) {
		_, err := writer.WriteString(tag + " BAD STATUS requires status data items\r\n")
		return false, err
	}
	statusItems, statusErr, ok := imapStatusItems(fields[3:])
	if !ok {
		_, err := writer.WriteString(tag + " BAD " + statusErr + "\r\n")
		return false, err
	}
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD STATUS mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD STATUS mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapStatusRequestsItem(statusItems, "HIGHESTMODSEQ") {
		state.condstoreAware = true
	}
	mailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "STATUS"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO STATUS failed\r\n")
		return false, writeErr
	}
	statusName := imapEncodeMailboxName(imapMailboxWireName(imapMailboxDisplayName(mailbox)))
	if err := writeIMAPStatusLine(writer, imapQuotedString(statusName), imapStatusData(mailbox, statusItems)); err != nil {
		return false, err
	}
	_, err = writer.WriteString(tag + " OK STATUS completed\r\n")
	return false, err
}

func (s *Server) handleCreate(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD CREATE requires mailbox name\r\n")
		return false, err
	}
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD CREATE mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD CREATE mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(mailboxName) {
		_, err := writer.WriteString(tag + " NO CREATE cannot create INBOX\r\n")
		return false, err
	}
	if _, err := s.options.Backend.CreateMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName)); err != nil {
		_, writeErr := writer.WriteString(tag + " NO CREATE failed\r\n")
		return false, writeErr
	}
	_, err := writer.WriteString(tag + " OK CREATE completed\r\n")
	return false, err
}

func (s *Server) handleDeleteMailbox(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD DELETE requires mailbox name\r\n")
		return false, err
	}
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD DELETE mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD DELETE mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(mailboxName) {
		_, err := writer.WriteString(tag + " NO DELETE cannot delete INBOX\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "DELETE"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO DELETE failed\r\n")
		return false, writeErr
	}
	if err := s.options.Backend.DeleteMailbox(state.ctx, state.session.UserID, mailbox.ID); err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "DELETE"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO DELETE failed\r\n")
		return false, writeErr
	}
	if state.selectedMailbox == mailbox.ID {
		state.deselectMailbox()
	}
	_, err = writer.WriteString(tag + " OK DELETE completed\r\n")
	return false, err
}

func (s *Server) handleRenameMailbox(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) != 4 {
		_, err := writer.WriteString(tag + " BAD RENAME requires source and destination mailbox names\r\n")
		return false, err
	}
	sourceName, sourceValid, sourceNonEmpty := imapDecodeRequiredMailboxName(fields[2])
	destName, destValid, destNonEmpty := imapDecodeRequiredMailboxName(fields[3])
	if !sourceValid || !destValid {
		_, err := writer.WriteString(tag + " BAD RENAME mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !sourceNonEmpty || !destNonEmpty {
		_, err := writer.WriteString(tag + " BAD RENAME mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(sourceName) {
		_, err := writer.WriteString(tag + " NO RENAME INBOX special semantics are not supported\r\n")
		return false, err
	}
	if imapMailboxNameIsINBOX(destName) {
		_, err := writer.WriteString(tag + " NO RENAME cannot rename to INBOX\r\n")
		return false, err
	}
	mailbox, err := s.options.Backend.GetMailbox(state.ctx, state.session.UserID, MailboxID(sourceName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "RENAME"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO RENAME failed\r\n")
		return false, writeErr
	}
	renamed, err := s.options.Backend.RenameMailbox(state.ctx, state.session.UserID, mailbox.ID, MailboxID(destName))
	if err != nil {
		if errors.Is(err, ErrMailboxNotFound) {
			_, writeErr := writer.WriteString(imapMailboxNotFoundResponse(tag, "RENAME"))
			return false, writeErr
		}
		_, writeErr := writer.WriteString(tag + " NO RENAME failed\r\n")
		return false, writeErr
	}
	if state.selectedMailbox == mailbox.ID && renamed.ID != "" && renamed.ID != state.selectedMailbox {
		state.closeSubscription()
		state.selectedMailbox = renamed.ID
		state.selectedHighestModSeq = renamed.HighestModSeq
		state.selectedNoModSeq = renamed.HighestModSeq == 0 && state.condstoreAware
		if events, cancel, err := s.options.Backend.Subscribe(state.ctx, state.session.UserID, renamed.ID); err == nil {
			state.events = events
			state.cancelEvents = cancel
		}
	}
	_, err = writer.WriteString(tag + " OK RENAME completed\r\n")
	return false, err
}

func imapMailboxNameIsINBOX(name string) bool {
	return strings.EqualFold(name, "INBOX")
}

func imapMailboxNotFoundResponse(tag string, command string) string {
	return tag + " NO [NONEXISTENT] " + command + " mailbox does not exist\r\n"
}

func (s *Server) handleSubscriptionCommand(writer *bufio.Writer, tag string, fields []string, state *imapConnState, command string) (bool, error) {
	if len(fields) != 3 {
		_, err := writer.WriteString(tag + " BAD " + command + " requires a mailbox atom\r\n")
		return false, err
	}
	var err error
	mailboxName, valid, nonEmpty := imapDecodeRequiredMailboxName(fields[2])
	if !valid {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is not valid modified UTF-7\r\n")
		return false, err
	}
	if !nonEmpty {
		_, err := writer.WriteString(tag + " BAD " + command + " mailbox name is empty\r\n")
		return false, err
	}
	if state.session == nil {
		_, err := writer.WriteString(tag + " NO authentication required\r\n")
		return false, err
	}
	if command == "SUBSCRIBE" {
		_, err = s.options.Backend.SubscribeMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName))
	} else {
		err = s.options.Backend.UnsubscribeMailbox(state.ctx, state.session.UserID, MailboxID(mailboxName))
	}
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO " + command + " failed\r\n")
		return false, writeErr
	}
	_, err = writer.WriteString(tag + " OK " + command + " completed\r\n")
	return false, err
}

func writeIMAPStatusLine(writer *bufio.Writer, mailboxName string, statusData string) error {
	var buf [128]byte
	out := append(buf[:0], "* STATUS "...)
	out = append(out, mailboxName...)
	out = append(out, " ("...)
	out = append(out, statusData...)
	out = append(out, ")\r\n"...)
	_, err := writer.Write(out)
	return err
}

func writeIMAPUnseenLine(writer *bufio.Writer, unseenSequence uint32) error {
	var buf [128]byte
	out := append(buf[:0], "* OK [UNSEEN "...)
	out = strconv.AppendUint(out, uint64(unseenSequence), 10)
	out = append(out, "] Message "...)
	out = strconv.AppendUint(out, uint64(unseenSequence), 10)
	out = append(out, " is first unseen\r\n"...)
	_, err := writer.Write(out)
	return err
}

func imapMailboxDisplayName(mailbox Mailbox) string {
	if strings.TrimSpace(mailbox.FullPath) != "" {
		if value := strings.Trim(strings.TrimSpace(mailbox.FullPath), "/"); value != "" {
			return value
		}
	}
	if strings.TrimSpace(mailbox.Name) != "" {
		return mailbox.Name
	}
	return strings.TrimSpace(string(mailbox.ID))
}

func imapMailboxListAttributes(mailbox Mailbox, hasChildren bool) []string {
	attributes := []string{`\HasNoChildren`}
	if hasChildren {
		attributes[0] = `\HasChildren`
	}
	switch strings.ToLower(strings.TrimSpace(mailbox.SystemType)) {
	case "all":
		attributes = append(attributes, `\All`)
	case "archive":
		attributes = append(attributes, `\Archive`)
	case "drafts":
		attributes = append(attributes, `\Drafts`)
	case "flagged":
		attributes = append(attributes, `\Flagged`)
	case "junk", "spam":
		attributes = append(attributes, `\Junk`)
	case "sent":
		attributes = append(attributes, `\Sent`)
	case "trash":
		attributes = append(attributes, `\Trash`)
	}
	return attributes
}

func imapMailboxChildren(mailboxes []Mailbox) map[MailboxID]bool {
	children := make(map[MailboxID]bool)
	byWireName := make(map[string]MailboxID, len(mailboxes))
	for _, mailbox := range mailboxes {
		if mailbox.ID == "" {
			continue
		}
		wireName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		if wireName != "" {
			byWireName[strings.ToLower(wireName)] = mailbox.ID
		}
	}
	for _, mailbox := range mailboxes {
		if mailbox.ParentID != "" {
			children[mailbox.ParentID] = true
			continue
		}
		wireName := imapMailboxWireName(imapMailboxDisplayName(mailbox))
		parentName, ok := imapMailboxParentWireName(wireName)
		if !ok {
			continue
		}
		if parentID, ok := byWireName[parentName]; ok {
			children[parentID] = true
		}
	}
	return children
}

func imapMailboxParentWireName(wireName string) (string, bool) {
	wireName = strings.ToLower(strings.Trim(wireName, "/"))
	index := strings.LastIndex(wireName, "/")
	if index <= 0 {
		return "", false
	}
	return wireName[:index], true
}

func imapSelectCondstore(fields []string) (bool, bool) {
	if len(fields) == 0 {
		return false, true
	}
	inner, ok := imapStatusItemListInner(fields)
	if !ok {
		return false, false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return false, false
	}
	if len(tokens) != 1 || tokens[0] != "CONDSTORE" {
		return false, false
	}
	return true, true
}

func imapStatusItems(items []string) ([]string, string, bool) {
	inner, ok := imapStatusItemListInner(items)
	if !ok {
		return nil, "STATUS item is unsupported", false
	}
	tokens, ok := imapParenthesizedAtomListTokens(inner)
	if !ok {
		return nil, "STATUS item is unsupported", false
	}
	return imapStatusItemsFromTokens(tokens)
}

func imapStatusItemsFromTokens(tokens []string) ([]string, string, bool) {
	out := make([]string, 0, len(tokens))
	seen := make(map[string]struct{}, len(tokens))
	for _, token := range tokens {
		item := strings.ToUpper(token)
		switch item {
		case "MESSAGES", "RECENT", "UIDNEXT", "UIDVALIDITY", "UNSEEN", "HIGHESTMODSEQ", "SIZE":
			if _, ok := seen[item]; ok {
				return nil, "STATUS item is duplicated", false
			}
			seen[item] = struct{}{}
			out = append(out, item)
		default:
			return nil, "STATUS item is unsupported", false
		}
	}
	if len(out) == 0 {
		return nil, "STATUS requires status data items", false
	}
	return out, "", true
}

func imapStatusItemListInner(items []string) (string, bool) {
	if len(items) == 0 {
		return "", false
	}
	joined := strings.Join(items, " ")
	if strings.TrimSpace(joined) != joined ||
		!strings.HasPrefix(joined, "(") ||
		!strings.HasSuffix(joined, ")") ||
		strings.Count(joined, "(") != 1 ||
		strings.Count(joined, ")") != 1 {
		return "", false
	}
	return joined[1 : len(joined)-1], true
}

func imapStatusItemListIsParenthesized(items []string) bool {
	if len(items) == 0 {
		return false
	}
	_, ok := imapStatusItemListInner(items)
	return ok
}

func imapStatusItemListIsEmpty(items []string) bool {
	joined := strings.TrimSpace(strings.Join(items, " "))
	if !strings.HasPrefix(joined, "(") || !strings.HasSuffix(joined, ")") {
		return false
	}
	return strings.TrimSpace(joined[1:len(joined)-1]) == ""
}

func imapStatusRequestsItem(items []string, want string) bool {
	for _, item := range items {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}

func imapStatusData(mailbox Mailbox, items []string) string {
	parts := make([]string, 0, len(items)*2)
	for _, item := range items {
		switch item {
		case "MESSAGES":
			parts = append(parts, "MESSAGES", strconv.FormatUint(uint64(mailbox.Messages), 10))
		case "RECENT":
			parts = append(parts, "RECENT", strconv.FormatUint(uint64(mailbox.Recent), 10))
		case "UIDNEXT":
			parts = append(parts, "UIDNEXT", strconv.FormatUint(uint64(mailbox.UIDNext), 10))
		case "UIDVALIDITY":
			parts = append(parts, "UIDVALIDITY", strconv.FormatUint(uint64(mailbox.UIDValidity), 10))
		case "UNSEEN":
			parts = append(parts, "UNSEEN", strconv.FormatUint(uint64(mailbox.Unseen), 10))
		case "HIGHESTMODSEQ":
			parts = append(parts, "HIGHESTMODSEQ", strconv.FormatUint(mailbox.HighestModSeq, 10))
		case "SIZE":
			parts = append(parts, "SIZE", strconv.FormatInt(mailbox.Size, 10))
		}
	}
	return strings.Join(parts, " ")
}

