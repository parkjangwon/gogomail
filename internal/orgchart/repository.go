package orgchart

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository manages organization structure in the database.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new organization repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// CreateUnit creates a new organization unit.
func (r *Repository) CreateUnit(ctx context.Context, unit *OrganizationUnit) error {
	if unit.CompanyID == "" || unit.Name == "" {
		return fmt.Errorf("company_id and name are required")
	}

	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO organization_units (company_id, parent_id, name, name_normalized, type, description, display_name, manager_user_id, status)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id, created_at, updated_at`,
		unit.CompanyID, unit.ParentID, unit.Name, normalizeString(unit.Name),
		unit.Type, unit.Description, unit.DisplayName, unit.ManagerUserID, unit.Status,
	).Scan(&unit.ID, &unit.CreatedAt, &unit.UpdatedAt)

	return err
}

// GetUnit retrieves an organization unit by ID.
func (r *Repository) GetUnit(ctx context.Context, id string) (*OrganizationUnit, error) {
	var unit OrganizationUnit
	err := r.db.QueryRowContext(
		ctx,
		`SELECT id, company_id, parent_id, name, type, description, display_name, manager_user_id, status, created_at, updated_at
		FROM organization_units WHERE id = $1`,
		id,
	).Scan(
		&unit.ID, &unit.CompanyID, &unit.ParentID, &unit.Name, &unit.Type,
		&unit.Description, &unit.DisplayName, &unit.ManagerUserID, &unit.Status,
		&unit.CreatedAt, &unit.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("organization unit not found")
	}
	return &unit, err
}

// ListUnits lists organization units for a company.
func (r *Repository) ListUnits(ctx context.Context, companyID string) ([]OrganizationUnit, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, company_id, parent_id, name, type, description, display_name, manager_user_id, status, created_at, updated_at
		FROM organization_units WHERE company_id = $1 ORDER BY name`,
		companyID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var units []OrganizationUnit
	for rows.Next() {
		var unit OrganizationUnit
		if err := rows.Scan(
			&unit.ID, &unit.CompanyID, &unit.ParentID, &unit.Name, &unit.Type,
			&unit.Description, &unit.DisplayName, &unit.ManagerUserID, &unit.Status,
			&unit.CreatedAt, &unit.UpdatedAt,
		); err != nil {
			return nil, err
		}
		units = append(units, unit)
	}
	return units, rows.Err()
}

// UpdateUnit updates an organization unit.
func (r *Repository) UpdateUnit(ctx context.Context, unit *OrganizationUnit) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE organization_units SET name = $1, name_normalized = $2, type = $3, description = $4,
		display_name = $5, manager_user_id = $6, status = $7, updated_at = NOW()
		WHERE id = $8`,
		unit.Name, normalizeString(unit.Name), unit.Type, unit.Description,
		unit.DisplayName, unit.ManagerUserID, unit.Status, unit.ID,
	)
	return err
}

// DeleteUnit deletes an organization unit.
func (r *Repository) DeleteUnit(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(
		ctx,
		`DELETE FROM organization_units WHERE id = $1`,
		id,
	)
	return err
}

// AssignUser assigns a user to an organization unit.
func (r *Repository) AssignUser(ctx context.Context, member *OrganizationMember) error {
	if member.OrganizationUnitID == "" || member.UserID == "" {
		return fmt.Errorf("organization_unit_id and user_id are required")
	}

	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO organization_members (organization_unit_id, user_id, role, title, started_at, is_primary)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at`,
		member.OrganizationUnitID, member.UserID, member.Role, member.Title, member.StartedAt, member.IsPrimary,
	).Scan(&member.ID, &member.CreatedAt, &member.UpdatedAt)

	return err
}

// GetMembersInUnit gets all members of an organization unit.
func (r *Repository) GetMembersInUnit(ctx context.Context, unitID string) ([]OrganizationMember, error) {
	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, organization_unit_id, user_id, role, title, started_at, ended_at, is_primary, created_at, updated_at
		FROM organization_members WHERE organization_unit_id = $1 AND ended_at IS NULL ORDER BY created_at`,
		unitID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var members []OrganizationMember
	for rows.Next() {
		var member OrganizationMember
		if err := rows.Scan(
			&member.ID, &member.OrganizationUnitID, &member.UserID, &member.Role,
			&member.Title, &member.StartedAt, &member.EndedAt, &member.IsPrimary,
			&member.CreatedAt, &member.UpdatedAt,
		); err != nil {
			return nil, err
		}
		members = append(members, member)
	}
	return members, rows.Err()
}

// RemoveUser removes a user from an organization unit.
func (r *Repository) RemoveUser(ctx context.Context, memberID string) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE organization_members SET ended_at = NOW(), updated_at = NOW() WHERE id = $1`,
		memberID,
	)
	return err
}

// LogSync logs an organization sync operation.
func (r *Repository) LogSync(ctx context.Context, log *SyncLog) error {
	err := r.db.QueryRowContext(
		ctx,
		`INSERT INTO organization_sync_log (company_id, sync_source, started_at, status)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`,
		log.CompanyID, log.SyncSource, log.StartedAt, log.Status,
	).Scan(&log.ID, &log.CreatedAt)

	return err
}

// UpdateSyncLog updates a sync log with completion info.
func (r *Repository) UpdateSyncLog(ctx context.Context, log *SyncLog) error {
	_, err := r.db.ExecContext(
		ctx,
		`UPDATE organization_sync_log SET status = $1, completed_at = $2, units_created = $3, units_updated = $4, users_synced = $5, error_message = $6
		WHERE id = $7`,
		log.Status, log.CompletedAt, log.UnitsCreated, log.UnitsUpdated, log.UsersSynced, log.ErrorMessage, log.ID,
	)
	return err
}

func normalizeString(s string) string {
	// Normalize for case-insensitive matching
	return s // Go's database should handle this, or use PostgreSQL lower()
}
