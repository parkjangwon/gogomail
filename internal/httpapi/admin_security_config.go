package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/configstore"
	"github.com/gogomail/gogomail/internal/maildb"
	"github.com/gogomail/gogomail/internal/spamfilter"
	webhookguard "github.com/gogomail/gogomail/internal/webhook"
)

const dmarcSpfPolicyKey = "dmarc_spf_policy"

type dmarcSpfPolicy struct {
	DMARCPolicy     string   `json:"dmarc_policy"`
	DMARCPct        int      `json:"dmarc_pct"`
	DMARCRua        string   `json:"dmarc_rua"`
	DMARCRuf        string   `json:"dmarc_ruf"`
	DMARCSubdomains string   `json:"dmarc_subdomains"`
	DMARCAlignMode  string   `json:"dmarc_align_mode"`
	SPFIncludes     []string `json:"spf_includes"`
	SPFAllMechanism string   `json:"spf_all_mechanism"`
	SPFIP4List      []string `json:"spf_ip4_list"`
}

func defaultDmarcSpfPolicy() dmarcSpfPolicy {
	return dmarcSpfPolicy{
		DMARCPolicy:     "quarantine",
		DMARCPct:        100,
		DMARCSubdomains: "none",
		DMARCAlignMode:  "r",
		SPFIncludes:     []string{},
		SPFAllMechanism: "~all",
		SPFIP4List:      []string{},
	}
}

func buildDmarcRecord(p dmarcSpfPolicy) string {
	record := fmt.Sprintf("v=DMARC1; p=%s; pct=%d; adkim=%s; aspf=%s", p.DMARCPolicy, p.DMARCPct, p.DMARCAlignMode, p.DMARCAlignMode)
	if p.DMARCRua != "" {
		record += "; rua=mailto:" + p.DMARCRua
	}
	if p.DMARCRuf != "" {
		record += "; ruf=mailto:" + p.DMARCRuf
	}
	if p.DMARCSubdomains != "none" && p.DMARCSubdomains != "" {
		record += "; sp=" + p.DMARCSubdomains
	}
	return record
}

func buildSpfRecord(p dmarcSpfPolicy) string {
	parts := []string{"v=spf1"}
	for _, inc := range p.SPFIncludes {
		parts = append(parts, "include:"+inc)
	}
	for _, ip := range p.SPFIP4List {
		parts = append(parts, "ip4:"+ip)
	}
	parts = append(parts, p.SPFAllMechanism)
	return strings.Join(parts, " ")
}

// ─── Spam / Content Filter Policy ────────────────────────────────────────────

const spamFilterPolicyKey = "spam_filter_policy"

func defaultSpamFilterPolicy() spamfilter.Policy {
	return spamfilter.DefaultPolicy()
}

func handleGetCompanySpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, spamFilterPolicyKey)
	policy := defaultSpamFilterPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		policy = spamfilter.NormalizePolicy(policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutCompanySpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy spamfilter.Policy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.SpamThreshold < 1 || policy.SpamThreshold > 10 {
		writeError(w, http.StatusBadRequest, "spam_threshold must be 1-10")
		return
	}
	if policy.MaxAttachmentMB < 0 {
		writeError(w, http.StatusBadRequest, "max_attachment_mb must be >= 0")
		return
	}
	policy = spamfilter.NormalizePolicy(policy)
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainSpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, spamFilterPolicyKey)
	policy := defaultSpamFilterPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		policy = spamfilter.NormalizePolicy(policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainSpamFilterPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy spamfilter.Policy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.SpamThreshold < 1 || policy.SpamThreshold > 10 {
		writeError(w, http.StatusBadRequest, "spam_threshold must be 1-10")
		return
	}
	if policy.MaxAttachmentMB < 0 {
		writeError(w, http.StatusBadRequest, "max_attachment_mb must be >= 0")
		return
	}
	policy = spamfilter.NormalizePolicy(policy)
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, spamFilterPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleListCompanySpamFilterEvents(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectUnknownQueryKeys(w, r, "limit", "domain_id", "user_id", "from_addr", "to_addr", "subject", "flow_status", "since", "until") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	limit, ok := parseQueryLimit(w, r)
	if !ok {
		return
	}
	req, ok := parseMailFlowLogListRequest(w, r, limit)
	if !ok {
		return
	}
	req.CompanyID = id
	req.Direction = string(maildb.MailFlowDirectionInbound)
	if strings.TrimSpace(req.FlowStatus) == "" {
		req.FlowStatus = string(maildb.MailFlowStatusFiltered)
	}
	logs, err := service.ListMailFlowLogs(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spam_filter_events": logs})
}

func handleGetCompanySpamFilterStats(w http.ResponseWriter, r *http.Request, service AdminService) {
	if !rejectUnknownQueryKeys(w, r, "domain_id", "user_id", "since", "until") {
		return
	}
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	req, ok := parseMailFlowLogStatsRequest(w, r)
	if !ok {
		return
	}
	req.CompanyID = id
	req.Direction = string(maildb.MailFlowDirectionInbound)
	stats, err := service.GetMailFlowLogStats(r.Context(), req)
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"spam_filter_stats": stats})
}

