package storage

import (
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
)

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
	// Use the actual payload hash from the x-amz-content-sha256 header.
	// For HTTPS endpoints the caller sets this to "UNSIGNED-PAYLOAD";
	// for HTTP endpoints it is set to the hex-encoded SHA-256 of the body
	// because MinIO (and AWS) reject unsigned payloads over plain HTTP.
	payloadHash := req.Header.Get("x-amz-content-sha256")
	if payloadHash == "" {
		payloadHash = "UNSIGNED-PAYLOAD"
	}
	canonicalRequest := strings.Join([]string{
		req.Method,
		req.URL.EscapedPath(),
		req.URL.RawQuery,
		canonicalHeaders,
		signedHeaders,
		payloadHash,
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

// s3PayloadHash returns the value for the x-amz-content-sha256 header.
// For HTTPS endpoints it returns "UNSIGNED-PAYLOAD" (AWS/MinIO allow skipping
// body signing over TLS). For HTTP endpoints the actual SHA-256 of the body
// must be provided because MinIO rejects unsigned payloads over plain HTTP.
// If body is nil (e.g. GET/DELETE requests) the hash of an empty body is returned.
func (s *S3Store) s3PayloadHash(body io.Reader) (string, error) {
	if s.endpoint != nil && s.endpoint.Scheme == "https" {
		return "UNSIGNED-PAYLOAD", nil
	}
	// HTTP endpoint: compute real hash.
	if body == nil {
		return sha256Hex(nil), nil
	}
	// If the body supports seeking, compute hash then seek back to start.
	if seeker, ok := body.(io.ReadSeeker); ok {
		start, err := seeker.Seek(0, io.SeekCurrent)
		if err != nil {
			return "", fmt.Errorf("seek to current: %w", err)
		}
		h := sha256.New()
		if _, err := io.Copy(h, seeker); err != nil {
			return "", fmt.Errorf("hash body: %w", err)
		}
		if _, err := seeker.Seek(start, io.SeekStart); err != nil {
			return "", fmt.Errorf("seek back: %w", err)
		}
		return hex.EncodeToString(h.Sum(nil)), nil
	}
	// Non-seekable streaming body: fall back to UNSIGNED-PAYLOAD.
	// This may fail on HTTP-only MinIO; callers should use HTTPS or provide
	// a seekable body for HTTP endpoints.
	return "UNSIGNED-PAYLOAD", nil
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
