package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

type S3Options struct {
	Endpoint        string
	Region          string
	Bucket          string
	Prefix          string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ForcePathStyle  bool
	HTTPClient      *http.Client
}

type S3Store struct {
	endpoint        *url.URL
	region          string
	bucket          string
	prefix          string
	accessKeyID     string
	secretAccessKey string
	sessionToken    string
	forcePathStyle  bool
	client          *http.Client
	now             func() time.Time
}

type S3MoveCleanupError struct {
	SourcePath string
	DestPath   string
	Err        error
}

func (e S3MoveCleanupError) Error() string {
	return fmt.Sprintf("move s3 object copied %q to %q but failed to delete source: %v", e.SourcePath, e.DestPath, e.Err)
}

func (e S3MoveCleanupError) Unwrap() error {
	return e.Err
}

const (
	maxS3AccessKeyIDBytes     = 4096
	maxS3SecretAccessKeyBytes = 4096
	maxS3SessionTokenBytes    = 8192
	maxS3ContentTypeBytes     = 1024
	maxS3ETagBytes            = 1024
)

func NewS3Store(opts S3Options) (*S3Store, error) {
	region := strings.TrimSpace(opts.Region)
	if err := ValidateS3Region(region); err != nil {
		return nil, err
	}
	bucket := strings.TrimSpace(opts.Bucket)
	if err := ValidateS3BucketName(bucket); err != nil {
		return nil, err
	}
	accessKeyID := opts.AccessKeyID
	if err := validateS3Credential("s3 access key id", accessKeyID, maxS3AccessKeyIDBytes, true); err != nil {
		return nil, err
	}
	if err := validateS3Credential("s3 secret access key", opts.SecretAccessKey, maxS3SecretAccessKeyBytes, true); err != nil {
		return nil, err
	}
	if err := validateS3Credential("s3 session token", opts.SessionToken, maxS3SessionTokenBytes, false); err != nil {
		return nil, err
	}
	endpointValue := strings.TrimSpace(opts.Endpoint)
	if endpointValue == "" {
		endpointValue = "https://s3." + region + ".amazonaws.com"
	}
	endpoint, err := ValidateS3Endpoint(endpointValue)
	if err != nil {
		return nil, err
	}
	prefix, err := normalizeS3Prefix(opts.Prefix)
	if err != nil {
		return nil, err
	}
	client := opts.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	forcePathStyle := opts.ForcePathStyle || s3BucketNeedsPathStyle(endpoint, bucket)
	return &S3Store{
		endpoint:        endpoint,
		region:          region,
		bucket:          bucket,
		prefix:          prefix,
		accessKeyID:     accessKeyID,
		secretAccessKey: opts.SecretAccessKey,
		sessionToken:    opts.SessionToken,
		forcePathStyle:  forcePathStyle,
		client:          client,
		now:             time.Now,
	}, nil
}

func (s *S3Store) Put(ctx context.Context, objectPath string, body io.Reader) error {
	if body == nil {
		return fmt.Errorf("storage body is required")
	}
	req, err := s.newRequest(ctx, http.MethodPut, objectPath, body)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("put s3 object: %w", err)
	}
	defer drainAndCloseS3Body(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return s3StatusError("put", resp)
	}
	return nil
}

func (s *S3Store) Get(ctx context.Context, objectPath string) (io.ReadCloser, error) {
	req, err := s.newRequest(ctx, http.MethodGet, objectPath, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get s3 object: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		err := s3StatusError("get", resp)
		drainAndCloseS3Body(resp.Body)
		return nil, err
	}
	return &s3ObjectReadCloser{ctx: ctx, body: resp.Body}, nil
}

func (s *S3Store) GetRange(ctx context.Context, objectPath string, rangeReq RangeRequest) (io.ReadCloser, error) {
	validated, err := ValidateRangeRequest(rangeReq)
	if err != nil {
		return nil, err
	}
	end := validated.Offset + validated.Length - 1
	req, err := s.newRequestWithHeaders(ctx, http.MethodGet, objectPath, nil, map[string]string{
		"Range": "bytes=" + strconv.FormatInt(validated.Offset, 10) + "-" + strconv.FormatInt(end, 10),
	})
	if err != nil {
		return nil, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("get s3 object range: %w", err)
	}
	if resp.StatusCode != http.StatusPartialContent {
		err := s3StatusError("get range", resp)
		drainAndCloseS3Body(resp.Body)
		return nil, err
	}
	if err := validateS3ContentRange(resp.Header.Get("Content-Range"), validated); err != nil {
		drainAndCloseS3Body(resp.Body)
		return nil, err
	}
	return &exactReadCloser{ctx: ctx, reader: resp.Body, closer: resp.Body, remaining: validated.Length}, nil
}

