package carddavgw

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

type CreateAddressBookRequest struct {
	UserID      string
	Name        string
	Description string
}

type ListAddressBooksRequest struct {
	UserID string
	Status string
	Limit  int
}

type GetAddressBookRequest struct {
	UserID        string
	AddressBookID string
	Status        string
}

type UpsertContactObjectRequest struct {
	UserID        string
	AddressBookID string
	ObjectName    string
	UID           string
	VCard         []byte
	ObservedETag  string
}

type ListContactObjectsRequest struct {
	UserID        string
	AddressBookID string
	Status        string
	Limit         int
}

type GetContactObjectRequest struct {
	UserID        string
	AddressBookID string
	ObjectName    string
	Status        string
}

type DeleteContactObjectRequest struct {
	UserID        string
	AddressBookID string
	ObjectName    string
}

type UpdateAddressBookRequest struct {
	UserID        string
	AddressBookID string
	Name          *string
	Description   *string
}

type ListAddressBookChangesSinceRequest struct {
	UserID        string
	AddressBookID string
	SyncToken     string
	Limit         int
}

func (r *Repository) CreateAddressBook(ctx context.Context, req CreateAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateCreateAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book create: %w", err)
	}
	defer tx.Rollback()
	const query = `
WITH active_user AS (
  SELECT u.id AS user_id, d.id AS domain_id, c.id AS company_id
  FROM users u
  JOIN domains d ON d.id = u.domain_id
  JOIN companies c ON c.id = d.company_id
  WHERE u.id = $1::uuid
    AND u.status = 'active'
    AND d.status = 'active'
    AND c.status = 'active'
)
INSERT INTO carddav_addressbooks (
  company_id, domain_id, user_id, name, normalized_name, description, sync_token
)
SELECT company_id, domain_id, user_id, $2, $3, $4, $5
FROM active_user
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.Name,
		normalizedName,
		req.Description,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("active user not found")
		}
		return AddressBook{}, fmt.Errorf("create CardDAV address book: %w", err)
	}
	if err := insertAddressBookChange(ctx, tx, book.UserID, book.ID, book.SyncToken, "addressbook-created", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book create: %w", err)
	}
	return book, nil
}

func (r *Repository) ListAddressBooks(ctx context.Context, req ListAddressBooksRequest) ([]AddressBook, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListAddressBooksRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text, user_id::text, name, description, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND status = $2
ORDER BY updated_at DESC, id DESC
LIMIT $3`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.Status, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV address books: %w", err)
	}
	defer rows.Close()
	var books []AddressBook
	for rows.Next() {
		var book AddressBook
		if err := rows.Scan(&book.ID, &book.UserID, &book.Name, &book.Description, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV address book: %w", err)
		}
		books = append(books, book)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV address books: %w", err)
	}
	return books, nil
}

func (r *Repository) GetAddressBook(ctx context.Context, req GetAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, err := ValidateGetAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	const query = `
SELECT id::text, user_id::text, name, description, sync_token, created_at, updated_at
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = $3`
	var book AddressBook
	if err := r.db.QueryRowContext(ctx, query, req.UserID, req.AddressBookID, req.Status).Scan(&book.ID, &book.UserID, &book.Name, &book.Description, &book.SyncToken, &book.CreatedAt, &book.UpdatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("CardDAV address book not found")
		}
		return AddressBook{}, fmt.Errorf("get CardDAV address book: %w", err)
	}
	return book, nil
}

