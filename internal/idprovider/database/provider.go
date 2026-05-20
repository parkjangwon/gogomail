package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/gogomail/gogomail/internal/idprovider"
	"github.com/gogomail/gogomail/internal/maildb"
)

// Provider implements idprovider.IdentityProvider using the maildb database.
type Provider struct {
	db *sql.DB
	mr *maildb.Repository
}

// New creates a new database identity provider.
func New(db *sql.DB, mr *maildb.Repository) *Provider {
	return &Provider{
		db: db,
		mr: mr,
	}
}

// GetUser retrieves a user by ID.
func (p *Provider) GetUser(ctx context.Context, userID string) (*idprovider.User, error) {
	view, err := p.mr.GetUser(ctx, userID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("user not found")
		}
		return nil, err
	}

	return &idprovider.User{
		ID:            view.ID,
		DomainID:      view.DomainID,
		Username:      view.Username,
		DisplayName:   view.DisplayName,
		RecoveryEmail: view.RecoveryEmail,
		AuthSource:    "local",
		Role:          view.Role,
		Status:        view.Status,
		Settings:      make(map[string]interface{}),
		CreatedAt:     view.CreatedAt,
		UpdatedAt:     view.CreatedAt,
	}, nil
}

// GetGroup retrieves a group by ID.
func (p *Provider) GetGroup(ctx context.Context, groupID string) (*idprovider.Group, error) {
	row := p.db.QueryRowContext(ctx, `
		SELECT id, company_id, domain_id, org_id, name, slug, description, status, settings, created_at, updated_at
		FROM directory_groups WHERE id = $1
	`, groupID)

	var group idprovider.Group
	var settings json.RawMessage
	var orgID *string

	err := row.Scan(&group.ID, nil, &group.DomainID, &orgID, &group.Name, &group.Slug, &group.Description, &group.Status, &settings, &group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("group not found")
		}
		return nil, err
	}

	group.OrgID = orgID
	if err := json.Unmarshal(settings, &group.Settings); err != nil {
		group.Settings = make(map[string]interface{})
	}

	return &group, nil
}

// ListUsers lists users matching the filter.
func (p *Provider) ListUsers(ctx context.Context, filter *idprovider.UserFilter) ([]*idprovider.User, error) {
	req := maildb.UserListRequest{
		Limit: 100,
	}

	if filter != nil && filter.Limit > 0 {
		req.Limit = filter.Limit
	}

	views, _, err := p.mr.ListUsers(ctx, req)
	if err != nil {
		return nil, err
	}

	users := make([]*idprovider.User, len(views))
	for i, view := range views {
		users[i] = &idprovider.User{
			ID:            view.ID,
			DomainID:      view.DomainID,
			Username:      view.Username,
			DisplayName:   view.DisplayName,
			RecoveryEmail: view.RecoveryEmail,
			AuthSource:    "local",
			Role:          view.Role,
			Status:        view.Status,
			Settings:      make(map[string]interface{}),
			CreatedAt:     view.CreatedAt,
			UpdatedAt:     view.CreatedAt,
		}
	}

	return users, nil
}

// ListGroups lists groups matching the filter.
func (p *Provider) ListGroups(ctx context.Context, filter *idprovider.GroupFilter) ([]*idprovider.Group, error) {
	query, args := buildListGroupsQuery(filter)
	rows, err := p.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var groups []*idprovider.Group
	for rows.Next() {
		var g idprovider.Group
		var settings json.RawMessage
		var orgID *string
		var domainID string

		err := rows.Scan(&g.ID, &domainID, &orgID, &g.Name, &g.Slug, &g.Description, &g.Status, &settings, &g.CreatedAt, &g.UpdatedAt)
		if err != nil {
			return nil, err
		}

		g.DomainID = domainID
		g.OrgID = orgID
		if err := json.Unmarshal(settings, &g.Settings); err != nil {
			g.Settings = make(map[string]interface{})
		}

		groups = append(groups, &g)
	}

	return groups, rows.Err()
}

