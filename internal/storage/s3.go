package storage

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
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
	now             func() time.Time
}

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
	if accessKeyID == "" || s3CredentialContainsWhitespace(accessKeyID) {
		return nil, fmt.Errorf("s3 access key id is required and must not contain whitespace")
	}
	if opts.SecretAccessKey == "" || s3CredentialContainsWhitespace(opts.SecretAccessKey) {
		return nil, fmt.Errorf("s3 secret access key is required and must not contain whitespace")
	}
	if s3CredentialContainsWhitespace(opts.SessionToken) {
		return nil, fmt.Errorf("s3 session token must not contain whitespace")
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
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		err := s3StatusError("get", resp)
		_ = resp.Body.Close()
		return nil, err
	}
	return resp.Body, nil
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
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s3StatusError("delete", resp)
	}
	return nil
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
	if err := s.Delete(ctx, objectPath); err != nil {
		return fmt.Errorf("delete readiness probe: %w", err)
	}
	return nil
}

func (s *S3Store) newRequest(ctx context.Context, method string, objectPath string, body io.Reader) (*http.Request, error) {
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
	s.sign(req)
	return req, nil
}

func (s *S3Store) objectURL(objectPath string) url.URL {
	target := *s.endpoint
	key := s.key(objectPath)
	escapedKey := escapeS3Key(key)
	basePath := strings.TrimRight(target.Path, "/")
	escapedBasePath := strings.TrimRight(target.EscapedPath(), "/")
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

func (s *S3Store) key(objectPath string) string {
	if s.prefix == "" {
		return objectPath
	}
	return s.prefix + "/" + objectPath
}

func s3BucketNeedsPathStyle(endpoint *url.URL, bucket string) bool {
	return endpoint != nil && endpoint.Scheme == "https" && strings.Contains(bucket, ".")
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
		"host":                 req.URL.Host,
		"x-amz-content-sha256": req.Header.Get("x-amz-content-sha256"),
		"x-amz-date":           req.Header.Get("x-amz-date"),
	}
	if token := req.Header.Get("x-amz-security-token"); token != "" {
		headers["x-amz-security-token"] = token
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

var _ Store = (*S3Store)(nil)
