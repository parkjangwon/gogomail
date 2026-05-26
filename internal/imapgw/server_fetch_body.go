package imapgw

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	stdmail "net/mail"
)

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