func (r *Repository) UpdateAddressBookProperties(ctx context.Context, req UpdateAddressBookRequest) (AddressBook, error) {
	if r == nil || r.db == nil {
		return AddressBook{}, fmt.Errorf("database handle is required")
	}
	req, normalizedName, syncToken, err := ValidateUpdateAddressBookRequest(req)
	if err != nil {
		return AddressBook{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return AddressBook{}, fmt.Errorf("begin CardDAV address book update: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return AddressBook{}, err
	}
	nameValue, nameSet := optionalStringArg(req.Name)
	descriptionValue, descriptionSet := optionalStringArg(req.Description)
	const query = `
UPDATE carddav_addressbooks
SET
  name = CASE WHEN $3 THEN $4 ELSE name END,
  normalized_name = CASE WHEN $3 THEN $5 ELSE normalized_name END,
  description = CASE WHEN $6 THEN $7 ELSE description END,
  sync_token = $8,
  updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
RETURNING id::text, user_id::text, name, description, sync_token, created_at, updated_at`
	var book AddressBook
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		nameSet,
		nameValue,
		normalizedName,
		descriptionSet,
		descriptionValue,
		syncToken,
	).Scan(
		&book.ID,
		&book.UserID,
		&book.Name,
		&book.Description,
		&book.SyncToken,
		&book.CreatedAt,
		&book.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AddressBook{}, fmt.Errorf("CardDAV address book not found")
		}
		return AddressBook{}, fmt.Errorf("update CardDAV address book properties: %w", err)
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.AddressBookID, syncToken, "addressbook-updated", "", ""); err != nil {
		return AddressBook{}, err
	}
	if err := tx.Commit(); err != nil {
		return AddressBook{}, fmt.Errorf("commit CardDAV address book update: %w", err)
	}
	return book, nil
}

func (r *Repository) UpsertContactObject(ctx context.Context, req UpsertContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, etag, syncToken, err := ValidateUpsertContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactObject{}, fmt.Errorf("begin CardDAV contact upsert: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if req.ObservedETag != "" {
		if err := ensureContactObjectETag(ctx, tx, req.UserID, req.AddressBookID, req.ObjectName, req.ObservedETag); err != nil {
			return ContactObject{}, err
		}
	}
	const query = `
INSERT INTO carddav_contact_objects (
  user_id, addressbook_id, object_name, uid, etag, size, vcard
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6, $7)
ON CONFLICT (addressbook_id, object_name) WHERE status = 'active'
DO UPDATE SET
  uid = EXCLUDED.uid,
  etag = EXCLUDED.etag,
  size = EXCLUDED.size,
  vcard = EXCLUDED.vcard,
  updated_at = now()
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at`
	var object ContactObject
	err = tx.QueryRowContext(ctx, query,
		req.UserID,
		req.AddressBookID,
		req.ObjectName,
		req.UID,
		etag,
		len(req.VCard),
		string(req.VCard),
	).Scan(
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
		return ContactObject{}, fmt.Errorf("upsert CardDAV contact object: %w", err)
	}
	if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
		return ContactObject{}, err
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.AddressBookID, syncToken, "contact-upserted", req.ObjectName, etag); err != nil {
		return ContactObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactObject{}, fmt.Errorf("commit CardDAV contact upsert: %w", err)
	}
	return object, nil
}

func (r *Repository) ListContactObjects(ctx context.Context, req ListContactObjectsRequest) ([]ContactObject, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListContactObjectsRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
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
		if err := rows.Scan(&object.ID, &object.UserID, &object.AddressBookID, &object.ObjectName, &object.UID, &object.ETag, &object.Size, &object.VCard, &object.CreatedAt, &object.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV contact object: %w", err)
		}
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
SELECT id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at
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
		&object.CreatedAt,
		&object.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ContactObject{}, fmt.Errorf("CardDAV contact object not found")
		}
		return ContactObject{}, fmt.Errorf("get CardDAV contact object: %w", err)
	}
	return object, nil
}

