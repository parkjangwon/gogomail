package imapgw

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	stdmail "net/mail"

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

type imapPartialBodyRequest struct {
	offset uint64
	count  uint64
}

type imapPartialSectionRequest struct {
	section string
	partial imapPartialBodyRequest
}

type imapMIMEPartRequest struct {
	path                []int
	mime                bool
	messageSection      string
	messageHeaderFields []string
	messageHeaderNot    bool
	partial             imapPartialBodyRequest
}

func (r imapMIMEPartRequest) sectionName() string {
	parts := make([]string, 0, len(r.path)+1)
	for _, value := range r.path {
		parts = append(parts, strconv.Itoa(value))
	}
	if r.mime {
		parts = append(parts, "MIME")
	}
	if r.messageSection != "" {
		parts = append(parts, r.messageSection)
		if strings.HasPrefix(r.messageSection, "HEADER.FIELDS") {
			parts[len(parts)-1] += " (" + strings.Join(r.messageHeaderFields, " ") + ")"
		}
	}
	return strings.Join(parts, ".")
}

func (r imapMIMEPartRequest) partialSuffix() string {
	if r.partial.count == 0 {
		return ""
	}
	return imapPartialOffsetSuffix(r.partial.offset)
}

func (r imapPartialSectionRequest) headerLike() bool {
	return r.section == "HEADER" || r.section == "1.MIME"
}

func imapFetchPartialBody(items []string) (imapPartialBodyRequest, bool) {
	for _, item := range items {
		var req imapPartialBodyRequest
		found := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			if !strings.HasPrefix(token, "BODY[]<") && !strings.HasPrefix(token, "BODY.PEEK[]<") && !strings.HasPrefix(token, "RFC822<") {
				return true
			}
			req, found = imapParsePartialBodyToken(token)
			return false
		})
		if found {
			return req, true
		}
	}
	return imapPartialBodyRequest{}, false
}

func imapFetchPartialSection(items []string) (imapPartialSectionRequest, bool) {
	sections := []struct {
		prefixes []string
		section  string
	}{
		{[]string{"BODY[HEADER]<", "BODY.PEEK[HEADER]<", "RFC822.HEADER<"}, "HEADER"},
		{[]string{"BODY[TEXT]<", "BODY.PEEK[TEXT]<", "RFC822.TEXT<"}, "TEXT"},
		{[]string{"BODY[1]<", "BODY.PEEK[1]<"}, "1"},
		{[]string{"BODY[1.MIME]<", "BODY.PEEK[1.MIME]<"}, "1.MIME"},
	}
	for _, item := range items {
		var req imapPartialSectionRequest
		found := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			for _, candidate := range sections {
				for _, prefix := range candidate.prefixes {
					if !strings.HasPrefix(token, prefix) {
						continue
					}
					partial, ok := imapParsePartialBodyToken(token)
					if !ok {
						found = false
						return false
					}
					req = imapPartialSectionRequest{section: candidate.section, partial: partial}
					found = true
					return false
				}
			}
			return true
		})
		if found {
			return req, true
		}
	}
	return imapPartialSectionRequest{}, false
}

func imapFetchMIMEPartRequest(items []string) (imapMIMEPartRequest, bool) {
	if req, ok := imapParseMIMEPartHeaderFieldsRequest(items); ok {
		return req, true
	}
	for _, item := range items {
		var req imapMIMEPartRequest
		found := false
		imapEachNormalizedFetchToken(item, func(token string) bool {
			req, found = imapParseMIMEPartRequestToken(token)
			return !found
		})
		if found {
			return req, true
		}
	}
	return imapMIMEPartRequest{}, false
}

func imapParseMIMEPartHeaderFieldsRequest(items []string) (imapMIMEPartRequest, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
		idx := strings.Index(joined, "."+marker)
		if idx < 0 {
			continue
		}
		openIdx := strings.LastIndex(joined[:idx], "BODY[")
		if peekIdx := strings.LastIndex(joined[:idx], "BODY.PEEK["); peekIdx > openIdx {
			openIdx = peekIdx
		}
		if openIdx < 0 {
			return imapMIMEPartRequest{}, false
		}
		pathText := joined[openIdx:idx]
		pathText = strings.TrimPrefix(pathText, "BODY.PEEK[")
		pathText = strings.TrimPrefix(pathText, "BODY[")
		path, ok := parseIMAPMIMEPartPath(pathText)
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		fieldsStart := strings.Index(joined[idx+len(marker)+1:], "(")
		if fieldsStart < 0 {
			return imapMIMEPartRequest{}, false
		}
		fieldsStart += idx + len(marker) + 1
		fieldsEnd := strings.Index(joined[fieldsStart+1:], ")")
		if fieldsEnd < 0 {
			return imapMIMEPartRequest{}, false
		}
		fieldsEnd += fieldsStart + 1
		fields, ok := imapHeaderFieldListNames(joined[fieldsStart+1 : fieldsEnd])
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		req := imapMIMEPartRequest{
			path:                path,
			messageSection:      marker,
			messageHeaderFields: fields,
			messageHeaderNot:    marker == "HEADER.FIELDS.NOT",
		}
		suffix := strings.TrimSpace(joined[fieldsEnd+1:])
		suffix = strings.TrimPrefix(suffix, "]")
		if strings.HasPrefix(suffix, "<") {
			partial, ok := imapParsePartialBodyToken(suffix)
			if !ok {
				return imapMIMEPartRequest{}, false
			}
			req.partial = partial
		}
		return req, true
	}
	return imapMIMEPartRequest{}, false
}