func buildListGroupsQuery(filter *idprovider.GroupFilter) (string, []any) {
	query := `
		SELECT id, domain_id, org_id, name, slug, description, status, settings, created_at, updated_at
		FROM directory_groups WHERE status = 'active'
	`
	args := []any{}
	if filter != nil && filter.OrgID != nil {
		args = append(args, *filter.OrgID)
		query += fmt.Sprintf(" AND org_id = $%d", len(args))
	}
	if filter != nil && filter.SearchQuery != nil {
		search := strings.ToLower(strings.TrimSpace(*filter.SearchQuery))
		if search != "" {
			args = append(args, "%"+search+"%")
			placeholder := fmt.Sprintf("$%d", len(args))
			query += " AND (lower(name) LIKE " + placeholder + " OR lower(slug) LIKE " + placeholder + " OR lower(description) LIKE " + placeholder + ")"
		}
	}
	limit := 100
	if filter != nil && filter.Limit > 0 {
		limit = filter.Limit
	}
	args = append(args, limit)
	query += fmt.Sprintf(" ORDER BY lower(name), id LIMIT $%d", len(args))
	if filter != nil && filter.Offset > 0 {
		args = append(args, filter.Offset)
		query += fmt.Sprintf(" OFFSET $%d", len(args))
	}
	return query, args
}

// CreateUser creates a new user.
func (p *Provider) CreateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.DomainID == "" || user.Username == "" || user.DisplayName == "" {
		return fmt.Errorf("invalid user: missing required fields")
	}

	result, err := p.db.ExecContext(ctx, `
		INSERT INTO users (domain_id, org_id, username, display_name, recovery_email, auth_source, role, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`, user.DomainID, user.OrgID, user.Username, user.DisplayName, user.RecoveryEmail, user.AuthSource, user.Role, user.Status)

	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check creation result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user creation failed: no rows affected")
	}

	return nil
}

// UpdateUser updates an existing user.
func (p *Provider) UpdateUser(ctx context.Context, user *idprovider.User) error {
	if user == nil || user.ID == "" {
		return fmt.Errorf("invalid user: missing id")
	}

	result, err := p.db.ExecContext(ctx, `
		UPDATE users SET display_name = $1, recovery_email = $2, role = $3, status = $4, updated_at = now()
		WHERE id = $5
	`, user.DisplayName, user.RecoveryEmail, user.Role, user.Status, user.ID)

	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check update result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// DeleteUser deletes a user (soft delete by marking as deleted).
func (p *Provider) DeleteUser(ctx context.Context, userID string) error {
	if userID == "" {
		return fmt.Errorf("invalid user id")
	}

	result, err := p.db.ExecContext(ctx, `
		UPDATE users SET status = 'deleted', updated_at = now() WHERE id = $1
	`, userID)

	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check delete result: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("user not found")
	}

	return nil
}

// CreateGroup creates a new group.
func (p *Provider) CreateGroup(ctx context.Context, group *idprovider.Group) error {
	settings, _ := json.Marshal(group.Settings)

	_, err := p.db.ExecContext(ctx, `
		INSERT INTO directory_groups (domain_id, org_id, name, slug, description, status, settings)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, group.DomainID, group.OrgID, group.Name, group.Slug, group.Description, group.Status, settings)

	return err
}

// DeleteGroup deletes a group.
func (p *Provider) DeleteGroup(ctx context.Context, groupID string) error {
	_, err := p.db.ExecContext(ctx, `
		UPDATE directory_groups SET status = 'deleted' WHERE id = $1
	`, groupID)

	return err
}

// AddMember adds a member to a group.
func (p *Provider) AddMember(ctx context.Context, groupID string, member *idprovider.Member) error {
	_, err := p.db.ExecContext(ctx, `
		INSERT INTO directory_group_memberships (group_id, member_kind, member_id, role, status)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (group_id, member_kind, member_id) WHERE status = 'active' DO NOTHING
	`, groupID, member.Kind, member.MemberID, member.Role, member.Status)

	return err
}

// RemoveMember removes a member from a group.
func (p *Provider) RemoveMember(ctx context.Context, groupID, memberID string) error {
	_, err := p.db.ExecContext(ctx, `
		UPDATE directory_group_memberships SET status = 'deleted' WHERE group_id = $1 AND member_id = $2
	`, groupID, memberID)

	return err
}
