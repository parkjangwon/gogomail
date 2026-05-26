package imapgw

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	messageparse "github.com/gogomail/gogomail/internal/message"
)

func (s *Server) handleFetch(writer *bufio.Writer, tag string, fields []string, state *imapConnState) (bool, error) {
	if len(fields) < 4 {
		_, err := writer.WriteString(tag + " BAD FETCH requires sequence set and data items\r\n")
		return false, err
	}
	if !imapSequenceSetSyntaxValid(fields[2]) {
		_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
		return false, err
	}
	if message, ok := imapFetchDataItemsSyntaxError(fields[3:]); ok {
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
		_, err := writer.WriteString(tag + " BAD FETCH requires a valid message sequence set\r\n")
		return false, err
	}
	uids, err := s.uidsForSequenceNumbers(state.ctx, state, sequenceNumbers)
	if err != nil {
		_, writeErr := writer.WriteString(tag + " NO FETCH failed\r\n")
		return false, writeErr
	}
	return s.writeFetchResponses(writer, tag, fields[3:], state, uids, "FETCH")
}

func (s *Server) writeFetchResponses(writer *bufio.Writer, tag string, items []string, state *imapConnState, uids []UID, completionCommand string) (bool, error) {
	changedSince, requestsChangedSince, _ := imapFetchChangedSince(items)
	if message, ok := imapFetchDataItemsSyntaxError(items); ok {
		_, err := writer.WriteString(tag + " BAD " + message + "\r\n")
		return false, err
	}
	items = imapExpandFetchItems(items)
	requestsBody := imapFetchRequestsBody(items)
	partial, requestsPartialBody := imapFetchPartialBody(items)
	partialSection, requestsPartialSection := imapFetchPartialSection(items)
	partRequest, requestsMIMEPart := imapFetchMIMEPartRequest(items)
	requestsHeader := imapFetchRequestsHeader(items)
	requestsText := imapFetchRequestsText(items)
	requestsPartText := imapFetchRequestsPartText(items)
	requestsPartMIME := imapFetchRequestsPartMIME(items)
	headerFields, requestsHeaderFields := imapFetchHeaderFields(items)
	headerFieldsNot, requestsHeaderFieldsNot := imapFetchHeaderFieldsNot(items)
	partialHeaderFields, requestsPartialHeaderFields := imapFetchPartialHeaderFields(items)
	partialHeaderFieldsNot, requestsPartialHeaderFieldsNot := imapFetchPartialHeaderFieldsNot(items)
	requestsEnvelope := imapFetchRequestsEnvelope(items)
	requestsInternalDate := imapFetchRequestsInternalDate(items)
	requestsModSeq := requestsChangedSince || imapFetchRequestsModSeq(items)
	if requestsModSeq {
		if !state.selectedSupportsPersistentModSeq() {
			return s.rejectSelectedNoModSeq(writer, tag, state, completionCommand)
		}
		state.condstoreAware = true
	}
	requestsBodyAttribute := imapFetchRequestsBodyAttribute(items)
	requestsBodyStructure := imapFetchRequestsBodyStructure(items)
	setsSeen := imapFetchSetsSeen(items)
	for _, uid := range uids {
		fetchReq := FetchMessageRequest{
			UserID:    state.session.UserID,
			MailboxID: state.selectedMailbox,
			UID:       uid,
		}
		message, err := s.options.Backend.FetchMessage(state.ctx, fetchReq)
		if err != nil {
			_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
			return false, writeErr
		}
		summary := message.Summary
		if summary.UID == 0 {
			summary.UID = uid
		}
		if requestsChangedSince && summary.ModSeq <= changedSince {
			if message.Body != nil {
				if err := message.Body.Close(); err != nil {
					return false, err
				}
			}
			continue
		}
		requestsLiteral := requestsBody || requestsPartialBody || requestsPartialSection || requestsMIMEPart || requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME
		bodyAttribute := ""
		bodyStructure := ""
		if requestsBodyAttribute || requestsBodyStructure {
			structureMessage := message
			if requestsLiteral {
				var err error
				structureMessage, err = s.options.Backend.FetchMessage(state.ctx, fetchReq)
				if err != nil {
					structureMessage = Message{}
				}
			}
			if structureMessage.Body != nil {
				structure, err := messageparse.ParseMIMEStructure(structureMessage.Body, messageparse.MIMEStructureOptions{})
				if closeErr := structureMessage.Body.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err == nil {
					bodyAttribute = imapBodyFromMIMEStructure(summary, structure)
					bodyStructure = imapBodyStructureFromMIMEStructure(summary, structure)
				}
			}
			if bodyAttribute == "" {
				bodyAttribute = imapBody(summary)
			}
			if bodyStructure == "" {
				bodyStructure = imapBodyStructure(summary)
			}
			if !requestsLiteral {
				message.Body = nil
			}
		} else if !requestsLiteral && message.Body != nil {
			if err := message.Body.Close(); err != nil {
				return false, err
			}
			message.Body = nil
		}
		if !requestsLiteral {
			if message.Body != nil {
				if err := message.Body.Close(); err != nil {
					return false, err
				}
				message.Body = nil
			}
		}
		sequenceNumber, ok := imapSequenceNumber(summary)
		if !ok {
			_, err := writer.WriteString(tag + " NO " + completionCommand + " sequence number is unavailable\r\n")
			return false, err
		}
		if requestsLiteral {
			if message.Body == nil {
				_, err := writer.WriteString(tag + " NO " + completionCommand + " body is unavailable\r\n")
				return false, err
			}
			body := message.Body
			if summary.Size < 0 {
				_ = body.Close()
				_, err := writer.WriteString(tag + " NO " + completionCommand + " body size is unavailable\r\n")
				return false, err
			}
			if setsSeen {
				var err error
				summary, err = s.markFetchSeen(state.ctx, state, summary)
				if err != nil {
					_ = body.Close()
					_, writeErr := writer.WriteString(tag + " NO " + completionCommand + " failed\r\n")
					return false, writeErr
				}
			}
			if requestsMIMEPart {
				literal, found, err := readIMAPMIMEPartLiteral(body, partRequest)
				if closeErr := body.Close(); closeErr != nil && err == nil {
					err = closeErr
				}
				if err != nil {
					return false, err
				}
				if !found {
					_, err := writer.WriteString(tag + " NO " + completionCommand + " body section is unavailable\r\n")
					return false, err
				}
				attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
				tail := " BODY[" + partRequest.sectionName() + "]" + partRequest.partialSuffix() + " {" + strconv.Itoa(len(literal)) + "}"
				if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), tail); err != nil {
					return false, err
				}
				if _, err := writer.Write(literal); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			if requestsPartialSection || requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || requestsText || requestsPartText || requestsPartMIME {
				wantHeader := requestsHeader || requestsHeaderFields || requestsHeaderFieldsNot || partialSection.headerLike()
				literal, err := readIMAPSectionLiteral(body, wantHeader)
				if err != nil {
					_ = body.Close()
					return false, err
				}
				if requestsPartMIME || partialSection.section == "1.MIME" {
					literal = []byte("\r\n")
				}
				if requestsHeaderFields {
					literal = filterIMAPHeaderFields(literal, headerFields, false)
				}
				if requestsHeaderFieldsNot {
					literal = filterIMAPHeaderFields(literal, headerFieldsNot, true)
				}
				if requestsPartialHeaderFields {
					literal = imapPartialLiteral(literal, partialHeaderFields)
				}
				if requestsPartialHeaderFieldsNot {
					literal = imapPartialLiteral(literal, partialHeaderFieldsNot)
				}
				if err := body.Close(); err != nil {
					return false, err
				}
				attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
				section := "TEXT"
				if requestsPartText {
					section = "1"
				}
				if requestsPartMIME {
					section = "1.MIME"
				}
				if requestsPartialSection {
					section = partialSection.section
					literal = imapPartialLiteral(literal, partialSection.partial)
				}
				if requestsHeader {
					section = "HEADER"
				}
				if requestsHeaderFields {
					section = imapHeaderFieldsSectionName("HEADER.FIELDS", headerFields)
				}
				if requestsHeaderFieldsNot {
					section = imapHeaderFieldsSectionName("HEADER.FIELDS.NOT", headerFieldsNot)
				}
				partialSuffix := ""
				if requestsPartialSection {
					partialSuffix = imapPartialOffsetSuffix(partialSection.partial.offset)
				}
				if requestsPartialHeaderFields {
					partialSuffix = imapPartialOffsetSuffix(partialHeaderFields.offset)
				}
				if requestsPartialHeaderFieldsNot {
					partialSuffix = imapPartialOffsetSuffix(partialHeaderFieldsNot.offset)
				}
				itemName := imapSectionLiteralResponseName(items, section)
				tail := " " + itemName + partialSuffix + " {" + strconv.Itoa(len(literal)) + "}"
				if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), tail); err != nil {
					return false, err
				}
				if _, err := writer.Write(literal); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			attributes := imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure)
			if requestsPartialBody {
				itemName := imapPartialBodyLiteralResponseName(items)
				count := partial.count
				if partial.offset >= uint64(summary.Size) {
					count = 0
				} else if remaining := uint64(summary.Size) - partial.offset; count > remaining {
					count = remaining
				}
				if _, err := io.CopyN(io.Discard, body, int64(partial.offset)); err != nil && !errors.Is(err, io.EOF) {
					_ = body.Close()
					return false, err
				}
				tail := " " + itemName + "<" + strconv.FormatUint(partial.offset, 10) + "> {" + strconv.FormatUint(count, 10) + "}"
				if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), tail); err != nil {
					_ = body.Close()
					return false, err
				}
				if count > 0 {
					if _, err := io.CopyN(writer, body, int64(count)); err != nil {
						_ = body.Close()
						return false, err
					}
				}
				if err := body.Close(); err != nil {
					return false, err
				}
				if _, err := writer.WriteString(")\r\n"); err != nil {
					return false, err
				}
				continue
			}
			tail := " " + imapFullBodyLiteralResponseName(items) + " {" + strconv.FormatUint(uint64(summary.Size), 10) + "}"
			if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(attributes, " "), tail); err != nil {
				_ = body.Close()
				return false, err
			}
			if _, err := io.CopyN(writer, body, summary.Size); err != nil {
				_ = body.Close()
				return false, err
			}
			if err := body.Close(); err != nil {
				return false, err
			}
			if _, err := writer.WriteString(")\r\n"); err != nil {
				return false, err
			}
			continue
		}
		if message.Body != nil {
			_ = message.Body.Close()
		}
		if err := writeIMAPFetchLine(writer, sequenceNumber, strings.Join(imapFetchAttributes(summary, requestsEnvelope, requestsInternalDate, requestsModSeq, requestsBodyAttribute, requestsBodyStructure, bodyAttribute, bodyStructure), " "), ")"); err != nil {
			return false, err
		}
	}
	_, err := writer.WriteString(tag + " OK " + completionCommand + " completed\r\n")
	return false, err
}