func imapParseMIMEPartRequestToken(token string) (imapMIMEPartRequest, bool) {
	if strings.HasPrefix(token, "BODY.PEEK[") {
		token = "BODY[" + strings.TrimPrefix(token, "BODY.PEEK[")
	}
	if !strings.HasPrefix(token, "BODY[") {
		return imapMIMEPartRequest{}, false
	}
	closeIdx := strings.Index(token, "]")
	if closeIdx < 0 {
		return imapMIMEPartRequest{}, false
	}
	section := token[len("BODY["):closeIdx]
	if section == "" || section == "HEADER" || section == "TEXT" || strings.HasPrefix(section, "HEADER.") {
		return imapMIMEPartRequest{}, false
	}
	parts := strings.Split(section, ".")
	mimeSection := false
	if parts[len(parts)-1] == "MIME" {
		mimeSection = true
		parts = parts[:len(parts)-1]
	}
	messageSection := ""
	if !mimeSection && (parts[len(parts)-1] == "HEADER" || parts[len(parts)-1] == "TEXT") {
		messageSection = parts[len(parts)-1]
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return imapMIMEPartRequest{}, false
	}
	if len(parts) > maxIMAPMIMEPartPathDepth {
		return imapMIMEPartRequest{}, false
	}
	path, ok := parseIMAPMIMEPartPath(strings.Join(parts, "."))
	if !ok {
		return imapMIMEPartRequest{}, false
	}
	req := imapMIMEPartRequest{path: path, mime: mimeSection, messageSection: messageSection}
	if suffix := token[closeIdx+1:]; suffix != "" {
		if !strings.HasPrefix(suffix, "<") {
			return imapMIMEPartRequest{}, false
		}
		partial, ok := imapParsePartialBodyToken(token)
		if !ok {
			return imapMIMEPartRequest{}, false
		}
		req.partial = partial
	}
	return req, true
}

func parseIMAPMIMEPartPath(value string) ([]int, bool) {
	if strings.TrimSpace(value) != value {
		return nil, false
	}
	parts := strings.Split(value, ".")
	if len(parts) == 0 || len(parts) > maxIMAPMIMEPartPathDepth {
		return nil, false
	}
	path := make([]int, 0, len(parts))
	for _, part := range parts {
		if !imapNZNumberAtomDigitsOnly(part) {
			return nil, false
		}
		number, err := strconv.ParseUint(part, 10, 32)
		if err != nil || number == 0 {
			return nil, false
		}
		if strconv.IntSize == 32 && number > uint64(int(^uint(0)>>1)) {
			return nil, false
		}
		path = append(path, int(number))
	}
	return path, true
}

func imapParsePartialBodyToken(token string) (imapPartialBodyRequest, bool) {
	start := strings.Index(token, "<")
	end := strings.LastIndex(token, ">")
	if start < 0 || end <= start || end != len(token)-1 {
		return imapPartialBodyRequest{}, false
	}
	offsetText, countText, ok := strings.Cut(token[start+1:end], ".")
	if !ok {
		return imapPartialBodyRequest{}, false
	}
	if !imapNumberAtomRFC3501(offsetText) || !imapNZNumberAtomDigitsOnly(countText) {
		return imapPartialBodyRequest{}, false
	}
	offset, err := strconv.ParseUint(offsetText, 10, 32)
	if err != nil {
		return imapPartialBodyRequest{}, false
	}
	count, err := strconv.ParseUint(countText, 10, 32)
	if err != nil || count == 0 {
		return imapPartialBodyRequest{}, false
	}
	return imapPartialBodyRequest{offset: offset, count: count}, true
}

func imapPartialLiteral(literal []byte, partial imapPartialBodyRequest) []byte {
	if partial.offset >= uint64(len(literal)) {
		return nil
	}
	end := partial.offset + partial.count
	if end > uint64(len(literal)) {
		end = uint64(len(literal))
	}
	return literal[partial.offset:end]
}

