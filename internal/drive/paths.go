package drive

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/storage"
)

const driveObjectRoot = "drive/users"

func BuildStagedObjectPath(userID string, uploadID string) (string, error) {
	userID, err := validateDriveObjectPathID("user_id", userID)
	if err != nil {
		return "", err
	}
	uploadID, err = validateDriveObjectPathID("upload_id", uploadID)
	if err != nil {
		return "", err
	}
	path := fmt.Sprintf("%s/%s/staging/%s", driveObjectRoot, userID, uploadID)
	if _, err := storage.ValidateObjectPath(path); err != nil {
		return "", fmt.Errorf("build staged drive object path: %w", err)
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
	path := fmt.Sprintf("%s/%s/objects/%s", driveObjectRoot, userID, nodeID)
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
	prefix := fmt.Sprintf("%s/%s/", driveObjectRoot, userID)
	if _, err := storage.ValidateObjectPath(prefix + "prefix-check"); err != nil {
		return "", fmt.Errorf("build drive user object prefix: %w", err)
	}
	return prefix, nil
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