func (s *S3Store) Stat(ctx context.Context, objectPath string) (ObjectInfo, error) {
	req, err := s.newRequest(ctx, http.MethodHead, objectPath, nil)
	if err != nil {
		return ObjectInfo{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: %w", err)
	}
	defer drainAndCloseS3Body(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ObjectInfo{}, s3StatusError("stat", resp)
	}
	size := resp.ContentLength
	if size < 0 {
		size, err = parseS3ContentLength(resp.Header.Get("Content-Length"))
		if err != nil {
			return ObjectInfo{}, err
		}
	}
	objectPath, err = ValidateObjectPath(objectPath)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("unsafe storage path %q: %w", objectPath, err)
	}
	return ObjectInfo{
		Path:         objectPath,
		Size:         size,
		ContentType:  cleanS3MetadataValue(resp.Header.Get("Content-Type"), maxS3ContentTypeBytes),
		ETag:         cleanS3ETag(resp.Header.Get("ETag")),
		LastModified: parseHTTPTime(resp.Header.Get("Last-Modified")),
	}, nil
}

func (s *S3Store) Delete(ctx context.Context, objectPath string) error {
	req, err := s.newRequest(ctx, http.MethodDelete, objectPath, nil)
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("delete s3 object: %w", err)
	}
	defer drainAndCloseS3Body(resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return s3StatusError("delete", resp)
	}
	return nil
}

func (s *S3Store) Copy(ctx context.Context, sourcePath string, destPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceObjectPath, err := ValidateObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := ValidateObjectPath(destPath)
	if err != nil {
		return fmt.Errorf("unsafe destination storage path %q: %w", destPath, err)
	}
	if sourceObjectPath == destObjectPath {
		return nil
	}
	req, err := s.newRequestWithHeaders(ctx, http.MethodPut, destObjectPath, nil, map[string]string{
		"x-amz-copy-source": s.copySource(sourceObjectPath),
	})
	if err != nil {
		return err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("copy s3 object: %w", err)
	}
	defer drainAndCloseS3Body(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return s3StatusError("copy", resp)
	}
	if err := validateS3CopyResponse(resp.Body); err != nil {
		return err
	}
	return nil
}

func (s *S3Store) Move(ctx context.Context, sourcePath string, destPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceObjectPath, err := ValidateObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := ValidateObjectPath(destPath)
	if err != nil {
		return fmt.Errorf("unsafe destination storage path %q: %w", destPath, err)
	}
	if sourceObjectPath == destObjectPath {
		return nil
	}
	if err := s.Copy(ctx, sourceObjectPath, destObjectPath); err != nil {
		return fmt.Errorf("copy source storage object for move: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := s.Delete(ctx, sourceObjectPath); err != nil {
		return S3MoveCleanupError{SourcePath: sourceObjectPath, DestPath: destObjectPath, Err: err}
	}
	return nil
}

func (s *S3Store) List(ctx context.Context, opts ListOptions) (ObjectListPage, error) {
	prefix, err := ValidateObjectPrefix(opts.Prefix)
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("unsafe storage prefix %q: %w", opts.Prefix, err)
	}
	cursor, err := ValidateListCursor(opts.Cursor)
	if err != nil {
		return ObjectListPage{}, err
	}
	limit := NormalizeListLimit(opts.Limit)
	req, err := s.newListRequest(ctx, prefix, limit, cursor)
	if err != nil {
		return ObjectListPage{}, err
	}
	resp, err := s.client.Do(req)
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("list s3 objects: %w", err)
	}
	defer drainAndCloseS3Body(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return ObjectListPage{}, s3StatusError("list", resp)
	}
	result, err := decodeS3ListObjects(resp.Body)
	if err != nil {
		return ObjectListPage{}, err
	}
	nextCursor, err := ValidateListCursor(result.NextContinuationToken)
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid continuation token: %w", err)
	}
	if result.IsTruncated && nextCursor == "" {
		return ObjectListPage{}, fmt.Errorf("list s3 objects: truncated response missing continuation token")
	}
	page := ObjectListPage{
		Objects:    make([]ObjectInfo, 0, len(result.Contents)),
		NextCursor: nextCursor,
		HasMore:    result.IsTruncated,
	}
	for _, item := range result.Contents {
		objectPath, ok := s.objectPathFromKey(item.Key)
		if !ok {
			continue
		}
		if item.Size < 0 {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid object size")
		}
		if len(page.Objects) >= limit {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: response contains more objects than requested limit")
		}
		page.Objects = append(page.Objects, ObjectInfo{
			Path:         objectPath,
			Size:         item.Size,
			ETag:         cleanS3ETag(item.ETag),
			LastModified: parseS3ListTime(item.LastModified),
		})
	}
	if !page.HasMore {
		page.NextCursor = ""
	}
	return page, nil
}

