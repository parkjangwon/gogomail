package storage

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (s *S3Store) newRequest(ctx context.Context, method string, objectPath string, body io.Reader) (*http.Request, error) {
	return s.newRequestWithHeaders(ctx, method, objectPath, body, nil)
}

func (s *S3Store) newListRequest(ctx context.Context, prefix string, limit int, cursor string) (*http.Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target := s.bucketURL()
	query := url.Values{}
	query.Set("list-type", "2")
	query.Set("max-keys", strconv.Itoa(limit))
	listPrefix := s.listPrefix(prefix)
	if listPrefix != "" {
		query.Set("prefix", listPrefix)
	}
	if cursor != "" {
		query.Set("continuation-token", cursor)
	}
	target.RawQuery = encodeS3CanonicalQuery(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create s3 list request: %w", err)
	}
	payloadHash, err := s.s3PayloadHash(nil)
	if err != nil {
		return nil, fmt.Errorf("compute s3 list payload hash: %w", err)
	}
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", s.now().UTC().Format("20060102T150405Z"))
	if s.sessionToken != "" {
		req.Header.Set("x-amz-security-token", s.sessionToken)
	}
	s.sign(req)
	return req, nil
}

func (s *S3Store) newRequestWithHeaders(ctx context.Context, method string, objectPath string, body io.Reader, headers map[string]string) (*http.Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	rawObjectPath := objectPath
	objectPath, err := validateS3ObjectPath(objectPath)
	if err != nil {
		return nil, fmt.Errorf("unsafe storage path %q: %w", rawObjectPath, err)
	}
	target := s.objectURL(objectPath)
	req, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("create s3 request: %w", err)
	}
	if err := setS3ContentLength(req, body); err != nil {
		return nil, err
	}
	payloadHash, err := s.s3PayloadHash(body)
	if err != nil {
		return nil, fmt.Errorf("compute s3 payload hash: %w", err)
	}
	req.Header.Set("x-amz-content-sha256", payloadHash)
	req.Header.Set("x-amz-date", s.now().UTC().Format("20060102T150405Z"))
	if s.sessionToken != "" {
		req.Header.Set("x-amz-security-token", s.sessionToken)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	s.sign(req)
	return req, nil
}

func (s *S3Store) objectURL(objectPath string) url.URL {
	target := *s.endpoint
	key := s.key(objectPath)
	escapedKey := escapeS3Key(key)
	basePath := strings.TrimRight(target.Path, "/")
	escapedBasePath := escapeS3BasePath(basePath)
	if s.forcePathStyle {
		target.Path = basePath + "/" + s.bucket + "/" + key
		target.RawPath = escapedBasePath + "/" + escapeS3Segment(s.bucket) + "/" + escapedKey
		return target
	}
	target.Host = s.bucket + "." + target.Host
	target.Path = basePath + "/" + key
	target.RawPath = escapedBasePath + "/" + escapedKey
	return target
}

func (s *S3Store) bucketURL() url.URL {
	target := *s.endpoint
	basePath := strings.TrimRight(target.Path, "/")
	escapedBasePath := escapeS3BasePath(basePath)
	if s.forcePathStyle {
		target.Path = basePath + "/" + s.bucket
		target.RawPath = escapedBasePath + "/" + escapeS3Segment(s.bucket)
		return target
	}
	target.Host = s.bucket + "." + target.Host
	target.Path = basePath
	target.RawPath = escapedBasePath
	return target
}

type s3ContentLengthProvider interface {
	ContentLength() int64
}

func setS3ContentLength(req *http.Request, body io.Reader) error {
	if req == nil || req.Method != http.MethodPut || body == nil || req.ContentLength > 0 {
		return nil
	}
	if provider, ok := body.(s3ContentLengthProvider); ok {
		length := provider.ContentLength()
		if length < 0 {
			return fmt.Errorf("s3 body length is invalid")
		}
		req.ContentLength = length
		return nil
	}
	seeker, ok := body.(io.Seeker)
	if !ok {
		return nil
	}
	current, err := seeker.Seek(0, io.SeekCurrent)
	if err != nil {
		return fmt.Errorf("determine s3 body position: %w", err)
	}
	end, err := seeker.Seek(0, io.SeekEnd)
	if err != nil {
		return fmt.Errorf("determine s3 body length: %w", err)
	}
	if _, err := seeker.Seek(current, io.SeekStart); err != nil {
		return fmt.Errorf("restore s3 body position: %w", err)
	}
	if end < current {
		return fmt.Errorf("s3 body length is invalid")
	}
	req.ContentLength = end - current
	return nil
}

