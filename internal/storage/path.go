package storage

import (
	"fmt"
	"path"
	"strings"
)

// ValidateObjectPath verifies that object paths are unambiguous slash-separated
// relative keys before they reach a storage adapter.
func ValidateObjectPath(objectPath string) (string, error) {
	objectPath = strings.TrimSpace(objectPath)
	if objectPath == "" {
		return "", fmt.Errorf("storage path is required")
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
	}
	if cleaned := path.Clean(objectPath); cleaned != objectPath {
		return "", fmt.Errorf("storage path must be canonical")
	}
	return objectPath, nil
}
