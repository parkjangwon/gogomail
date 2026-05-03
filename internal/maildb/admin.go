package maildb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/mail"
)

type QueueStat struct {
	Topic  string `json:"topic"`
	Status string `json:"status"`
	Count  int64  `json:"count"`
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

type DomainView struct {
	ID         string    `json:"id"`
	CompanyID  string    `json:"company_id"`
	Name       string    `json:"name"`
	NameACE    string    `json:"name_ace"`
	Status     string    `json:"status"`
	QuotaUsed  int64     `json:"quota_used"`
	QuotaLimit int64     `json:"quota_limit,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
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

type UpdateDomainStatusRequest struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

type UpdateDomainQuotaRequest struct {
	ID         string `json:"id"`
	QuotaLimit int64  `json:"quota_limit"`
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
  id::text,
  company_id::text,
  name,
  name_ace,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM domains
ORDER BY created_at DESC
LIMIT $1`

	rows, err := r.db.QueryContext(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("list domains: %w", err)
	}
	defer rows.Close()

	var domains []DomainView
	for rows.Next() {
		var domain DomainView
		if err := rows.Scan(
			&domain.ID,
			&domain.CompanyID,
			&domain.Name,
			&domain.NameACE,
			&domain.Status,
			&domain.QuotaUsed,
			&domain.QuotaLimit,
			&domain.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan domain: %w", err)
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
  id::text,
  company_id::text,
  name,
  name_ace,
  status,
  quota_used,
  COALESCE(quota_limit, 0),
  created_at
FROM domains
WHERE id = $1
LIMIT 1`

	var domain DomainView
	if err := r.db.QueryRowContext(ctx, query, id).Scan(
		&domain.ID,
		&domain.CompanyID,
		&domain.Name,
		&domain.NameACE,
		&domain.Status,
		&domain.QuotaUsed,
		&domain.QuotaLimit,
		&domain.CreatedAt,
	); err != nil {
		if err == sql.ErrNoRows {
			return DomainView{}, fmt.Errorf("domain %q not found", id)
		}
		return DomainView{}, fmt.Errorf("get domain: %w", err)
	}
	return domain, nil
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
