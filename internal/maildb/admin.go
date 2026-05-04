package maildb

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/dnscheck"
	"github.com/gogomail/gogomail/internal/mail"
)

var ErrDeliveryRouteNotFound = errors.New("delivery route not found")

type stringArray []string

func (a stringArray) Value() (driver.Value, error) {
	var b strings.Builder
	b.WriteByte('{')
	for i, value := range a {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteByte('"')
		for _, r := range value {
			switch r {
			case '\\', '"':
				b.WriteByte('\\')
				b.WriteRune(r)
			default:
				b.WriteRune(r)
			}
		}
		b.WriteByte('"')
	}
	b.WriteByte('}')
	return b.String(), nil
}

func (a *stringArray) Scan(src any) error {
	switch value := src.(type) {
	case string:
		parsed, err := parsePostgresTextArray(value)
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	case []byte:
		parsed, err := parsePostgresTextArray(string(value))
		if err != nil {
			return err
		}
		*a = parsed
		return nil
	default:
		return fmt.Errorf("unsupported text array source %T", src)
	}
}

func parsePostgresTextArray(raw string) ([]string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "{}" {
		return nil, nil
	}
	if !strings.HasPrefix(raw, "{") || !strings.HasSuffix(raw, "}") {
		return nil, fmt.Errorf("invalid text array")
	}
	raw = strings.TrimSuffix(strings.TrimPrefix(raw, "{"), "}")
	var values []string
	var b strings.Builder
	inQuote := false
	escaped := false
	for _, r := range raw {
		if escaped {
			b.WriteRune(r)
			escaped = false
			continue
		}
		if inQuote && r == '\\' {
			escaped = true
			continue
		}
		if r == '"' {
			inQuote = !inQuote
			continue
		}
		if r == ',' && !inQuote {
			values = append(values, b.String())
			b.Reset()
			continue
		}
		b.WriteRune(r)
	}
	if inQuote || escaped {
		return nil, fmt.Errorf("invalid quoted text array")
	}
	values = append(values, b.String())
	return values, nil
}

type QueueStat struct {
	Topic  string `json:"topic"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
}

type QuotaUsageView struct {
	Scope      string    `json:"scope"`
	ID         string    `json:"id"`
	DomainID   string    `json:"domain_id,omitempty"`
	Name       string    `json:"name"`
	QuotaUsed  int64     `json:"quota_used"`
	QuotaLimit int64     `json:"quota_limit"`
	UsageRatio float64   `json:"usage_ratio"`
	OverLimit  bool      `json:"over_limit"`
	UpdatedAt  time.Time `json:"updated_at"`
}

type DeliveryAttemptView struct {
	ID              string    `json:"id"`
	MessageID       string    `json:"message_id"`
	RFCMessageID    string    `json:"rfc_message_id"`
	Farm            string    `json:"farm"`
	Recipient       string    `json:"recipient"`
	RecipientDomain string    `json:"recipient_domain"`
	Status          string    `json:"status"`
	ErrorMessage    string    `json:"error_message"`
	AttemptedAt     time.Time `json:"attempted_at"`
}

type SuppressionEntry struct {
	ID              string    `json:"id"`
	DomainID        string    `json:"domain_id"`
	Email           string    `json:"email"`
	Reason          string    `json:"reason"`
	SourceMessageID string    `json:"source_message_id"`
	CreatedAt       time.Time `json:"created_at"`
}

type TrustedRelayView struct {
	ID          string    `json:"id"`
	CIDR        string    `json:"cidr"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type DeliveryRouteView struct {
	ID            string    `json:"id"`
	DomainPattern string    `json:"domain_pattern"`
	Farm          string    `json:"farm"`
	Hosts         []string  `json:"hosts"`
	Port          int       `json:"port"`
	TLSMode       string    `json:"tls_mode"`
	ImplicitTLS   bool      `json:"implicit_tls"`
	SMTPHello     string    `json:"smtp_hello"`
	PoolName      string    `json:"pool_name"`
	AuthIdentity  string    `json:"auth_identity,omitempty"`
	AuthUsername  string    `json:"auth_username,omitempty"`
	AuthPassword  string    `json:"-"`
	Status        string    `json:"status"`
	Description   string    `json:"description"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type DeliveryRouteResolveView struct {
	Domain  string             `json:"domain"`
	Matched bool               `json:"matched"`
	Route   *DeliveryRouteView `json:"route,omitempty"`
}

type DomainDNSCheckView struct {
	ID        string                `json:"id"`
	DomainID  string                `json:"domain_id"`
	Status    string                `json:"status"`
	Report    dnscheck.DomainReport `json:"report"`
	CheckedAt time.Time             `json:"checked_at"`
}

type DomainView struct {
	ID                 string     `json:"id"`
	CompanyID          string     `json:"company_id"`
	Name               string     `json:"name"`
	NameACE            string     `json:"name_ace"`
	Status             string     `json:"status"`
	QuotaUsed          int64      `json:"quota_used"`
	QuotaLimit         int64      `json:"quota_limit,omitempty"`
	LastDNSCheckStatus string     `json:"last_dns_check_status,omitempty"`
	LastDNSCheckedAt   *time.Time `json:"last_dns_checked_at,omitempty"`
	CreatedAt          time.Time  `json:"created_at"`
}

type UserView struct {
	ID          string    `json:"id"`
	DomainID    string    `json:"domain_id"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Role        string    `json:"role"`
	Status      string    `json:"status"`
	QuotaUsed   int64     `json:"quota_used"`
	QuotaLimit  int64     `json:"quota_limit,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type DomainPolicyView struct {
	DomainID                string    `json:"domain_id"`
	InboundMode             string    `json:"inbound_mode"`
	OutboundMode            string    `json:"outbound_mode"`
	MaxRecipientsPerMessage int       `json:"max_recipients_per_message,omitempty"`
	MaxMessageBytes         int64     `json:"max_message_bytes,omitempty"`
	UpdatedAt               time.Time `json:"updated_at"`
}

type UpdateDomainStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateDomainQuotaRequest struct {
	ID         string `json:"id"`
	QuotaLimit int64  `json:"quota_limit"`
}

type UpdateDomainPolicyRequest struct {
	ID                      string `json:"id"`
	InboundMode             string `json:"inbound_mode"`
	OutboundMode            string `json:"outbound_mode"`
	MaxRecipientsPerMessage int    `json:"max_recipients_per_message,omitempty"`
	MaxMessageBytes         int64  `json:"max_message_bytes,omitempty"`
}

type CreateDomainRequest struct {
	CompanyID  string `json:"company_id"`
	Name       string `json:"name"`
	NameACE    string `json:"name_ace"`
	QuotaLimit int64  `json:"quota_limit,omitempty"`
}

type CreateUserRequest struct {
	DomainID    string `json:"domain_id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Address     string `json:"address"`
	QuotaLimit  int64  `json:"quota_limit,omitempty"`
}

type UpdateUserStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateUserQuotaRequest struct {
	ID         string `json:"id"`
	QuotaLimit int64  `json:"quota_limit"`
}

type CreateTrustedRelayRequest struct {
	CIDR        string `json:"cidr"`
	Description string `json:"description,omitempty"`
}

type CreateDeliveryRouteRequest struct {
	DomainPattern string   `json:"domain_pattern"`
	Farm          string   `json:"farm,omitempty"`
	Hosts         []string `json:"hosts"`
	Port          int      `json:"port,omitempty"`
	TLSMode       string   `json:"tls_mode,omitempty"`
	ImplicitTLS   bool     `json:"implicit_tls,omitempty"`
	SMTPHello     string   `json:"smtp_hello,omitempty"`
	PoolName      string   `json:"pool_name,omitempty"`
	AuthIdentity  string   `json:"auth_identity,omitempty"`
	AuthUsername  string   `json:"auth_username,omitempty"`
	AuthPassword  string   `json:"auth_password,omitempty"`
	Description   string   `json:"description,omitempty"`
}

type UpdateDeliveryRouteStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

func ValidateUpdateDomainStatusRequest(req UpdateDomainStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	switch normalizeAdminStatus(req.Status) {
	case "active", "suspended", "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported domain status %q", req.Status)
	}
}