func (s *Server) markFetchSeen(ctx context.Context, state *imapConnState, summary MessageSummary) (MessageSummary, error) {
	if state == nil || state.readOnly || summary.Flags.Read || summary.UID == 0 {
		return summary, nil
	}
	if s == nil || s.options.Backend == nil {
		return summary, fmt.Errorf("imap backend is required")
	}
	updated, err := s.options.Backend.StoreFlags(ctx, StoreFlagsRequest{
		UserID:    state.session.UserID,
		MailboxID: state.selectedMailbox,
		UIDs:      []UID{summary.UID},
		Flags:     MessageFlags{Read: true},
		Mode:      StoreFlagsAdd,
	})
	if err != nil {
		return summary, err
	}
	summary.Flags.Read = true
	for _, item := range updated {
		if item.UID != summary.UID {
			continue
		}
		if item.ModSeq > summary.ModSeq {
			summary.ModSeq = item.ModSeq
		}
		if item.SequenceNumber != 0 {
			summary.SequenceNumber = item.SequenceNumber
		}
		break
	}
	return summary, nil
}

func imapFetchRequestsBody(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[]" || token == "BODY.PEEK[]" || token == "RFC822" {
			return true
		}
	}
	return false
}

func imapFetchSetsSeen(items []string) bool {
	for _, item := range items {
		seen := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			switch {
			case token == "RFC822" || strings.HasPrefix(token, "RFC822<") || token == "RFC822.TEXT" || strings.HasPrefix(token, "RFC822.TEXT<"):
				seen = true
				return false
			case token == "RFC822.HEADER" || strings.HasPrefix(token, "RFC822.HEADER<"):
				return true
			case strings.HasPrefix(token, "BODY.PEEK["):
				return true
			case strings.HasPrefix(token, "BODY["):
				seen = true
				return false
			}
			return true
		})
		if seen {
			return true
		}
	}
	return false
}

