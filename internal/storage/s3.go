package storage

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"io"
	"mime"
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
	s3XMLNamespace            = "http://s3.amazonaws.com/doc/2006-03-01/"
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
	if err := validateS3UnsupportedRequestChargedHeader("put", resp); err != nil {
		return err
	}
	if err := validateS3OptionalSuccessETag("put", resp); err != nil {
		return err
	}
	return validateS3EmptySuccessResponse("put", resp.Body)
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
	if err := validateS3UnsupportedRequestChargedHeader("get", resp); err != nil {
		drainAndCloseS3Body(resp.Body)
		return nil, err
	}
	if size, known, err := s3GetObjectContentLength(resp); err != nil {
		drainAndCloseS3Body(resp.Body)
		return nil, err
	} else if known {
		return &exactReadCloser{ctx: ctx, reader: resp.Body, closer: resp.Body, remaining: size}, nil
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
	switch resp.StatusCode {
	case http.StatusPartialContent:
		if err := validateS3UnsupportedRequestChargedHeader("get range", resp); err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, err
		}
		contentRange, err := s3ResponseContentRangeHeader(resp)
		if err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, fmt.Errorf("get range s3 object: %w", err)
		}
		if err := validateS3ContentRange(contentRange, validated); err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, err
		}
		if err := validateS3RangeContentLength(resp, validated); err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, err
		}
	case http.StatusOK:
		if err := validateS3UnsupportedRequestChargedHeader("get range", resp); err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, err
		}
		if err := validateS3FullRangeCompatibilityResponse(resp, validated); err != nil {
			drainAndCloseS3Body(resp.Body)
			return nil, err
		}
	default:
		err := s3StatusError("get range", resp)
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
	if err := validateS3UnsupportedRequestChargedHeader("stat", resp); err != nil {
		return ObjectInfo{}, err
	}
	size, err := s3StatContentLength(resp)
	if err != nil {
		return ObjectInfo{}, err
	}
	rawObjectPath := objectPath
	objectPath, err = validateS3ObjectPath(objectPath)
	if err != nil {
		return ObjectInfo{}, fmt.Errorf("unsafe storage path %q: %w", rawObjectPath, err)
	}
	rawLastModified, lastModifiedPresent, ok := s3ResponseOptionalSingleHeader(resp, "Last-Modified")
	if !ok {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: duplicate last-modified")
	}
	if lastModifiedPresent && strings.TrimSpace(rawLastModified) == "" {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: invalid last-modified")
	}
	lastModified, err := parseS3StatLastModified(rawLastModified)
	if err != nil {
		return ObjectInfo{}, err
	}
	rawETag, etagPresent, ok := s3ResponseOptionalSingleHeader(resp, "ETag")
	if !ok {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: duplicate etag")
	}
	if etagPresent && strings.TrimSpace(rawETag) == "" {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: invalid etag")
	}
	etag, err := parseS3StatETag(rawETag)
	if err != nil {
		return ObjectInfo{}, err
	}
	rawContentType, contentTypePresent, ok := s3ResponseOptionalSingleHeader(resp, "Content-Type")
	if !ok {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: duplicate content-type")
	}
	if contentTypePresent && strings.TrimSpace(rawContentType) == "" {
		return ObjectInfo{}, fmt.Errorf("stat s3 object: invalid content-type")
	}
	contentType, err := parseS3StatContentType(rawContentType)
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Path:         objectPath,
		Size:         size,
		ContentType:  contentType,
		ETag:         etag,
		LastModified: lastModified,
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
	if err := validateS3UnsupportedRequestChargedHeader("delete", resp); err != nil {
		return err
	}
	return validateS3EmptySuccessResponse("delete", resp.Body)
}

