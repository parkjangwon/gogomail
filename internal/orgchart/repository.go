package orgchart

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/lib/pq"
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

// ListMembersInUnits gets active members grouped by organization unit.
func (r *Repository) ListMembersInUnits(ctx context.Context, unitIDs []string) (map[string][]OrganizationMember, error) {
	membersByUnit := make(map[string][]OrganizationMember, len(unitIDs))
	if len(unitIDs) == 0 {
		return membersByUnit, nil
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT id, organization_unit_id, user_id, role, title, started_at, ended_at, is_primary, created_at, updated_at
		FROM organization_members
		WHERE organization_unit_id = ANY($1)
		  AND ended_at IS NULL
		ORDER BY organization_unit_id, is_primary DESC, created_at`,
		pq.Array(unitIDs),
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var member OrganizationMember
		if err := rows.Scan(
			&member.ID, &member.OrganizationUnitID, &member.UserID, &member.Role,
			&member.Title, &member.StartedAt, &member.EndedAt, &member.IsPrimary,
			&member.CreatedAt, &member.UpdatedAt,
		); err != nil {
			return nil, err
		}
		membersByUnit[member.OrganizationUnitID] = append(membersByUnit[member.OrganizationUnitID], member)
	}
	return membersByUnit, rows.Err()
}

// ListUnitsForUser lists active organization units currently assigned to a user.
func (r *Repository) ListUnitsForUser(ctx context.Context, userID string) ([]OrganizationUnit, error) {
	if userID == "" {
		return nil, fmt.Errorf("user_id is required")
	}

	rows, err := r.db.QueryContext(
		ctx,
		`SELECT ou.id, ou.company_id, ou.parent_id, ou.name, ou.type, ou.description,
		        ou.display_name, ou.manager_user_id, ou.status, ou.created_at, ou.updated_at
		FROM organization_members om
		JOIN organization_units ou ON ou.id = om.organization_unit_id
		WHERE om.user_id = $1
		  AND om.ended_at IS NULL
		  AND ou.status = 'active'
		ORDER BY om.is_primary DESC, ou.name, ou.id`,
		userID,
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