func ValidateUpdateDomainQuotaRequest(req UpdateDomainQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func ValidateUpdateDomainPolicyRequest(req UpdateDomainPolicyRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("domain id is required")
	}
	if _, err := normalizeDomainPolicyMode(req.InboundMode); err != nil {
		return fmt.Errorf("inbound_mode %w", err)
	}
	if _, err := normalizeDomainPolicyMode(req.OutboundMode); err != nil {
		return fmt.Errorf("outbound_mode %w", err)
	}
	if req.MaxRecipientsPerMessage < 0 {
		return fmt.Errorf("max_recipients_per_message must not be negative")
	}
	if req.MaxMessageBytes < 0 {
		return fmt.Errorf("max_message_bytes must not be negative")
	}
	return nil
}

func normalizeDomainPolicyMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "inherit", nil
	}
	switch value {
	case "inherit", "monitor", "enforce":
		return value, nil
	default:
		return "", fmt.Errorf("must be inherit, monitor, or enforce")
	}
}

func ValidateCreateDomainRequest(req CreateDomainRequest) error {
	if strings.TrimSpace(req.CompanyID) == "" {
		return fmt.Errorf("company_id is required")
	}
	if strings.TrimSpace(req.Name) == "" {
		return fmt.Errorf("name is required")
	}
	if !validAdminDomainName(req.Name) {
		return fmt.Errorf("name must be a domain name")
	}
	if strings.TrimSpace(req.NameACE) != "" && !validAdminDomainName(req.NameACE) {
		return fmt.Errorf("name_ace must be a domain name")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func validAdminDomainName(name string) bool {
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 253 || strings.ContainsAny(name, " \t\r\n/\\") {
		return false
	}
	labels := strings.Split(name, ".")
	if len(labels) < 2 {
		return false
	}
	for _, label := range labels {
		if label == "" || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
			return false
		}
	}
	return true
}

func ValidateCreateUserRequest(req CreateUserRequest) error {
	if strings.TrimSpace(req.DomainID) == "" {
		return fmt.Errorf("domain_id is required")
	}
	if strings.TrimSpace(req.Username) == "" {
		return fmt.Errorf("username is required")
	}
	if !validAdminUsername(req.Username) {
		return fmt.Errorf("username must be a local account name")
	}
	if strings.TrimSpace(req.DisplayName) == "" {
		return fmt.Errorf("display_name is required")
	}
	if strings.TrimSpace(req.Address) == "" {
		return fmt.Errorf("address is required")
	}
	if _, err := mail.NormalizeAddress(req.Address); err != nil {
		return err
	}
	local, _, _ := strings.Cut(strings.ToLower(strings.TrimSpace(req.Address)), "@")
	if local != strings.ToLower(strings.TrimSpace(req.Username)) {
		return fmt.Errorf("address local part must match username")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func validAdminUsername(username string) bool {
	username = strings.TrimSpace(username)
	if username == "" || len(username) > 64 || strings.ContainsAny(username, " \t\r\n@/\\") {
		return false
	}
	if strings.HasPrefix(username, ".") || strings.HasSuffix(username, ".") || strings.Contains(username, "..") {
		return false
	}
	return true
}

func ValidateCreateTrustedRelayRequest(req CreateTrustedRelayRequest) error {
	if _, err := normalizeTrustedRelayCIDR(req.CIDR); err != nil {
		return err
	}
	if strings.ContainsAny(req.Description, "\r\n") {
		return fmt.Errorf("description must not contain newlines")
	}
	if len(req.Description) > 512 {
		return fmt.Errorf("description is too long")
	}
	return nil
}

func ValidateCreateDeliveryRouteRequest(req CreateDeliveryRouteRequest) error {
	if _, err := normalizeDeliveryRouteDomainPattern(req.DomainPattern); err != nil {
		return err
	}
	if _, err := normalizeDeliveryRouteHosts(req.Hosts); err != nil {
		return err
	}
	if req.Port != 0 && (req.Port < 1 || req.Port > 65535) {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if _, err := normalizeDeliveryRouteTLSMode(req.TLSMode); err != nil {
		return err
	}
	for field, value := range map[string]string{
		"farm":          req.Farm,
		"smtp_hello":    req.SMTPHello,
		"pool_name":     req.PoolName,
		"auth_identity": req.AuthIdentity,
		"auth_username": req.AuthUsername,
		"auth_password": req.AuthPassword,
		"description":   req.Description,
	} {
		if strings.ContainsAny(value, "\r\n") {
			return fmt.Errorf("%s must not contain newlines", field)
		}
	}
	if len(req.Description) > 512 {
		return fmt.Errorf("description is too long")
	}
	return nil
}

func ValidateUpdateDeliveryRouteStatusRequest(req UpdateDeliveryRouteStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("delivery route id is required")
	}
	switch strings.ToLower(strings.TrimSpace(req.Status)) {
	case "active", "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported delivery route status %q", req.Status)
	}
}

func normalizeDeliveryRouteDomainPattern(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "", fmt.Errorf("domain_pattern is required")
	}
	if value == "*" {
		return value, nil
	}
	if strings.HasPrefix(value, "*.") {
		suffix := strings.TrimPrefix(value, "*.")
		if !validAdminDomainName(suffix) {
			return "", fmt.Errorf("domain_pattern wildcard suffix must be a domain name")
		}
		return "*." + suffix, nil
	}
	if !validAdminDomainName(value) {
		return "", fmt.Errorf("domain_pattern must be a domain name, wildcard domain, or *")
	}
	return value, nil
}

func normalizeDeliveryRouteHosts(hosts []string) ([]string, error) {
	normalized := make([]string, 0, len(hosts))
	seen := make(map[string]struct{}, len(hosts))
	for _, host := range hosts {
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" || strings.ContainsAny(host, " \t\r\n/\\") {
			return nil, fmt.Errorf("hosts must contain DNS names or IP literals")
		}
		if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
			return nil, fmt.Errorf("hosts must not include ports")
		}
		host = strings.Trim(host, "[]")
		if host == "" {
			return nil, fmt.Errorf("hosts must contain DNS names or IP literals")
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		normalized = append(normalized, host)
	}
	if len(normalized) == 0 {
		return nil, fmt.Errorf("hosts is required")
	}
	return normalized, nil
}