func (s *S3Store) Copy(ctx context.Context, sourcePath string, destPath string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	sourceObjectPath, err := validateS3ObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := validateS3ObjectPath(destPath)
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
	if err := validateS3UnsupportedRequestChargedHeader("copy", resp); err != nil {
		return err
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
	sourceObjectPath, err := validateS3ObjectPath(sourcePath)
	if err != nil {
		return fmt.Errorf("unsafe source storage path %q: %w", sourcePath, err)
	}
	destObjectPath, err := validateS3ObjectPath(destPath)
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
	prefix, err := validateS3ObjectPrefix(opts.Prefix)
	if err != nil {
		return ObjectListPage{}, fmt.Errorf("unsafe storage prefix %q: %w", opts.Prefix, err)
	}
	cursor, err := ValidateListCursor(opts.Cursor)
	if err != nil {
		return ObjectListPage{}, err
	}
	limit := NormalizeListLimit(opts.Limit)
	listPrefix := s.listPrefix(prefix)
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
	if err := validateS3UnsupportedRequestChargedHeader("list", resp); err != nil {
		return ObjectListPage{}, err
	}
	result, err := decodeS3ListObjects(resp.Body)
	if err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListKeyCount(result.KeyCount, len(result.Contents)); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListMaxKeys(result.MaxKeys, len(result.Contents)); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListPrefix(result.Prefix, listPrefix); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListBucketName(result.Name, s.bucket); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListEncodingType(result.EncodingType); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListContinuationToken(result.ContinuationToken, cursor); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListStartAfter(result.StartAfter); err != nil {
		return ObjectListPage{}, err
	}
	if err := validateS3ListDelimiter(result.Delimiter); err != nil {
		return ObjectListPage{}, err
	}
	isTruncated, ok := parseS3ListIsTruncated(result.IsTruncated)
	if !ok {
		return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid IsTruncated value")
	}
	nextCursor := ""
	if isTruncated {
		var err error
		nextCursor, err = ValidateListCursor(result.NextContinuationToken)
		if err != nil {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid continuation token: %w", err)
		}
		if nextCursor == "" {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: truncated response missing continuation token")
		}
	}
	page := ObjectListPage{
		Objects:    make([]ObjectInfo, 0, len(result.Contents)),
		NextCursor: nextCursor,
		HasMore:    isTruncated,
	}
	for _, item := range result.Contents {
		if strings.TrimSpace(item.Key) == "" {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: object key is required")
		}
		objectPath, ok, err := s.objectPathFromKey(item.Key)
		if err != nil {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid object key: %w", err)
		}
		if !ok {
			continue
		}
		if prefix != "" && objectPath != prefix && !strings.HasPrefix(objectPath, prefix+"/") {
			continue
		}
		size, ok := parseS3ListObjectSize(item.Size)
		if !ok {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid object size")
		}
		if len(page.Objects) >= limit {
			return ObjectListPage{}, fmt.Errorf("list s3 objects: response contains more objects than requested limit")
		}
		lastModified := time.Time{}
		if item.LastModified != nil {
			if strings.TrimSpace(*item.LastModified) == "" {
				return ObjectListPage{}, fmt.Errorf("list s3 objects: last-modified is empty")
			}
			var ok bool
			lastModified, ok = parseS3ListTime(*item.LastModified)
			if !ok {
				return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid last-modified")
			}
		}
		etag := ""
		if item.ETag != nil {
			if strings.TrimSpace(*item.ETag) == "" {
				return ObjectListPage{}, fmt.Errorf("list s3 objects: object etag is empty")
			}
			if strings.TrimSpace(*item.ETag) != *item.ETag {
				return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid object etag")
			}
			etag = cleanS3ETag(*item.ETag)
			if etag == "" {
				return ObjectListPage{}, fmt.Errorf("list s3 objects: invalid object etag")
			}
		}
		page.Objects = append(page.Objects, ObjectInfo{
			Path:         objectPath,
			Size:         size,
			ETag:         etag,
			LastModified: lastModified,
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
	req.Header.Set("x-amz-content-sha256", "UNSIGNED-PAYLOAD")
	req.Header.Set("x-amz-date", s.now().UTC().Format("20060102T150405Z"))
	if s.sessionToken != "" {
		req.Header.Set("x-amz-security-token", s.sessionToken)
	}
	s.sign(req)
	return req, nil
}

func (s *S3Store) listPrefix(prefix string) string {
	if prefix != "" {
		return s.key(prefix)
	}
	if s.prefix != "" {
		return s.prefix + "/"
	}
	return ""
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
		if resp.ContentLength > 0 && resp.ContentLength != size {
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

func parseS3NonNegativeDecimal(value string) (int64, bool) {
	if value == "" {
		return 0, false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return 0, false
		}
	}
	size, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return 0, false
	}
	return size, true
}

func parseS3ListObjectSize(value string) (int64, bool) {
	return parseS3NonNegativeDecimal(value)
}

func validateS3ListKeyCount(value *string, contents int) error {
	if value == nil {
		return nil
	}
	count, ok := parseS3NonNegativeDecimal(*value)
	if !ok {
		return fmt.Errorf("list s3 objects: invalid KeyCount value")
	}
	if count != int64(contents) {
		return fmt.Errorf("list s3 objects: KeyCount does not match contents")
	}
	return nil
}

func validateS3ListMaxKeys(value *string, contents int) error {
	if value == nil {
		return nil
	}
	maxKeys, ok := parseS3NonNegativeDecimal(*value)
	if !ok {
		return fmt.Errorf("list s3 objects: invalid MaxKeys value")
	}
	if int64(contents) > maxKeys {
		return fmt.Errorf("list s3 objects: MaxKeys is less than contents")
	}
	return nil
}

func validateS3ListPrefix(value *string, expected string) error {
	if value == nil {
		return nil
	}
	if *value != expected {
		return fmt.Errorf("list s3 objects: response Prefix does not match request")
	}
	return nil
}

func validateS3ListBucketName(value *string, expected string) error {
	if value == nil {
		return nil
	}
	if *value != expected {
		return fmt.Errorf("list s3 objects: response Name does not match bucket")
	}
	return nil
}

func validateS3ListEncodingType(value *string) error {
	if value == nil {
		return nil
	}
	return fmt.Errorf("list s3 objects: unsupported EncodingType value")
}

func validateS3ListContinuationToken(value *string, expected string) error {
	if value == nil {
		return nil
	}
	if expected == "" || *value != expected {
		return fmt.Errorf("list s3 objects: response ContinuationToken does not match request")
	}
	return nil
}

func validateS3ListStartAfter(value *string) error {
	if value == nil {
		return nil
	}
	return fmt.Errorf("list s3 objects: unsupported StartAfter value")
}

func validateS3UnsupportedRequestChargedHeader(operation string, resp *http.Response) error {
	value, present, ok := s3ResponseOptionalSingleHeader(resp, "x-amz-request-charged")
	if !ok {
		return fmt.Errorf("%s s3 object: duplicate request-charged header", operation)
	}
	if !present {
		return nil
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s s3 object: invalid request-charged header", operation)
	}
	return fmt.Errorf("%s s3 object: unsupported request-charged header", operation)
}

func validateS3ListDelimiter(value *string) error {
	if value == nil {
		return nil
	}
	return fmt.Errorf("list s3 objects: unsupported Delimiter value")
}

func parseS3ListIsTruncated(value string) (bool, bool) {
	switch value {
	case "true":
		return true, true
	case "false":
		return false, true
	default:
		return false, false
	}
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
	if strings.HasPrefix(value, `"`) || strings.HasSuffix(value, `"`) {
		if len(value) < 2 || !strings.HasPrefix(value, `"`) || !strings.HasSuffix(value, `"`) {
			return ""
		}
		value = value[1 : len(value)-1]
		if value == "" || strings.Contains(value, `"`) {
			return ""
		}
	} else if strings.Contains(value, `"`) {
		return ""
	}
	if value == "" || len(value) > maxS3ETagBytes || strings.ContainsAny(value, "\r\n") {
		return ""
	}
	if !s3ETagOpaqueValueValid(value) {
		return ""
	}
	return value
}

func s3ETagOpaqueValueValid(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] <= 0x20 || value[i] >= 0x7f || value[i] == '"' {
			return false
		}
	}
	return true
}

func parseS3StatContentType(value string) (string, error) {
	contentType := cleanS3MetadataValue(value, maxS3ContentTypeBytes)
	if strings.TrimSpace(value) != "" && (contentType == "" || !s3ContentTypeValueValid(contentType)) {
		return "", fmt.Errorf("stat s3 object: invalid content-type")
	}
	return contentType, nil
}

func s3ContentTypeValueValid(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] >= 0x7f {
			return false
		}
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.Contains(mediaType, "/")
}

func parseS3StatETag(value string) (string, error) {
	etag := cleanS3ETag(value)
	if strings.TrimSpace(value) != "" && etag == "" {
		return "", fmt.Errorf("stat s3 object: invalid etag")
	}
	return etag, nil
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

func (s *S3Store) key(objectPath string) string {
	if s.prefix == "" {
		return objectPath
	}
	return s.prefix + "/" + objectPath
}

func (s *S3Store) objectPathFromKey(key string) (string, bool, error) {
	if strings.TrimSpace(key) != key {
		return "", false, fmt.Errorf("storage key must not contain leading or trailing whitespace")
	}
	if s.prefix != "" {
		prefix := s.prefix + "/"
		if !strings.HasPrefix(key, prefix) {
			return "", false, nil
		}
		key = strings.TrimPrefix(key, prefix)
	}
	if strings.TrimSpace(key) != key {
		return "", false, fmt.Errorf("storage key must not contain leading or trailing whitespace")
	}
	objectPath, err := validateS3ObjectPath(key)
	if err != nil {
		return "", false, err
	}
	return objectPath, true, nil
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

func S3BucketNeedsPathStyle(endpoint *url.URL, bucket string) bool {
	return s3BucketNeedsPathStyle(endpoint, bucket)
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

func encodeS3CanonicalQuery(values url.Values) string {
	if len(values) == 0 {
		return ""
	}
	pairs := make([]string, 0)
	for name, values := range values {
		encodedName := escapeS3QueryComponent(name)
		if len(values) == 0 {
			pairs = append(pairs, encodedName+"=")
			continue
		}
		for _, value := range values {
			pairs = append(pairs, encodedName+"="+escapeS3QueryComponent(value))
		}
	}
	sort.Strings(pairs)
	return strings.Join(pairs, "&")
}

func normalizeS3Prefix(prefix string) (string, error) {
	prefix = strings.Trim(strings.TrimSpace(prefix), "/")
	if prefix == "" {
		return "", nil
	}
	return validateS3ObjectPath(prefix)
}

func validateS3ObjectPath(objectPath string) (string, error) {
	return ValidateObjectPath(objectPath)
}

func validateS3ObjectPrefix(prefix string) (string, error) {
	return ValidateObjectPrefix(prefix)
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

func escapeS3QueryComponent(value string) string {
	var b strings.Builder
	for i := 0; i < len(value); i++ {
		c := value[i]
		if (c >= 'A' && c <= 'Z') ||
			(c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '.' || c == '_' || c == '~' {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteString(strings.ToUpper(hex.EncodeToString([]byte{c})))
	}
	return b.String()
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

type s3ListObjectsResult struct {
	XMLName               xml.Name              `xml:"ListBucketResult"`
	IsTruncated           string                `xml:"IsTruncated"`
	ContinuationToken     *string               `xml:"ContinuationToken"`
	NextContinuationToken string                `xml:"NextContinuationToken"`
	Name                  *string               `xml:"Name"`
	Prefix                *string               `xml:"Prefix"`
	Delimiter             *string               `xml:"Delimiter"`
	StartAfter            *string               `xml:"StartAfter"`
	EncodingType          *string               `xml:"EncodingType"`
	KeyCount              *string               `xml:"KeyCount"`
	MaxKeys               *string               `xml:"MaxKeys"`
	Contents              []s3ListObjectContent `xml:"Contents"`
}

type s3ListObjectContent struct {
	Key          string  `xml:"Key"`
	Size         string  `xml:"Size"`
	ETag         *string `xml:"ETag"`
	LastModified *string `xml:"LastModified"`
}

type s3CopyResponse struct {
	XMLName      xml.Name
	Code         string  `xml:"Code"`
	Message      string  `xml:"Message"`
	RequestID    string  `xml:"RequestId"`
	HostID       string  `xml:"HostId"`
	ETag         string  `xml:"ETag"`
	LastModified *string `xml:"LastModified"`
}

const maxS3ListResponseBytes = 4 << 20
const maxS3CopyResponseBytes = 1 << 20
const maxS3ErrorPreviewFieldBytes = 1024

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
	if preview, ok := s3XMLError(data); ok {
		if preview == "" {
			return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: embedded error")
		}
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: embedded error: %s", preview)
	}
	if err := validateS3ListControlCardinality(data); err != nil {
		return s3ListObjectsResult{}, err
	}
	var result s3ListObjectsResult
	if err := xml.Unmarshal(data, &result); err != nil {
		return s3ListObjectsResult{}, fmt.Errorf("decode s3 list response: %w", err)
	}
	if !s3XMLNamespaceAllowed(result.XMLName.Space) {
		return s3ListObjectsResult{}, fmt.Errorf("list s3 objects: unexpected response namespace")
	}
	return result, nil
}

func validateS3ListControlCardinality(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rootDepth int
	var isTruncatedSeen bool
	var continuationTokenSeen bool
	var inContent bool
	var keySeen bool
	var sizeSeen bool
	var etagSeen bool
	var lastModifiedSeen bool
	var storageClassSeen bool
	var ownerSeen bool
	var checksumTypeSeen bool
	var restoreStatusSeen bool
	var simpleObjectMetadata string
	var simpleStandardMetadata string
	var structuredObjectMetadata string
	structuredObjectChildSeen := make(map[string]struct{})
	rootSimpleSeen := make(map[string]struct{})
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode s3 list response: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			rootDepth++
			switch {
			case rootDepth == 2:
				switch token.Name.Local {
				case "IsTruncated":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if isTruncatedSeen {
						return fmt.Errorf("list s3 objects: duplicate IsTruncated value")
					}
					isTruncatedSeen = true
				case "NextContinuationToken":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if continuationTokenSeen {
						return fmt.Errorf("list s3 objects: duplicate continuation token")
					}
					continuationTokenSeen = true
				case "Contents":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					inContent = true
					keySeen = false
					sizeSeen = false
					etagSeen = false
					lastModifiedSeen = false
					storageClassSeen = false
					ownerSeen = false
					checksumTypeSeen = false
					restoreStatusSeen = false
					simpleObjectMetadata = ""
					simpleStandardMetadata = ""
				case "CommonPrefixes":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					return fmt.Errorf("list s3 objects: CommonPrefixes are unsupported")
				default:
					if s3ListStandardRootMetadata(token.Name.Local) && !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if s3ListStandardSimpleRootMetadata(token.Name.Local) {
						if _, ok := rootSimpleSeen[token.Name.Local]; ok {
							return fmt.Errorf("list s3 objects: duplicate %s value", token.Name.Local)
						}
						rootSimpleSeen[token.Name.Local] = struct{}{}
						simpleStandardMetadata = token.Name.Local
					}
				}
			case inContent && rootDepth == 3:
				switch token.Name.Local {
				case "Key":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if keySeen {
						return fmt.Errorf("list s3 objects: duplicate object key")
					}
					keySeen = true
					simpleObjectMetadata = token.Name.Local
				case "Size":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if sizeSeen {
						return fmt.Errorf("list s3 objects: duplicate object size")
					}
					sizeSeen = true
					simpleObjectMetadata = token.Name.Local
				case "ETag":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if etagSeen {
						return fmt.Errorf("list s3 objects: duplicate object etag")
					}
					etagSeen = true
					simpleObjectMetadata = token.Name.Local
				case "LastModified":
					if !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					if lastModifiedSeen {
						return fmt.Errorf("list s3 objects: duplicate object last-modified")
					}
					lastModifiedSeen = true
					simpleObjectMetadata = token.Name.Local
				default:
					if s3ListStandardObjectMetadata(token.Name.Local) && !s3XMLNamespaceAllowed(token.Name.Space) {
						return fmt.Errorf("list s3 objects: unexpected response namespace")
					}
					switch token.Name.Local {
					case "StorageClass":
						if storageClassSeen {
							return fmt.Errorf("list s3 objects: duplicate object StorageClass value")
						}
						storageClassSeen = true
					case "Owner":
						if ownerSeen {
							return fmt.Errorf("list s3 objects: duplicate object Owner value")
						}
						ownerSeen = true
						structuredObjectMetadata = token.Name.Local
						clear(structuredObjectChildSeen)
					case "ChecksumType":
						if checksumTypeSeen {
							return fmt.Errorf("list s3 objects: duplicate object ChecksumType value")
						}
						checksumTypeSeen = true
					case "RestoreStatus":
						if restoreStatusSeen {
							return fmt.Errorf("list s3 objects: duplicate object RestoreStatus value")
						}
						restoreStatusSeen = true
						structuredObjectMetadata = token.Name.Local
						clear(structuredObjectChildSeen)
					}
					if s3ListStandardSimpleObjectMetadata(token.Name.Local) {
						simpleStandardMetadata = token.Name.Local
					}
				}
			case inContent && rootDepth > 3 && structuredObjectMetadata != "":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("list s3 objects: object %s metadata contains unexpected namespace", structuredObjectMetadata)
				}
				if rootDepth > 4 {
					return fmt.Errorf("list s3 objects: object %s metadata contains nested element %q", structuredObjectMetadata, token.Name.Local)
				}
				if !s3ListStructuredObjectMetadataChildAllowed(structuredObjectMetadata, token.Name.Local) {
					return fmt.Errorf("list s3 objects: object %s metadata contains unsupported child %q", structuredObjectMetadata, token.Name.Local)
				}
				if _, ok := structuredObjectChildSeen[token.Name.Local]; ok {
					return fmt.Errorf("list s3 objects: object %s metadata contains duplicate child %q", structuredObjectMetadata, token.Name.Local)
				}
				structuredObjectChildSeen[token.Name.Local] = struct{}{}
			case rootDepth > 2 && simpleStandardMetadata != "":
				return fmt.Errorf("list s3 objects: metadata %s contains nested element %q", simpleStandardMetadata, token.Name.Local)
			case inContent && rootDepth > 3 && simpleObjectMetadata != "":
				return fmt.Errorf("list s3 objects: object %s metadata contains nested element %q", simpleObjectMetadata, token.Name.Local)
			}
		case xml.EndElement:
			if simpleStandardMetadata == token.Name.Local {
				simpleStandardMetadata = ""
			}
			if inContent && rootDepth == 3 && simpleObjectMetadata == token.Name.Local {
				simpleObjectMetadata = ""
			}
			if inContent && rootDepth == 3 && structuredObjectMetadata == token.Name.Local {
				structuredObjectMetadata = ""
				clear(structuredObjectChildSeen)
			}
			if inContent && rootDepth == 2 && token.Name.Local == "Contents" {
				inContent = false
				simpleObjectMetadata = ""
				simpleStandardMetadata = ""
				structuredObjectMetadata = ""
				clear(structuredObjectChildSeen)
			}
			if rootDepth > 0 {
				rootDepth--
			}
		case xml.CharData:
			if inContent && rootDepth == 3 && structuredObjectMetadata != "" && len(bytes.TrimSpace(token)) > 0 {
				return fmt.Errorf("list s3 objects: object %s metadata contains direct text", structuredObjectMetadata)
			}
		}
	}
}

func s3ListStandardSimpleRootMetadata(local string) bool {
	switch local {
	case "Name", "Prefix", "Delimiter", "MaxKeys", "KeyCount", "ContinuationToken", "StartAfter", "EncodingType":
		return true
	default:
		return false
	}
}

func s3ListStandardRootMetadata(local string) bool {
	switch local {
	case "Name", "Prefix", "Delimiter", "MaxKeys", "KeyCount", "ContinuationToken", "StartAfter", "EncodingType":
		return true
	default:
		return false
	}
}

func s3ListStandardSimpleObjectMetadata(local string) bool {
	switch local {
	case "StorageClass", "ChecksumAlgorithm", "ChecksumType":
		return true
	default:
		return false
	}
}

func s3ListStandardObjectMetadata(local string) bool {
	switch local {
	case "StorageClass", "Owner", "ChecksumAlgorithm", "ChecksumType", "RestoreStatus":
		return true
	default:
		return false
	}
}

func s3ListStructuredObjectMetadataChildAllowed(parent string, local string) bool {
	switch parent {
	case "Owner":
		switch local {
		case "ID", "DisplayName":
			return true
		default:
			return false
		}
	case "RestoreStatus":
		switch local {
		case "IsRestoreInProgress", "RestoreExpiryDate":
			return true
		default:
			return false
		}
	default:
		return false
	}
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
	if err := validateS3CopyResultShape(data); err != nil {
		return err
	}
	var response s3CopyResponse
	if err := xml.Unmarshal(data, &response); err != nil {
		return fmt.Errorf("decode s3 copy response: %w", err)
	}
	switch response.XMLName.Local {
	case "CopyObjectResult":
		if !s3XMLNamespaceAllowed(response.XMLName.Space) {
			return fmt.Errorf("copy s3 object: unexpected response namespace")
		}
		if strings.TrimSpace(response.ETag) == "" {
			return fmt.Errorf("copy s3 object: etag is required")
		}
		if strings.TrimSpace(response.ETag) != response.ETag {
			return fmt.Errorf("copy s3 object: invalid etag")
		}
		if cleanS3ETag(response.ETag) == "" {
			return fmt.Errorf("copy s3 object: invalid etag")
		}
		if response.LastModified != nil {
			if strings.TrimSpace(*response.LastModified) == "" {
				return fmt.Errorf("copy s3 object: last-modified is empty")
			}
			if _, ok := parseS3ListTime(*response.LastModified); !ok {
				return fmt.Errorf("copy s3 object: invalid last-modified")
			}
		}
		return nil
	case "Error":
		preview, _ := s3XMLError(data)
		if preview == "" {
			return fmt.Errorf("copy s3 object: embedded error")
		}
		return fmt.Errorf("copy s3 object: embedded error: %s", preview)
	default:
		return fmt.Errorf("copy s3 object: unexpected response %q", response.XMLName.Local)
	}
}

func validateS3CopyResultShape(data []byte) error {
	decoder := xml.NewDecoder(bytes.NewReader(data))
	var rootName xml.Name
	var rootDepth int
	var etagSeen bool
	var lastModifiedSeen bool
	var simpleCopyMetadata string
	for {
		token, err := decoder.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return fmt.Errorf("decode s3 copy response: %w", err)
		}
		switch token := token.(type) {
		case xml.StartElement:
			rootDepth++
			if rootDepth == 1 {
				rootName = token.Name
				continue
			}
			if rootDepth > 2 && rootName.Local == "CopyObjectResult" && simpleCopyMetadata != "" {
				return fmt.Errorf("copy s3 object: %s metadata contains nested element %q", simpleCopyMetadata, token.Name.Local)
			}
			if rootDepth != 2 || rootName.Local != "CopyObjectResult" {
				continue
			}
			switch token.Name.Local {
			case "ETag":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				if etagSeen {
					return fmt.Errorf("copy s3 object: duplicate etag")
				}
				etagSeen = true
				simpleCopyMetadata = token.Name.Local
			case "LastModified":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				if lastModifiedSeen {
					return fmt.Errorf("copy s3 object: duplicate last-modified")
				}
				lastModifiedSeen = true
				simpleCopyMetadata = token.Name.Local
			case "Error":
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				simpleCopyMetadata = ""
				response, err := parseS3XMLErrorElement(decoder, token)
				if err != nil {
					if _, ok := err.(s3AmbiguousErrorFieldError); ok {
						return fmt.Errorf("copy s3 object: embedded error")
					}
					return fmt.Errorf("decode s3 copy response: %w", err)
				}
				preview := s3ErrorPreview(response.Code, response.Message, s3ErrorDetail("request-id", response.RequestID), s3ErrorDetail("host-id", response.HostID))
				if preview == "" {
					return fmt.Errorf("copy s3 object: embedded error")
				}
				return fmt.Errorf("copy s3 object: embedded error: %s", preview)
			default:
				if !s3XMLNamespaceAllowed(token.Name.Space) {
					return fmt.Errorf("copy s3 object: unexpected response namespace")
				}
				return fmt.Errorf("copy s3 object: unexpected response child %q", token.Name.Local)
			}
		case xml.EndElement:
			if rootDepth == 2 && simpleCopyMetadata == token.Name.Local {
				simpleCopyMetadata = ""
			}
			if rootDepth > 0 {
				rootDepth--
			}
		}
	}
}

func s3XMLNamespaceAllowed(value string) bool {
	return value == "" || value == s3XMLNamespace
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

var _ Store = (*S3Store)(nil)
