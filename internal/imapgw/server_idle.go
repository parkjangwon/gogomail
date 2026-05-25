package imapgw

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

func (s *Server) handleIdleDone(writer *bufio.Writer, line string, state *imapConnState) (bool, error) {
	tag := state.pendingIdleTag
	if !strings.EqualFold(line, "DONE") {
		_, err := writer.WriteString(tag + " BAD IDLE terminated by unexpected command\r\n")
		state.pendingIdleTag = ""
		return false, err
	}
	state.pendingIdleTag = ""
	if err := s.drainMailboxEvents(writer, state); err != nil {
		return false, err
	}
	_, err := writer.WriteString(tag + " OK IDLE completed\r\n")
	return false, err
}

type idleLineResult struct {
	line string
	err  error
}

func (s *Server) serveIdle(conn net.Conn, reader *bufio.Reader, writer *bufio.Writer, state *imapConnState) (retErr error) {
	if err := s.setReadDeadline(conn, s.options.IdleTimeout); err != nil {
		return err
	}
	lineCh := make(chan idleLineResult, 1)
	go func() {
		line, err := readIMAPLine(reader, maxIMAPCommandLineBytes)
		lineCh <- idleLineResult{line: line, err: err}
	}()
	readerConsumed := false
	// On error return, the reader goroutine above may still be blocked on
	// the socket read. Force its read to unblock by setting a read
	// deadline in the past so the reader exits promptly instead of
	// lingering until the connection is finally closed. This prevents
	// goroutine pile-up under error storms with many concurrent IDLE
	// clients.
	defer func() {
		if retErr != nil && !readerConsumed && conn != nil {
			_ = conn.SetReadDeadline(time.Now().Add(-time.Second))
			// Drain the reader goroutine so it can exit before we return.
			<-lineCh
		}
	}()
	for state.pendingIdleTag != "" {
		select {
		case result := <-lineCh:
			readerConsumed = true
			if result.err != nil {
				if errors.Is(result.err, io.EOF) {
					return nil
				}
				if errors.Is(result.err, errIMAPCommandLineTooLong) {
					return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line is too long"}
				}
				return result.err
			}
			if len(result.line) > 8192 {
				return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line is too long"}
			}
			if !imapLineHasCRLF(result.line) {
				return imapProtocolFramingError{line: state.pendingIdleTag + " IDLE", message: "command line must end with CRLF"}
			}
			_, err := s.handleIdleDone(writer, strings.TrimRight(result.line, "\r\n"), state)
			return err
		case event, ok := <-state.events:
			if !ok {
				state.events = nil
				state.cancelEvents = nil
				continue
			}
			if event.UserID != state.session.UserID || event.MailboxID != state.selectedMailbox {
				continue
			}
			if err := s.writeMailboxEvent(writer, state, event); err != nil {
				return err
			}
			if err := s.setWriteDeadline(conn); err != nil {
				return err
			}
			if err := writer.Flush(); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Server) drainMailboxEvents(writer *bufio.Writer, state *imapConnState) error {
	if state == nil || state.events == nil || state.session == nil || state.selectedMailbox == "" {
		return nil
	}
	for {
		select {
		case event, ok := <-state.events:
			if !ok {
				state.events = nil
				state.cancelEvents = nil
				return nil
			}
			if event.UserID != state.session.UserID || event.MailboxID != state.selectedMailbox {
				continue
			}
			if err := s.writeMailboxEvent(writer, state, event); err != nil {
				return err
			}
		default:
			return nil
		}
	}
}

func (s *Server) firstUnseenSequenceNumber(ctx context.Context, userID UserID, mailbox MailboxState) uint32 {
	if s == nil || s.options.Backend == nil || mailbox.Unseen == 0 || mailbox.Messages == 0 {
		return 0
	}
	messages, err := s.options.Backend.ListMessages(ctx, ListMessagesRequest{
		UserID:    userID,
		MailboxID: mailbox.ID,
		Limit:     int(mailbox.Messages),
	})
	if err != nil {
		return 0
	}
	for i, summary := range messages {
		if summary.Flags.Read {
			continue
		}
		sequenceNumber := summary.SequenceNumber
		if sequenceNumber == 0 {
			sequenceNumber = uint32(i + 1)
		}
		return sequenceNumber
	}
	return 0
}

func (s *Server) writeMailboxEvent(writer *bufio.Writer, state *imapConnState, event MailboxEvent) error {
	switch event.Type {
	case MailboxEventExists:
		if event.Messages > 0 {
			if event.Messages <= state.selectedMessages {
				return nil
			}
			state.selectedMessages = event.Messages
		} else {
			state.selectedMessages++
		}
		return writeIMAPUintLine(writer, "* ", uint64(state.selectedMessages), " EXISTS\r\n")
	case MailboxEventExpunge:
		sequenceNumber := event.SequenceNumber
		if sequenceNumber == 0 {
			return nil
		}
		if state.selectedMessages == 0 {
			return nil
		}
		if sequenceNumber > state.selectedMessages {
			sequenceNumber = state.selectedMessages
		}
		if sequenceNumber == 0 {
			return nil
		}
		if state.selectedMessages > 0 {
			state.selectedMessages--
		}
		state.removeExpungedFromSavedSearch([]MessageSummary{{SequenceNumber: sequenceNumber}})
		return writeIMAPUintLine(writer, "* ", uint64(sequenceNumber), " EXPUNGE\r\n")
	case MailboxEventFlags:
		message, err := s.options.Backend.FetchMessage(state.ctx, FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       event.UID,
		})
		if err != nil {
			return err
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		state.observeHighestModSeq(message.Summary.ModSeq)
		sequenceNumber, ok := imapSequenceNumber(message.Summary)
		if !ok {
			return fmt.Errorf("imap event sequence number is unavailable")
		}
		attributes := []string{
			"UID " + strconv.FormatUint(uint64(message.Summary.UID), 10),
			"FLAGS " + imapFlagList(message.Summary.Flags.IMAPFlags()),
		}
		if state.condstoreAware {
			attributes = append(attributes, "MODSEQ ("+strconv.FormatUint(message.Summary.ModSeq, 10)+")")
		}
		return writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), ")")
	default:
		return nil
	}
}

func (state *imapConnState) closeSubscription() {
	if state == nil || state.cancelEvents == nil {
		return
	}
	state.cancelEvents()
	state.cancelEvents = nil
	state.events = nil
}

func (state *imapConnState) deselectMailbox() {
	if state == nil {
		return
	}
	state.selectedMailbox = ""
	state.selectedMessages = 0
	state.selectedHighestModSeq = 0
	state.selectedNoModSeq = false
	state.permanentFlags = nil
	state.readOnly = false
	state.savedSearch = nil
	state.closeSubscription()
}

func imapCommandShouldDrainSelectedEvents(command string) bool {
	switch strings.ToUpper(command) {
	case "FETCH", "STORE", "COPY", "MOVE", "SEARCH", "SORT", "THREAD", "CHECK", "CLOSE", "UNSELECT", "EXPUNGE", "UID", "APPEND":
		return true
	default:
		return false
	}
}

