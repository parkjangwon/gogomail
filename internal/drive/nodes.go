package drive

import (
	"fmt"
	"strings"
	"unicode"
)

const (
	NodeTypeFolder = "folder"
	NodeTypeFile   = "file"

	NodeStatusActive  = "active"
	NodeStatusTrashed = "trashed"
	NodeStatusDeleted = "deleted"

	MaxNodeNameBytes = 255
)

func NormalizeNodeName(name string) (string, error) {
	name, err := ValidateNodeName(name)
	if err != nil {
		return "", err
	}
	return strings.ToLower(name), nil
}

func ValidateNodeName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("drive node name is required")
	}
	if len(name) > MaxNodeNameBytes {
		return "", fmt.Errorf("drive node name is too long")
	}
	if name == "." || name == ".." {
		return "", fmt.Errorf("drive node name is reserved")
	}
	if strings.ContainsAny(name, `/\`) {
		return "", fmt.Errorf("drive node name must not contain path separators")
	}
	if strings.ContainsAny(name, "\r\n") {
		return "", fmt.Errorf("drive node name must not contain line breaks")
	}
	for _, r := range name {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("drive node name must not contain control characters")
		}
	}
	return name, nil
}

func ValidateNodeType(nodeType string) (string, error) {
	nodeType = strings.TrimSpace(strings.ToLower(nodeType))
	switch nodeType {
	case NodeTypeFolder, NodeTypeFile:
		return nodeType, nil
	default:
		return "", fmt.Errorf("unsupported drive node type %q", nodeType)
	}
}

func ValidateNodeStatus(status string) (string, error) {
	status = strings.TrimSpace(strings.ToLower(status))
	switch status {
	case NodeStatusActive, NodeStatusTrashed, NodeStatusDeleted:
		return status, nil
	default:
		return "", fmt.Errorf("unsupported drive node status %q", status)
	}
}
