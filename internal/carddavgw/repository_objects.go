package carddavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/lib/pq"
)

func (r *Repository) UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, etag, syncToken, err := ValidateUpsertContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}

	cleanVCard, photoMediaType, photoData, _ := extractPhotoFromVCard(req.VCard)
	cleanVCard, categories, group, _ := extractCategoriesAndGroupFromVCard(cleanVCard)

	var object ContactObject
	if err := runCardDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CardDAV contact upsert: %w", err)
		}
		defer tx.Rollback()
		if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
			return err
		}
		if req.ObservedETag != "" {
			if err := ensureContactObjectETag(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.ObservedETag); err != nil {
				return err
			}
		}
		const query = `
INSERT INTO carddav_contact_objects (
  user_id, addressbook_id, object_name, uid, etag, size, vcard, photo_data, photo_media_type, categories_list, group_name
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8, $9, $10, $11)
ON CONFLICT (addressbook_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  vcard = EXCLUDED.vcard,
  photo_data = EXCLUDED.photo_data,
  photo_media_type = EXCLUDED.photo_media_type,
  categories_list = EXCLUDED.categories_list,
  group_name = EXCLUDED.group_name,
  updated_at = now()
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, photo_data, photo_media_type, categories_list, group_name, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query,
			req.UserID,
			req.AddressBookID,
			req.ObjectName,
			req.UID,
			etag,
			len(cleanVCard),
			string(cleanVCard),
			photoData,
			photoMediaType,
			categories,
			group,
		).Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.PhotoData,
			&object.PhotoMediaType,
			pq.Array(&object.Categories),
			&object.Group,
			&object.CreatedAt,
			&object.UpdatedAt,
		)
		if err != nil {
			return mapContactObjectUpsertError(err)
		}
		if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
			return err
		}
		if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "contact-upserted", req.ObjectName, etag); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CardDAV contact upsert: %w", err)
		}
		return nil
	}); err != nil {
		return ContactObject{}, err
	}
	return object, nil
}

func (r *Repository) ListContactObjects(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	req, err := ValidateListContactObjectsRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listContactObjects(ctx, req)
}

func (r *Repository) listContactObjectsForSync(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	req, err := ValidateListContactObjectsForSyncRequest(req)
	if err != nil {
		return nil, err
	}
	return r.listContactObjects(ctx, req)
}

func (r *Repository) listContactObjects(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, photo_data, COALESCE(photo_media_type, ''), categories_list, COALESCE(group_name, ''), created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND status = $3
ORDER BY updated_at DESC, id DESC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.AddressBookID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV contact objects: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.PhotoData, &object.PhotoMediaType, pq.Array(&object.Categories), &object.Group, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV contact object: %w", err)
		}
		object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
		object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) GetContactObject(ctx context.Context, req GetContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, photo_data, COALESCE(photo_media_type, ''), categories_list, COALESCE(group_name, ''), created_at, updated_at
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = $4`
	var object ContactObject
	err = r.db.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.ObjectName, req.Status).Scan(
		&object.ID,
		&object.UserID,
		&object.AddressBookID,
		&object.ObjectName,
		&object.UID,
		&object.ETag,
		&object.Size,
		&object.VCard,
		&object.PhotoData,
		&object.PhotoMediaType,
		pq.Array(&object.Categories),
		&object.Group,
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ContactObject{}, fmt.Errorf("CardDAV contact object not found")
		}
		return ContactObject{}, fmt.Errorf("get CardDAV contact object: %w", err)
	}
	object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
	object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
	return object, nil
}