func readIMAPMIMEPartLiteral(reader io.Reader, req imapMIMEPartRequest) ([]byte, bool, error) {
	if reader == nil {
		return nil, false, nil
	}
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		if len(req.path) == 1 && req.path[0] == 1 && !req.mime {
			if req.partial.count > 0 {
				data = imapPartialLiteral(data, req.partial)
			}
			return data, true, nil
		}
		return nil, false, nil
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		if mediaType == "message/rfc822" && len(req.path) > 1 && req.path[0] == 1 {
			literal, found, err := readIMAPMIMEPartLiteralFromMessage(message.Body, req.path[1:], req)
			if err != nil || !found {
				return nil, found, err
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		if req.messageSection != "" && len(req.path) == 1 && req.path[0] == 1 && mediaType == "message/rfc822" {
			literal, found, err := readIMAPMIMEPartLiteralFromMessage(message.Body, nil, req)
			if err != nil || !found {
				return nil, false, err
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		if len(req.path) == 1 && req.path[0] == 1 && req.mime {
			return []byte("\r\n"), true, nil
		}
		if len(req.path) == 1 && req.path[0] == 1 && !req.mime {
			literal, err := io.ReadAll(io.LimitReader(message.Body, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			if req.partial.count > 0 {
				literal = imapPartialLiteral(literal, req.partial)
			}
			return literal, true, nil
		}
		return nil, false, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, false, nil
	}
	literal, found, err := readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(message.Body, boundary), req.path, req)
	if err != nil || !found {
		return nil, found, err
	}
	if req.partial.count > 0 {
		literal = imapPartialLiteral(literal, req.partial)
	}
	return literal, true, nil
}

func readIMAPMIMEPartLiteralFromMultipart(reader *multipart.Reader, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	if len(path) == 0 {
		return nil, false, nil
	}
	for i := 1; ; i++ {
		part, err := reader.NextRawPart()
		if err == io.EOF {
			return nil, false, nil
		}
		if err != nil {
			return nil, false, err
		}
		if i != path[0] {
			_ = part.Close()
			continue
		}
		defer part.Close()
		if len(path) == 1 {
			if req.messageSection != "" {
				mediaType, _, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
				if err != nil || strings.ToLower(mediaType) != "message/rfc822" {
					return nil, false, nil
				}
				literal, found, err := readIMAPMIMEPartLiteralFromMessage(part, nil, req)
				if err != nil || !found {
					return nil, false, err
				}
				return literal, true, nil
			}
			if req.mime {
				return imapMIMEHeaderLiteral(part.Header), true, nil
			}
			literal, err := io.ReadAll(io.LimitReader(part, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			return literal, true, nil
		}
		mediaType, params, err := mime.ParseMediaType(part.Header.Get("Content-Type"))
		mediaType = strings.ToLower(mediaType)
		if err == nil && mediaType == "message/rfc822" {
			return readIMAPMIMEPartLiteralFromMessage(part, path[1:], req)
		}
		if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
			return nil, false, nil
		}
		boundary := strings.TrimSpace(params["boundary"])
		if boundary == "" {
			return nil, false, nil
		}
		return readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(part, boundary), path[1:], req)
	}
}

func readIMAPMIMEPartLiteralFromMessage(reader io.Reader, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maxIMAPSearchLiteralBytes+1))
	if err != nil {
		return nil, false, err
	}
	if len(data) > maxIMAPSearchLiteralBytes {
		return nil, false, fmt.Errorf("imap message/rfc822 literal exceeds limit")
	}
	if req.messageSection != "" {
		if len(path) != 0 {
			return nil, false, nil
		}
		return readIMAPRawMessageSectionLiteral(data, req), true, nil
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(data))
	if err != nil {
		return readIMAPMalformedMessageLiteral(data, path, req)
	}
	if len(path) == 0 {
		return nil, false, nil
	}
	mediaType, params, err := mime.ParseMediaType(message.Header.Get("Content-Type"))
	mediaType = strings.ToLower(mediaType)
	if err != nil || !strings.HasPrefix(mediaType, "multipart/") {
		if len(path) == 1 && path[0] == 1 && req.mime {
			return []byte("\r\n"), true, nil
		}
		if len(path) == 1 && path[0] == 1 && !req.mime {
			literal, err := io.ReadAll(io.LimitReader(message.Body, maxIMAPSearchLiteralBytes+1))
			if err != nil {
				return nil, false, err
			}
			if len(literal) > maxIMAPSearchLiteralBytes {
				return nil, false, fmt.Errorf("imap mime part literal exceeds limit")
			}
			return literal, true, nil
		}
		return nil, false, nil
	}
	boundary := strings.TrimSpace(params["boundary"])
	if boundary == "" {
		return nil, false, nil
	}
	return readIMAPMIMEPartLiteralFromMultipart(multipart.NewReader(message.Body, boundary), path, imapMIMEPartRequest{mime: req.mime})
}

func readIMAPRawMessageSectionLiteral(data []byte, req imapMIMEPartRequest) []byte {
	end := imapHeaderEnd(data)
	if end < 0 {
		if req.messageSection == "TEXT" {
			return data
		}
		return []byte("\r\n")
	}
	if req.messageSection == "TEXT" {
		return data[end:]
	}
	header := data[:end]
	if strings.HasPrefix(req.messageSection, "HEADER.FIELDS") {
		header = filterIMAPHeaderFields(header, req.messageHeaderFields, req.messageHeaderNot)
	}
	return header
}

func readIMAPMalformedMessageLiteral(data []byte, path []int, req imapMIMEPartRequest) ([]byte, bool, error) {
	if req.messageSection != "" {
		if len(path) != 0 {
			return nil, false, nil
		}
		if req.messageSection == "TEXT" {
			return data, true, nil
		}
		return []byte("\r\n"), true, nil
	}
	if len(path) == 1 && path[0] == 1 {
		if req.mime {
			return []byte("\r\n"), true, nil
		}
		return data, true, nil
	}
	return nil, false, nil
}

func readIMAPMessageSectionLiteral(reader io.Reader, req imapMIMEPartRequest) ([]byte, error) {
	literal, err := readIMAPSectionLiteral(reader, req.messageSection != "TEXT")
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(req.messageSection, "HEADER.FIELDS") {
		literal = filterIMAPHeaderFields(literal, req.messageHeaderFields, req.messageHeaderNot)
	}
	return literal, nil
}

func imapMIMEHeaderLiteral(header textproto.MIMEHeader) []byte {
	if len(header) == 0 {
		return []byte("\r\n")
	}
	keys := make([]string, 0, len(header))
	for key := range header {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var out strings.Builder
	for _, key := range keys {
		for _, value := range header[key] {
			out.WriteString(key)
			out.WriteString(": ")
			out.WriteString(value)
			out.WriteString("\r\n")
		}
	}
	out.WriteString("\r\n")
	return []byte(out.String())
}

func imapFetchRequestsHeader(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[HEADER]" || token == "BODY.PEEK[HEADER]" || token == "RFC822.HEADER" {
			return true
		}
	}
	return false
}

func imapFetchHeaderFields(items []string) ([]string, bool) {
	return imapFetchHeaderFieldList(items, "HEADER.FIELDS")
}

func imapFetchHeaderFieldsNot(items []string) ([]string, bool) {
	return imapFetchHeaderFieldList(items, "HEADER.FIELDS.NOT")
}

func imapFetchPartialHeaderFields(items []string) (imapPartialBodyRequest, bool) {
	return imapFetchPartialHeaderFieldList(items, "HEADER.FIELDS")
}

func imapFetchPartialHeaderFieldsNot(items []string) (imapPartialBodyRequest, bool) {
	return imapFetchPartialHeaderFieldList(items, "HEADER.FIELDS.NOT")
}

func imapFetchPartialHeaderFieldList(items []string, marker string) (imapPartialBodyRequest, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	idx := strings.Index(joined, marker)
	if idx < 0 {
		return imapPartialBodyRequest{}, false
	}
	if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
		return imapPartialBodyRequest{}, false
	}
	start := strings.Index(joined[idx:], "(")
	if start < 0 {
		return imapPartialBodyRequest{}, false
	}
	end := strings.Index(joined[idx+start+1:], ")")
	if end < 0 {
		return imapPartialBodyRequest{}, false
	}
	suffix := strings.TrimSpace(joined[idx+start+1+end+1:])
	suffix = strings.TrimPrefix(suffix, "]")
	if !strings.HasPrefix(suffix, "<") {
		return imapPartialBodyRequest{}, false
	}
	return imapParsePartialBodyToken(suffix)
}

