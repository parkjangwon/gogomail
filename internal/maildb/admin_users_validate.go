package maildb

import (
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/auth"
	"github.com/google/uuid"
)

func ValidateUpdateUserStatusRequest(req UpdateUserStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if !isUserStatus(normalizeAdminStatus(req.Status)) {
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
	return nil
}

func ValidateBulkUpdateUserStatusRequest(req BulkUpdateUserStatusRequest) error {
	if len(req.IDs) == 0 {
		return fmt.Errorf("user ids are required")
	}
	if !isUserStatus(normalizeAdminStatus(req.Status)) {
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
	for _, id := range req.IDs {
		if _, err := uuid.Parse(strings.TrimSpace(id)); err != nil {
			return fmt.Errorf("invalid user id %q", id)
		}
	}
	if companyID := strings.TrimSpace(req.CompanyID); companyID != "" {
		if _, err := uuid.Parse(companyID); err != nil {
			return fmt.Errorf("invalid company id %q", req.CompanyID)
		}
	}
	return nil
}

func dedupeTrimmedStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func ValidateDeleteUserRequest(req DeleteUserRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	return nil
}

func ValidateUserListRequest(req UserListRequest) error {
	status := normalizeAdminStatus(req.Status)
	if status != "" && !isUserStatus(status) {
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
	return nil
}

func isUserStatus(status string) bool {
	switch status {
	case "active", "suspended", "disabled":
		return true
	default:
		return false
	}
}

func ValidateUpdateUserQuotaRequest(req UpdateUserQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	if _, err := normalizeQuotaSource(req.QuotaSource, "custom"); err != nil {
		return err
	}
	return nil
}

func ValidateUpdateUserPasswordHashRequest(req UpdateUserPasswordHashRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if err := auth.ValidatePasswordHash(req.PasswordHash, true); err != nil {
		return err
	}
	return nil
}

func normalizeQuotaSource(value string, fallback string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		value = fallback
	}
	switch value {
	case "default", "custom":
		return value, nil
	default:
		return "", fmt.Errorf("quota_source must be default or custom")
	}
}