func (r *Repository) ListContactObjectsByNameGroups(ctx context.Context, userID string, objectNamesByAddressBook map[string][]string, status string) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	userID, err := validateCardDAVID("user_id", userID, true)
	if err != nil {
		return nil, err
	}
	status, err = ValidateAddressBookStatus(status)
	if err != nil {
		return nil, err
	}
	seen := make(map[contactObjectNameLookup]struct{})
	lookups := make([]contactObjectNameLookup, 0)
	for addressBookID, names := range objectNamesByAddressBook {
		addressBookID, err := validateCardDAVID("addressbook_id", addressBookID, true)
		if err != nil {
			return nil, err
		}
		for _, name := range names {
			name, err := ValidateContactObjectName(name)
			if err != nil {
				return nil, err
			}
			key := contactObjectNameLookup{addressBookID: addressBookID, objectName: name}
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			lookups = append(lookups, key)
		}
	}
	if len(lookups) == 0 {
		return nil, nil
	}

	objects := make([]ContactObject, 0, len(lookups))
	for start := 0; start < len(lookups); start += carddavContactObjectLookupBatchSize {
		end := start + carddavContactObjectLookupBatchSize
		if end > len(lookups) {
			end = len(lookups)
		}
		chunkObjects, err := r.listContactObjectsByNameGroupsChunk(ctx, userID, status, lookups[start:end])
		if err != nil {
			return nil, err
		}
		objects = append(objects, chunkObjects...)
	}
	return objects, nil
}

func (r *Repository) listContactObjectsByNameGroupsChunk(ctx context.Context, userID string, status string, lookups []contactObjectNameLookup) ([]ContactObject, error) {
	var values strings.Builder
	args := make([]any, 0, 2+len(lookups)*2)
	args = append(args, userID, status)
	for i, lookup := range lookups {
		if i > 0 {
			values.WriteString(", ")
		}
		addressBookArg := len(args) + 1
		objectNameArg := len(args) + 2
		values.WriteString(fmt.Sprintf("($%d::uuid, $%d)", addressBookArg, objectNameArg))
		args = append(args, lookup.addressBookID, lookup.objectName)
	}
	query := `
WITH requested(addressbook_id, object_name) AS (
  VALUES ` + values.String() + `
)
SELECT c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.photo_data,
       COALESCE(c.photo_media_type, ''),
       c.categories_list,
       COALESCE(c.group_name, ''),
       c.created_at,
       c.updated_at
FROM requested r
JOIN carddav_contact_objects c
  ON c.addressbook_id = r.addressbook_id
 AND c.object_name = r.object_name
WHERE c.user_id = $1::uuid
  AND c.status = $2`
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV contact objects by names: %w", err)
	}
	defer rows.Close()
	objects := make([]ContactObject, 0, len(lookups))
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.PhotoData, &object.PhotoMediaType, pq.Array(&object.Categories), &object.Group, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV contact object by names: %w", err)
		}
		object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
		object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV contact objects by names: %w", err)
	}
	return objects, nil
}