func imapFetchHeaderFieldList(items []string, marker string) ([]string, bool) {
	joined := strings.ToUpper(strings.Join(items, " "))
	idx := strings.Index(joined, marker)
	if idx < 0 {
		return nil, false
	}
	if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
		return nil, false
	}
	start := strings.Index(joined[idx:], "(")
	if start < 0 {
		return nil, false
	}
	end := strings.Index(joined[idx+start+1:], ")")
	if end < 0 {
		return nil, false
	}
	return imapHeaderFieldListNames(joined[idx+start+1 : idx+start+1+end])
}

func imapFetchHeaderFieldListsValid(items []string) bool {
	joined := strings.ToUpper(strings.Join(items, " "))
	for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
		offset := 0
		for {
			idx := strings.Index(joined[offset:], marker)
			if idx < 0 {
				break
			}
			idx += offset
			if marker == "HEADER.FIELDS" && strings.Contains(joined[idx:minInt(len(joined), idx+len("HEADER.FIELDS.NOT"))], "HEADER.FIELDS.NOT") {
				offset = idx + len(marker)
				continue
			}
			start := strings.Index(joined[idx:], "(")
			if start < 0 {
				return false
			}
			end := strings.Index(joined[idx+start+1:], ")")
			if end < 0 {
				return false
			}
			if _, ok := imapHeaderFieldListNames(joined[idx+start+1 : idx+start+1+end]); !ok {
				return false
			}
			offset = idx + start + 1 + end + 1
		}
	}
	return true
}

func imapHeaderFieldListNames(fieldsText string) ([]string, bool) {
	if fieldsText == "" {
		return nil, true
	}
	if strings.TrimSpace(fieldsText) != fieldsText {
		return nil, false
	}
	fields := strings.Split(fieldsText, " ")
	names := make([]string, 0, len(fields))
	for _, field := range fields {
		if !imapHeaderFieldNameValid(field) {
			return nil, false
		}
		names = append(names, field)
	}
	return names, true
}

func imapHeaderFieldNameValid(field string) bool {
	if field == "" {
		return false
	}
	for i := 0; i < len(field); i++ {
		c := field[i]
		switch c {
		case '(', ')', '{', '%', '*', '"', '\\', ']', ':':
			return false
		default:
			if c <= 0x20 || c >= 0x7f {
				return false
			}
		}
	}
	return true
}

func imapSearchHeaderFieldNameValid(field string) bool {
	return imapHeaderFieldNameValid(strings.ToUpper(field))
}

func filterIMAPHeaderFields(header []byte, fields []string, exclude bool) []byte {
	if len(header) == 0 {
		return []byte("\r\n")
	}
	if len(fields) == 0 {
		if exclude {
			return header
		}
		return []byte("\r\n")
	}
	allowed := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		allowed[strings.ToUpper(field)] = struct{}{}
	}
	lines := strings.SplitAfter(string(header), "\n")
	var out strings.Builder
	include := false
	for _, line := range lines {
		trimmed := strings.TrimRight(line, "\r\n")
		if trimmed == "" {
			break
		}
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			if include {
				out.WriteString(line)
			}
			continue
		}
		name, _, ok := strings.Cut(trimmed, ":")
		if !ok {
			include = false
			continue
		}
		_, found := allowed[strings.ToUpper(name)]
		include = found
		if exclude {
			include = !found
		}
		if include {
			out.WriteString(line)
		}
	}
	out.WriteString("\r\n")
	return []byte(out.String())
}

func minInt(a int, b int) int {
	if a < b {
		return a
	}
	return b
}

func imapFetchRequestsText(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[TEXT]" || token == "BODY.PEEK[TEXT]" || token == "RFC822.TEXT" {
			return true
		}
	}
	return false
}

func imapFetchRequestsPartText(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[1]" || token == "BODY.PEEK[1]" {
			return true
		}
	}
	return false
}

func imapFetchRequestsPartMIME(items []string) bool {
	for _, item := range items {
		token := strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
		if token == "BODY[1.MIME]" || token == "BODY.PEEK[1.MIME]" {
			return true
		}
	}
	return false
}

func readIMAPSectionLiteral(reader io.Reader, wantHeader bool) ([]byte, error) {
	const maxHeaderBytes = 1 << 20

	var data []byte
	buffer := acquireLiteralBuffer()
	defer releaseLiteralBuffer(buffer)

	for {
		n, err := reader.Read(buffer)
		if n > 0 {
			data = append(data, buffer[:n]...)
			if len(data) > maxHeaderBytes {
				return nil, fmt.Errorf("imap header literal exceeds limit")
			}
			if end := imapHeaderEnd(data); end >= 0 {
				if wantHeader {
					return data[:end], nil
				}
				return readRemainingIMAPSectionText(data[end:], reader)
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) {
				if wantHeader {
					return data, nil
				}
				return nil, nil
			}
			return nil, err
		}
	}
}

