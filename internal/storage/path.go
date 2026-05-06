package storage

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"unicode/utf8"
)

const (
	MaxObjectPathBytes        = 1024
	MaxObjectPathSegmentBytes = 255
	DefaultListLimit          = 100
	MaxListLimit              = 1000
	MaxListCursorBytes        = 2048
)

// ValidateObjectPath verifies that object paths are unambiguous slash-separated
// relative keys before they reach a storage adapter.
func ValidateObjectPath(objectPath string) (string, error) {
	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		return "", fmt.Errorf("storage path is required")
	}
	if len(objectPath) > MaxObjectPathBytes {
		return "", fmt.Errorf("storage path is too long")
	}
	if !utf8.ValidString(objectPath) {
		return "", fmt.Errorf("storage path must be valid UTF-8")
	}
	if strings.ContainsAny(objectPath, "\r\n") {
		return "", fmt.Errorf("storage path must not contain newlines")
	}
	if strings.Contains(objectPath, `\`) {
		return "", fmt.Errorf("storage path must use forward slash separators")
	}
	if storageContainsEncodedPathSeparator(objectPath) {
		return "", fmt.Errorf("storage path must not contain percent-encoded path separators")
	}
	if path.IsAbs(objectPath) {
		return "", fmt.Errorf("storage path must be relative")
	}
	for _, segment := range strings.Split(objectPath, "/") {
		if segment == "." || segment == ".." || strings.TrimSpace(segment) == "" {
			return "", fmt.Errorf("storage path contains an invalid segment")
		}
		if len(segment) > MaxObjectPathSegmentBytes {
			return "", fmt.Errorf("storage path segment is too long")
		}
	}
	if cleaned := path.Clean(objectPath); cleaned != objectPath {
		return "", fmt.Errorf("storage path must be canonical")
	}
	return objectPath, nil
}

func ValidateObjectPrefix(prefix string) (string, error) {
	prefix = strings.TrimSpace(prefix)
	prefix = strings.TrimSuffix(prefix, "/")
	if prefix == "" {
		return "", nil
	}
	if len(prefix) > MaxObjectPathBytes {
		return "", fmt.Errorf("storage prefix is too long")
	}
	if !utf8.ValidString(prefix) {
		return "", fmt.Errorf("storage prefix must be valid UTF-8")
	}
	if strings.ContainsAny(prefix, "\r\n") {
		return "", fmt.Errorf("storage prefix must not contain newlines")
	}
	if strings.Contains(prefix, `\`) {
		return "", fmt.Errorf("storage prefix must use forward slash separators")
	}
	if storageContainsEncodedPathSeparator(prefix) {
		return "", fmt.Errorf("storage prefix must not contain percent-encoded path separators")
	}
	if path.IsAbs(prefix) {
		return "", fmt.Errorf("storage prefix must be relative")
	}
	for _, segment := range strings.Split(prefix, "/") {
		if segment == "." || segment == ".." || strings.TrimSpace(segment) == "" {
			return "", fmt.Errorf("storage prefix contains an invalid segment")
		}
		if len(segment) > MaxObjectPathSegmentBytes {
			return "", fmt.Errorf("storage prefix segment is too long")
		}
	}
	if cleaned := path.Clean(prefix); cleaned != prefix {
		return "", fmt.Errorf("storage prefix must be canonical")
	}
	return prefix, nil
}

func storageContainsEncodedPathSeparator(value string) bool {
	for i := 0; i < 4; i++ {
		lower := strings.ToLower(value)
		if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
			return true
		}
		decoded, err := url.PathUnescape(value)
		if err != nil || decoded == value {
			return false
		}
		value = decoded
	}
	return false
}

func NormalizeListLimit(limit int) int {
	if limit <= 0 {
		return DefaultListLimit
	}
	if limit > MaxListLimit {
		return MaxListLimit
	}
	return limit
}

func ValidateListCursor(cursor string) (string, error) {
	if cursor == "" {
		return "", nil
	}
	if strings.TrimSpace(cursor) == "" {
		return "", nil
	}
	if strings.TrimSpace(cursor) != cursor {
		return "", fmt.Errorf("storage list cursor must not contain leading or trailing whitespace")
	}
	if len(cursor) > MaxListCursorBytes {
		return "", fmt.Errorf("storage list cursor is too long")
	}
	if !utf8.ValidString(cursor) {
		return "", fmt.Errorf("storage list cursor must be valid UTF-8")
	}
	for _, r := range cursor {
		if r < 0x20 || r == 0x7f {
			return "", fmt.Errorf("storage list cursor must not contain control characters")
		}
	}
	return cursor, nil
}

func ValidateRangeRequest(req RangeRequest) (RangeRequest, error) {
	if req.Offset < 0 {
		return RangeRequest{}, fmt.Errorf("storage range offset must not be negative")
	}
	if req.Length <= 0 {
		return RangeRequest{}, fmt.Errorf("storage range length must be positive")
	}
	if req.Offset > math.MaxInt64-req.Length+1 {
		return RangeRequest{}, fmt.Errorf("storage range end overflows")
	}
	return req, nil
}