func (s *S3Store) Check(ctx context.Context) error {
	objectPath := "health/readiness-" + fmt.Sprintf("%d", s.now().UnixNano()) + ".txt"
	const body = "gogomail storage readiness\n"
	if err := s.Put(ctx, objectPath, strings.NewReader(body)); err != nil {
		return fmt.Errorf("write readiness probe: %w", err)
	}
	readCloser, err := s.Get(ctx, objectPath)
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read readiness probe: %w", err)
	}
	got, readErr := readStorageCheckBody(readCloser, len(body))
	closeErr := readCloser.Close()
	if readErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read readiness probe body: %w", readErr)
	}
	if closeErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("close readiness probe body: %w", closeErr)
	}
	if string(got) != body {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe body mismatch")
	}
	info, err := s.Stat(ctx, objectPath)
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("stat readiness probe: %w", err)
	}
	if info.Path != objectPath || info.Size != int64(len(body)) {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe metadata mismatch")
	}
	rangeCloser, err := s.GetRange(ctx, objectPath, RangeRequest{Offset: 0, Length: int64(len("gogomail"))})
	if err != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("range readiness probe: %w", err)
	}
	rangeGot, rangeReadErr := readStorageCheckBody(rangeCloser, len("gogomail"))
	rangeCloseErr := rangeCloser.Close()
	if rangeReadErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("read range readiness probe body: %w", rangeReadErr)
	}
	if rangeCloseErr != nil {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("close range readiness probe body: %w", rangeCloseErr)
	}
	if string(rangeGot) != "gogomail" {
		_ = s.Delete(ctx, objectPath)
		return fmt.Errorf("readiness probe range body mismatch")
	}
	if err := s.Delete(ctx, objectPath); err != nil {
		return fmt.Errorf("delete readiness probe: %w", err)
	}
	return nil
}

func (s *S3Store) newRequest(ctx context.Context, method string, objectPath string, body io.Reader) (*http.Request, error) {
	return s.newRequestWithHeaders(ctx, method, objectPath, body, nil)
}

func (s *S3Store) newListRequest(ctx context.Context, prefix string, limit int, cursor string) (*http.Request, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target := s.bucketURL()
	query := target.Query()
	query.Set("list-type", "2")
	query.Set("max-keys", strconv.Itoa(limit))
	listPrefix := ""
	if prefix != "" {
		listPrefix = s.key(prefix)
	} else if s.prefix != "" {
		listPrefix = s.prefix + "/"
	}
	if listPrefix != "" {
		query.Set("prefix", listPrefix)
	}
	if cursor != "" {
		query.Set("continuation-token", cursor)
	}
	target.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("create s3 list request: %w", err)
	}
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")
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
	objectPath, err := ValidateObjectPath(objectPath)
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
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")
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

