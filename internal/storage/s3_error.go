package storage

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

func s3StatusError(operation string, resp *http.Response) error {
	preview := s3ErrorBodyPreview(resp.Body, 512)
	if resp.StatusCode == http.StatusNotFound {
		if preview == "" {
			return fmt.Errorf("%s s3 object: status %d: %w", operation, resp.StatusCode, os.ErrNotExist)
		}
		return fmt.Errorf("%s s3 object: status %d: %s: %w", operation, resp.StatusCode, preview, os.ErrNotExist)
	}
	if preview == "" {
		return fmt.Errorf("%s s3 object: status %d", operation, resp.StatusCode)
	}
	return fmt.Errorf("%s s3 object: status %d: %s", operation, resp.StatusCode, preview)
}

func validateS3EmptySuccessResponse(operation string, body io.Reader) error {
	if body == nil {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(body, maxS3CopyResponseBytes+1))
	if err != nil {
		return fmt.Errorf("%s s3 object: read success response: %w", operation, err)
	}
	if len(data) > maxS3CopyResponseBytes {
		return fmt.Errorf("%s s3 object: success response body is too large", operation)
	}
	if preview, ok := s3XMLError(data); ok {
		if preview == "" {
			return fmt.Errorf("%s s3 object: embedded error", operation)
		}
		return fmt.Errorf("%s s3 object: embedded error: %s", operation, preview)
	}
	if strings.TrimSpace(string(data)) != "" {
		return fmt.Errorf("%s s3 object: unexpected success response body", operation)
	}
	return nil
}

func validateS3OptionalSuccessETag(operation string, resp *http.Response) error {
	rawETag, present, ok := s3ResponseOptionalSingleHeader(resp, "ETag")
	if !ok {
		return fmt.Errorf("%s s3 object: duplicate etag", operation)
	}
	if !present {
		return nil
	}
	if strings.TrimSpace(rawETag) != rawETag {
		return fmt.Errorf("%s s3 object: invalid etag", operation)
	}
	if cleanS3ETag(rawETag) == "" {
		return fmt.Errorf("%s s3 object: invalid etag", operation)
	}
	return nil
}

func s3ErrorBodyPreview(body io.Reader, maxBytes int64) string {
	if body == nil || maxBytes <= 0 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(body, maxBytes))
	if err != nil {
		return ""
	}
	if preview, ok := s3XMLError(data); ok {
		return preview
	}
	return s3PlainErrorPreview(string(data))
}

func s3XMLErrorPreview(data []byte) string {
	preview, ok := s3XMLError(data)
	if !ok {
		return ""
	}
	return preview
}

func s3XMLError(data []byte) (string, bool) {
	if strings.TrimSpace(string(data)) == "" {
		return "", false
	}
	response, ok := parseS3XMLError(data)
	if !ok {
		return "", false
	}
	if response.XMLName.Local == "" {
		return "", true
	}
	return s3ErrorPreview(response.Code, response.Message, s3ErrorDetail("request-id", response.RequestID), s3ErrorDetail("host-id", response.HostID)), true
}

func parseS3XMLError(data []byte) (s3CopyResponse, bool) {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return s3CopyResponse{}, false
		}
		if err != nil {
			return s3CopyResponse{}, false
		}
		start, ok := token.(xml.StartElement)
		if !ok {
			continue
		}
		if start.Name.Local != "Error" || !s3XMLNamespaceAllowed(start.Name.Space) {
			return s3CopyResponse{}, false
		}
		response, err := parseS3XMLErrorElement(decoder, start)
		if err != nil {
			if _, ok := err.(s3AmbiguousErrorFieldError); ok {
				return s3CopyResponse{}, true
			}
		}
		return response, true
	}
}

type s3AmbiguousErrorFieldError string

func (err s3AmbiguousErrorFieldError) Error() string {
	return string(err)
}

func parseS3XMLErrorElement(decoder *xml.Decoder, root xml.StartElement) (s3CopyResponse, error) {
	response := s3CopyResponse{XMLName: root.Name}
	depth := 1
	current := ""
	seenFields := map[string]struct{}{}
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return response, nil
		}
		if err != nil {
			return response, err
		}
		switch token := token.(type) {
		case xml.StartElement:
			if depth == 2 && current != "" {
				return response, s3AmbiguousErrorFieldError("nested " + current)
			}
			depth++
			if depth == 2 && s3ErrorPreviewField(token.Name.Local) {
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return response, s3AmbiguousErrorFieldError("unexpected namespace " + token.Name.Local)
				}
				if _, ok := seenFields[token.Name.Local]; ok {
					return response, s3AmbiguousErrorFieldError("duplicate " + token.Name.Local)
				}
				seenFields[token.Name.Local] = struct{}{}
				current = token.Name.Local
			}
		case xml.EndElement:
			if depth == 2 {
				current = ""
			}
			if depth == 1 {
				return response, nil
			}
			depth--
		case xml.CharData:
			if depth != 2 {
				continue
			}
			switch current {
			case "Code":
				appendS3ErrorPreviewField(&response.Code, token)
			case "Message":
				appendS3ErrorPreviewField(&response.Message, token)
			case "RequestId":
				appendS3ErrorPreviewField(&response.RequestID, token)
			case "HostId":
				appendS3ErrorPreviewField(&response.HostID, token)
			}
		}
	}
}

func s3ErrorPreviewField(local string) bool {
	switch local {
	case "Code", "Message", "RequestId", "HostId":
		return true
	default:
		return false
	}
}

func appendS3ErrorPreviewField(dst *string, value []byte) {
	if len(*dst) >= maxS3ErrorPreviewFieldBytes {
		return
	}
	remaining := maxS3ErrorPreviewFieldBytes - len(*dst)
	if len(value) > remaining {
		value = value[:remaining]
	}
	*dst += string(value)
}

func s3PlainErrorPreview(value string) string {
	text := strings.ToValidUTF8(value, "")
	text = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

func s3ErrorPreview(code string, message string, details ...string) string {
	parts := make([]string, 0, 2+len(details))
	for _, value := range append([]string{code, message}, details...) {
		value = sanitizeS3ErrorPreviewPart(value)
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ": ")
}

func s3ErrorDetail(name string, value string) string {
	value = sanitizeS3ErrorPreviewPart(value)
	if value == "" {
		return ""
	}
	return name + "=" + value
}

func sanitizeS3ErrorPreviewPart(value string) string {
	return strings.Join(strings.Fields(strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, strings.ToValidUTF8(value, ""))), " ")
}

const maxS3ResponseDrainBytes = 4096

func drainAndCloseS3Body(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxS3ResponseDrainBytes))
	_ = body.Close()
}