func readRemainingIMAPSectionText(prefix []byte, reader io.Reader) ([]byte, error) {
	if len(prefix) > maxIMAPSearchLiteralBytes {
		return nil, fmt.Errorf("imap text literal exceeds limit")
	}
	remainingLimit := maxIMAPSearchLiteralBytes - len(prefix)
	rest, err := io.ReadAll(io.LimitReader(reader, int64(remainingLimit)+1))
	if err != nil {
		return nil, err
	}
	if len(rest) > remainingLimit {
		return nil, fmt.Errorf("imap text literal exceeds limit")
	}
	return append(prefix, rest...), nil
}

func imapHeaderEnd(value []byte) int {
	if idx := bytes.Index(value, []byte("\r\n\r\n")); idx >= 0 {
		return idx + 4
	}
	if idx := bytes.Index(value, []byte("\n\n")); idx >= 0 {
		return idx + 2
	}
	return -1
}

func imapFetchRequestsEnvelope(items []string) bool {
	return imapFetchRequestsToken(items, "ENVELOPE")
}

func imapFetchRequestsInternalDate(items []string) bool {
	return imapFetchRequestsToken(items, "INTERNALDATE")
}

func imapFetchRequestsModSeq(items []string) bool {
	return imapFetchRequestsToken(items, "MODSEQ")
}

func imapFetchChangedSince(items []string) (uint64, bool, bool) {
	var threshold uint64
	found := false
	for i := 0; i < len(items); i++ {
		token := strings.ToUpper(strings.TrimSpace(items[i]))
		if !strings.Contains(token, "CHANGEDSINCE") {
			continue
		}
		if found || token != "(CHANGEDSINCE" || i+1 >= len(items) {
			return 0, false, false
		}
		valueToken := items[i+1]
		if !strings.HasSuffix(valueToken, ")") || strings.HasSuffix(valueToken, "))") {
			return 0, false, false
		}
		value := strings.TrimSuffix(valueToken, ")")
		modseq, ok := parseIMAPModSeqValue(value)
		if !ok {
			return 0, false, false
		}
		threshold = modseq
		found = true
		i++
	}
	return threshold, found, true
}

func imapFetchDataItemParenthesesValid(items []string) bool {
	for _, item := range items {
		token := strings.TrimSpace(item)
		if strings.HasPrefix(token, "((") || strings.HasSuffix(token, "))") {
			return false
		}
	}
	return true
}

func imapFetchDataItemsSyntaxError(items []string) (string, bool) {
	if _, _, ok := imapFetchChangedSince(items); !ok {
		return "FETCH CHANGEDSINCE modifier is invalid", true
	}
	if !imapFetchDataItemOuterWhitespaceValid(items) {
		return "FETCH data item list is invalid", true
	}
	if !imapFetchDataItemParenthesesValid(items) {
		return "FETCH data item list is invalid", true
	}
	if !imapFetchMacroUsageValid(items) {
		return "FETCH macro is invalid", true
	}
	if !imapFetchHeaderFieldListsValid(imapExpandFetchItems(items)) {
		return "FETCH header field list is invalid", true
	}
	if !imapFetchDataItemsSupported(imapExpandFetchItems(items)) {
		return "FETCH data item is unsupported", true
	}
	return "", false
}

func imapFetchDataItemOuterWhitespaceValid(items []string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) != item {
			return false
		}
	}
	return true
}

func imapFetchNormalizedTokens(items []string) []string {
	tokens := make([]string, 0, len(items))
	for _, item := range items {
		imapEachNormalizedFetchToken(item, func(token string) bool {
			if token != "" {
				tokens = append(tokens, token)
			}
			return true
		})
	}
	return tokens
}

func imapFetchDataItemsSupported(items []string) bool {
	for i := 0; i < len(items); i++ {
		token := imapFetchToken(items[i])
		if token == "" {
			continue
		}
		if token == "CHANGEDSINCE" {
			i++
			continue
		}
		if imapFetchHeaderFieldSectionStart(token) {
			end, ok := imapFetchHeaderFieldSectionEnd(items, i)
			if !ok {
				return false
			}
			i = end
			continue
		}
		if imapFetchDataItemTokenSupported(token) {
			continue
		}
		return false
	}
	return true
}

func imapFetchToken(item string) string {
	return strings.Trim(strings.ToUpper(strings.TrimSpace(item)), "()")
}

func imapEachNormalizedFetchToken(item string, visit func(token string) bool) {
	token := strings.TrimSpace(item)
	if token == "" {
		return
	}
	token = strings.Trim(token, "()")
	start := -1
	for i := 0; i < len(token); i++ {
		switch token[i] {
		case ' ', '\t', '\r', '\n':
			if start >= 0 {
				if !visit(strings.ToUpper(token[start:i])) {
					return
				}
				start = -1
			}
		default:
			if start < 0 {
				start = i
			}
		}
	}
	if start >= 0 {
		if !visit(strings.ToUpper(token[start:])) {
			return
		}
	}
}

func imapFetchHeaderFieldSectionStart(token string) bool {
	for _, prefix := range []string{"BODY.PEEK[", "BODY["} {
		section, ok := strings.CutPrefix(token, prefix)
		if !ok {
			continue
		}
		for _, marker := range []string{"HEADER.FIELDS.NOT", "HEADER.FIELDS"} {
			if strings.HasPrefix(section, marker) {
				return true
			}
			markerIndex := strings.Index(section, "."+marker)
			if markerIndex <= 0 {
				continue
			}
			if _, ok := parseIMAPMIMEPartPath(section[:markerIndex]); ok {
				return true
			}
		}
	}
	return false
}

func imapFetchHeaderFieldSectionEnd(items []string, start int) (int, bool) {
	for i := start; i < len(items); i++ {
		token := strings.ToUpper(strings.TrimSpace(items[i]))
		closeIdx := strings.Index(token, ")]")
		if closeIdx < 0 {
			continue
		}
		suffix := strings.Trim(token[closeIdx+2:], ")")
		if suffix == "" {
			return i, true
		}
		if strings.HasPrefix(suffix, "<") {
			_, ok := imapParsePartialBodyToken(suffix)
			return i, ok
		}
		return i, false
	}
	return 0, false
}