func normalizeDeliveryRouteTLSMode(value string) (string, error) {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "opportunistic", nil
	}
	switch value {
	case "opportunistic", "require", "disable":
		return value, nil
	default:
		return "", fmt.Errorf("unsupported tls_mode %q", value)
	}
}

func normalizeTrustedRelayCIDR(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", fmt.Errorf("cidr is required")
	}
	if prefix, err := netip.ParsePrefix(value); err == nil {
		return prefix.Masked().String(), nil
	}
	addr, err := netip.ParseAddr(value)
	if err != nil {
		return "", fmt.Errorf("cidr must be an IP address or CIDR prefix")
	}
	if addr.Is4() {
		return netip.PrefixFrom(addr, 32).String(), nil
	}
	return netip.PrefixFrom(addr, 128).String(), nil
}

func (r *Repository) CreateDomain(ctx context.Context, req CreateDomainRequest) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDomainRequest(req); err != nil {
		return DomainView{}, err
	}
	name := strings.ToLower(strings.TrimSpace(req.Name))
	nameACE := strings.ToLower(strings.TrimSpace(req.NameACE))
	if nameACE == "" {
		nameACE = name
	}

	const query = `
INSERT INTO domains (company_id, name, name_ace, quota_limit)
VALUES ($1, $2, $3, NULLIF($4, 0))
RETURNING id::text, company_id::text, name, name_ace, status, quota_used, COALESCE(quota_limit, 0), created_at`

	var domain DomainView
	if err := r.db.QueryRowContext(ctx, query, strings.TrimSpace(req.CompanyID), name, nameACE, req.QuotaLimit).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.CreatedAt,
	); err != nil {
		return DomainView{}, fmt.Errorf("create domain: %w", err)
	}
	return domain, nil
}

