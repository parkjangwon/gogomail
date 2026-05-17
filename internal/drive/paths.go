package drive

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/storage"
)

const (
	driveObjectRoot       = "drive"
	legacyDriveObjectRoot = "drive/users"
)

type ObjectPathScope struct {
	CompanyID string
	DomainID  string
	UserID    string
}

func BuildStagedObjectPath(userID string, uploadID string) (string, error) {
	userID, err := validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	uploadID, err = validateDriveObjectPathID("upload_id", uploadID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s/staging/%s", legacyDriveObjectRoot, userID, uploadID)
	if _, err := storage.ValidateObjectPath(path); err != nil {
		return "", fmt.Errorf("build staged drive object path: %w", err)
	}
	return path, nil
}

func BuildUploadSessionBodyPath(userID string, sessionID string, objectID string) (string, error) {
	userID, err := validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	sessionID, err = validateDriveObjectPathID("session_id", sessionID)
	if err != nil {
		return "", err
	}
	objectID, err = validateDriveObjectPathID("object_id", objectID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s/upload-sessions/%s/bodies/%s", legacyDriveObjectRoot, userID, sessionID, objectID)
	if _, err := storage.ValidateObjectPath(path); err != nil {
		return "", fmt.Errorf("build drive upload session body path: %w", err)
	}
	return path, nil
}

func BuildNodeObjectPath(userID string, nodeID string) (string, error) {
	userID, err := validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	nodeID, err = validateDriveObjectPathID("node_id", nodeID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s/objects/%s", legacyDriveObjectRoot, userID, nodeID)
	if _, err := storage.ValidateObjectPath(path); err != nil {
		return "", fmt.Errorf("build committed drive object path: %w", err)
	}
	return path, nil
}

func UserObjectPrefix(userID string) (string, error) {
	userID, err := validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	prefix := fmt.Sprintf("%s/%s/", legacyDriveObjectRoot, userID)
	if _, err := storage.ValidateObjectPath(prefix + "prefix-check"); err != nil {
		return "", fmt.Errorf("build drive user object prefix: %w", err)
	}
	return prefix, nil
}

func BuildScopedStagedObjectPath(scope ObjectPathScope, uploadID string) (string, error) {
	scope, err := validateObjectPathScope(scope)
	if err != nil {
		return "", err
	}
	uploadID, err = validateDriveObjectPathID("upload_id", uploadID)
	if err != nil {
		return "", err
	}
	return scopedDrivePath(scope, "staging", uploadID)
}

func BuildScopedUploadSessionBodyPath(scope ObjectPathScope, sessionID string, objectID string) (string, error) {
	scope, err := validateObjectPathScope(scope)
	if err != nil {
		return "", err
	}
	sessionID, err = validateDriveObjectPathID("session_id", sessionID)
	if err != nil {
		return "", err
	}
	objectID, err = validateDriveObjectPathID("object_id", objectID)
	if err != nil {
		return "", err
	}
	return scopedDrivePath(scope, "upload-sessions", sessionID, "bodies", objectID)
}

func BuildScopedNodeObjectPath(scope ObjectPathScope, nodeID string) (string, error) {
	scope, err := validateObjectPathScope(scope)
	if err != nil {
		return "", err
	}
	nodeID, err = validateDriveObjectPathID("node_id", nodeID)
	if err != nil {
		return "", err
	}
	return scopedDrivePath(scope, "objects", nodeID)
}

func scopedDrivePath(scope ObjectPathScope, parts ...string) (string, error) {
	base := []string{
		driveObjectRoot,
		scope.CompanyID,
		scope.DomainID,
		"users",
		scope.UserID,
	}
	path := strings.Join(append(base, parts...), "/")
	if _, err := storage.ValidateObjectPath(path); err != nil {
		return "", fmt.Errorf("build tenant drive object path: %w", err)
	}
	return path, nil
}

func validateUserObjectPath(userID string, path string) (string, error) {
	storagePath, err := storage.ValidateObjectPath(path)
	if err != nil {
		return "", err
	}
	userID, err = validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	parts := strings.Split(storagePath, "/")
	legacy := len(parts) >= 3 && parts[0] == "drive" && parts[1] == "users" && parts[2] == userID
	scoped := len(parts) >= 5 && parts[0] == "drive" && parts[3] == "users" && parts[4] == userID
	if !legacy && !scoped {
		return "", fmt.Errorf("drive object path does not belong to user")
	}
	return storagePath, nil
}

func validateObjectPathScope(scope ObjectPathScope) (ObjectPathScope, error) {
	companyID, err := validateDriveObjectPathID("company_id", scope.CompanyID)
	if err != nil {
		return ObjectPathScope{}, err
	}
	domainID, err := validateDriveObjectPathID("domain_id", scope.DomainID)
	if err != nil {
		return ObjectPathScope{}, err
	}
	userID, err := validateDriveObjectPathID("user_id", scope.UserID)
	if err != nil {
		return ObjectPathScope{}, err
	}
	return ObjectPathScope{CompanyID: companyID, DomainID: domainID, UserID: userID}, nil
}

func validateDriveObjectPathID(field string, value string) (string, error) {
	value, err := validateDriveID(field, value, true)
	if err != nil {
		return "", err
	}
	if strings.ContainsAny(value, `/\`) || value == "." || value == ".." {
		return "", fmt.Errorf("%s is not safe for drive object paths", field)
	}
	return value, nil
}
