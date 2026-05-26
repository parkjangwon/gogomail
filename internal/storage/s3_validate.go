package storage

import (
	"fmt"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"unicode/utf8"
)

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
	if strings.TrimSpace(value) == "" || strings.TrimSpace(value) != value {
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

func s3ContentTypeValueValid(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < 0x20 || value[i] >= 0x7f {
			return false
		}
	}
	mediaType, _, err := mime.ParseMediaType(value)
	return err == nil && strings.Contains(mediaType, "/")
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