func (r *Repository) CreateUser(ctx context.Context, req CreateUserRequest) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateUserRequest(req); err != nil {
		return UserView{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserView{}, fmt.Errorf("begin create user transaction: %w", err)
	}
	defer tx.Rollback()

	const insertUser = `
INSERT INTO users (domain_id, username, display_name, quota_limit)
VALUES ($1, $2, $3, NULLIF($4, 0))
RETURNING id::text, domain_id::text, username, display_name, role, status, quota_used, COALESCE(quota_limit, 0), created_at`

	var user UserView
	if err := tx.QueryRowContext(ctx, insertUser, strings.TrimSpace(req.DomainID), strings.TrimSpace(req.Username), strings.TrimSpace(req.DisplayName), req.QuotaLimit).Scan(
		&user.ID,
		&user.DomainID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.CreatedAt,
	); err != nil {
		return UserView{}, fmt.Errorf("create user: %w", err)
	}
	if err := createPrimaryAddress(ctx, tx, user.ID, user.DomainID, req.Address); err != nil {
		return UserView{}, err
	}
	if err := createSystemFolders(ctx, tx, user.ID); err != nil {
		return UserView{}, err
	}
	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("commit create user transaction: %w", err)
	}
	return user, nil
}

func createPrimaryAddress(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, userID string, domainID string, address string) error {
	address = strings.ToLower(strings.TrimSpace(address))
	local, domainACE, ok := strings.Cut(address, "@")
	if !ok || local == "" || domainACE == "" {
		return fmt.Errorf("address must be an email address")
	}
	if _, err := tx.ExecContext(ctx, `
INSERT INTO user_addresses (user_id, domain_id, local_part, local_part_ace, domain_ace, address, address_ace, is_primary)
VALUES ($1, $2, $3, $3, $4, $5, $5, true)`, userID, domainID, local, domainACE, address); err != nil {
		return fmt.Errorf("create primary user address: %w", err)
	}
	return nil
}

func createSystemFolders(ctx context.Context, tx interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}, userID string) error {
	folders := []struct {
		name       string
		systemType string
	}{
		{"Inbox", "inbox"},
		{"Drafts", "drafts"},
		{"Sent", "sent"},
		{"Trash", "trash"},
	}
	for i, folder := range folders {
		if _, err := tx.ExecContext(ctx, `
INSERT INTO folders (user_id, name, full_path, type, system_type, order_index)
VALUES ($1, $2, $3, 'system', $4, $5)
ON CONFLICT (user_id, full_path) DO NOTHING`, userID, folder.name, "/"+folder.name, folder.systemType, i); err != nil {
			return fmt.Errorf("create %s folder: %w", folder.systemType, err)
		}
	}
	return nil
}

func ValidateUpdateUserStatusRequest(req UpdateUserStatusRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	switch normalizeAdminStatus(req.Status) {
	case "active", "suspended", "disabled":
		return nil
	default:
		return fmt.Errorf("unsupported user status %q", req.Status)
	}
}

func ValidateUpdateUserQuotaRequest(req UpdateUserQuotaRequest) error {
	if strings.TrimSpace(req.ID) == "" {
		return fmt.Errorf("user id is required")
	}
	if req.QuotaLimit < 0 {
		return fmt.Errorf("quota_limit must not be negative")
	}
	return nil
}

func (r *Repository) UpdateDomainStatus(ctx context.Context, req UpdateDomainStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainStatusRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE domains
SET status = $2,
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status))
	if err != nil {
		return fmt.Errorf("update domain status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("domain %q not found", req.ID)
	}
	return nil
}

func (r *Repository) UpdateDomainQuota(ctx context.Context, req UpdateDomainQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainQuotaRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE domains
SET quota_limit = NULLIF($2, 0),
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), req.QuotaLimit)
	if err != nil {
		return fmt.Errorf("update domain quota: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("domain %q not found", req.ID)
	}
	return nil
}

func (r *Repository) UpdateDomainPolicy(ctx context.Context, req UpdateDomainPolicyRequest) (DomainPolicyView, error) {
	if r.db == nil {
		return DomainPolicyView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDomainPolicyRequest(req); err != nil {
		return DomainPolicyView{}, err
	}
	inboundMode, _ := normalizeDomainPolicyMode(req.InboundMode)
	outboundMode, _ := normalizeDomainPolicyMode(req.OutboundMode)
	policy := DomainPolicyView{
		DomainID:                strings.TrimSpace(req.ID),
		InboundMode:             inboundMode,
		OutboundMode:            outboundMode,
		MaxRecipientsPerMessage: req.MaxRecipientsPerMessage,
		MaxMessageBytes:         req.MaxMessageBytes,
	}
	policyJSON, err := json.Marshal(policy)
	if err != nil {
		return DomainPolicyView{}, fmt.Errorf("marshal domain policy: %w", err)
	}

	const query = `
UPDATE domains
SET settings = jsonb_set(settings, '{policy}', $2::jsonb, true),
    updated_at = now()
WHERE id = $1
RETURNING updated_at`
	if err := r.db.QueryRowContext(ctx, query, policy.DomainID, policyJSON).Scan(&policy.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return DomainPolicyView{}, fmt.Errorf("domain %q not found", req.ID)
		}
		return DomainPolicyView{}, fmt.Errorf("update domain policy: %w", err)
	}
	return policy, nil
}