func (r *Repository) DeleteContactObject(ctx context.Context, req DeleteContactObjectRequest) (ContactObject, error) {
	if r == nil || r.db == nil {
		return ContactObject{}, fmt.Errorf("database handle is required")
	}
	req, syncToken, err := ValidateDeleteContactObjectRequest(req)
	if err != nil {
		return ContactObject{}, err
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return ContactObject{}, fmt.Errorf("begin CardDAV contact delete: %w", err)
	}
	defer tx.Rollback()
	if err := lockActiveAddressBook(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	if err := ensureAddressBookSyncMarker(ctx, tx, req.UserID, req.AddressBookID); err != nil {
		return ContactObject{}, err
	}
	const query = `
UPDATE carddav_contact_objects
SET status = 'deleted', deleted_at = now(), updated_at = now()
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
RETURNING id::text, user_id::text, addressbook_id::text, object_name, uid, etag, size, vcard, created_at, updated_at`
	var object ContactObject
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
			return ContactObject{}, fmt.Errorf("CardDAV contact object not found")
		}
		return ContactObject{}, fmt.Errorf("delete CardDAV contact object: %w", err)
	}
	if err := updateAddressBookSyncToken(ctx, tx, req.UserID, req.AddressBookID, syncToken); err != nil {
		return ContactObject{}, err
	}
	if err := insertAddressBookChange(ctx, tx, req.UserID, req.AddressBookID, syncToken, "contact-deleted", req.ObjectName, object.ETag); err != nil {
		return ContactObject{}, err
	}
	if err := tx.Commit(); err != nil {
		return ContactObject{}, fmt.Errorf("commit CardDAV contact delete: %w", err)
	}
	return object, nil
}

func (r *Repository) ListAddressBookChangesSince(ctx context.Context, req ListAddressBookChangesSinceRequest) ([]AddressBookChange, error) {
	if r == nil || r.db == nil {
		return nil, fmt.Errorf("database handle is required")
	}
	req, err := ValidateListAddressBookChangesSinceRequest(req)
	if err != nil {
		return nil, err
	}
	const query = `
WITH marker AS (
  SELECT id
  FROM carddav_addressbook_changes
  WHERE user_id = $1::uuid
    AND addressbook_id = $2::uuid
    AND sync_token = $3
)
SELECT c.id, c.user_id::text, c.addressbook_id::text, c.object_name, c.etag, c.action, c.sync_token, c.changed_at
FROM carddav_addressbook_changes c
JOIN marker m ON c.id > m.id
WHERE c.user_id = $1::uuid
  AND c.addressbook_id = $2::uuid
ORDER BY c.id ASC
LIMIT $4`
	rows, err := r.db.QueryContext(ctx, query, req.UserID, req.AddressBookID, req.SyncToken, req.Limit)
	if err != nil {
		return nil, fmt.Errorf("list CardDAV sync changes: %w", err)
	}
	defer rows.Close()
	var changes []AddressBookChange
	for rows.Next() {
		var change AddressBookChange
		if err := rows.Scan(&change.ID, &change.UserID, &change.AddressBookID, &change.ObjectName, &change.ETag, &change.Action, &change.SyncToken, &change.ChangedAt); err != nil {
			return nil, fmt.Errorf("scan CardDAV sync change: %w", err)
		}
		changes = append(changes, change)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate CardDAV sync changes: %w", err)
	}
	if len(changes) == 0 {
		var markerExists bool
		err := r.db.QueryRowContext(ctx, `
SELECT EXISTS (
  SELECT 1
  FROM carddav_addressbook_changes
  WHERE user_id = $1::uuid
    AND addressbook_id = $2::uuid
    AND sync_token = $3
)`, req.UserID, req.AddressBookID, req.SyncToken).Scan(&markerExists)
		if err != nil {
			return nil, fmt.Errorf("check CardDAV sync marker: %w", err)
		}
		if !markerExists {
			return nil, InvalidSyncTokenError{Token: req.SyncToken}
		}
	}
	return changes, nil
}

func ValidateCreateAddressBookRequest(req CreateAddressBookRequest) (CreateAddressBookRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	name, err := ValidateAddressBookName(req.Name)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	normalizedName, err := NormalizeAddressBookName(name)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	description, err := ValidateAddressBookDescription(req.Description)
	if err != nil {
		return CreateAddressBookRequest{}, "", "", err
	}
	syncToken := AddressBookSyncToken(userID, normalizedName, time.Now().UTC().Format(time.RFC3339Nano))
	return CreateAddressBookRequest{UserID: userID, Name: name, Description: description}, normalizedName, syncToken, nil
}