func imapFullBodyLiteralResponseName(items []string) string {
	for _, item := range items {
		found := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			if token == "RFC822" {
				found = true
				return false
			}
			return true
		})
		if found {
			return "RFC822"
		}
	}
	return "BODY[]"
}

func imapPartialBodyLiteralResponseName(items []string) string {
	for _, item := range items {
		found := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			if strings.HasPrefix(token, "RFC822<") {
				found = true
				return false
			}
			return true
		})
		if found {
			return "RFC822"
		}
	}
	return "BODY[]"
}

func imapSectionLiteralResponseName(items []string, section string) string {
	for _, item := range items {
		found := ""
		imapEachNormalizedFetchToken(item, func(token string) bool {
			if section == "HEADER" && (token == "RFC822.HEADER" || strings.HasPrefix(token, "RFC822.HEADER<")) {
				found = "RFC822.HEADER"
				return false
			}
			if section == "TEXT" && (token == "RFC822.TEXT" || strings.HasPrefix(token, "RFC822.TEXT<")) {
				found = "RFC822.TEXT"
				return false
			}
			return true
		})
		if found != "" {
			return found
		}
	}
	return "BODY[" + section + "]"
}

func imapHeaderFieldsSectionName(marker string, fields []string) string {
	normalized := make([]string, 0, len(fields))
	for _, field := range fields {
		field = strings.ToUpper(strings.TrimSpace(field))
		if field != "" {
			normalized = append(normalized, field)
		}
	}
	return marker + " (" + strings.Join(normalized, " ") + ")"
}

func imapExpandFetchItems(items []string) []string {
	expanded := make([]string, 0, len(items)+4)
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		switch token {
		case "FAST":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE")
		case "ALL":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE")
		case "FULL":
			expanded = append(expanded, "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE", "BODY")
		default:
			expanded = append(expanded, item)
		}
	}
	return expanded
}

