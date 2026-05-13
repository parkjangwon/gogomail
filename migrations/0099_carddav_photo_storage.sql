-- +goose Up
-- AddCardDAVContactPhotoBlobStorage
-- Add support for storing vCard PHOTO as BYTEA blob alongside the vCard text.
-- Photos are stored separately to support binary encodings and large media types.
-- Max size: 5MB per photo (independent of MaxContactObjectBytes).

ALTER TABLE carddav_contact_objects
ADD COLUMN photo_data BYTEA,
ADD COLUMN photo_media_type VARCHAR(100);

CREATE INDEX idx_carddav_contact_objects_photo
ON carddav_contact_objects(addressbook_id, photo_data IS NOT NULL)
WHERE photo_data IS NOT NULL;

-- Constraint: photo_data AND photo_media_type must both be NULL or both be NOT NULL
-- (Database-level check is not possible in PostgreSQL without triggers, enforced at application level)

COMMENT ON COLUMN carddav_contact_objects.photo_data IS 'Binary photo data from vCard PHOTO property, max 5MB';
COMMENT ON COLUMN carddav_contact_objects.photo_media_type IS 'Media type of photo (e.g. image/jpeg, image/png)';

-- +goose Down
DROP INDEX IF EXISTS idx_carddav_contact_objects_photo;
ALTER TABLE carddav_contact_objects
DROP COLUMN IF EXISTS photo_media_type,
DROP COLUMN IF EXISTS photo_data;