func (r *Repository) SearchContactsByEmail(ctx context.Context, req SearchContactsByEmailRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if req.Email == "" {
		return nil, fmt.Errorf("email is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	const query = `
SELECT c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.photo_data,
       COALESCE(c.photo_media_type, ''),
       c.categories_list,
       COALESCE(c.group_name, ''),
       c.created_at,
       c.updated_at
FROM carddav_contact_objects c
JOIN carddav_addressbooks a ON a.id = c.addressbook_id
WHERE a.user_id = $1::uuid
  AND a.status = 'active'
  AND c.status = 'active'
  AND lower(c.vcard::text) LIKE '%' || lower($2) || '%'
ORDER BY c.updated_at DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Email, limit)
	if err != nil {
		return nil, fmt.Errorf("search contacts by email: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.PhotoData,
			&object.PhotoMediaType,
			pq.Array(&object.Categories),
			&object.Group,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact object: %w", err)
		}
		object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
		object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) SearchContactsByEmails(ctx context.Context, req SearchContactsByEmailsRequest) (map[string][]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	emails := normalizeContactEmailList(req.Emails)
	out := make(map[string][]ContactObject, len(emails))
	if len(emails) == 0 {
		return out, nil
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 1
	}
	if limit > 10 {
		limit = 10
	}
	rows, err := r.db.QueryContext(ctx, buildSearchContactsByEmailsQuery(), req.UserID, pq.Array(emails), limit)
	if err != nil {
		return nil, fmt.Errorf("search contacts by emails: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var email string
		var object ContactObject
		if err := rows.Scan(
			&email,
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.PhotoData,
			&object.PhotoMediaType,
			pq.Array(&object.Categories),
			&object.Group,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact object by email batch: %w", err)
		}
		object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
		object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
		key := strings.ToLower(strings.TrimSpace(email))
		out[key] = append(out[key], object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact objects by email batch: %w", err)
	}
	return out, nil
}

func buildSearchContactsByEmailsQuery() string {
	return `
WITH requested AS (
  SELECT email, email_order
  FROM unnest($2::text[]) WITH ORDINALITY AS req(email, email_order)
)
SELECT req.email,
       c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.photo_data,
       COALESCE(c.photo_media_type, ''),
       c.categories_list,
       COALESCE(c.group_name, ''),
       c.created_at,
       c.updated_at
FROM requested req
JOIN LATERAL (
  SELECT c.id,
         c.user_id,
         c.addressbook_id,
         c.object_name,
         c.uid,
         c.etag,
         c.size,
         c.vcard,
         c.photo_data,
         COALESCE(c.photo_media_type, ''),
         c.categories_list,
         COALESCE(c.group_name, ''),
         c.created_at,
         c.updated_at
  FROM carddav_contact_objects c
  JOIN carddav_addressbooks a ON a.id = c.addressbook_id
  WHERE a.user_id = $1::uuid
    AND a.status = 'active'
    AND c.status = 'active'
    AND lower(c.vcard::text) LIKE '%' || lower(req.email) || '%'
  ORDER BY c.updated_at DESC
  LIMIT $3
) c ON true
ORDER BY req.email_order, c.updated_at DESC`
}

func normalizeContactEmailList(emails []string) []string {
	out := make([]string, 0, len(emails))
	seen := make(map[string]struct{}, len(emails))
	for _, email := range emails {
		email = strings.TrimSpace(email)
		if email == "" {
			continue
		}
		key := strings.ToLower(email)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, email)
	}
	return out
}

func (r *Repository) SearchContacts(ctx context.Context, req SearchContactsRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	if req.UserID == "" {
		return nil, fmt.Errorf("user id is required")
	}
	if req.Query == "" {
		return nil, fmt.Errorf("query is required")
	}
	limit := req.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	const query = `
SELECT c.id::text,
       c.user_id::text,
       c.addressbook_id::text,
       c.object_name,
       c.uid,
       c.etag,
       c.size,
       c.vcard,
       c.photo_data,
       COALESCE(c.photo_media_type, ''),
       c.categories_list,
       COALESCE(c.group_name, ''),
       c.created_at,
       c.updated_at
FROM carddav_contact_objects c
JOIN carddav_addressbooks a ON a.id = c.addressbook_id
WHERE a.user_id = $1::uuid
  AND a.status = 'active'
  AND c.status = 'active'
  AND (lower(c.vcard::text) LIKE '%' || lower($2) || '%')
ORDER BY c.updated_at DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Query, limit)
	if err != nil {
		return nil, fmt.Errorf("search contacts: %w", err)
	}
	defer rows.Close()
	var objects []ContactObject
	for rows.Next() {
		var object ContactObject
		if err := rows.Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.PhotoData,
			&object.PhotoMediaType,
			pq.Array(&object.Categories),
			&object.Group,
			&object.CreatedAt,
			&object.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan contact object: %w", err)
		}
		object.VCard = mergePhotoIntoVCard(object.VCard, object.PhotoData, object.PhotoMediaType)
		object.VCard = mergeCategoriesAndGroupIntoVCard(object.VCard, object.Categories, object.Group)
		objects = append(objects, object)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate contact objects: %w", err)
	}
	return objects, nil
}

func (r *Repository) DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, syncToken, err := ValidateDeleteContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	var object ContactObject
	if err := runCardDAVWriteWithRetry(ctx, func() error {
		tx, err := r.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("begin CardDAV contact delete: %w", err)
		}
		defer tx.Rollback()
		if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
			return err
		}
		if req.ObservedETag != "" {
			if err := ensureContactObjectETag(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.ObservedETag); err != nil {
				return err
			}
		}
		const query = `
UPDATE carddav_contact_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at`
		err = tx.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.ObjectName).Scan(
			&object.ID,
			&object.UserID,
			&object.AddressBookID,
			&object.ObjectName,
			&object.UID,
			&object.ETag,
			&object.Size,
			&object.VCard,
			&object.CreatedAt,
			&object.UpdatedAt,
		)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("CardDAV contact object not found")
			}
			return fmt.Errorf("delete CardDAV contact object: %w", err)
		}
		if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
			return err
		}
		if err := insertAddressBookChange(ctx, tx, req.UserID, req.ActorUserID, req.AddressBookID, syncToken, "contact-deleted", req.ObjectName, object.ETag); err != nil {
			return err
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit CardDAV contact delete: %w", err)
		}
		return nil
	}); err != nil {
		return ContactObject{}, err
	}
	return object, nil
}

func ensureContactObjectETag(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, objectName string, etag string) error {
	var current string
	err := tx.QueryRowContext(ctx, `
SELECT etag
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'`, userID, addressBookID, objectName).Scan(&current)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV contact object not found")
		}
		return fmt.Errorf("read CardDAV contact object etag: %w", err)
	}
	if current != etag {
		return fmt.Errorf("CardDAV contact object etag mismatch")
	}
	return nil
}

func ValidateUpsertContactObjectRequest(req UpsertContactObjectRequest) (UpsertContactObjectRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	meta, err := ValidateVCardObject(req.VCard)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	uid := strings.TrimSpace(req.UID)
	if uid == "" {
		uid = meta.UID
	}
	uid, err = ValidateContactObjectUID(uid)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	if uid != meta.UID {
		return UpsertContactObjectRequest{}, "", "", fmt.Errorf("contact object uid does not match vcard UID")
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	etag, err := ContactObjectETag(req.VCard)
	if err != nil {
		return UpsertContactObjectRequest{}, "", "", err
	}
	syncToken := AddressBookSyncToken(userID, bookID, objectName, etag, time.Now().UTC().Format(time.RFC3339Nano))
	return UpsertContactObjectRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, ObjectName: objectName, UID: uid, VCard: req.VCard, ObservedETag: observedETag}, etag, syncToken, nil
}

func ValidateListContactObjectsRequest(req ListContactObjectsRequest) (ListContactObjectsRequest, error) {
	return validateListContactObjectsRequest(req, normalizeCardDAVLimit)
}

func ValidateListContactObjectsForSyncRequest(req ListContactObjectsRequest) (ListContactObjectsRequest, error) {
	return validateListContactObjectsRequest(req, normalizeCardDAVChangeLimit)
}

func validateListContactObjectsRequest(req ListContactObjectsRequest, normalizeLimit func(int) int) (ListContactObjectsRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return ListContactObjectsRequest{}, err
	}
	return ListContactObjectsRequest{UserID: userID, AddressBookID: bookID, Status: status, Limit: normalizeLimit(req.Limit)}, nil
}

func ValidateGetContactObjectRequest(req GetContactObjectRequest) (GetContactObjectRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return GetContactObjectRequest{}, err
	}
	return GetContactObjectRequest{UserID: userID, AddressBookID: bookID, ObjectName: objectName, Status: status}, nil
}

func ValidateDeleteContactObjectRequest(req DeleteContactObjectRequest) (DeleteContactObjectRequest, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	actorUserID, err := validateCardDAVActorUserID(req.ActorUserID, userID)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	observedETag, err := validateOptionalContactETag(req.ObservedETag)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	syncToken := AddressBookSyncToken(userID, bookID, objectName, "contact-delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteContactObjectRequest{UserID: userID, ActorUserID: actorUserID, AddressBookID: bookID, ObjectName: objectName, ObservedETag: observedETag}, syncToken, nil
}