func imapFetchDataItemTokenSupported(token string) bool {
	switch token {
	case "FLAGS", "INTERNALDATE", "RFC822.SIZE", "ENVELOPE", "BODY", "BODYSTRUCTURE", "UID", "MODSEQ":
		return true
	case "RFC822", "RFC822.HEADER", "RFC822.TEXT":
		return true
	case "BODY[]", "BODY.PEEK[]", "BODY[HEADER]", "BODY.PEEK[HEADER]", "BODY[TEXT]", "BODY.PEEK[TEXT]":
		return true
	}
	switch {
	case strings.HasPrefix(token, "BODY[]<") || strings.HasPrefix(token, "BODY.PEEK[]<") || strings.HasPrefix(token, "RFC822<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "BODY[HEADER]<") || strings.HasPrefix(token, "BODY.PEEK[HEADER]<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "BODY[TEXT]<") || strings.HasPrefix(token, "BODY.PEEK[TEXT]<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	case strings.HasPrefix(token, "RFC822.HEADER<") || strings.HasPrefix(token, "RFC822.TEXT<"):
		_, ok := imapParsePartialBodyToken(token)
		return ok
	}
	_, ok := imapParseMIMEPartRequestToken(token)
	return ok
}

func imapFetchMacroUsageValid(items []string) bool {
	tokens := imapFetchNormalizedTokens(items)
	for _, token := range tokens {
		switch token {
		case "FAST", "ALL", "FULL":
			return len(tokens) == 1 && strings.EqualFold(strings.TrimSpace(strings.Join(items, " ")), token)
		}
	}
	return true
}

func imapFetchRequestsBodyStructure(items []string) bool {
	return imapFetchRequestsToken(items, "BODYSTRUCTURE")
}

func imapFetchRequestsBodyAttribute(items []string) bool {
	return imapFetchRequestsToken(items, "BODY")
}

func imapFetchRequestsToken(items []string, want string) bool {
	for _, token := range imapFetchNormalizedTokens(items) {
		if token == want {
			return true
		}
	}
	return false
}

func imapFetchAttributes(summary MessageSummary, includeEnvelope bool, includeInternalDate bool, includeModSeq bool, includeBody bool, includeBodyStructure bool, bodyAttribute string, bodyStructure string) []string {
	attributes := []string{
		"UID " + strconv.FormatUint(uint64(summary.UID), 10),
		"FLAGS " + imapFlagList(summary.Flags.IMAPFlags()),
		"RFC822.SIZE " + strconv.FormatUint(uint64(summary.Size), 10),
	}
	if includeInternalDate {
		attributes = append(attributes, "INTERNALDATE "+imapQuotedString(imapInternalDate(summary.InternalDate)))
	}
	if includeEnvelope {
		attributes = append(attributes, "ENVELOPE "+imapEnvelope(summary))
	}
	if includeModSeq {
		attributes = append(attributes, "MODSEQ ("+strconv.FormatUint(summary.ModSeq, 10)+")")
	}
	if includeBody {
		if bodyAttribute == "" {
			bodyAttribute = imapBody(summary)
		}
		attributes = append(attributes, "BODY "+bodyAttribute)
	}
	if includeBodyStructure {
		if bodyStructure == "" {
			bodyStructure = imapBodyStructure(summary)
		}
		attributes = append(attributes, "BODYSTRUCTURE "+bodyStructure)
	}
	return attributes
}

func imapInternalDate(value time.Time) string {
	if value.IsZero() {
		value = time.Unix(0, 0).UTC()
	}
	return value.Format("02-Jan-2006 15:04:05 -0700")
}

func imapEnvelope(summary MessageSummary) string {
	envelope := summary.Envelope
	date := envelope.Date
	if date.IsZero() {
		date = summary.InternalDate
	}
	sender := envelope.Sender
	if len(sender) == 0 {
		sender = envelope.From
	}
	replyTo := envelope.ReplyTo
	if len(replyTo) == 0 {
		replyTo = envelope.From
	}
	return "(" + strings.Join([]string{
		imapEnvelopeNString(imapEnvelopeDate(date)),
		imapEnvelopeNString(envelope.Subject),
		imapAddressList(envelope.From),
		imapAddressList(sender),
		imapAddressList(replyTo),
		imapAddressList(envelope.To),
		imapAddressList(envelope.Cc),
		imapAddressList(envelope.Bcc),
		imapEnvelopeNString(envelope.InReplyTo),
		imapEnvelopeNString(envelope.MessageID),
	}, " ") + ")"
}

func imapEnvelopeDate(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC1123Z)
}

func imapAddressList(addresses []Address) string {
	if len(addresses) == 0 {
		return "NIL"
	}
	parts := make([]string, 0, min(len(addresses), maxIMAPEnvelopeAddressCount))
	for _, address := range addresses {
		if !imapEnvelopeAddressRenderable(address) {
			continue
		}
		parts = append(parts, "("+strings.Join([]string{
			imapEnvelopeNString(address.Name),
			"NIL",
			imapEnvelopeNString(address.Mailbox),
			imapEnvelopeNString(address.Host),
		}, " ")+")")
		if len(parts) == maxIMAPEnvelopeAddressCount {
			break
		}
	}
	if len(parts) == 0 {
		return "NIL"
	}
	return "(" + strings.Join(parts, " ") + ")"
}

func imapEnvelopeAddressRenderable(address Address) bool {
	return strings.TrimSpace(address.Mailbox) != "" && strings.TrimSpace(address.Host) != ""
}