func (r *Repository) UpdateUserStatus(ctx context.Context, req UpdateUserStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserStatusRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE users
SET status = $2,
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status))
	if err != nil {
		return fmt.Errorf("update user status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func (r *Repository) UpdateUserQuota(ctx context.Context, req UpdateUserQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserQuotaRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE users
SET quota_limit = NULLIF($2, 0),
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), req.QuotaLimit)
	if err != nil {
		return fmt.Errorf("update user quota: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func normalizeAdminStatus(status string) string {
	return strings.ToLower(strings.TrimSpace(status))
}

func (r *Repository) ListUsers(ctx context.Context, domainID string, limit int) ([]UserView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  domain_id::text,
  username,
  display_name,
  role,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM users
WHERE ($1 = '' OR domain_id::text = $1)
ORDER BY created_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, domainID, limit)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	defer rows.Close()

	var users []UserView
	for rows.Next() {
		var user UserView
		if err := rows.Scan(
			&user.ID,
			&user.DomainID,
			&user.Username,
			&user.DisplayName,
			&user.Role,
			&user.Status,
			&user.QuotaUsed,
			&user.QuotaLimit,
			&user.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan user: %w", err)
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate users: %w", err)
	}
	return users, nil
}

func (r *Repository) GetUser(ctx context.Context, id string) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return UserView{}, fmt.Errorf("user id is required")
	}

	const query = `
SELECT
  id::text,
  domain_id::text,
  username,
  display_name,
  role,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM users
WHERE id = $1
LIMIT 1`

	var user UserView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&user.ID,
		&user.DomainID,
		&user.Username,
		&user.DisplayName,
		&user.Role,
		&user.Status,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return UserView{}, fmt.Errorf("user %q not found", id)
		}
		return UserView{}, fmt.Errorf("get user: %w", err)
	}
	return user, nil
}

func (r *Repository) ListDomains(ctx context.Context, limit int) ([]DomainView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  d.id::text,
  d.company_id::text,
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC
  LIMIT 1
) latest ON true
ORDER BY d.created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []DomainView
	for rows.Next() {
		var domain DomainView
		var lastDNSCheckedAt sql.NullTime
		if err := rows.Scan(
			&domain.ID,
			&domain.CompanyID,
			&domain.Name,
			&domain.NameACE,
			&domain.Status,
			&domain.QuotaUsed,
			&domain.QuotaLimit,
			&domain.LastDNSCheckStatus,
			&lastDNSCheckedAt,
			&domain.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
		}
		if lastDNSCheckedAt.Valid {
			domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
		}
		domains = append(domains, domain)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domains: %w", err)
	}
	return domains, nil
}

func (r *Repository) GetDomain(ctx context.Context, id string) (DomainView, error) {
	if r.db == nil {
		return DomainView{}, fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return DomainView{}, fmt.Errorf("domain id is required")
	}

	const query = `
SELECT
  d.id::text,
  d.company_id::text,
  d.name,
  d.name_ace,
  d.status,
  d.quota_used,
  COALESCE(d.quota_limit, 0),
  COALESCE(latest.status, ''),
  latest.checked_at,
  d.created_at
FROM domains d
LEFT JOIN LATERAL (
  SELECT status, checked_at
  FROM domain_dns_checks
  WHERE domain_id = d.id
  ORDER BY checked_at DESC
  LIMIT 1
) latest ON true
WHERE d.id = $1
LIMIT 1`

	var domain DomainView
	var lastDNSCheckedAt sql.NullTime
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.LastDNSCheckStatus,
		&lastDNSCheckedAt,
		&domain.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DomainView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainView{}, fmt.Errorf("get domain: %w", err)
	}
	if lastDNSCheckedAt.Valid {
		domain.LastDNSCheckedAt = &lastDNSCheckedAt.Time
	}
	return domain, nil
}