func ValidateListAddressBooksRequest(req ListAddressBooksRequest) (ListAddressBooksRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListAddressBooksRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return ListAddressBooksRequest{}, err
	}
	return ListAddressBooksRequest{UserID: userID, Status: status, Limit: normalizeCardDAVLimit(req.Limit)}, nil
}

func ValidateGetAddressBookRequest(req GetAddressBookRequest) (GetAddressBookRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	status, err := ValidateAddressBookStatus(req.Status)
	if err != nil {
		return GetAddressBookRequest{}, err
	}
	return GetAddressBookRequest{UserID: userID, AddressBookID: bookID, Status: status}, nil
}

func ValidateUpdateAddressBookRequest(req UpdateAddressBookRequest) (UpdateAddressBookRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return UpdateAddressBookRequest{}, "", "", err
	}
	if req.Name == nil && req.Description == nil {
		return UpdateAddressBookRequest{}, "", "", fmt.Errorf("at least one address book property is required")
	}
	var normalizedName string
	var name *string
	if req.Name != nil {
		value, err := ValidateAddressBookName(*req.Name)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		normalizedName, err = NormalizeAddressBookName(value)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		name = &value
	}
	var description *string
	if req.Description != nil {
		value, err := ValidateAddressBookDescription(*req.Description)
		if err != nil {
			return UpdateAddressBookRequest{}, "", "", err
		}
		description = &value
	}
	syncToken := AddressBookSyncToken(userID, bookID, "addressbook-update", time.Now().UTC().Format(time.RFC3339Nano))
	return UpdateAddressBookRequest{UserID: userID, AddressBookID: bookID, Name: name, Description: description}, normalizedName, syncToken, nil
}

func ValidateUpsertContactObjectRequest(req UpsertContactObjectRequest) (UpsertContactObjectRequest, string, string, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
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
	return UpsertContactObjectRequest{UserID: userID, AddressBookID: bookID, ObjectName: objectName, UID: uid, VCard: req.VCard, ObservedETag: observedETag}, etag, syncToken, nil
}

func ValidateListContactObjectsRequest(req ListContactObjectsRequest) (ListContactObjectsRequest, error) {
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
	return ListContactObjectsRequest{UserID: userID, AddressBookID: bookID, Status: status, Limit: normalizeCardDAVLimit(req.Limit)}, nil
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
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	objectName, err := ValidateContactObjectName(req.ObjectName)
	if err != nil {
		return DeleteContactObjectRequest{}, "", err
	}
	syncToken := AddressBookSyncToken(userID, bookID, objectName, "contact-delete", time.Now().UTC().Format(time.RFC3339Nano))
	return DeleteContactObjectRequest{UserID: userID, AddressBookID: bookID, ObjectName: objectName}, syncToken, nil
}

func ValidateListAddressBookChangesSinceRequest(req ListAddressBookChangesSinceRequest) (ListAddressBookChangesSinceRequest, error) {
	userID, err := validateCardDAVID("user_id", req.UserID, true)
	if err != nil {
		return ListAddressBookChangesSinceRequest{}, err
	}
	bookID, err := validateCardDAVID("addressbook_id", req.AddressBookID, true)
	if err != nil {
		return ListAddressBookChangesSinceRequest{}, err
	}
	syncToken := strings.TrimSpace(req.SyncToken)
	if syncToken == "" {
		return ListAddressBookChangesSinceRequest{}, fmt.Errorf("sync token is required")
	}
	if len(syncToken) > 128 || strings.ContainsAny(syncToken, "\r\n") {
		return ListAddressBookChangesSinceRequest{}, fmt.Errorf("sync token is invalid")
	}
	return ListAddressBookChangesSinceRequest{UserID: userID, AddressBookID: bookID, SyncToken: syncToken, Limit: normalizeCardDAVLimit(req.Limit)}, nil
}