func parseS3ContentLength(value string) (int64, error) {
	if value == "" {
		return -1, fmt.Errorf("stat s3 object: content length is required")
	}
	size, ok := parseS3NonNegativeDecimal(value)
	if !ok {
		return -1, fmt.Errorf("stat s3 object: invalid content length")
	}
	return size, nil
}

func s3StatContentLength(resp *http.Response) (int64, error) {
	value, err := s3ResponseContentLengthHeader(resp)
	if err != nil {
		return -1, fmt.Errorf("stat s3 object: %w", err)
	}
	if value != "" {
		size, err := parseS3ContentLength(value)
		if err != nil {
			return -1, err
		}
		if resp.ContentLength >= 0 && resp.ContentLength != size {
			return -1, fmt.Errorf("stat s3 object: content-length mismatch")
		}
		return size, nil
	}
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}
	return parseS3ContentLength("")
}

func s3GetObjectContentLength(resp *http.Response) (int64, bool, error) {
	value, err := s3ResponseContentLengthHeader(resp)
	if err != nil {
		return -1, false, fmt.Errorf("get s3 object: %w", err)
	}
	if value != "" {
		size, ok := parseS3NonNegativeDecimal(value)
		if !ok {
			return -1, false, fmt.Errorf("get s3 object: invalid content length")
		}
		if resp.ContentLength >= 0 && resp.ContentLength != size {
			return -1, false, fmt.Errorf("get s3 object: content-length mismatch")
		}
		return size, true, nil
	}
	if resp.ContentLength > 0 {
		return resp.ContentLength, true, nil
	}
	return -1, false, nil
}

func s3ResponseContentLengthHeader(resp *http.Response) (string, error) {
	value, ok := s3ResponseSingleHeader(resp, "Content-Length")
	if !ok {
		return "", fmt.Errorf("duplicate content length")
	}
	return value, nil
}

func s3ResponseContentRangeHeader(resp *http.Response) (string, error) {
	value, ok := s3ResponseSingleHeader(resp, "Content-Range")
	if !ok {
		return "", fmt.Errorf("duplicate content-range")
	}
	return value, nil
}

func s3ResponseSingleHeader(resp *http.Response, name string) (string, bool) {
	var found string
	count := 0
	for key, values := range resp.Header {
		if !strings.EqualFold(key, name) {
			continue
		}
		for _, value := range values {
			count++
			if count > 1 {
				return "", false
			}
			found = value
		}
	}
	if count == 1 {
		return found, true
	}
	return "", true
}

func s3ResponseOptionalSingleHeader(resp *http.Response, name string) (string, bool, bool) {
	var found string
	count := 0
	for key, values := range resp.Header {
		if !strings.EqualFold(key, name) {
			continue
		}
		for _, value := range values {
			count++
			if count > 1 {
				return "", true, false
			}
			found = value
		}
	}
	if count == 1 {
		return found, true, true
	}
	return "", false, true
}