func (r *Repository) ListDomainDNSChecks(ctx context.Context, domainID string, limit int) ([]DomainDNSCheckView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	domainID = strings.TrimSpace(domainID)
	if domainID == "" {
		return nil, fmt.Errorf("domain id is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  domain_id::text,
  status,
  report,
  checked_at
FROM domain_dns_checks
WHERE domain_id = $1
ORDER BY checked_at DESC
LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, domainID, limit)
	if err != nil {
		return nil, fmt.Errorf("list domain dns checks: %w", err)
	}
	defer rows.Close()

	var checks []DomainDNSCheckView
	for rows.Next() {
		var check DomainDNSCheckView
		var rawReport []byte
		if err := rows.Scan(
			&check.ID,
			&check.DomainID,
			&check.Status,
			&rawReport,
			&check.CheckedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain dns check: %w", err)
		}
		if err := json.Unmarshal(rawReport, &check.Report); err != nil {
			return nil, fmt.Errorf("decode domain dns check report: %w", err)
		}
		checks = append(checks, check)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate domain dns checks: %w", err)
	}
	return checks, nil
}

func (r *Repository) VerifyDomainDNS(ctx context.Context, id string) (dnscheck.DomainReport, error) {
	if r.db == nil {
		return dnscheck.DomainReport{}, fmt.Errorf("database handle is required")
	}
	domain, err := r.GetDomain(ctx, id)
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	keys, err := r.ListDKIMKeys(ctx, id, 200)
	if err != nil {
		return dnscheck.DomainReport{}, err
	}
	expectations := make([]dnscheck.DKIMExpectation, 0, len(keys))
	for _, key := range keys {
		if normalizeAdminStatus(key.Status) != "active" {
			continue
		}
		expectations = append(expectations, dnscheck.DKIMExpectation{
			Selector:     key.Selector,
			PublicKeyDNS: key.PublicKeyDNS,
		})
	}
	name := strings.TrimSpace(domain.NameACE)
	if name == "" {
		name = domain.Name
	}
	report := dnscheck.Verifier{}.VerifyDomain(ctx, name, expectations)
	if err := r.recordDomainDNSCheck(ctx, domain, report); err != nil {
		return dnscheck.DomainReport{}, err
	}
	return report, nil
}

func (r *Repository) recordDomainDNSCheck(ctx context.Context, domain DomainView, report dnscheck.DomainReport) error {
	reportJSON, err := json.Marshal(report)
	if err != nil {
		return fmt.Errorf("marshal domain dns check report: %w", err)
	}
	status := string(report.SummaryStatus())

	var checkID string
	if err := r.db.QueryRowContext(ctx, `
INSERT INTO domain_dns_checks (domain_id, status, report)
VALUES ($1, $2, $3)
RETURNING id::text`, domain.ID, status, reportJSON).Scan(&checkID); err != nil {
		return fmt.Errorf("record domain dns check: %w", err)
	}

	detailJSON, err := json.Marshal(map[string]any{
		"dns_check_id": checkID,
		"domain":       report.Domain,
		"status":       status,
	})
	if err != nil {
		return fmt.Errorf("marshal domain dns check audit detail: %w", err)
	}
	if _, err := r.db.ExecContext(ctx, `
INSERT INTO audit_logs (
  company_id, domain_id, category, action, target_type, target_id, result, detail
)
VALUES ($1, $2, 'admin', 'domain.dns_check', 'domain', $2, $3, $4)`,
		domain.CompanyID,
		domain.ID,
		status,
		detailJSON,
	); err != nil {
		return fmt.Errorf("record domain dns check audit: %w", err)
	}
	return nil
}

func (r *Repository) ListQueueStats(ctx context.Context) ([]QueueStat, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}

	const query = `
SELECT topic, status, count(*)
FROM outbox
GROUP BY topic, status
ORDER BY topic, status`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list queue stats: %w", err)
	}
	defer rows.Close()

	var stats []QueueStat
	for rows.Next() {
		var stat QueueStat
		if err := rows.Scan(&stat.Topic, &stat.Status, &stat.Count); err != nil {
			return nil, fmt.Errorf("scan queue stat: %w", err)
		}
		stats = append(stats, stat)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate queue stats: %w", err)
	}
	return stats, nil
}

func (r *Repository) ListQuotaUsage(ctx context.Context, limit int) ([]QuotaUsageView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT scope, id, domain_id, name, quota_used, quota_limit, updated_at
FROM (
  SELECT
    'domain' AS scope,
    id::text AS id,
    id::text AS domain_id,
    name AS name,
    quota_used,
    quota_limit,
    updated_at
  FROM domains
  WHERE quota_limit IS NOT NULL AND quota_limit > 0
  UNION ALL
  SELECT
    'user' AS scope,
    users.id::text AS id,
    users.domain_id::text AS domain_id,
    users.username || '@' || domains.name_ace AS name,
    users.quota_used,
    users.quota_limit,
    users.updated_at
  FROM users
  JOIN domains ON domains.id = users.domain_id
  WHERE users.quota_limit IS NOT NULL AND users.quota_limit > 0
) usage
ORDER BY (quota_used::double precision / quota_limit::double precision) DESC, updated_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list quota usage: %w", err)
	}
	defer rows.Close()

	var usages []QuotaUsageView
	for rows.Next() {
		var usage QuotaUsageView
		if err := rows.Scan(
			&usage.Scope,
			&usage.ID,
			&usage.DomainID,
			&usage.Name,
			&usage.QuotaUsed,
			&usage.QuotaLimit,
			&usage.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan quota usage: %w", err)
		}
		usage.UsageRatio = quotaUsageRatio(usage.QuotaUsed, usage.QuotaLimit)
		usage.OverLimit = usage.QuotaLimit > 0 && usage.QuotaUsed >= usage.QuotaLimit
		usages = append(usages, usage)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate quota usage: %w", err)
	}
	return usages, nil
}

func quotaUsageRatio(used int64, limit int64) float64 {
	if limit <= 0 {
		return 0
	}
	if used <= 0 {
		return 0
	}
	return float64(used) / float64(limit)
}

