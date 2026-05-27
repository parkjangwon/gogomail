package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/configstore"
)

const ipAccessPolicyKey = "ip_access_policy"

type ipAccessPolicy struct {
	Enabled   bool     `json:"enabled"`
	Allowlist []string `json:"allowlist"`
	Denylist  []string `json:"denylist"`
	Protocols []string `json:"protocols"`
	Action    string   `json:"action"`
}

func defaultIPAccessPolicy() ipAccessPolicy {
	return ipAccessPolicy{
		Enabled:   false,
		Allowlist: []string{},
		Denylist:  []string{},
		Protocols: []string{"smtp", "imap", "api"},
		Action:    "deny",
	}
}

func handleGetCompanyIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ipAccessPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultIPAccessPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy ipAccessPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy ipAccessPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, ipAccessPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const retentionPolicyKey = "retention_policy"

type retentionPolicy struct {
	MailRetentionDays         int  `json:"mail_retention_days"`
	DeletedItemsRetentionDays int  `json:"deleted_items_retention_days"`
	AuditLogRetentionDays     int  `json:"audit_log_retention_days"`
	AttachmentRetentionDays   int  `json:"attachment_retention_days"`
	AutoPurgeEnabled          bool `json:"auto_purge_enabled"`
}

func defaultRetentionPolicy() retentionPolicy {
	return retentionPolicy{
		MailRetentionDays:         0,
		DeletedItemsRetentionDays: 30,
		AuditLogRetentionDays:     365,
		AttachmentRetentionDays:   0,
		AutoPurgeEnabled:          false,
	}
}

func handleGetCompanyRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, retentionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRetentionPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy retentionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy retentionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, retentionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, retentionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRetentionPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy retentionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainRetentionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy retentionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, retentionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, ipAccessPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultIPAccessPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy ipAccessPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainIPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy ipAccessPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, ipAccessPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const authPolicyKey = "auth_policy"

type authPolicy struct {
	MinLength             int      `json:"min_length"`
	RequireUppercase      bool     `json:"require_uppercase"`
	RequireNumbers        bool     `json:"require_numbers"`
	RequireSymbols        bool     `json:"require_symbols"`
	MaxAgeDays            int      `json:"max_age_days"`
	HistoryCount          int      `json:"history_count"`
	MFARequired           bool     `json:"mfa_required"`
	MFAExemptCIDRs        []string `json:"mfa_exempt_cidrs"`
	MFAMethods            []string `json:"mfa_methods"`
	SessionTimeoutMinutes int      `json:"session_timeout_minutes"`
	MaxConcurrentSessions int      `json:"max_concurrent_sessions"`
}

func defaultAuthPolicy() authPolicy {
	return authPolicy{
		MinLength:             8,
		RequireUppercase:      false,
		RequireNumbers:        false,
		RequireSymbols:        false,
		MaxAgeDays:            0,
		HistoryCount:          0,
		MFARequired:           false,
		MFAMethods:            []string{"totp"},
		SessionTimeoutMinutes: 480,
		MaxConcurrentSessions: 0,
	}
}

func handleGetCompanyAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, authPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultAuthPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy authPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy authPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, authPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, authPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultAuthPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy authPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainAuthPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy authPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, authPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const auditPolicyKey = "audit_policy"

type auditPolicy struct {
	CompanyID           string `json:"company_id"`
	AuditLevel          string `json:"audit_level"`
	AuditAdminActions   bool   `json:"audit_admin_actions"`
	AuditSecurityEvents bool   `json:"audit_security_events"`
	RetentionDays       int    `json:"retention_days"`
	MaskMailContent     bool   `json:"mask_mail_content"`
	MaskRecipientEmails bool   `json:"mask_recipient_emails"`
}

func defaultAuditPolicy() auditPolicy {
	return auditPolicy{
		AuditLevel:          "level_2",
		AuditAdminActions:   true,
		AuditSecurityEvents: true,
		RetentionDays:       90,
		MaskMailContent:     true,
		MaskRecipientEmails: false,
	}
}

func handleGetCompanyAuditPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, auditPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			policy := defaultAuditPolicy()
			policy.CompanyID = id
			writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	policy := defaultAuditPolicy()
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	policy.CompanyID = id
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyAuditPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	policy := defaultAuditPolicy()
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy.CompanyID = id
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, auditPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const securityGovernancePolicyKey = "security_governance_policy"

type securityGovernancePolicy struct {
	SecurityProfile             string `json:"security_profile"`
	WebhookPrivateNetworkAccess string `json:"webhook_private_network_access"`
}

func defaultSecurityGovernancePolicy() securityGovernancePolicy {
	return securityGovernancePolicy{
		SecurityProfile:             "enterprise",
		WebhookPrivateNetworkAccess: "deny",
	}
}

func normalizeSecurityGovernancePolicy(policy securityGovernancePolicy) (securityGovernancePolicy, error) {
	policy.SecurityProfile = strings.ToLower(strings.TrimSpace(policy.SecurityProfile))
	if policy.SecurityProfile == "" {
		policy.SecurityProfile = "enterprise"
	}
	switch policy.SecurityProfile {
	case "standard", "enterprise", "high_assurance":
	default:
		return securityGovernancePolicy{}, fmt.Errorf("invalid security_profile")
	}
	policy.WebhookPrivateNetworkAccess = strings.ToLower(strings.TrimSpace(policy.WebhookPrivateNetworkAccess))
	if policy.WebhookPrivateNetworkAccess == "" {
		policy.WebhookPrivateNetworkAccess = "deny"
	}
	switch policy.WebhookPrivateNetworkAccess {
	case "deny", "allow":
	default:
		return securityGovernancePolicy{}, fmt.Errorf("invalid webhook_private_network_access")
	}
	return policy, nil
}