func imapEnvelopeNString(value string) string {
	value = imapBodyMetadataText(value)
	if value == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapNString(value string) string {
	if strings.TrimSpace(value) == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapBodyStructure(summary MessageSummary) string {
	return imapBodyStructureFromHeader(summary, nil)
}

func imapBodyStructureFromMIMEStructure(summary MessageSummary, structure messageparse.MIMEStructure) string {
	if structure.Root.MediaType == "" {
		return imapBodyStructure(summary)
	}
	return imapMIMEPartBody(structure.Root, maxInt64(summary.Size, 0), true)
}

func imapBodyStructureFromHeader(summary MessageSummary, header []byte) string {
	return imapBodyFromHeaderExtended(summary, header, true)
}

func imapBody(summary MessageSummary) string {
	return imapBodyFromHeader(summary, nil)
}

func imapBodyFromMIMEStructure(summary MessageSummary, structure messageparse.MIMEStructure) string {
	if structure.Root.MediaType == "" {
		return imapBody(summary)
	}
	return imapMIMEPartBody(structure.Root, maxInt64(summary.Size, 0), false)
}

func imapBodyFromHeader(summary MessageSummary, header []byte) string {
	return imapBodyFromHeaderExtended(summary, header, false)
}

func imapMIMEPartBody(part messageparse.MIMEPart, fallbackSize int64, extended bool) string {
	if part.MediaType == "MULTIPART" {
		childBodies := make([]string, 0, len(part.Parts)+5)
		for _, child := range part.Parts {
			childBodies = append(childBodies, imapMIMEPartBody(child, child.Size, extended))
		}
		if len(childBodies) == 0 {
			return imapBodyFromHeaderExtended(MessageSummary{Size: fallbackSize}, nil, extended)
		}
		_, mediaSubtype := imapMIMETypePair("MULTIPART", part.MediaSubtype, "MULTIPART", "MIXED")
		childBodies = append(childBodies, imapQuotedString(mediaSubtype))
		if extended {
			childBodies = append(childBodies, imapMIMEBodyParameterList(part.Params), "NIL", "NIL", "NIL")
		}
		return "(" + strings.Join(childBodies, " ") + ")"
	}
	return imapMIMESinglePartBody(part, fallbackSize, extended)
}

func imapMIMESinglePartBody(part messageparse.MIMEPart, fallbackSize int64, extended bool) string {
	mediaType, mediaSubtype := imapMIMETypePair(part.MediaType, part.MediaSubtype, "TEXT", "PLAIN")
	size := part.Size
	if size == 0 && fallbackSize > 0 {
		size = fallbackSize
	}
	fields := []string{
		imapQuotedString(mediaType),
		imapQuotedString(mediaSubtype),
		imapMIMEBodyParameterList(part.Params),
		imapBodyMetadataNString(part.ContentID),
		imapBodyMetadataNString(part.Description),
		imapQuotedString(imapMIMEToken(part.Encoding, "7BIT")),
		strconv.FormatInt(maxInt64(size, 0), 10),
	}
	if mediaType == "MESSAGE" && mediaSubtype == "RFC822" {
		fields = append(fields, imapMIMEEnvelope(part.Envelope), imapMIMEMessageBody(part, extended), strconv.FormatInt(maxInt64(part.Lines, 0), 10))
	} else if mediaType == "TEXT" {
		lines := part.Lines
		if lines == 0 && size > 0 {
			lines = 1
		}
		fields = append(fields, strconv.FormatInt(lines, 10))
	}
	if extended {
		fields = append(fields, "NIL", imapMIMEBodyDisposition(part), "NIL", "NIL")
	}
	return "(" + strings.Join(fields, " ") + ")"
}

func imapMIMEEnvelope(envelope messageparse.MIMEEnvelope) string {
	return imapEnvelope(MessageSummary{
		InternalDate: envelope.Date,
		Envelope: Envelope{
			Date:      envelope.Date,
			Subject:   envelope.Subject,
			From:      imapMIMEEnvelopeAddresses(envelope.From),
			Sender:    imapMIMEEnvelopeAddresses(envelope.Sender),
			ReplyTo:   imapMIMEEnvelopeAddresses(envelope.ReplyTo),
			To:        imapMIMEEnvelopeAddresses(envelope.To),
			Cc:        imapMIMEEnvelopeAddresses(envelope.Cc),
			Bcc:       imapMIMEEnvelopeAddresses(envelope.Bcc),
			InReplyTo: envelope.InReplyTo,
			MessageID: envelope.MessageID,
		},
	})
}

func imapMIMEEnvelopeAddresses(addresses []messageparse.Address) []Address {
	if len(addresses) == 0 {
		return nil
	}
	out := make([]Address, 0, len(addresses))
	for _, address := range addresses {
		mailbox, host, ok := strings.Cut(address.Address, "@")
		if !ok {
			continue
		}
		out = append(out, Address{Name: address.Name, Mailbox: mailbox, Host: host})
	}
	return out
}

func imapMIMEMessageBody(part messageparse.MIMEPart, extended bool) string {
	if len(part.Parts) > 0 {
		child := part.Parts[0]
		return imapMIMEPartBody(child, child.Size, extended)
	}
	return imapBodyFromHeaderExtended(MessageSummary{Size: part.Size}, nil, extended)
}

func imapMIMEBodyDisposition(part messageparse.MIMEPart) string {
	disposition := imapMIMEToken(part.Disposition, "")
	if disposition == "" {
		return "NIL"
	}
	return "(" + imapQuotedString(disposition) + " " + imapMIMEBodyParameterList(part.DispositionParams) + ")"
}

func imapBodyMetadataNString(value string) string {
	value = imapBodyMetadataText(value)
	if value == "" {
		return "NIL"
	}
	return imapQuotedString(value)
}

func imapBodyMetadataText(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > maxIMAPBodyMetadataTextBytes {
		value = value[:maxIMAPBodyMetadataTextBytes]
		for !utf8.ValidString(value) && len(value) > 0 {
			value = value[:len(value)-1]
		}
	}
	return value
}

func imapBodyFromHeaderExtended(summary MessageSummary, header []byte, extended bool) string {
	metadata := imapBodyMetadataFromHeader(header)
	lines := int64(0)
	if summary.Size > 0 {
		lines = 1
	}
	size := maxInt64(summary.Size, 0)
	fields := []string{
		imapQuotedString(metadata.mediaType),
		imapQuotedString(metadata.mediaSubtype),
		imapBodyParameterList(metadata.params),
		imapBodyMetadataNString(metadata.id),
		imapBodyMetadataNString(metadata.description),
		imapQuotedString(metadata.encoding),
		strconv.FormatInt(size, 10),
	}
	if metadata.mediaType == "TEXT" {
		fields = append(fields, strconv.FormatInt(lines, 10))
	}
	if extended {
		fields = append(fields, "NIL", "NIL", "NIL", "NIL")
	}
	return "(" + strings.Join(fields, " ") + ")"
}

type imapBodyMetadata struct {
	mediaType    string
	mediaSubtype string
	params       map[string]string
	id           string
	description  string
	encoding     string
}

func imapBodyMetadataFromHeader(header []byte) imapBodyMetadata {
	metadata := imapBodyMetadata{
		mediaType:    "TEXT",
		mediaSubtype: "PLAIN",
		params:       map[string]string{"CHARSET": "UTF-8"},
		encoding:     "7BIT",
	}
	if len(header) == 0 {
		return metadata
	}
	message, err := stdmail.ReadMessage(bytes.NewReader(header))
	if err != nil {
		return metadata
	}
	contentType := strings.TrimSpace(message.Header.Get("Content-Type"))
	if contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err == nil {
			if typ, subtype, ok := imapMediaTypeParts(mediaType); ok {
				if typ == "MULTIPART" {
					return metadata
				}
				metadata.mediaType = typ
				metadata.mediaSubtype = subtype
				metadata.params = imapBodyParams(params)
			}
		}
	}
	if encoding := strings.TrimSpace(message.Header.Get("Content-Transfer-Encoding")); encoding != "" {
		metadata.encoding = imapMIMEToken(encoding, "7BIT")
	}
	metadata.id = strings.TrimSpace(message.Header.Get("Content-ID"))
	metadata.description = strings.TrimSpace(message.Header.Get("Content-Description"))
	return metadata
}

func imapMediaTypeParts(value string) (string, string, bool) {
	typ, subtype, ok := strings.Cut(strings.TrimSpace(value), "/")
	typ = strings.ToUpper(strings.TrimSpace(typ))
	subtype = strings.ToUpper(strings.TrimSpace(subtype))
	if !ok || !imapMIMETokenValid(typ) || !imapMIMETokenValid(subtype) {
		return "", "", false
	}
	return typ, subtype, true
}

func imapBodyParams(params map[string]string) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for key, value := range params {
		key = strings.ToUpper(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if !imapMIMETokenValid(key) || value == "" {
			continue
		}
		out[key] = value
	}
	return out
}

func imapBodyParameterList(params map[string]string) string {
	return imapMIMEBodyParameterList(params)
}

func imapMIMEBodyParameterList(params map[string]string) string {
	if len(params) == 0 {
		return "NIL"
	}
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	canonical := make(map[string]string, len(keys))
	for _, rawKey := range keys {
		key := strings.ToUpper(strings.TrimSpace(rawKey))
		value := imapBodyMetadataText(params[rawKey])
		if !imapMIMETokenValid(key) || value == "" {
			continue
		}
		if _, exists := canonical[key]; exists {
			continue
		}
		canonical[key] = value
	}
	if len(canonical) == 0 {
		return "NIL"
	}
	keys = keys[:0]
	for key := range canonical {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	values := make([]string, 0, len(keys)*2)
	for _, key := range keys {
		values = append(values, imapQuotedString(key), imapQuotedString(canonical[key]))
	}
	return "(" + strings.Join(values, " ") + ")"
}

func imapMIMEToken(value string, fallback string) string {
	value = strings.ToUpper(strings.TrimSpace(value))
	if !imapMIMETokenValid(value) {
		return fallback
	}
	return value
}

func imapMIMETokenValid(value string) bool {
	if value == "" {
		return false
	}
	for i := 0; i < len(value); i++ {
		c := value[i]
		if c <= 0x20 || c >= 0x7f || strings.ContainsRune("()<>@,;:\\\"/[]?=", rune(c)) {
			return false
		}
	}
	return true
}

func imapMIMETypePair(mediaType string, mediaSubtype string, fallbackType string, fallbackSubtype string) (string, string) {
	mediaType = strings.ToUpper(strings.TrimSpace(mediaType))
	mediaSubtype = strings.ToUpper(strings.TrimSpace(mediaSubtype))
	if !imapMIMETokenValid(mediaType) || !imapMIMETokenValid(mediaSubtype) {
		return fallbackType, fallbackSubtype
	}
	return mediaType, mediaSubtype
}

func maxInt64(a int64, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func writeIMAPFetchLine(writer *bufio.Writer, sequenceNumber uint32, attributes string, tail string) error {
	var buf [128]byte
	out := append(buf[:0], "* "...)
	out = strconv.AppendUint(out, uint64(sequenceNumber), 10)
	out = append(out, " FETCH ("...)
	out = append(out, attributes...)
	out = append(out, tail...)
	out = append(out, '\r', '\n')
	_, err := writer.Write(out)
	return err
}

func imapPartialOffsetSuffix(offset uint64) string {
	return "<" + strconv.FormatUint(offset, 10) + ">"
}