func (r *Repository) ListDeliveryAttempts(ctx context.Context, limit int) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  recipient,
  recipient_domain,
  status,
  error_message,
  attempted_at
FROM delivery_attempts
ORDER BY attempted_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := rows.Scan(
			&attempt.ID,
			&attempt.MessageID,
			&attempt.RFCMessageID,
			&attempt.Farm,
			&attempt.Recipient,
			&attempt.RecipientDomain,
			&attempt.Status,
			&attempt.ErrorMessage,
			&attempt.AttemptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) ListExhaustedAttempts(ctx context.Context, limit int) ([]DeliveryAttemptView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  message_id::text,
  rfc_message_id,
  farm,
  recipient,
  recipient_domain,
  status,
  error_message,
  attempted_at
FROM delivery_attempts
WHERE status = 'exhausted'
ORDER BY attempted_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list exhausted delivery attempts: %w", err)
	}
	defer rows.Close()

	var attempts []DeliveryAttemptView
	for rows.Next() {
		var attempt DeliveryAttemptView
		if err := rows.Scan(
			&attempt.ID,
			&attempt.MessageID,
			&attempt.RFCMessageID,
			&attempt.Farm,
			&attempt.Recipient,
			&attempt.RecipientDomain,
			&attempt.Status,
			&attempt.ErrorMessage,
			&attempt.AttemptedAt,
		); err != nil {
			return nil, fmt.Errorf("scan exhausted delivery attempt: %w", err)
		}
		attempts = append(attempts, attempt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate exhausted delivery attempts: %w", err)
	}
	return attempts, nil
}

func (r *Repository) ListSuppressionEntries(ctx context.Context, limit int) ([]SuppressionEntry, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  COALESCE(domain_id::text, ''),
  email,
  reason,
  COALESCE(source_message_id::text, ''),
  created_at
FROM suppression_list
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list suppression entries: %w", err)
	}
	defer rows.Close()

	var entries []SuppressionEntry
	for rows.Next() {
		var entry SuppressionEntry
		if err := rows.Scan(
			&entry.ID,
			&entry.DomainID,
			&entry.Email,
			&entry.Reason,
			&entry.SourceMessageID,
			&entry.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan suppression entry: %w", err)
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate suppression entries: %w", err)
	}
	return entries, nil
}

func (r *Repository) ListTrustedRelays(ctx context.Context, limit int) ([]TrustedRelayView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  cidr::text,
  description,
  created_at
FROM trusted_relays
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list trusted relays: %w", err)
	}
	defer rows.Close()

	var relays []TrustedRelayView
	for rows.Next() {
		var relay TrustedRelayView
		if err := rows.Scan(&relay.ID, &relay.CIDR, &relay.Description, &relay.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan trusted relay: %w", err)
		}
		relays = append(relays, relay)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate trusted relays: %w", err)
	}
	return relays, nil
}

func (r *Repository) CreateTrustedRelay(ctx context.Context, req CreateTrustedRelayRequest) (TrustedRelayView, error) {
	if r.db == nil {
		return TrustedRelayView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateTrustedRelayRequest(req); err != nil {
		return TrustedRelayView{}, err
	}
	cidr, err := normalizeTrustedRelayCIDR(req.CIDR)
	if err != nil {
		return TrustedRelayView{}, err
	}

	const query = `
INSERT INTO trusted_relays (cidr, description)
VALUES ($1, $2)
RETURNING id::text, cidr::text, description, created_at`

	var relay TrustedRelayView
	if err := r.db.QueryRowContext(ctx, query, cidr, strings.TrimSpace(req.Description)).Scan(
		&relay.ID,
		&relay.CIDR,
		&relay.Description,
		&relay.CreatedAt,
	); err != nil {
		return TrustedRelayView{}, fmt.Errorf("create trusted relay: %w", err)
	}
	return relay, nil
}

func (r *Repository) DeleteTrustedRelay(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("trusted relay id is required")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM trusted_relays WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete trusted relay: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("trusted relay %q not found", id)
	}
	return nil
}