func setS3ContentLength(req *http.Request, body io.Reader) error {
	if req == nil || req.Method != http.MethodPut || body == nil || req.ContentLength > 0 {
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
	value = strings.TrimSpace(value)
	if value == "" {
		return -1, fmt.Errorf("stat s3 object: content length is required")
	}
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil || size < 0 {
		return -1, fmt.Errorf("stat s3 object: invalid content length")
	}
	return size, nil
}

func cleanS3MetadataValue(value string, maxBytes int) string {
	value = strings.TrimSpace(value)
	if value == "" || len(value) > maxBytes || !utf8.ValidString(value) || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
}

func cleanS3ETag(value string) string {
	value = cleanS3MetadataValue(value, maxS3ETagBytes)
	if value == "" {
		return ""
	}
	value = strings.Trim(value, `"`)
	if value == "" || len(value) > maxS3ETagBytes || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	return value
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
	start, err := strconv.ParseInt(strings.TrimSpace(startText), 10, 64)
	if err != nil || start < 0 {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	end, err := strconv.ParseInt(strings.TrimSpace(endText), 10, 64)
	if err != nil || end < start {
		return fmt.Errorf("get range s3 object: invalid content-range")
	}
	if size := strings.TrimSpace(sizePart); size != "*" {
		total, err := strconv.ParseInt(size, 10, 64)
		if err != nil || total <= end {
			return fmt.Errorf("get range s3 object: invalid content-range")
		}
	}
	wantEnd := req.Offset + req.Length - 1
	if start != req.Offset || end != wantEnd {
		return fmt.Errorf("get range s3 object: content-range mismatch")
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

func parseS3ListTime(value string) time.Time {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed
	}
	return parseHTTPTime(value)
}

func (s *S3Store) key(objectPath string) string {
	if s.prefix == "" {
		return objectPath
	}
	return s.prefix + "/" + objectPath
}

func (s *S3Store) objectPathFromKey(key string) (string, bool) {
	if strings.TrimSpace(key) != key {
		return "", false
	}
	if s.prefix != "" {
		prefix := s.prefix + "/"
		if !strings.HasPrefix(key, prefix) {
			return "", false
		}
		key = strings.TrimPrefix(key, prefix)
	}
	if strings.TrimSpace(key) != key {
		return "", false
	}
	objectPath, err := ValidateObjectPath(key)
	if err != nil {
		return "", false
	}
	return objectPath, true
}

func (s *S3Store) copySource(objectPath string) string {
	return "/" + escapeS3Segment(s.bucket) + "/" + escapeS3Key(s.key(objectPath))
}

func s3BucketNeedsPathStyle(endpoint *url.URL, bucket string) bool {
	if endpoint == nil {
		return false
	}
	if endpoint.Scheme == "https" && strings.Contains(bucket, ".") {
		return true
	}
	host := s3EndpointHostname(endpoint)
	return host == "localhost" || net.ParseIP(host) != nil
}

func s3EndpointHostname(endpoint *url.URL) string {
	if endpoint == nil {
		return ""
	}
	host := endpoint.Hostname()
	return strings.Trim(strings.ToLower(host), "[]")
}

func (s *S3Store) sign(req *http.Request) {
	now := req.Header.Get("x-amz-date")
	date := now[:8]
	headers := signedHeaderValues(req)
	canonicalHeaders := canonicalS3Headers(headers)
	signedHeaders := strings.Join(sortedHeaderNames(headers), ";")
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		"UNSIGNED-PAYLOAD",
	}, "\n")
	scope := date + "/" + s.region + "/s3/aws4_request"
	hashedCanonicalRequest := sha256Hex([]byte(canonicalRequest))
	stringToSign := strings.Join([]string{
		"AWS4-HMAC-SHA256",
		now,
		scope,
		hashedCanonicalRequest,
	}, "\n")
	signingKey := s3SigningKey(s.secretAccessKey, date, s.region)
	signature := hex.EncodeToString(hmacSHA256(signingKey, []byte(stringToSign)))
	req.Header.Set("Authorization", "AWS4-HMAC-SHA256 Credential="+s.accessKeyID+"/"+scope+", SignedHeaders="+signedHeaders+", Signature="+signature)
}

func signedHeaderValues(req *http.Request) map[string]string {
	headers := map[string]string{
		"host": req.URL.Host,
	}
	for name, values := range req.Header {
		name = strings.ToLower(name)
		if !strings.HasPrefix(name, "x-amz-") {
			continue
		}
		headers[name] = strings.Join(values, ",")
	}
	return headers
}

func canonicalS3Headers(headers map[string]string) string {
	names := sortedHeaderNames(headers)
	var b strings.Builder
	for _, name := range names {
		b.WriteString(name)
		b.WriteByte(':')
		b.WriteString(strings.Join(strings.Fields(headers[name]), " "))
		b.WriteByte('\n')
	}
	return b.String()
}

func sortedHeaderNames(headers map[string]string) []string {
	names := make([]string, 0, len(headers))
	for name := range headers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func normalizeS3Prefix(prefix string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "", nil
	}
	return ValidateObjectPath(prefix)
}

func ValidateS3BucketName(bucket string) error {
	if len(bucket) < 3 || len(bucket) > 63 {
		return fmt.Errorf("s3 bucket name must be between 3 and 63 characters")
	}
	if strings.ContainsAny(bucket, " /\r\n") {
		return fmt.Errorf("s3 bucket name must not contain whitespace, slashes, or line breaks")
	}
	if bucket[0] == '-' || bucket[0] == '.' || bucket[len(bucket)-1] == '-' || bucket[len(bucket)-1] == '.' {
		return fmt.Errorf("s3 bucket name must start and end with a letter or digit")
	}
	previousDot := false
	for _, r := range bucket {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '.'
		if !valid {
			return fmt.Errorf("s3 bucket name contains unsupported characters")
		}
		if r == '.' {
			if previousDot {
				return fmt.Errorf("s3 bucket name must not contain adjacent dots")
			}
			previousDot = true
			continue
		}
		previousDot = false
	}
	if strings.Contains(bucket, ".-") || strings.Contains(bucket, "-.") {
		return fmt.Errorf("s3 bucket name must not contain dots next to hyphens")
	}
	if net.ParseIP(bucket) != nil && strings.Count(bucket, ".") == 3 {
		return fmt.Errorf("s3 bucket name must not be formatted as an IP address")
	}
	for _, prefix := range []string{"xn--", "sthree-", "amzn-s3-demo-"} {
		if strings.HasPrefix(bucket, prefix) {
			return fmt.Errorf("s3 bucket name must not use reserved prefix %q", prefix)
		}
	}
	for _, suffix := range []string{"-s3alias", "--ol-s3", ".mrap", "--x-s3", "--table-s3"} {
		if strings.HasSuffix(bucket, suffix) {
			return fmt.Errorf("s3 bucket name must not use reserved suffix %q", suffix)
		}
	}
	return nil
}

func s3CredentialContainsWhitespace(value string) bool {
	return strings.ContainsAny(value, " \t\r\n")
}

func validateS3Credential(name string, value string, maxBytes int, required bool) error {
	if value == "" {
		if required {
			return fmt.Errorf("%s is required and must not contain whitespace", name)
		}
		return nil
	}
	if len(value) > maxBytes {
		return fmt.Errorf("%s is too long", name)
	}
	if s3CredentialContainsWhitespace(value) {
		return fmt.Errorf("%s must not contain whitespace", name)
	}
	return nil
}

func ValidateS3Endpoint(endpointValue string) (*url.URL, error) {
	endpointValue = strings.TrimSpace(endpointValue)
	if endpointValue == "" {
		return nil, fmt.Errorf("s3 endpoint is required")
	}
	if strings.ContainsAny(endpointValue, "\r\n") {
		return nil, fmt.Errorf("s3 endpoint must not contain line breaks")
	}
	endpoint, err := url.Parse(endpointValue)
	if err != nil {
		return nil, fmt.Errorf("parse s3 endpoint: %w", err)
	}
	if endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		return nil, fmt.Errorf("s3 endpoint must use http or https")
	}
	if endpoint.Host == "" {
		return nil, fmt.Errorf("s3 endpoint host is required")
	}
	if endpoint.User != nil {
		return nil, fmt.Errorf("s3 endpoint must not contain user info")
	}
	if endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return nil, fmt.Errorf("s3 endpoint must not contain query or fragment")
	}
	escapedPath := strings.ToLower(endpoint.EscapedPath())
	if strings.Contains(escapedPath, "%2f") || strings.Contains(escapedPath, "%5c") {
		return nil, fmt.Errorf("s3 endpoint path must not contain encoded path separators")
	}
	if err := validateS3EndpointPath(endpoint.Path); err != nil {
		return nil, err
	}
	return endpoint, nil
}