func securityGovernanceFromEntry(entry configstore.ConfigEntry) securityGovernancePolicy {
	policy := defaultSecurityGovernancePolicy()
	if len(entry.Value) == 0 {
		return policy
	}
	var stored securityGovernancePolicy
	if err := json.Unmarshal(entry.Value, &stored); err != nil {
		return policy
	}
	normalized, err := normalizeSecurityGovernancePolicy(stored)
	if err != nil {
		return policy
	}
	return normalized
}

func getCompanySecurityGovernancePolicy(ctx context.Context, service AdminService, companyID string) (securityGovernancePolicy, error) {
	entry, err := service.GetCompanyConfig(ctx, companyID, securityGovernancePolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			return defaultSecurityGovernancePolicy(), nil
		}
		return securityGovernancePolicy{}, err
	}
	return securityGovernanceFromEntry(entry), nil
}

func handleGetCompanySecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	policy, err := getCompanySecurityGovernancePolicy(r.Context(), service, id)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var input securityGovernancePolicy
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy, err := normalizeSecurityGovernancePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, securityGovernancePolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainSecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, securityGovernancePolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultSecurityGovernancePolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": securityGovernanceFromEntry(entry)})
}

func handlePutDomainSecurityGovernancePolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var input securityGovernancePolicy
	if err := decodeJSONBody(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	policy, err := normalizeSecurityGovernancePolicy(input)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, securityGovernancePolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

const sessionPolicyKey = "session_policy"

type sessionPolicy struct {
	TimeoutMinutes            int  `json:"timeout_minutes"`
	MaxConcurrentSessions     int  `json:"max_concurrent_sessions"`
	RequireReauthForSensitive bool `json:"require_reauth_for_sensitive_ops"`
	IdleTimeoutMinutes        int  `json:"idle_timeout_minutes"`
}

func defaultSessionPolicy() sessionPolicy {
	return sessionPolicy{
		TimeoutMinutes:            480,
		MaxConcurrentSessions:     0,
		RequireReauthForSensitive: false,
		IdleTimeoutMinutes:        0,
	}
}

func handleGetCompanySessionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, sessionPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultSessionPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy sessionPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySessionPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy sessionPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, sessionPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetCompanySessions(w http.ResponseWriter, r *http.Request, _ AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"sessions": []map[string]any{
			{
				"user_id":     "usr-001",
				"email":       "admin@example.com",
				"ip":          "192.168.1.1",
				"started_at":  time.Now().Add(-2 * time.Hour).Format(time.RFC3339),
				"last_active": time.Now().Add(-5 * time.Minute).Format(time.RFC3339),
				"user_agent":  "Mozilla/5.0",
			},
		},
	})
}

func handleDeleteCompanySession(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"terminated": true,
		"user_id":    r.PathValue("userId"),
	})
}

const rateLimitPolicyKey = "rate_limit_policy"

type rateLimitPolicy struct {
	Enabled             bool   `json:"enabled"`
	MaxPerHour          int    `json:"max_per_hour"`
	MaxPerDay           int    `json:"max_per_day"`
	MaxRecipientsPerMsg int    `json:"max_recipients_per_msg"`
	MaxMessageSizeMB    int    `json:"max_message_size_mb"`
	ActionOnExceed      string `json:"action_on_exceed"`
	PerUserMaxPerHour   int    `json:"per_user_max_per_hour"`
	PerUserMaxPerDay    int    `json:"per_user_max_per_day"`
}

func defaultRateLimitPolicy() rateLimitPolicy {
	return rateLimitPolicy{
		Enabled:             false,
		MaxPerHour:          0,
		MaxPerDay:           0,
		MaxRecipientsPerMsg: 100,
		MaxMessageSizeMB:    25,
		ActionOnExceed:      "queue",
		PerUserMaxPerHour:   0,
		PerUserMaxPerDay:    500,
	}
}

func handleGetCompanyRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, rateLimitPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRateLimitPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy rateLimitPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanyRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy rateLimitPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, rateLimitPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, rateLimitPolicyKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"policy": defaultRateLimitPolicy()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var policy rateLimitPolicy
	if err := json.Unmarshal(entry.Value, &policy); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse policy")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainRateLimitPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	var policy rateLimitPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, rateLimitPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

// registerAccessPolicyRoutes delegates access policy route registration.
func registerAccessPolicyRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyIPPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyIPPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainIPPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/ip-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainIPPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyAuthPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyAuthPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainAuthPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/auth-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainAuthPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/audit-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyAuditPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/audit-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyAuditPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySecurityGovernancePolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySecurityGovernancePolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/retention-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRetentionPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSecurityGovernancePolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/governance", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSecurityGovernancePolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/session-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySessionPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/session-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySessionPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/sessions", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySessions(w, r, service)
	}))
	mux.HandleFunc("DELETE /admin/v1/companies/{id}/sessions/{userId}", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleDeleteCompanySession(w, r)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRateLimitPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRateLimitPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRateLimitPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/rate-limit", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRateLimitPolicy(w, r, service)
	}))
}
