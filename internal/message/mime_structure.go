package message

import (
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"strings"

	gomessage "github.com/emersion/go-message"
)

type MIMEStructureOptions struct {
	MaxHeaderBytes   int64
	MaxParts         int
	MaxDepth         int
	MaxMetadataBytes int
}

type MIMEStructure struct {
	Root           MIMEPart
	PartsTruncated bool
}

type MIMEPart struct {
	MediaType         string
	MediaSubtype      string
	Params            map[string]string
	ContentID         string
	Description       string
	Encoding          string
	Disposition       string
	DispositionParams map[string]string
	Size              int64
	Lines             int64
	Parts             []MIMEPart
}

type mimeStructureState struct {
	opts           MIMEStructureOptions
	partsSeen      int
	partsTruncated bool
}

type mimeHeader interface {
	Get(string) string
}

func ParseMIMEStructure(r io.Reader, opts MIMEStructureOptions) (MIMEStructure, error) {
	opts = normalizeMIMEStructureOptions(opts)
	entity, err := gomessage.ReadWithOptions(r, &gomessage.ReadOptions{MaxHeaderBytes: opts.MaxHeaderBytes})
	if err != nil && !gomessage.IsUnknownCharset(err) {
		return MIMEStructure{}, fmt.Errorf("create mime structure reader: %w", err)
	}
	state := &mimeStructureState{opts: opts}
	root, err := parseMIMEPartStructure(&entity.Header, entity.Body, state, 0)
	if err != nil {
		return MIMEStructure{}, err
	}
	return MIMEStructure{Root: root, PartsTruncated: state.partsTruncated}, nil
}

func normalizeMIMEStructureOptions(opts MIMEStructureOptions) MIMEStructureOptions {
	if opts.MaxHeaderBytes <= 0 {
		opts.MaxHeaderBytes = 1 << 20
	}
	if opts.MaxParts <= 0 {
		opts.MaxParts = 10000
	}
	if opts.MaxDepth <= 0 {
		opts.MaxDepth = 32
	}
	if opts.MaxMetadataBytes <= 0 {
		opts.MaxMetadataBytes = defaultMaxMetadataBytes
	}
	return opts
}

func parseMIMEPartStructure(header mimeHeader, body io.Reader, state *mimeStructureState, depth int) (MIMEPart, error) {
	part := mimePartFromHeader(header, state.opts)
	state.partsSeen++
	if state.partsSeen > state.opts.MaxParts || depth > state.opts.MaxDepth {
		state.partsTruncated = true
		_, _ = io.Copy(io.Discard, body)
		return part, nil
	}
	if part.MediaType == "MULTIPART" {
		boundary := strings.TrimSpace(part.Params["boundary"])
		if boundary == "" {
			counter := &mimeBodyCounter{}
			if _, err := io.Copy(counter, body); err != nil {
				return MIMEPart{}, fmt.Errorf("read malformed multipart body: %w", err)
			}
			part.Size = counter.Size
			return part, nil
		}
		reader := multipart.NewReader(body, boundary)
		for {
			if state.partsSeen >= state.opts.MaxParts {
				state.partsTruncated = true
				break
			}
			child, err := reader.NextRawPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				return MIMEPart{}, fmt.Errorf("read multipart child: %w", err)
			}
			childPart, err := parseMIMEPartStructure(child.Header, child, state, depth+1)
			_ = child.Close()
			if err != nil {
				return MIMEPart{}, err
			}
			part.Parts = append(part.Parts, childPart)
			if state.partsTruncated {
				break
			}
		}
		return part, nil
	}
	if part.MediaType == "MESSAGE" && part.MediaSubtype == "RFC822" {
		counter := &mimeBodyCounter{}
		countedBody := io.TeeReader(body, counter)
		entity, err := gomessage.ReadWithOptions(countedBody, &gomessage.ReadOptions{MaxHeaderBytes: state.opts.MaxHeaderBytes})
		if err != nil && !gomessage.IsUnknownCharset(err) {
			if _, copyErr := io.Copy(io.Discard, countedBody); copyErr != nil {
				return MIMEPart{}, fmt.Errorf("read malformed message/rfc822 body: %w", copyErr)
			}
			part.Size = counter.Size
			part.Lines = counter.Lines()
			return part, nil
		}
		childPart, err := parseMIMEPartStructure(&entity.Header, entity.Body, state, depth+1)
		if err != nil {
			return MIMEPart{}, err
		}
		part.Parts = append(part.Parts, childPart)
		part.Size = counter.Size
		part.Lines = counter.Lines()
		return part, nil
	}
	counter := &mimeBodyCounter{}
	if _, err := io.Copy(counter, body); err != nil {
		return MIMEPart{}, fmt.Errorf("read mime part body: %w", err)
	}
	part.Size = counter.Size
	if part.MediaType == "TEXT" {
		part.Lines = counter.Lines()
	}
	return part, nil
}