func validateS3EndpointPath(endpointPath string) error {
	if endpointPath == "" || endpointPath == "/" {
		return nil
	}
	relativePath := strings.TrimSuffix(strings.TrimPrefix(endpointPath, "/"), "/")
	if relativePath == "" {
		return fmt.Errorf("s3 endpoint path must be canonical")
	}
	if _, err := ValidateObjectPath(relativePath); err != nil {
		return fmt.Errorf("s3 endpoint path: %w", err)
	}
	return nil
}

func ValidateS3Region(region string) error {
	if region == "" {
		return fmt.Errorf("s3 region is required")
	}
	if len(region) > 128 {
		return fmt.Errorf("s3 region is too long")
	}
	if strings.ContainsAny(region, " /\r\n\t") {
		return fmt.Errorf("s3 region must not contain whitespace, slashes, or line breaks")
	}
	for _, r := range region {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-'
		if !valid {
			return fmt.Errorf("s3 region contains unsupported characters")
		}
	}
	return nil
}

func escapeS3Key(key string) string {
	segments := strings.Split(key, "/")
	for i, segment := range segments {
		segments[i] = escapeS3Segment(segment)
	}
	return strings.Join(segments, "/")
}

func escapeS3BasePath(basePath string) string {
	basePath = strings.Trim(basePath, "/")
	if basePath == "" {
		return ""
	}
	return "/" + escapeS3Key(basePath)
}

