package storage

import (
	"fmt"
	"path"
	"strings"
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
	if strings.ContainsAny(objectPath, "\r\n") {
		return "", fmt.Errorf("storage path must not contain newlines")
	}
	if strings.Contains(objectPath, `\`) {
		return "", fmt.Errorf("storage path must use forward slash separators")
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
	if strings.ContainsAny(prefix, "\r\n") {
		return "", fmt.Errorf("storage prefix must not contain newlines")
	}
	if strings.Contains(prefix, `\`) {
		return "", fmt.Errorf("storage prefix must use forward slash separators")
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
	cursor = strings.TrimSpace(cursor)
	if cursor == "" {
		return "", nil
	}
	if len(cursor) > MaxListCursorBytes {
		return "", fmt.Errorf("storage list cursor is too long")
	}
	if strings.ContainsAny(cursor, "\r\n") {
		return "", fmt.Errorf("storage list cursor must not contain newlines")
	}
	return cursor, nil
}