func mimePartFromHeader(header mimeHeader, opts MIMEStructureOptions) MIMEPart {
	part := MIMEPart{
		MediaType:    "TEXT",
		MediaSubtype: "PLAIN",
		Params:       map[string]string{"charset": "UTF-8"},
		Encoding:     "7BIT",
	}
	if header == nil {
		return part
	}
	if contentType := strings.TrimSpace(header.Get("Content-Type")); contentType != "" {
		mediaType, params, err := mime.ParseMediaType(contentType)
		if err == nil {
			if typ, subtype, ok := mimeMediaTypeParts(mediaType); ok {
				part.MediaType = typ
				part.MediaSubtype = subtype
				part.Params = cleanMIMEParams(params, opts.MaxMetadataBytes)
			}
		}
	}
	if encoding := cleanMIMEToken(header.Get("Content-Transfer-Encoding")); encoding != "" {
		part.Encoding = strings.ToUpper(encoding)
	}
	part.ContentID, _ = sanitizeHeaderMetadata(header.Get("Content-ID"), opts.MaxMetadataBytes, false)
	part.Description, _ = sanitizeHeaderMetadata(header.Get("Content-Description"), opts.MaxMetadataBytes, false)
	if disposition := strings.TrimSpace(header.Get("Content-Disposition")); disposition != "" {
		value, params, err := mime.ParseMediaType(disposition)
		if err == nil {
			part.Disposition = strings.ToUpper(cleanMIMEToken(value))
			part.DispositionParams = cleanMIMEParams(params, opts.MaxMetadataBytes)
		}
	}
	return part
}

func mimeMediaTypeParts(value string) (string, string, bool) {
	typ, subtype, ok := strings.Cut(strings.TrimSpace(value), "/")
	typ = strings.ToUpper(cleanMIMEToken(typ))
	subtype = strings.ToUpper(cleanMIMEToken(subtype))
	if !ok || typ == "" || subtype == "" {
		return "", "", false
	}
	return typ, subtype, true
}

func cleanMIMEToken(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, " \t\r\n") {
		return ""
	}
	for _, r := range value {
		if r < 0x20 || r == 0x7f {
			return ""
		}
	}
	return value
}

func cleanMIMEParams(params map[string]string, maxBytes int) map[string]string {
	if len(params) == 0 {
		return nil
	}
	out := make(map[string]string, len(params))
	for key, value := range params {
		key = strings.ToLower(cleanMIMEToken(key))
		if key == "" {
			continue
		}
		value, _ = sanitizeHeaderMetadata(value, maxBytes, false)
		if value != "" {
			out[key] = value
		}
	}
	return out
}

type mimeBodyCounter struct {
	Size     int64
	lines    int64
	lastByte byte
}

func (c *mimeBodyCounter) Write(p []byte) (int, error) {
	c.Size += int64(len(p))
	for _, b := range p {
		if b == '\n' {
			c.lines++
		}
		c.lastByte = b
	}
	return len(p), nil
}

func (c *mimeBodyCounter) Lines() int64 {
	if c.Size == 0 {
		return 0
	}
	if c.lastByte != '\n' {
		return c.lines + 1
	}
	return c.lines
}
