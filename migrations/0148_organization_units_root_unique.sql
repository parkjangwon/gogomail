-- +goose Up
-- The existing UNIQUE(company_id, parent_id, name_normalized) constraint
-- on organization_units does NOT enforce uniqueness for root units,
-- because NULL is not equal to NULL in a multi-column UNIQUE constraint.
-- Add a partial unique index covering root units (parent_id IS NULL) so
-- that two root units cannot share the same normalized name within a company.
CREATE UNIQUE INDEX IF NOT EXISTS idx_organization_units_root_name
    ON organization_units(company_id, name_normalized)
    WHERE parent_id IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_organization_units_root_name;
