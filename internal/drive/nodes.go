package drive

import (
	"crypto/rand"
	"encoding/hex"
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

	NodeSortName    = "name"
	NodeSortUpdated = "updated"
	NodeSortCreated = "created"
	NodeSortSize    = "size"

	MaxNodeNameBytes = 255
)

func NewNodeID() (string, error) {
	var random [16]byte
	if _, err := rand.Read(random[:]); err != nil {
		return "", fmt.Errorf("generate drive node id: %w", err)
	}
	random[6] = (random[6] & 0x0f) | 0x40
	random[8] = (random[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(random[:])
	return encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32], nil
}

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

func ValidateNodeSort(sort string) (string, error) {
	sort = strings.TrimSpace(strings.ToLower(sort))
	if sort == "" {
		return NodeSortName, nil
	}
	switch sort {
	case NodeSortName, NodeSortUpdated, NodeSortCreated, NodeSortSize:
		return sort, nil
	default:
		return "", fmt.Errorf("unsupported drive node sort %q", sort)
	}
}

func validateDriveNodeSearchQuery(query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}
	if len(query) > MaxNodeNameBytes {
		return "", fmt.Errorf("drive node search query is too long")
	}
	if strings.ContainsAny(query, "\r\n") {
		return "", fmt.Errorf("drive node search query must not contain line breaks")
	}
	for _, r := range query {
		if unicode.IsControl(r) {
			return "", fmt.Errorf("drive node search query must not contain control characters")
		}
	}
	return strings.ToLower(query), nil
}

func escapeDriveNodeLikeQuery(query string) string {
	query = strings.ReplaceAll(query, `\`, `\\`)
	query = strings.ReplaceAll(query, `%`, `\%`)
	query = strings.ReplaceAll(query, `_`, `\_`)
	return query
}