// ─── Quota Summary ────────────────────────────────────────────────────────────

func handleGetCompanyQuotaSummary(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}

	quotaItems, err := service.ListQuotaUsage(r.Context(), maildb.QuotaUsageListRequest{Limit: 1000})
	if err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	// Filter to this company — quota items have domain_id; filter by company id via scope or keep all if no filter
	var totalUsed, totalLimit int64
	var overLimitCount int
	for _, q := range quotaItems {
		totalUsed += q.QuotaUsed
		totalLimit += q.QuotaLimit
		if q.OverLimit {
			overLimitCount++
		}
	}

	// Top 5 by usage (already sorted descending by the DB query)
	top := quotaItems
	if len(top) > 5 {
		top = top[:5]
	}

	usageRatio := 0.0
	if totalLimit > 0 {
		usageRatio = float64(totalUsed) / float64(totalLimit)
	}

	_ = id // company scoping handled by service layer
	writeJSON(w, http.StatusOK, map[string]any{
		"summary": map[string]any{
			"total_entries":     len(quotaItems),
			"total_used_bytes":  totalUsed,
			"total_limit_bytes": totalLimit,
			"over_limit_count":  overLimitCount,
			"usage_ratio":       usageRatio,
		},
		"top_consumers": top,
	})
}

// ─── Routing Rules ────────────────────────────────────────────────────────────

const routingRulesKey = "routing_rules"

type routingRule struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Enabled      bool   `json:"enabled"`
	Priority     int    `json:"priority"`
	MatchFrom    string `json:"match_from"`
	MatchTo      string `json:"match_to"`
	MatchSubject string `json:"match_subject"`
	Action       string `json:"action"`
	ActionValue  string `json:"action_value"`
}

type routingRulesConfig struct {
	Rules []routingRule `json:"rules"`
}

func handleGetCompanyRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, routingRulesKey)
	cfg := routingRulesConfig{Rules: []routingRule{}}
	if err == nil {
		_ = json.Unmarshal(entry.Value, &cfg)
		if cfg.Rules == nil {
			cfg.Rules = []routingRule{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handlePutCompanyRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var cfg routingRulesConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if cfg.Rules == nil {
		cfg.Rules = []routingRule{}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal rules")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, routingRulesKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handleGetDomainRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, routingRulesKey)
	cfg := routingRulesConfig{Rules: []routingRule{}}
	if err == nil {
		_ = json.Unmarshal(entry.Value, &cfg)
		if cfg.Rules == nil {
			cfg.Rules = []routingRule{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

func handlePutDomainRoutingRules(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var cfg routingRulesConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if cfg.Rules == nil {
		cfg.Rules = []routingRule{}
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal rules")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, routingRulesKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"rules": cfg.Rules})
}

// ─── SSO / SAML Configuration ─────────────────────────────────────────────────

const ssoConfigKey = "sso_config"

type ssoConfig struct {
	Enabled        bool   `json:"enabled"`
	Provider       string `json:"provider"`
	EntityID       string `json:"entity_id"`
	MetadataURL    string `json:"metadata_url"`
	SSOLoginURL    string `json:"sso_login_url"`
	Certificate    string `json:"certificate"`
	AttributeEmail string `json:"attribute_email"`
	AttributeName  string `json:"attribute_name"`
	ForceSSO       bool   `json:"force_sso"`
	AutoProvision  bool   `json:"auto_provision"`
	DefaultRole    string `json:"default_role"`
}

func defaultSSOConfig() ssoConfig {
	return ssoConfig{
		Enabled:        false,
		Provider:       "saml",
		EntityID:       "",
		MetadataURL:    "",
		SSOLoginURL:    "",
		Certificate:    "",
		AttributeEmail: "email",
		AttributeName:  "displayName",
		ForceSSO:       false,
		AutoProvision:  false,
		DefaultRole:    "viewer",
	}
}

func handleGetCompanySSOConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ssoConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"config": defaultSSOConfig()})
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var cfg ssoConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse sso config")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func handlePutCompanySSOConfig(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var cfg ssoConfig
	if err := decodeJSONBody(r, &cfg); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	b, err := json.Marshal(cfg)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal sso config")
		return
	}
	if _, err := service.SetCompanyConfig(r.Context(), id, ssoConfigKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"config": cfg})
}

func handlePostCompanySSOTest(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	if err := requiresCompanyAccess(r.Context(), id); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetCompanyConfig(r.Context(), id, ssoConfigKey)
	if err != nil {
		if errors.Is(err, configstore.ErrConfigNotFound) {
			writeError(w, http.StatusBadRequest, "SSO is not configured")
			return
		}
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	var cfg ssoConfig
	if err := json.Unmarshal(entry.Value, &cfg); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to parse sso config")
		return
	}
	if cfg.MetadataURL == "" && cfg.SSOLoginURL == "" {
		writeError(w, http.StatusBadRequest, "metadata_url or sso_login_url is required")
		return
	}

	if cfg.MetadataURL != "" {
		// Validate URL syntax first.
		if _, err := url.Parse(cfg.MetadataURL); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("invalid metadata URL: %v", err),
			})
			return
		}
		// Guard against SSRF: reject private/loopback/link-local targets.
		if _, err := webhookguard.ValidateOutboundHTTPURL(r.Context(), cfg.MetadataURL, webhookguard.OutboundURLGuardOptions{}); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": "metadata_url is not allowed: " + err.Error(),
			})
			return
		}
		// Actually fetch the metadata endpoint.
		client := &http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(cfg.MetadataURL)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("failed to reach metadata endpoint: %v", err),
			})
			return
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("metadata endpoint returned HTTP %d", resp.StatusCode),
			})
			return
		}
		ct := resp.Header.Get("Content-Type")
		if !strings.Contains(ct, "xml") && !strings.Contains(ct, "saml") && !strings.Contains(ct, "text") {
			writeJSON(w, http.StatusOK, map[string]any{
				"success": false,
				"message": fmt.Sprintf("unexpected content type %q (expected XML/SAML metadata)", ct),
			})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"message": "SSO metadata endpoint is reachable and returned a valid response",
		})
		return
	}

	// No metadata URL — validate the login URL syntax.
	if _, err := url.Parse(cfg.SSOLoginURL); err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": false,
			"message": fmt.Sprintf("invalid SSO login URL: %v", err),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"message": "SSO login URL is valid",
	})
}

// ─── Outbound SMTP Policy ─────────────────────────────────────────────────────

const smtpPolicyKey = "smtp_policy"

type smtpPolicy struct {
	TLSRequired          bool     `json:"tls_required"`
	TLSMinVersion        string   `json:"tls_min_version"`
	STARTTLSEnabled      bool     `json:"starttls_enabled"`
	DedicatedIPEnabled   bool     `json:"dedicated_ip_enabled"`
	DedicatedIPs         []string `json:"dedicated_ips"`
	RetryCount           int      `json:"retry_count"`
	RetryIntervalMinutes int      `json:"retry_interval_minutes"`
	ConnectionTimeout    int      `json:"connection_timeout_seconds"`
	HELOHostname         string   `json:"helo_hostname"`
	BounceAddress        string   `json:"bounce_address"`
}

