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
	uploadClient    *http.Client
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
	// uploadClient has no overall timeout so large uploads aren't cut short.
	// Response-header timeout prevents hung connections; body transfer is
	// bounded by the caller's context instead.
	// When a custom HTTPClient is provided (e.g. in tests), reuse it for
	// uploads so mock transports apply uniformly.
	var uploadClient *http.Client
	if opts.HTTPClient != nil {
		uploadClient = opts.HTTPClient
	} else {
		uploadClient = &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
			},
		}
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
		uploadClient:    uploadClient,
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
	resp, err := s.uploadClient.Do(req)
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

func (s *S3Store) listPrefix(prefix string) string {
	if prefix != "" {
		return s.key(prefix)
	}
	if s.prefix != "" {
		return s.prefix + "/"
	}
	return ""
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

var _ Store = (*S3Store)(nil)
