-- +goose Up
-- AddCardDAVCategoriesAndGroupSupport
-- Add support for vCard CATEGORIES (comma-separated list) and GROUP properties.
-- RFC 6350 Section 6.7.10 (CATEGORIES) and 6.7.2 (GROUP).

ALTER TABLE carddav_contact_objects
ADD COLUMN categories_list TEXT[],
ADD COLUMN group_name VARCHAR(255);

CREATE INDEX idx_carddav_contact_objects_categories
ON carddav_contact_objects USING GIN(categories_list)
WHERE categories_list IS NOT NULL AND array_length(categories_list, 1) > 0;

CREATE INDEX idx_carddav_contact_objects_group
ON carddav_contact_objects(group_name)
WHERE group_name IS NOT NULL AND group_name != '';

COMMENT ON COLUMN carddav_contact_objects.categories_list IS 'Array of category strings from vCard CATEGORIES property (RFC 6350 6.7.10)';
COMMENT ON COLUMN carddav_contact_objects.group_name IS 'Group identifier from vCard GROUP property (RFC 6350 6.7.2), max 255 chars';

-- +goose Down
DROP INDEX IF EXISTS idx_carddav_contact_objects_group;
DROP INDEX IF EXISTS idx_carddav_contact_objects_categories;
ALTER TABLE carddav_contact_objects
DROP COLUMN IF EXISTS group_name,
DROP COLUMN IF EXISTS categories_list;