func escapeS3Segment(segment string) string {
	return strings.ReplaceAll(url.PathEscape(segment), "+", "%2B")
}

func s3SigningKey(secret string, date string, region string) []byte {
	kDate := hmacSHA256([]byte("AWS4"+secret), []byte(date))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte("s3"))
	return hmacSHA256(kService, []byte("aws4_request"))
}

func hmacSHA256(key []byte, value []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(value)
	return mac.Sum(nil)
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

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

func s3ErrorBodyPreview(body io.Reader, maxBytes int64) string {
	if body == nil || maxBytes <= 0 {
		return ""
	}
	data, err := io.ReadAll(io.LimitReader(body, maxBytes))
	if err != nil {
		return ""
	}
	text := strings.ToValidUTF8(string(data), "")
	text = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return ' '
		}
		return r
	}, text)
	return strings.Join(strings.Fields(text), " ")
}

type s3ListObjectsResult struct {
	XMLName               xml.Name              `xml:"ListBucketResult"`
	IsTruncated           bool                  `xml:"IsTruncated"`
	NextContinuationToken string                `xml:"NextContinuationToken"`
	Contents              []s3ListObjectContent `xml:"Contents"`
}

type s3ListObjectContent struct {
	Key          string `xml:"Key"`
	Size         int64  `xml:"Size"`
	ETag         string `xml:"ETag"`
	LastModified string `xml:"LastModified"`
}

type s3CopyResponse struct {
	XMLName xml.Name
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

const maxS3ListResponseBytes = 4 << 20
const maxS3CopyResponseBytes = 1 << 20

func decodeS3ListObjects(body io.Reader) (s3ListObjectsResult, error) {
	if body == nil {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: response body is required")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxS3ListResponseBytes+1))
	if err != nil {
		return s3ListObjectsResult{}, fmt.Errorf("read s3 list response: %w", err)
	}
	if len(data) > maxS3ListResponseBytes {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: response body is too large")
	}
	var result s3ListObjectsResult
	if err := xml.Unmarshal(data, &result); err != nil {
		return s3ListObjectsResult{}, fmt.Errorf("decode s3 list response: %w", err)
	}
	return result, nil
}

func validateS3CopyResponse(body io.Reader) error {
	if body == nil {
		return fmt.Errorf("copy s3 object: response body is required")
	}
	data, err := io.ReadAll(io.LimitReader(body, maxS3CopyResponseBytes+1))
	if err != nil {
		return fmt.Errorf("read s3 copy response: %w", err)
	}
	if len(data) > maxS3CopyResponseBytes {
		return fmt.Errorf("copy s3 object: response body is too large")
	}
	if strings.TrimSpace(string(data)) == "" {
		return fmt.Errorf("copy s3 object: response body is required")
	}
	var response s3CopyResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("decode s3 copy response: %w", err)
	}
	switch response.XMLName.Local {
	case "CopyObjectResult":
		return nil
	case "Error":
		preview := s3ErrorPreview(response.Code, response.Message)
		if preview == "" {
			return fmt.Errorf("copy s3 object: embedded error")
		}
		return fmt.Errorf("copy s3 object: embedded error: %s", preview)
	default:
		return fmt.Errorf("copy s3 object: unexpected response %q", response.XMLName.Local)
	}
}

func s3ErrorPreview(code string, message string) string {
	parts := make([]string, 0, 2)
	for _, value := range []string{code, message} {
		value = strings.Join(strings.Fields(strings.Map(func(r rune) rune {
			if r < 0x20 || r == 0x7f {
				return ' '
			}
			return r
		}, strings.ToValidUTF8(value, ""))), " ")
		if value != "" {
			parts = append(parts, value)
		}
	}
	return strings.Join(parts, ": ")
}

const maxS3ResponseDrainBytes = 4096

func drainAndCloseS3Body(body io.ReadCloser) {
	if body == nil {
		return
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(body, maxS3ResponseDrainBytes))
	_ = body.Close()
}

var _ Store = (*S3Store)(nil)