type addressBookChangeExecer interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

func insertAddressBookChange(ctx context.Context, execer addressBookChangeExecer, userID string, addressBookID string, syncToken string, action string, objectName string, etag string) error {
	_, err := execer.ExecContext(ctx, `
INSERT INTO carddav_addressbook_changes (
  user_id, addressbook_id, sync_token, action, object_name, etag
) VALUES ($1::uuid, $2::uuid, $3, $4, $5, $6)`, userID, addressBookID, syncToken, action, objectName, etag)
	if err != nil {
		return fmt.Errorf("insert CardDAV address book change: %w", err)
	}
	return nil
}

func optionalStringArg(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	return *value, true
}

func lockActiveAddressBook(ctx context.Context, tx *sql.Tx, userID string, addressBookID string) error {
	var id string
	err := tx.QueryRowContext(ctx, `
SELECT id::text
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'
FOR UPDATE`, userID, addressBookID).Scan(&id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV address book not found")
		}
		return fmt.Errorf("lock CardDAV address book: %w", err)
	}
	return nil
}

func ensureContactObjectETag(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, objectName string, etag string) error {
	var current string
	err := tx.QueryRowContext(ctx, `
SELECT etag
FROM carddav_contact_objects
WHERE user_id = $1::uuid
  AND addressbook_id = $2::uuid
  AND object_name = $3
  AND status = 'active'
FOR UPDATE`, userID, addressBookID, objectName).Scan(&current)
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

func updateAddressBookSyncToken(ctx context.Context, tx *sql.Tx, userID string, addressBookID string, syncToken string) error {
	res, err := tx.ExecContext(ctx, `
UPDATE carddav_addressbooks
SET sync_token = $3, updated_at = now()
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, addressBookID, syncToken)
	if err != nil {
		return fmt.Errorf("update CardDAV sync token: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("read CardDAV sync token update count: %w", err)
	}
	if affected != 1 {
		return fmt.Errorf("CardDAV address book not found")
	}
	return nil
}

func ensureAddressBookSyncMarker(ctx context.Context, tx *sql.Tx, userID string, addressBookID string) error {
	var token string
	err := tx.QueryRowContext(ctx, `
SELECT sync_token
FROM carddav_addressbooks
WHERE user_id = $1::uuid
  AND id = $2::uuid
  AND status = 'active'`, userID, addressBookID).Scan(&token)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("CardDAV address book not found")
		}
		return fmt.Errorf("read CardDAV sync marker: %w", err)
	}
	_, err = tx.ExecContext(ctx, `
INSERT INTO carddav_addressbook_changes (
  user_id, addressbook_id, sync_token, action
)
SELECT $1::uuid, $2::uuid, $3, 'addressbook-created'
WHERE NOT EXISTS (
  SELECT 1
  FROM carddav_addressbook_changes
  WHERE addressbook_id = $2::uuid
    AND sync_token = $3
    AND action = 'addressbook-created'
)`, userID, addressBookID, token)
	if err != nil {
		return fmt.Errorf("ensure CardDAV sync marker: %w", err)
	}
	return nil
}

func validateOptionalContactETag(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return ValidateContactObjectETag(value)
}

func validateCardDAVID(field string, value string, required bool) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		if required {
			return "", fmt.Errorf("%s is required", field)
		}
		return "", nil
	}
	if len(value) > maxSegmentBytes {
		return "", fmt.Errorf("%s is too long", field)
	}
	if strings.ContainsAny(value, "\r\n") {
		return "", fmt.Errorf("%s must not contain line breaks", field)
	}
	return value, nil
}

func normalizeCardDAVLimit(limit int) int {
	if limit <= 0 {
		return 200
	}
	if limit > 1000 {
		return 1000
	}
	return limit
}