func validateS3ContentRange(value string, req RangeRequest) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return fmt.Errorf("get range s3 object: content-range is required")
	}
	if !strings.HasPrefix(strings.ToLower(value), "bytes ") {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	rangeAndSize := strings.TrimSpace(value[len("bytes "):])
	rangePart, sizePart, ok := strings.Cut(rangeAndSize, "/")
	if !ok || strings.TrimSpace(sizePart) == "" {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	startText, endText, ok := strings.Cut(strings.TrimSpace(rangePart), "-")
	if !ok {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	if startText == "" || endText == "" || strings.TrimSpace(startText) != startText || strings.TrimSpace(endText) != endText {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	start, ok := parseS3NonNegativeDecimal(startText)
	if !ok {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	end, ok := parseS3NonNegativeDecimal(endText)
	if !ok || end < start {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	if size := strings.TrimSpace(sizePart); size != "*" {
		if size == "" || size != sizePart {
			return fmt.Errorf("get range s3 object: invalid content-range")
		}
		total, ok := parseS3NonNegativeDecimal(size)
		if !ok || total <= end {
			return fmt.Errorf("get range s3 object: invalid content-range")
		}
	}
	wantEnd := req.Offset + req.Length - 1
	if start != req.Offset || end != wantEnd {
		return fmt.Errorf("get range s3 object: content-range mismatch")
	}
	return nil
}

func validateS3FullRangeCompatibilityResponse(resp *http.Response, req RangeRequest) error {
	contentRange, err := s3ResponseContentRangeHeader(resp)
	if err != nil {
		return fmt.Errorf("get range s3 object: %w", err)
	}
	if strings.TrimSpace(contentRange) != "" {
		if err := validateS3ContentRange(contentRange, req); err != nil {
			return err
		}
		return validateS3RangeContentLength(resp, req)
	}
	if req.Offset != 0 {
		return fmt.Errorf("get range s3 object: status 200 without content-range for non-zero offset")
	}
	size, err := s3RangeResponseContentLength(resp)
	if err != nil {
		return err
	}
	if size != req.Length {
		return fmt.Errorf("get range s3 object: content-length mismatch")
	}
	return nil
}

func s3RangeResponseContentLength(resp *http.Response) (int64, error) {
	value, err := s3ResponseContentLengthHeader(resp)
	if err != nil {
		return -1, fmt.Errorf("get range s3 object: %w", err)
	}
	if value != "" {
		size, ok := parseS3NonNegativeDecimal(value)
		if !ok {
			return -1, fmt.Errorf("get range s3 object: invalid content length")
		}
		if resp.ContentLength >= 0 && resp.ContentLength != size {
			return -1, fmt.Errorf("get range s3 object: content-length mismatch")
		}
		return size, nil
	}
	if resp.ContentLength >= 0 {
		return resp.ContentLength, nil
	}
	return -1, fmt.Errorf("get range s3 object: content length is required")
}

func validateS3RangeContentLength(resp *http.Response, req RangeRequest) error {
	value, err := s3ResponseContentLengthHeader(resp)
	if err != nil {
		return fmt.Errorf("get range s3 object: %w", err)
	}
	if value == "" {
		return nil
	}
	size, ok := parseS3NonNegativeDecimal(value)
	if !ok {
		return fmt.Errorf("get range s3 object: invalid content length")
	}
	if size != req.Length {
		return fmt.Errorf("get range s3 object: content-length mismatch")
	}
	return nil
}

type exactReadCloser struct {
	ctx       context.Context
	reader    io.Reader
	closer    io.Closer
	remaining int64
}

func (r *exactReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	if r.remaining <= 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.remaining {
		p = p[:r.remaining]
	}
	n, err := r.reader.Read(p)
	r.remaining -= int64(n)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	if err == io.EOF && r.remaining > 0 {
		return n, io.ErrUnexpectedEOF
	}
	if err == io.EOF && n > 0 && r.remaining == 0 {
		return n, nil
	}
	return n, err
}

func (r *exactReadCloser) Close() error {
	_, _ = io.Copy(io.Discard, io.LimitReader(r.reader, maxS3ResponseDrainBytes))
	return r.closer.Close()
}

type s3ObjectReadCloser struct {
	ctx  context.Context
	body io.ReadCloser
}

func (r *s3ObjectReadCloser) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	n, err := r.body.Read(p)
	if ctxErr := r.ctx.Err(); ctxErr != nil {
		return n, ctxErr
	}
	return n, err
}

func (r *s3ObjectReadCloser) Close() error {
	_, _ = io.Copy(io.Discard, io.LimitReader(r.body, maxS3ResponseDrainBytes))
	return r.body.Close()
}

func parseHTTPTime(value string) time.Time {
	parsed, err := http.ParseTime(strings.TrimSpace(value))
	if err != nil {
		return time.Time{}
	}
	return parsed
}

func parseS3StatLastModified(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, nil
	}
	parsed, err := http.ParseTime(value)
	if err != nil {
		return time.Time{}, fmt.Errorf("stat s3 object: invalid last-modified")
	}
	return parsed, nil
}

func parseS3ListTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, true
	}
	if strings.TrimSpace(value) != value {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	parsed := parseHTTPTime(value)
	if parsed.IsZero() {
		return time.Time{}, false
	}
	return parsed, true
}

func parseS3StatContentType(value string) (string, error) {
	if strings.TrimSpace(value) != value {
		return "", fmt.Errorf("stat s3 object: invalid content-type")
	}
	contentType := cleanS3MetadataValue(value, maxS3ContentTypeBytes)
	if strings.TrimSpace(value) != "" && (contentType == "" || !s3ContentTypeValueValid(contentType)) {
		return "", fmt.Errorf("stat s3 object: invalid content-type")
	}
	return contentType, nil
}

func parseS3StatETag(value string) (string, error) {
	if strings.TrimSpace(value) != value {
		return "", fmt.Errorf("stat s3 object: invalid etag")
	}
	etag := cleanS3ETag(value)
	if strings.TrimSpace(value) != "" && etag == "" {
		return "", fmt.Errorf("stat s3 object: invalid etag")
	}
	return etag, nil
}
