package maildb

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gogomail/gogomail/internal/audit"
)

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

	quotaSource := "default"
	if req.QuotaLimit > 0 {
		quotaSource = "custom"
	}

	const insertUser = `
INSERT INTO users (domain_id, username, display_name, recovery_email, password_hash, quota_limit, quota_source, must_change_password)
SELECT
  d.id,
  $2,
  $3,
  $4,
  NULLIF($5, ''),
  CASE
    WHEN $6::bigint > 0 THEN $6::bigint
    ELSE NULLIF(COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0), 0)
  END,
  $7,
  $8
FROM domains d
WHERE d.id = $1
RETURNING id::text, domain_id::text, username, display_name, recovery_email, role, status, COALESCE(password_hash, '') <> '', must_change_password, quota_used, COALESCE(quota_limit, 0), quota_source, created_at`

	var user UserView
	recoveryEmail, _ := normalizeRecoveryEmail(req.RecoveryEmail)
	if err := tx.QueryRowContext(ctx, insertUser, strings.TrimSpace(req.DomainID), strings.TrimSpace(req.Username), strings.TrimSpace(req.DisplayName), recoveryEmail, strings.TrimSpace(req.PasswordHash), req.QuotaLimit, quotaSource, req.MustChangePassword).Scan(
		&user.ID,
		&user.DomainID,
		&user.Username,
		&user.DisplayName,
		&user.RecoveryEmail,
		&user.Role,
		&user.Status,
		&user.PasswordConfigured,
		&user.MustChangePassword,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.QuotaSource,
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
	var companyID string
	if err := tx.QueryRowContext(ctx, `SELECT company_id::text FROM domains WHERE id = $1`, user.DomainID).Scan(&companyID); err != nil {
		return UserView{}, fmt.Errorf("lookup user company for audit: %w", err)
	}
	detail, err := userCreateAuditDetail(userCreateAuditView{
		User:      user,
		CompanyID: companyID,
		Address:   strings.ToLower(strings.TrimSpace(req.Address)),
	})
	if err != nil {
		return UserView{}, err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  companyID,
		DomainID:   user.DomainID,
		UserID:     user.ID,
		Category:   "admin",
		Action:     "user.create",
		TargetType: "user",
		TargetID:   user.ID,
		Result:     "created",
		Detail:     detail,
	}); err != nil {
		return UserView{}, fmt.Errorf("record user create audit: %w", err)
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
		{"Spam", "spam"},
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

func (r *Repository) EnsureSystemFolders(ctx context.Context, userID string) error {
	return createSystemFolders(ctx, r.db, userID)
}

func (r *Repository) UpdateUserStatus(ctx context.Context, req UpdateUserStatusRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserStatusRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user status transaction: %w", err)
	}
	defer tx.Rollback()

	var view userStatusAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET status = $2,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, u.status`, strings.TrimSpace(req.ID), normalizeAdminStatus(req.Status)).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.Status,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user status: %w", err)
	}
	detail, err := userStatusAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.status_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     view.Status,
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user status audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user status transaction: %w", err)
	}
	return nil
}

func (r *Repository) DeleteUser(ctx context.Context, id string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	req := DeleteUserRequest{ID: strings.TrimSpace(id)}
	if err := ValidateDeleteUserRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE users
		    SET status = 'disabled',
		        session_version = session_version + 1,
		        updated_at = now()
		  WHERE id = $1::uuid`,
		req.ID,
	)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete user rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func (r *Repository) BulkUpdateUserStatus(ctx context.Context, req BulkUpdateUserStatusRequest) (BulkUpdateUserStatusResult, error) {
	if r.db == nil {
		return BulkUpdateUserStatusResult{}, fmt.Errorf("database handle is required")
	}
	if err := ValidateBulkUpdateUserStatusRequest(req); err != nil {
		return BulkUpdateUserStatusResult{}, err
	}
	ids := dedupeTrimmedStrings(req.IDs)
	status := normalizeAdminStatus(req.Status)
	companyID := strings.TrimSpace(req.CompanyID)

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return BulkUpdateUserStatusResult{}, fmt.Errorf("begin bulk user status transaction: %w", err)
	}
	defer tx.Rollback()

	query, args := buildBulkUpdateUserStatusQuery(ids, status, companyID)
	rows, err := tx.QueryContext(ctx, query, args...)
	if err != nil {
		return BulkUpdateUserStatusResult{}, fmt.Errorf("bulk update user status: %w", err)
	}
	defer rows.Close()

	updatedSet := make(map[string]struct{}, len(ids))
	var result BulkUpdateUserStatusResult
	for rows.Next() {
		var view userStatusAuditView
		if err := rows.Scan(&view.ID, &view.DomainID, &view.CompanyID, &view.Username, &view.Status); err != nil {
			return BulkUpdateUserStatusResult{}, fmt.Errorf("scan bulk user status update: %w", err)
		}
		detail, err := userStatusAuditDetail(view)
		if err != nil {
			return BulkUpdateUserStatusResult{}, err
		}
		if err := audit.InsertTx(ctx, tx, audit.Log{
			CompanyID:  view.CompanyID,
			DomainID:   view.DomainID,
			UserID:     view.ID,
			Category:   "admin",
			Action:     "user.status_update",
			TargetType: "user",
			TargetID:   view.ID,
			Result:     view.Status,
			Detail:     detail,
		}); err != nil {
			return BulkUpdateUserStatusResult{}, fmt.Errorf("record bulk user status audit: %w", err)
		}
		updatedSet[view.ID] = struct{}{}
		result.Updated = append(result.Updated, view.ID)
	}
	if err := rows.Err(); err != nil {
		return BulkUpdateUserStatusResult{}, fmt.Errorf("iterate bulk user status update: %w", err)
	}
	for _, id := range ids {
		if _, ok := updatedSet[id]; !ok {
			result.Failed = append(result.Failed, id)
		}
	}
	if err := tx.Commit(); err != nil {
		return BulkUpdateUserStatusResult{}, fmt.Errorf("commit bulk user status transaction: %w", err)
	}
	return result, nil
}