func (r *Repository) ListDeliveryRoutes(ctx context.Context, limit int) ([]DeliveryRouteView, error) {
	if r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	limit = normalizeLimit(limit)

	const query = `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list delivery routes: %w", err)
	}
	defer rows.Close()

	var routes []DeliveryRouteView
	for rows.Next() {
		var route DeliveryRouteView
		if err := rows.Scan(
			&route.ID,
			&route.DomainPattern,
			&route.Farm,
			(*stringArray)(&route.Hosts),
			&route.Port,
			&route.TLSMode,
			&route.ImplicitTLS,
			&route.SMTPHello,
			&route.PoolName,
			&route.AuthIdentity,
			&route.AuthUsername,
			&route.Status,
			&route.Description,
			&route.CreatedAt,
			&route.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan delivery route: %w", err)
		}
		routes = append(routes, route)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate delivery routes: %w", err)
	}
	return routes, nil
}

func (r *Repository) CreateDeliveryRoute(ctx context.Context, req CreateDeliveryRouteRequest) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateCreateDeliveryRouteRequest(req); err != nil {
		return DeliveryRouteView{}, err
	}
	domainPattern, err := normalizeDeliveryRouteDomainPattern(req.DomainPattern)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	hosts, err := normalizeDeliveryRouteHosts(req.Hosts)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	tlsMode, err := normalizeDeliveryRouteTLSMode(req.TLSMode)
	if err != nil {
		return DeliveryRouteView{}, err
	}
	port := req.Port
	if port == 0 {
		port = 25
	}

	const query = `
INSERT INTO delivery_routes (
  domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, auth_password,
  description
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
RETURNING
  id::text, domain_pattern, farm, hosts, port, tls_mode, implicit_tls,
  smtp_hello, pool_name, auth_identity, auth_username, status, description,
  created_at, updated_at`

	var route DeliveryRouteView
	if err := r.db.QueryRowContext(
		ctx,
		query,
		domainPattern,
		strings.TrimSpace(req.Farm),
		stringArray(hosts),
		port,
		tlsMode,
		req.ImplicitTLS,
		strings.TrimSpace(req.SMTPHello),
		strings.TrimSpace(req.PoolName),
		strings.TrimSpace(req.AuthIdentity),
		strings.TrimSpace(req.AuthUsername),
		strings.TrimSpace(req.AuthPassword),
		strings.TrimSpace(req.Description),
	).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		return DeliveryRouteView{}, fmt.Errorf("create delivery route: %w", err)
	}
	return route, nil
}

func (r *Repository) UpdateDeliveryRouteStatus(ctx context.Context, req UpdateDeliveryRouteStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateDeliveryRouteStatusRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE delivery_routes
SET status = $2,
    updated_at = now()
WHERE id = $1`, strings.TrimSpace(req.ID), strings.ToLower(strings.TrimSpace(req.Status)))
	if err != nil {
		return fmt.Errorf("update delivery route status: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("delivery route %q not found", req.ID)
	}
	return nil
}

func (r *Repository) DeliveryRouteForDomain(ctx context.Context, domain string) (DeliveryRouteView, error) {
	if r.db == nil {
		return DeliveryRouteView{}, fmt.Errorf("database handle is required")
	}
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteView{}, fmt.Errorf("domain must be a domain name")
	}

	const query = `
SELECT
  id::text,
  domain_pattern,
  farm,
  hosts,
  port,
  tls_mode,
  implicit_tls,
  smtp_hello,
  pool_name,
  auth_identity,
  auth_username,
  auth_password,
  status,
  description,
  created_at,
  updated_at
FROM delivery_routes
WHERE status = 'active'
  AND (
    domain_pattern = $1
    OR domain_pattern = '*'
    OR (
      left(domain_pattern, 2) = '*.'
      AND right($1, length(domain_pattern) - 1) = substring(domain_pattern from 2)
    )
  )
ORDER BY
  CASE
    WHEN domain_pattern = $1 THEN 0
    WHEN left(domain_pattern, 2) = '*.' THEN 1
    ELSE 2
  END,
  length(domain_pattern) DESC,
  created_at DESC
LIMIT 1`

	var route DeliveryRouteView
	if err := r.db.QueryRowContext(ctx, query, domain).Scan(
		&route.ID,
		&route.DomainPattern,
		&route.Farm,
		(*stringArray)(&route.Hosts),
		&route.Port,
		&route.TLSMode,
		&route.ImplicitTLS,
		&route.SMTPHello,
		&route.PoolName,
		&route.AuthIdentity,
		&route.AuthUsername,
		&route.AuthPassword,
		&route.Status,
		&route.Description,
		&route.CreatedAt,
		&route.UpdatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DeliveryRouteView{}, ErrDeliveryRouteNotFound
		}
		return DeliveryRouteView{}, fmt.Errorf("get delivery route for domain: %w", err)
	}
	return route, nil
}

func (r *Repository) ResolveDeliveryRoute(ctx context.Context, domain string) (DeliveryRouteResolveView, error) {
	domain = strings.ToLower(strings.TrimSpace(domain))
	if !validAdminDomainName(domain) {
		return DeliveryRouteResolveView{}, fmt.Errorf("domain must be a domain name")
	}
	route, err := r.DeliveryRouteForDomain(ctx, domain)
	if err != nil {
		if errors.Is(err, ErrDeliveryRouteNotFound) {
			return DeliveryRouteResolveView{Domain: domain, Matched: false}, nil
		}
		return DeliveryRouteResolveView{}, err
	}
	return DeliveryRouteResolveView{Domain: domain, Matched: true, Route: &route}, nil
}

func (r *Repository) DeleteDeliveryRoute(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("delivery route id is required")
	}
	result, err := r.db.ExecContext(ctx, `DELETE FROM delivery_routes WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete delivery route: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("delivery route %q not found", id)
	}
	return nil
}

func (r *Repository) RetryOutbox(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `
UPDATE outbox
SET status = 'pending',
    attempts = 0,
    last_error = NULL,
    locked_at = NULL,
    available_at = now(),
    processed_at = NULL
WHERE id = $1`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("outbox event %q not found", id)
	}
	return nil
}

func (r *Repository) DeleteSuppressionEntry(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}

	const query = `DELETE FROM suppression_list WHERE id = $1`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("delete suppression entry: %w", err)
	}
	affected, err := result.RowsAffected()
	if err == nil && affected == 0 {
		return fmt.Errorf("suppression entry %q not found", id)
	}
	return nil
}