func defaultSMTPPolicy() smtpPolicy {
	return smtpPolicy{
		TLSRequired:          false,
		TLSMinVersion:        "tls1.2",
		STARTTLSEnabled:      true,
		DedicatedIPEnabled:   false,
		DedicatedIPs:         []string{},
		RetryCount:           3,
		RetryIntervalMinutes: 60,
		ConnectionTimeout:    30,
		HELOHostname:         "",
		BounceAddress:        "",
	}
}

func handleGetDomainSMTPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, smtpPolicyKey)
	policy := defaultSMTPPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
		if policy.DedicatedIPs == nil {
			policy.DedicatedIPs = []string{}
		}
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handlePutDomainSMTPPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy smtpPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.RetryCount < 0 || policy.RetryCount > 10 {
		writeError(w, http.StatusBadRequest, "retry_count must be 0-10")
		return
	}
	if policy.DedicatedIPs == nil {
		policy.DedicatedIPs = []string{}
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal smtp policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, smtpPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"policy": policy})
}

func handleGetDomainDmarcSpfPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	entry, err := service.GetDomainConfig(r.Context(), id, dmarcSpfPolicyKey)
	policy := defaultDmarcSpfPolicy()
	if err == nil {
		_ = json.Unmarshal(entry.Value, &policy)
	} else if !errors.Is(err, configstore.ErrConfigNotFound) {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policy,
		"generated_records": map[string]any{
			"dmarc":      buildDmarcRecord(policy),
			"spf":        buildSpfRecord(policy),
			"dmarc_host": "_dmarc.<domain>",
			"spf_host":   "<domain>",
		},
	})
}

func handlePutDomainDmarcSpfPolicy(w http.ResponseWriter, r *http.Request, service AdminService) {
	defer r.Body.Close()
	id, ok := parseBoundedAdminPathValue(w, r, "id")
	if !ok {
		return
	}
	domain, err := service.GetDomain(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "domain not found")
		return
	}
	if err := requiresCompanyAccess(r.Context(), domain.CompanyID); err != nil {
		writeError(w, http.StatusForbidden, "access denied")
		return
	}
	var policy dmarcSpfPolicy
	if err := decodeJSONBody(r, &policy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	if policy.DMARCPct < 0 || policy.DMARCPct > 100 {
		writeError(w, http.StatusBadRequest, "dmarc_pct must be 0-100")
		return
	}
	b, err := json.Marshal(policy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to marshal policy")
		return
	}
	if _, err := service.SetDomainConfig(r.Context(), id, dmarcSpfPolicyKey, json.RawMessage(b), false, 0); err != nil {
		slog.ErrorContext(r.Context(), "admin handler error", "error", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"policy": policy,
		"generated_records": map[string]any{
			"dmarc":      buildDmarcRecord(policy),
			"spf":        buildSpfRecord(policy),
			"dmarc_host": "_dmarc.<domain>",
			"spf_host":   "<domain>",
		},
	})
}

// registerSecurityConfigRoutes delegates security config route registration.
func registerSecurityConfigRoutes(mux *http.ServeMux, service AdminService, adminAuth func(http.HandlerFunc) http.HandlerFunc) {
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/dmarc-spf", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainDmarcSpfPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/dmarc-spf", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainDmarcSpfPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySpamFilterPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySpamFilterPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter/events", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleListCompanySpamFilterEvents(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/security/spam-filter/stats", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySpamFilterStats(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSpamFilterPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/security/spam-filter", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSpamFilterPolicy(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/quota-summary", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyQuotaSummary(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanyRoutingRules(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanyRoutingRules(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainRoutingRules(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/routing-rules", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainRoutingRules(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/companies/{id}/sso/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetCompanySSOConfig(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/companies/{id}/sso/config", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutCompanySSOConfig(w, r, service)
	}))
	mux.HandleFunc("POST /admin/v1/companies/{id}/sso/test", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePostCompanySSOTest(w, r, service)
	}))
	mux.HandleFunc("GET /admin/v1/domains/{id}/smtp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handleGetDomainSMTPPolicy(w, r, service)
	}))
	mux.HandleFunc("PUT /admin/v1/domains/{id}/smtp-policy", adminAuth(func(w http.ResponseWriter, r *http.Request) {
		handlePutDomainSMTPPolicy(w, r, service)
	}))
}