func buildBulkUpdateUserStatusQuery(ids []string, status, companyID string) (string, []any) {
	args := []any{stringArray(ids), status}
	companyPredicate := ""
	if companyID != "" {
		args = append(args, companyID)
		companyPredicate = fmt.Sprintf("    AND d.company_id = $%d::uuid\n", len(args))
	}

	query := `
WITH input AS (
  SELECT id::uuid
  FROM unnest($1::text[]) AS id
),
updated AS (
  UPDATE users u
  SET status = $2,
      updated_at = now()
  FROM domains d, input i
  WHERE u.domain_id = d.id
    AND u.id = i.id
` + companyPredicate + `  RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, u.status
)
SELECT id, domain_id, company_id, username, status
FROM updated
ORDER BY id`
	return query, args
}

func (r *Repository) UpdateUserQuota(ctx context.Context, req UpdateUserQuotaRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserQuotaRequest(req); err != nil {
		return err
	}
	quotaSource, _ := normalizeQuotaSource(req.QuotaSource, "custom")

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user quota transaction: %w", err)
	}
	defer tx.Rollback()

	var view userQuotaAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET quota_limit = CASE
      WHEN $3 = 'default' THEN NULLIF(COALESCE((d.settings #>> '{policy,default_user_quota}')::bigint, 0), 0)
      ELSE NULLIF($2::bigint, 0)
    END,
    quota_source = $3,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, COALESCE(u.quota_limit, 0), u.quota_source`, strings.TrimSpace(req.ID), req.QuotaLimit, quotaSource).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.QuotaLimit,
		&view.QuotaSource,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user quota: %w", err)
	}
	detail, err := userQuotaAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.quota_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user quota audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user quota transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateUserPasswordHash(ctx context.Context, req UpdateUserPasswordHashRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserPasswordHashRequest(req); err != nil {
		return err
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin user password hash transaction: %w", err)
	}
	defer tx.Rollback()

	var view userCredentialAuditView
	if err := tx.QueryRowContext(ctx, `
UPDATE users u
SET password_hash = $2,
    updated_at = now()
FROM domains d
WHERE u.domain_id = d.id
  AND u.id = $1
RETURNING u.id::text, u.domain_id::text, d.company_id::text, u.username, COALESCE(u.password_hash, '') <> ''`, strings.TrimSpace(req.ID), strings.TrimSpace(req.PasswordHash)).Scan(
		&view.ID,
		&view.DomainID,
		&view.CompanyID,
		&view.Username,
		&view.PasswordConfigured,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("user %q not found", req.ID)
		}
		return fmt.Errorf("update user password hash: %w", err)
	}
	detail, err := userCredentialAuditDetail(view)
	if err != nil {
		return err
	}
	if err := audit.InsertTx(ctx, tx, audit.Log{
		CompanyID:  view.CompanyID,
		DomainID:   view.DomainID,
		UserID:     view.ID,
		Category:   "admin",
		Action:     "user.password_update",
		TargetType: "user",
		TargetID:   view.ID,
		Result:     "updated",
		Detail:     detail,
	}); err != nil {
		return fmt.Errorf("record user password hash audit: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit user password hash transaction: %w", err)
	}
	return nil
}

func (r *Repository) UpdateUserRole(ctx context.Context, req UpdateUserRoleRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserRoleRequest(req); err != nil {
		return err
	}
	result, err := r.db.ExecContext(ctx, `
UPDATE users SET role = $2, updated_at = now() WHERE id = $1::uuid`, strings.TrimSpace(req.ID), req.Role)
	if err != nil {
		return fmt.Errorf("update user role: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

func (r *Repository) UpdateUserRecoveryEmail(ctx context.Context, req UpdateUserRecoveryEmailRequest) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserRecoveryEmailRequest(req); err != nil {
		return err
	}
	recoveryEmail, _ := normalizeRecoveryEmail(req.RecoveryEmail)
	result, err := r.db.ExecContext(ctx, `
UPDATE users SET recovery_email = $2, updated_at = now() WHERE id = $1::uuid`, strings.TrimSpace(req.ID), recoveryEmail)
	if err != nil {
		return fmt.Errorf("update user recovery email: %w", err)
	}
	n, _ := result.RowsAffected()
	if n == 0 {
		return fmt.Errorf("user %q not found", req.ID)
	}
	return nil
}

type userStatusAuditView struct {
	ID        string
	DomainID  string
	CompanyID string
	Username  string
	Status    string
}

func userStatusAuditDetail(view userStatusAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":    view.ID,
		"domain_id":  view.DomainID,
		"company_id": view.CompanyID,
		"username":   view.Username,
		"status":     view.Status,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user status audit detail: %w", err)
	}
	return detail, nil
}

type userCreateAuditView struct {
	User      UserView
	CompanyID string
	Address   string
}

func userCreateAuditDetail(view userCreateAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":             view.User.ID,
		"domain_id":           view.User.DomainID,
		"company_id":          view.CompanyID,
		"username":            view.User.Username,
		"display_name":        view.User.DisplayName,
		"address":             view.Address,
		"role":                view.User.Role,
		"status":              view.User.Status,
		"password_configured": view.User.PasswordConfigured,
		"quota_limit":         view.User.QuotaLimit,
		"quota_source":        view.User.QuotaSource,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user create audit detail: %w", err)
	}
	return detail, nil
}

type userCredentialAuditView struct {
	ID                 string
	DomainID           string
	CompanyID          string
	Username           string
	PasswordConfigured bool
}

func userCredentialAuditDetail(view userCredentialAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":             view.ID,
		"domain_id":           view.DomainID,
		"company_id":          view.CompanyID,
		"username":            view.Username,
		"password_configured": view.PasswordConfigured,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user credential audit detail: %w", err)
	}
	return detail, nil
}
type userQuotaAuditView struct {
	ID          string
	DomainID    string
	CompanyID   string
	Username    string
	QuotaLimit  int64
	QuotaSource string
}

func userQuotaAuditDetail(view userQuotaAuditView) (json.RawMessage, error) {
	detail, err := json.Marshal(map[string]any{
		"user_id":      view.ID,
		"domain_id":    view.DomainID,
		"company_id":   view.CompanyID,
		"username":     view.Username,
		"quota_limit":  view.QuotaLimit,
		"quota_source": view.QuotaSource,
	})
	if err != nil {
		return nil, fmt.Errorf("marshal user quota audit detail: %w", err)
	}
	return detail, nil
}
func (r *Repository) ListUsers(ctx context.Context, req UserListRequest) ([]UserView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	if err := ValidateUserListRequest(req); err != nil {
		return nil, false, err
	}
	limit := normalizeLimit(req.Limit)
	status := normalizeAdminStatus(req.Status)
	queryLimit := limit
	if req.ProbeMore {
		queryLimit = limit + 1
	}

	query, args := buildListUsersQuery(req, status, queryLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list users: %w", err)
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
			&user.RecoveryEmail,
			&user.Role,
			&user.Status,
			&user.PasswordConfigured,
			&user.MustChangePassword,
			&user.QuotaUsed,
			&user.QuotaLimit,
			&user.QuotaSource,
			&user.CreatedAt,
		); err != nil {
			return nil, false, fmt.Errorf("scan user: %w", err)
		}
		user.QuotaRemaining = quotaRemaining(user.QuotaUsed, user.QuotaLimit)
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate users: %w", err)
	}
	hasMore := req.ProbeMore && len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	return users, hasMore, nil
}

func buildListUsersQuery(req UserListRequest, status string, queryLimit int) (string, []any) {
	args := make([]any, 0, 5)
	where := make([]string, 0, 4)
	joinDomains := false
	if companyID := strings.TrimSpace(req.CompanyID); companyID != "" {
		joinDomains = true
		args = append(args, companyID)
		where = append(where, fmt.Sprintf("d.company_id = $%d::uuid", len(args)))
	}
	if domainID := strings.TrimSpace(req.DomainID); domainID != "" {
		args = append(args, domainID)
		where = append(where, fmt.Sprintf("u.domain_id = $%d::uuid", len(args)))
	}
	if status != "" {
		args = append(args, status)
		where = append(where, fmt.Sprintf("u.status = $%d", len(args)))
	}
	if req.PasswordConfigured != nil {
		args = append(args, *req.PasswordConfigured)
		where = append(where, fmt.Sprintf("(COALESCE(u.password_hash, '') <> '') = $%d::boolean", len(args)))
	}
	args = append(args, queryLimit)

	var builder strings.Builder
	builder.WriteString(`
SELECT
  u.id::text,
  u.domain_id::text,
  u.username,
  u.display_name,
  COALESCE(u.recovery_email, ''),
  u.role,
  u.status,
  COALESCE(u.password_hash, '') <> '' AS password_configured,
  u.must_change_password,
  u.quota_used,
  COALESCE(u.quota_limit, 0),
  u.quota_source,
  u.created_at
FROM users u
`)
	if joinDomains {
		builder.WriteString("JOIN domains d ON d.id = u.domain_id\n")
	}
	if len(where) > 0 {
		builder.WriteString("WHERE ")
		builder.WriteString(strings.Join(where, "\n  AND "))
		builder.WriteByte('\n')
	}
	builder.WriteString(fmt.Sprintf("ORDER BY u.created_at DESC, u.id DESC\nLIMIT $%d", len(args)))
	return builder.String(), args
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
  COALESCE(recovery_email, ''),
  role,
  status,
  COALESCE(password_hash, '') <> '' AS password_configured,
  must_change_password,
  quota_used,
  COALESCE(quota_limit, 0),
  quota_source,
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
		&user.RecoveryEmail,
		&user.Role,
		&user.Status,
		&user.PasswordConfigured,
		&user.MustChangePassword,
		&user.QuotaUsed,
		&user.QuotaLimit,
		&user.QuotaSource,
		&user.CreatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return UserView{}, fmt.Errorf("user %q not found", id)
		}
		return UserView{}, fmt.Errorf("get user: %w", err)
	}
	user.QuotaRemaining = quotaRemaining(user.QuotaUsed, user.QuotaLimit)
	return user, nil
}

// AdminUserView is a simplified view of an admin-role user.
type AdminUserView struct {
	ID        string    `json:"id"`
	DomainID  string    `json:"domain_id"`
	CompanyID string    `json:"company_id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminUserListRequest is the filter/pagination request for ListAdminUsers.
type AdminUserListRequest struct {
	CompanyID string
	Limit     int
	ProbeMore bool // when true the query fetches Limit+1 to detect has_more
}

// ListAdminUsers returns users whose role is system_admin or company_admin.
// If req.CompanyID is non-empty, only users in domains belonging to that company are returned.
// When req.ProbeMore is true an extra row is fetched to populate the has_more return value.
func (r *Repository) ListAdminUsers(ctx context.Context, req AdminUserListRequest) ([]AdminUserView, bool, error) {
	if r.db == nil {
		return nil, false, fmt.Errorf("database handle is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1000
	}
	queryLimit := limit
	if req.ProbeMore {
		queryLimit = limit + 1
	}
	query, args := buildListAdminUsersQuery(req.CompanyID, queryLimit)
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, false, fmt.Errorf("list admin users: %w", err)
	}
	defer rows.Close()

	var users []AdminUserView
	for rows.Next() {
		var u AdminUserView
		if err := rows.Scan(
			&u.ID,
			&u.DomainID,
			&u.CompanyID,
			&u.Username,
			&u.Email,
			&u.Role,
			&u.Status,
			&u.CreatedAt,
		); err != nil {
			return nil, false, fmt.Errorf("scan admin user: %w", err)
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, false, fmt.Errorf("iterate admin users: %w", err)
	}
	hasMore := req.ProbeMore && len(users) > limit
	if hasMore {
		users = users[:limit]
	}
	if users == nil {
		users = []AdminUserView{}
	}
	return users, hasMore, nil
}

func buildListAdminUsersQuery(companyID string, queryLimit int) (string, []any) {
	args := make([]any, 0, 2)
	where := []string{"u.role IN ('system_admin', 'company_admin')"}
	if companyID = strings.TrimSpace(companyID); companyID != "" {
		args = append(args, companyID)
		where = append(where, fmt.Sprintf("d.company_id = $%d::uuid", len(args)))
	}
	args = append(args, queryLimit)

	query := `
SELECT
  u.id::text,
  u.domain_id::text,
  d.company_id::text,
  u.username,
  COALESCE(ua.address, u.username),
  u.role,
  u.status,
  u.created_at
FROM users u
JOIN domains d ON d.id = u.domain_id
LEFT JOIN user_addresses ua ON ua.user_id = u.id AND ua.address LIKE u.username || '@%'
WHERE ` + strings.Join(where, "\n  AND ") + `
ORDER BY u.created_at DESC, u.id DESC
LIMIT $` + fmt.Sprint(len(args))
	return query, args
}

// SetUserRole updates the role of a user by ID. The role must be a valid value.
func (r *Repository) SetUserRole(ctx context.Context, userID, role string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	if err := ValidateUpdateUserRoleRequest(UpdateUserRoleRequest{ID: userID, Role: role}); err != nil {
		return err
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET role = $2, updated_at = now() WHERE id = $1::uuid`,
		strings.TrimSpace(userID), role)
	if err != nil {
		return fmt.Errorf("set user role: %w", err)
	}
	return nil
}

// ClearUserAdminRole removes admin role from a user, resetting it to the default 'user' role.
func (r *Repository) ClearUserAdminRole(ctx context.Context, userID string) error {
	if r.db == nil {
		return fmt.Errorf("database handle is required")
	}
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return fmt.Errorf("user id is required")
	}
	_, err := r.db.ExecContext(ctx,
		`UPDATE users SET role = 'user', updated_at = now() WHERE id = $1::uuid AND role IN ('system_admin', 'company_admin')`,
		userID)
	if err != nil {
		return fmt.Errorf("clear admin role: %w", err)
	}
	return nil
}
type InviteToken struct {
	ID         string     `json:"id"`
	UserID     string     `json:"user_id"`
	DomainID   string     `json:"domain_id"`
	Token      string     `json:"token"`
	ExpiresAt  time.Time  `json:"expires_at"`
	AcceptedAt *time.Time `json:"accepted_at,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
	CreatedBy  string     `json:"created_by,omitempty"`
}

func (r *Repository) CreateInviteToken(ctx context.Context, userID, createdBy string) (InviteToken, error) {
	if r.db == nil {
		return InviteToken{}, fmt.Errorf("database handle is required")
	}
	rawToken := make([]byte, 32)
	if _, err := rand.Read(rawToken); err != nil {
		return InviteToken{}, fmt.Errorf("generate invite token: %w", err)
	}
	token := hex.EncodeToString(rawToken)
	expiresAt := time.Now().Add(72 * time.Hour)

	var it InviteToken
	err := r.db.QueryRowContext(ctx, `
INSERT INTO user_invite_tokens (user_id, domain_id, token, expires_at, created_by)
SELECT u.id, u.domain_id, $2, $3, NULLIF($4, '')::uuid
FROM users u WHERE u.id = $1
RETURNING id::text, user_id::text, domain_id::text, token, expires_at, accepted_at, created_at, COALESCE(created_by::text, '')`,
		userID, token, expiresAt, createdBy,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt, &it.CreatedAt, &it.CreatedBy)
	if err != nil {
		return InviteToken{}, fmt.Errorf("create invite token: %w", err)
	}
	return it, nil
}

func (r *Repository) GetInviteToken(ctx context.Context, token string) (InviteToken, error) {
	if r.db == nil {
		return InviteToken{}, fmt.Errorf("database handle is required")
	}
	var it InviteToken
	err := r.db.QueryRowContext(ctx, `
SELECT id::text, user_id::text, domain_id::text, token, expires_at, accepted_at, created_at, COALESCE(created_by::text, '')
FROM user_invite_tokens WHERE token = $1`, token,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt, &it.CreatedAt, &it.CreatedBy)
	if errors.Is(err, sql.ErrNoRows) {
		return InviteToken{}, fmt.Errorf("invite token not found")
	}
	if err != nil {
		return InviteToken{}, fmt.Errorf("get invite token: %w", err)
	}
	return it, nil
}

func (r *Repository) AcceptInviteToken(ctx context.Context, token, passwordHash string) (UserView, error) {
	if r.db == nil {
		return UserView{}, fmt.Errorf("database handle is required")
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return UserView{}, fmt.Errorf("begin accept invite transaction: %w", err)
	}
	defer tx.Rollback()

	var it InviteToken
	err = tx.QueryRowContext(ctx, `
SELECT id::text, user_id::text, domain_id::text, token, expires_at, accepted_at
FROM user_invite_tokens WHERE token = $1 FOR UPDATE`, token,
	).Scan(&it.ID, &it.UserID, &it.DomainID, &it.Token, &it.ExpiresAt, &it.AcceptedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return UserView{}, fmt.Errorf("invite token not found")
	}
	if err != nil {
		return UserView{}, fmt.Errorf("lookup invite token: %w", err)
	}
	if it.AcceptedAt != nil {
		return UserView{}, fmt.Errorf("invite token already accepted")
	}
	if time.Now().After(it.ExpiresAt) {
		return UserView{}, fmt.Errorf("invite token expired")
	}

	var user UserView
	err = tx.QueryRowContext(ctx, `
UPDATE users SET password_hash = $2, must_change_password = false, status = 'active'
WHERE id = $1
RETURNING id::text, domain_id::text, username, display_name, role, status,
          COALESCE(password_hash, '') <> '', must_change_password,
          quota_used, COALESCE(quota_limit, 0), quota_source, created_at`,
		it.UserID, passwordHash,
	).Scan(&user.ID, &user.DomainID, &user.Username, &user.DisplayName,
		&user.Role, &user.Status, &user.PasswordConfigured, &user.MustChangePassword,
		&user.QuotaUsed, &user.QuotaLimit, &user.QuotaSource, &user.CreatedAt)
	if err != nil {
		return UserView{}, fmt.Errorf("set user password: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `UPDATE user_invite_tokens SET accepted_at = now() WHERE id = $1`, it.ID); err != nil {
		return UserView{}, fmt.Errorf("mark invite accepted: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return UserView{}, fmt.Errorf("commit accept invite: %w", err)
	}
	return user, nil
}
